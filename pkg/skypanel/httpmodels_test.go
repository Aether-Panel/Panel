package skypanel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerStats_CPUPercent(t *testing.T) {
	cases := []struct {
		cpu, max, want float64
	}{
		{206, 200, 100}, // two cores, a little over: capped
		{100, 200, 50},  // one of two cores
		{50, 50, 100},   // half a core, all of it
		{120, 0, 120},   // no allowance known: as reported
		{-3, 100, 0},
	}
	for _, c := range cases {
		s := ServerStats{CPU: c.cpu, MaxCPU: c.max}
		assert.InDelta(t, c.want, s.CPUPercent(), 0.001, "%v of %v", c.cpu, c.max)
	}
}
