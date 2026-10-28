package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func readDomainStatus(t *testing.T, dir string) (int, DomainStatus) {
	previous := viper.ConfigFileUsed()
	viper.SetConfigFile(filepath.Join(dir, "config.json"))
	t.Cleanup(func() { viper.SetConfigFile(previous) })

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	domainStatus(c)

	var status DomainStatus
	_ = json.Unmarshal(w.Body.Bytes(), &status)
	return w.Code, status
}

func TestDomainStatus_NoneWithoutTheService(t *testing.T) {
	code, status := readDomainStatus(t, t.TempDir())

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "none", status.State)
}

func TestDomainStatus_ReadsWhatTheServiceWrote(t *testing.T) {
	dir := t.TempDir()
	written := `{"state":"ok","url":"https://panel.example.com","domain":"panel.example.com","https":true,"message":"listo","updatedAt":"2026-10-28T03:00:00Z"}`
	assert.NoError(t, os.WriteFile(filepath.Join(dir, domainStatusFile), []byte(written), 0o644))

	code, status := readDomainStatus(t, dir)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, DomainStatus{State: "ok", URL: "https://panel.example.com", Domain: "panel.example.com", HTTPS: true, Message: "listo", UpdatedAt: "2026-10-28T03:00:00Z"}, status)
}
