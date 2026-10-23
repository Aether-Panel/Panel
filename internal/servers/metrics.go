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

// Performance history: the node already samples every server every few seconds for alerts;
// those samples are folded into one-minute points (kept for a day) and fifteen-minute
// points (kept for a week), so owners can see how the server behaved while nobody watched.

const (
	minuteStep  = time.Minute
	quarterStep = 15 * time.Minute
	keepMinutes = 24 * 60
	keepQuarter = 7 * 24 * 4
	// metricsSaveEvery is how often the history is written to disk, so a restart loses at
	// most this much
	metricsSaveEvery = 5 * time.Minute
)

// MetricPoint is the average of the samples in one bucket
type MetricPoint struct {
	// Time is the start of the bucket, in Unix seconds
	Time      int64   `json:"t"`
	CPU       float64 `json:"cpu"`
	Memory    float64 `json:"memory"`
	MaxMemory float64 `json:"maxMemory"`
	NetRx     float64 `json:"rx"`
	NetTx     float64 `json:"tx"`
	// Online is the share of samples taken while the server was running (0 to 1)
	Online float64 `json:"online"`
} // @name MetricPoint

type metricBucket struct {
	start                            time.Time
	count                            float64
	cpu, mem, maxMem, rx, tx, online float64
}

func (b *metricBucket) add(s *skypanel.ServerStats) {
	b.count++
	b.cpu += s.CPU
	b.mem += s.Memory
	b.maxMem += s.MaxMemory
	b.rx += s.NetworkRx
	b.tx += s.NetworkTx
	if s.Running {
		b.online++
	}
}

func (b *metricBucket) point() MetricPoint {
	if b.count == 0 {
		return MetricPoint{Time: b.start.Unix()}
	}
	return MetricPoint{
		Time:      b.start.Unix(),
		CPU:       b.cpu / b.count,
		Memory:    b.mem / b.count,
		MaxMemory: b.maxMem / b.count,
		NetRx:     b.rx / b.count,
		NetTx:     b.tx / b.count,
		Online:    b.online / b.count,
	}
}

type metricHistory struct {
	Minutes []MetricPoint `json:"minutes"`
	Quarter []MetricPoint `json:"quarter"`

	minute  metricBucket
	quarter metricBucket
	saved   time.Time
}

var (
	metricHistories     = make(map[string]*metricHistory)
	metricHistoriesLock sync.Mutex
)

func metricsFile(serverID string) string {
	return filepath.Join(config.ServersFolder.Value(), serverID+".metrics")
}

// history returns the server's history, reading it from disk the first time. The caller
// holds metricHistoriesLock.
func historyFor(serverID string) *metricHistory {
	h, ok := metricHistories[serverID]
	if ok {
		return h
	}
	h = &metricHistory{}
	if data, err := os.ReadFile(metricsFile(serverID)); err == nil {
		_ = json.Unmarshal(data, h)
	}
	metricHistories[serverID] = h
	return h
}

// recordMetrics folds one stats sample into the server's history
func (p *Server) recordMetrics(stats *skypanel.ServerStats, now time.Time) {
	if stats == nil {
		return
	}
	metricHistoriesLock.Lock()
	defer metricHistoriesLock.Unlock()
	h := historyFor(p.ID())

	minuteStart := now.Truncate(minuteStep)
	if !h.minute.start.Equal(minuteStart) {
		if h.minute.count > 0 {
			h.Minutes = appendCapped(h.Minutes, h.minute.point(), keepMinutes)
		}
		h.minute = metricBucket{start: minuteStart}
	}
	h.minute.add(stats)

	quarterStart := now.Truncate(quarterStep)
	if !h.quarter.start.Equal(quarterStart) {
		if h.quarter.count > 0 {
			h.Quarter = appendCapped(h.Quarter, h.quarter.point(), keepQuarter)
		}
		h.quarter = metricBucket{start: quarterStart}
	}
	h.quarter.add(stats)

	if now.Sub(h.saved) >= metricsSaveEvery {
		h.saved = now
		if err := saveHistory(p.ID(), h); err != nil {
			logging.Error.Printf("Error saving performance history for %s: %s", p.ID(), err)
		}
	}
}

func appendCapped(points []MetricPoint, point MetricPoint, keep int) []MetricPoint {
	points = append(points, point)
	if len(points) > keep {
		points = append([]MetricPoint(nil), points[len(points)-keep:]...)
	}
	return points
}

func saveHistory(serverID string, h *metricHistory) error {
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return os.WriteFile(metricsFile(serverID), data, 0600)
}

// SaveAllMetrics writes every history to disk; called when the node shuts down
func SaveAllMetrics() {
	metricHistoriesLock.Lock()
	defer metricHistoriesLock.Unlock()
	for id, h := range metricHistories {
		if err := saveHistory(id, h); err != nil {
			logging.Error.Printf("Error saving performance history for %s: %s", id, err)
		}
	}
}

func forgetMetrics(serverID string) {
	metricHistoriesLock.Lock()
	delete(metricHistories, serverID)
	metricHistoriesLock.Unlock()
}

// MetricRanges are the windows the panel offers, with the step their points are averaged to
var MetricRanges = map[string]struct {
	Window time.Duration
	Step   time.Duration
}{
	"1h":  {time.Hour, minuteStep},
	"24h": {24 * time.Hour, 5 * time.Minute},
	"7d":  {7 * 24 * time.Hour, time.Hour},
}

// Metrics returns the history for a window, including the bucket still being filled, with
// points averaged to the window's step
func (p *Server) Metrics(window, step time.Duration, now time.Time) []MetricPoint {
	metricHistoriesLock.Lock()
	h := historyFor(p.ID())
	var source []MetricPoint
	var current metricBucket
	if step < quarterStep {
		source = append(source, h.Minutes...)
		current = h.minute
	} else {
		source = append(source, h.Quarter...)
		current = h.quarter
	}
	metricHistoriesLock.Unlock()

	if current.count > 0 {
		source = append(source, current.point())
	}
	since := now.Add(-window).Unix()
	return resample(source, since, int64(step/time.Second))
}

// resample averages points into steps of the given size, dropping those before since
func resample(points []MetricPoint, since, step int64) []MetricPoint {
	result := make([]MetricPoint, 0)
	var bucket metricBucket
	var bucketTime int64 = -1
	flush := func() {
		if bucket.count > 0 {
			p := bucket.point()
			p.Time = bucketTime
			result = append(result, p)
		}
	}
	for _, pt := range points {
		if pt.Time < since {
			continue
		}
		t := pt.Time - pt.Time%step
		if t != bucketTime {
			flush()
			bucket = metricBucket{}
			bucketTime = t
		}
		bucket.count++
		bucket.cpu += pt.CPU
		bucket.mem += pt.Memory
		bucket.maxMem += pt.MaxMemory
		bucket.rx += pt.NetRx
		bucket.tx += pt.NetTx
		bucket.online += pt.Online
	}
	flush()
	return result
}
