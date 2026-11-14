package models

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTemplate_UnmarshalKeepsTheJSONAsWritten(t *testing.T) {
	file := `{"readme":"from file","data":{"b":{"value":"1","note":"x"},"a":{"value":"2"}},"name":"paper","extra":[1, 2]}`
	tmpl := &Template{Name: "fallback"}
	require.NoError(t, json.Unmarshal([]byte(file), tmpl))

	assert.Equal(t, "paper", tmpl.Name, "a name in the file wins, as before")
	assert.Equal(t, "from file", tmpl.Readme)
	assert.Equal(t, "string", tmpl.Variables["b"].Type.Type, "Server is decoded as before")

	out, err := json.Marshal(tmpl)
	require.NoError(t, err)
	assert.Equal(t, `{"data":{"b":{"value":"1","note":"x"},"a":{"value":"2"}},"extra":[1,2],"name":"paper","readme":"from file"}`, string(out))
}

func TestTemplate_BuiltInGoIsMarshalledAsBefore(t *testing.T) {
	tmpl := &Template{Name: "x", Server: skypanel.Server{Display: "X"}}
	out, err := json.Marshal(tmpl)
	require.NoError(t, err)

	type plain Template
	before, err := json.Marshal(plain(*tmpl))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(out))
}

func TestTemplate_AChangedServerIsNotHiddenByTheWrittenJSON(t *testing.T) {
	tmpl := &Template{Name: "x"}
	require.NoError(t, tmpl.SetDefinition([]byte(`{"display":"Old","custom":1}`)))
	tmpl.Display = "New"

	out, err := json.Marshal(tmpl)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"display":"New"`)

	require.NoError(t, tmpl.BeforeSave(nil))
	assert.Contains(t, tmpl.RawValue, `"display":"New"`, "a Go change to Server is what gets stored")
}

func TestTemplate_SetFieldKeepsItsPlace(t *testing.T) {
	tmpl := &Template{Name: "x"}
	require.NoError(t, tmpl.SetDefinition([]byte(`{"a":1,"display":"","b":2}`)))
	require.NoError(t, tmpl.SetField("display", "Shown"))
	assert.Equal(t, "Shown", tmpl.Display)

	require.NoError(t, tmpl.SetField("added", true))
	out, err := json.Marshal(tmpl)
	require.NoError(t, err)
	assert.Equal(t, `{"a":1,"display":"Shown","b":2,"added":true,"name":"x"}`, string(out))
}

func TestTemplate_SetDefinitionNeedsAnObject(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `"x"`, `{"a":`} {
		assert.Error(t, (&Template{}).SetDefinition([]byte(body)), body)
	}
	tmpl := &Template{}
	require.NoError(t, tmpl.SetDefinition([]byte(`{}`)))
	out, err := json.Marshal(tmpl)
	require.NoError(t, err)
	assert.Equal(t, `{"name":""}`, string(out))
}

func TestTemplate_DatabaseRoundTripKeepsTheOrder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "templates.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Template{}))

	tmpl := &Template{Name: "mine"}
	require.NoError(t, tmpl.SetDefinition([]byte(`{"data":{"z":{"value":"1"},"10":{"value":"2"},"a":{"value":"3","x":true}},"run":{"command":"go"}}`)))
	require.NoError(t, db.Save(tmpl).Error)

	loaded := &Template{Name: "mine"}
	require.NoError(t, db.First(loaded).Error)
	assert.Empty(t, loaded.RawValue, "cleared after reading, as before")
	assert.Equal(t, "go", loaded.Execution.Command)
	out, err := json.Marshal(loaded)
	require.NoError(t, err)
	assert.Equal(t, `{"data":{"z":{"value":"1"},"10":{"value":"2"},"a":{"value":"3","x":true}},"run":{"command":"go"},"name":"mine"}`, string(out))

	// saving what was read, untouched, keeps it as written too
	require.NoError(t, db.Save(loaded).Error)
	again := &Template{Name: "mine"}
	require.NoError(t, db.First(again).Error)
	out2, err := json.Marshal(again)
	require.NoError(t, err)
	assert.Equal(t, string(out), string(out2))
}
