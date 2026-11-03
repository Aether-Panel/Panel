package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/internal/web/daemon"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
)

// nodeCall sends one request to the node that hosts the workspace's server
type nodeCall func(ctx context.Context, method, path string, body io.Reader, header http.Header) (*http.Response, error)

// nodeWorkspace is the toolkit's view of one server's files from the panel. The panel never
// touches files itself: every operation is a call to the node that hosts the server, on that
// server's /daemon/server/<id>/ routes, where the node's confined file server does the work.
// The server is fixed when the workspace is made, from the server the route resolved; no
// argument of a tool can change it. Paths are cleaned here and checked again on the node.
type nodeWorkspace struct {
	prefix string // daemonServerPrefix and the escaped server identifier
	node   string // the node's name, for the log
	call   nodeCall
}

var _ workspace.Workspace = (*nodeWorkspace)(nil)

const (
	// nodeListLimit caps a folder listing read from the node
	nodeListLimit = 16 << 20
	// nodeAnswerLimit caps the other JSON answers of the node
	nodeAnswerLimit = 4 << 20
	// nodeErrorLimit caps the error body read from the node
	nodeErrorLimit = 64 << 10
)

// newNodeWorkspace returns the workspace of server, reached through its node
func newNodeWorkspace(ns *services.Node, server *models.Server) *nodeWorkspace {
	node := &server.Node
	return &nodeWorkspace{
		prefix: daemonServerPrefix + url.PathEscape(server.Identifier),
		node:   node.Name,
		call: func(ctx context.Context, method, p string, body io.Reader, header http.Header) (*http.Response, error) {
			var rc io.ReadCloser
			if body != nil {
				rc = io.NopCloser(body)
			}
			return ns.CallNodeContext(ctx, node, method, p, rc, header)
		},
	}
}

// nodeError is a user-safe error that errors.Is matches with the workspace error it stands for
type nodeError struct {
	msg  string
	kind error
	// coded is true when the node named the error with a code (so it has the routes)
	coded bool
}

func (e *nodeError) Error() string { return e.msg }

func (e *nodeError) Unwrap() error { return e.kind }

func (w *nodeWorkspace) Stat(ctx context.Context, p string) (workspace.Entry, error) {
	rel, err := security.CleanDir(p)
	if err != nil {
		return workspace.Entry{}, err
	}
	var entry workspace.Entry
	err = w.getJSON(ctx, w.prefix+"/fs/stat?"+url.Values{"path": {rel}}.Encode(), rel, &entry)
	return entry, err
}

func (w *nodeWorkspace) List(ctx context.Context, dir string) ([]workspace.Entry, error) {
	rel, err := security.CleanDir(dir)
	if err != nil {
		return nil, err
	}
	res, err := w.do(ctx, http.MethodGet, w.filePath(rel), nil, nil)
	if err != nil {
		return nil, err
	}
	defer utils.CloseResponse(res)
	if res.StatusCode != http.StatusOK {
		return nil, answerError(res, rel)
	}
	if !isJSONAnswer(res) {
		// the file route sends a file's content instead of a listing
		return nil, fmt.Errorf("%w: %s", workspace.ErrNotDir, rel)
	}
	var items []skypanel.FileDesc
	if err := json.NewDecoder(io.LimitReader(res.Body, nodeListLimit)).Decode(&items); err != nil {
		return nil, unreadableAnswer(ctx)
	}
	entries := make([]workspace.Entry, 0, len(items))
	for _, item := range items {
		// ".." (the file manager's way up) and the empty slots of entries it could not read
		// are not part of the folder
		if item.Name == "" || item.Name == "." || item.Name == ".." || strings.Contains(item.Name, "/") {
			continue
		}
		entry := workspace.Entry{Name: item.Name, Path: path.Join(rel, item.Name), IsDir: !item.File}
		if item.File {
			entry.Size = item.Size
		}
		if item.Modified > 0 {
			entry.ModTime = time.Unix(item.Modified, 0)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (w *nodeWorkspace) Read(ctx context.Context, p string, maxBytes int64) ([]byte, error) {
	rel, err := security.CleanPath(p)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		maxBytes = security.Limits.MaxReadBytes
	}
	res, err := w.do(ctx, http.MethodGet, w.filePath(rel), nil, nil)
	if err != nil {
		return nil, err
	}
	defer utils.CloseResponse(res)
	if res.StatusCode != http.StatusOK {
		return nil, answerError(res, rel)
	}
	if isJSONAnswer(res) {
		// the file route sends a listing instead of a file's content
		return nil, fmt.Errorf("%w: %s", workspace.ErrIsDir, rel)
	}
	if res.ContentLength > maxBytes {
		return nil, readTooLarge(rel, maxBytes)
	}
	// only what fits is read: the rest of a bigger file is never downloaded
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, unreadableAnswer(ctx)
	}
	if int64(len(data)) > maxBytes {
		return nil, readTooLarge(rel, maxBytes)
	}
	return data, nil
}

func (w *nodeWorkspace) Write(ctx context.Context, p string, data []byte, overwrite bool) error {
	rel, err := security.CleanPath(p)
	if err != nil {
		return err
	}
	target := w.filePath(rel)
	if !overwrite {
		// the node creates the file with O_EXCL: whatever is already there is never replaced
		target += "?create=true"
	}
	header := http.Header{headerContentType: {"application/octet-stream"}}
	return w.expectNoContent(ctx, http.MethodPut, target, bytes.NewReader(data), header, rel)
}

func (w *nodeWorkspace) Mkdir(ctx context.Context, p string) error {
	rel, err := security.CleanPath(p)
	if err != nil {
		return err
	}
	// the node's folder route succeeds when a file is in the way, so look first
	entry, err := w.Stat(ctx, rel)
	switch {
	case err == nil && entry.IsDir:
		return nil
	case err == nil:
		return fmt.Errorf("%w: %s", workspace.ErrExists, rel)
	case !errors.Is(err, workspace.ErrNotFound):
		return err
	}
	return w.mkdirAll(ctx, rel)
}

func (w *nodeWorkspace) Move(ctx context.Context, src, dst string) error {
	from, to, err := cleanTransfer(src, dst)
	if err != nil {
		return err
	}
	if _, err := w.Stat(ctx, from); err != nil {
		return err
	}
	if err := w.mustNotExist(ctx, to); err != nil {
		return err
	}
	if parent := path.Dir(to); !security.IsRoot(parent) {
		if err := w.mkdirAll(ctx, parent); err != nil {
			return err
		}
	}
	target := w.filePath(from) + "?" + url.Values{"destination": {to}}.Encode()
	return w.expectNoContent(ctx, http.MethodPost, target, nil, nil, from)
}

func (w *nodeWorkspace) Copy(ctx context.Context, src, dst string) error {
	from, to, err := cleanTransfer(src, dst)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"source": from, "destination": to})
	if err != nil {
		return err
	}
	header := http.Header{headerContentType: {contentTypeJSON}}
	return w.expectNoContent(ctx, http.MethodPost, w.prefix+"/fs/copy", bytes.NewReader(body), header, from)
}

