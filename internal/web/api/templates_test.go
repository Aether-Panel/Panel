package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// templatesRouter serves the local template routes over a fresh database
func templatesRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "templates.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Template{}))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.GET("/api/templates/:repo", getsTemplatesForRepo)
	r.GET("/api/templates/:repo/:name", getTemplateFromRepo)
	r.PUT("/api/templates/0/:name", putTemplate)
	r.POST("/api/templates/upload", uploadTemplates)
	return r, db
}

func sendRaw(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// jsonKeys lists the keys of the JSON object at path (top level when path is empty) in the
// order they are written
func jsonKeys(t *testing.T, data []byte, path ...string) []string {
	t.Helper()
	for _, key := range path {
		var object map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &object))
		data = object[key]
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), tok)
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		require.NoError(t, err)
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		require.NoError(t, dec.Decode(&skip))
	}
	return keys
}

const orderedTemplate = `{
  "name": "mine",
  "display": "Mine",
  "type": "minecraft-java",
  "authorNote": {"keep": true},
  "data": {
    "zeta": {"type": "string", "value": "z", "display": "Zeta", "hint": "kept"},
    "alpha": {"type": "integer", "value": 1},
    "mid": {"value": "m"}
  },
  "install": [{"type": "command", "commands": ["echo hi"]}],
  "run": {"command": "java -jar server.jar"}
}`

func TestPutTemplate_GetAnswersTheJSONAsWritten(t *testing.T) {
	r, _ := templatesRouter(t)

	w := sendRaw(r, http.MethodPut, "/api/templates/0/mine", orderedTemplate)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	w = sendRaw(r, http.MethodGet, "/api/templates/0/mine", "")
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.Bytes()

	assert.Equal(t, []string{"display", "type", "authorNote", "data", "install", "run", "name"}, jsonKeys(t, body),
		"the definition in its order, then the name, as before")
	assert.Equal(t, []string{"zeta", "alpha", "mid"}, jsonKeys(t, body, "data"), "variables keep the order they were sent in")

	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "mine", got["name"])
	assert.Equal(t, map[string]any{"keep": true}, got["authorNote"], "an unknown top-level key survives")
	zeta := got["data"].(map[string]any)["zeta"].(map[string]any)
	assert.Equal(t, "kept", zeta["hint"], "an unknown variable field survives")
	assert.NotContains(t, got["data"].(map[string]any)["mid"], "type", "nothing is added to what was written")
}

func TestPutTemplate_TheServerDefinitionIsStillDecoded(t *testing.T) {
	r, db := templatesRouter(t)
	require.Equal(t, http.StatusNoContent, sendRaw(r, http.MethodPut, "/api/templates/0/mine", orderedTemplate).Code)

	stored := &models.Template{Name: "mine"}
	require.NoError(t, db.First(stored).Error)
	assert.Equal(t, "Mine", stored.Display)
	assert.Equal(t, "java -jar server.jar", stored.Execution.Command)
	assert.Equal(t, 1, stored.Variables["alpha"].Value)
	assert.Equal(t, "string", stored.Variables["mid"].Type.Type, "creating a server still gets the defaults")
}

func TestPutTemplate_ReplacingKeepsTheNewOrder(t *testing.T) {
	r, _ := templatesRouter(t)
	require.Equal(t, http.StatusNoContent, sendRaw(r, http.MethodPut, "/api/templates/0/mine", orderedTemplate).Code)

	reordered := `{"data":{"mid":{"value":"m"},"zeta":{"value":"z"},"alpha":{"type":"integer","value":2}},"run":{"command":"x"}}`
	require.Equal(t, http.StatusNoContent, sendRaw(r, http.MethodPut, "/api/templates/0/mine", reordered).Code)

	w := sendRaw(r, http.MethodGet, "/api/templates/0/mine", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"mid", "zeta", "alpha"}, jsonKeys(t, w.Body.Bytes(), "data"))
	assert.Equal(t, []string{"data", "run", "name"}, jsonKeys(t, w.Body.Bytes()))
}

func TestPutTemplate_IntegerLikeVariableKeys(t *testing.T) {
	r, db := templatesRouter(t)
	body := `{"data":{"25565":{"value":"a"},"b":{"value":"b"},"10":{"type":"integer","value":"7"}},"run":{"command":"x"}}`
	require.Equal(t, http.StatusNoContent, sendRaw(r, http.MethodPut, "/api/templates/0/nums", body).Code)

	w := sendRaw(r, http.MethodGet, "/api/templates/0/nums", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"25565", "b", "10"}, jsonKeys(t, w.Body.Bytes(), "data"))
	assert.Contains(t, w.Body.String(), `"10":{"type":"integer","value":"7"}`, "values are served as written")

	stored := &models.Template{Name: "nums"}
	require.NoError(t, db.First(stored).Error)
	assert.Equal(t, 7, stored.Variables["10"].Value, "Go still reads them typed")
}

