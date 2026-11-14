package filesystem

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
)

// The read tools only look at files, so they run without confirmation and need only
// server.files.view.

type listDirectory struct{ base }

func newListDirectory() listDirectory {
	return listDirectory{base{
		name:        "list_directory",
		description: "List the files and folders inside a folder of this server, folders first. Each entry has name, path, isDir, size (bytes) and modTime. Use it to find your way before reading or changing files. Leave path empty or use \".\" for the server folder itself.",
		kind:        toolkit.KindRead,
		scope:       toolkit.ScopeFilesView,
		params: []param{
			{"path", "string", "Folder to list. " + pathRules + " Empty or \".\" is the server folder.", false},
		},
	}}
}

func (t listDirectory) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	dir, err := t.args(args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	return toolkit.Summary{Text: "List " + label(dir), Paths: []string{dir}}, nil
}

func (t listDirectory) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	dir, err := t.args(args)
	if err != nil {
		return toolkit.Result{}, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return toolkit.Result{}, err
	}
	all, err := ws.List(ctx, dir)
	if err != nil {
		return toolkit.Result{}, err
	}
	entries := make([]workspace.Entry, 0, len(all))
	for _, e := range all {
		if !security.IsProtected(e.Path) {
			entries = append(entries, e)
		}
	}
	total := len(entries)
	truncated := total > security.Limits.MaxListEntries
	if truncated {
		entries = entries[:security.Limits.MaxListEntries]
	}
	output := map[string]any{"path": dir, "entries": entries, "total": total, "truncated": truncated}
	summary := fmt.Sprintf("Listed %s (%d %s)", label(dir), total, plural(total, "entry", "entries"))
	return toolkit.Result{Output: output, Summary: summary}, nil
}

func (t listDirectory) args(args map[string]any) (string, error) {
	dir, err := dirArg(args, "path")
	if err != nil {
		return "", err
	}
	return dir, checkReadable(dir)
}

type readFile struct{ base }

func newReadFile() readFile {
	return readFile{base{
		name:        "read_file",
		description: "Read a text file of this server and return its content exactly as stored, with its size and line count. Files up to 1 MiB can be read; binary files (jars, worlds, images) are refused. For long files pass offset and limit to read a range of lines, and check truncated to know whether more lines follow. Always read a file before editing it.",
		kind:        toolkit.KindRead,
		scope:       toolkit.ScopeFilesView,
		params: []param{
			{"path", "string", "File to read. " + pathRules, true},
			{"offset", "integer", "First line to return, counting from 1. Default 1.", false},
			{"limit", "integer", "Most lines to return. Default: every line to the end of the file.", false},
		},
	}}
}

type readArgs struct {
	path   string
	offset int
	limit  int
}

func (t readFile) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	a, err := t.args(args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	return toolkit.Summary{Text: "Read " + a.path, Paths: []string{a.path}}, nil
}

func (t readFile) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	a, err := t.args(args)
	if err != nil {
		return toolkit.Result{}, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return toolkit.Result{}, err
	}
	data, err := readText(ctx, ws, a.path, security.Limits.MaxReadBytes)
	if err != nil {
		return toolkit.Result{}, err
	}

	// SplitAfter keeps every line ending, so the returned text is byte for byte what the
	// file holds and can be used as old_text for edit_file
	lines := strings.SplitAfter(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := len(lines)
	if total > 0 && a.offset > total {
		return toolkit.Result{}, invalid("offset %d is past the end of %s, which has %d lines", a.offset, a.path, total)
	}
	start, end := 0, 0
	content := ""
	if total > 0 {
		start, end = a.offset, total
		if a.limit > 0 && a.offset-1+a.limit < total {
			end = a.offset - 1 + a.limit
		}
		content = strings.Join(lines[start-1:end], "")
	}
	output := map[string]any{
		"path":       a.path,
		"content":    content,
		"size":       len(data),
		"totalLines": total,
		"startLine":  start,
		"endLine":    end,
		"truncated":  end < total,
	}
	summary := fmt.Sprintf("Read %s (%s)", a.path, formatSize(int64(len(data))))
	return toolkit.Result{Output: output, Summary: summary}, nil
}

func (t readFile) args(args map[string]any) (readArgs, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return readArgs{}, err
	}
	if err := checkReadable(p); err != nil {
		return readArgs{}, err
	}
	offset, err := intArg(args, "offset", 1)
	if err != nil {
		return readArgs{}, err
	}
	if offset < 1 {
		return readArgs{}, invalid("offset counts from 1")
	}
	limit, err := intArg(args, "limit", 0)
	if err != nil {
		return readArgs{}, err
	}
	if limit < 0 {
		return readArgs{}, invalid("limit cannot be negative")
	}
	return readArgs{path: p, offset: offset, limit: limit}, nil
}

// readText reads a text file within limit, with errors that tell the model what to do next
func readText(ctx context.Context, ws workspace.Workspace, p string, limit int64) ([]byte, error) {
	data, err := ws.Read(ctx, p, limit)
	if errors.Is(err, workspace.ErrTooLarge) {
		return nil, fmt.Errorf("%w (the limit is %s; the assistant cannot open bigger files, and search_files skips them too)", err, formatSize(limit))
	}
	if errors.Is(err, workspace.ErrIsDir) {
		return nil, fmt.Errorf("%w (use list_directory for folders)", err)
	}
	if err != nil {
		return nil, err
	}
	if security.LooksBinary(data) {
		return nil, invalid("%s is a binary file; only text files can be read or edited", p)
	}
	return data, nil
}

