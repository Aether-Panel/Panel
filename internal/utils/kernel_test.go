package utils

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

// withOpenat2Probe answers the probe with err and points the fallback at a symbol list with
// the given content, for the length of the test
func withOpenat2Probe(t *testing.T, err error, symbols string) {
	previousProbe, previousFile, previousUse := openat2Probe, kallsymsFile, useOpenat2
	t.Cleanup(func() { openat2Probe, kallsymsFile, useOpenat2 = previousProbe, previousFile, previousUse })

	openat2Probe = func() error { return err }
	kallsymsFile = filepath.Join(t.TempDir(), "kallsyms")
	assert.NoError(t, os.WriteFile(kallsymsFile, []byte(symbols), 0644))
}

const symbolsWithOpenat2 = "ffffffff81234560 T do_sys_open\nffffffff81234570 T do_sys_openat2\n"

func TestTestOpenat2_TheSyscallDecides(t *testing.T) {
	withOpenat2Probe(t, nil, "")
	assert.True(t, testOpenat2(), "it worked, whatever the symbol list says")

	for _, err := range []error{unix.ENOSYS, unix.EPERM, unix.EOPNOTSUPP} {
		withOpenat2Probe(t, err, symbolsWithOpenat2)
		assert.False(t, testOpenat2(), "%v: the kernel or a filter refuses it", err)
	}
}

func TestTestOpenat2_FallsBackToTheSymbols(t *testing.T) {
	other := errors.New("something else")

	// a global symbol ("T") counts as well as a local one ("t")
	withOpenat2Probe(t, other, symbolsWithOpenat2)
	assert.True(t, testOpenat2())

	withOpenat2Probe(t, other, "0000000000000000 t do_sys_openat2\n")
	assert.True(t, testOpenat2())

	withOpenat2Probe(t, other, "ffffffff81234560 T do_sys_open\nffffffff81234590 t do_sys_openat2_helper\n")
	assert.False(t, testOpenat2())

	withOpenat2Probe(t, other, "")
	kallsymsFile = filepath.Join(t.TempDir(), "missing")
	assert.False(t, testOpenat2(), "no list, no proof of support")
}

func TestProbeOpenat2_ThisKernel(t *testing.T) {
	err := probeOpenat2()
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM) {
		t.Skipf("openat2 is not available here: %v", err)
	}
	assert.NoError(t, err)
}
