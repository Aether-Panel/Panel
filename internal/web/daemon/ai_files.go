package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
	"github.com/gin-gonic/gin"
)

// The file routes of the server assistant (stat, copy and search) work through the toolkit's
// workspace over the server's confined file server, so the node applies the same path checks
// and limits as the toolkit does on the panel. The panel calls them for the server of the
// conversation only; the other file operations of the assistant use the file manager's routes.

// fileErrorKind pairs a workspace error with the status and code the node answers it with
type fileErrorKind struct {
	err    error
	status int
	code   string
}

// fileErrorKinds are checked in order with errors.Is. The panel reads the code back with
// FileErrorForCode, so it reports the same workspace error the node had.
var fileErrorKinds = []fileErrorKind{
	{workspace.ErrNotFound, http.StatusNotFound, skypanel.ErrFileNotFound.Code},
	{workspace.ErrExists, http.StatusConflict, skypanel.ErrFileExists.Code},
	{workspace.ErrTooLarge, http.StatusRequestEntityTooLarge, "ErrFileTooLarge"},
	{workspace.ErrNotDir, http.StatusBadRequest, "ErrFileNotAFolder"},
	{workspace.ErrIsDir, http.StatusBadRequest, "ErrFileIsAFolder"},
	{workspace.ErrSymlink, http.StatusBadRequest, "ErrFileSymlink"},
	{workspace.ErrNotRegular, http.StatusBadRequest, "ErrFileNotRegular"},
	{workspace.ErrInvalidPath, http.StatusBadRequest, "ErrFileInvalidPath"},
	{workspace.ErrOutsideRoot, http.StatusBadRequest, "ErrFileOutsideServer"},
	{workspace.ErrProtected, http.StatusForbidden, "ErrFileProtected"},
	{workspace.ErrInvalid, http.StatusBadRequest, "ErrFileInvalidRequest"},
	{workspace.ErrPermission, http.StatusForbidden, "ErrFilePermission"},
	{workspace.ErrNoSpace, http.StatusInsufficientStorage, "ErrFileNoSpace"},
	{skypanel.ErrDiskLimitExceeded, http.StatusConflict, skypanel.ErrDiskLimitExceeded.Code},
	{workspace.ErrFailed, http.StatusInternalServerError, fileFailedCode},
}

// fileFailedCode is answered for an operation that failed for any other reason
const fileFailedCode = "ErrFileOperationFailed"

// FileErrorStatus returns the status and code the assistant's file routes answer err with.
// known is false for an error that is none of the workspace errors (it may then name host
// paths, so its text is not sent).
func FileErrorStatus(err error) (status int, code string, known bool) {
	for _, kind := range fileErrorKinds {
		if errors.Is(err, kind.err) {
			return kind.status, kind.code, true
		}
	}
	return http.StatusInternalServerError, fileFailedCode, false
}

// FileErrorForCode returns the workspace error a code of the assistant's file routes stands
// for, or nil for a code that is not one of them. A full disk is reported as ErrNoSpace.
func FileErrorForCode(code string) error {
	if code == skypanel.ErrDiskLimitExceeded.Code {
		return workspace.ErrNoSpace
	}
	for _, kind := range fileErrorKinds {
		if kind.code == code {
			return kind.err
		}
	}
	return nil
}

// answerFileError answers a failed file operation with its status, code and user-safe message
func answerFileError(c *gin.Context, err error) {
	status, code, known := FileErrorStatus(err)
	message := err.Error()
	if !known {
		message = workspace.ErrFailed.Error()
	}
	if status >= http.StatusInternalServerError || !known {
		logging.Error.Printf("Assistant file operation failed: %s", err)
	}
	c.AbortWithStatusJSON(status, &skypanel.ErrorResponse{Error: skypanel.CreateError(message, code)})
}

// fileSearchResponse is what the search route answers
type fileSearchResponse struct {
	Matches []workspace.Match `json:"matches"`
	// Truncated is true when the search stopped at the most results it returns
	Truncated bool `json:"truncated"`
} // @name FileSearchResponse

// fileCopyRequest is the body of the copy route
type fileCopyRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
} // @name FileCopyRequest

