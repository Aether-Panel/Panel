package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/middleware"
	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/SkyPanel/SkyPanel/v3/response"
	"github.com/SkyPanel/SkyPanel/v3/scopes"
	"github.com/SkyPanel/SkyPanel/v3/services"
	"github.com/SkyPanel/SkyPanel/v3/utils"
)

func registerUptime(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeAdmin), getAllUptime)
	g.Handle("GET", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerView), middleware.ResolveServerPanel, getServerUptime)
	g.Handle("OPTIONS", "", response.CreateOptions("GET"))
	g.Handle("OPTIONS", "/:serverId", response.CreateOptions("GET"))
}

// @Summary Get all servers uptime
// @Description Gets uptime statistics for all servers
// @Success 200 {object} map[string]interface{} "Uptime statistics"
// @Router /api/uptime [get]
// @Security OAuth2Application[admin]
func getAllUptime(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.Uptime{DB: db}
	ns := &services.Node{DB: db}

	// Obtener período desde query parameter (por defecto últimos 30 días)
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 30
	}
	if days > 365 {
		days = 365 // Límite máximo de 1 año
	}

	since := time.Now().AddDate(0, 0, -days)

	uptimeStats, err := us.GetAllServerUptime(since)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Obtener TODOS los servidores de la base de datos para incluir los que no tienen datos de uptime
	dbServers := &services.Server{DB: db}
	searchCriteria := services.ServerSearch{
		PageSize: 1000, // Obtener todos los servidores
		Page:     1,
	}
	allServers, totalServers, err := dbServers.Search(searchCriteria)
	if err != nil {
		// Si falla, solo usar los que tienen datos de uptime
		for serverID := range uptimeStats {
			server, err := dbServers.Get(serverID)
			if err == nil && server != nil {
				uptimeStats[serverID]["serverName"] = server.Name
				nodeName := server.Node.Name
				if nodeName == "" || server.Node.IsLocal() {
					nodeName = "LocalNode"
				}
				uptimeStats[serverID]["nodeName"] = nodeName
			}
		}
		c.JSON(http.StatusOK, uptimeStats)
		return
	}

	// Si no hay servidores en la base de datos, devolver objeto vacío
	if totalServers == 0 {
		c.JSON(http.StatusOK, make(map[string]interface{}))
		return
	}

	// Mapa para rastrear qué servidores ya tienen datos de uptime
	hasUptimeData := make(map[string]bool)
	for serverID := range uptimeStats {
		hasUptimeData[serverID] = true
	}

	// Agregar información de nodos y nombres a servidores existentes
	// Y obtener estado actual desde el daemon para servidores con datos históricos
	for serverID := range uptimeStats {
		server, err := dbServers.Get(serverID)
		if err == nil && server != nil {
			uptimeStats[serverID]["serverName"] = server.Name
			nodeName := server.Node.Name
			if nodeName == "" || server.Node.IsLocal() {
				nodeName = "LocalNode"
			}
			uptimeStats[serverID]["nodeName"] = nodeName

			// Obtener estado actual desde el daemon para actualizar currentStatus
			isRunning := false
			nodeResponse, err := ns.CallNode(&server.Node, "GET", "/daemon/server/"+serverID+"/status", nil, nil)
			if err == nil && nodeResponse != nil {
				if nodeResponse.StatusCode == http.StatusOK {
					var statusResponse SkyPanel.ServerRunning
					if err := json.NewDecoder(nodeResponse.Body).Decode(&statusResponse); err == nil {
						isRunning = statusResponse.Running
					}
				}
				utils.CloseResponse(nodeResponse)
			}
			uptimeStats[serverID]["currentStatus"] = isRunning
		}
	}

	// Para servidores sin datos de uptime, obtener estado actual y crear entrada mínima
	now := time.Now()
	for _, server := range allServers {
		if !hasUptimeData[server.Identifier] {
			// Obtener estado actual del servidor desde el daemon
			isRunning := false
			nodeResponse, err := ns.CallNode(&server.Node, "GET", "/daemon/server/"+server.Identifier+"/status", nil, nil)
			if err == nil && nodeResponse != nil {
				if nodeResponse.StatusCode == http.StatusOK {
					var statusResponse SkyPanel.ServerRunning
					if err := json.NewDecoder(nodeResponse.Body).Decode(&statusResponse); err == nil {
						isRunning = statusResponse.Running
					}
				}
				utils.CloseResponse(nodeResponse)
			}

			nodeName := server.Node.Name
			if nodeName == "" || server.Node.IsLocal() {
				nodeName = "LocalNode"
			}

			// Crear entrada con datos mínimos (desde ahora)
			// Para servidores sin datos históricos, mostrar uptime actual desde inicio del período
			var currentUptime int64 = 0
			var currentStartTime time.Time = now
			
			// Si está online, calcular uptime desde el inicio del período
			if isRunning {
				currentUptime = int64(time.Since(since).Seconds())
				currentStartTime = since
			}

			uptimeStats[server.Identifier] = map[string]interface{}{
				"serverName":      server.Name,
				"nodeName":        nodeName,
				"uptime":          currentUptime,
				"downtime":        int64(0),
				"uptimePercent":   100.0, // Si está online, 100%, si está offline, será actualizado
				"currentStatus":   isRunning,
				"currentStartTime": currentStartTime,
			}

			// Si está offline, ajustar porcentaje
			if !isRunning {
				periodDuration := int64(time.Since(since).Seconds())
				if periodDuration > 0 {
					uptimeStats[server.Identifier]["downtime"] = periodDuration
					uptimeStats[server.Identifier]["uptimePercent"] = 0.0
				} else {
					uptimeStats[server.Identifier]["uptimePercent"] = 0.0
				}
			}
		}
	}

	c.JSON(http.StatusOK, uptimeStats)
}

