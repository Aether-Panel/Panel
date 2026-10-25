package servers

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mpSHA512(data []byte) string {
	sum := sha512.Sum512(data)
	return hex.EncodeToString(sum[:])
}

// buildMrpack zips an index and extra entries into a .mrpack
func buildMrpack(t *testing.T, index interface{}, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	data, err := json.Marshal(index)
	require.NoError(t, err)
	w, err := zw.Create("modrinth.index.json")
	require.NoError(t, err)
	_, _ = w.Write(data)
	for name, content := range entries {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, _ = w.Write([]byte(content))
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func mpNewFS(t *testing.T) (files.FileServer, string) {
	t.Helper()
	root := t.TempDir()
	fsys, err := files.NewFileServer(root, -1, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fsys.Close() })
	return fsys, root
}

func mpWriteFile(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0644))
}

func mpExists(root, name string) bool {
	_, err := os.Lstat(filepath.Join(root, name))
	return err == nil
}

// allowTestHost lets the downloads reach a local test server
func allowTestHost(t *testing.T, srv *httptest.Server) {
	t.Helper()
	host := strings.TrimPrefix(srv.URL, "http://")
	previous := modpackDownloadAllowed
	modpackDownloadAllowed = func(u *url.URL) bool { return u.Host == host }
	t.Cleanup(func() { modpackDownloadAllowed = previous })
}

func TestJavaForMinecraft(t *testing.T) {
	for mc, want := range map[string]string{
		"1.21.4": "21", "1.20.6": "21", "1.20.5": "21",
		"1.20.4": "17", "1.20.1": "17", "1.18.2": "17", "1.18": "17",
		"1.17.1": "17", "1.17": "17",
		"1.16.5": "8", "1.12.2": "8", "1.7.10": "8",
		"26.1": "25", "nonsense": "21",
	} {
		assert.Equal(t, want, javaForMinecraft(mc), mc)
	}
}

func TestCleanPackPath(t *testing.T) {
	good := map[string]string{
		"mods/a.jar":            filepath.Join("mods", "a.jar"),
		"./config/x.toml":       filepath.Join("config", "x.toml"),
		"config/sub/./deep.cfg": filepath.Join("config", "sub", "deep.cfg"),
		"server-icon.png":       "server-icon.png",
	}
	for in, want := range good {
		got, err := cleanPackPath(in)
		assert.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", ".", "../evil.jar", "mods/../../evil.jar", "/etc/passwd", `mods\..\..\evil.jar`, "mods/\x00.jar", "a/../.."} {
		_, err := cleanPackPath(bad)
		assert.ErrorIs(t, err, errUnsafePackPath, bad)
	}
}

