package daemon

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

// NetworkPoint is the node's traffic over one step: KB/s received and sent, averaged over the
// time measured in it. T is the step's start, in Unix seconds.
type NetworkPoint struct {
	T  int64   `json:"t"`
	Rx float64 `json:"rx"`
	Tx float64 `json:"tx"`
}

// NetworkHistory is what /daemon/network/history answers: the points of the range, Step
// seconds apart. A step the node did not measure (it was down) has no point.
type NetworkHistory struct {
	Step   int64          `json:"step"`
	Points []NetworkPoint `json:"points"`
}

// NetworkRanges are the ranges the dashboard offers, with the step each is drawn at
var NetworkRanges = map[string]time.Duration{
	"1h":  time.Minute,
	"6h":  5 * time.Minute,
	"24h": 15 * time.Minute,
}

var networkRangeWindows = map[string]time.Duration{"1h": time.Hour, "6h": 6 * time.Hour, "24h": 24 * time.Hour}

const (
	networkSampleEvery = 10 * time.Second
	networkKeep        = 24 * time.Hour
	networkSaveEvery   = 5 * time.Minute
	// a longer gap between two readings (the host was suspended) is not spread over the minute
	networkMaxGap = 5 * time.Minute
)

// networkHistory keeps the node's traffic for the last 24 hours, a point a minute, on disk
// across restarts. The dashboard used to build it in the browser, so it only held what one
// page had seen while it stayed open: a few points and long empty stretches.
type networkHistory struct {
	mu      sync.Mutex
	minutes []NetworkPoint

	// the minute being filled: KB received and sent, and the seconds they were measured over
	minute          int64
	rx, tx, seconds float64

	lastSent, lastRecv uint64
	lastAt             time.Time
}

var nodeNetwork = &networkHistory{}

// StartNetworkHistory loads the saved history and samples the node's traffic in the background
func StartNetworkHistory() {
	go func() {
		defer skypanel.Recover()
		file := networkHistoryFile()
		nodeNetwork.load(file, time.Now())
		saved := time.Now()
		for {
			sent, recv := totalNetworkBytes()
			now := time.Now()
			nodeNetwork.sample(now, sent, recv)
			if now.Sub(saved) >= networkSaveEvery {
				if err := nodeNetwork.save(file); err != nil {
					logging.Error.Printf("Could not save the node's network history: %s", err)
				}
				saved = now
			}
			time.Sleep(networkSampleEvery)
		}
	}()
}

func networkHistoryFile() string {
	return filepath.Join(filepath.Dir(config.ServersFolder.Value()), "network.history")
}

// sample adds the traffic since the last reading to the current minute. Counters that went
// backwards (an interface came back, the counter wrapped) only start a new reading.
func (h *networkHistory) sample(now time.Time, sent, recv uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	minute := now.Truncate(time.Minute).Unix()
	if minute != h.minute {
		h.closeMinute(now)
		h.minute = minute
	}
	elapsed := now.Sub(h.lastAt)
	if !h.lastAt.IsZero() && sent >= h.lastSent && recv >= h.lastRecv && elapsed > 0 && elapsed <= networkMaxGap {
		h.rx += float64(recv-h.lastRecv) / 1024
		h.tx += float64(sent-h.lastSent) / 1024
		h.seconds += elapsed.Seconds()
	}
	h.lastSent, h.lastRecv, h.lastAt = sent, recv, now
}

// closeMinute stores the minute being filled, if anything was measured in it, and drops
// what is older than a day
func (h *networkHistory) closeMinute(now time.Time) {
	if h.seconds > 0 && h.minute != 0 {
		h.minutes = append(h.minutes, NetworkPoint{T: h.minute, Rx: round2(h.rx / h.seconds), Tx: round2(h.tx / h.seconds)})
	}
	h.rx, h.tx, h.seconds = 0, 0, 0
	cutoff := now.Add(-networkKeep).Unix()
	i := 0
	for i < len(h.minutes) && h.minutes[i].T < cutoff {
		i++
	}
	h.minutes = h.minutes[i:]
}

// history returns the minutes of the window, the one being filled included, averaged to step
func (h *networkHistory) history(window, step time.Duration, now time.Time) NetworkHistory {
	h.mu.Lock()
	minutes := append([]NetworkPoint{}, h.minutes...)
	if h.seconds > 0 {
		minutes = append(minutes, NetworkPoint{T: h.minute, Rx: round2(h.rx / h.seconds), Tx: round2(h.tx / h.seconds)})
	}
	h.mu.Unlock()

	stepSeconds := int64(step / time.Second)
	cutoff := now.Add(-window).Unix()
	type acc struct {
		rx, tx float64
		n      int
	}
	buckets := make(map[int64]*acc)
	for _, m := range minutes {
		if m.T < cutoff {
			continue
		}
		key := m.T - m.T%stepSeconds
		b := buckets[key]
		if b == nil {
			b = &acc{}
			buckets[key] = b
		}
		b.rx += m.Rx
		b.tx += m.Tx
		b.n++
	}
	points := make([]NetworkPoint, 0, len(buckets))
	for t, b := range buckets {
		points = append(points, NetworkPoint{T: t, Rx: round2(b.rx / float64(b.n)), Tx: round2(b.tx / float64(b.n))})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].T < points[j].T })
	return NetworkHistory{Step: stepSeconds, Points: points}
}

func (h *networkHistory) load(file string, now time.Time) {
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	var minutes []NetworkPoint
	if json.Unmarshal(data, &minutes) != nil {
		return
	}
	cutoff := now.Add(-networkKeep).Unix()
	kept := make([]NetworkPoint, 0, len(minutes))
	for _, m := range minutes {
		if m.T >= cutoff && m.Rx >= 0 && m.Tx >= 0 {
			kept = append(kept, m)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].T < kept[j].T })
	h.mu.Lock()
	h.minutes = kept
	h.mu.Unlock()
}

func (h *networkHistory) save(file string) error {
	h.mu.Lock()
	data, err := json.Marshal(h.minutes)
	h.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err = os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// @Summary Node network history
// @Description The node's traffic over the last hour, 6 hours or day (range=1h|6h|24h)
// @Success 200 {object} NetworkHistory
// @Tags Daemon
// @Router /daemon/network/history [get]
// @Security OAuth2Application[nodes.view]
func getNetworkHistory(c *gin.Context) {
	name := c.DefaultQuery("range", "1h")
	step, ok := NetworkRanges[name]
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "range must be 1h, 6h or 24h"})
		return
	}
	c.JSON(http.StatusOK, nodeNetwork.history(networkRangeWindows[name], step, time.Now()))
}
