# Kernel isolation: openat2 and unshare

Two Linux kernel features protect the node from what runs on it:

- **`openat2`** keeps every file the daemon opens for a server inside that server's folder.
- **`unshare`** (user and mount namespaces) jails servers that run directly on the node, with the `host` environment.

Servers in the `docker` environment, the default, are isolated by Docker instead: see [Container user and isolation](../backend/04-servers.md#container-user-and-isolation). How the `files` package uses `openat2` is described in [Server files and archives](19-files-compression.md).

## openat2

### Kernel check at start-up

When the daemon starts (`utils.DetermineKernelSupport()`, called from `daemon()` in `cmd/panel/run.go`), it checks that the kernel has `openat2` (added in Linux 5.6):

1. It tries the call the way the file server uses it: it opens `/` and, from there, `.` with `openat2` and `RESOLVE_BENEATH`. If that works, `openat2` is available.
2. `ENOSYS` (the kernel lacks it), `EOPNOTSUPP` or `EPERM` mean it is not available. `EPERM` is what a seccomp filter that blocks it returns (an older Docker profile, for example): with it every file access would fail the same way, so it counts as unavailable even if the kernel has it.
3. Any other error leaves the probe inconclusive and `/proc/kallsyms` is read instead, looking for the `do_sys_openat2` symbol (whatever its type, `t` or `T`). If that file cannot be opened, `openat2` counts as unavailable.
4. If `openat2` is not available, the daemon **does not start**:

```
OpenAt2 is not supported. Cowardly not starting to avoid security issues.
```

To start anyway, set `security.forceOpenat` (see below).

### What it is used for

With `openat2`, every open done by the file server uses `RESOLVE_BENEATH`, relative to the server's folder opened once. The kernel then refuses any path, `..` or symbolic link that leads outside that folder, also when a folder is swapped for a link while the request is running.

### Fallback: `security.forceOpenat`

| Key | Environment variable | Default |
|-----|----------------------|---------|
| `security.forceOpenat` | `SKYPANEL_SECURITY_FORCEOPENAT` | `false` |

When it is `true`, the daemon skips the check, logs `WARNING: OPENAT2 SUPPORT NOT ENABLED. OVERRIDE IS BEING USED.` and uses plain `openat`: each part of the path is opened one by one with `O_NOFOLLOW`. It still stays inside the folder, but **any** symbolic link on the way fails, even one that points inside the server. Use it only on kernels without `openat2`, or where a seccomp filter blocks it and cannot be changed.

There is no automatic fallback: without `openat2` and without this key, the daemon does not start.

### `internal/sys`

`sys.SyscallMode` turns a Go `os.FileMode` into the mode the system calls expect, keeping the permission bits and the setuid, setgid and sticky bits. The file server uses it for `openat2`, `openat` and `mkdirat`.

## unshare: the `host` environment jail

Servers with the `host` environment (the older names `tty` and `standard` are changed to `host` when the server is loaded) run as processes of the daemon, without a container. Unless it is turned off, each one is started inside a jail (`internal/servers/tty/tty.go`).

### How the jail is built

The daemon writes a small Bash script to a temporary folder (`/tmp/unshare-pp-*`) and runs it in new namespaces:

| Namespace | Flag |
|-----------|------|
| User | `CLONE_NEWUSER` |
| Mount | `CLONE_NEWNS` |
| Cgroup | `CLONE_NEWCGROUP` |
| IPC | `CLONE_NEWIPC` |
| Hostname (UTS) | `CLONE_NEWUTS` |

The process also gets its own file descriptor table (`CLONE_FILES`). There is **no** PID or network namespace: the server uses the node's network and its `/proc` is the node's.

User mapping: user and group `0` inside the jail are the daemon's own user and group outside (a single-ID map). The server believes it is root, but on the node it has only the daemon user's rights, and the files it creates belong to that user.

The script then builds a new root in the temporary folder and switches to it:

| Inside the jail | Comes from |
|-----------------|------------|
| `/bin`, `/lib` | Bind mount of the node's folders. |
| `/lib64` | Bind mount, on `amd64` only. |
| `/usr`, `/etc`, `/dev`, `/proc` | Recursive bind mount of the node's folders. |
| `/tmp` | A new `tmpfs` of 50 MB. |
| The server folder | Bind mount, at the same absolute path. |
| The binaries folder (`daemon.data.binaries`) and the cache folder (`daemon.data.cache`) | Bind mount, at the same absolute path. |
| Each path in the server's `mounts` | Bind mount, at the same path. |
| The target of `/etc/resolv.conf` | Bind mount, when `/etc/resolv.conf` is a symbolic link (systemd-resolved), so DNS keeps working. |

Then `pivot_root` makes that folder the root, the old root is unmounted (`umount -l`) and removed, and the start command runs from the server folder. The command and its arguments are quoted one by one, so the shell does not interpret them; a start command that is empty or contains a NUL or a line break is refused.

The rest of the node (`/home`, `/root`, `/var`, other servers' folders...) is not visible. Inside the jail, the binaries folder is put first in `PATH`; a relative binaries folder is made absolute (`/` + path).

The script needs `bash`, `mount`, `umount`, `mkdir`, `touch`, `rm` and `pivot_root` on the node (util-linux and coreutils).

### Settings

| Where | Key | Default | Effect |
|-------|-----|---------|--------|
| `config.json` | `security.disableUnshare` (`SKYPANEL_SECURITY_DISABLEUNSHARE`) | `false` | `true` turns the jail off for every `host` server on the node. |
| Server environment | `disableUnshare` | `false` | `true` turns the jail off for that server. |
| Server environment | `mounts` | empty | Extra node paths to bind-mount inside the jail. |

```json
{
  "environment": {
    "type": "host",
    "disableUnshare": false,
    "mounts": ["/opt/shared-maps"]
  }
}
```

Without the jail, the command runs directly as the daemon's user, in the server folder, with access to everything that user can reach.

### Turning off host environments: `docker.disallowHost`

`docker.disallowHost` (default `false`) turns off the environments that run on the node itself (`host`, `tty`, `standard`, `bubblewrap`). When it is on, the node does not offer them, and creating, redefining, installing or starting a server with one of them is refused with `environment "<type>" is disabled on this node (docker.disallowHost)`.

### Ubuntu 24.04 and later

Ubuntu 24.04 restricts unprivileged user namespaces through AppArmor. The jail then fails to start, and the console shows:

```
ERROR: Permission denied while starting the security jail.
Your OS (e.g. Ubuntu 24.04) might be restricting unprivileged user namespaces.
FIX 1 (Recommended): Run 'sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0'
FIX 2: Add '"security": {"disableUnshare": true}' to your config.json and restart.
```

To keep the first fix after a reboot, put it in a file under `/etc/sysctl.d/`:

```bash
echo "kernel.apparmor_restrict_unprivileged_userns=0" | sudo tee /etc/sysctl.d/99-aether-userns.conf
sudo sysctl --system
```

More on this in [Troubleshooting](../setup/troubleshooting.md).

## Summary

| Protection | Applies to | Can be turned off |
|------------|------------|-------------------|
| `openat2` + `RESOLVE_BENEATH` | Every server file the daemon opens (file manager, SFTP, archives, backups, downloads) | Only replaced by `openat` with `security.forceOpenat` |
| `unshare` jail | `host` environment servers | `security.disableUnshare` or `disableUnshare` per server |
| Docker isolation | `docker` environment servers | See [Container user and isolation](../backend/04-servers.md#container-user-and-isolation) |
