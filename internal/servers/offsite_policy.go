package servers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/offsite"
	"github.com/gofrs/uuid/v5"
)

// OffsiteBackupPrefix names the copies the periodic schedule makes, so they never get mixed
// with the ones a task makes or with backups made by hand
const OffsiteBackupPrefix = "offsite-"

// OffsiteFrequency says how often a task run may upload its backup to the bucket
type OffsiteFrequency string

const (
	OffsiteNever    OffsiteFrequency = "never"
	OffsiteAlways   OffsiteFrequency = "always"
	OffsiteWeekend  OffsiteFrequency = "weekend"
	OffsiteWeekly   OffsiteFrequency = "weekly"
	OffsiteBiweekly OffsiteFrequency = "biweekly"
	OffsiteMonthly  OffsiteFrequency = "monthly"
)

const (
	defaultOffsiteKeep = 4
	maxOffsiteKeep     = 50
	// biweeklySlack keeps a task that runs every 14 days from skipping a turn because
	// the previous upload finished a few minutes late
	biweeklySlack = 2 * time.Hour
	// suffix is "-" plus a 36 character uuid
	backupUUIDSuffixLen = 37
)

var periodicHours = map[int]bool{0: true, 12: true, 24: true, 48: true, 72: true, 168: true, 336: true, 720: true}

// ParseOffsiteFrequency reads the value of a task's options; a missing one means "always" so
// tasks created before this option existed keep uploading every backup
func ParseOffsiteFrequency(v interface{}) (OffsiteFrequency, error) {
	if v == nil {
		return OffsiteAlways, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("invalid off-site frequency %v", v)
	}
	switch f := OffsiteFrequency(s); f {
	case "":
		return OffsiteAlways, nil
	case OffsiteNever, OffsiteAlways, OffsiteWeekend, OffsiteWeekly, OffsiteBiweekly, OffsiteMonthly:
		return f, nil
	}
	return "", fmt.Errorf("invalid off-site frequency %q", s)
}

// OffsiteKeep is how many copies to keep in the bucket: 1-50, 4 when not set
func OffsiteKeep(v interface{}) (int, error) {
	if v == nil {
		return defaultOffsiteKeep, nil
	}
	var n int
	switch t := v.(type) {
	case int:
		n = t
	case int64:
		n = int(t)
	case float64: // what encoding/json gives for numbers
		n = int(t)
		if float64(n) != t {
			return 0, fmt.Errorf("invalid off-site keep %v", v)
		}
	default:
		return 0, fmt.Errorf("invalid off-site keep %v", v)
	}
	if n < 1 || n > maxOffsiteKeep {
		return 0, fmt.Errorf("off-site keep must be between 1 and %d", maxOffsiteKeep)
	}
	return n, nil
}

// ValidPeriodicHours says whether h is an interval the periodic schedule offers (0 = off)
func ValidPeriodicHours(h int) bool {
	return periodicHours[h]
}

// offsiteDue decides whether a run at now should upload, given when the last copy was made
// (zero when there is none). Calendar boundaries use now's location, so the admin's timezone
// decides when a week starts.
func offsiteDue(f OffsiteFrequency, last time.Time, now time.Time) bool {
	var since time.Time // upload when nothing newer than this exists
	switch f {
	case OffsiteAlways:
		return true
	case OffsiteWeekend:
		wd := now.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			return false
		}
		back := 0 // days since Saturday
		if wd == time.Sunday {
			back = 1
		}
		since = time.Date(now.Year(), now.Month(), now.Day()-back, 0, 0, 0, 0, now.Location())
	case OffsiteWeekly:
		back := (int(now.Weekday()) + 6) % 7 // days since Monday
		since = time.Date(now.Year(), now.Month(), now.Day()-back, 0, 0, 0, 0, now.Location())
	case OffsiteMonthly:
		since = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case OffsiteBiweekly:
		return last.IsZero() || now.Sub(last) >= 15*24*time.Hour-biweeklySlack
	default: // never, or anything unknown
		return false
	}
	return last.Before(since)
}

// taskBackupPrefix is the start of the name of every backup a task makes
func taskBackupPrefix(taskID string) string {
	return "auto-" + taskID + "-"
}

// taskIDFromBackupName extracts the task from "auto-<task>-<uuid>.tar.gz"; names without a
// task (older auto- copies) and other kinds of backup report false
func taskIDFromBackupName(name string) (string, bool) {
	if !strings.HasPrefix(name, "auto-") || !strings.HasSuffix(name, ".tar.gz") {
		return "", false
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(name, "auto-"), ".tar.gz")
	if len(rest) <= backupUUIDSuffixLen {
		return "", false
	}
	cut := len(rest) - backupUUIDSuffixLen
	if rest[cut] != '-' {
		return "", false
	}
	if _, err := uuid.FromString(rest[cut+1:]); err != nil {
		return "", false
	}
	return rest[:cut], true
}

// newestBeyond returns the names to delete so only the keep newest objects remain, oldest first
func newestBeyond(objects []offsite.Object, keep int) []string {
	if len(objects) <= keep {
		return nil
	}
	sorted := append([]offsite.Object(nil), objects...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ModTime.Before(sorted[j].ModTime) })
	names := make([]string, 0, len(sorted)-keep)
	for _, o := range sorted[:len(sorted)-keep] {
		names = append(names, o.FileName)
	}
	return names
}
