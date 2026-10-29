package servers

import (
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func op(kind string, args map[string]interface{}) skypanel.ConditionalMetadataType {
	return skypanel.ConditionalMetadataType{MetadataType: skypanel.MetadataType{Type: kind, Metadata: args}}
}

func TestVersionTarget(t *testing.T) {
	p := CreateProgram()
	p.Variables = map[string]skypanel.Variable{"version": {Value: "1.20.4"}, "project": {Value: "folia"}}

	p.Installation = []skypanel.ConditionalMetadataType{op("mkdir", nil), op("paperdl", map[string]interface{}{"project": "${project}", "minecraftVersion": "${version}"})}
	platform, variable, ok := p.versionTarget()
	assert.True(t, ok)
	assert.Equal(t, "folia", platform)
	assert.Equal(t, "version", variable)

	p.Installation = []skypanel.ConditionalMetadataType{op("mojangdl", map[string]interface{}{"version": "${version}"})}
	platform, _, ok = p.versionTarget()
	assert.True(t, ok)
	assert.Equal(t, "vanilla", platform)

	// a fixed version, or a token for a variable the server does not have, cannot be switched
	p.Installation = []skypanel.ConditionalMetadataType{op("mojangdl", map[string]interface{}{"version": "1.21"})}
	_, _, ok = p.versionTarget()
	assert.False(t, ok)
	p.Installation = []skypanel.ConditionalMetadataType{op("paperdl", map[string]interface{}{"minecraftVersion": "${mc}"})}
	_, _, ok = p.versionTarget()
	assert.False(t, ok)
}

func TestSortVersionsDesc(t *testing.T) {
	assert.Equal(t, []string{"1.21.4", "1.21", "1.20.6", "1.9"}, sortVersionsDesc([]string{"1.9", "1.20.6", "1.21.4", "1.21"}))
}

func TestChangeVersion_RefusesUnknownVersionsAndTemplatesWithout(t *testing.T) {
	p := CreateProgram()
	assert.ErrorIs(t, p.ChangeVersion("1.21"), ErrNoVersionSwitch)

	p.Variables = map[string]skypanel.Variable{"version": {Value: "1.20.4"}}
	p.Installation = []skypanel.ConditionalMetadataType{op("paperdl", map[string]interface{}{"project": "paper", "minecraftVersion": "${version}"})}
	versionCache["paper"] = cachedVersions{versions: []string{"1.21.4", "1.20.4"}, fetched: time.Now()}
	defer delete(versionCache, "paper")
	assert.ErrorIs(t, p.ChangeVersion("9.9.9"), ErrUnknownVersion)
}

func TestBuildVariable(t *testing.T) {
	p := CreateProgram()
	p.Variables = map[string]skypanel.Variable{"version": {Value: "1.20.4"}, "build": {Value: "496"}}
	p.Installation = []skypanel.ConditionalMetadataType{op("paperdl", map[string]interface{}{"minecraftVersion": "${version}", "build": "${build}"})}
	assert.Equal(t, "build", p.buildVariable())

	p.Installation = []skypanel.ConditionalMetadataType{op("paperdl", map[string]interface{}{"minecraftVersion": "${version}", "build": "latest"})}
	assert.Equal(t, "", p.buildVariable())
}

func TestListVersions_RefusesAnUnknownPlatform(t *testing.T) {
	// the name becomes part of the PaperMC URL, so only the known ones are fetched
	_, err := ListVersions("../../evil")
	assert.ErrorIs(t, err, ErrUnknownPlatform)
}

func TestListVersions_KnownPlatformUsesTheCache(t *testing.T) {
	versionCacheLock.Lock()
	versionCache["folia"] = cachedVersions{versions: []string{"1.21.4", "1.21.3"}, fetched: time.Now()}
	versionCacheLock.Unlock()
	t.Cleanup(func() {
		versionCacheLock.Lock()
		delete(versionCache, "folia")
		versionCacheLock.Unlock()
	})

	versions, err := ListVersions("folia")
	assert.NoError(t, err)
	assert.Equal(t, []string{"1.21.4", "1.21.3"}, versions)
}

// The Minecraft template downloads the game in a different step for each launcher; only the
// step of the server's own launcher counts
func TestVersionTarget_FollowsTheModLauncher(t *testing.T) {
	when := func(cond string, o skypanel.ConditionalMetadataType) skypanel.ConditionalMetadataType {
		o.If = cond
		return o
	}
	p := CreateProgram()
	p.Installation = []skypanel.ConditionalMetadataType{
		when("modlauncher == ''", op("mojangdl", map[string]interface{}{"version": "${version}"})),
		when("modlauncher == 'fabric'", op("mojangdl", map[string]interface{}{"version": "${version}"})),
		when("modlauncher == 'paper'", op("paperdl", map[string]interface{}{"minecraftVersion": "${version}", "build": "${paperbuild}"})),
		when("modlauncher == 'purpur'", op("download", map[string]interface{}{"files": []string{"https://example.invalid/${version}"}})),
	}
	launch := func(launcher string) {
		p.Variables = map[string]skypanel.Variable{"version": {Value: "1.21.4"}, "paperbuild": {Value: "latest"}, "modlauncher": {Value: launcher}}
	}

	launch("paper")
	platform, _, ok := p.versionTarget()
	assert.True(t, ok)
	assert.Equal(t, "paper", platform, "a Paper server is not offered vanilla's versions")
	assert.Equal(t, "paperbuild", p.buildVariable())

	launch("")
	platform, _, ok = p.versionTarget()
	assert.True(t, ok)
	assert.Equal(t, "vanilla", platform)
	assert.Equal(t, "vanilla", p.serverSoftware(platform))
	assert.Equal(t, "", p.buildVariable(), "the Paper build is not vanilla's")

	launch("fabric")
	platform, _, _ = p.versionTarget()
	assert.Equal(t, "vanilla", platform)
	assert.Equal(t, "fabric", p.serverSoftware(platform), "a Fabric server is not called vanilla")

	launch("purpur")
	_, _, ok = p.versionTarget()
	assert.False(t, ok, "no version list the panel knows: the card is hidden instead of naming another server")
}
