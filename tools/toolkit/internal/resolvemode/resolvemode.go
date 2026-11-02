// Package resolvemode lets the toolkit tests run under both ways the file server resolves
// paths: openat2 with RESOLVE_BENEATH, which the node uses, and one folder at a time without
// following links, which is the default in tests and the node's fallback when
// security.forceOpenat is set. Only tests import it.
package resolvemode

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"golang.org/x/sys/unix"
)

// Each runs fn once per resolve mode, as subtests. The openat2 run is skipped where the
// kernel or a seccomp filter refuses the syscall.
func Each(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	t.Run("per-folder", func(t *testing.T) {
		if utils.UseOpenat2() {
			t.Skip("openat2 is in use")
		}
		fn(t)
	})
	t.Run("openat2", func(t *testing.T) {
		if err := probe(); err != nil {
			t.Skipf("openat2 is not available here: %v", err)
		}
		enable(t)
		fn(t)
	})
}

// enable switches the file server to openat2 until the test ends
func enable(t *testing.T) {
	t.Helper()
	if err := config.SecurityForceOpenat.Set(false, false); err != nil {
		t.Fatal(err)
	}
	utils.DetermineKernelSupport()
	if !utils.UseOpenat2() {
		t.Fatal("openat2 was not enabled")
	}
	t.Cleanup(func() {
		// forcing the override is the only way back to the per-folder mode from outside
		_ = config.SecurityForceOpenat.Set(true, false)
		utils.DetermineKernelSupport()
		_ = config.SecurityForceOpenat.Set(false, false)
	})
}

// probe tries openat2 the way the file server uses it, so enable never reaches the panic
// in DetermineKernelSupport
func probe() error {
	root, err := unix.Open("/", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(root) }()
	fd, err := unix.Openat2(root, ".", &unix.OpenHow{Flags: unix.O_PATH | unix.O_CLOEXEC, Resolve: unix.RESOLVE_BENEATH})
	if err != nil {
		return err
	}
	return unix.Close(fd)
}