func TestCleanForModpack(t *testing.T) {
	fsys, root := mpNewFS(t)
	outside := t.TempDir()
	mpWriteFile(t, outside, "precious.txt", "keep me")

	for _, name := range []string{
		"mods/old.jar", "config/old.toml", "defaultconfigs/a.toml", "kubejs/a.js", "libraries/lib.jar",
		"server.jar", "fabric-server-launch.jar", "run.sh", "run.bat", "user_jvm_args.txt",
		"forge-1.20.1-47.2.0-installer.jar", "forge-1.20.1-47.2.0-installer.jar.log", "neoforge-21.1.1-installer.jar",
		"survival/level.dat", "survival_nether/level.dat", "survival_the_end/level.dat", "world/level.dat",
		"plugins/a.jar", "whitelist.json", "ops.json", "banned-players.json", "eula.txt", "logs/latest.log",
	} {
		mpWriteFile(t, root, name, "x")
	}
	mpWriteFile(t, root, "server.properties", "motd=hi\nlevel-name=survival\n")
	// a symlink the owner made: only the link may go, never what it points to
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "scripts")))

	require.NoError(t, cleanForModpack(fsys, root, false))

	for _, gone := range []string{"mods", "config", "defaultconfigs", "kubejs", "libraries", "server.jar", "fabric-server-launch.jar",
		"run.sh", "run.bat", "user_jvm_args.txt", "forge-1.20.1-47.2.0-installer.jar", "forge-1.20.1-47.2.0-installer.jar.log",
		"neoforge-21.1.1-installer.jar", "scripts"} {
		assert.False(t, mpExists(root, gone), "%s should be removed", gone)
	}
	for _, kept := range []string{"survival/level.dat", "survival_nether", "world", "plugins/a.jar", "whitelist.json", "ops.json",
		"banned-players.json", "eula.txt", "server.properties", "logs/latest.log"} {
		assert.True(t, mpExists(root, kept), "%s should be kept", kept)
	}
	assert.True(t, mpExists(outside, "precious.txt"), "the symlink target must not be touched")

	// a fresh world removes the world named in server.properties, nothing else
	require.NoError(t, cleanForModpack(fsys, root, true))
	assert.False(t, mpExists(root, "survival"))
	assert.False(t, mpExists(root, "survival_nether"))
	assert.False(t, mpExists(root, "survival_the_end"))
	assert.True(t, mpExists(root, "world"), "only the configured level is a world to replace")
	assert.True(t, mpExists(root, "server.properties"))
}

func TestWorldFolders(t *testing.T) {
	assert.Equal(t, []string{"world", "world_nether", "world_the_end"}, worldFolders(nil))
	assert.Equal(t, []string{"lobby", "lobby_nether", "lobby_the_end"}, worldFolders([]byte("level-name=lobby\n")))
	assert.Nil(t, worldFolders([]byte("level-name=../../etc\n")))
	assert.Nil(t, worldFolders([]byte("level-name=a/b\n")))
}

func TestMergeProperties(t *testing.T) {
	pack := []byte("# pack\nlevel-type=skyblock\nserver-port=25565\nmotd=Pack MOTD\n")
	owner := []byte("server-port=25601\nmotd=Mine\nlevel-type=default\nserver-ip=0.0.0.0\n")
	merged := string(mergeProperties(pack, owner, keptProperties))
	assert.Contains(t, merged, "level-type=skyblock", "the pack's world settings are used")
	assert.Contains(t, merged, "server-port=25601", "the owner's port stays")
	assert.Contains(t, merged, "motd=Mine")
	assert.Contains(t, merged, "server-ip=0.0.0.0", "keys the pack lacks are added")
	assert.NotContains(t, merged, "25565")
}

func TestKeptFilesSurviveThePack(t *testing.T) {
	fsys, root := mpNewFS(t)
	mpWriteFile(t, root, "server.properties", "server-port=25601\nlevel-type=default\n")
	mpWriteFile(t, root, "ops.json", `["owner"]`)
	kept, err := readKeptFiles(fsys)
	require.NoError(t, err)

	// the pack ships its own copies
	mpWriteFile(t, root, "server.properties", "server-port=25565\nlevel-type=skyblock\n")
	mpWriteFile(t, root, "ops.json", `[]`)
	require.NoError(t, restoreKeptFiles(fsys, kept, false))
	data, _ := os.ReadFile(filepath.Join(root, "server.properties"))
	assert.Equal(t, "server-port=25601\nlevel-type=default\n", string(data))
	data, _ = os.ReadFile(filepath.Join(root, "ops.json"))
	assert.Equal(t, `["owner"]`, string(data))

	mpWriteFile(t, root, "server.properties", "server-port=25565\nlevel-type=skyblock\n")
	require.NoError(t, restoreKeptFiles(fsys, kept, true))
	data, _ = os.ReadFile(filepath.Join(root, "server.properties"))
	assert.Equal(t, "server-port=25601\nlevel-type=skyblock\n", string(data))
}

