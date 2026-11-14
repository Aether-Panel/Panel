package filesystem

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
)

// The modify tools change or replace existing data and delete_path removes it: the user
// confirms each call before it runs (the kinds say so). They need server.files.edit.

type editFile struct{ base }

func newEditFile() editFile {
	return editFile{base{
		name:        "edit_file",
		description: "Change part of an existing text file by replacing old_text with new_text. old_text must be copied exactly from the file as read_file returned it, including spaces, indentation and line breaks, and must appear exactly expected_occurrences times (default 1); otherwise nothing is changed. Include enough surrounding lines to make old_text unique. Prefer this over write_file for small changes.",
		kind:        toolkit.KindModify,
		scope:       toolkit.ScopeFilesEdit,
		params: []param{
			{"path", "string", "File to edit. " + pathRules, true},
			{"old_text", "string", "Exact text to replace, as it appears in the file. Cannot be empty.", true},
			{"new_text", "string", "Text to put in its place. May be empty to remove old_text.", true},
			{"expected_occurrences", "integer", "How many times old_text appears in the file; every one is replaced. Default 1.", false},
		},
	}}
}

type plannedEdit struct {
	path     string
	replaced int
	content  []byte
	ws       workspace.Workspace
}

func (t editFile) Describe(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	plan, err := t.plan(ctx, ec, args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	text := fmt.Sprintf("Replace %d %s in %s", plan.replaced, plural(plan.replaced, "occurrence", "occurrences"), plan.path)
	return toolkit.Summary{Text: text, Paths: []string{plan.path}}, nil
}

func (t editFile) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	// planned again against the file as it is now: it may have changed since Describe
	plan, err := t.plan(ctx, ec, args)
	if err != nil {
		return toolkit.Result{}, err
	}
	if err := plan.ws.Write(ctx, plan.path, plan.content, true); err != nil {
		return toolkit.Result{}, err
	}
	output := map[string]any{"path": plan.path, "replaced": plan.replaced, "size": len(plan.content)}
	summary := fmt.Sprintf("Replaced %d %s in %s", plan.replaced, plural(plan.replaced, "occurrence", "occurrences"), plan.path)
	return toolkit.Result{Output: output, Summary: summary, Changed: []string{plan.path}}, nil
}

func (t editFile) plan(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (plannedEdit, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return plannedEdit{}, err
	}
	if err := checkChangeable(p); err != nil {
		return plannedEdit{}, err
	}
	oldText, err := requiredString(args, "old_text")
	if err != nil {
		return plannedEdit{}, err
	}
	if oldText == "" {
		return plannedEdit{}, invalid("old_text cannot be empty; use write_file to replace a whole file")
	}
	newText, err := requiredString(args, "new_text")
	if err != nil {
		return plannedEdit{}, err
	}
	if oldText == newText {
		return plannedEdit{}, invalid("new_text is the same as old_text, so nothing would change")
	}
	expected, err := intArg(args, "expected_occurrences", 1)
	if err != nil {
		return plannedEdit{}, err
	}
	if expected < 1 {
		return plannedEdit{}, invalid("expected_occurrences must be 1 or more")
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return plannedEdit{}, err
	}
	data, err := readText(ctx, ws, p, security.Limits.MaxEditBytes)
	if err != nil {
		return plannedEdit{}, err
	}

	content := string(data)
	found := strings.Count(content, oldText)
	if found == 0 && strings.Contains(oldText, "\n") && !strings.Contains(oldText, "\r") && strings.Contains(content, "\r\n") {
		// the model wrote the lines with \n but the file uses Windows line endings
		crlf := strings.ReplaceAll(oldText, "\n", "\r\n")
		if n := strings.Count(content, crlf); n > 0 {
			oldText, newText, found = crlf, strings.ReplaceAll(newText, "\n", "\r\n"), n
		}
	}
	switch {
	case found == 0:
		return plannedEdit{}, invalid("old_text was not found in %s; read the file again and copy the text exactly, including spaces and line breaks", p)
	case found != expected:
		return plannedEdit{}, invalid("old_text appears %d times in %s but expected_occurrences is %d; add surrounding lines to make it unique, or set expected_occurrences to %d to replace every one", found, p, expected, found)
	}
	updated := strings.ReplaceAll(content, oldText, newText)
	if int64(len(updated)) > security.Limits.MaxEditBytes {
		return plannedEdit{}, fmt.Errorf("%w: the edited file would be %s, more than %s", toolkit.ErrTooLarge, formatSize(int64(len(updated))), formatSize(security.Limits.MaxEditBytes))
	}
	return plannedEdit{path: p, replaced: found, content: []byte(updated), ws: ws}, nil
}

