package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginFilter(t *testing.T) {
	assert.Equal(t, []string{"paper", "spigot", "bukkit"}, pluginFilter("paper, Spigot ,bukkit"))
	assert.Equal(t, []string{"1.21.4"}, pluginFilter("1.21.4"))
	assert.Empty(t, pluginFilter(""))
	assert.Empty(t, pluginFilter(" , "))
	// anything that is not a plain name or version is dropped, not sent to Modrinth
	assert.Equal(t, []string{"fabric"}, pluginFilter(`fabric,"],a&b=c,../x`))
}

func TestModrinthVersionsURL(t *testing.T) {
	previous := pluginModrinthAPI
	pluginModrinthAPI = "https://api.example.test/v2"
	t.Cleanup(func() { pluginModrinthAPI = previous })

	assert.Equal(t, "https://api.example.test/v2/project/abc/version", modrinthVersionsURL("abc", nil, nil),
		"without a filter every version is listed, as before")

	got := modrinthVersionsURL("abc", []string{"paper", "spigot"}, []string{"1.21.4"})
	assert.Equal(t, "https://api.example.test/v2/project/abc/version?game_versions=%5B%221.21.4%22%5D&loaders=%5B%22paper%22%2C%22spigot%22%5D", got)

	assert.Equal(t, "https://api.example.test/v2/project/a%2Fb/version", modrinthVersionsURL("a/b", nil, nil),
		"the project id stays one path segment")
}

func TestFetchLatestModrinthFileFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotLoaders, gotGameVersions string
	versions := `[{"id":"v2","files":[{"url":"https://cdn.modrinth.com/b.jar","filename":"b.jar","primary":false},{"url":"https://cdn.modrinth.com/a.jar","filename":"a.jar","primary":true}]}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLoaders = r.URL.Query().Get("loaders")
		gotGameVersions = r.URL.Query().Get("game_versions")
		w.Header().Set("Content-Type", "application/json")
		if gotLoaders == `["forge"]` {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(versions))
	}))
	t.Cleanup(srv.Close)

	previous := pluginModrinthAPI
	pluginModrinthAPI = srv.URL + "/v2"
	t.Cleanup(func() { pluginModrinthAPI = previous })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	downloadURL, fileName, ok := fetchLatestModrinthFile(c, "abc", []string{"paper", "spigot"}, []string{"1.21.4"})
	require.True(t, ok)
	assert.Equal(t, `["paper","spigot"]`, gotLoaders)
	assert.Equal(t, `["1.21.4"]`, gotGameVersions)
	assert.Equal(t, "https://cdn.modrinth.com/a.jar", downloadURL, "the primary file is picked")
	assert.Equal(t, "a.jar", fileName)

	// no version fits the server: a clear error code instead of a generic failure
	w := httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	_, _, ok = fetchLatestModrinthFile(c, "abc", []string{"forge"}, nil)
	require.False(t, ok)
	assert.Equal(t, http.StatusNotFound, w.Code)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ErrPluginNoCompatibleVersion", body.Error.Code)
}

func TestPluginFolder(t *testing.T) {
	assert.Equal(t, "mods", pluginFolder("fabric"))
	assert.Equal(t, "mods", pluginFolder("forge"))
	assert.Equal(t, "mods", pluginFolder("neoforge"))
	assert.Equal(t, "plugins", pluginFolder("paper"))
	assert.Equal(t, "plugins", pluginFolder(""), "a server the panel cannot tell keeps plugins/, as before")
}

func TestPluginSearchFacets(t *testing.T) {
	assert.Equal(t, `[["project_type:mod","project_type:plugin"]]`, pluginSearchFacets(""),
		"an unknown platform searches everything, as before")
	assert.Equal(t, `[["project_type:mod","project_type:plugin"],["categories:paper","categories:spigot","categories:bukkit"]]`,
		pluginSearchFacets("paper"))
	assert.Equal(t, `[["project_type:mod"],["categories:fabric"],["server_side:required","server_side:optional"]]`,
		pluginSearchFacets("fabric"))
	assert.Equal(t, `[["project_type:mod"],["categories:neoforge"],["server_side:required","server_side:optional"]]`,
		pluginSearchFacets("neoforge"))
}

func TestFetchLatestModrinthFileRefusesUnsafeNames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, name := range []string{"../evil.jar", "mods/evil.jar", `..\evil.jar`, "evil.zip", "", ".."} {
		srv := modrinthServer(t, map[string]string{
			"/v2/project/abc/version": `[{"id":"v1","files":[{"url":"https://cdn.modrinth.com/x","filename":` + jsonString(name) + `,"primary":true}]}]`,
		})
		usePluginModrinthAPI(t, srv.URL+"/v2")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		_, _, ok := fetchLatestModrinthFile(c, "abc", nil, nil)
		assert.False(t, ok, name)
		assert.Equal(t, http.StatusBadGateway, w.Code, name)
		assert.Equal(t, "ErrPluginBadFileName", errorCode(t, w), name)
	}
}

func TestInstallPlugin_FabricGoesToModsWithARestorePoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "fabric")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/project/abc/version":
			_, _ = w.Write([]byte(`[{"id":"v1","files":[{"url":"http://` + r.Host + `/files/sodium-1.0.jar","filename":"sodium-1.0.jar","primary":true}]}]`))
		case "/files/sodium-1.0.jar":
			_, _ = w.Write([]byte("jar"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	usePluginModrinthAPI(t, srv.URL+"/v2")

	w := runPluginHandler(installPlugin, server, http.MethodPost, "/?loaders=fabric", gin.Params{{Key: "pluginId", Value: "abc"}})
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	data, err := os.ReadFile(filepath.Join(dir, "mods", "sodium-1.0.jar"))
	require.NoError(t, err)
	assert.Equal(t, "jar", string(data))
	assert.NoDirExists(t, filepath.Join(dir, "plugins"))
	assert.Len(t, restorePointFiles(t), 1)

	// the list and the delete use the same folder
	w = runPluginHandler(getPlugins, server, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var list []PluginInfo
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	if assert.Len(t, list, 1) {
		assert.Equal(t, "sodium-1.0.jar", list[0].Name)
	}

	w = runPluginHandler(deletePlugin, server, http.MethodDelete, "/?name=sodium-1.0.jar", nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.NoFileExists(t, filepath.Join(dir, "mods", "sodium-1.0.jar"))
}

func TestInstallPlugin_PaperStaysInPlugins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "paper")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/project/abc/version":
			_, _ = w.Write([]byte(`[{"id":"v1","files":[{"url":"http://` + r.Host + `/files/luckperms.jar","filename":"luckperms.jar","primary":true}]}]`))
		case "/files/luckperms.jar":
			_, _ = w.Write([]byte("jar"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	usePluginModrinthAPI(t, srv.URL+"/v2")

	w := runPluginHandler(installPlugin, server, http.MethodPost, "/", gin.Params{{Key: "pluginId", Value: "abc"}})
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.FileExists(t, filepath.Join(dir, "plugins", "luckperms.jar"))
	assert.NoDirExists(t, filepath.Join(dir, "mods"))
}

func TestInstallPlugin_RefusedLookupLeavesNoRestorePoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "paper")

	srv := modrinthServer(t, map[string]string{
		"/v2/project/none/version": `[]`,
		"/v2/project/evil/version": `[{"id":"v1","files":[{"url":"https://cdn.modrinth.com/x","filename":"../evil.jar","primary":true}]}]`,
	})
	usePluginModrinthAPI(t, srv.URL+"/v2")

	w := runPluginHandler(installPlugin, server, http.MethodPost, "/?loaders=paper", gin.Params{{Key: "pluginId", Value: "none"}})
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "ErrPluginNoCompatibleVersion", errorCode(t, w))

	w = runPluginHandler(installPlugin, server, http.MethodPost, "/", gin.Params{{Key: "pluginId", Value: "evil"}})
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Equal(t, "ErrPluginBadFileName", errorCode(t, w))

	assert.Empty(t, restorePointFiles(t), "nothing was installed, so nothing to go back to")
	assert.NoFileExists(t, filepath.Join(filepath.Dir(dir), "evil.jar"))
	assert.NoDirExists(t, filepath.Join(dir, "plugins"))
}

func TestSearchPlugins_FiltersByPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var facets string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		facets = r.URL.Query().Get("facets")
		_, _ = w.Write([]byte(`{"hits":[{"project_id":"abc","title":"Sodium","downloads":5}]}`))
	}))
	t.Cleanup(srv.Close)
	usePluginModrinthAPI(t, srv.URL+"/v2")

	server, _ := newPluginTestServer(t, "fabric")
	w := runPluginHandler(searchPlugins, server, http.MethodGet, "/?q=sodium", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, pluginSearchFacets("fabric"), facets)
	var results []PluginSearchResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &results))
	if assert.Len(t, results, 1) {
		assert.Equal(t, "abc", results[0].ID)
	}

	server, _ = newPluginTestServer(t, "")
	w = runPluginHandler(searchPlugins, server, http.MethodGet, "/?q=sodium", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, pluginSearchFacets(""), facets, "a server the panel cannot tell searches everything")
}

// newPluginTestServer is a stopped Minecraft server in a temporary folder, with restore
// points kept in that folder too; launcher is its "modlauncher" variable (none when empty)
func newPluginTestServer(t *testing.T, launcher string) (*servers.Server, string) {
	root := t.TempDir()
	previous := config.BackupsFolder.Value()
	require.NoError(t, config.BackupsFolder.Set(filepath.Join(root, "backups"), false))
	t.Cleanup(func() { _ = config.BackupsFolder.Set(previous, false) })

	serverDir := filepath.Join(root, "server")
	require.NoError(t, os.MkdirAll(serverDir, 0755))
	fs, err := files.NewFileServer(serverDir, os.Getuid(), os.Getgid())
	require.NoError(t, err)

	p := servers.CreateProgram()
	p.Identifier = "plugintest"
	p.Type.Type = "minecraft-java"
	if launcher != "" {
		p.Variables["modlauncher"] = skypanel.Variable{Value: launcher}
	}
	p.SetFileServer(fs)
	p.RunningEnvironment = &skypanel.Environment{
		ConsoleBuffer:  skypanel.CreateCache(),
		ConsoleTracker: skypanel.CreateTracker(),
		Implementation: &stoppedEnvironment{},
	}
	return p, serverDir
}

// stoppedEnvironment is a server that is not running: enough for plugins and restore points,
// which only touch files
type stoppedEnvironment struct{}

func (stoppedEnvironment) ExecuteAsyncImpl(*skypanel.Environment, skypanel.ExecutionData) error {
	return nil
}
func (stoppedEnvironment) KillImpl(*skypanel.Environment) error { return nil }
func (stoppedEnvironment) GetStatsImpl(*skypanel.Environment) (*skypanel.ServerStats, error) {
	return &skypanel.ServerStats{}, nil
}
func (stoppedEnvironment) SendCodeImpl(*skypanel.Environment, int) error     { return nil }
func (stoppedEnvironment) GetUIDImpl(*skypanel.Environment) int              { return os.Getuid() }
func (stoppedEnvironment) GetGidImpl(*skypanel.Environment) int              { return os.Getgid() }
func (stoppedEnvironment) IsRunningImpl(*skypanel.Environment) (bool, error) { return false, nil }

// runPluginHandler calls a plugin handler for the server the way the router does
func runPluginHandler(handler gin.HandlerFunc, server *servers.Server, method, target string, params gin.Params) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	c.Params = params
	c.Set("program", server)
	handler(c)
	// gin only writes a bare c.Status(...) when the response is finished; do it here as the
	// server does, or the recorder keeps its default 200
	c.Writer.WriteHeaderNow()
	return w
}

// modrinthServer answers each path with its JSON body and anything else with 404
func modrinthServer(t *testing.T, bodies map[string]string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func usePluginModrinthAPI(t *testing.T, api string) {
	previous := pluginModrinthAPI
	pluginModrinthAPI = api
	t.Cleanup(func() { pluginModrinthAPI = previous })
}

// restorePointFiles lists the restore points written under the test's backups folder
func restorePointFiles(t *testing.T) []string {
	found := make([]string, 0)
	err := filepath.WalkDir(config.BackupsFolder.Value(), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			return err
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), servers.RestorePointPrefix) {
			found = append(found, path)
		}
		return nil
	})
	require.NoError(t, err)
	return found
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Error.Code
}

func jsonString(s string) string {
	encoded, _ := json.Marshal(s)
	return string(encoded)
}
