package database

import (
	"net/http"
	"strconv"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/gin-gonic/gin"
)

func registerDatabaseHosts(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeAdmin), getAllDatabaseHosts)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeAdmin), createDatabaseHost)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))

	g.Handle("GET", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), getDatabaseHost)
	g.Handle("PUT", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), updateDatabaseHost)
	g.Handle("DELETE", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), deleteDatabaseHost)
	g.Handle("OPTIONS", "/:id", response.CreateOptions("PUT", "GET", "DELETE"))
}

// @Summary Get database hosts
// @Description Gets all database hosts registered to the panel
// @Success 200 {array} domain.DatabaseHostView "Database Hosts"
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Database Hosts
// @Router /api/databasehosts [get]
// @Security OAuth2Application[admin]
func getAllDatabaseHosts(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	dhs := &database.DatabaseHostRepo{DB: db}

	var hosts []*domain.DatabaseHost
	if hosts, err = dhs.GetAll(); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data := make([]*domain.DatabaseHostView, len(hosts))
	for i, host := range hosts {
		data[i] = &domain.DatabaseHostView{
			ID:           host.ID,
			Name:         host.Name,
			Host:         host.Host,
			Port:         host.Port,
			Username:     host.Username,
			MaxDatabases: host.MaxDatabases,
			NodeID:       host.NodeID,
			CreatedAt:    host.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:    host.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	c.JSON(http.StatusOK, data)
}

// @Summary Get database host
// @Description Gets information about a single database host
// @Success 200 {object} domain.DatabaseHostView "Database Host"
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path string true "Database Host Id"
// @Tags Database Hosts
// @Router /api/databasehosts/{id} [get]
// @Security OAuth2Application[admin]
func getDatabaseHost(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	dhs := &database.DatabaseHostRepo{DB: db}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	host, err := dhs.Get(uint(id))
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data := &domain.DatabaseHostView{
		ID:           host.ID,
		Name:         host.Name,
		Host:         host.Host,
		Port:         host.Port,
		Username:     host.Username,
		MaxDatabases: host.MaxDatabases,
		NodeID:       host.NodeID,
		CreatedAt:    host.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    host.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	c.JSON(http.StatusOK, data)
}

// @Summary Create database host
// @Description Creates a database host
// @Success 200 {object} domain.DatabaseHostView "Database Host created"
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param databaseHost body domain.DatabaseHostCreate true "Database Host information"
// @Tags Database Hosts
// @Router /api/databasehosts [post]
// @Security OAuth2Application[admin]
func createDatabaseHost(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	dhs := &database.DatabaseHostRepo{DB: db}

	model := &domain.DatabaseHostCreate{}
	if err = c.BindJSON(model); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	create := &domain.DatabaseHost{
		Name:         model.Name,
		Host:         model.Host,
		Port:         model.Port,
		Username:     model.Username,
		Password:     model.Password,
		MaxDatabases: model.MaxDatabases,
		NodeID:       model.NodeID,
	}

	if err = dhs.Create(create); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data := &domain.DatabaseHostView{
		ID:           create.ID,
		Name:         create.Name,
		Host:         create.Host,
		Port:         create.Port,
		Username:     create.Username,
		MaxDatabases: create.MaxDatabases,
		NodeID:       create.NodeID,
		CreatedAt:    create.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    create.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	c.JSON(http.StatusOK, data)
}

// @Summary Update database host
// @Description Updates a database host with given information
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path string true "Database Host Id"
// @Param databaseHost body domain.DatabaseHostUpdate true "Database Host information"
// @Tags Database Hosts
// @Router /api/databasehosts/{id} [put]
// @Security OAuth2Application[admin]
func updateDatabaseHost(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	dhs := &database.DatabaseHostRepo{DB: db}

	viewModel := &domain.DatabaseHostUpdate{}
	if err = c.BindJSON(viewModel); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	existing, err := dhs.Get(uint(id))
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	existing.Name = viewModel.Name
	existing.Host = viewModel.Host
	existing.Port = viewModel.Port
	existing.Username = viewModel.Username
	if viewModel.Password != "" {
		existing.Password = viewModel.Password
	}
	existing.MaxDatabases = viewModel.MaxDatabases
	existing.NodeID = viewModel.NodeID

	if err = dhs.Update(existing); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Delete database host
// @Description Deletes a database host
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path string true "Database Host Id"
// @Tags Database Hosts
// @Router /api/databasehosts/{id} [delete]
// @Security OAuth2Application[admin]
func deleteDatabaseHost(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	dhs := &database.DatabaseHostRepo{DB: db}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if err = dhs.Delete(uint(id)); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}
