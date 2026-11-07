package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerProducts(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeAdmin), getProducts)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeAdmin), createProduct)
	g.Handle("PUT", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), updateProduct)
	g.Handle("DELETE", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), deleteProduct)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))
	g.Handle("OPTIONS", "/:id", response.CreateOptions("PUT", "DELETE"))
}

func getProducts(c *gin.Context) {
	db := middleware.GetDatabase(c)

	var products []*models.ProvisionProduct
	err := db.Find(&products).Error
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, products)
}

func createProduct(c *gin.Context) {
	db := middleware.GetDatabase(c)

	var req models.ProvisionProduct
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if !validateProduct(c, db, &req) {
		return
	}

	err := db.Create(&req).Error
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, req)
}

func updateProduct(c *gin.Context) {
	db := middleware.GetDatabase(c)

	id, err := strconv.Atoi(c.Param("id"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	var product models.ProvisionProduct
	err = db.First(&product, id).Error
	if response.HandleError(c, err, http.StatusNotFound) {
		return
	}

	if err := c.ShouldBindJSON(&product); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	product.ID = uint(id)
	if !validateProduct(c, db, &product) {
		return
	}
	err = db.Save(&product).Error
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, product)
}

func deleteProduct(c *gin.Context) {
	db := middleware.GetDatabase(c)

	id, err := strconv.Atoi(c.Param("id"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = db.Delete(&models.ProvisionProduct{}, id).Error
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// validateProduct refuses a product provisioning could not use: a template no repo has, looked
// up the way provisioning does, or a node (the pinned one, else the fallback) that does not
// exist. False when a response has been written.
func validateProduct(c *gin.Context, db *gorm.DB, product *models.ProvisionProduct) bool {
	if strings.TrimSpace(product.Template) == "" {
		response.HandleError(c, skypanel.ErrProductTemplateRequired, http.StatusBadRequest)
		return false
	}
	if err := lookupProductTemplate(&services.Template{DB: db}, product.Template); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, skypanel.ErrProductTemplateNotFound("")):
			status = http.StatusBadRequest
		case errors.Is(err, skypanel.ErrProductTemplateUnreadable("", "")):
			// the request may be right: the repo (the templates.url index) could not be asked
			status = http.StatusBadGateway
		}
		response.HandleError(c, err, status)
		return false
	}

	nodeID := provisionNodeID(product)
	_, err := (&services.Node{DB: db}).Get(nodeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.HandleError(c, skypanel.ErrProductNodeNotFound(nodeID), http.StatusBadRequest)
		return false
	}
	return !response.HandleError(c, err, http.StatusInternalServerError)
}

// lookupProductTemplate is nil when a repo provisioning reads has the template (see
// findTemplateInRepos). Otherwise it tells a template no repo has from one a repo that could
// not be read may have.
func lookupProductTemplate(ts *services.Template, name string) error {
	repos, err := ts.GetRepos()
	if err != nil {
		return err
	}
	var unreadable *models.TemplateRepo
	for _, repo := range repos {
		template, err := ts.Get(repo.ID, name)
		switch {
		case err == nil && template != nil:
			return nil
		case err == nil, errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, skypanel.ErrNoTemplate("")):
		case repo.IsLocal:
			// the panel's own database
			return err
		default:
			logging.Error.Printf("Could not look for template %s in repo %s: %s", name, repo.Name, err)
			if unreadable == nil {
				unreadable = repo
			}
		}
	}
	if unreadable != nil {
		return skypanel.ErrProductTemplateUnreadable(name, unreadable.Name)
	}
	return skypanel.ErrProductTemplateNotFound(name)
}
