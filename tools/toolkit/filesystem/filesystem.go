// Package filesystem holds the file tools of the toolkit: reading (list_directory,
// read_file, file_info, search_files), creating (create_file, create_directory, copy_path),
// changing (edit_file, write_file, move_path, rename_path) and deleting (delete_path) files
// of the server of the conversation. Every tool works through the ExecContext's workspace,
// which is bound to that server; no tool takes a server argument.
package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
)

// Register adds every file tool to r
func Register(r *toolkit.Registry) {
	for _, t := range Tools() {
		r.Register(t)
	}
}

// Tools returns every file tool
func Tools() []toolkit.Tool {
	return []toolkit.Tool{
		newListDirectory(), newReadFile(), newFileInfo(), newSearchFiles(),
		newCreateFile(), newCreateDirectory(), newCopyPath(),
		newEditFile(), newWriteFile(), newMovePath(), newRenamePath(),
		newDeletePath(),
	}
}

// pathRules is repeated in every path parameter, so the model sees it where it fills paths
const pathRules = "Relative to the server folder, with / between folders (for example config/paper.yml); no leading slash, no drive letter and no \"..\"."

// param describes one argument of a tool for the model
type param struct {
	name        string
	typ         string
	description string
	required    bool
}

// base carries what every tool states about itself
type base struct {
	name        string
	description string
	kind        toolkit.Kind
	scope       string
	params      []param
}

func (b base) Name() string        { return b.name }
func (b base) Description() string { return b.description }
func (b base) Kind() toolkit.Kind  { return b.kind }
func (b base) Scope() string       { return b.scope }

// Parameters builds the JSON schema object Gemini expects: type, properties (each with type
// and description) and required. A new map every time, so a caller cannot change the tool.
func (b base) Parameters() map[string]any {
	properties := make(map[string]any, len(b.params))
	required := []string{}
	for _, p := range b.params {
		properties[p.name] = map[string]any{"type": p.typ, "description": p.description}
		if p.required {
			required = append(required, p.name)
		}
	}
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// workspaceOf returns the workspace a call runs against
func workspaceOf(ec toolkit.ExecContext) (workspace.Workspace, error) {
	if ec.Workspace == nil {
		return nil, errors.New("the files of this server are not available")
	}
	return ec.Workspace, nil
}

// invalid builds an ErrInvalidArgs error with a reason the model can act on
func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", toolkit.ErrInvalidArgs, fmt.Sprintf(format, a...))
}

// stringArg reads a string argument; present is false when it is missing or null
func stringArg(args map[string]any, name string) (value string, present bool, err error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return "", false, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", true, invalid("%s must be a string", name)
	}
	return s, true, nil
}

// requiredString reads a string argument that must be given; it may be empty
func requiredString(args map[string]any, name string) (string, error) {
	s, present, err := stringArg(args, name)
	if err != nil {
		return "", err
	}
	if !present {
		return "", invalid("%s is required", name)
	}
	return s, nil
}

// intArg reads a whole-number argument. Models send numbers as JSON numbers (float64 after
// decoding) and sometimes as strings; both are accepted when they hold a whole number.
func intArg(args map[string]any, name string, fallback int) (int, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return fallback, nil
	}
	var f float64
	switch v := raw.(type) {
	case float64:
		f = v
	case float32:
		f = float64(v)
	case int:
		return v, nil
	case int64:
		f = float64(v)
	case json.Number:
		parsed, err := v.Float64()
		if err != nil {
			return 0, invalid("%s must be a whole number", name)
		}
		f = parsed
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, invalid("%s must be a whole number", name)
		}
		return parsed, nil
	default:
		return 0, invalid("%s must be a whole number", name)
	}
	if f != math.Trunc(f) || math.Abs(f) > math.MaxInt32 {
		return 0, invalid("%s must be a whole number", name)
	}
	return int(f), nil
}

// boolArg reads a true/false argument, also given as the strings "true" and "false"
func boolArg(args map[string]any, name string, fallback bool) (bool, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return fallback, nil
	}
	switch v := raw.(type) {
	case bool:
		return v, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			return parsed, nil
		}
	}
	return false, invalid("%s must be true or false", name)
}

// pathArg reads a required path naming a file or folder inside the server folder
func pathArg(args map[string]any, name string) (string, error) {
	raw, err := requiredString(args, name)
	if err != nil {
		return "", err
	}
	p, err := security.CleanPath(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", toolkit.ErrInvalidArgs, name, err)
	}
	return p, nil
}

// dirArg reads an optional folder path; missing, "" and "." are the server folder
func dirArg(args map[string]any, name string) (string, error) {
	raw, _, err := stringArg(args, name)
	if err != nil {
		return "", err
	}
	p, err := security.CleanDir(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", toolkit.ErrInvalidArgs, name, err)
	}
	return p, nil
}

// checkReadable refuses reading a protected path
func checkReadable(p string) error {
	if security.IsProtected(p) {
		return fmt.Errorf("%w: %s", toolkit.ErrProtected, p)
	}
	return nil
}

// checkChangeable refuses changing a protected path or a folder holding one
func checkChangeable(paths ...string) error {
	for _, p := range paths {
		if security.TouchesProtected(p) {
			return fmt.Errorf("%w: %s", toolkit.ErrProtected, p)
		}
	}
	return nil
}

// statIfExists describes p, or reports it is missing; other errors (a way out of the server
// folder, a file where a folder should be) are returned
func statIfExists(ctx context.Context, ws workspace.Workspace, p string) (workspace.Entry, bool, error) {
	e, err := ws.Stat(ctx, p)
	if errors.Is(err, workspace.ErrNotFound) {
		return workspace.Entry{}, false, nil
	}
	if err != nil {
		return workspace.Entry{}, false, err
	}
	return e, true, nil
}

// mustNotExist refuses a destination that is already there
func mustNotExist(ctx context.Context, ws workspace.Workspace, p string) error {
	_, exists, err := statIfExists(ctx, ws, p)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%w: %s", toolkit.ErrExists, p)
	}
	return nil
}

// contentArg reads the content of a file to write and checks its size
func contentArg(args map[string]any) ([]byte, error) {
	content, err := requiredString(args, "content")
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > security.Limits.MaxWriteBytes {
		return nil, fmt.Errorf("%w: the content is %s, more than the %s a file can be written with", toolkit.ErrTooLarge, formatSize(int64(len(content))), formatSize(security.Limits.MaxWriteBytes))
	}
	return []byte(content), nil
}

// inside reports whether p is below folder
func inside(p, folder string) bool {
	return strings.HasPrefix(p, folder+"/")
}

// label names a path in a sentence
func label(p string) string {
	if security.IsRoot(p) {
		return "the server folder"
	}
	return p
}

// kindLabel names an entry in a sentence: "the folder x" or "the file x"
func kindLabel(e workspace.Entry, p string) string {
	if e.IsDir {
		return "the folder " + p
	}
	return "the file " + p
}

// formatSize writes a size for people: 512 bytes, 2.1 KB, 1.0 MB
func formatSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

// quote shows a model-given text in a sentence, shortened
func quote(s string) string {
	const limit = 60
	if utf8.RuneCountInString(s) > limit {
		s = string([]rune(s)[:limit-1]) + "…"
	}
	return strconv.Quote(s)
}

// plural picks the word for n things
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
