package database

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/gin-gonic/gin"
)

func registerDatabases(g *gin.RouterGroup) {
	g.Handle("GET", "/:serverId/databases", middleware.RequiresPermission(scopes.ScopeServerView), getAllDatabasesForServer)
	g.Handle("POST", "/:serverId/databases", middleware.RequiresPermission(scopes.ScopeServerEditData), createDatabase)
	g.Handle("OPTIONS", "/:serverId/databases", response.CreateOptions("GET", "POST"))

	g.Handle("DELETE", "/:serverId/databases/:id", middleware.RequiresPermission(scopes.ScopeServerEditData), deleteDatabase)
	g.Handle("OPTIONS", "/:serverId/databases/:id", response.CreateOptions("DELETE"))
}

// @Summary Get databases for server
// @Description Gets all databases for a specific server
// @Success 200 {array} domain.DatabaseView "Databases"
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param serverId path string true "Server Id"
// @Tags Databases
// @Router /api/servers/{serverId}/databases [get]
// @Security OAuth2Application[server.view]
func getAllDatabasesForServer(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	ds := &database.DatabaseRepo{DB: db}

	serverID := c.Param("serverId")

	var databases []*domain.Database
	if databases, err = ds.GetAllForServer(serverID); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data := make([]*domain.DatabaseView, len(databases))
	for i, database := range databases {
		var hostName string
		var host string
		var port uint16

		if database.DatabaseHost != nil {
			hostName = database.DatabaseHost.Name
			host = database.DatabaseHost.Host
			port = database.DatabaseHost.Port
		}

		data[i] = &domain.DatabaseView{
			ID:               database.ID,
			ServerID:         database.ServerID,
			DatabaseHostID:   database.DatabaseHostID,
			DatabaseName:     database.DatabaseName,
			Username:         database.Username,
			Password:         database.Password,
			RemoteConnection: database.RemoteConnection,
			MaxConnections:   database.MaxConnections,
			Host:             host,
			Port:             port,
			HostName:         hostName,
			CreatedAt:        database.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:        database.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	c.JSON(http.StatusOK, data)
}

// @Summary Create database
// @Description Creates a database for a server
// @Success 200 {object} domain.DatabaseView "Database created"
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param serverId path string true "Server Id"
// @Param database body domain.DatabaseCreate true "Database information"
// @Tags Databases
// @Router /api/servers/{serverId}/databases [post]
// @Security OAuth2Application[server.data.edit]
func createDatabase(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	ds := &database.DatabaseRepo{DB: db}

	serverID := c.Param("serverId")

	model := &domain.DatabaseCreate{}
	if err = c.BindJSON(model); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	create := &domain.Database{
		ServerID:       serverID,
		DatabaseHostID: model.DatabaseHostID,
		DatabaseName:   model.DatabaseName,
	}

	if err = ds.Create(create); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Recargar con relaciones
	created, err := ds.Get(create.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	var hostName string
	var host string
	var port uint16

	if created.DatabaseHost != nil {
		hostName = created.DatabaseHost.Name
		host = created.DatabaseHost.Host
		port = created.DatabaseHost.Port
	}

	data := &domain.DatabaseView{
		ID:               created.ID,
		ServerID:         created.ServerID,
		DatabaseHostID:   created.DatabaseHostID,
		DatabaseName:     created.DatabaseName,
		Username:         created.Username,
		Password:         created.Password,
		RemoteConnection: created.RemoteConnection,
		MaxConnections:   created.MaxConnections,
		Host:             host,
		Port:             port,
		HostName:         hostName,
		CreatedAt:        created.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        created.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	c.JSON(http.StatusOK, data)
}

// @Summary Delete database
// @Description Deletes a database (and its MySQL user)
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param serverId path string true "Server Id"
// @Param id path string true "Database Id"
// @Tags Databases
// @Router /api/servers/{serverId}/databases/{id} [delete]
// @Security OAuth2Application[server.data.edit]
func deleteDatabase(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	ds := &database.DatabaseRepo{DB: db}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// Verificar que la base de datos pertenece al servidor
	database, err := ds.Get(uint(id))
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	serverID := c.Param("serverId")
	if database.ServerID != serverID {
		response.HandleError(c, fmt.Errorf("database does not belong to this server"), http.StatusForbidden)
		return
	}

	if err = ds.Delete(uint(id)); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}
