# Security: unshare & Kernel Isolation

Aether Panel uses Linux `unshare` syscall for server process isolation. This document explains the implementation, kernel requirements, and configuration.

---

## Overview

`unshare` creates new Linux namespaces, isolating processes from the host system:

| Namespace | Constant | Isolates |
|-----------|----------|----------|
| User | `CLONE_NEWUSER` | UID/GID mappings |
| Mount | `CLONE_NEWNS` | Filesystem mounts |
| PID | `CLONE_NEWPID` | Process IDs |
| Network | `CLONE_NEWNET` | Network interfaces |
| IPC | `CLONE_NEWIPC` | IPC resources |
| UTS | `CLONE_NEWUTS` | Hostname/domain |
| Cgroup | `CLONE_NEWCGROUP` | Cgroup hierarchy |

---

## Implementation (`internal/sys/syscall.go`)

```go
func Unshare(flags int) error {
    return unix.Unshare(flags)
}

// Default flags for TTY environment
const DefaultUnshareFlags = unix.CLONE_NEWUSER |
    unix.CLONE_NEWNS |
    unix.CLONE_NEWPID |
    unix.CLONE_NEWNET |
    unix.CLONE_NEWIPC |
    unix.CLONE_NEWUTS |
    unix.CLONE_NEWCGROUP
```

**Execution Flow:**
1. Panel forks child process
2. Child calls `unshare(flags)`
3. Sets up UID/GID maps in new user namespace
4. Mounts proc, sysfs in new mount namespace
5. Executes server process

---

## Kernel Requirements

| Feature | Kernel Version | Config |
|---------|----------------|--------|
| `unshare(CLONE_NEWUSER)` | 3.8+ | `CONFIG_USER_NS=y` |
| `unshare(CLONE_NEWPID)` | 3.8+ | `CONFIG_PID_NS=y` |
| `unshare(CLONE_NEWNET)` | 3.8+ | `CONFIG_NET_NS=y` |
| `unshare(CLONE_NEWNS)` | 2.4.19+ | `CONFIG_NAMESPACES=y` |
| `openat2` / `RESOLVE_BENEATH` | 5.6+ | `CONFIG_OPENAT2=y` |

**Ubuntu 24.04+** restricts unprivileged user namespaces by default:
```bash
# Check current setting
sysctl kernel.apparmor_restrict_unprivileged_userns

# Enable (required for Panel)
sysctl -w kernel.apparmor_restrict_unprivileged_userns=0

# Persistent
echo "kernel.apparmor_restrict_unprivileged_userns=0" >> /etc/sysctl.d/99-panel.conf
sysctl --system
```

---

## Configuration

```json
{
  "security": {
    "disableUnshare": false
  }
}
```

**Environment Variable:**
```bash
SKYPANEL_SECURITY_DISABLEUNSHARE=true
```

**Per-Server Override (in server definition):**
```json
{
  "environment": {
    "type": "tty",
    "disableUnshare": true
  }
}
```

---

## UID/GID Mapping

In the new user namespace:

```go
// Write to /proc/<pid>/uid_map
echo "0 1000 1" > /proc/$PID/uid_map  // Container root (0) -> Host UID 1000
echo "1 1001 1" > /proc/$PID/uid_map  // Container user (1) -> Host UID 1001

// Write to /proc/<pid>/gid_map
echo "0 1000 1" > /proc/$PID/gid_map
echo "1 1001 1" > /proc/$PID/gid_map
```

**Requirements:**
- `/proc/sys/kernel/max_user_namespaces` > 0
- `/proc/sys/kernel/max_map_count` sufficient for mappings

---

## Mount Namespace

Isolated filesystem view:

```go
// In new mount namespace
unix.Mount("proc", "/proc", "proc", 0, "")
unix.Mount("sysfs", "/sys", "sysfs", 0, "")
unix.Mount("tmpfs", "/tmp", "tmpfs", 0, "")
unix.Mount("devpts", "/dev/pts", "devpts", 0, "")
```

