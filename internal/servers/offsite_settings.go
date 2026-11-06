package servers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// OffsiteSettings is a server's own periodic off-site copy: how often (0 = never) and how
// many of those copies stay in the bucket. nil follows the panel's setting. It lives in its
// own 0600 file next to the server, on the node that runs it.
type OffsiteSettings struct {
	PeriodicHours *int `json:"periodicHours"`
	PeriodicKeep  *int `json:"periodicKeep"`
}

// Validate refuses an interval the panel does not offer and a number of copies out of 1-50
func (s OffsiteSettings) Validate() error {
	if s.PeriodicHours != nil && !ValidPeriodicHours(*s.PeriodicHours) {
		return skypanel.ErrOffsitePeriodicHours
	}
	if s.PeriodicKeep != nil {
		if _, err := OffsiteKeep(*s.PeriodicKeep); err != nil {
			return skypanel.ErrOffsiteKeep
		}
	}
	return nil
}

// OffsiteStatus is what the server's page shows: its own setting, what it ends up using, and
// when its last periodic copy was made (RFC3339, nil when none is known)
type OffsiteStatus struct {
	OffsiteSettings
	EffectiveHours int     `json:"effectiveHours"`
	EffectiveKeep  int     `json:"effectiveKeep"`
	Configured     bool    `json:"configured"`
	LastPeriodic   *string `json:"lastPeriodic"`
}

var offsiteSettingsLock sync.Mutex

func offsiteSettingsFile(serverID string) string {
	return filepath.Join(config.ServersFolder.Value(), serverID+".offsite")
}

// OffsiteSettings reads the server's own setting; without a file everything follows the panel
func (p *Server) OffsiteSettings() (OffsiteSettings, error) {
	offsiteSettingsLock.Lock()
	defer offsiteSettingsLock.Unlock()
	data, err := os.ReadFile(offsiteSettingsFile(p.ID()))
	if os.IsNotExist(err) {
		return OffsiteSettings{}, nil
	}
	if err != nil {
		return OffsiteSettings{}, err
	}
	var s OffsiteSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return OffsiteSettings{}, err
	}
	return s, nil
}

// SetOffsiteSettings validates and saves the server's own setting
func (p *Server) SetOffsiteSettings(s OffsiteSettings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	offsiteSettingsLock.Lock()
	defer offsiteSettingsLock.Unlock()
	return os.WriteFile(offsiteSettingsFile(p.ID()), data, 0600)
}

// EffectivePeriodic is the interval in hours (0 = never) and the copies to keep this server
// uses: its own values where it set them, the panel's otherwise
func (p *Server) EffectivePeriodic() (hours, keep int) {
	hours, keep, err := p.periodicSettings()
	if err != nil {
		p.Log(logging.Error, "Error reading the off-site settings: %s", err)
	}
	return hours, keep
}

// periodicSettings is EffectivePeriodic with the error reading the server's own setting, which
// the periodic copy needs: the panel's values are only a guess for a server that may have
// turned its copy off
func (p *Server) periodicSettings() (hours, keep int, err error) {
	own, err := p.OffsiteSettings()
	hours, keep = effectivePeriodic(own)
	return hours, keep, err
}

// effectivePeriodic lays a server's own values over the panel's. A value the panel does not
// offer (a file edited by hand) turns the copy off, or keeps the default number of copies,
// rather than being used as it is.
func effectivePeriodic(own OffsiteSettings) (hours, keep int) {
	hours, keep = config.BackupsS3PeriodicHours.Value(), config.BackupsS3PeriodicKeep.Value()
	if own.PeriodicHours != nil {
		hours = *own.PeriodicHours
	}
	if own.PeriodicKeep != nil {
		keep = *own.PeriodicKeep
	}
	if !ValidPeriodicHours(hours) {
		hours = 0
	}
	if _, err := OffsiteKeep(keep); err != nil {
		keep = defaultOffsiteKeep
	}
	return hours, keep
}

// OffsiteStatus gathers what the server's page shows about its periodic off-site copy
func (p *Server) OffsiteStatus() (OffsiteStatus, error) {
	own, err := p.OffsiteSettings()
	if err != nil {
		return OffsiteStatus{}, err
	}
	status := OffsiteStatus{OffsiteSettings: own, Configured: bucket.Configured()}
	status.EffectiveHours, status.EffectiveKeep = effectivePeriodic(own)
	if last, ok := LastPeriodic(p.ID()); ok {
		formatted := last.UTC().Format(time.RFC3339)
		status.LastPeriodic = &formatted
	}
	return status, nil
}
