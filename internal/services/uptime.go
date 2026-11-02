package services

import (
	"errors"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"gorm.io/gorm"
)

// newestStartFirst orders uptime rows with the latest one first
const newestStartFirst = "start_time DESC"

type Uptime struct {
	DB *gorm.DB
}

// TrackStatus registra o actualiza el estado de uptime/downtime de un servidor
func (us *Uptime) TrackStatus(serverID string, isRunning bool) error {
	// Verificar que el servidor existe en la base de datos antes de trackear (evitar fallos de FK)
	var count int64
	if err := us.DB.Table("servers").Where("identifier = ?", serverID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		// El servidor no existe en la BD, no podemos trackear su uptime aún
		return nil
	}

	// Buscar si hay un registro activo (sin EndTime) para este servidor
	var currentStatus *models.UptimeStatus
	err := us.DB.Where("server_id = ? AND end_time IS NULL", serverID).Order(newestStartFirst).First(&currentStatus).Error

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

// RecordNodeStatuses records what a remote node reported for its servers. serverIDs are the
// servers the panel places on that node; reported holds the state of those the node answered
// for, and is nil when the node did not answer at all.
//
// A node that does not answer is not taken as "all its servers stopped": nothing is recorded
// for it and its open periods stay as they are. Only a period nobody has confirmed for
// staleAfter is closed, at the moment it was last confirmed, so a long outage counts as
// neither up nor down (the rows have no "unknown" state, a gap between periods is how they
// say it). When the node answers again a new period starts from then.
func (us *Uptime) RecordNodeStatuses(serverIDs []string, reported map[string]bool, staleAfter time.Duration) error {
	if err := us.closeStalePeriods(serverIDs, time.Now().Add(-staleAfter)); err != nil {
		return err
	}

	var errs []error
	for _, serverID := range serverIDs {
		running, ok := reported[serverID]
		if !ok {
			// the node did not say: unknown, not stopped
			continue
		}
		if err := us.TrackStatus(serverID, running); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// closeStalePeriods ends the open periods of these servers that were last confirmed (TrackStatus
// saves the row on every reading, which moves updated_at) before cutoff, at that confirmation
func (us *Uptime) closeStalePeriods(serverIDs []string, cutoff time.Time) error {
	if len(serverIDs) == 0 {
		return nil
	}

	var stale []*models.UptimeStatus
	err := us.DB.Where("server_id IN ? AND end_time IS NULL AND updated_at < ?", serverIDs, cutoff).Find(&stale).Error
	if err != nil {
		return err
	}

	for _, period := range stale {
		end := period.UpdatedAt
		if end.Before(period.StartTime) {
			end = period.StartTime
		}
		// update through the loaded row: the model's save hook validates it, and an empty
		// model would fail on the missing server id
		err = us.DB.Model(period).Updates(map[string]interface{}{
			"end_time": end,
			"duration": int64(end.Sub(period.StartTime).Seconds()),
		}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// uptimePeriod is the part of an uptime row the stats need
type uptimePeriod struct {
	ServerID  string
	IsRunning bool
	StartTime time.Time
	EndTime   *time.Time
}

// secondsInWindow is how much of the period falls between since and now. A period that
// started before since only counts from since on, and the open period (no end) lasts until now.
func (p uptimePeriod) secondsInWindow(since, now time.Time) int64 {
	start := p.StartTime
	if start.Before(since) {
		start = since
	}
	end := now
	if p.EndTime != nil && p.EndTime.Before(now) {
		end = *p.EndTime
	}
	if !end.After(start) {
		return 0
	}
	return int64(end.Sub(start).Seconds())
}

// periodsSince reads the periods that overlap the window: those still open and those that
// ended after since. Filtering on the start instead would drop a server that has been up the
// whole window, because its only period started before it.
func periodsSince(db *gorm.DB, since time.Time) ([]uptimePeriod, error) {
	var periods []uptimePeriod
	err := db.Model(&models.UptimeStatus{}).
		Select("server_id, is_running, start_time, end_time").
		Where("(end_time IS NULL OR end_time > ?)", since).
		Find(&periods).Error
	return periods, err
}

// uptimePercentage is the share of the known time the server was up; with no data it counts as 100%
func uptimePercentage(uptimeSeconds, downtimeSeconds int64) float64 {
	total := uptimeSeconds + downtimeSeconds
	if total <= 0 {
		return 100.0
	}
	return float64(uptimeSeconds) / float64(total) * 100.0
}

// GetUptimeStats returns how long the server was up and down between since and now,
// the running period included
func (us *Uptime) GetUptimeStats(serverID string, since time.Time) (uptimeSeconds, downtimeSeconds int64, uptimePercent float64, err error) {
	periods, err := periodsSince(us.DB.Where("server_id = ?", serverID), since)
	if err != nil {
		return
	}

	now := time.Now()
	for _, period := range periods {
		if period.IsRunning {
			uptimeSeconds += period.secondsInWindow(since, now)
		} else {
			downtimeSeconds += period.secondsInWindow(since, now)
		}
	}

	uptimePercent = uptimePercentage(uptimeSeconds, downtimeSeconds)
	return
}

// GetRecentHistory obtiene el historial reciente de uptime/downtime
func (us *Uptime) GetRecentHistory(serverID string, limit int) ([]*models.UptimeStatus, error) {
	var records []*models.UptimeStatus
	err := us.DB.Where("server_id = ?", serverID).
		Order(newestStartFirst).
		Limit(limit).
		Find(&records).Error
	return records, err
}

// GetAllServerUptime returns, per server, how long it was up and down between since and now,
// plus the state it is in now and since when
func (us *Uptime) GetAllServerUptime(since time.Time) (map[string]map[string]interface{}, error) {
	periods, err := periodsSince(us.DB, since)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	serverStats := make(map[string]map[string]interface{})
	for _, period := range periods {
		addUptimeDuration(serverStats, period.ServerID, period.IsRunning, period.secondsInWindow(since, now))
		if period.EndTime == nil {
			serverStats[period.ServerID]["currentStatus"] = period.IsRunning
			serverStats[period.ServerID]["currentStartTime"] = period.StartTime
		}
	}

	for _, stats := range serverStats {
		stats["uptimePercent"] = uptimePercentage(stats["uptime"].(int64), stats["downtime"].(int64))
	}

	return serverStats, nil
}

// addUptimeDuration adds the seconds to the server's uptime or downtime, creating its entry if needed
func addUptimeDuration(serverStats map[string]map[string]interface{}, serverID string, running bool, duration int64) {
	if _, exists := serverStats[serverID]; !exists {
		serverStats[serverID] = make(map[string]interface{})
		serverStats[serverID]["uptime"] = int64(0)
		serverStats[serverID]["downtime"] = int64(0)
	}
	if running {
		serverStats[serverID]["uptime"] = serverStats[serverID]["uptime"].(int64) + duration
	} else {
		serverStats[serverID]["downtime"] = serverStats[serverID]["downtime"].(int64) + duration
	}
}