func TestInstallerFor(t *testing.T) {
	address, jar, err := installerFor("forge", "1.20.1", "47.2.0")
	require.NoError(t, err)
	assert.Equal(t, forgeMaven+"/net/minecraftforge/forge/1.20.1-47.2.0/forge-1.20.1-47.2.0-installer.jar", address)
	assert.Equal(t, "forge-1.20.1-47.2.0-installer.jar", jar)

	address, jar, err = installerFor("neoforge", "1.21.1", "21.1.77")
	require.NoError(t, err)
	assert.Equal(t, neoforgeMaven+"/net/neoforged/neoforge/21.1.77/neoforge-21.1.77-installer.jar", address)
	assert.Equal(t, "neoforge-21.1.77-installer.jar", jar)

	address, _, err = installerFor("neoforge", "1.20.1", "47.1.106")
	require.NoError(t, err)
	assert.Equal(t, neoforgeMaven+"/net/neoforged/forge/1.20.1-47.1.106/forge-1.20.1-47.1.106-installer.jar", address)

	// the version ends up in a command line: anything odd is refused
	_, _, err = installerFor("forge", "1.20.1", "47.2.0; rm -rf /")
	assert.Error(t, err)
	_, _, err = installerFor("quilt", "1.20.1", "0.26.0")
	assert.ErrorIs(t, err, ErrModpackUnsupported)
}

func TestValidateModpackRequest(t *testing.T) {
	previous := config.CurseForgeKey.Value()
	t.Cleanup(func() { _ = config.CurseForgeKey.Set(previous, false) })
	_ = config.CurseForgeKey.Set("", false)

	assert.NoError(t, validateModpackRequest(ModpackInstallRequest{Source: "modrinth", ProjectID: "1KVo5zza", VersionID: "abcDEF12"}))
	assert.ErrorIs(t, validateModpackRequest(ModpackInstallRequest{Source: "modrinth", ProjectID: "../x", VersionID: "a"}), ErrModpackBadRequest)
	assert.ErrorIs(t, validateModpackRequest(ModpackInstallRequest{Source: "ftb", ProjectID: "1", VersionID: "2"}), ErrModpackBadRequest)
	assert.ErrorIs(t, validateModpackRequest(ModpackInstallRequest{Source: "curseforge", ProjectID: "123", VersionID: "456"}), ErrCurseForgeKeyMissing)
	_ = config.CurseForgeKey.Set("test-key", false)
	assert.NoError(t, validateModpackRequest(ModpackInstallRequest{Source: "curseforge", ProjectID: "123", VersionID: "456"}))
	assert.ErrorIs(t, validateModpackRequest(ModpackInstallRequest{Source: "curseforge", ProjectID: "abc", VersionID: "456"}), ErrModpackBadRequest)
}

type packFile struct {
	Path      string            `json:"path"`
	Hashes    map[string]string `json:"hashes"`
	Env       map[string]string `json:"env,omitempty"`
	Downloads []string          `json:"downloads"`
	FileSize  int               `json:"fileSize"`
}

func packIndex(files []packFile, deps map[string]string) map[string]interface{} {
	return map[string]interface{}{"formatVersion": 1, "game": "minecraft", "versionId": "1.0", "name": "Test", "files": files, "dependencies": deps}
}

func writePack(t *testing.T, data []byte) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "pack.mrpack")
	require.NoError(t, os.WriteFile(file, data, 0644))
	return file
}

func noSteps(string, int, int) {}

func TestApplyMrpack_RejectsHashMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tampered mod"))
	}))
	defer srv.Close()
	allowTestHost(t, srv)
	fsys, root := mpNewFS(t)

	pack := buildMrpack(t, packIndex([]packFile{{
		Path: "mods/a.jar", Hashes: map[string]string{"sha512": mpSHA512([]byte("real mod"))}, Downloads: []string{srv.URL + "/a.jar"},
	}}, map[string]string{"minecraft": "1.20.1", "fabric-loader": "0.15.11"}), nil)

	_, err := applyMrpack(context.Background(), fsys, writePack(t, pack), noSteps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not match")
	assert.False(t, mpExists(root, "mods/a.jar"), "a file that fails its hash must not stay")
}

