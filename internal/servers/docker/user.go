package docker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/moby/moby/api/types/container"
	"golang.org/x/sys/unix"
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

// limitProcesses caps the container's processes, threads included, at docker.pidsLimit, unless
// the template sets its own cap. Wings uses 512, but Java servers start threads per CPU core,
// so a modded server on a many-core host can need hundreds; the default leaves room for that
// and still stops a fork bomb.
func limitProcesses(hostConfig *container.HostConfig, limit int) {
	if hostConfig.PidsLimit != nil || limit <= 0 {
		return
	}
	pids := int64(limit)
	hostConfig.PidsLimit = &pids
}

// ownServerFiles gives every file in the server folder to the user the game runs as, before
// each start, as Wings does. Files an install (run as root) or the panel left behind would
// otherwise be read-only for the game, and servers created while games ran as root move over
// on their first start. The walk goes folder by folder through open descriptors, never by path:
// symlinks are changed themselves, never followed, and a folder the game swaps for a symlink
// while the walk runs (to / for example) is refused instead of walked into.
func ownServerFiles(root string, uid, gid int) error {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		return nil // nothing installed yet: Docker creates the folder
	}
	if err != nil {
		return &os.PathError{Op: "open", Path: root, Err: err}
	}
	if err = ownIfNeeded(fd, "", uid, gid); err != nil {
		_ = unix.Close(fd)
		return err
	}
	return ownFolder(fd, root, uid, gid)
}

// ownFolder owns everything in the open folder dirFd and closes it
func ownFolder(dirFd int, path string, uid, gid int) error {
	folder := os.NewFile(uintptr(dirFd), path)
	defer folder.Close()
	entries, err := folder.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if err = ownIfNeeded(dirFd, name, uid, gid); err != nil {
			if errors.Is(err, unix.ENOENT) {
				continue // removed while walking
			}
			return err
		}
		if !entry.IsDir() {
			continue
		}
		child, err := unix.Openat(dirFd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
			continue // removed, or swapped for a symlink or a file, since it was listed
		}
		if err != nil {
			return &os.PathError{Op: "open", Path: filepath.Join(path, name), Err: err}
		}
		if err = ownFolder(child, filepath.Join(path, name), uid, gid); err != nil {
			return err
		}
	}
	return nil
}

// ownIfNeeded changes the owner of name in dirFd (dirFd itself when name is empty), unless it
// already has it; a symlink is changed itself
func ownIfNeeded(dirFd int, name string, uid, gid int) error {
	flags := unix.AT_SYMLINK_NOFOLLOW
	if name == "" {
		flags |= unix.AT_EMPTY_PATH
	}
	var st unix.Stat_t
	if err := unix.Fstatat(dirFd, name, &st, flags); err != nil {
		return err
	}
	if int(st.Uid) == uid && int(st.Gid) == gid {
		return nil
	}
	return unix.Fchownat(dirFd, name, uid, gid, flags)
}