func (w *nodeWorkspace) Delete(ctx context.Context, p string) error {
	rel, err := security.CleanPath(p)
	if err != nil {
		return err
	}
	return w.expectNoContent(ctx, http.MethodDelete, w.filePath(rel), nil, nil, rel)
}

func (w *nodeWorkspace) Search(ctx context.Context, dir string, opts workspace.SearchOptions) ([]workspace.Match, error) {
	rel, err := security.CleanDir(dir)
	if err != nil {
		return nil, err
	}
	query := url.Values{"dir": {rel}, "query": {opts.Query}, "content": {strconv.FormatBool(opts.InContent)}}
	var answer struct {
		Matches []workspace.Match `json:"matches"`
	}
	if err := w.getJSON(ctx, w.prefix+"/fs/search?"+query.Encode(), rel, &answer); err != nil {
		return nil, err
	}
	matches := answer.Matches
	if matches == nil {
		matches = []workspace.Match{}
	}
	// the node applies its own limits; a smaller limit asked for here is applied on top
	if limit := workspace.ClampSearchOptions(opts).MaxResults; len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

// filePath is the node's file route for a cleaned path, each element escaped as the file
// manager does, so a "#", "?" or "%" in a name stays part of the name
func (w *nodeWorkspace) filePath(rel string) string {
	if security.IsRoot(rel) {
		return w.prefix + "/file/"
	}
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return w.prefix + "/file/" + strings.Join(parts, "/")
}

// do sends a request to the node. A node that cannot be reached or refuses the panel's
// credentials gives a user-safe error; the details (the node's address) go to the log only.
func (w *nodeWorkspace) do(ctx context.Context, method, target string, body io.Reader, header http.Header) (*http.Response, error) {
	res, err := w.call(ctx, method, target, body, header)
	if err != nil {
		utils.CloseResponse(res)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		logging.Error.Printf("The assistant could not reach node %q: %s", w.node, err)
		return nil, &nodeError{msg: "could not reach the node that hosts this server", kind: workspace.ErrFailed}
	}
	if nodeRefusedPanel(res) {
		utils.CloseResponse(res)
		logging.Error.Printf("Node %q refused the panel's token (status %d): check that its daemon.auth.clientSecret is the node's secret in the panel and that both clocks are right", w.node, res.StatusCode)
		return nil, &nodeError{msg: "the node that hosts this server refused the panel's credentials; an administrator has to check the node", kind: workspace.ErrFailed}
	}
	return res, nil
}

func (w *nodeWorkspace) getJSON(ctx context.Context, target, rel string, into any) error {
	res, err := w.do(ctx, http.MethodGet, target, nil, nil)
	if err != nil {
		return err
	}
	defer utils.CloseResponse(res)
	if res.StatusCode != http.StatusOK {
		return assistantRouteError(target, res, rel)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, nodeAnswerLimit)).Decode(into); err != nil {
		return unreadableAnswer(ctx)
	}
	return nil
}

