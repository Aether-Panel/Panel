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
	// NoData marks a step nothing was recorded in (the node was down): the charts leave it
	// blank and count it as offline instead of drawing a line across it
	NoData bool `json:"noData,omitempty"`
	// Players is the average number of people online and PlayersPeak the most at once;
	// HasPlayers is false when the game did not answer any query in the bucket
	Players     float64 `json:"players"`
	PlayersPeak int     `json:"playersPeak"`
	MaxPlayers  int     `json:"maxPlayers"`
	HasPlayers  bool    `json:"hasPlayers"`
} // @name MetricPoint

type metricBucket struct {
	start time.Time
	// count is every sample, running only those taken while the server ran: CPU, memory and
	// network are averaged over the running ones, so an hour half spent stopped does not halve them
	count, running                   float64
	cpu, mem, maxMem, rx, tx, online float64
	playerSamples, players           float64
	playersPeak, maxPlayers          int
}

func (b *metricBucket) addPlayers(online, slots int) {
	b.playerSamples++
	b.players += float64(online)
	if online > b.playersPeak {
		b.playersPeak = online
	}
	if slots > b.maxPlayers {
		b.maxPlayers = slots
	}
}

func (b *metricBucket) fillPlayers(point *MetricPoint) {
	if b.playerSamples == 0 {
		return
	}
	point.HasPlayers = true
	point.Players = b.players / b.playerSamples
	point.PlayersPeak = b.playersPeak
	point.MaxPlayers = b.maxPlayers
}

func (b *metricBucket) add(s *skypanel.ServerStats) {
	b.count++
	if !s.Running {
		return
	}
	b.running++
	b.online++
	// CPU as a share of what the server may use, the same number its live charts show
	b.cpu += s.CPUPercent()
	b.mem += s.Memory
	b.maxMem += s.MaxMemory
	b.rx += s.NetworkRx
	b.tx += s.NetworkTx
}

func (b *metricBucket) point() MetricPoint {
	point := MetricPoint{Time: b.start.Unix()}
	if b.running > 0 {
		point.CPU = b.cpu / b.running
		point.Memory = b.mem / b.running
		point.MaxMemory = b.maxMem / b.running
		point.NetRx = b.rx / b.running
		point.NetTx = b.tx / b.running
	}
	switch {
	case b.count > 0:
		point.Online = b.online / b.count
	case b.playerSamples > 0:
		// only player counts arrived: a game answering queries is running
		point.Online = 1
	}
	b.fillPlayers(&point)
	return point
}

func (b *metricBucket) empty() bool {
	return b.count == 0 && b.playerSamples == 0
}

type metricHistory struct {
	Minutes []MetricPoint `json:"minutes"`
	Quarter []MetricPoint `json:"quarter"`

	// RecordPlayers is the most people ever seen online at once, and RecordAt when (Unix seconds)
	RecordPlayers int   `json:"recordPlayers"`
	RecordAt      int64 `json:"recordAt"`

	minute  metricBucket
	quarter metricBucket
	saved   time.Time
}

// roll closes the buckets that ended before now and opens the current ones
func (h *metricHistory) roll(now time.Time) {
	minuteStart := now.Truncate(minuteStep)
	if !h.minute.start.Equal(minuteStart) {
		if !h.minute.empty() {
			h.Minutes = appendCapped(h.Minutes, h.minute.point(), keepMinutes)
		}
		h.minute = metricBucket{start: minuteStart}
	}
	quarterStart := now.Truncate(quarterStep)
	if !h.quarter.start.Equal(quarterStart) {
		if !h.quarter.empty() {
			h.Quarter = appendCapped(h.Quarter, h.quarter.point(), keepQuarter)
		}
		h.quarter = metricBucket{start: quarterStart}
	}
}

func (h *metricHistory) maybeSave(serverID string, now time.Time) {
	if now.Sub(h.saved) < metricsSaveEvery {
		return
	}
	h.saved = now
	if err := saveHistory(serverID, h); err != nil {
		logging.Error.Printf("Error saving performance history for %s: %s", serverID, err)
	}
}

// recordPlayers folds one player count into the server's history
func (p *Server) recordPlayers(online, slots int, now time.Time) {
	metricHistoriesLock.Lock()
	defer metricHistoriesLock.Unlock()
	h := historyFor(p.ID())
	h.roll(now)
	h.minute.addPlayers(online, slots)
	h.quarter.addPlayers(online, slots)
	if online > h.RecordPlayers {
		h.RecordPlayers = online
		h.RecordAt = now.Unix()
	}
	h.maybeSave(p.ID(), now)
}

// PlayerRecord is the most people ever seen online at once, and when
func (p *Server) PlayerRecord() (players int, at int64) {
	metricHistoriesLock.Lock()
	defer metricHistoriesLock.Unlock()
	h := historyFor(p.ID())
	return h.RecordPlayers, h.RecordAt
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
	h.roll(now)
	h.minute.add(stats)
	h.quarter.add(stats)
	h.maybeSave(p.ID(), now)
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

	if !current.empty() {
		source = append(source, current.point())
	}
	since := now.Add(-window).Unix()
	stepSeconds := int64(step / time.Second)
	return fillGaps(resample(source, since, stepSeconds), stepSeconds, now.Unix())
}

// fillGaps adds a NoData point for every step with nothing recorded between the first point
// and now: the node was down then, so the server was too. Without them the chart drew a line
// across the hours and the uptime only counted the minutes that had samples.
func fillGaps(points []MetricPoint, step, now int64) []MetricPoint {
	if len(points) == 0 || step <= 0 {
		return points
	}
	last := now - now%step
	filled := make([]MetricPoint, 0, len(points))
	for i, pt := range points {
		filled = append(filled, pt)
		next := last + step // after the last point: up to the current step
		if i+1 < len(points) {
			next = points[i+1].Time
		}
		// the step still running (last) may simply not have its first sample yet
		for t := pt.Time + step; t < next && t < last; t += step {
			filled = append(filled, MetricPoint{Time: t, NoData: true})
		}
	}
	return filled
}

// resample averages points into steps of the given size, dropping those before since. CPU,
// memory and network are weighted by how long each point's server ran, as in a bucket.
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
		if pt.Time < since || pt.NoData {
			continue
		}
		t := pt.Time - pt.Time%step
		if t != bucketTime {
			flush()
			bucket = metricBucket{}
			bucketTime = t
		}
		bucket.addPoint(pt)
	}
	flush()
	return result
}

// addPoint folds an already averaged point into a coarser bucket, weighting its resources by
// how long its server ran
func (b *metricBucket) addPoint(pt MetricPoint) {
	b.count++
	b.online += pt.Online
	b.running += pt.Online
	b.cpu += pt.CPU * pt.Online
	b.mem += pt.Memory * pt.Online
	b.maxMem += pt.MaxMemory * pt.Online
	b.rx += pt.NetRx * pt.Online
	b.tx += pt.NetTx * pt.Online
	if !pt.HasPlayers {
		return
	}
	b.playerSamples++
	b.players += pt.Players
	b.playersPeak = max(b.playersPeak, pt.PlayersPeak)
	b.maxPlayers = max(b.maxPlayers, pt.MaxPlayers)
}
