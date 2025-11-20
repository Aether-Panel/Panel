package services

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/SkyPanel/SkyPanel/v3/database"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	gatusConfig "github.com/TwiN/gatus/v5/config"
	gatusController "github.com/TwiN/gatus/v5/controller"
	gatusMetrics "github.com/TwiN/gatus/v5/metrics"
	gatusStorage "github.com/TwiN/gatus/v5/storage/store"
	gatusWatchdog "github.com/TwiN/gatus/v5/watchdog"
	"gopkg.in/yaml.v3"
)

var gatusConfigInstance *gatusConfig.Config
var gatusRunning bool

// StartGatus inicia Gatus como servicio independiente en puerto interno
func StartGatus() error {
	// Obtener la ruta del directorio de configuración
	// Usar la misma carpeta base que los servidores
	dataRoot := config.DataRootFolder.Value()
	if dataRoot == "" {
		dataRoot = "."
	}
	configPath := filepath.Join(dataRoot, "gatus", "config.yaml")

	// Crear directorio si no existe
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Si no existe config.yaml, crear uno por defecto
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := createDefaultGatusConfig(configPath); err != nil {
			return err
		}
	}

	// Configurar variable de entorno para Gatus
	os.Setenv("GATUS_CONFIG_PATH", configPath)
	os.Setenv("GATUS_LOG_LEVEL", "WARN") // Solo mostrar warnings y errores, no cada verificación

	// Cargar configuración de Gatus
	cfg, err := gatusConfig.LoadConfiguration(configPath)
	if err != nil {
		logging.Error.Printf("Error loading Gatus configuration: %s", err.Error())
		return err
	}

	// Configurar puerto interno (8081)
	cfg.Web.Address = "127.0.0.1"
	cfg.Web.Port = 8081

	// Validar configuración web (los otros ya fueron validados por LoadConfiguration)
	if err := cfg.Web.ValidateAndSetDefaults(); err != nil {
		logging.Error.Printf("Error validating Gatus web configuration: %s", err.Error())
		return err
	}

	gatusConfigInstance = cfg

	// Inicializar storage de Gatus
	if err := gatusStorage.Initialize(cfg.Storage); err != nil {
		logging.Error.Printf("Error initializing Gatus storage: %s", err.Error())
		return err
	}

	// Sincronizar nodos automáticamente antes de iniciar
	if err := syncNodesToGatus(configPath, cfg); err != nil {
		logging.Error.Printf("Error syncing nodes to Gatus: %s", err.Error())
	} else {
		// Si la sincronización fue exitosa, recargar la configuración actualizada
		cfg, err = gatusConfig.LoadConfiguration(configPath)
		if err != nil {
			logging.Error.Printf("Error reloading Gatus configuration after node sync: %s", err.Error())
			return err
		}
		cfg.Web.Address = "127.0.0.1"
		cfg.Web.Port = 8081
		if err := cfg.Web.ValidateAndSetDefaults(); err != nil {
			logging.Error.Printf("Error validating Gatus web configuration: %s", err.Error())
			return err
		}
		gatusConfigInstance = cfg
	}

	// Sincronizar el nombre de la empresa al iniciar
	if err := SyncCompanyNameToGatus(); err != nil {
		logging.Error.Printf("Error syncing company name to Gatus: %s", err.Error())
		// No es crítico, continuar con el inicio
	}

	// Limpiar estados de endpoints que ya no existen
	cleanupGatusStorage(cfg)

	// Inicializar métricas de Prometheus (si están habilitadas)
	if cfg.Metrics {
		gatusMetrics.InitializePrometheusMetrics(cfg, nil)
	}

	// Iniciar monitoreo de endpoints
	gatusWatchdog.Monitor(cfg)

	// Iniciar controller de Gatus en goroutine separada
	go func() {
		gatusRunning = true
		gatusController.Handle(cfg)
		gatusRunning = false
	}()

	logging.Info.Printf("Gatus service started on http://127.0.0.1:8081")
	return nil
}

// StopGatus detiene el servicio de Gatus
func StopGatus() {
	if gatusConfigInstance != nil {
		gatusWatchdog.Shutdown(gatusConfigInstance)
		gatusController.Shutdown()
		gatusMetrics.UnregisterPrometheusMetrics()
		gatusStorage.Get().Close()
		gatusRunning = false
		logging.Info.Printf("Gatus service stopped")
	}
}