func TestApplyMrpack_RejectsPathTraversal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("mod"))
	}))
	defer srv.Close()
	allowTestHost(t, srv)
	deps := map[string]string{"minecraft": "1.20.1", "fabric-loader": "0.15.11"}

	fsys, root := mpNewFS(t)
	pack := buildMrpack(t, packIndex([]packFile{{
		Path: "../escape.jar", Hashes: map[string]string{"sha512": mpSHA512([]byte("mod"))}, Downloads: []string{srv.URL + "/a.jar"},
	}}, deps), nil)
	_, err := applyMrpack(context.Background(), fsys, writePack(t, pack), noSteps)
	assert.ErrorIs(t, err, errUnsafePackPath)
	assert.False(t, mpExists(filepath.Dir(root), "escape.jar"))

	fsys, root = mpNewFS(t)
	pack = buildMrpack(t, packIndex(nil, deps), map[string]string{"overrides/../../escape.txt": "x"})
	_, err = applyMrpack(context.Background(), fsys, writePack(t, pack), noSteps)
	assert.Error(t, err)
	assert.False(t, mpExists(filepath.Dir(root), "escape.txt"))
	assert.False(t, mpExists(filepath.Dir(filepath.Dir(root)), "escape.txt"))
}

func TestApplyMrpack_RefusesHostsOutsideTheAllowList(t *testing.T) {
	fsys, _ := mpNewFS(t)
	pack := buildMrpack(t, packIndex([]packFile{{
		Path: "mods/a.jar", Hashes: map[string]string{"sha512": mpSHA512([]byte("mod"))}, Downloads: []string{"https://evil.example.com/a.jar"},
	}}, map[string]string{"minecraft": "1.20.1", "fabric-loader": "0.15.11"}), nil)
	_, err := applyMrpack(context.Background(), fsys, writePack(t, pack), noSteps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestApplyMrpack_RefusesQuilt(t *testing.T) {
	fsys, _ := mpNewFS(t)
	pack := buildMrpack(t, packIndex(nil, map[string]string{"minecraft": "1.20.1", "quilt-loader": "0.26.0"}), nil)
	_, err := applyMrpack(context.Background(), fsys, writePack(t, pack), noSteps)
	assert.ErrorIs(t, err, ErrModpackUnsupported)
}

// fakeModrinth serves a small Fabric pack the way Modrinth, its CDN and Fabric's meta do
func fakeModrinth(t *testing.T) *httptest.Server {
	t.Helper()
	modA := []byte("mod A bytes")
	mux := http.NewServeMux()
	// the pack names the server's address, so the listener exists before the pack is built
	// and the server starts after it
	srv := httptest.NewUnstartedServer(mux)
	base := "http://" + srv.Listener.Addr().String()
	var pack []byte
	mux.HandleFunc("/cdn/a.jar", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(modA) })
	mux.HandleFunc("/cdn/client-only.jar", func(http.ResponseWriter, *http.Request) {
		t.Error("a client-only mod was downloaded")
	})
	mux.HandleFunc("/cdn/pack.mrpack", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(pack) })
	mux.HandleFunc("/v2/project/fakepack", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"id":"fakepack","slug":"fake-pack","title":"Fake Pack","icon_url":"https://cdn.modrinth.com/icon.png"}`)
	})
	mux.HandleFunc("/v2/version/v1", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "v1", "project_id": "fakepack", "name": "Fake Pack 1.0", "version_number": "1.0.0",
			"game_versions": []string{"1.20.1"}, "loaders": []string{"fabric"},
			"files": []map[string]interface{}{{
				"url": base + "/cdn/pack.mrpack", "filename": "fake-1.0.0.mrpack", "primary": true, "size": len(pack),
				"hashes": map[string]string{"sha512": mpSHA512(pack)},
			}},
		})
	})
	mux.HandleFunc("/fabric/versions/installer", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `[{"version":"1.1.0","stable":false},{"version":"1.0.1","stable":true}]`)
	})
	mux.HandleFunc("/fabric/versions/loader/1.20.1/0.15.11/1.0.1/server/jar", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("fabric launcher"))
	})
	pack = buildMrpack(t, packIndex([]packFile{
		{Path: "mods/a.jar", Hashes: map[string]string{"sha512": mpSHA512(modA)}, Downloads: []string{base + "/cdn/a.jar"}, FileSize: len(modA),
			Env: map[string]string{"client": "required", "server": "required"}},
		{Path: "mods/client-only.jar", Hashes: map[string]string{"sha512": "00"}, Downloads: []string{base + "/cdn/client-only.jar"},
			Env: map[string]string{"client": "required", "server": "unsupported"}},
	}, map[string]string{"minecraft": "1.20.1", "fabric-loader": "0.15.11"}), map[string]string{
		"overrides/config/a.toml":        "from overrides",
		"overrides/config/b.toml":        "from overrides",
		"server-overrides/config/b.toml": "from server-overrides",
		"overrides/server.properties":    "server-port=25565\n",
	})
	srv.Start()
	t.Cleanup(srv.Close)

	previousAPI, previousFabric := modrinthAPI, fabricMetaAPI
	modrinthAPI, fabricMetaAPI = srv.URL+"/v2", srv.URL+"/fabric"
	t.Cleanup(func() { modrinthAPI, fabricMetaAPI = previousAPI, previousFabric })
	allowTestHost(t, srv)
	return srv
}

func newModpackTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	p, dir := newBackupTestServer(t)
	for _, setting := range []config.DataFolder{config.ServersFolder, config.CacheFolder} {
		previous := setting.Value()
		folder := t.TempDir()
		_ = setting.Set(folder, false)
		t.Cleanup(func() { _ = setting.Set(previous, false) })
	}
	p.Identifier = "modpacktest"
	p.Type.Type = "minecraft-java"
	p.Environment.Type = "docker"
	p.RunningEnvironment.Type = "docker"
	p.RunningEnvironment.RootDirectory = dir
	p.RunningEnvironment.StatusTracker = skypanel.CreateTracker()
	p.Variables = map[string]skypanel.Variable{
		"version":     {Value: "1.21.4"},
		"javaversion": {Value: "21"},
		"memory":      {Value: 2048},
	}
	p.Installation = []skypanel.ConditionalMetadataType{op("mojangdl", map[string]interface{}{"version": "${version}"})}
	t.Cleanup(func() { ForgetModpack(p.ID()) })
	return p, dir
}

func waitModpack(t *testing.T, p *Server) ModpackStatus {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for p.modpackState().running.Load() {
		if time.Now().After(deadline) {
			t.Fatal("the modpack install never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return p.ModpackStatus()
}

func TestInstallModpack_ModrinthFabric(t *testing.T) {
	fakeModrinth(t)
	p, dir := newModpackTestServer(t)
	mpWriteFile(t, dir, "mods/old-mod.jar", "old")
	mpWriteFile(t, dir, "server.jar", "vanilla")
	mpWriteFile(t, dir, "world/level.dat", "my world")
	mpWriteFile(t, dir, "server.properties", "server-port=25601\n")

	versionCache["vanilla"] = cachedVersions{versions: []string{"1.21.4", "1.20.1"}, fetched: time.Now()}
	defer delete(versionCache, "vanilla")
	_, err := p.GameVersions()
	require.NoError(t, err, "the version picker works before a pack is installed")

	require.NoError(t, p.InstallModpack(ModpackInstallRequest{Source: "modrinth", ProjectID: "fakepack", VersionID: "v1", Label: "Fake Pack · 1.0.0"}))
	status := waitModpack(t, p)

	require.Empty(t, status.LastError)
	assert.Equal(t, ModpackStepDone, status.Step)
	read := func(name string) string {
		data, _ := os.ReadFile(filepath.Join(dir, name))
		return string(data)
	}
	assert.Equal(t, "mod A bytes", read("mods/a.jar"))
	assert.False(t, mpExists(dir, "mods/old-mod.jar"), "the old mods are replaced")
	assert.False(t, mpExists(dir, "mods/client-only.jar"))
	assert.Equal(t, "from overrides", read("config/a.toml"))
	assert.Equal(t, "from server-overrides", read("config/b.toml"), "server-overrides win over overrides")
	assert.Equal(t, "fabric launcher", read("server.jar"))
	assert.Equal(t, "my world", read("world/level.dat"))
	assert.Equal(t, "server-port=25601\n", read("server.properties"), "the owner's settings stay")

	assert.Equal(t, "1.20.1", p.Variables["version"].Value)
	assert.Equal(t, "17", p.Variables["javaversion"].Value)

	require.NotNil(t, status.Installed)
	assert.Equal(t, "Fake Pack", status.Installed.Title)
	assert.Equal(t, "1.0.0", status.Installed.VersionName)
	assert.Equal(t, "fabric", status.Installed.Loader)
	assert.Equal(t, "1.20.1", status.Installed.GameVersion)
	assert.Equal(t, ModpackPlatform{Loader: "fabric", GameVersion: "1.20.1"}, status.Platform)
	assert.Equal(t, 2048, status.Memory)

	_, err = p.GameVersions()
	assert.ErrorIs(t, err, ErrNoVersionSwitch, "the version picker hides under a pack")
	assert.ErrorIs(t, p.ChangeVersion("1.21.4"), ErrNoVersionSwitch)

	points := p.restorePoints()
	require.Len(t, points, 1)
	assert.Equal(t, RestoreReasonModpack, RestorePointReason(points[0].FileName))

	// restoring an older backup puts another server.jar back: the pack is no longer there
	mpWriteFile(t, dir, "server.jar", "vanilla again")
	assert.Nil(t, p.InstalledModpack())
}

func TestInstallModpack_RefusesWhileBusy(t *testing.T) {
	fakeModrinth(t)
	p, _ := newModpackTestServer(t)
	p.modpackState().running.Store(true)
	assert.ErrorIs(t, p.InstallModpack(ModpackInstallRequest{Source: "modrinth", ProjectID: "fakepack", VersionID: "v1"}), ErrModpackInstalling)
	p.modpackState().running.Store(false)

	p.versionState().running.Store(true)
	assert.ErrorIs(t, p.InstallModpack(ModpackInstallRequest{Source: "modrinth", ProjectID: "fakepack", VersionID: "v1"}), ErrVersionChanging)
	p.versionState().running.Store(false)

	// a version of another project is refused before anything runs
	err := p.InstallModpack(ModpackInstallRequest{Source: "modrinth", ProjectID: "otherpack", VersionID: "v1"})
	var up *UpstreamError
	assert.ErrorAs(t, err, &up, "the unknown project is Modrinth's 404")
	assert.False(t, p.modpackState().running.Load())
}

func TestModrinthVersionsAndSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/project/pack/version":
			_, _ = fmt.Fprint(w, `[
				{"id":"old","name":"Old","version_number":"1","game_versions":["1.19.2"],"loaders":["forge"],"date_published":"2023-01-01T00:00:00Z","files":[{"filename":"a.mrpack","primary":true}]},
				{"id":"quilt","name":"Quilt","version_number":"2","game_versions":["1.20.1"],"loaders":["quilt"],"date_published":"2024-01-01T00:00:00Z","files":[{"filename":"b.mrpack","primary":true}]},
				{"id":"nopack","name":"Zip","version_number":"3","game_versions":["1.20.1"],"loaders":["fabric"],"date_published":"2024-02-01T00:00:00Z","files":[{"filename":"b.zip","primary":true}]},
				{"id":"new","name":"New","version_number":"4","game_versions":["1.20.1","23w31a"],"loaders":["fabric","quilt"],"date_published":"2024-03-01T00:00:00Z","files":[{"filename":"c.mrpack","primary":true}]}
			]`)
		case "/v2/search":
			q := r.URL.Query()
			assert.Contains(t, q.Get("facets"), "project_type:modpack")
			if q.Get("query") == "" {
				assert.Equal(t, "downloads", q.Get("index"), "an empty search lists the most popular packs")
			}
			_, _ = fmt.Fprint(w, `{"hits":[{"project_id":"p1","slug":"atm","title":"All the Mods","author":"ATM","downloads":5,"categories":["neoforge","technology"],"versions":["1.21.1","1.20.1","24w10a"]}],"offset":0,"limit":20,"total_hits":1}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	previous := modrinthAPI
	modrinthAPI = srv.URL + "/v2"
	defer func() { modrinthAPI = previous }()

	versions, err := ModpackVersions("modrinth", "pack")
	require.NoError(t, err)
	require.Len(t, versions, 2, "quilt-only versions and versions without a .mrpack are not offered")
	assert.Equal(t, "new", versions[0].ID, "newest first")
	assert.Equal(t, []string{"fabric"}, versions[0].Loaders)
	assert.Equal(t, []string{"1.20.1"}, versions[0].GameVersions)

	result, err := SearchModpacks("modrinth", "", 0)
	require.NoError(t, err)
	require.Len(t, result.Hits, 1)
	assert.Equal(t, []string{"neoforge"}, result.Hits[0].Loaders)
	assert.Equal(t, []string{"1.21.1", "1.20.1"}, result.Hits[0].GameVersions)

	_, err = ModpackVersions("modrinth", "missing")
	var up *UpstreamError
	assert.ErrorAs(t, err, &up)
}

