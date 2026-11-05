package api

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func systemInfoOf(t *testing.T, node *models.Node) (int, string) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "system.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}))
	node.Secret = "node-secret-0123456789"
	node.SFTPPort = 5658
	require.NoError(t, db.Create(node).Error)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.GET("/nodes/:id/system", getSystemInfo)
	w := doJSON(r, http.MethodGet, "/nodes/"+itoa(node.ID)+"/system", nil)
	return w.Code, w.Body.String()
}

// A node that refuses the panel (it could not read the panel's key, or has another secret)
// answers with an error. The panel passed that error on as a 200, so the nodes page showed
// the node online with no metrics instead of saying something is wrong.
func TestGetSystemInfo_ANodeRefusalIsNotItsMetrics(t *testing.T) {
	code, body := systemInfoOf(t, fakeRemoteNode(t, http.StatusUnauthorized, nil))
	assert.Equal(t, http.StatusBadGateway, code)
	assert.Contains(t, body, "ErrNodeAuthFailed")

	code, _ = systemInfoOf(t, fakeRemoteNode(t, http.StatusInternalServerError, nil))
	assert.Equal(t, http.StatusBadGateway, code, "any other failure of the node is not metrics either")

	code, _ = systemInfoOf(t, fakeRemoteNode(t, http.StatusOK, nil))
	assert.Equal(t, http.StatusOK, code, "a node that answers gives its metrics")
}