// IsGatusRunning retorna si Gatus está corriendo
func IsGatusRunning() bool {
	return gatusRunning
}

// createDefaultGatusConfig crea una configuración por defecto para Gatus
func createDefaultGatusConfig(configPath string) error {
	defaultConfig := `# Configuración de Gatus para PufferPanel
# Este archivo monitorea los nodos y servidores del panel

# Configuración del servidor web
web:
  address: "127.0.0.1"
  port: 8081

# Almacenamiento en memoria (puedes cambiar a SQLite o PostgreSQL)
storage:
  type: memory

# Endpoints a monitorear
# Se pueden añadir endpoints aquí o se generarán automáticamente
endpoints:
  - name: "Panel Principal"
    group: "Panel"
    url: "http://127.0.0.1:5656/daemon"
    interval: 1m
    conditions:
      - "[STATUS] == 200"
`

	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(defaultConfig)
	return err
}

// cleanupGatusStorage limpia estados de endpoints que ya no existen en la configuración
func cleanupGatusStorage(cfg *gatusConfig.Config) {
	// Obtener todas las claves de endpoints
	var endpointKeys []string
	for _, ep := range cfg.Endpoints {
		endpointKeys = append(endpointKeys, ep.Key())
	}
	for _, ee := range cfg.ExternalEndpoints {
		endpointKeys = append(endpointKeys, ee.Key())
	}
	for _, suite := range cfg.Suites {
		for _, ep := range suite.Endpoints {
			endpointKeys = append(endpointKeys, ep.Key())
		}
	}

	// Eliminar estados de endpoints que ya no existen
	if len(endpointKeys) > 0 {
		deleted := gatusStorage.Get().DeleteAllEndpointStatusesNotInKeys(endpointKeys)
		if deleted > 0 {
			logging.Debug.Printf("Gatus: Deleted %d endpoint statuses that no longer exist", deleted)
		}
	}

	// Limpiar suites
	var suiteKeys []string
	for _, suite := range cfg.Suites {
		suiteKeys = append(suiteKeys, suite.Key())
	}
	if len(suiteKeys) > 0 {
		deleted := gatusStorage.Get().DeleteAllSuiteStatusesNotInKeys(suiteKeys)
		if deleted > 0 {
			logging.Debug.Printf("Gatus: Deleted %d suite statuses that no longer exist", deleted)
		}
	}
}

// syncNodesToGatus sincroniza los nodos de PufferPanel con la configuración de Gatus
func syncNodesToGatus(configPath string, cfg *gatusConfig.Config) error {
	// Obtener conexión a la base de datos
	db, err := database.GetConnection()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Obtener todos los nodos
	ns := &Node{DB: db}
	nodes, err := ns.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	// Leer el archivo YAML actual
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parsear YAML
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Obtener o crear la sección de endpoints
	endpoints, ok := configMap["endpoints"].([]interface{})
	if !ok {
		endpoints = []interface{}{}
	}

	// Crear un mapa de endpoints de nodos existentes para evitar duplicados
	nodeEndpointNames := make(map[string]bool)

	// Identificar endpoints que son de nodos
	for _, ep := range endpoints {
		if epMap, ok := ep.(map[string]interface{}); ok {
			// Si el grupo es "Nodos PufferPanel", es un endpoint de nodo
			if group, ok := epMap["group"].(string); ok && group == "Nodos PufferPanel" {
				if name, ok := epMap["name"].(string); ok {
					nodeEndpointNames[name] = true
				}
			}
		}
	}

	// Crear endpoints para cada nodo
	newEndpoints := []interface{}{}
	for _, node := range nodes {
		nodeName := node.Name
		if node.IsLocal() {
			nodeName = "LocalNode"
		}

		// Verificar si ya existe un endpoint para este nodo
		if nodeEndpointNames[nodeName] {
			continue // Ya existe, no duplicar
		}

		// Determinar el protocolo (http o https)
		protocol := "http"
		if !node.IsLocal() {
			// Intentar detectar si usa SSL (similar a doesDaemonUseSSL)
			// Por ahora asumimos http, pero podríamos hacer una verificación
			// Para simplificar, usamos http por defecto
		}

		// Construir la URL del daemon
		daemonURL := fmt.Sprintf("%s://%s:%d/daemon", protocol, node.PrivateHost, node.PrivatePort)

		// Crear el endpoint
		endpoint := map[string]interface{}{
			"name":     nodeName,
			"group":    "Nodos PufferPanel",
			"url":      daemonURL,
			"interval": "1m",
			"conditions": []interface{}{
				"[STATUS] == 200",
				"[BODY].message == daemon is running",
				"[RESPONSE_TIME] < 5000",
			},
		}

		newEndpoints = append(newEndpoints, endpoint)
		logging.Info.Printf("Gatus: Added endpoint for node '%s' at %s", nodeName, daemonURL)
	}

	// Añadir los nuevos endpoints a la lista existente
	// Primero, mantener los endpoints que NO son de nodos
	filteredEndpoints := []interface{}{}
	for _, ep := range endpoints {
		if epMap, ok := ep.(map[string]interface{}); ok {
			if group, ok := epMap["group"].(string); ok && group == "Nodos PufferPanel" {
				continue // Eliminar endpoints de nodos antiguos
			}
			filteredEndpoints = append(filteredEndpoints, ep)
		}
	}

	// Añadir los nuevos endpoints de nodos
	filteredEndpoints = append(filteredEndpoints, newEndpoints...)

	// Actualizar el mapa de configuración
	configMap["endpoints"] = filteredEndpoints

	// Escribir el YAML actualizado
	updatedData, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Crear backup del archivo original
	backupPath := configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		logging.Error.Printf("Failed to create backup of Gatus config: %s", err.Error())
	}

	// Escribir el archivo actualizado
	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	if len(newEndpoints) > 0 {
		logging.Info.Printf("Gatus: Synced %d node(s) to configuration", len(newEndpoints))
	}

	return nil
}