// @Summary Get server uptime
// @Description Gets uptime statistics for a specific server
// @Success 200 {object} map[string]interface{} "Uptime statistics"
// @Param id path string true "Server ID"
// @Param days query int false "Number of days to look back (default: 30, max: 365)"
// @Param limit query int false "Number of history records to return (default: 50, max: 200)"
// @Router /api/uptime/{id} [get]
// @Security OAuth2Application[server.view]
func getServerUptime(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	us := &services.Uptime{DB: db}

	// Obtener período desde query parameter (por defecto últimos 30 días)
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 30
	}
	if days > 365 {
		days = 365 // Límite máximo de 1 año
	}

	since := time.Now().AddDate(0, 0, -days)

	// Obtener estadísticas de uptime
	uptimeSeconds, downtimeSeconds, uptimePercent, err := us.GetUptimeStats(server.Identifier, since)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Obtener historial reciente
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	history, err := us.GetRecentHistory(server.Identifier, limit)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Obtener estado actual del servidor desde el daemon
	isRunning := false
	ns := &services.Node{DB: db}
	nodeResponse, err := ns.CallNode(&server.Node, "GET", "/daemon/server/"+server.Identifier+"/status", nil, nil)
	if err == nil && nodeResponse != nil {
		defer utils.CloseResponse(nodeResponse)
		if nodeResponse.StatusCode == http.StatusOK {
			var statusResponse SkyPanel.ServerRunning
			if err := json.NewDecoder(nodeResponse.Body).Decode(&statusResponse); err == nil {
				isRunning = statusResponse.Running
			}
		}
	}

	// Calcular tiempo actual si está corriendo
	currentUptime := int64(0)
	var currentStartTime *time.Time
	if isRunning {
		// Buscar cuando empezó el estado actual
		var currentStatus *models.UptimeStatus
		err = db.Where("server_id = ? AND end_time IS NULL", server.Identifier).
			Order("start_time DESC").First(&currentStatus).Error
		if err == nil && currentStatus != nil {
			currentUptime = int64(time.Since(currentStatus.StartTime).Seconds())
			currentStartTime = &currentStatus.StartTime
		}
	}

	totalSeconds := uptimeSeconds + downtimeSeconds
	if isRunning {
		totalSeconds += currentUptime
	}

	nodeName := server.Node.Name
	if nodeName == "" || server.Node.IsLocal() {
		nodeName = "LocalNode"
	}

	c.JSON(http.StatusOK, gin.H{
		"serverId":        server.Identifier,
		"serverName":      server.Name,
		"nodeName":        nodeName,
		"currentStatus":   isRunning,
		"currentStartTime": currentStartTime,
		"currentUptime":   currentUptime,
		"uptimeSeconds":   uptimeSeconds,
		"downtimeSeconds": downtimeSeconds,
		"totalSeconds":    totalSeconds,
		"uptimePercent":   uptimePercent,
		"period": gin.H{
			"days": days,
			"since": since,
			"until": time.Now(),
		},
		"history": history,
	})
}
