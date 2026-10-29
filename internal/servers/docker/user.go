package docker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/moby/moby/api/types/container"
)

// defaultServerUser is the uid:gid game servers run as when the panel runs as root. It is the
// one Pterodactyl uses, so it is not the host's first login user (1000), who often has sudo.
const defaultServerUser = "988:988"

// Capabilities taken from a server container, as Wings does. A container that runs as root (an
// install, or a template that asks for root) keeps the ones package installs and chown need.
var (
	serverCapDrop = []string{"setpcap", "mknod", "audit_write", "net_raw", "dac_override",
		"fowner", "fsetid", "net_bind_service", "sys_chroot", "setfcap"}
	rootCapDrop = []string{"setpcap", "mknod", "audit_write", "net_raw", "sys_chroot"}
)

// containerUser is the uid:gid the container runs as. A template that names a user keeps it.
// A panel running as root runs installs as root (their scripts install packages) and the game
// as docker.serverUser; a panel running as another user runs both as itself, as before.
func (d *Docker) containerUser(installing bool) (uid, gid int) {
	if d.Config.User != "" {
		return parseUser(d.Config.User)
	}
	if os.Getuid() != 0 {
		return os.Getuid(), os.Getgid()
	}
	if installing {
		return 0, 0
	}
	return serverUser()
}

// serverUser reads docker.serverUser; "0:0" keeps game servers running as root.
func serverUser() (uid, gid int) {
	value := config.DockerServerUser.Value()
	uid, gid, err := parseNumericUser(value)
	if err != nil {
		logging.Error.Printf("docker.serverUser %q is not a numeric uid:gid, using %s: %s", value, defaultServerUser, err)
		uid, gid, _ = parseNumericUser(defaultServerUser)
	}
	return uid, gid
}

// parseUser reads a template's user. A name other than root cannot be resolved outside the
// image, so it reports -1 and the panel does not change the owner of the files.
func parseUser(user string) (uid, gid int) {
	if name, _, _ := strings.Cut(user, ":"); name == "root" {
		return 0, 0
	}
	uid, gid, err := parseNumericUser(user)
	if err != nil {
		return -1, -1
	}
	return uid, gid
}

// parseNumericUser reads "uid:gid" or "uid" (the group is then the same number).
func parseNumericUser(user string) (uid, gid int, err error) {
	u, g, hasGroup := strings.Cut(strings.TrimSpace(user), ":")
	if uid, err = strconv.Atoi(u); err != nil || uid < 0 {
		return 0, 0, fmt.Errorf("invalid uid %q", u)
	}
	if !hasGroup {
		return uid, uid, nil
	}
	if gid, err = strconv.Atoi(g); err != nil || gid < 0 {
		return 0, 0, fmt.Errorf("invalid gid %q", g)
	}
	return uid, gid, nil
}

// harden takes the capabilities a game server does not need and stops its processes from
// gaining privileges (setuid binaries), like Wings. The template's own lists are kept.
func harden(hostConfig *container.HostConfig, root bool) {
	drop := serverCapDrop
	if root {
		drop = rootCapDrop
	}
	capDrop := slices.Clone(hostConfig.CapDrop)
	for _, c := range drop {
		if !slices.ContainsFunc(capDrop, func(have string) bool { return strings.EqualFold(have, c) }) {
			capDrop = append(capDrop, c)
		}
	}
	hostConfig.CapDrop = capDrop

	securityOpt := slices.Clone(hostConfig.SecurityOpt)
	if !slices.ContainsFunc(securityOpt, func(o string) bool { return strings.HasPrefix(o, "no-new-privileges") }) {
		securityOpt = append(securityOpt, "no-new-privileges")
	}
	hostConfig.SecurityOpt = securityOpt
}

// ownServerFiles gives every file in the server folder to the user the game runs as, before
// each start, as Wings does. Files an install (run as root) or the panel left behind would
// otherwise be read-only for the game, and servers created while games ran as root move over
// on their first start. Symlinks are changed themselves, never followed, so a link that points
// outside the folder cannot hand anything else to the server.
func ownServerFiles(root string, uid, gid int) error {
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil // removed while walking
			}
			return err
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == uid && int(st.Gid) == gid {
			return nil
		}
		return os.Lchown(path, uid, gid)
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil // nothing installed yet: Docker creates the folder
	}
	return err
}
