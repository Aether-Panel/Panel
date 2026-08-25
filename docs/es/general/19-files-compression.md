# Files & Compression Utilities

Aether Panel provides robust file system operations with path traversal protection and multi-format archive support.

---

## FileServer (`internal/files/fileserver.go`)

Secure file server with path traversal prevention using `openat2` (Linux) or `openat` fallback.

### Features

- **Path Traversal Prevention**: Uses `RESOLVE_BENEATH` flag (Linux 5.6+) or manual validation
- **Symlink Protection**: `O_NOFOLLOW` prevents symlink attacks
- **Root Jail**: All operations relative to configured root
- **Permission Preservation**: Maintains file modes, ownership

### Interface

```go
type FileServer interface {
    Open(name string) (File, error)
    Stat(name string) (FileInfo, error)
    ReadDir(name string) ([]FileInfo, error)
    Mkdir(name string, perm FileMode) error
    Remove(name string) error
    RemoveAll(name string) error
    Create(name string) (File, error)
    OpenFile(name string, flag int, perm FileMode) (File, error)
}
```

### Security Flags (Linux)

```go
// openat2 with RESOLVE_BENEATH (kernel 5.6+)
flags := unix.O_RDONLY | unix.O_NOFOLLOW
resolveFlags := unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS
fd, err := unix.Openat2(dirFD, name, &unix.OpenHow{Flags: uint64(flags), Resolve: resolveFlags})
```

**Fallback:** If `openat2` unavailable, uses `openat` with manual path validation:
```go
func safePath(root, path string) (string, error) {
    absRoot, _ := filepath.Abs(root)
    absPath, _ := filepath.Abs(filepath.Join(root, path))
    if !strings.HasPrefix(absPath, absRoot) {
        return "", errors.New("path traversal attempt")
    }
    return absPath, nil
}
```

---

## MergedFS (`internal/files/mergedfs.go`)

Merges multiple `fs.FS` sources with priority order.

```go
type MergedFS []fs.FS

func (m MergedFS) Open(name string) (fs.File, error) {
    for _, fs := range m {
        if f, err := fs.Open(name); err == nil {
            return f, nil
        }
    }
    return nil, fs.ErrNotExist
}
```

**Use Case:** Template system merges:
1. Custom templates (user-uploaded)
2. Embedded default templates
3. Remote repository cache

---

## Compression (`internal/files/compression.go`)

Multi-format archive support via `mholt/archives`.

### Supported Formats

| Format | Extensions | Compression | Use Case |
|--------|------------|-------------|----------|
| ZIP | `.zip` | Deflate | Cross-platform, server backups |
| TAR | `.tar` | None | Unix backups, Docker layers |
| GZIP | `.tar.gz`, `.tgz` | gzip | Compressed backups |
| BZIP2 | `.tar.bz2`, `.tbz2` | bzip2 | High compression |
| XZ | `.tar.xz`, `.txz` | LZMA | Maximum compression |

### Compress()

```go
func Compress(format, source, destination string) error {
    // Creates archive from source (file or directory)
    // Preserves permissions, symlinks, timestamps
}
```

**Example:**
```go
Compress("zip", "/server/world", "/backups/world_20240115.zip")
```

### Extract()

```go
func Extract(format, source, destination string, options ...ExtractOption) error {
    // Extracts archive to destination
    // skipRoot: skip top-level directory in archive
    // overwrite: overwrite existing files
    // preservePermissions: keep original permissions
}
```

**Options:**
```go
type ExtractOption func(*ExtractConfig)

func SkipRoot(skip bool) ExtractOption
func Overwrite(overwrite bool) ExtractOption
func PreservePermissions(preserve bool) ExtractOption
```

**Example:**
```go
Extract("zip", "backup.zip", "/server", 
    SkipRoot(true),           // Skip top-level "backup/" folder
    Overwrite(true),          // Overwrite existing files
    PreservePermissions(true) // Keep file modes
)
```

