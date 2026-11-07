package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupProductForm is a panel with the local template my-paper and node eu-1, and the product routes
func setupProductForm(t *testing.T) (*gorm.DB, *gin.Engine, *models.Node) {
	t.Helper()
	db, r := setupProductsInUse(t)
	r.POST("/products", createProduct)
	r.PUT("/products/:id", updateProduct)
	tmpl := &models.Template{Name: "my-paper"}
	require.NoError(t, tmpl.SetDefinition([]byte(`{"display":"Paper","run":{"command":"java"}}`)))
	require.NoError(t, db.Create(tmpl).Error)
	node := &models.Node{Name: "eu-1", PublicHost: "10.0.0.2", PrivateHost: "10.0.0.2", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "node-secret-0123456789"}
	require.NoError(t, db.Create(node).Error)
	return db, r, node
}

func refusedCode(t *testing.T, w *httptest.ResponseRecorder) (code, msg string) {
	t.Helper()
	var out struct {
		Error struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return out.Error.Code, out.Error.Msg
}

func TestCreateProduct_RefusesWhatProvisioningCouldNotUse(t *testing.T) {
	db, r, node := setupProductForm(t)
	missing := uint(999)

	for _, tc := range []struct {
		name    string
		product gin.H
		code    string
		msg     string
	}{
		{"no template", gin.H{"product_id": "a", "default_node": node.ID}, "ErrProductTemplateRequired", "a product needs a template"},
		{"unknown template", gin.H{"product_id": "a", "template": "paperr", "default_node": node.ID}, "ErrProductTemplateNotFound", "named paperr"},
		{"unknown fallback node", gin.H{"product_id": "a", "template": "my-paper", "default_node": missing}, "ErrProductNodeNotFound", "node 999 does not exist"},
		{"unknown pinned node", gin.H{"product_id": "a", "template": "my-paper", "node_id": missing, "default_node": node.ID}, "ErrProductNodeNotFound", "node 999 does not exist"},
	} {
		w := doJSON(r, http.MethodPost, "/products", tc.product)
		assert.Equal(t, http.StatusBadRequest, w.Code, tc.name)
		code, msg := refusedCode(t, w)
		assert.Equal(t, tc.code, code, tc.name)
		assert.Contains(t, msg, tc.msg, tc.name)
	}
	var count int64
	require.NoError(t, db.Model(&models.ProvisionProduct{}).Count(&count).Error)
	assert.Zero(t, count, "nothing was saved")

	w := doJSON(r, http.MethodPost, "/products", gin.H{"product_id": "minecraft_2gb", "template": "my-paper", "default_node": node.ID})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.Model(&models.ProvisionProduct{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestUpdateProduct_RefusesWhatProvisioningCouldNotUse(t *testing.T) {
	db, r, node := setupProductForm(t)
	product := &models.ProvisionProduct{ProductID: "minecraft_2gb", Template: "my-paper", DefaultNode: node.ID}
	require.NoError(t, db.Create(product).Error)
	path := "/products/" + itoa(product.ID)

	w := doJSON(r, http.MethodPut, path, gin.H{"template": "paperr"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	code, _ := refusedCode(t, w)
	assert.Equal(t, "ErrProductTemplateNotFound", code)

	w = doJSON(r, http.MethodPut, path, gin.H{"default_node": 999})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	code, _ = refusedCode(t, w)
	assert.Equal(t, "ErrProductNodeNotFound", code)

	var stored models.ProvisionProduct
	require.NoError(t, db.First(&stored, product.ID).Error)
	assert.Equal(t, "my-paper", stored.Template, "a refused update changes nothing")
	assert.Equal(t, node.ID, stored.DefaultNode)

	w = doJSON(r, http.MethodPut, path, gin.H{"display_name": "Minecraft 2 GB", "memory": 2048})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&stored, product.ID).Error)
	assert.Equal(t, "Minecraft 2 GB", stored.DisplayName)
}

// With templates.url set and its index unreachable, a template no other repo has may still be
// there: the answer says the index could not be read, not that the template does not exist.
func TestCreateProduct_SaysWhenTheTemplateIndexCannotBeRead(t *testing.T) {
	_, r, node := setupProductForm(t)
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	require.NoError(t, config.TemplatesURL.Set(down.URL+"/index.json", false))

	w := doJSON(r, http.MethodPost, "/products", gin.H{"product_id": "a", "template": "paper", "default_node": node.ID})
	assert.Equal(t, http.StatusBadGateway, w.Code)
	code, msg := refusedCode(t, w)
	assert.Equal(t, "ErrProductTemplateUnreadable", code)
	assert.Contains(t, msg, "could not be read")

	// a local template is found without the index
	w = doJSON(r, http.MethodPost, "/products", gin.H{"product_id": "b", "template": "my-paper", "default_node": node.ID})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// An edit is checked only for what it changes: with the templates.url index down, the limits
// of a product whose template is in the index can still be changed, its template cannot.
func TestUpdateProduct_ChecksOnlyWhatChanges(t *testing.T) {
	db, r, node := setupProductForm(t)
	other := &models.Node{Name: "eu-2", PublicHost: "10.0.0.3", PrivateHost: "10.0.0.3", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "node-secret-0123456789"}
	require.NoError(t, db.Create(other).Error)
	product := &models.ProvisionProduct{ProductID: "minecraft_2gb", Template: "paper", DefaultNode: node.ID}
	require.NoError(t, db.Create(product).Error)
	path := "/products/" + itoa(product.ID)
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	require.NoError(t, config.TemplatesURL.Set(down.URL+"/index.json", false))

	w := doJSON(r, http.MethodPut, path, gin.H{"cpu": 200, "memory": 4096, "display_name": "Minecraft 4 GB"})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doJSON(r, http.MethodPut, path, gin.H{"default_node": other.ID})
	assert.Equal(t, http.StatusOK, w.Code, "a node that exists, the template untouched: %s", w.Body.String())

	var stored models.ProvisionProduct
	require.NoError(t, db.First(&stored, product.ID).Error)
	assert.Equal(t, 200, stored.CPU)
	assert.Equal(t, other.ID, stored.DefaultNode)

	w = doJSON(r, http.MethodPut, path, gin.H{"template": "rust"})
	assert.Equal(t, http.StatusBadGateway, w.Code)
	code, _ := refusedCode(t, w)
	assert.Equal(t, "ErrProductTemplateUnreadable", code)
	require.NoError(t, db.First(&stored, product.ID).Error)
	assert.Equal(t, "paper", stored.Template)
}