type writeFile struct{ base }

func newWriteFile() writeFile {
	return writeFile{base{
		name:        "write_file",
		description: "Replace the whole content of an existing text file (up to 1 MiB). The old content is lost, so read the file first and keep everything that should stay. Fails if the file does not exist: use create_file for new files. For small changes use edit_file instead.",
		kind:        toolkit.KindModify,
		scope:       toolkit.ScopeFilesEdit,
		params: []param{
			{"path", "string", "Existing file to overwrite. " + pathRules, true},
			{"content", "string", "Complete new content of the file.", true},
		},
	}}
}

type plannedWrite struct {
	path    string
	content []byte
	oldSize int64
	ws      workspace.Workspace
}

func (t writeFile) Describe(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	plan, err := t.plan(ctx, ec, args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	text := fmt.Sprintf("Overwrite %s (%s now, %s after)", plan.path, formatSize(plan.oldSize), formatSize(int64(len(plan.content))))
	return toolkit.Summary{Text: text, Paths: []string{plan.path}}, nil
}

func (t writeFile) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	plan, err := t.plan(ctx, ec, args)
	if err != nil {
		return toolkit.Result{}, err
	}
	if err := plan.ws.Write(ctx, plan.path, plan.content, true); err != nil {
		return toolkit.Result{}, err
	}
	size := int64(len(plan.content))
	output := map[string]any{"path": plan.path, "size": size}
	return toolkit.Result{Output: output, Summary: fmt.Sprintf("Overwrote %s (%s)", plan.path, formatSize(size)), Changed: []string{plan.path}}, nil
}

func (t writeFile) plan(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (plannedWrite, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return plannedWrite{}, err
	}
	if err := checkChangeable(p); err != nil {
		return plannedWrite{}, err
	}
	data, err := contentArg(args)
	if err != nil {
		return plannedWrite{}, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return plannedWrite{}, err
	}
	e, exists, err := statIfExists(ctx, ws, p)
	if err != nil {
		return plannedWrite{}, err
	}
	if !exists {
		return plannedWrite{}, fmt.Errorf("%w: %s (use create_file for new files)", toolkit.ErrNotFound, p)
	}
	if e.IsDir {
		return plannedWrite{}, invalid("%s is a folder, not a file", p)
	}
	return plannedWrite{path: p, content: data, oldSize: e.Size, ws: ws}, nil
}

type movePath struct{ base }

func newMovePath() movePath {
	return movePath{base{
		name:        "move_path",
		description: "Move a file or folder to another path inside this server, which may also rename it. Missing parent folders of the destination are created. Fails if the destination already exists. To only change the name in the same folder, rename_path is simpler.",
		kind:        toolkit.KindModify,
		scope:       toolkit.ScopeFilesEdit,
		params: []param{
			{"source", "string", "File or folder to move. " + pathRules, true},
			{"destination", "string", "New full path, which must not exist yet. " + pathRules, true},
		},
	}}
}

func (t movePath) Describe(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	src, dst, ws, err := sourceAndDestination(ec, args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	e, err := checkTransfer(ctx, ws, src, dst)
	if err != nil {
		return toolkit.Summary{}, err
	}
	text := fmt.Sprintf("Move %s to %s", kindLabel(e, src), dst)
	return toolkit.Summary{Text: text, Paths: []string{src, dst}}, nil
}

func (t movePath) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	src, dst, ws, err := sourceAndDestination(ec, args)
	if err != nil {
		return toolkit.Result{}, err
	}
	if err := ws.Move(ctx, src, dst); err != nil {
		return toolkit.Result{}, err
	}
	output := map[string]any{"source": src, "destination": dst}
	return toolkit.Result{Output: output, Summary: fmt.Sprintf("Moved %s to %s", src, dst), Changed: []string{src, dst}}, nil
}

// checkTransfer checks the source of a move exists and the destination does not
func checkTransfer(ctx context.Context, ws workspace.Workspace, src, dst string) (workspace.Entry, error) {
	e, err := ws.Stat(ctx, src)
	if err != nil {
		return workspace.Entry{}, err
	}
	if err := mustNotExist(ctx, ws, dst); err != nil {
		return workspace.Entry{}, err
	}
	return e, nil
}

type renamePath struct{ base }

