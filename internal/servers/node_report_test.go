package servers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Discord node report used to show the panel's own CPU, RAM and OS for every node. A
// remote node's figures now come from its daemon's /daemon/system answer.
func TestNodeSystemInfo_Hardware(t *testing.T) {
	var info nodeSystemInfo
	require.NoError(t, json.Unmarshal([]byte(`{
		"hostname": "node2", "os": "linux", "platform": "debian", "platformVersion": "12.5",
		"cpuModel": "AMD EPYC 7302", "cpuCores": 16, "cpuThreads": 32,
		"memoryTotal": 68719476736, "memoryUsed": 17179869184
	}`), &info))

	hw := info.hardware()

	assert.Equal(t, "AMD EPYC 7302", hw.cpuModel)
	assert.Equal(t, 16, hw.cpuCores)
	assert.Equal(t, 32, hw.cpuThreads)
	assert.Zero(t, hw.cpuGhz, "the daemon does not send the frequency")
	assert.EqualValues(t, 68719476736, hw.memTotal)
	assert.EqualValues(t, 17179869184, hw.memUsed)
	assert.Equal(t, "debian 12.5", hw.osName)
}

func TestNodeSystemInfo_HardwareWithoutDetails(t *testing.T) {
	hw := nodeSystemInfo{OS: "linux"}.hardware()

	assert.Equal(t, unknownCPUModel, hw.cpuModel, "an unknown model is left out of the report")
	assert.Equal(t, "linux", hw.osName)
}
