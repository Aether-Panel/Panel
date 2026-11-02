package files

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"golang.org/x/sys/unix"
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

func TestOpenFileDoesNotHangOnAPipe(t *testing.T) {
	root := t.TempDir()
	if err := unix.Mkfifo(filepath.Join(root, "pipe"), 0644); err != nil {
		t.Skip("no named pipes here:", err)
	}
	fs, err := NewFileServer(root, -1, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	done := make(chan error, 1)
	go func() {
		info, err := fs.Stat("pipe")
		if err == nil && info.Mode().IsRegular() {
			err = os.ErrInvalid
		}
		done <- err
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("opening a named pipe blocked")
	}
}

func TestSymlinkIsMadeInsideTheRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "server")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// what a game can leave behind: a link to a folder of the host
	if err := os.Symlink(outside, filepath.Join(root, "e")); err != nil {
		t.Fatal(err)
	}
	fs, err := NewFileServer(root, -1, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	if err := fs.Symlink("x.json", "e/planted.json"); err == nil {
		t.Fatal("a link was made through a symlinked folder")
	}
	if _, err := os.Lstat(filepath.Join(outside, "planted.json")); !os.IsNotExist(err) {
		t.Fatal("the link landed outside the server")
	}
	if err := fs.Symlink("world.dat", "inside"); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(root, "inside")); err != nil || target != "world.dat" {
		t.Fatalf("link inside the root: %q %v", target, err)
	}
}
