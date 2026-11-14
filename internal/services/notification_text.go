package services

import (
	"fmt"
	"sync/atomic"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
)

// DefaultNotificationLanguage is used when the setting is empty or names a language the
// catalogue does not have
const DefaultNotificationLanguage = "es"

// notificationTexts holds every text the panel sends outside its own pages (Discord, Telegram,
// webhooks, extension events), per language. Texts with values are fmt formats; the test
// checks that both languages have the same keys and the same verbs in the same order.
// The texts mention tokens and passwords, which gosec G101 reads as credentials: they are not.
var notificationTexts = map[string]map[string]string{ // #nosec G101 -- message catalogue
	"es": { // #nosec G101 -- message catalogue
		// embed field names
		"field.server":         "Servidor",
		"field.status":         "Estado",
		"field.resource":       "Recurso",
		"field.currentUsage":   "Uso Actual",
		"field.threshold":      "Umbral",
		"field.details":        "Detalles",
		"field.autoRestart":    "Reinicio automático",
		"field.note":           "Nota",
		"field.totalServers":   "Servidores Totales",
		"field.onlineServers":  "Servidores Online",
		"field.offlineServers": "Servidores Offline",
		"field.cores":          "Núcleos",
		"field.threads":        "Hilos",
		"field.frequency":      "Frecuencia",
		"field.ramTotal":       "RAM Total",
		"field.ramUsed":        "RAM Usada",
		"field.ramFree":        "RAM Libre",
		"field.os":             "Sistema Operativo",
		"field.type":           "Tipo",
		"yes":                  "Sí",
		"no":                   "No",

		// resource names: as a title word, and inside a sentence
		"resource.cpu":               "CPU",
		"resource.cpu.inSentence":    "cpu",
		"resource.memory":            "Memoria",
		"resource.memory.inSentence": "memoria",

		// the panel's Discord alerts
		"serverOffline.title":       "⚠️ Servidor Desconectado",
		"serverOffline.description": "El servidor **%s** se ha desconectado o está offline.",
		"serverOnline.title":        "✅ Servidor Conectado",
		"serverOnline.description":  "El servidor **%s** está ahora online.",
		"resourceAlert.title":       "⚠️ Alto Uso de %s",
		"resourceAlert.description": "El servidor **%s** está usando **%.1f%%** de %s (umbral: %.1f%%)",
		"backupAlert.successTitle":  "✅ Backup Completado",
		"backupAlert.success":       "El backup del servidor **%s** se completó exitosamente.",
		"backupAlert.failedTitle":   "❌ Backup Fallido",
		"backupAlert.failed":        "El backup del servidor **%s** falló: %s",
		"backupAlert.statusSuccess": "Completado exitosamente",
		"backupAlert.statusFailed":  "Falló durante la creación",
		"crashAlert.title":          "💥 Caída del servidor",
		"crashAlert.description":    "**%s** se ha caído (código de salida %d).",
		"systemStatus.title":        "📊 Estado del Sistema",
		"systemStatus.footer":       "Actualizado cada minuto",
		"systemStatus.total":        "**Total de Servidores:** %d",
		"systemStatus.averages":     "**Promedio de Recursos (Servidores Online):**",
		"systemStatus.memory":       "• Memoria: %.1f MB",
		"systemStatus.showing":      "Mostrando %d de %d servidores",
		"nodeStatus.title":          "🖥️ Estado del Nodo",
		"nodeStatus.localNode":      "Nodo Local",
		"nodeStatus.node":           "**Nodo:** %s",
		"nodeStatus.address":        "**Dirección:** %s",
		"nodeStatus.active":         "🟢 Activo",
		"nodeStatus.noAnswer":       "🔴 Sin respuesta",
		"nodeStatus.unreachable":    "El nodo no responde: el estado de sus servidores es desconocido.",
		"discordTest.title":         "🧪 Test de Webhook Discord",
		"discordTest.description":   "Este es un mensaje de prueba para verificar que el webhook de Discord está configurado correctamente.",
		"discordTest.type":          "Mensaje de Prueba",
		"discordTest.status":        "✅ Configuración Correcta",
		"discordTest.system":        "Sistema",
		"discordTest.node":          "Nodo",
		"discordTest.transfers":     "Migraciones",

		// a server's own alerts (Discord, Telegram, webhook) and the extension events
		"notify.crashTitle":        "💥 Caída del servidor",
		"notify.crash":             "%s se ha caído (código de salida %d). %s",
		"notify.notRestarted":      "No se reinició automáticamente.",
		"notify.restarting":        "Se está reiniciando solo (intento %d de %d).",
		"notify.backupTitle":       "✅ Copia de seguridad completada",
		"notify.backup":            "La copia de seguridad de %s se ha guardado correctamente.",
		"notify.backupFailedTitle": "❌ La copia de seguridad falló",
		"notify.backupFailed":      "No se pudo completar la copia de seguridad de %s. Revisa la consola del servidor.",
		"notify.resourceTitle":     "⚠️ Uso alto de %s",
		"notify.resource":          "%s está usando el %.0f %% de %s (aviso a partir del %.0f %%).",
		"notify.startedTitle":      "🟢 Servidor encendido",
		"notify.started":           "%s está en línea.",
		"notify.stoppedTitle":      "⚪ Servidor apagado",
		"notify.stopped":           "%s se ha detenido.",
		"notify.testTitle":         "🔔 Notificaciones conectadas",
		"notify.test":              "A partir de ahora recibirás aquí los avisos de %s.",

		// external transfer reports
		"extransfer.report":                      "Reporte de transferencia externa",
		"extransfer.reason":                      "Motivo: %s",
		"extransfer.status.completed":            "Completada",
		"extransfer.status.completedOrigin":      "Completada (Origen)",
		"extransfer.status.failed":               "Fallida",
		"extransfer.status.failedOrigin":         "Fallida (Origen)",
		"extransfer.status.cancelled":            "Cancelada",
		"extransfer.status.cancelledOrigin":      "Cancelada (Origen)",
		"extransfer.status.cancelledDestination": "Cancelada (Destino)",
		"extransfer.completed":                   "La transferencia externa finalizó correctamente.",
		"extransfer.destinationConfirmed":        "El panel de destino confirmó que recibió los datos del servidor.",
		"extransfer.destinationCancelled":        "El panel de destino canceló la transferencia.",
		"extransfer.originCancelled":             "El panel de origen canceló la transferencia.",
		"extransfer.ownerCancelledOrigin":        "El propietario canceló la transferencia desde el panel de origen.",
		"extransfer.ownerCancelledDestination":   "El propietario canceló la transferencia desde el panel de destino.",
		"extransfer.archiveFailed":               "Fallo al comprimir los archivos en el servidor de origen.",
		"extransfer.expired":                     "El panel de destino dejó de responder y la transferencia caducó sin terminar.",

		// the destination's import steps and failures (the page translates them itself; these
		// are the report's details and the English fallback of the status API)
		"extransfer.STEP_STARTING":            "Iniciando...",
		"extransfer.STEP_VALIDATING":          "Validando conexión con el panel de origen...",
		"extransfer.STEP_ORIGIN_START":        "Iniciando la transferencia en el servidor origen...",
		"extransfer.STEP_WAITING_ARCHIVE":     "Esperando a que el origen comprima los archivos...",
		"extransfer.STEP_DOWNLOADING":         "Descargando paquete de datos desde el origen...",
		"extransfer.STEP_UPLOADING":           "Subiendo archivos al daemon local (esto puede tardar)...",
		"extransfer.STEP_EXTRACTING":          "Descomprimiendo archivos...",
		"extransfer.STEP_CLEANUP":             "Limpiando archivos temporales...",
		"extransfer.ERROR_ORIGIN_URL":         "La URL de origen no es válida o apunta a una dirección no permitida",
		"extransfer.ERROR_ORIGIN_UNREACHABLE": "Fallo de red al conectar con origen",
		"extransfer.ERROR_TOKEN_REJECTED":     "Token inválido o expirado",
		"extransfer.ERROR_ORIGIN_BAD_ANSWER":  "Respuesta inválida del origen",
		"extransfer.ERROR_ORIGIN_BAD_SESSION": "El origen devolvió un ID de sesión inválido",
		"extransfer.ERROR_ORIGIN_START":       "Fallo al iniciar transferencia en origen",
		"extransfer.ERROR_ORIGIN_REFUSED":     "El origen rechazó la transferencia",
		"extransfer.ERROR_DOWNLOAD_NETWORK":   "Error de red al descargar paquete",
		"extransfer.ERROR_DOWNLOAD_REFUSED":   "El origen rechazó la descarga",
		"extransfer.ERROR_DOWNLOAD_TIMEOUT":   "El origen no preparó el paquete a tiempo",
		"extransfer.ERROR_UPLOAD":             "Error al subir archivo al daemon",
		"extransfer.ERROR_EXTRACT":            "Error al descomprimir en el destino",
	},
	"en": { // #nosec G101 -- message catalogue
		"field.server":         "Server",
		"field.status":         "Status",
		"field.resource":       "Resource",
		"field.currentUsage":   "Current usage",
		"field.threshold":      "Threshold",
		"field.details":        "Details",
		"field.autoRestart":    "Automatic restart",
		"field.note":           "Note",
		"field.totalServers":   "Total servers",
		"field.onlineServers":  "Online servers",
		"field.offlineServers": "Offline servers",
		"field.cores":          "Cores",
		"field.threads":        "Threads",
		"field.frequency":      "Frequency",
		"field.ramTotal":       "Total RAM",
		"field.ramUsed":        "Used RAM",
		"field.ramFree":        "Free RAM",
		"field.os":             "Operating system",
		"field.type":           "Type",
		"yes":                  "Yes",
		"no":                   "No",

		"resource.cpu":               "CPU",
		"resource.cpu.inSentence":    "CPU",
		"resource.memory":            "memory",
		"resource.memory.inSentence": "memory",

		"serverOffline.title":       "⚠️ Server offline",
		"serverOffline.description": "The server **%s** has disconnected or is offline.",
		"serverOnline.title":        "✅ Server online",
		"serverOnline.description":  "The server **%s** is online again.",
		"resourceAlert.title":       "⚠️ High %s usage",
		"resourceAlert.description": "The server **%s** is using **%.1f%%** of its %s (threshold: %.1f%%)",
		"backupAlert.successTitle":  "✅ Backup completed",
		"backupAlert.success":       "The backup of the server **%s** completed successfully.",
		"backupAlert.failedTitle":   "❌ Backup failed",
		"backupAlert.failed":        "The backup of the server **%s** failed: %s",
		"backupAlert.statusSuccess": "Completed successfully",
		"backupAlert.statusFailed":  "Failed while it was being made",
		"crashAlert.title":          "💥 Server crashed",
		"crashAlert.description":    "**%s** crashed (exit code %d).",
		"systemStatus.title":        "📊 System status",
		"systemStatus.footer":       "Updated every minute",
		"systemStatus.total":        "**Total servers:** %d",
		"systemStatus.averages":     "**Average resources (online servers):**",
		"systemStatus.memory":       "• Memory: %.1f MB",
		"systemStatus.showing":      "Showing %d of %d servers",
		"nodeStatus.title":          "🖥️ Node status",
		"nodeStatus.localNode":      "Local node",
		"nodeStatus.node":           "**Node:** %s",
		"nodeStatus.address":        "**Address:** %s",
		"nodeStatus.active":         "🟢 Active",
		"nodeStatus.noAnswer":       "🔴 No answer",
		"nodeStatus.unreachable":    "The node does not answer: the state of its servers is unknown.",
		"discordTest.title":         "🧪 Discord webhook test",
		"discordTest.description":   "This is a test message to check that the Discord webhook is set up correctly.",
		"discordTest.type":          "Test message",
		"discordTest.status":        "✅ Set up correctly",
		"discordTest.system":        "System",
		"discordTest.node":          "Node",
		"discordTest.transfers":     "Transfers",

		"notify.crashTitle":        "💥 Server crashed",
		"notify.crash":             "%s crashed (exit code %d). %s",
		"notify.notRestarted":      "It was not restarted automatically.",
		"notify.restarting":        "It is restarting by itself (attempt %d of %d).",
		"notify.backupTitle":       "✅ Backup completed",
		"notify.backup":            "The backup of %s was saved successfully.",
		"notify.backupFailedTitle": "❌ Backup failed",
		"notify.backupFailed":      "The backup of %s could not be completed. Check the server's console.",
		"notify.resourceTitle":     "⚠️ High %s usage",
		"notify.resource":          "%s is using %.0f%% of its %s (the alert starts at %.0f%%).",
		"notify.startedTitle":      "🟢 Server started",
		"notify.started":           "%s is online.",
		"notify.stoppedTitle":      "⚪ Server stopped",
		"notify.stopped":           "%s has stopped.",
		"notify.testTitle":         "🔔 Notifications connected",
		"notify.test":              "From now on, the alerts of %s will arrive here.",

		"extransfer.report":                      "External transfer report",
		"extransfer.reason":                      "Reason: %s",
		"extransfer.status.completed":            "Completed",
		"extransfer.status.completedOrigin":      "Completed (origin)",
		"extransfer.status.failed":               "Failed",
		"extransfer.status.failedOrigin":         "Failed (origin)",
		"extransfer.status.cancelled":            "Cancelled",
		"extransfer.status.cancelledOrigin":      "Cancelled (origin)",
		"extransfer.status.cancelledDestination": "Cancelled (destination)",
		"extransfer.completed":                   "The external transfer finished successfully.",
		"extransfer.destinationConfirmed":        "The destination panel confirmed it received the server's data.",
		"extransfer.destinationCancelled":        "The destination panel cancelled the transfer.",
		"extransfer.originCancelled":             "The origin panel cancelled the transfer.",
		"extransfer.ownerCancelledOrigin":        "The owner cancelled the transfer from the origin panel.",
		"extransfer.ownerCancelledDestination":   "The owner cancelled the transfer from the destination panel.",
		"extransfer.archiveFailed":               "The files could not be compressed on the origin server.",
		"extransfer.expired":                     "The destination panel stopped answering and the transfer expired before it finished.",

		"extransfer.STEP_STARTING":            "Starting...",
		"extransfer.STEP_VALIDATING":          "Checking the connection with the origin panel...",
		"extransfer.STEP_ORIGIN_START":        "Starting the transfer on the origin server...",
		"extransfer.STEP_WAITING_ARCHIVE":     "Waiting for the origin to compress the files...",
		"extransfer.STEP_DOWNLOADING":         "Downloading the data package from the origin...",
		"extransfer.STEP_UPLOADING":           "Uploading the files to the local daemon (this can take a while)...",
		"extransfer.STEP_EXTRACTING":          "Unpacking the files...",
		"extransfer.STEP_CLEANUP":             "Removing temporary files...",
		"extransfer.ERROR_ORIGIN_URL":         "The origin URL is not valid or points to an address that is not allowed",
		"extransfer.ERROR_ORIGIN_UNREACHABLE": "Network error while connecting to the origin",
		"extransfer.ERROR_TOKEN_REJECTED":     "Invalid or expired token",
		"extransfer.ERROR_ORIGIN_BAD_ANSWER":  "The origin sent an invalid answer",
		"extransfer.ERROR_ORIGIN_BAD_SESSION": "The origin returned an invalid session ID",
		"extransfer.ERROR_ORIGIN_START":       "The transfer could not be started on the origin",
		"extransfer.ERROR_ORIGIN_REFUSED":     "The origin refused the transfer",
		"extransfer.ERROR_DOWNLOAD_NETWORK":   "Network error while downloading the package",
		"extransfer.ERROR_DOWNLOAD_REFUSED":   "The origin refused the download",
		"extransfer.ERROR_DOWNLOAD_TIMEOUT":   "The origin did not prepare the package in time",
		"extransfer.ERROR_UPLOAD":             "The file could not be uploaded to the daemon",
		"extransfer.ERROR_EXTRACT":            "The files could not be unpacked on the destination",
	},
}

