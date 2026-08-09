package product

import (
	"net/http"
	"strconv"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/gin-gonic/gin"
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

	var products []*domain.ProvisionProduct
	err := db.Find(&products).Error
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, products)
}

func createProduct(c *gin.Context) {
	db := middleware.GetDatabase(c)

	var req domain.ProvisionProduct
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
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

	var product domain.ProvisionProduct
	err = db.First(&product, id).Error
	if response.HandleError(c, err, http.StatusNotFound) {
		return
	}

	if err := c.ShouldBindJSON(&product); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	product.ID = uint(id)
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

	err = db.Delete(&domain.ProvisionProduct{}, id).Error
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}
