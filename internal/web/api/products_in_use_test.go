package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProductsInUse(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	previousURL := config.TemplatesURL.Value()
	require.NoError(t, config.TemplatesURL.Set("", false))
	t.Cleanup(func() { _ = config.TemplatesURL.Set(previousURL, false) })

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "products.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}, &models.Server{}, &models.ProvisionProduct{}, &models.Template{}, &models.TemplateRepo{}))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.DELETE("/nodes/:id", deleteNode)
	r.DELETE("/templates/0/:name", deleteTemplate)
	return db, r
}

// refusedProducts reads the code and the products of a refused delete
func refusedProducts(t *testing.T, body []byte) (string, []string) {
	t.Helper()
	var out struct {
		Error struct {
			Code     string `json:"code"`
			Metadata struct {
				Products []string `json:"products"`
			} `json:"metadata"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	return out.Error.Code, out.Error.Metadata.Products
}

func TestDeleteNode_ConflictWhileAProductUsesIt(t *testing.T) {
	db, r := setupProductsInUse(t)
	node := &models.Node{Name: "eu-1", PublicHost: "10.0.0.2", PrivateHost: "10.0.0.2", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "node-secret-0123456789"}
	require.NoError(t, db.Create(node).Error)
	product := &models.ProvisionProduct{ProductID: "minecraft_2gb", Template: "paper", NodeID: &node.ID}
	require.NoError(t, db.Create(product).Error)

	w := doJSON(r, http.MethodDelete, "/nodes/"+itoa(node.ID), nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	code, products := refusedProducts(t, w.Body.Bytes())
	assert.Equal(t, "ErrNodeHasProducts", code)
	assert.Equal(t, []string{"minecraft_2gb"}, products)

	require.NoError(t, db.Delete(product).Error)
	w = doJSON(r, http.MethodDelete, "/nodes/"+itoa(node.ID), nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteTemplate_ConflictWhileAProductUsesIt(t *testing.T) {
	db, r := setupProductsInUse(t)
	tmpl := &models.Template{Name: "my-paper"}
	require.NoError(t, tmpl.SetDefinition([]byte(`{"display":"Paper","run":{"command":"java"}}`)))
	require.NoError(t, db.Create(tmpl).Error)
	product := &models.ProvisionProduct{ProductID: "minecraft_2gb", Template: "my-paper"}
	require.NoError(t, db.Create(product).Error)

	w := doJSON(r, http.MethodDelete, "/templates/0/my-paper", nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	code, products := refusedProducts(t, w.Body.Bytes())
	assert.Equal(t, "ErrTemplateInUse", code)
	assert.Equal(t, []string{"minecraft_2gb"}, products)

	require.NoError(t, db.Delete(product).Error)
	w = doJSON(r, http.MethodDelete, "/templates/0/my-paper", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}
