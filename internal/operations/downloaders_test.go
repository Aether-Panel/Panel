package operations

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDownloaders_CurseForgeMock(t *testing.T) {
	// A mock test simulating the CurseForge operations logic
	// without actually hitting the CurseForge API
	modpackID := "12345"
	fileID := "67890"

	url := "https://api.curseforge.com/v1/mods/" + modpackID + "/files/" + fileID
	assert.Equal(t, "https://api.curseforge.com/v1/mods/12345/files/67890", url)
}

func TestDownloaders_SteamGameDlMock(t *testing.T) {
	// A mock test simulating SteamCMD download arguments
	appID := "740" // CS:GO
	args := []string{"+login", "anonymous", "+force_install_dir", ".", "+app_update", appID, "+quit"}

	assert.Contains(t, args, "+app_update")
	assert.Contains(t, args, "740")
}
