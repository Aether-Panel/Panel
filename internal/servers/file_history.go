package servers

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
)

// File history keeps the previous text of a config file each time it is saved from the panel,
// so an edit can be compared and undone. It lives next to the server folder
// (<servers>/<id>.history/), never inside it, so the owner cannot see or tamper with it there.
// Each edited file gets its own folder, named after a hash of its path, holding one file per
// version and an index.json that lists them.

// Limits for the history; variables so tests can lower them
var (
	// fileHistoryKeepPerFile is how many versions of one file are kept
	fileHistoryKeepPerFile = 20
	// fileHistoryMaxServerBytes caps the whole history of a server; the oldest versions go first
	fileHistoryMaxServerBytes int64 = 50 << 20
	// fileHistoryMaxFileBytes skips bigger files: they are not hand-edited configs
	fileHistoryMaxFileBytes int64 = 1 << 20
)

// fileHistorySniffBytes is how much of a file is checked for NUL bytes to spot binaries
const fileHistorySniffBytes = 8 << 10

const fileHistoryIndexName = "index.json"

var (
	// ErrFileHistoryPath is returned for a path that is empty or leaves the server folder
	ErrFileHistoryPath = errors.New("invalid file path")
	// ErrFileVersionNotFound is returned when the file has no version with that id
	ErrFileVersionNotFound = errors.New("file version not found")
)

// fileHistoryExtensions are the text formats game servers keep their settings in
var fileHistoryExtensions = map[string]bool{
	".properties": true, ".yml": true, ".yaml": true, ".json": true, ".json5": true,
	".toml": true, ".conf": true, ".cfg": true, ".ini": true, ".txt": true, ".xml": true,
	".sk": true, ".js": true, ".ts": true, ".lua": true, ".mcmeta": true, ".cnf": true,
	".env": true,
}

var fileVersionIDPattern = regexp.MustCompile(`^[0-9a-z]{1,16}-[0-9a-f]{6}$`)

// FileVersion is one stored version of a file, as listed to the panel
type FileVersion struct {
	ID string `json:"id"`
	// SavedAt is when this text was written to the file (its modification time), in unix ms
	SavedAt int64 `json:"savedAt"`
	Size    int64 `json:"size"`
}

type storedFileVersion struct {
	FileVersion
	// RecordedAt is when the version was kept (unix ns); the server-wide cap drops the oldest first
	RecordedAt int64  `json:"recordedAt"`
	Sum        string `json:"sum"`
}

type fileHistoryIndex struct {
	Path string `json:"path"`
	// Versions are newest first
	Versions []storedFileVersion `json:"versions"`
}

var fileHistoryLocks sync.Map

