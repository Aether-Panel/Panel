package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNodeJoinCodes_WorkOnceAndForAnHour(t *testing.T) {
	codes := newNodeJoinCodes()
	now := time.Now()
	codes.now = func() time.Time { return now }

	code, expires, err := codes.issue(7)
	require.NoError(t, err)
	assert.Len(t, code, 32, "160 random bits")
	assert.Equal(t, now.Add(time.Hour), expires)

	id, ok := codes.redeem(" " + strings.ToUpper(code) + " ")
	assert.True(t, ok, "pasted with spaces or in capitals it still works")
	assert.Equal(t, uint(7), id)
	_, ok = codes.redeem(code)
	assert.False(t, ok, "a code works once")

	old, _, _ := codes.issue(7)
	fresh, _, _ := codes.issue(7)
	_, ok = codes.redeem(old)
	assert.False(t, ok, "a new code for the node replaces the old one")

	now = now.Add(61 * time.Minute)
	_, ok = codes.redeem(fresh)
	assert.False(t, ok, "after an hour it no longer works")
	_, ok = codes.redeem("")
	assert.False(t, ok)
}

func nodeJoinRouter(t *testing.T, masterURL string) (*gin.Engine, *models.Node) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "join.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}))
	node := &models.Node{Name: "eu-2", PublicHost: "node2.example.com", PrivateHost: "10.0.0.9", PublicPort: 8443, PrivatePort: 8081, SFTPPort: 5658, Secret: "node-secret-0123456789"}
	require.NoError(t, db.Create(node).Error)

	previous := config.MasterURL.Value()
	require.NoError(t, config.MasterURL.Set(masterURL, false))
	t.Cleanup(func() { _ = config.MasterURL.Set(previous, false) })
	previousCodes := joinCodes
	joinCodes = newNodeJoinCodes()
	t.Cleanup(func() { joinCodes = previousCodes })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.POST("/nodes/:id/join", createNodeJoinCode)
	r.POST(NodeJoinPath, joinNode)
	return r, node
}

func TestNodeJoin_TheCommandBringsTheNodeItsCredentials(t *testing.T) {
	r, node := nodeJoinRouter(t, "panel.example.com/")

	w := doJSON(r, http.MethodPost, "/nodes/"+itoa(node.ID)+"/join", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var issued nodeJoinCodeView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &issued))
	assert.Equal(t, "https://panel.example.com", issued.PanelURL)
	assert.Equal(t, "curl -fsSL "+nodeJoinInstaller+" | sudo bash -s -- --node --panel https://panel.example.com --join "+issued.Code, issued.Command)

	w = doJSON(r, http.MethodPost, NodeJoinPath, map[string]string{"code": issued.Code})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var joined nodeJoinView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &joined))
	assert.Equal(t, nodeJoinView{NodeID: node.ID, Name: "eu-2", ClientID: ".node_" + itoa(node.ID), ClientSecret: "node-secret-0123456789", Port: 8081, SFTPPort: 5658}, joined)

	w = doJSON(r, http.MethodPost, NodeJoinPath, map[string]string{"code": issued.Code})
	assert.Equal(t, http.StatusNotFound, w.Code, "the code is used up")
	assert.Contains(t, w.Body.String(), "ErrNodeJoinCode")
}

func TestNodeJoin_Refusals(t *testing.T) {
	r, node := nodeJoinRouter(t, "http://localhost:8080")

	w := doJSON(r, http.MethodPost, "/nodes/"+itoa(node.ID)+"/join", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "a node on another machine cannot reach localhost")
	assert.Contains(t, w.Body.String(), "ErrNodeJoinURL")

	require.NoError(t, config.MasterURL.Set("https://panel.example.com", false))
	w = doJSON(r, http.MethodPost, "/nodes/0/join", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "the panel's own node")
	assert.Contains(t, w.Body.String(), "ErrNodeJoinLocal")
	w = doJSON(r, http.MethodPost, "/nodes/999/join", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	previous := nodeJoinAttempts
	t.Cleanup(func() { nodeJoinAttempts = previous })
	nodeJoinAttempts = services.NewAttemptLimiter(2, time.Hour)
	for range 2 {
		w = doJSON(r, http.MethodPost, NodeJoinPath, map[string]string{"code": "guess"})
		assert.Equal(t, http.StatusNotFound, w.Code)
	}
	w = doJSON(r, http.MethodPost, NodeJoinPath, map[string]string{"code": "guess"})
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "codes cannot be guessed")
}