**Server Directory Mount:**
```go
// Bind mount server root
unix.Mount(serverRoot, "/server", "bind", unix.MS_BIND|unix.MS_REC, "")
```

---

## PID Namespace

```go
// In new PID namespace, process sees itself as PID 1
// Children see sequential PIDs
// Host sees actual PID
```

**Benefits:**
- Process isolation
- Clean process tree
- `kill -9 1` inside container only kills container

---

## Network Namespace

```go
// Creates new network stack
// Own loopback, routing table, firewall
// No access to host network by default
```

**Docker Integration:** When using Docker environment, network namespace handled by Docker (not `unshare`).

---

## Disabling unshare

### Global (config.json)
```json
{
  "security": {
    "disableUnshare": true
  }
}
```

### Per-Server (server definition)
```json
{
  "environment": {
    "type": "tty",
    "disableUnshare": true
  }
}
```

### Environment Variable
```bash
SKYPANEL_SECURITY_DISABLEUNSHARE=true
```

---

## Troubleshooting

| Error | Cause | Solution |
|-------|-------|----------|
| `operation not permitted` | User namespaces disabled | Enable `kernel.apparmor_restrict_unprivileged_userns=0` |
| `invalid argument` | Kernel too old | Upgrade kernel (3.8+ required) |
| `permission denied` | AppArmor/SELinux | Adjust policy or disable |
| `no space left` | User namespace limit | Increase `max_user_namespaces` |
| `cannot allocate memory` | Map count exceeded | Increase `max_map_count` |

### Check Kernel Support

```bash
# Check namespaces
lsns

# Check user namespace limit
cat /proc/sys/kernel/max_user_namespaces

# Check kernel version
uname -r

# Check AppArmor
aa-status | grep userns
```

### Ubuntu 24.04+ Fix

```bash
# Temporary
sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0

# Permanent
echo "kernel.apparmor_restrict_unprivileged_userns=0" | \
    sudo tee /etc/sysctl.d/99-panel-userns.conf
sudo sysctl --system
```

### Docker Alternative

If `unshare` cannot be enabled, use Docker environment:
```json
{
  "environment": {
    "type": "docker",
    "image": "eclipse-temurin:17"
  }
}
```

---

## Security Implications

| Aspect | With unshare | Without unshare |
|--------|--------------|-----------------|
| Filesystem | Isolated (bind mounts) | Full host access |
| Processes | PID namespace isolated | Visible to host |
| Network | Isolated stack | Host network access |
| Users | UID mapping | Real host UID |
| Breakout risk | Low | High |

---

## AppArmor Profile

For production, use custom AppArmor profile:

```apparmor
# /etc/apparmor.d/usr.bin.skypanel
#include <tunables/global>

/usr/bin/SkyPanel {
  #include <abstractions/base>
  
  capability setuid,
  capability setgid,
  capability sys_admin,
  capability sys_resource,
  capability dac_override,
  
  # unshare
  capability sys_admin,
  
  # Mount namespace
  mount options=(rw, bind),
  
  # Network
  network inet tcp,
  network inet udp,
  
  # File access
  /var/lib/SkyPanel/** rw,
  /tmp/** rw,
}
```

Apply:
```bash
sudo apparmor_parser -r /etc/apparmor.d/usr.bin.skypanel
```

---

## SELinux

```bash
# Allow unshare
setsebool -P allow_unshare 1

# Allow mount namespace
setsebool -P container_manage_cgroup 1
```

---

## Verification

```go
// Check if unshare works
func TestUnshare() error {
    // Try in child process
    cmd := exec.Command("unshare", "--user", "--pid", "--mount", "true")
    return cmd.Run()
}
```

---

## Performance Impact

| Metric | Overhead |
|--------|----------|
| Process start | +5-10ms |
| Memory | +1-2MB per namespace |
| CPU | Negligible |
| File I/O | Minimal (bind mounts) |

---

## Best Practices

1. **Always enable** on production (security)
2. **Test on target kernel** before deployment
3. **Monitor** `dmesg` for AppArmor/SELinux denials
4. **Use Docker** if kernel support insufficient
5. **Keep kernel updated** for security patches