package extensions

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const helloManifest = `{
  "id": "hello",
  "name": "Hello",
  "version": "1.0.0",
  "pages": [{"id": "main", "title": "Hello", "placement": "server", "path": "index.html"}],
  "webhook": "https://hooks.example.com/aether",
  "events": ["server.started", "server.crashed"]
}`

func setup(t *testing.T) *gorm.DB {
	dir := t.TempDir()
	previous := config.ExtensionsFolder.Value()
	_ = config.ExtensionsFolder.Set(filepath.Join(dir, "extensions"), false)
	t.Cleanup(func() { _ = config.ExtensionsFolder.Set(previous, false) })

	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "ext.db")), &gorm.Config{})
	if !assert.NoError(t, err) || !assert.NoError(t, db.AutoMigrate(&models.Extension{})) {
		t.FailNow()
	}
	t.Cleanup(func() {
		activeMu.Lock()
		active, loaded = nil, false
		activeMu.Unlock()
	})
	return db
}

func zipOf(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		assert.NoError(t, err)
		_, _ = w.Write([]byte(content))
	}
	assert.NoError(t, zw.Close())
	return buf.Bytes()
}

func install(db *gorm.DB, data []byte) (*models.Extension, error) {
	return Install(db, bytes.NewReader(data), int64(len(data)))
}

func TestInstall_UnpacksAndStartsSwitchedOff(t *testing.T) {
	db := setup(t)
	ext, err := install(db, zipOf(t, map[string]string{
		"extension.json": helloManifest,
		"index.html":     "<h1>hi</h1>",
		"js/app.js":      "console.log(1)",
	}))
	if !assert.NoError(t, err) {
		return
	}
	assert.False(t, ext.Enabled)
	assert.Len(t, ext.Secret, 64)
	assert.Equal(t, "https://hooks.example.com/aether", ext.WebhookURL)

	content, err := os.ReadFile(filepath.Join(Folder("hello"), "js", "app.js"))
	assert.NoError(t, err)
	assert.Equal(t, "console.log(1)", string(content))
}

func TestInstall_AcceptsAZippedFolder(t *testing.T) {
	db := setup(t)
	_, err := install(db, zipOf(t, map[string]string{
		"hello/extension.json": helloManifest,
		"hello/index.html":     "<h1>hi</h1>",
	}))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(Folder("hello"), "index.html"))
	assert.NoError(t, err)
}

func TestInstall_RefusesBadPackages(t *testing.T) {
	db := setup(t)
	cases := map[string]map[string]string{
		"no manifest":       {"index.html": "x"},
		"escaping path":     {"extension.json": helloManifest, "index.html": "x", "../evil.sh": "x"},
		"page not in zip":   {"extension.json": helloManifest},
		"unknown event":     {"extension.json": `{"id":"hello","name":"H","version":"1","webhook":"https://a.example","events":["user.deleted"]}`},
		"plain http hook":   {"extension.json": `{"id":"hello","name":"H","version":"1","webhook":"http://a.example","events":["server.started"]}`},
		"bad id":            {"extension.json": `{"id":"Hello World","name":"H","version":"1","webhook":"https://a.example"}`},
		"unknown field":     {"extension.json": `{"id":"hello","name":"H","version":"1","webhook":"https://a.example","exec":"rm -rf /"}`},
		"page outside root": {"extension.json": `{"id":"hello","name":"H","version":"1","pages":[{"id":"a","title":"A","placement":"server","path":"../a.html"}]}`},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := install(db, zipOf(t, files))
			assert.Error(t, err)
		})
	}
	_, err := os.Stat(filepath.Join(filepath.Dir(Folder("hello")), "evil.sh"))
	assert.True(t, os.IsNotExist(err), "nothing is written outside the extension's folder")
}

func TestInstall_UpdateKeepsSwitchSecretAndChangedWebhook(t *testing.T) {
	db := setup(t)
	files := map[string]string{"extension.json": helloManifest, "index.html": "v1"}
	first, err := install(db, zipOf(t, files))
	if !assert.NoError(t, err) {
		return
	}
	assert.NoError(t, db.Model(first).Updates(map[string]interface{}{"enabled": true, "webhook_url": "https://mine.example.com/hook"}).Error)

	files["index.html"] = "v2"
	second, err := install(db, zipOf(t, files))
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, second.Enabled)
	assert.Equal(t, first.Secret, second.Secret)
	assert.Equal(t, "https://mine.example.com/hook", second.WebhookURL)
	content, _ := os.ReadFile(filepath.Join(Folder("hello"), "index.html"))
	assert.Equal(t, "v2", string(content))
}

func TestUninstall_RemovesFilesAndRecord(t *testing.T) {
	db := setup(t)
	_, err := install(db, zipOf(t, map[string]string{"extension.json": helloManifest, "index.html": "x"}))
	if !assert.NoError(t, err) {
		return
	}
	assert.NoError(t, Uninstall(db, "hello"))
	_, err = os.Stat(Folder("hello"))
	assert.True(t, os.IsNotExist(err))
	assert.ErrorIs(t, Uninstall(db, "hello"), ErrNotFound)
}

func TestPublish_OnlySwitchedOnSubscribers(t *testing.T) {
	db := setup(t)
	ext, err := install(db, zipOf(t, map[string]string{"extension.json": helloManifest, "index.html": "x"}))
	if !assert.NoError(t, err) {
		return
	}

	var mu sync.Mutex
	var got []map[string]interface{}
	previous := send
	send = func(_, secret string, payload interface{}) error {
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, ext.Secret, secret)
		got = append(got, payload.(map[string]interface{}))
		return nil
	}
	t.Cleanup(func() { send = previous })

	Publish(EventServerStarted, ServerData("abc", "Survival"))
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Empty(t, got, "a switched-off extension gets nothing")
	mu.Unlock()

	assert.NoError(t, db.Model(ext).Update("enabled", true).Error)
	Reload(db)
	Publish(EventServerStarted, ServerData("abc", "Survival"))
	Publish(EventServerDeleted, ServerData("abc", "Survival"))

	assert.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 1 }, time.Second, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, got, 1, "only the events it subscribed to")
	assert.Equal(t, EventServerStarted, got[0]["event"])
	assert.Equal(t, "hello", got[0]["extension"])
	assert.Equal(t, map[string]string{"id": "abc", "name": "Survival"}, got[0]["server"])
}
