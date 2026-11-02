package utils

import (
	"bufio"
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"golang.org/x/sys/unix"
	"os"
	"strings"
)

// openat2Probe tries the syscall the way the file server uses it; a variable so tests can
// stand in for kernels without it
var openat2Probe = probeOpenat2

// kallsymsFile is where the kernel lists its symbols, the fallback when the probe is inconclusive
var kallsymsFile = "/proc/kallsyms"

func testOpenat2() bool {
	err := openat2Probe()
	switch {
	case err == nil:
		useOpenat2 = true
	case errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EPERM), errors.Is(err, unix.EOPNOTSUPP):
		// ENOSYS: the kernel does not have it. EPERM: a seccomp filter (an older Docker
		// profile, say) blocks it, so every file access through it would fail the same way.
		useOpenat2 = false
	default:
		logging.Info.Printf("Could not try openat2 to validate kernel support, falling back to %s\n%s", kallsymsFile, err.Error())
		useOpenat2 = kallsymsHasOpenat2(kallsymsFile)
	}
	return useOpenat2
}

// probeOpenat2 opens the root folder with openat2, kept beneath it as the file server does
func probeOpenat2() error {
	root, err := unix.Open("/", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(root) }()

	fd, err := unix.Openat2(root, ".", &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_BENEATH,
	})
	if err != nil {
		return err
	}
	return unix.Close(fd)
}

// kallsymsHasOpenat2 looks for the kernel function behind openat2 in a symbol list, whatever
// its symbol type ("t" or "T") and however the columns are spaced
func kallsymsHasOpenat2(file string) bool {
	f, err := os.Open(file)
	if err != nil {
		logging.Info.Printf("Could not open %s to validate kernel support\n%s", file, err.Error())
		return false
	}
	defer Close(f)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 && fields[2] == "do_sys_openat2" {
			return true
		}
	}
	return false
}
