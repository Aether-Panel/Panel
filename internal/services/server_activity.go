package services

import (
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"gorm.io/gorm"
)

// ServerActivityRetention is how long a server's activity is kept, the same quarter as the
// panel's sign-in feed
const ServerActivityRetention = ActivityRetention

// serverActivityPruneEvery is how often recording an entry also clears out old ones
const serverActivityPruneEvery = time.Hour

// ServerActivityMaxLimit caps a page of the activity log
const ServerActivityMaxLimit = 100

// ServerActivityFamilies groups action keys the way the activity log filters them. An entry
// ending in "*" matches every action starting with what comes before it; anything else must
// match exactly.
var ServerActivityFamilies = map[string][]string{
	"power":    {"server.start", "server.stop", "server.restart", "server.kill", "server.install", "server.reload"},
	"console":  {"console.*"},
	"files":    {"file.*", "plugin.*"},
	"backups":  {"backup.*"},
	"users":    {"user.*"},
	"settings": {"settings.*", "server.rename", "server.suspend", "server.unsuspend", "server.transfer", "server.create", "server.change", "extransfer.*"},
	"tasks":    {"task.*"},
}

// a free-form filter is a plain action prefix; keeping it to letters and dots means no LIKE
// wildcard can reach the query
var serverActivityPrefixPattern = regexp.MustCompile(`^[a-z]+(\.[a-z]*)?$`)

// lastServerActivityPrune is the unix time of the last retention pass, shared by every
// request so the pass runs at most once per serverActivityPruneEvery
var lastServerActivityPrune atomic.Int64

type ServerActivity struct {
	DB *gorm.DB
}

// Record stores one entry, trimming the detail to what the column keeps. Every so often it
// also drops entries past retention and those of servers that no longer exist.
func (s *ServerActivity) Record(entry *models.ServerActivity) error {
	entry.Detail = TruncateRunes(entry.Detail, models.ServerActivityDetailMax)
	entry.Username = TruncateRunes(entry.Username, 100)
	entry.IP = TruncateRunes(entry.IP, 64)

	if err := s.DB.Create(entry).Error; err != nil {
		return err
	}

	now := time.Now()
	last := lastServerActivityPrune.Load()
	if now.Sub(time.Unix(last, 0)) >= serverActivityPruneEvery && lastServerActivityPrune.CompareAndSwap(last, now.Unix()) {
		return s.Prune(now)
	}
	return nil
}

// Prune drops entries older than the retention window and entries whose server was deleted
// by any path the activity log does not see (node removal, migrations, direct DB edits)
func (s *ServerActivity) Prune(now time.Time) error {
	err := s.DB.
		Where("created_at < ?", now.Add(-ServerActivityRetention)).
		Delete(&models.ServerActivity{}).
		Error
	if err != nil {
		return err
	}

	return s.DB.
		Where("server_id NOT IN (?)", s.DB.Model(&models.Server{}).Select("identifier")).
		Delete(&models.ServerActivity{}).
		Error
}

// DeleteForServer removes a deleted server's whole log
func (s *ServerActivity) DeleteForServer(serverID string) error {
	return s.DB.Where("server_id = ?", serverID).Delete(&models.ServerActivity{}).Error
}

// List returns up to limit entries of a server, newest first, starting below the "before" id
// when it is not zero. filter is a family name from ServerActivityFamilies or an action
// prefix such as "file." or "backup.restore"; empty lists everything. The returned cursor is
// the "before" value for the next page, or nil when there is none.
func (s *ServerActivity) List(serverID string, limit int, before uint, filter string) ([]*models.ServerActivity, *uint, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > ServerActivityMaxLimit {
		limit = ServerActivityMaxLimit
	}

	tx := s.DB.Model(&models.ServerActivity{}).Where("server_id = ?", serverID)
	if before > 0 {
		tx = tx.Where("id < ?", before)
	}
	if patterns := serverActivityPatterns(filter); patterns != nil {
		cond := s.DB.Where("1 = 0")
		for _, p := range patterns {
			if prefix, ok := strings.CutSuffix(p, "*"); ok {
				cond = cond.Or("action LIKE ?", prefix+"%")
			} else {
				cond = cond.Or("action = ?", p)
			}
		}
		tx = tx.Where(cond)
	}

	entries := make([]*models.ServerActivity, 0, limit+1)
	if err := tx.Order("id DESC").Limit(limit + 1).Find(&entries).Error; err != nil {
		return nil, nil, err
	}

	var next *uint
	if len(entries) > limit {
		entries = entries[:limit]
		cursor := entries[limit-1].ID
		next = &cursor
	}
	return entries, next, nil
}

// serverActivityPatterns resolves a filter to the action patterns it covers: nil means no
// filtering. A filter that is neither a family nor a plain lowercase prefix matches nothing.
func serverActivityPatterns(filter string) []string {
	filter = strings.TrimSpace(filter)
	if filter == "" || filter == "all" {
		return nil
	}
	if family, ok := ServerActivityFamilies[filter]; ok {
		return family
	}
	if !serverActivityPrefixPattern.MatchString(filter) {
		return []string{}
	}
	if !strings.Contains(filter, ".") {
		filter += "."
	}
	return []string{filter + "*"}
}

// TruncateRunes shortens text to at most limit characters without splitting one, marking
// the cut with an ellipsis
func TruncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}
