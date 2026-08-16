package utils

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/sys"
	"golang.org/x/sys/unix"
	"os"
	"strings"
)

func testOpenat2() bool {
	f, err := os.Open("/proc/kallsyms")
	switch {
	case errors.Is(err, os.ErrNotExist):
		logging.Info.Printf("Could not open /proc/kallsyms to validate kernel support, falling back to temp file test\n%s", err.Error())

		var testPath string
		testPath, err = os.MkdirTemp("", "skypanel-openat2-test-*")
		if err != nil {
			panic(fmt.Errorf("failed to validate kernel support with test file\n%s", err.Error()))
		}
		defer func(tar string) {
			_ = os.Remove(tar)
		}(testPath)

		var testFile *os.File
		testFile, err = os.Open(testPath)
		if err != nil {
			panic(fmt.Errorf("failed to validate kernel support with test file\n%s", err.Error()))
		}
		defer Close(testFile)

		// we have a file now, let's see if we can... read it with openat2
		var fd int
		fd, err = unix.Openat2(int(testFile.Fd()), "validate", &unix.OpenHow{
			Flags: uint64(os.O_CREATE),
			Mode:  uint64(sys.SyscallMode(0644)),
		})
		switch {
		case err == nil:
			_ = unix.Close(fd)
			useOpenat2 = true
		case errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS):
			useOpenat2 = false
		default:
			panic(fmt.Errorf("failed to validate kernel support with test file\n%s", err.Error()))
		}
	case err == nil:
		defer Close(f)
		reader := bufio.NewScanner(f)
		var line string
		for reader.Scan() {
			line = reader.Text()
			if strings.Contains(line, " t do_sys_openat2") {
				useOpenat2 = true
				break
			}
		}
	default:
		panic(fmt.Errorf("could not open /proc/kallsyms to validate kernel support\n%s", err.Error()))
	}
	return useOpenat2
}