---

## Archive Operations in Operations Engine

### `archive` Operation

```json
{
  "type": "archive",
  "source": "world",
  "destination": "backups/world_{{timestamp}}.zip",
  "format": "zip"
}
```

### `extract` Operation

```json
{
  "type": "extract",
  "source": "backup.zip",
  "destination": "{{rootDir}}",
  "format": "zip",
  "skipRoot": true,
  "overwrite": true
}
```

---

## File Operations

### CopyFile

```go
func CopyFile(src, dst string) error {
    // Copies file with permissions
    // Creates parent directories
}
```

### WriteFile

```go
func WriteFile(path, content string, perm os.FileMode) error {
    // Writes string to file
    // Creates parent directories
}
```

### ReadFile

```go
func ReadFile(path string) (string, error) {
    // Reads file as string (UTF-8)
}
```

### SafeWriteFile

```go
func SafeWriteFile(path, content string, perm os.FileMode) error {
    // Atomic write: writes to temp file, then renames
    // Prevents partial writes on crash
}
```

---

## Path Utilities

### SafeJoin

```go
func SafeJoin(root, elem ...string) (string, error) {
    // Joins path elements
    // Validates result is within root
    // Prevents path traversal
}
```

### IsWithinRoot

```go
func IsWithinRoot(root, path string) bool {
    // Validates path is within root directory
    // Handles symlinks, relative paths
}
```

---

## Security

### Path Traversal Prevention

All file operations validate paths:

```go
func validatePath(root, requested string) (string, error) {
    absRoot, _ := filepath.Abs(root)
    absRequested, _ := filepath.Abs(filepath.Join(root, requested))
    
    if !strings.HasPrefix(absRequested, absRoot) {
        return "", errors.New("path traversal detected")
    }
    return absRequested, nil
}
```

### Symlink Handling

- `O_NOFOLLOW` on open
- `lstat` instead of `stat` for metadata
- Symlinks preserved in archives but not followed on extract

### Permission Handling

- Default file mode: `0644`
- Default dir mode: `0755`
- Preserves executable bit on scripts
- Ownership preserved in archives

---

## Testing

```go
// Test path traversal
func TestPathTraversal(t *testing.T) {
    fs := NewFileServer("/safe/root")
    _, err := fs.Open("../../etc/passwd")
    assert.Error(t, err) // Should fail
}

// Test archive round-trip
func TestArchiveRoundTrip(t *testing.T) {
    os.MkdirTemp("", "test")
    // Create files
    Compress("zip", src, dest)
    Extract("zip", dest, dst)
    // Verify files match
}
```

---

## API Endpoints

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| POST | `/api/servers/:id/archive/*filename` | `server.files.edit` | Create archive |
| POST | `/api/servers/:id/extract/*filename` | `server.files.edit` | Extract archive |

**Archive Request:**
```json
POST /api/servers/abc123/archive/backup.zip
Content-Type: application/json
["world/", "plugins/", "server.properties"]
```

**Extract Query:**
```
POST /api/servers/abc123/extract/backup.zip?destination=world_restored
```

---

## Performance

| Operation | Optimization |
|-----------|--------------|
| Large archives | Streaming (no full memory load) |
| Many small files | Batched operations |
| Permissions | Preserved without extra syscalls |
| Symlinks | Skipped on extract by default |

---

## Error Codes

| Error | Code | Description |
|-------|------|-------------|
| `ErrPathTraversal` | `E_PATH_TRAVERSAL` | Path escapes root |
| `ErrNotFound` | `E_NOT_FOUND` | File/directory not found |
| `ErrPermission` | `E_PERMISSION` | Insufficient permissions |
| `ErrInvalidFormat` | `E_INVALID_FORMAT` | Unsupported archive format |
| `ErrCorrupted` | `E_CORRUPTED` | Archive corrupted |