// expectNoContent sends a change and checks the node answered it with success
func (w *nodeWorkspace) expectNoContent(ctx context.Context, method, target string, body io.Reader, header http.Header, rel string) error {
	res, err := w.do(ctx, method, target, body, header)
	if err != nil {
		return err
	}
	defer utils.CloseResponse(res)
	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
		return assistantRouteError(target, res, rel)
	}
	return nil
}

// errNodeOutdated is a node without the assistant's routes: its 404 must not read as "the file
// is not there", or create_file would go ahead and an old node would write a wrongly named file
var errNodeOutdated = &nodeError{msg: "the node that hosts this server must be updated to let the assistant work with files", kind: workspace.ErrFailed}

// assistantRouteError reads a failed answer; on the assistant's own routes (fs/* and
// create=true), a 404 without an error code means the node does not have them
func assistantRouteError(target string, res *http.Response, rel string) error {
	err := answerError(res, rel)
	var ne *nodeError
	if res.StatusCode == http.StatusNotFound && isAssistantRoute(target) && errors.As(err, &ne) && !ne.coded {
		return errNodeOutdated
	}
	return err
}

func isAssistantRoute(target string) bool {
	return strings.Contains(target, "/fs/") || strings.Contains(target, "create=true")
}

// mkdirAll makes a folder and its parents through the file manager's folder route
func (w *nodeWorkspace) mkdirAll(ctx context.Context, rel string) error {
	return w.expectNoContent(ctx, http.MethodPut, w.filePath(rel)+"?folder", nil, nil, rel)
}

func (w *nodeWorkspace) mustNotExist(ctx context.Context, rel string) error {
	_, err := w.Stat(ctx, rel)
	switch {
	case err == nil:
		return fmt.Errorf("%w: %s", workspace.ErrExists, rel)
	case errors.Is(err, workspace.ErrNotFound):
		return nil
	default:
		return err
	}
}

// cleanTransfer checks the two paths of a move or copy, as the workspace on the node does
func cleanTransfer(src, dst string) (string, string, error) {
	from, err := security.CleanPath(src)
	if err != nil {
		return "", "", err
	}
	to, err := security.CleanPath(dst)
	if err != nil {
		return "", "", err
	}
	if from == to {
		return "", "", fmt.Errorf("%w: %s", workspace.ErrExists, to)
	}
	if strings.HasPrefix(to, from+"/") {
		return "", "", fmt.Errorf("%w: %s cannot be put inside itself", workspace.ErrInvalid, from)
	}
	return from, to, nil
}

// answerError turns a node's refusal into the workspace error it stands for. The routes made
// for the assistant send a code from daemon.FileErrorForCode with a user-safe message; the
// file manager's routes are read by their status, and their text (which may be the node's
// own words) is not passed on.
func answerError(res *http.Response, rel string) error {
	var body struct {
		Error *skypanel.Error `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(res.Body, nodeErrorLimit)).Decode(&body)
	code, message := "", ""
	if body.Error != nil {
		code, message = body.Error.Code, body.Error.Message
	}
	if code == skypanel.ErrDiskLimitExceeded.Code {
		return &nodeError{msg: skypanel.ErrDiskLimitExceeded.Message, kind: workspace.ErrNoSpace}
	}
	if kind := daemon.FileErrorForCode(code); kind != nil {
		if message == "" {
			message = kind.Error() + ": " + rel
		}
		return &nodeError{msg: message, kind: kind, coded: true}
	}
	var kind error
	switch res.StatusCode {
	case http.StatusNotFound:
		kind = workspace.ErrNotFound
	case http.StatusConflict:
		kind = workspace.ErrExists
	case http.StatusRequestEntityTooLarge:
		kind = workspace.ErrTooLarge
	case http.StatusBadRequest:
		// the file routes refuse a path that leaves the server folder with 400
		kind = workspace.ErrOutsideRoot
	case http.StatusForbidden:
		kind = workspace.ErrPermission
	default:
		kind = workspace.ErrFailed
	}
	return &nodeError{msg: kind.Error() + ": " + rel, kind: kind}
}

func isJSONAnswer(res *http.Response) bool {
	return strings.HasPrefix(res.Header.Get(headerContentType), contentTypeJSON)
}

func readTooLarge(rel string, limit int64) error {
	return fmt.Errorf("%w: %s is bigger than %d bytes", workspace.ErrTooLarge, rel, limit)
}

// unreadableAnswer reports an answer that broke off or could not be decoded
func unreadableAnswer(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return &nodeError{msg: "the node that hosts this server sent an answer that could not be read", kind: workspace.ErrFailed}
}
