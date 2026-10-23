package servers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
)

// File handling for modpack installs. Everything is written through the server's file
// server, which refuses paths that leave the server folder (also through symlinks), and every
// path that comes from a pack is checked before it gets there.

var errUnsafePackPath = errors.New("the pack contains a file path outside the server folder")

// modpackReplacedDirs are folders a pack brings its own version of
var modpackReplacedDirs = map[string]bool{
	"mods":           true,
	"config":         true,
	"defaultconfigs": true,
	"kubejs":         true,
	"libraries":      true,
	"scripts":        true,
	".fabric":        true,
}

// modpackReplacedFiles are the launchers and start scripts of the previous loader
var modpackReplacedFiles = map[string]bool{
	"server.jar":                        true,
	"fabric-server-launch.jar":          true,
	"fabric-server-launcher.properties": true,
	"run.sh":                            true,
	"run.bat":                           true,
	"user_jvm_args.txt":                 true,
	"forge-installer.jar":               true,
	"neoforge-installer.jar":            true,
	"installer.jar":                     true,
	"installer.jar.log":                 true,
	"fabric-installer.jar":              true,
	"modrinth.index.json":               true,
	"variables.txt":                     true,
}

var modpackReplacedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^.*installer.*\.jar(\.log)?$`),
	regexp.MustCompile(`^(neo)?forge-.*\.jar$`),
	regexp.MustCompile(`^minecraft_server.*\.jar$`),
}

// modpackKeptFiles survive the install even when the pack ships its own copy: they hold the
// owner's settings and player lists
var modpackKeptFiles = []string{
	"server.properties",
	"whitelist.json",
	"ops.json",
	"banned-players.json",
	"banned-ips.json",
	"eula.txt",
}

// keptProperties are taken from the owner's server.properties when a fresh world uses the
// pack's: the address, ports and access rules belong to the server, not the pack
var keptProperties = []string{
	"server-ip", "server-port", "query.port", "enable-query", "rcon.port", "enable-rcon",
	"rcon.password", "motd", "white-list", "enforce-whitelist", "online-mode", "max-players",
}

const keptFileLimit = 1 << 20

func isReplacedByModpack(name string, dir bool) bool {
	if dir {
		return modpackReplacedDirs[name]
	}
	if modpackReplacedFiles[name] {
		return true
	}
	for _, pattern := range modpackReplacedPatterns {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}

// worldFolders are the folders of the world named in server.properties ("world" by default)
func worldFolders(properties []byte) []string {
	name := "world"
	if value, ok := readProperty(properties, "level-name"); ok && strings.TrimSpace(value) != "" {
		name = strings.TrimSpace(value)
	}
	if !filepath.IsLocal(name) || strings.ContainsAny(name, `/\`) {
		return nil
	}
	return []string{name, name + "_nether", name + "_the_end"}
}

// cleanForModpack removes what a pack replaces from the top of the server folder; worlds,
// settings and player lists stay unless freshWorld asks for a new world
func cleanForModpack(fsys files.FileServer, root string, freshWorld bool) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	worlds := map[string]bool{}
	if freshWorld {
		properties, _ := readSmallFile(fsys, "server.properties")
		for _, name := range worldFolders(properties) {
			worlds[name] = true
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		isLink := entry.Type()&fs.ModeSymlink != 0
		replaced := worlds[name] || isReplacedByModpack(name, entry.IsDir())
		if isLink {
			// a link named like a replaced folder or file goes as well
			replaced = worlds[name] || isReplacedByModpack(name, true) || isReplacedByModpack(name, false)
		}
		if !replaced {
			continue
		}
		switch {
		case isLink:
			// the link itself goes, never what it points to
			err = os.Remove(filepath.Join(root, name))
		case entry.IsDir():
			err = fsys.RemoveAll(name)
		default:
			err = fsys.Remove(name)
		}
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", name, err)
		}
	}
	return nil
}

func readSmallFile(fsys files.FileServer, name string) ([]byte, error) {
	f, err := fsys.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer utils.Close(f)
	return io.ReadAll(io.LimitReader(f, keptFileLimit))
}

// readKeptFiles saves the owner's settings and lists before the pack is unpacked
func readKeptFiles(fsys files.FileServer) (map[string][]byte, error) {
	kept := make(map[string][]byte)
	for _, name := range modpackKeptFiles {
		data, err := readSmallFile(fsys, name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		kept[name] = data
	}
	return kept, nil
}

// restoreKeptFiles puts the owner's files back over whatever the pack shipped. With a fresh
// world the pack's server.properties is used (it may set a world type or seed), keeping the
// owner's address, ports and access rules.
func restoreKeptFiles(fsys files.FileServer, kept map[string][]byte, freshWorld bool) error {
	for name, data := range kept {
		if name == "server.properties" && freshWorld {
			if packProperties, err := readSmallFile(fsys, name); err == nil {
				data = mergeProperties(packProperties, data, keptProperties)
			}
		}
		if _, err := writeFileFS(fsys, name, bytes.NewReader(data), keptFileLimit); err != nil {
			return fmt.Errorf("restoring %s: %w", name, err)
		}
	}
	return nil
}

func readProperty(properties []byte, key string) (string, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(properties))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(k) == key {
			return v, true
		}
	}
	return "", false
}

// mergeProperties is base with the given keys taken from override (added when base lacks them)
func mergeProperties(base, override []byte, keys []string) []byte {
	values := make(map[string]string)
	for _, key := range keys {
		if v, ok := readProperty(override, key); ok {
			values[key] = v
		}
	}
	var out bytes.Buffer
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(base))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if k, _, ok := strings.Cut(trimmed, "="); ok && !strings.HasPrefix(trimmed, "#") {
			key := strings.TrimSpace(k)
			if v, replace := values[key]; replace {
				line = key + "=" + v
				seen[key] = true
			}
		}
		out.WriteString(line + "\n")
	}
	for _, key := range keys {
		if v, ok := values[key]; ok && !seen[key] {
			out.WriteString(key + "=" + v + "\n")
		}
	}
	return out.Bytes()
}

// cleanPackPath checks a path from a pack ("mods/x.jar") and turns it into an OS path that
// stays inside the server folder
func cleanPackPath(p string) (string, error) {
	if p == "" || strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%w: %q", errUnsafePackPath, p)
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == ".." {
			return "", fmt.Errorf("%w: %q", errUnsafePackPath, p)
		}
	}
	clean := path.Clean(p)
	if clean == "." || !filepath.IsLocal(filepath.FromSlash(clean)) {
		return "", fmt.Errorf("%w: %q", errUnsafePackPath, p)
	}
	return filepath.FromSlash(clean), nil
}

// writeFileFS writes r to name inside the server folder, at most limit bytes
func writeFileFS(fsys files.FileServer, name string, r io.Reader, limit int64) (int64, error) {
	if dir := filepath.Dir(name); dir != "." {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			return 0, err
		}
	}
	f, err := fsys.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, io.LimitReader(r, limit+1))
	closeErr := f.Close()
	if err == nil && n > limit {
		err = fmt.Errorf("%s is larger than allowed", name)
	}
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = fsys.Remove(name)
	}
	return n, err
}
