package services

import (
	"time"

	"github.com/SkyPanel/SkyPanel/v3/models"
	"gorm.io/gorm"
)

type Uptime struct {
	DB *gorm.DB
}

// TrackStatus registra o actualiza el estado de uptime/downtime de un servidor
func (us *Uptime) TrackStatus(serverID string, isRunning bool) error {
	// Buscar si hay un registro activo (sin EndTime) para este servidor
	var currentStatus *models.UptimeStatus
	err := us.DB.Where("server_id = ? AND end_time IS NULL", serverID).Order("start_time DESC").First(&currentStatus).Error

	now := time.Now()

	if err == gorm.ErrRecordNotFound {
		// No hay registro activo, crear uno nuevo
		newStatus := &models.UptimeStatus{
			ServerID:  serverID,
			IsRunning: isRunning,
			StartTime: now,
		}
		return us.DB.Create(newStatus).Error
	} else if err != nil {
		return err
	}

	// Si el estado cambió, cerrar el registro anterior y crear uno nuevo
	if currentStatus.IsRunning != isRunning {
		// Calcular duración
		duration := int64(now.Sub(currentStatus.StartTime).Seconds())
		currentStatus.Duration = duration
		currentStatus.EndTime = &now

		// Guardar el registro anterior
		if err := us.DB.Save(currentStatus).Error; err != nil {
			return err
		}

		// Crear nuevo registro para el nuevo estado
		newStatus := &models.UptimeStatus{
			ServerID:  serverID,
			IsRunning: isRunning,
			StartTime: now,
		}
		return us.DB.Create(newStatus).Error
	}

	// Si el estado no cambió, actualizar el timestamp
	currentStatus.UpdatedAt = now
	return us.DB.Save(currentStatus).Error
}

// GetUptimeStats obtiene estadísticas de uptime para un servidor
func (us *Uptime) GetUptimeStats(serverID string, since time.Time) (uptimeSeconds, downtimeSeconds int64, uptimePercent float64, err error) {
	var results []struct {
		IsRunning bool
		Duration  int64
	}

	query := us.DB.Model(&models.UptimeStatus{}).
		Where("server_id = ? AND start_time >= ?", serverID, since).
		Select("is_running, SUM(duration) as duration").
		Group("is_running")

	err = query.Find(&results).Error
	if err != nil {
		return
	}

	var totalSeconds int64
	for _, result := range results {
		if result.IsRunning {
			uptimeSeconds += result.Duration
		} else {
			downtimeSeconds += result.Duration
		}
		totalSeconds += result.Duration
	}

	// Si hay un registro activo sin EndTime, calcular su duración hasta ahora
	var activeStatus *models.UptimeStatus
	activeErr := us.DB.Where("server_id = ? AND end_time IS NULL", serverID).
		Order("start_time DESC").First(&activeStatus).Error

	if activeErr == nil && activeStatus != nil {
		activeDuration := int64(time.Since(activeStatus.StartTime).Seconds())
		if activeStatus.IsRunning {
			uptimeSeconds += activeDuration
		} else {
			downtimeSeconds += activeDuration
		}
		totalSeconds += activeDuration
	}

	if totalSeconds > 0 {
		uptimePercent = float64(uptimeSeconds) / float64(totalSeconds) * 100.0
	} else {
		uptimePercent = 100.0 // Si no hay datos, asumir 100% uptime
	}

	return
}

// GetRecentHistory obtiene el historial reciente de uptime/downtime
func (us *Uptime) GetRecentHistory(serverID string, limit int) ([]*models.UptimeStatus, error) {
	var records []*models.UptimeStatus
	err := us.DB.Where("server_id = ?", serverID).
		Order("start_time DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// GetAllServerUptime obtiene estadísticas de uptime para todos los servidores
func (us *Uptime) GetAllServerUptime(since time.Time) (map[string]map[string]interface{}, error) {
	var results []struct {
		ServerID        string
		IsRunning       bool
		TotalDuration   int64
		CurrentRunning  bool
		CurrentStartTime time.Time
	}

	// Obtener duración total por estado
	query := us.DB.Model(&models.UptimeStatus{}).
		Where("start_time >= ?", since).
		Select("server_id, is_running, SUM(duration) as total_duration").
		Group("server_id, is_running")

	err := query.Find(&results).Error
	if err != nil {
		return nil, err
	}

	// Obtener estados actuales
	var activeStatuses []models.UptimeStatus
	err = us.DB.Where("end_time IS NULL").Find(&activeStatuses).Error
	if err != nil {
		return nil, err
	}

	activeMap := make(map[string]*models.UptimeStatus)
	for i := range activeStatuses {
		activeMap[activeStatuses[i].ServerID] = &activeStatuses[i]
	}

	// Agrupar por servidor
	serverStats := make(map[string]map[string]interface{})
	for _, result := range results {
		if _, exists := serverStats[result.ServerID]; !exists {
			serverStats[result.ServerID] = make(map[string]interface{})
			serverStats[result.ServerID]["uptime"] = int64(0)
			serverStats[result.ServerID]["downtime"] = int64(0)
		}
		if result.IsRunning {
			serverStats[result.ServerID]["uptime"] = serverStats[result.ServerID]["uptime"].(int64) + result.TotalDuration
		} else {
			serverStats[result.ServerID]["downtime"] = serverStats[result.ServerID]["downtime"].(int64) + result.TotalDuration
		}
	}

	// Agregar estados actuales
	for serverID, status := range activeMap {
		if _, exists := serverStats[serverID]; !exists {
			serverStats[serverID] = make(map[string]interface{})
			serverStats[serverID]["uptime"] = int64(0)
			serverStats[serverID]["downtime"] = int64(0)
		}
		activeDuration := int64(time.Since(status.StartTime).Seconds())
		if status.IsRunning {
			serverStats[serverID]["uptime"] = serverStats[serverID]["uptime"].(int64) + activeDuration
		} else {
			serverStats[serverID]["downtime"] = serverStats[serverID]["downtime"].(int64) + activeDuration
		}
		serverStats[serverID]["currentStatus"] = status.IsRunning
		serverStats[serverID]["currentStartTime"] = status.StartTime
	}

	// Calcular porcentajes
	for _, stats := range serverStats {
		uptime := stats["uptime"].(int64)
		downtime := stats["downtime"].(int64)
		total := uptime + downtime

		if total > 0 {
			stats["uptimePercent"] = float64(uptime) / float64(total) * 100.0
		} else {
			stats["uptimePercent"] = 100.0
		}
	}

	return serverStats, nil
}
