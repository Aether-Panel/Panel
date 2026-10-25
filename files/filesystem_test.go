package files

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestGlobLooksInsideTheFolderOfThePattern(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"server.jar", "spigot-1.21.jar", "pack/resources/x.lua", "pack/README.md"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(f)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, f), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := NewFileServer(root, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	cases := map[string][]string{
		"spigot-*.jar": {"spigot-1.21.jar"},
		"*.jar":        {"server.jar", "spigot-1.21.jar"},
		// a folder in the pattern used to make Glob read the wrong folder and match nothing
		"pack/resources": {"pack/resources"},
		"pack/*.md":      {"pack/README.md"},
	}
	for pattern, want := range cases {
		got, err := fs.Glob(pattern)
		if err != nil {
			t.Fatalf("%s: %v", pattern, err)
		}
		sort.Strings(got)
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", pattern, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v, want %v", pattern, got, want)
			}
		}
	}
}
