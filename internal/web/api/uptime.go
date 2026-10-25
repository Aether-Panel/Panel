package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerUptime(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresAnyPermission(scopes.ScopeAdmin, scopes.ScopeUptimeView), getAllUptime)
	g.Handle("GET", "/:serverId", middleware.RequiresPermission(scopes.ScopeServerView), middleware.ResolveServerPanel, getServerUptime)
	g.Handle("OPTIONS", "", response.CreateOptions("GET"))
	g.Handle("OPTIONS", "/:serverId", response.CreateOptions("GET"))
}

// @Summary Get all servers uptime
// @Description Gets uptime statistics for all servers
// @Success 200 {object} map[string]interface{} "Uptime statistics"
// @Tags Uptime
// @Router /api/uptime [get]
// @Security OAuth2Application[admin]
func getAllUptime(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.Uptime{DB: db}
	ns := &services.Node{DB: db}

	// Obtener período desde query parameter (por defecto últimos 30 días, máximo 1 año)
	days := clampedQueryInt(c, "days", 30, 365)

	since := time.Now().AddDate(0, 0, -days)

	uptimeStats, err := us.GetAllServerUptime(since)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Obtener todos los servidores de la base de datos para incluir los que no tienen datos de uptime
	dbServers := &services.Server{DB: db}
	searchCriteria := services.ServerSearch{
		PageSize: 1000, // Obtener todos los servidores
		Page:     1,
	}

	// Un usuario sin admin solo puede ver los servidores a los que tiene acceso
	user := c.MustGet("user").(*models.User)
	isAdmin := scopes.ContainsScope(getUserScopesForSearch(user, &services.Permission{DB: db}), scopes.ScopeAdmin)
	if !isAdmin {
		searchCriteria.Username = user.Username
	}

	allServers, totalServers, err := dbServers.Search(searchCriteria)
	if err != nil && !isAdmin {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}
	if !isAdmin {
		uptimeStats = filterUptimeToServers(uptimeStats, allServers)
	}
	if err != nil {
		// Si falla, solo usar los que tienen datos de uptime
		for serverID := range uptimeStats {
			describeUptimeServer(dbServers, uptimeStats, serverID)
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
		if server := describeUptimeServer(dbServers, uptimeStats, serverID); server != nil {
			uptimeStats[serverID]["currentStatus"] = fetchServerRunning(c, ns, &server.Node, serverID)
		}
	}

	// Para servidores sin datos de uptime, obtener estado actual y crear entrada mínima
	addMissingUptimeEntries(c, ns, uptimeStats, hasUptimeData, allServers, since)

	c.JSON(http.StatusOK, uptimeStats)
}

// addMissingUptimeEntries crea una entrada mínima, con su estado actual, para cada servidor
// que no tiene datos de uptime
func addMissingUptimeEntries(c *gin.Context, ns *services.Node, uptimeStats map[string]map[string]interface{}, hasUptimeData map[string]bool, allServers []*models.Server, since time.Time) {
	now := time.Now()
	for _, server := range allServers {
		if hasUptimeData[server.Identifier] {
			continue
		}
		isRunning := fetchServerRunning(c, ns, &server.Node, server.Identifier)
		uptimeStats[server.Identifier] = minimalUptimeEntry(server, isRunning, since, now)
	}
}

// clampedQueryInt lee un entero de la query: def si falta o es menor que 1, max como tope
func clampedQueryInt(c *gin.Context, name string, def, maxValue int) int {
	value, err := strconv.Atoi(c.DefaultQuery(name, strconv.Itoa(def)))
	if err != nil || value < 1 {
		value = def
	}
	if value > maxValue {
		value = maxValue
	}
	return value
}

// uptimeNodeName es el nombre del nodo tal como se muestra en las estadísticas
func uptimeNodeName(node *models.Node) string {
	if node.Name == "" || node.IsLocal() {
		return "LocalNode"
	}
	return node.Name
}

// describeUptimeServer añade el nombre del servidor y de su nodo a sus estadísticas; devuelve
// el servidor, o nil si no se encuentra
func describeUptimeServer(dbServers *services.Server, uptimeStats map[string]map[string]interface{}, serverID string) *models.Server {
	server, err := dbServers.Get(serverID)
	if err != nil || server == nil {
		return nil
	}
	uptimeStats[serverID]["serverName"] = server.Name
	uptimeStats[serverID]["nodeName"] = uptimeNodeName(&server.Node)
	return server
}

// fetchServerRunning pregunta al daemon si el servidor está encendido; cualquier fallo cuenta
// como apagado
func fetchServerRunning(c *gin.Context, ns *services.Node, node *models.Node, serverID string) bool {
	nodeResponse, err := callNodeStatus(c, ns, node, serverID)
	if err != nil || nodeResponse == nil {
		return false
	}
	defer utils.CloseResponse(nodeResponse)
	if nodeResponse.StatusCode != http.StatusOK {
		return false
	}
	var statusResponse skypanel.ServerRunning
	if err := json.NewDecoder(nodeResponse.Body).Decode(&statusResponse); err != nil {
		return false
	}
	return statusResponse.Running
}

// minimalUptimeEntry crea la entrada de un servidor sin datos históricos: según su estado
// actual cuenta como encendido o apagado durante el período entero
func minimalUptimeEntry(server *models.Server, isRunning bool, since, now time.Time) map[string]interface{} {
	var currentUptime int64
	currentStartTime := now

	// Si está online, calcular uptime desde el inicio del período
	if isRunning {
		currentUptime = int64(time.Since(since).Seconds())
		currentStartTime = since
	}

	entry := map[string]interface{}{
		"serverName":       server.Name,
		"nodeName":         uptimeNodeName(&server.Node),
		"uptime":           currentUptime,
		"downtime":         int64(0),
		"uptimePercent":    100.0, // Si está online, 100%, si está offline, será actualizado
		"currentStatus":    isRunning,
		"currentStartTime": currentStartTime,
	}

	// Si está offline, ajustar porcentaje
	if !isRunning {
		if periodDuration := int64(time.Since(since).Seconds()); periodDuration > 0 {
			entry["downtime"] = periodDuration
		}
		entry["uptimePercent"] = 0.0
	}
	return entry
}

// nodeStatusTimeout acota cada consulta de estado: un nodo caído no debe dejar colgada la
// petición, que pregunta por todos los servidores uno tras otro
const nodeStatusTimeout = 5 * time.Second

func callNodeStatus(c *gin.Context, ns *services.Node, node *models.Node, serverID string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), nodeStatusTimeout)
	res, err := ns.CallNodeContext(ctx, node, "GET", "/daemon/server/"+serverID+"/status", nil, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	// the body is read by the caller, so the deadline is released when it is closed
	res.Body = &cancelOnClose{ReadCloser: res.Body, cancel: cancel}
	return res, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	defer b.cancel()
	return b.ReadCloser.Close()
}