// SyncCompanyNameToGatus sincroniza el nombre de la empresa de PufferPanel con la configuración UI de Gatus
func SyncCompanyNameToGatus() error {
	// Verificar si Gatus está habilitado
	if !config.GatusEnabled.Value() {
		return nil // Gatus no está habilitado, no hacer nada
	}

	// Obtener la ruta del archivo de configuración
	dataRoot := config.DataRootFolder.Value()
	if dataRoot == "" {
		dataRoot = "."
	}
	configPath := filepath.Join(dataRoot, "gatus", "config.yaml")

	// Verificar si el archivo existe
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// El archivo no existe, crear uno con la configuración básica
		if err := createDefaultGatusConfig(configPath); err != nil {
			return fmt.Errorf("failed to create default Gatus config: %w", err)
		}
	}

	// Leer el archivo YAML actual
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parsear YAML
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Obtener el nombre de la empresa de PufferPanel
	companyName := config.CompanyName.Value()

	// Obtener o crear la sección UI
	ui, ok := configMap["ui"].(map[string]interface{})
	if !ok {
		ui = make(map[string]interface{})
	}

	// Actualizar el header con el nombre de la empresa
	ui["header"] = companyName

	// Actualizar el mapa de configuración
	configMap["ui"] = ui

	// Escribir el YAML actualizado
	updatedData, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Crear backup del archivo original
	backupPath := configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		logging.Error.Printf("Failed to create backup of Gatus config: %s", err.Error())
	}

	// Escribir el archivo actualizado
	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Recargar la configuración de Gatus si está corriendo
	if gatusConfigInstance != nil && gatusRunning {
		cfg, err := gatusConfig.LoadConfiguration(configPath)
		if err != nil {
			logging.Error.Printf("Error reloading Gatus configuration: %s", err.Error())
			return nil // No es crítico, el archivo ya está actualizado
		}

		// Configurar puerto interno
		cfg.Web.Address = "127.0.0.1"
		cfg.Web.Port = 8081
		if err := cfg.Web.ValidateAndSetDefaults(); err != nil {
			logging.Error.Printf("Error validating Gatus web configuration: %s", err.Error())
			return nil // No es crítico
		}

		// Validar configuración UI
		if err := cfg.UI.ValidateAndSetDefaults(); err != nil {
			logging.Error.Printf("Error validating Gatus UI configuration: %s", err.Error())
			return nil // No es crítico
		}

		// Actualizar la instancia global (esto hará que las próximas peticiones HTML usen el nuevo nombre)
		// Especialmente importante: actualizar el UI config que se usa para renderizar el HTML
		gatusConfigInstance.UI = cfg.UI
		logging.Info.Printf("Gatus: Updated company name to '%s' in UI config", companyName)
	}

	return nil
}
