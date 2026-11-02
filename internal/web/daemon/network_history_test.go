package daemon

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNetworkHistory_AveragesEachMinute(t *testing.T) {
	h := &networkHistory{}
	start := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	var sent, recv uint64
	// 10 KB/s down and 2 KB/s up for three minutes, a reading every 10 s
	for s := 0; s <= 180; s += 10 {
		h.sample(start.Add(time.Duration(s)*time.Second), sent, recv)
		recv += 10 * 1024 * 10
		sent += 2 * 1024 * 10
	}

	got := h.history(time.Hour, time.Minute, start.Add(3*time.Minute))
	assert.Equal(t, int64(60), got.Step)
	if assert.Len(t, got.Points, 4, "three full minutes and the one being filled") {
		for _, p := range got.Points[:3] {
			assert.InDelta(t, 10, p.Rx, 0.01)
			assert.InDelta(t, 2, p.Tx, 0.01)
		}
	}
}

func TestNetworkHistory_GapsAndResets(t *testing.T) {
	h := &networkHistory{}
	start := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	h.sample(start, 1000, 1000)
	h.sample(start.Add(10*time.Second), 1000+10240, 1000+10240)
	// the counters went back (an interface came up again): not a negative rate
	h.sample(start.Add(20*time.Second), 5, 5)
	// the node was off for an hour: nothing is spread over that hour
	h.sample(start.Add(80*time.Minute), 5+10240, 5+10240)

	got := h.history(2*time.Hour, time.Minute, start.Add(81*time.Minute))
	if assert.Len(t, got.Points, 1, "only the minute that was measured") {
		assert.InDelta(t, 1, got.Points[0].Rx, 0.01)
		assert.Equal(t, start.Unix(), got.Points[0].T)
	}
	for _, p := range got.Points {
		assert.GreaterOrEqual(t, p.Rx, 0.0)
	}
}

func TestNetworkHistory_StepsAndRetention(t *testing.T) {
	now := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	h := &networkHistory{}
	for m := 0; m < 26*60; m++ {
		h.minutes = append(h.minutes, NetworkPoint{T: now.Add(-time.Duration(26*60-m) * time.Minute).Unix(), Rx: 4, Tx: 1})
	}
	h.closeMinute(now)
	assert.LessOrEqual(t, len(h.minutes), 24*60, "a day is kept")

	got := h.history(24*time.Hour, 15*time.Minute, now)
	assert.Equal(t, int64(900), got.Step)
	assert.InDelta(t, 96, len(got.Points), 1)
	assert.InDelta(t, 4, got.Points[0].Rx, 0.01)
}

func TestNetworkHistory_SurvivesARestart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "network.history")
	now := time.Now().Truncate(time.Minute)
	h := &networkHistory{minutes: []NetworkPoint{
		{T: now.Add(-48 * time.Hour).Unix(), Rx: 9, Tx: 9},
		{T: now.Add(-2 * time.Minute).Unix(), Rx: 3, Tx: 1},
	}}
	assert.NoError(t, h.save(file))

	loaded := &networkHistory{}
	loaded.load(file, now)
	assert.Equal(t, []NetworkPoint{{T: now.Add(-2 * time.Minute).Unix(), Rx: 3, Tx: 1}}, loaded.minutes, "older than a day is dropped")
}

func TestSumPhysicalTraffic(t *testing.T) {
	listing := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 498287173 100 0 0 0 0 0 0 498287173 100 0 0 0 0 0 0
  eth0: 177212608208 900 0 0 0 0 0 0 2119733635 800 0 0 0 0 0 0
docker0: 172679825 10 0 0 0 0 0 0 24307171478 10 0 0 0 0 0 0
br-312d66d5fe59: 1768584813 10 0 0 0 0 0 0 142729890149 10 0 0 0 0 0 0
vethbc8c7ac: 177879191 10 0 0 0 0 0 0 103550870 10 0 0 0 0 0 0
eth0.100: 5 1 0 0 0 0 0 0 5 1 0 0 0 0 0 0
  ens4: 1000 1 0 0 0 0 0 0 2000 1 0 0 0 0 0 0
`
	sent, recv := sumPhysicalTraffic(listing)
	assert.Equal(t, uint64(177212608208+1000), recv, "only eth0 and ens4: containers' traffic already crossed eth0")
	assert.Equal(t, uint64(2119733635+2000), sent)
}