func TestPutTemplate_InvalidBodiesAreRejected(t *testing.T) {
	r, db := templatesRouter(t)

	for name, body := range map[string]string{
		"not JSON":         `{"data":`,
		"wrong field type": `{"display": 5}`,
		"an array":         `[1, 2]`,
		"wrong variable":   `{"data": {"x": {"type": "boolean", "value": "maybe"}}}`,
		"null":             `null`,
	} {
		w := sendRaw(r, http.MethodPut, "/api/templates/0/bad", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
	}

	var count int64
	require.NoError(t, db.Model(&models.Template{}).Count(&count).Error)
	assert.Zero(t, count, "nothing was stored")
}

func TestPutTemplate_NameAndReadmeInTheBodyAreNotStored(t *testing.T) {
	r, db := templatesRouter(t)
	require.NoError(t, db.Create(&models.Template{Name: "mine", Readme: "# Mine"}).Error)

	body := `{"name":"other","readme":"ignored","display":"Mine","run":{"command":"x"}}`
	require.Equal(t, http.StatusNoContent, sendRaw(r, http.MethodPut, "/api/templates/0/mine", body).Code)

	var row struct{ RawValue, Readme string }
	require.NoError(t, db.Table("templates").Where("name = ?", "mine").Select("raw_value, readme").Scan(&row).Error)
	assert.Equal(t, `{"display":"Mine","run":{"command":"x"}}`, row.RawValue, "stored compact, as written, without name and readme")
	assert.Equal(t, "# Mine", row.Readme, "the stored readme is kept, as before")

	w := sendRaw(r, http.MethodGet, "/api/templates/0/mine", "")
	assert.JSONEq(t, `{"display":"Mine","run":{"command":"x"},"name":"mine","readme":"# Mine"}`, w.Body.String())
}

func TestGetTemplate_AnOldRowIsServedAsBefore(t *testing.T) {
	r, db := templatesRouter(t)
	// a template saved before the panel kept templates as written: Go's sorted output
	legacy := `{"type":"","data":{"a":{"type":"string","value":"1","required":false,"userEdit":false},"b":{"type":"string","value":"2","required":false,"userEdit":false}},"install":null,"uninstall":null,"run":{"command":null,"autostart":false,"autorecover":false,"autorestart":false},"environment":{ "type": "" }}`
	require.NoError(t, db.Exec("INSERT INTO templates (name, raw_value, readme) VALUES (?, ?, '')", "old", legacy).Error)

	w := sendRaw(r, http.MethodGet, "/api/templates/0/old", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"a", "b"}, jsonKeys(t, w.Body.Bytes(), "data"))
	assert.Equal(t, "old", jsonValue(t, w.Body.Bytes(), "name"))
}

func TestUploadTemplates_KeepsTheFileAsWritten(t *testing.T) {
	r, _ := templatesRouter(t)

	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	part, err := mw.CreateFormFile("files", "paper.json")
	require.NoError(t, err)
	_, err = part.Write([]byte(`{"data":{"z":{"value":"1","extra":1},"a":{"value":"2"}},"custom":1,"run":{"command":"x"}}`))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/templates/upload", &form)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = sendRaw(r, http.MethodGet, "/api/templates/0/paper", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"data", "custom", "run", "display", "name"}, jsonKeys(t, w.Body.Bytes()),
		"the missing display name is added at the end")
	assert.Equal(t, []string{"z", "a"}, jsonKeys(t, w.Body.Bytes(), "data"))
	assert.Contains(t, w.Body.String(), `"extra":1`)
	assert.Equal(t, "paper", jsonValue(t, w.Body.Bytes(), "display"))
}

func TestGetsTemplatesForRepo_LocalListIsStillASummary(t *testing.T) {
	r, _ := templatesRouter(t)
	require.Equal(t, http.StatusNoContent, sendRaw(r, http.MethodPut, "/api/templates/0/mine", orderedTemplate).Code)

	w := sendRaw(r, http.MethodGet, "/api/templates/0", "")
	require.Equal(t, http.StatusOK, w.Code)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list, 1)
	assert.Equal(t, "mine", list[0]["name"])
	assert.Equal(t, "Mine", list[0]["display"])
	assert.NotContains(t, list[0], "authorNote")
}

func jsonValue(t *testing.T, data []byte, key string) any {
	t.Helper()
	var object map[string]any
	require.NoError(t, json.Unmarshal(data, &object))
	return object[key]
}
