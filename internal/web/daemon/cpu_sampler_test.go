package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCurrentCPUUsage_AnswersAtOnceOnceSampled(t *testing.T) {
	start := time.Now()
	first := currentCPUUsage()
	assert.Less(t, time.Since(start), time.Second, "even the first reading does not hold the request a second")
	assert.GreaterOrEqual(t, first, 0.0)

	deadline := time.Now().Add(5 * time.Second)
	for !cpuSampler.ready.Load() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	assert.True(t, cpuSampler.ready.Load(), "the background sampler has a reading")

	start = time.Now()
	usage := currentCPUUsage()
	assert.Less(t, time.Since(start), 50*time.Millisecond, "later readings come from the sampler")
	assert.GreaterOrEqual(t, usage, 0.0)
	assert.LessOrEqual(t, usage, 100.0)
}
