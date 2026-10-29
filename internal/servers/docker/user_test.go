package docker

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNumericUser(t *testing.T) {
	for _, tc := range []struct {
		in       string
		uid, gid int
		ok       bool
	}{
		{"988:988", 988, 988, true},
		{" 1000:100 ", 1000, 100, true},
		{"1000", 1000, 1000, true},
		{"0:0", 0, 0, true},
		{"container", 0, 0, false},
		{"1000:staff", 0, 0, false},
		{"-1:5", 0, 0, false},
		{"", 0, 0, false},
	} {
		uid, gid, err := parseNumericUser(tc.in)
		if !tc.ok {
			assert.Error(t, err, tc.in)
			continue
		}
		require.NoError(t, err, tc.in)
		assert.Equal(t, []int{tc.uid, tc.gid}, []int{uid, gid}, tc.in)
	}
}

func TestContainerUser_TemplateUserWins(t *testing.T) {
	d := &Docker{}
	d.Config.User = "1000:1000"
	for _, installing := range []bool{true, false} {
		uid, gid := d.containerUser(installing)
		assert.Equal(t, []int{1000, 1000}, []int{uid, gid}, "the template's user is kept for installs and starts")
	}

	d.Config.User = "root"
	uid, _ := d.containerUser(false)
	assert.Equal(t, 0, uid, "root by name is root")

	d.Config.User = "steam"
	uid, gid := d.containerUser(false)
	assert.Equal(t, []int{-1, -1}, []int{uid, gid}, "a name the panel cannot resolve leaves the files alone")
}

func TestContainerUser_WithoutTemplateUser(t *testing.T) {
	d := &Docker{}
	uid, gid := d.containerUser(false)
	installUID, _ := d.containerUser(true)
	if os.Getuid() != 0 {
		assert.Equal(t, []int{os.Getuid(), os.Getgid()}, []int{uid, gid}, "a panel that is not root runs servers as itself")
		assert.Equal(t, os.Getuid(), installUID)
		return
	}
	assert.Equal(t, []int{988, 988}, []int{uid, gid}, "a root panel runs games unprivileged")
	assert.Equal(t, 0, installUID, "installs stay root to install packages")
}

func TestHarden(t *testing.T) {
	hc := &container.HostConfig{CapDrop: []string{"NET_RAW", "sys_time"}, SecurityOpt: []string{"label=disable"}}
	harden(hc, false)
	assert.Contains(t, hc.CapDrop, "dac_override")
	assert.Contains(t, hc.CapDrop, "sys_time", "the template's own drops are kept")
	assert.Equal(t, 1, countFold(hc.CapDrop, "net_raw"), "a capability the template already drops is not repeated")
	assert.Equal(t, []string{"label=disable", "no-new-privileges"}, hc.SecurityOpt)

	root := &container.HostConfig{}
	harden(root, true)
	assert.NotContains(t, root.CapDrop, "dac_override", "root keeps what package installs need")
	assert.NotContains(t, root.CapDrop, "fowner")
	assert.Contains(t, root.CapDrop, "net_raw")
	assert.Equal(t, []string{"no-new-privileges"}, root.SecurityOpt)

	template := &container.HostConfig{SecurityOpt: []string{"no-new-privileges:true"}}
	harden(template, false)
	assert.Equal(t, []string{"no-new-privileges:true"}, template.SecurityOpt)
}

func countFold(list []string, want string) int {
	n := 0
	for _, s := range list {
		if strings.EqualFold(s, want) {
			n++
		}
	}
	return n
}

func TestOwnServerFiles_MissingFolder(t *testing.T) {
	assert.NoError(t, ownServerFiles(filepath.Join(t.TempDir(), "never-installed"), os.Getuid(), os.Getgid()))
}

func TestOwnServerFiles_ChangesOwnerWithoutFollowingLinks(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("changing a file's owner needs root")
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o644))

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "world", "region"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "world", "region", "r.0.0.mca"), []byte("x"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	require.NoError(t, ownServerFiles(root, 988, 988))

	for _, p := range []string{root, filepath.Join(root, "world"), filepath.Join(root, "world", "region", "r.0.0.mca"), filepath.Join(root, "link")} {
		info, err := os.Lstat(p)
		require.NoError(t, err)
		st := info.Sys().(*syscall.Stat_t)
		assert.Equal(t, []uint32{988, 988}, []uint32{st.Uid, st.Gid}, p)
	}
	info, err := os.Stat(outside)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), info.Sys().(*syscall.Stat_t).Uid, "the file a link points to outside the folder keeps its owner")
}