// @Summary Describe a file or folder
// @Description Describes a file or folder of the server for the assistant. A symlink that stays inside the server folder is described by what it points at.
// @Success 200 {object} workspace.Entry
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Param id path string true "Server ID"
// @Param path query string false "Path relative to the server folder; empty is the folder itself"
// @Tags Daemon Server
// @Router /daemon/server/{id}/fs/stat [get]
// @Security OAuth2Application[none]
func statServerPath(c *gin.Context) {
	server := getServerFromGin(c)
	entry, err := workspace.NewFileServerWorkspace(server.GetFileServer()).Stat(c.Request.Context(), c.Query("path"))
	if err != nil {
		answerFileError(c, err)
		return
	}
	c.JSON(http.StatusOK, entry)
}

// @Summary Copy a file or folder
// @Description Copies a file or a folder tree inside the server folder for the assistant. Symlinks are never followed: inside a copied folder they are skipped. The copy must fit in the server's disk limit.
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 409 {object} skypanel.ErrorResponse "The destination exists, or the copy does not fit in the disk limit"
// @Param id path string true "Server ID"
// @Param body body fileCopyRequest true "Source and destination, relative to the server folder"
// @Tags Daemon Server
// @Router /daemon/server/{id}/fs/copy [post]
// @Security OAuth2Application[none]
func copyServerPath(c *gin.Context) {
	server := getServerFromGin(c)
	var req fileCopyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		answerFileError(c, fmt.Errorf("%w: the body must be a JSON object with source and destination", workspace.ErrInvalid))
		return
	}
	if err := checkCopySpace(server, req.Source); err != nil {
		answerFileError(c, err)
		return
	}
	err := workspace.NewFileServerWorkspace(server.GetFileServer()).Copy(c.Request.Context(), req.Source, req.Destination)
	// the workspace writes without counting bytes: the next check measures the folder again
	server.ForgetDiskUsage()
	if err != nil {
		answerFileError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Search the server's files
// @Description Looks for a text in the names of the files and folders below a folder, or in the lines of its text files, for the assistant. The node's search limits apply (depth, files visited, results, file size).
// @Success 200 {object} fileSearchResponse
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Param id path string true "Server ID"
// @Param dir query string false "Folder to search, relative to the server folder; empty is the folder itself"
// @Param query query string true "Text to look for, case-insensitive"
// @Param content query bool false "true searches the lines of the files instead of the names"
// @Tags Daemon Server
// @Router /daemon/server/{id}/fs/search [get]
// @Security OAuth2Application[none]
func searchServerFiles(c *gin.Context) {
	server := getServerFromGin(c)
	opts := workspace.SearchOptions{Query: c.Query("query"), InContent: c.Query("content") == "true"}
	matches, err := workspace.NewFileServerWorkspace(server.GetFileServer()).Search(c.Request.Context(), c.Query("dir"), opts)
	if err != nil {
		answerFileError(c, err)
		return
	}
	if matches == nil {
		matches = []workspace.Match{}
	}
	c.JSON(http.StatusOK, fileSearchResponse{
		Matches:   matches,
		Truncated: len(matches) >= workspace.ClampSearchOptions(opts).MaxResults,
	})
}

// checkCopySpace refuses a copy that would take the server over its disk limit. The workspace
// writes without counting bytes against the limit, so what would be copied is measured first.
func checkCopySpace(server *servers.Server, source string) error {
	if err := server.CheckDiskSpace(false); err != nil {
		return err
	}
	limit := server.DiskLimit()
	if limit == 0 {
		return nil
	}
	src, err := security.CleanPath(source)
	if err != nil {
		return err
	}
	room := limit - server.DiskUsage()
	if treeSize(server.GetFileServer(), src, room) > room {
		return skypanel.ErrDiskLimitExceeded
	}
	return nil
}

// treeSize adds up the regular files at and below rel, the ones a copy would write. Links are
// not followed (the copy skips them too), and the walk stops once the total passes stop.
func treeSize(fsys files.FileServer, rel string, stop int64) int64 {
	if fsys == nil {
		return 0
	}
	var size int64
	_ = fs.WalkDir(fsys, rel, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			// what cannot be read is not copied either
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		size += info.Size()
		if size > stop {
			return fs.SkipAll
		}
		return nil
	})
	return size
}
