package filesystem

import (
	"context"
	"fmt"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
)

// The create tools only add new files and folders and refuse to replace anything, so they
// run without confirmation; they need server.files.edit.

type createFile struct{ base }

func newCreateFile() createFile {
	return createFile{base{
		name:        "create_file",
		description: "Create a new text file with the given content (up to 1 MiB). Missing parent folders are created. Fails if something already exists at path: use edit_file to change part of an existing file or write_file to replace all of it.",
		kind:        toolkit.KindCreate,
		scope:       toolkit.ScopeFilesEdit,
		params: []param{
			{"path", "string", "File to create. " + pathRules, true},
			{"content", "string", "Full text of the new file. May be empty.", true},
		},
	}}
}

func (t createFile) Describe(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	p, data, ws, err := t.prepare(ec, args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	if err := mustNotExist(ctx, ws, p); err != nil {
		return toolkit.Summary{}, fmt.Errorf("%w (use edit_file or write_file to change an existing file)", err)
	}
	text := fmt.Sprintf("Create the file %s (%s)", p, formatSize(int64(len(data))))
	return toolkit.Summary{Text: text, Paths: []string{p}}, nil
}

func (t createFile) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	p, data, ws, err := t.prepare(ec, args)
	if err != nil {
		return toolkit.Result{}, err
	}
	// the workspace creates with O_EXCL, so an existing file is never replaced
	if err := ws.Write(ctx, p, data, false); err != nil {
		return toolkit.Result{}, err
	}
	size := int64(len(data))
	output := map[string]any{"path": p, "size": size, "created": true}
	return toolkit.Result{Output: output, Summary: fmt.Sprintf("Created %s (%s)", p, formatSize(size)), Changed: []string{p}}, nil
}

func (t createFile) prepare(ec toolkit.ExecContext, args map[string]any) (string, []byte, workspace.Workspace, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return "", nil, nil, err
	}
	if err := checkChangeable(p); err != nil {
		return "", nil, nil, err
	}
	data, err := contentArg(args)
	if err != nil {
		return "", nil, nil, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return "", nil, nil, err
	}
	return p, data, ws, nil
}

type createDirectory struct{ base }

func newCreateDirectory() createDirectory {
	return createDirectory{base{
		name:        "create_directory",
		description: "Create a folder, with any missing parent folders. Succeeds without changes if the folder already exists; fails if a file is in the way.",
		kind:        toolkit.KindCreate,
		scope:       toolkit.ScopeFilesEdit,
		params: []param{
			{"path", "string", "Folder to create. " + pathRules, true},
		},
	}}
}

func (t createDirectory) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	p, err := t.args(args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	return toolkit.Summary{Text: "Create the folder " + p, Paths: []string{p}}, nil
}

func (t createDirectory) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	p, err := t.args(args)
	if err != nil {
		return toolkit.Result{}, err
	}
	ws, err := workspaceOf(ec)
	if err != nil {
		return toolkit.Result{}, err
	}
	e, exists, err := statIfExists(ctx, ws, p)
	if err != nil {
		return toolkit.Result{}, err
	}
	if exists && e.IsDir {
		output := map[string]any{"path": p, "created": false, "existed": true}
		return toolkit.Result{Output: output, Summary: "The folder " + p + " already exists"}, nil
	}
	if exists {
		return toolkit.Result{}, fmt.Errorf("%w: %s is a file", toolkit.ErrExists, p)
	}
	if err := ws.Mkdir(ctx, p); err != nil {
		return toolkit.Result{}, err
	}
	output := map[string]any{"path": p, "created": true}
	return toolkit.Result{Output: output, Summary: "Created the folder " + p, Changed: []string{p}}, nil
}

func (t createDirectory) args(args map[string]any) (string, error) {
	p, err := pathArg(args, "path")
	if err != nil {
		return "", err
	}
	return p, checkChangeable(p)
}

type copyPath struct{ base }

func newCopyPath() copyPath {
	return copyPath{base{
		name:        "copy_path",
		description: "Copy a file, or a folder with everything in it, to a new path (for example a backup before a risky change). Fails if the destination already exists. Symbolic links inside a copied folder are skipped.",
		kind:        toolkit.KindCreate,
		scope:       toolkit.ScopeFilesEdit,
		params: []param{
			{"source", "string", "File or folder to copy. " + pathRules, true},
			{"destination", "string", "Path of the copy, which must not exist yet. " + pathRules, true},
		},
	}}
}

func (t copyPath) Describe(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	src, dst, ws, err := sourceAndDestination(ec, args)
	if err != nil {
		return toolkit.Summary{}, err
	}
	e, err := ws.Stat(ctx, src)
	if err != nil {
		return toolkit.Summary{}, err
	}
	if err := mustNotExist(ctx, ws, dst); err != nil {
		return toolkit.Summary{}, err
	}
	text := fmt.Sprintf("Copy %s to %s", kindLabel(e, src), dst)
	return toolkit.Summary{Text: text, Paths: []string{src, dst}}, nil
}

func (t copyPath) Run(ctx context.Context, ec toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	src, dst, ws, err := sourceAndDestination(ec, args)
	if err != nil {
		return toolkit.Result{}, err
	}
	if err := ws.Copy(ctx, src, dst); err != nil {
		return toolkit.Result{}, err
	}
	output := map[string]any{"source": src, "destination": dst}
	return toolkit.Result{Output: output, Summary: fmt.Sprintf("Copied %s to %s", src, dst), Changed: []string{dst}}, nil
}

// sourceAndDestination reads the source and destination of a copy or move. Both are checked
// as changed paths: a copy of a folder holding a protected path would expose it.
func sourceAndDestination(ec toolkit.ExecContext, args map[string]any) (string, string, workspace.Workspace, error) {
	src, err := pathArg(args, "source")
	if err != nil {
		return "", "", nil, err
	}
	dst, err := pathArg(args, "destination")
	if err != nil {
		return "", "", nil, err
	}
	if src == dst {
		return "", "", nil, invalid("source and destination are the same path")
	}
	if inside(dst, src) {
		return "", "", nil, invalid("%s cannot be put inside itself", src)
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