func TestCurseForgeVersions(t *testing.T) {
	previousKey := config.CurseForgeKey.Value()
	_ = config.CurseForgeKey.Set("test-key", false)
	defer func() { _ = config.CurseForgeKey.Set(previousKey, false) }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		_, _ = fmt.Fprint(w, `{"data":[
			{"id":10,"modId":1,"displayName":"Pack 1.0","fileStatus":4,"fileDate":"2024-01-01T00:00:00Z","gameVersions":["1.20.1","Forge"],"serverPackFileId":11},
			{"id":11,"modId":1,"displayName":"Pack 1.0 Server","fileStatus":4,"fileDate":"2024-01-01T00:00:01Z","gameVersions":["1.20.1","Forge"],"isServerPack":true,"parentProjectFileId":10},
			{"id":12,"modId":1,"displayName":"Client only","fileStatus":4,"fileDate":"2024-02-01T00:00:00Z","gameVersions":["1.20.1","Forge"]},
			{"id":13,"modId":1,"displayName":"Quilt","fileStatus":4,"fileDate":"2024-03-01T00:00:00Z","gameVersions":["1.20.1","Quilt"],"serverPackFileId":14},
			{"id":15,"modId":1,"displayName":"Pack 2.0","fileStatus":4,"fileDate":"2024-04-01T00:00:00Z","gameVersions":["1.21.1","NeoForge"],"serverPackFileId":16}
		]}`)
	}))
	defer srv.Close()
	previous := curseForgeAPI
	curseForgeAPI = srv.URL
	defer func() { curseForgeAPI = previous }()

	versions, err := ModpackVersions("curseforge", "1")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, "15", versions[0].ID)
	assert.Equal(t, []string{"neoforge"}, versions[0].Loaders)
	assert.True(t, versions[0].ServerPack)
	assert.Equal(t, "10", versions[1].ID, "the server pack is offered once, through its client file")
}