type fileInfo struct{ base }

func newFileInfo() fileInfo {
	return fileInfo{base{
		name:        "file_info",
		description: "Tell whether a file or folder exists and give its type (file or folder), size in bytes and last modification time, without reading it.",
		kind:        toolkit.KindRead,
		scope:       toolkit.ScopeFilesView,
		params: []param{
			{"path", "string", "File or folder to describe. " + pathRules + " \".\" is the server folder.", true},
		},
	}}
}

func (t fileInfo) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	p, err := t.args(args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	return toolkit.Summary{Text: "Check " + label(p), Paths: []string{p}}, nil
}

func (t fileInfo) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	p, err := t.args(args)
	if err != nil {
		return toolkit.Result{}, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return toolkit.Result{}, err
	}
	e, err := ws.Stat(ctx, p)
	if err != nil {
		return toolkit.Result{}, err
	}
	kind := "file"
	if e.IsDir {
		kind = "folder"
	}
	output := map[string]any{"name": e.Name, "path": e.Path, "type": kind, "isDir": e.IsDir, "size": e.Size, "modTime": e.ModTime}
	return toolkit.Result{Output: output, Summary: "Checked " + label(p)}, nil
}

func (t fileInfo) args(args map[string]any) (string, error) {
	raw, err := requiredString(args, "path")
	if err != nil {
		return "", err
	}
	p, err := security.CleanDir(raw)
	if err != nil {
		return "", fmt.Errorf("%w: path: %w", toolkit.ErrInvalidArgs, err)
	}
	return p, checkReadable(p)
}

type searchFiles struct{ base }

func newSearchFiles() searchFiles {
	return searchFiles{base{
		name:        "search_files",
		description: "Search this server's files below a folder, breadth first. By default it matches file and folder names containing query (case-insensitive). With in_content true it searches inside text files instead and returns each matching line with its line number. Limits: 8 folders deep, 5000 entries looked at, 200 matches, files up to 1 MiB; binary files are skipped. truncated tells whether more matches may exist.",
		kind:        toolkit.KindRead,
		scope:       toolkit.ScopeFilesView,
		params: []param{
			{"query", "string", "Text to look for, case-insensitive. At most 200 characters.", true},
			{"path", "string", "Folder to search in. " + pathRules + " Empty or \".\" searches the whole server folder.", false},
			{"in_content", "boolean", "true searches inside text files; false (default) matches file and folder names.", false},
			{"max_results", "integer", "Most matches to return, up to 200 (the default).", false},
		},
	}}
}

type searchArgs struct {
	dir     string
	options workspace.SearchOptions
}

func (t searchFiles) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	a, err := t.args(args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	what := "file names"
	if a.options.InContent {
		what = "file contents"
	}
	text := fmt.Sprintf("Search %s for %s in %s", what, quote(a.options.Query), label(a.dir))
	return toolkit.Summary{Text: text, Paths: []string{a.dir}}, nil
}

func (t searchFiles) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	a, err := t.args(args)
	if err != nil {
		return toolkit.Result{}, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return toolkit.Result{}, err
	}
	found, err := ws.Search(ctx, a.dir, a.options)
	if err != nil {
		return toolkit.Result{}, err
	}
	limit := workspace.ClampSearchOptions(a.options).MaxResults
	matches := make([]workspace.Match, 0, len(found))
	for _, m := range found {
		if !security.IsProtected(m.Path) && len(matches) < limit {
			matches = append(matches, m)
		}
	}
	output := map[string]any{
		"query":     a.options.Query,
		"path":      a.dir,
		"inContent": a.options.InContent,
		"matches":   matches,
		"count":     len(matches),
		"truncated": len(found) >= limit,
	}
	summary := fmt.Sprintf("Searched for %s (%d %s)", quote(a.options.Query), len(matches), plural(len(matches), "match", "matches"))
	return toolkit.Result{Output: output, Summary: summary}, nil
}

func (t searchFiles) args(args map[string]any) (searchArgs, error) {
	query, err := requiredString(args, "query")
	if err != nil {
		return searchArgs{}, err
	}
	if strings.TrimSpace(query) == "" {
		return searchArgs{}, invalid("query is empty")
	}
	if utf8.RuneCountInString(query) > security.Limits.MaxQueryLength {
		return searchArgs{}, invalid("query is longer than %d characters", security.Limits.MaxQueryLength)
	}
	dir, err := dirArg(args, "path")
	if err != nil {
		return searchArgs{}, err
	}
	if err := checkReadable(dir); err != nil {
		return searchArgs{}, err
	}
	inContent, err := boolArg(args, "in_content", false)
	if err != nil {
		return searchArgs{}, err
	}
	maxResults, err := intArg(args, "max_results", 0)
	if err != nil {
		return searchArgs{}, err
	}
	if maxResults < 0 {
		return searchArgs{}, invalid("max_results cannot be negative")
	}
	options := workspace.SearchOptions{Query: query, InContent: inContent, MaxResults: maxResults}
	return searchArgs{dir: dir, options: options}, nil
}
