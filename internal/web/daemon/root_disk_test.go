package daemon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiskLabelRank_PrefersRootThenTheServersFolder(t *testing.T) {
	servers := "/var/lib/SkyPanel/servers"
	assert.Less(t, diskLabelRank("/", servers), diskLabelRank("/var/lib/SkyPanel", servers))
	assert.Less(t, diskLabelRank("/var/lib/SkyPanel", servers), diskLabelRank("/etc/SkyPanel", servers))
	assert.Less(t, diskLabelRank("/repo", servers), diskLabelRank("/var/log/SkyPanel", servers), "otherwise the shortest")
	assert.NotEqual(t, 1, diskLabelRank("/var/lib/Sky", servers), "a name prefix is not a parent folder")
}
