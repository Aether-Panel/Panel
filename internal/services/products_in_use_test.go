package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func addProduct(t *testing.T, db *gorm.DB, id, template string, nodeID *uint, defaultNode uint) {
	t.Helper()
	require.NoError(t, db.Create(&models.ProvisionProduct{ProductID: id, Template: template, NodeID: nodeID, DefaultNode: defaultNode}).Error)
}

func productsOf(t *testing.T, err error) []string {
	t.Helper()
	var panelErr *skypanel.Error
	require.ErrorAs(t, err, &panelErr)
	products, _ := panelErr.Meta["products"].([]string)
	return products
}

func TestNodeDelete_RefusedWhileAProductCreatesServersOnIt(t *testing.T) {
	db := setupDeleteCleanupDB(t, &models.ProvisionProduct{})
	ns := &Node{DB: db}
	node := &models.Node{Name: "eu-1", PublicHost: "10.0.0.2", PrivateHost: "10.0.0.2", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "node-secret-0123456789"}
	require.NoError(t, db.Create(node).Error)
	other := uint(99)
	zero := uint(0)
	addProduct(t, db, "rust_4gb", "rust", &node.ID, 0)
	addProduct(t, db, "minecraft_2gb", "paper", nil, node.ID)
	addProduct(t, db, "terraria_1gb", "terraria", &zero, node.ID)
	// pinned elsewhere: its fallback node is never used
	addProduct(t, db, "valheim_4gb", "valheim", &other, node.ID)

	err := ns.Delete(node.ID)

	require.ErrorIs(t, err, skypanel.ErrNodeHasProducts(nil))
	assert.Equal(t, []string{"minecraft_2gb", "rust_4gb", "terraria_1gb"}, productsOf(t, err))
	var count int64
	require.NoError(t, db.Model(&models.Node{}).Where("id = ?", node.ID).Count(&count).Error)
	assert.EqualValues(t, 1, count, "the node is kept")

	require.NoError(t, db.Where("product_id <> ?", "valheim_4gb").Delete(&models.ProvisionProduct{}).Error)
	require.NoError(t, ns.Delete(node.ID), "no product uses it any more")
}

func addLocalTemplate(t *testing.T, db *gorm.DB, name string) {
	t.Helper()
	tmpl := &models.Template{Name: name}
	require.NoError(t, tmpl.SetDefinition([]byte(authoredTemplate)))
	require.NoError(t, db.Create(tmpl).Error)
}

// addRepoWithTemplates stores a Git repo whose checkout, fresh in the cache, holds the templates
func addRepoWithTemplates(t *testing.T, db *gorm.DB, name string, templates ...string) *models.TemplateRepo {
	t.Helper()
	repo := &models.TemplateRepo{Name: name, URL: "https://example.com/" + name + ".git", Branch: "main"}
	require.NoError(t, db.Create(repo).Error)
	checkout := repoCheckoutPath(repo.ID)
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, "games"), 0o755))
	for _, tmpl := range templates {
		require.NoError(t, os.WriteFile(filepath.Join(checkout, "games", tmpl+".json"), []byte(authoredTemplate), 0o600))
	}
	pathLock.Lock()
	existingPaths[repo.ID] = repoCache{LastRefreshed: time.Now(), Path: checkout}
	pathLock.Unlock()
	t.Cleanup(func() {
		pathLock.Lock()
		delete(existingPaths, repo.ID)
		pathLock.Unlock()
	})
	return repo
}

func setupTemplatesInUse(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, config.CacheFolder.Set(t.TempDir(), false))
	t.Cleanup(func() { _ = config.CacheFolder.Set("cache", false) })
	previousURL := config.TemplatesURL.Value()
	require.NoError(t, config.TemplatesURL.Set("", false))
	t.Cleanup(func() { _ = config.TemplatesURL.Set(previousURL, false) })
	return setupDeleteCleanupDB(t, &models.ProvisionProduct{}, &models.Template{}, &models.TemplateRepo{})
}

func TestTemplateDelete_RefusedWhileAProductUsesIt(t *testing.T) {
	db := setupTemplatesInUse(t)
	ts := &Template{DB: db}
	addLocalTemplate(t, db, "my-paper")
	addLocalTemplate(t, db, "spare")
	addProduct(t, db, "minecraft_2gb", "my-paper", nil, 0)
	addProduct(t, db, "minecraft_4gb", "my-paper", nil, 0)

	err := ts.Delete("my-paper")

	require.ErrorIs(t, err, skypanel.ErrTemplateInUse(nil))
	assert.Equal(t, []string{"minecraft_2gb", "minecraft_4gb"}, productsOf(t, err))
	_, err = ts.Get(0, "my-paper")
	require.NoError(t, err, "the template is kept")

	require.NoError(t, ts.Delete("spare"), "a template no product uses goes")
	require.ErrorIs(t, ts.Delete("spare"), gorm.ErrRecordNotFound)
}

// Provisioning finds a template by name in any repo: a local copy of a template a repo also
// has can go, the products keep finding it there.
func TestTemplateDelete_AllowedWhenAnotherRepoHasTheTemplate(t *testing.T) {
	db := setupTemplatesInUse(t)
	ts := &Template{DB: db}
	addRepoWithTemplates(t, db, "community", "paper")
	addLocalTemplate(t, db, "paper")
	addProduct(t, db, "minecraft_2gb", "paper", nil, 0)

	require.NoError(t, ts.Delete("paper"))
}

func TestDeleteRepo_RefusedWhileItIsTheOnlyRepoWithAProductsTemplate(t *testing.T) {
	db := setupTemplatesInUse(t)
	ts := &Template{DB: db}
	community := addRepoWithTemplates(t, db, "community", "paper", "rust")
	addRepoWithTemplates(t, db, "mirror", "rust")
	addLocalTemplate(t, db, "terraria")
	addProduct(t, db, "minecraft_2gb", "paper", nil, 0)
	addProduct(t, db, "rust_4gb", "rust", nil, 0)
	addProduct(t, db, "terraria_1gb", "terraria", nil, 0)

	err := ts.DeleteRepo(community.ID)

	require.ErrorIs(t, err, skypanel.ErrTemplateRepoInUse(nil))
	assert.Equal(t, []string{"minecraft_2gb"}, productsOf(t, err), "rust is in another repo, terraria is local")
	assert.DirExists(t, repoCheckoutPath(community.ID), "the repo is kept")

	addLocalTemplate(t, db, "paper")
	require.NoError(t, ts.DeleteRepo(community.ID), "the local paper takes over")
}

// With templates.url set the panel reads none of the stored repos.
func TestDeleteRepo_AllowedWhenThePanelReadsTemplatesFromAnURL(t *testing.T) {
	db := setupTemplatesInUse(t)
	ts := &Template{DB: db}
	repo := addRepoWithTemplates(t, db, "community", "paper")
	addProduct(t, db, "minecraft_2gb", "paper", nil, 0)
	require.NoError(t, config.TemplatesURL.Set("http://127.0.0.1:1/index.json", false))

	require.NoError(t, ts.DeleteRepo(repo.ID))
}
