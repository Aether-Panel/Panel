package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