func newRenamePath() renamePath {
	return renamePath{base{
		name:        "rename_path",
		description: "Rename a file or folder, keeping it in the same folder. new_name is only the new name (for example paper-old.yml), never a path. Fails if something with that name already exists there. To move to another folder use move_path.",
		kind:        toolkit.KindModify,
		scope:       toolkit.ScopeFilesEdit,
		params: []param{
			{"path", "string", "File or folder to rename. " + pathRules, true},
			{"new_name", "string", "New name only, without any folder and without / or \\.", true},
		},
	}}
}

func (t renamePath) Describe(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	src, dst, ws, err := t.args(ec, args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	e, err := checkTransfer(ctx, ws, src, dst)
	if err != nil {
		return toolkit.Summary{}, err
	}
	text := fmt.Sprintf("Rename %s to %s", kindLabel(e, src), path.Base(dst))
	return toolkit.Summary{Text: text, Paths: []string{src, dst}}, nil
}

func (t renamePath) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	src, dst, ws, err := t.args(ec, args)
	if err != nil {
		return toolkit.Result{}, err
	}
	if err := ws.Move(ctx, src, dst); err != nil {
		return toolkit.Result{}, err
	}
	output := map[string]any{"path": src, "newPath": dst}
	return toolkit.Result{Output: output, Summary: fmt.Sprintf("Renamed %s to %s", src, path.Base(dst)), Changed: []string{src, dst}}, nil
}

func (t renamePath) args(ec toolkit.ExecContext, args map[string]any) (string, string, workspace.Workspace, error) {
	src, err := pathArg(args, "path")
	if err != nil {
		return "", "", nil, err
	}
	raw, err := requiredString(args, "new_name")
	if err != nil {
		return "", "", nil, err
	}
	name := strings.TrimSpace(raw)
	switch {
	case name == "":
		return "", "", nil, invalid("new_name is empty")
	case strings.ContainsAny(name, `/\`):
		return "", "", nil, invalid("new_name must be a name only, without / or \\; use move_path to move to another folder")
	case name == "." || name == "..":
		return "", "", nil, invalid("new_name cannot be %q", name)
	case strings.ContainsRune(name, 0):
		return "", "", nil, invalid("new_name contains a NUL byte")
	case len(name) > 255:
		return "", "", nil, invalid("new_name is longer than 255 bytes")
	}
	dst, err := security.CleanPath(path.Join(path.Dir(src), name))
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: new_name: %w", toolkit.ErrInvalidArgs, err)
	}
	if dst == src {
		return "", "", nil, invalid("%s already has that name", src)
	}
	if err := checkChangeable(src, dst); err != nil {
		return "", "", nil, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return "", "", nil, err
	}
	return src, dst, ws, nil
}

type deletePath struct{ base }

func newDeletePath() deletePath {
	return deletePath{base{
		name:        "delete_path",
		description: "Delete a file, or a folder with everything in it. This cannot be undone, so make sure of the path first and consider copy_path for a backup. The server folder itself cannot be deleted.",
		kind:        toolkit.KindDestructive,
		scope:       toolkit.ScopeFilesEdit,
		params: []param{
			{"path", "string", "File or folder to delete. " + pathRules, true},
		},
	}}
}

func (t deletePath) Describe(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	p, ws, err := t.args(ec, args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	e, err := ws.Stat(ctx, p)
	if err != nil {
		return toolkit.Summary{}, err
	}
	text := "Delete the file " + p
	if e.IsDir {
		text = "Delete the folder " + p + " and everything in it"
	}
	return toolkit.Summary{Text: text, Paths: []string{p}}, nil
}

func (t deletePath) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	p, ws, err := t.args(ec, args)
	if err != nil {
		return toolkit.Result{}, err
	}
	if err := ws.Delete(ctx, p); err != nil {
		return toolkit.Result{}, err
	}
	output := map[string]any{"path": p, "deleted": true}
	return toolkit.Result{Output: output, Summary: "Deleted " + p, Changed: []string{p}}, nil
}

func (t deletePath) args(ec toolkit.ExecContext, args map[string]any) (string, workspace.Workspace, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		if raw, present, _ := stringArg(args, "path"); present && namesRoot(raw) {
			return "", nil, invalid("the server folder itself cannot be deleted; name a file or folder inside it")
		}
		return "", nil, err
	}
	if err := checkChangeable(p); err != nil {
		return "", nil, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return "", nil, err
	}
	return p, ws, nil
}

// namesRoot reports whether a path argument means the server folder itself
func namesRoot(raw string) bool {
	dir, err := security.CleanDir(raw)
	return err == nil && security.IsRoot(dir)
}
