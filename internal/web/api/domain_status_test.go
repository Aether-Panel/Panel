package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func readDomainStatus(t *testing.T, dir string) (int, services.DomainStatus) {
	previous := viper.ConfigFileUsed()
	viper.SetConfigFile(filepath.Join(dir, "config.json"))
	t.Cleanup(func() { viper.SetConfigFile(previous) })

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	domainStatus(c)

	var status services.DomainStatus
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
	written := `{"state":"ok","url":"https://panel.example.com","domain":"panel.example.com","https":true,"proxied":true,"serverIp":"203.0.113.7","message":"listo","updatedAt":"2026-10-28T03:00:00Z"}`
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "domain-status.json"), []byte(written), 0o644))

	code, status := readDomainStatus(t, dir)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, services.DomainStatus{State: "ok", URL: "https://panel.example.com", Domain: "panel.example.com", HTTPS: true, Proxied: true, ServerIP: "203.0.113.7", Message: "listo", UpdatedAt: "2026-10-28T03:00:00Z"}, status)
}

func TestCurrentGitBranch_FallsBackToMain(t *testing.T) {
	// the container has no /repo/.git in tests: the stable branch is used
	assert.Equal(t, "main", currentGitBranch())
	assert.True(t, updateBranchPattern.MatchString("fide-dev2.0"))
	assert.False(t, updateBranchPattern.MatchString("main?x=1"))
}
