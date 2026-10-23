package servers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func newMetricsTestServer(t *testing.T) *Server {
	previous := config.ServersFolder.Value()
	_ = config.ServersFolder.Set(t.TempDir(), false)
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })
	p := CreateProgram()
	p.Identifier = "metricstest"
	t.Cleanup(func() { forgetMetrics("metricstest") })
	return p
}

func TestMetrics_MinutePointsAreAverages(t *testing.T) {
	p := newMetricsTestServer(t)
	start := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)

	// two samples in the first minute, one in the second
	p.recordMetrics(&skypanel.ServerStats{CPU: 10, Memory: 100, MaxMemory: 1000, Running: true}, start)
	p.recordMetrics(&skypanel.ServerStats{CPU: 30, Memory: 300, MaxMemory: 1000, Running: false}, start.Add(30*time.Second))
	p.recordMetrics(&skypanel.ServerStats{CPU: 50, Memory: 500, MaxMemory: 1000, Running: true}, start.Add(70*time.Second))

	points := p.Metrics(time.Hour, time.Minute, start.Add(90*time.Second))
	if assert.Len(t, points, 2, "a closed minute and the one still being filled") {
		assert.Equal(t, start.Unix(), points[0].Time)
		assert.InDelta(t, 20, points[0].CPU, 0.001)
		assert.InDelta(t, 200, points[0].Memory, 0.001)
		assert.InDelta(t, 0.5, points[0].Online, 0.001)
		assert.InDelta(t, 50, points[1].CPU, 0.001)
	}
}

func TestMetrics_ResampleAndWindow(t *testing.T) {
	p := newMetricsTestServer(t)
	start := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 120; i++ {
		p.recordMetrics(&skypanel.ServerStats{CPU: float64(i), Running: true}, start.Add(time.Duration(i)*time.Minute))
	}
	now := start.Add(120 * time.Minute)

	hour := p.Metrics(time.Hour, time.Minute, now)
	assert.LessOrEqual(t, len(hour), 61)
	assert.GreaterOrEqual(t, hour[0].Time, now.Add(-time.Hour).Unix())

	day := p.Metrics(24*time.Hour, 5*time.Minute, now)
	if assert.Len(t, day, 24) {
		assert.InDelta(t, 2, day[0].CPU, 0.001, "minutes 0-4 average to 2")
	}
}

func TestMetrics_HistorySurvivesARestart(t *testing.T) {
	p := newMetricsTestServer(t)
	start := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	p.recordMetrics(&skypanel.ServerStats{CPU: 42, Running: true}, start)
	p.recordMetrics(&skypanel.ServerStats{CPU: 42, Running: true}, start.Add(time.Minute))
	SaveAllMetrics()

	info, err := os.Stat(filepath.Join(config.ServersFolder.Value(), "metricstest.metrics"))
	if assert.NoError(t, err) {
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}

	forgetMetrics("metricstest")
	points := p.Metrics(time.Hour, time.Minute, start.Add(2*time.Minute))
	if assert.NotEmpty(t, points) {
		assert.InDelta(t, 42, points[0].CPU, 0.001)
	}
}