// filterUptimeToServers deja solo las estadísticas de los servidores indicados
func filterUptimeToServers(uptimeStats map[string]map[string]interface{}, allowed []*models.Server) map[string]map[string]interface{} {
	filtered := make(map[string]map[string]interface{}, len(allowed))
	for _, server := range allowed {
		if stats, ok := uptimeStats[server.Identifier]; ok {
			filtered[server.Identifier] = stats
		}
	}
	return filtered
}

// @Summary Get server uptime
// @Description Gets uptime statistics for a specific server
// @Success 200 {object} map[string]interface{} "Uptime statistics"
// @Param id path string true "Server ID"
// @Param days query int false "Number of days to look back (default: 30, max: 365)"
// @Param limit query int false "Number of history records to return (default: 50, max: 200)"
// @Tags Uptime
// @Router /api/uptime/{id} [get]
// @Security OAuth2Application[server.view]
func getServerUptime(c *gin.Context) {
	server := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	us := &services.Uptime{DB: db}

	// Obtener período desde query parameter (por defecto últimos 30 días, máximo 1 año)
	days := clampedQueryInt(c, "days", 30, 365)

	since := time.Now().AddDate(0, 0, -days)

	// Obtener estadísticas de uptime
	uptimeSeconds, downtimeSeconds, uptimePercent, err := us.GetUptimeStats(server.Identifier, since)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Obtener historial reciente
	limit := clampedQueryInt(c, "limit", 50, 200)

	history, err := us.GetRecentHistory(server.Identifier, limit)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Obtener estado actual del servidor desde el daemon
	ns := &services.Node{DB: db}
	isRunning := fetchServerRunning(c, ns, &server.Node, server.Identifier)

	// Calcular tiempo actual si está corriendo
	currentUptime := int64(0)
	var currentStartTime *time.Time
	if isRunning {
		currentUptime, currentStartTime = currentRunSince(db, server.Identifier)
	}

	totalSeconds := uptimeSeconds + downtimeSeconds
	if isRunning {
		totalSeconds += currentUptime
	}

	nodeName := uptimeNodeName(&server.Node)

	c.JSON(http.StatusOK, gin.H{
		"serverId":         server.Identifier,
		"serverName":       server.Name,
		"nodeName":         nodeName,
		"currentStatus":    isRunning,
		"currentStartTime": currentStartTime,
		"currentUptime":    currentUptime,
		"uptimeSeconds":    uptimeSeconds,
		"downtimeSeconds":  downtimeSeconds,
		"totalSeconds":     totalSeconds,
		"uptimePercent":    uptimePercent,
		"period": gin.H{
			"days":  days,
			"since": since,
			"until": time.Now(),
		},
		"history": history,
	})
}

// currentRunSince devuelve cuánto lleva el servidor en su estado actual y desde cuándo; cero y
// nil si no hay un estado abierto
func currentRunSince(db *gorm.DB, serverID string) (int64, *time.Time) {
	// Buscar cuando empezó el estado actual
	var currentStatus *models.UptimeStatus
	err := db.Where("server_id = ? AND end_time IS NULL", serverID).
		Order("start_time DESC").First(&currentStatus).Error
	if err != nil || currentStatus == nil {
		return 0, nil
	}
	return int64(time.Since(currentStatus.StartTime).Seconds()), &currentStatus.StartTime
}