// fileHistoryLock serialises every history change of one server
func fileHistoryLock(serverID string) *sync.Mutex {
	lock, _ := fileHistoryLocks.LoadOrStore(serverID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// fileHistoryRoot is <servers>/<id>.history
func fileHistoryRoot(serverID string) (string, error) {
	safeID, err := validateServerID(serverID)
	if err != nil || strings.ContainsAny(safeID, `/\`) {
		return "", fmt.Errorf("invalid server id: %s", serverID)
	}
	return filepath.Join(config.ServersFolder.Value(), safeID+".history"), nil
}

// NormalizeHistoryPath turns a path from the file manager into the relative form the file
// server opens ("/config/a.yml" -> "config/a.yml"), so the same file always maps to the same
// history. Any ".." segment is rejected outright.
func NormalizeHistoryPath(name string) (string, error) {
	if strings.ContainsRune(name, 0) {
		return "", ErrFileHistoryPath
	}
	name = strings.ReplaceAll(name, `\`, "/")
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", ErrFileHistoryPath
		}
	}
	name = strings.TrimPrefix(filepath.Clean("/"+name), "/")
	if name == "" || name == "." || !filepath.IsLocal(name) {
		return "", ErrFileHistoryPath
	}
	return name, nil
}

// historyTracksName says whether a file name looks like a text config worth keeping
func historyTracksName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		// extensionless files (e.g. "motd") are kept only when they turn out to be text
		return true
	}
	return fileHistoryExtensions[ext]
}

// looksLikeText rejects binaries: a NUL byte early on, or, for files without an extension, bytes
// that are not UTF-8
func looksLikeText(name string, data []byte) bool {
	sniff := data
	if len(sniff) > fileHistorySniffBytes {
		sniff = sniff[:fileHistorySniffBytes]
	}
	if bytes.IndexByte(sniff, 0) >= 0 {
		return false
	}
	if filepath.Ext(name) == "" && !utf8.Valid(data) {
		return false
	}
	return true
}

func fileHistoryFolder(root, relPath string) string {
	sum := sha256.Sum256([]byte(relPath))
	return filepath.Join(root, hex.EncodeToString(sum[:16]))
}

func loadFileHistoryIndex(folder string) (*fileHistoryIndex, error) {
	data, err := os.ReadFile(filepath.Join(folder, fileHistoryIndexName))
	if err != nil {
		return nil, err
	}
	index := &fileHistoryIndex{}
	if err := json.Unmarshal(data, index); err != nil {
		return nil, fmt.Errorf("reading file history index: %w", err)
	}
	return index, nil
}

// saveFileHistoryIndex writes the index through a temp file so a crash never leaves half of it
func saveFileHistoryIndex(folder string, index *fileHistoryIndex) error {
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	tmp := filepath.Join(folder, fileHistoryIndexName+".tmp")
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing file history index: %w", err)
	}
	return os.Rename(tmp, filepath.Join(folder, fileHistoryIndexName))
}

func newFileVersionID(now time.Time) string {
	suffix := make([]byte, 3)
	_, _ = rand.Read(suffix)
	return strconv.FormatInt(now.UnixNano(), 36) + "-" + hex.EncodeToString(suffix)
}

func versionFile(folder, id string) string {
	return filepath.Join(folder, id+".v")
}

// RecordFileVersion keeps the current text of a file that is about to be overwritten. It never
// fails the save: problems are only logged. Files that are missing, too big, binary, not a
// config format or unchanged since the last version are skipped.
func (p *Server) RecordFileVersion(name string) {
	relPath, err := NormalizeHistoryPath(name)
	if err != nil {
		return
	}
	lock := fileHistoryLock(p.ID())
	lock.Lock()
	defer lock.Unlock()

	if _, err := p.recordFileVersionLocked(relPath, time.Now()); err != nil {
		logging.Error.Printf("Error keeping a version of %s for server %s: %s", relPath, p.ID(), err)
	}
}

// recordFileVersionLocked does the work of RecordFileVersion; skipped files return false, nil
func (p *Server) recordFileVersionLocked(relPath string, now time.Time) (bool, error) {
	if !historyTracksName(relPath) {
		return false, nil
	}
	fileServer := p.GetFileServer()
	if fileServer == nil {
		return false, nil
	}
	data, info, err := readVersionCandidate(fileServer, relPath)
	if data == nil || err != nil {
		return false, err
	}

	root, err := fileHistoryRoot(p.ID())
	if err != nil {
		return false, err
	}
	folder := fileHistoryFolder(root, relPath)
	index := loadOrStartIndex(folder, relPath)

	sumBytes := sha256.Sum256(data)
	sum := hex.EncodeToString(sumBytes[:])
	if len(index.Versions) > 0 && index.Versions[0].Sum == sum {
		return false, nil
	}

	version := storedFileVersion{
		FileVersion: FileVersion{ID: newFileVersionID(now), SavedAt: versionSavedAt(info, now), Size: int64(len(data))},
		RecordedAt:  now.UnixNano(),
		Sum:         sum,
	}
	if err := addFileVersion(folder, relPath, index, version, data); err != nil {
		return false, err
	}

	if err := pruneFileHistory(root); err != nil {
		logging.Error.Printf("Error trimming file history for server %s: %s", p.ID(), err)
	}
	return true, nil
}

// readVersionCandidate reads the text a save is about to replace. data is nil, with no error,
// for a file there is no point keeping: missing (a new file), not a regular file, too big, or
// not text.
func readVersionCandidate(fileServer files.FileServer, relPath string) ([]byte, os.FileInfo, error) {
	file, err := fileServer.OpenFile(relPath, os.O_RDONLY, 0)
	if err != nil {
		// a new file has nothing to keep yet
		return nil, nil, nil
	}
	defer utils.Close(file)

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > fileHistoryMaxFileBytes {
		return nil, nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, fileHistoryMaxFileBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("reading current file: %w", err)
	}
	if int64(len(data)) > fileHistoryMaxFileBytes || !looksLikeText(relPath, data) {
		return nil, nil, nil
	}
	return data, info, nil
}

// loadOrStartIndex is the file's history index, or a new empty one when it has none (or its
// index cannot be read, which is logged)
func loadOrStartIndex(folder, relPath string) *fileHistoryIndex {
	index, err := loadFileHistoryIndex(folder)
	if err == nil {
		return index
	}
	if !os.IsNotExist(err) {
		logging.Error.Printf("Starting a new file history for %q: %s", relPath, err)
	}
	return &fileHistoryIndex{Path: relPath}
}

// versionSavedAt is when the kept text was last saved: its modification time, or now when the
// file has none
func versionSavedAt(info os.FileInfo, now time.Time) int64 {
	savedAt := info.ModTime().UnixMilli()
	if info.ModTime().IsZero() || savedAt <= 0 {
		return now.UnixMilli()
	}
	return savedAt
}

// addFileVersion writes the version's text and puts it first in the index, dropping the
// versions past the per-file limit. When the index cannot be saved, the new text goes too.
func addFileVersion(folder, relPath string, index *fileHistoryIndex, version storedFileVersion, data []byte) error {
	if err := os.MkdirAll(folder, 0700); err != nil {
		return fmt.Errorf("creating file history folder: %w", err)
	}
	if err := os.WriteFile(versionFile(folder, version.ID), data, 0600); err != nil {
		return fmt.Errorf("writing file version: %w", err)
	}

	index.Path = relPath
	index.Versions = append([]storedFileVersion{version}, index.Versions...)
	var dropped []storedFileVersion
	if len(index.Versions) > fileHistoryKeepPerFile {
		dropped = index.Versions[fileHistoryKeepPerFile:]
		index.Versions = index.Versions[:fileHistoryKeepPerFile]
	}
	if err := saveFileHistoryIndex(folder, index); err != nil {
		_ = os.Remove(versionFile(folder, version.ID))
		return err
	}
	for _, v := range dropped {
		_ = os.Remove(versionFile(folder, v.ID))
	}
	return nil
}

// pruneFileHistory drops the oldest versions across every file until the server's history fits
// its cap
func pruneFileHistory(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	type entry struct {
		folder  string
		version storedFileVersion
	}
	indexes := map[string]*fileHistoryIndex{}
	var all []entry
	var total int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		folder := filepath.Join(root, e.Name())
		index, err := loadFileHistoryIndex(folder)
		if err != nil {
			continue
		}
		indexes[folder] = index
		for _, v := range index.Versions {
			all = append(all, entry{folder: folder, version: v})
			total += v.Size
		}
	}
	if total <= fileHistoryMaxServerBytes {
		return nil
	}

	sort.SliceStable(all, func(i, j int) bool {
		return all[i].version.RecordedAt < all[j].version.RecordedAt
	})
	drop := map[string]map[string]bool{}
	for _, e := range all {
		if total <= fileHistoryMaxServerBytes {
			break
		}
		if drop[e.folder] == nil {
			drop[e.folder] = map[string]bool{}
		}
		drop[e.folder][e.version.ID] = true
		total -= e.version.Size
	}

	var errs []error
	for folder, ids := range drop {
		errs = append(errs, dropFileVersions(folder, indexes[folder], ids))
	}
	return errors.Join(errs...)
}

// dropFileVersions removes the given versions from one file's history, deleting the folder
// when nothing is left
func dropFileVersions(folder string, index *fileHistoryIndex, ids map[string]bool) error {
	kept := index.Versions[:0]
	for _, v := range index.Versions {
		if !ids[v.ID] {
			kept = append(kept, v)
		}
	}
	index.Versions = kept
	if len(kept) == 0 {
		return os.RemoveAll(folder)
	}
	if err := saveFileHistoryIndex(folder, index); err != nil {
		return err
	}
	for id := range ids {
		_ = os.Remove(versionFile(folder, id))
	}
	return nil
}

// FileVersions lists the stored versions of a file, newest first; empty when there are none
func (p *Server) FileVersions(name string) ([]FileVersion, error) {
	relPath, err := NormalizeHistoryPath(name)
	if err != nil {
		return nil, err
	}
	root, err := fileHistoryRoot(p.ID())
	if err != nil {
		return nil, err
	}
	lock := fileHistoryLock(p.ID())
	lock.Lock()
	defer lock.Unlock()

	result := make([]FileVersion, 0)
	index, err := loadFileHistoryIndex(fileHistoryFolder(root, relPath))
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}
	for _, v := range index.Versions {
		result = append(result, v.FileVersion)
	}
	return result, nil
}

// FileVersionContent returns the text of one stored version
func (p *Server) FileVersionContent(name, versionID string) ([]byte, error) {
	relPath, err := NormalizeHistoryPath(name)
	if err != nil {
		return nil, err
	}
	lock := fileHistoryLock(p.ID())
	lock.Lock()
	defer lock.Unlock()
	return p.fileVersionContentLocked(relPath, versionID)
}

func (p *Server) fileVersionContentLocked(relPath, versionID string) ([]byte, error) {
	if !fileVersionIDPattern.MatchString(versionID) {
		return nil, ErrFileVersionNotFound
	}
	root, err := fileHistoryRoot(p.ID())
	if err != nil {
		return nil, err
	}
	folder := fileHistoryFolder(root, relPath)
	index, err := loadFileHistoryIndex(folder)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileVersionNotFound
		}
		return nil, err
	}
	for _, v := range index.Versions {
		if v.ID != versionID {
			continue
		}
		data, err := os.ReadFile(versionFile(folder, v.ID))
		if os.IsNotExist(err) {
			return nil, ErrFileVersionNotFound
		}
		return data, err
	}
	return nil, ErrFileVersionNotFound
}

// RestoreFileVersion writes a stored version back to the file. The text being replaced is kept
// as a version first, so the restore itself can be undone.
func (p *Server) RestoreFileVersion(name, versionID string) error {
	relPath, err := NormalizeHistoryPath(name)
	if err != nil {
		return err
	}
	lock := fileHistoryLock(p.ID())
	lock.Lock()
	defer lock.Unlock()

	// read the version before recording: recording may prune it when it is the oldest
	data, err := p.fileVersionContentLocked(relPath, versionID)
	if err != nil {
		return err
	}
	if _, err := p.recordFileVersionLocked(relPath, time.Now()); err != nil {
		return fmt.Errorf("keeping the current text before restoring: %w", err)
	}

	fileServer := p.GetFileServer()
	if fileServer == nil {
		return errors.New("server files are not available")
	}
	if dir := filepath.Dir(relPath); dir != "." {
		if err := fileServer.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating parent folder: %w", err)
		}
	}
	file, err := fileServer.OpenFile(relPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening file to restore: %w", err)
	}
	defer utils.Close(file)
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("writing restored file: %w", err)
	}
	return nil
}

// eachHistoryUnder calls fn, holding the server's history lock, with the folder and index of
// the history of relPath and of every file under it when relPath is a folder. A server with no
// history at all is fine; the errors fn returns are gathered, so one bad folder stops no other.
func (p *Server) eachHistoryUnder(relPath string, fn func(root, folder string, index *fileHistoryIndex) error) error {
	root, err := fileHistoryRoot(p.ID())
	if err != nil {
		return err
	}
	lock := fileHistoryLock(p.ID())
	lock.Lock()
	defer lock.Unlock()

	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		folder := filepath.Join(root, e.Name())
		index, err := loadFileHistoryIndex(folder)
		if err != nil || (index.Path != relPath && !strings.HasPrefix(index.Path, relPath+"/")) {
			continue
		}
		errs = append(errs, fn(root, folder, index))
	}
	return errors.Join(errs...)
}

// ForgetFileHistory removes the stored versions of a deleted file, or of every file that was
// inside a deleted folder. Left behind, they would come back as the history of a new file
// given the same name, and keep taking space the owner can no longer see or free.
func (p *Server) ForgetFileHistory(name string) error {
	relPath, err := NormalizeHistoryPath(name)
	if err != nil {
		return err
	}
	return p.eachHistoryUnder(relPath, func(_, folder string, _ *fileHistoryIndex) error {
		return os.RemoveAll(folder)
	})
}

// MoveFileHistory carries the stored versions of a moved or renamed file along with it (of every
// file inside, for a folder): left at the old name, the history would vanish for the file and
// come back for whatever is given that name next.
func (p *Server) MoveFileHistory(from, to string) error {
	fromRel, err := NormalizeHistoryPath(from)
	if err != nil {
		return err
	}
	toRel, err := NormalizeHistoryPath(to)
	if err != nil {
		return err
	}
	return p.eachHistoryUnder(fromRel, func(root, folder string, index *fileHistoryIndex) error {
		newPath := toRel + strings.TrimPrefix(index.Path, fromRel)
		newFolder := fileHistoryFolder(root, newPath)
		// a history an earlier file of that name left behind is not this file's
		if err := os.RemoveAll(newFolder); err != nil {
			return err
		}
		if err := os.Rename(folder, newFolder); err != nil {
			return err
		}
		index.Path = newPath
		return saveFileHistoryIndex(newFolder, index)
	})
}

// DeleteFileHistory removes every stored version of a server; call it when the server is deleted
func DeleteFileHistory(serverID string) error {
	root, err := fileHistoryRoot(serverID)
	if err != nil {
		return err
	}
	lock := fileHistoryLock(serverID)
	lock.Lock()
	defer lock.Unlock()
	err = os.RemoveAll(root)
	fileHistoryLocks.Delete(serverID)
	return err
}
