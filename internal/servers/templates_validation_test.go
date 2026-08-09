package servers

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

var skipTemplateNames = map[string]bool{
	"data.json":      true,
	"spec.json":      true,
	"templates.json": true,
}

func templateRepoRoot(t *testing.T) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// this file lives at internal/servers/templates_validation_test.go
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// TestTemplates_AllRegisteredAndStructured validates every published template
// under Templates/ parses as a Server definition, that every operation `type`
// referenced by install/uninstall/run.pre/run.post is registered in
// commandMapping (an unregistered type is silently skipped at runtime, which is
// a common cause of "install does nothing"), and that docker environments
// declare portBindings (the format those are parsed in is covered separately by
// the docker package tests).
func TestTemplates_AllRegisteredAndStructured(t *testing.T) {
	templatesDir := filepath.Join(templateRepoRoot(t), "Templates")
	_, err := os.Stat(templatesDir)
	if os.IsNotExist(err) {
		// Templates are fetched at runtime and the Templates/ directory is
		// gitignored, so it is absent in a fresh CI checkout. Skip gracefully.
		t.Skipf("Templates directory not present at %s (gitignored); skipping", templatesDir)
	}
	if err != nil {
		t.Fatalf("Templates directory not found: %v", err)
	}

	var paths []string
	err = filepath.WalkDir(templatesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") || skipTemplateNames[d.Name()] {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		t.Fatalf("walking templates: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no template json files found")
	}

	if len(commandMapping) == 0 {
		t.Fatal("commandMapping is empty; the operation factories did not register")
	}

	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".json")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("reading %s: %v", p, err)
			}

			var srv skypanel.Server
			if err := json.Unmarshal(raw, &srv); err != nil {
				t.Fatalf("parsing %s as Server: %v", p, err)
			}

			if srv.Type.Type == "" {
				t.Errorf("template %s: missing top-level \"type\"", p)
			}

			opTypes := map[string]bool{}
			for _, c := range srv.Installation {
				opTypes[c.Type] = true
			}
			for _, c := range srv.Uninstallation {
				opTypes[c.Type] = true
			}
			for _, c := range srv.Execution.PreExecution {
				opTypes[c.Type] = true
			}
			for _, c := range srv.Execution.PostExecution {
				opTypes[c.Type] = true
			}

			for opType := range opTypes {
				if opType == "" {
					continue
				}
				if _, ok := commandMapping[opType]; !ok {
					t.Errorf("template %s references unregistered operation type %q", name, opType)
				}
			}

			for _, se := range srv.SupportedEnvironments {
				if se.Type == "docker" {
					pb, ok := se.Metadata["portBindings"]
					if !ok || pb == nil {
						// Some docker templates (e.g. outbound Discord bots) do not
						// expose a listening port, so portBindings is optional.
						continue
					}
					arr, ok := pb.([]interface{})
					if !ok {
						t.Errorf("template %s: portBindings is not a list", name)
						continue
					}
					if len(arr) == 0 {
						t.Errorf("template %s: portBindings is empty for docker environment", name)
						continue
					}
					for _, p := range arr {
						s, _ := p.(string)
						if s == "" || !strings.Contains(s, ":") {
							t.Errorf("template %s: malformed portBinding %q", name, s)
						}
					}
				}
			}
		})
	}
}