// IsNotificationLanguage reports whether the catalogue has texts in lang
func IsNotificationLanguage(lang string) bool {
	_, ok := notificationTexts[lang]
	return ok
}

// NotificationLanguageHeader carries the panel's notification language on its status poll of
// each remote node, so the alerts of servers there follow the panel's setting
const NotificationLanguageHeader = "X-Aether-Notification-Language"

// panelNotificationLanguage is the language the panel last sent this node. It is kept in memory
// only: the panel repeats it on every status poll (every 30 seconds), so a node that restarts
// picks it up again on the next one.
var panelNotificationLanguage atomic.Value

// SetPanelNotificationLanguage records the language the panel sent; an empty or unknown value
// is ignored, so an older panel that sends none leaves the node as it was
func SetPanelNotificationLanguage(lang string) bool {
	if !IsNotificationLanguage(lang) {
		return false
	}
	panelNotificationLanguage.Store(lang)
	return true
}

// PanelNotificationLanguage is the language the panel last sent this node, "" if none yet
func PanelNotificationLanguage() string {
	lang, _ := panelNotificationLanguage.Load().(string)
	return lang
}

// NotificationLanguage is the language of the messages this process sends: the one set in its
// own config file or environment, else the one the panel sent (on a remote node), else the
// default
func NotificationLanguage() string {
	return resolveNotificationLanguage(config.NotificationLanguage.IsSet(), config.NotificationLanguage.Value(), PanelNotificationLanguage())
}

// resolveNotificationLanguage picks the language. A value set on purpose on this machine wins
// over the panel's: an operator who sets a node's language in its own config means it for that
// node. On the panel itself the setting is always its own, as the settings page saves it there.
func resolveNotificationLanguage(setHere bool, own, fromPanel string) string {
	if setHere && IsNotificationLanguage(own) {
		return own
	}
	if IsNotificationLanguage(fromPanel) {
		return fromPanel
	}
	if IsNotificationLanguage(own) {
		return own
	}
	return DefaultNotificationLanguage
}

// NotificationText is the text for key in the panel's notification language, with args
// filled in
func NotificationText(key string, args ...interface{}) string {
	return NotificationTextIn(NotificationLanguage(), key, args...)
}

// NotificationTextIn is the text for key in lang, with args filled in. A key missing from
// lang falls back to English, and a key missing everywhere is returned as it is, so a typo
// shows up in the message instead of an empty one.
func NotificationTextIn(lang, key string, args ...interface{}) string {
	text, ok := notificationTexts[lang][key]
	if !ok {
		text, ok = notificationTexts["en"][key]
	}
	if !ok {
		return key
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}
