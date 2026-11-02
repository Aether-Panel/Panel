package servers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func step(op, condition string, args map[string]interface{}) skypanel.ConditionalMetadataType {
	return skypanel.ConditionalMetadataType{If: condition, MetadataType: skypanel.MetadataType{Type: op, Metadata: args}}
}

func TestOperationProcess_LaterStepsSeeVariablesSetBefore(t *testing.T) {
	p, dir := newBackupTestServer(t)
	steps := []skypanel.ConditionalMetadataType{
		// hands back the version in the output variable, as with an empty version it would
		// after finding the installed one
		step("resolveneoforgeversion", "", map[string]interface{}{"version": "21.4.150", "outputVariable": "neoforgeVersion"}),
		step("writefile", "", map[string]interface{}{"target": "version.txt", "text": "${neoforgeVersion}"}),
	}

	process, err := GenerateProcess(steps, p.RunningEnvironment, p.DataToMap(), p.Execution.EnvironmentVariables)
	require.NoError(t, err)
	require.NoError(t, process.Run(p))

	content, err := os.ReadFile(filepath.Join(dir, "version.txt"))
	require.NoError(t, err)
	assert.Equal(t, "21.4.150", string(content), "the token was replaced before the first step had run")
}

func TestOperationProcess_PostStepsSeeHowTheServerStopped(t *testing.T) {
	steps := []skypanel.ConditionalMetadataType{
		step("writefile", "!success", map[string]interface{}{"target": "crashed.txt", "text": "${exitCode}"}),
		step("writefile", "success", map[string]interface{}{"target": "clean.txt", "text": "x"}),
		step("writefile", "exitCode == 3", map[string]interface{}{"target": "code.txt", "text": "x"}),
		step("writefile", "", map[string]interface{}{"target": "always.txt", "text": "x"}),
	}
	exists := func(dir, name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}

	t.Run("after a crash", func(t *testing.T) {
		p, dir := newBackupTestServer(t)
		data := p.DataToMap()
		data["exitCode"] = 3
		process, err := GenerateProcess(steps, p.RunningEnvironment, data, nil)
		require.NoError(t, err)
		require.NoError(t, process.RunWith(p, map[string]interface{}{"success": false, "exitCode": 3}))

		assert.True(t, exists(dir, "crashed.txt"), `"if": "!success" never ran: success was always true`)
		assert.False(t, exists(dir, "clean.txt"))
		assert.True(t, exists(dir, "code.txt"))
		assert.True(t, exists(dir, "always.txt"), "a step without a condition always runs")
		content, _ := os.ReadFile(filepath.Join(dir, "crashed.txt"))
		assert.Equal(t, "3", string(content))
	})

	t.Run("after a clean stop", func(t *testing.T) {
		p, dir := newBackupTestServer(t)
		process, err := GenerateProcess(steps, p.RunningEnvironment, p.DataToMap(), nil)
		require.NoError(t, err)
		require.NoError(t, process.RunWith(p, map[string]interface{}{"success": true, "exitCode": 0}))

		assert.False(t, exists(dir, "crashed.txt"))
		assert.True(t, exists(dir, "clean.txt"))
		assert.False(t, exists(dir, "code.txt"))
		assert.True(t, exists(dir, "always.txt"))
	})
}

func TestOperationProcess_StopsAtTheFirstFailure(t *testing.T) {
	p, dir := newBackupTestServer(t)
	steps := []skypanel.ConditionalMetadataType{
		step("no-such-operation", "", nil),
		step("writefile", "", map[string]interface{}{"target": "after.txt", "text": "x"}),
	}
	process, err := GenerateProcess(steps, p.RunningEnvironment, p.DataToMap(), nil)
	require.NoError(t, err)

	assert.ErrorIs(t, process.Run(p), skypanel.ErrMissingFactory)
	_, err = os.Stat(filepath.Join(dir, "after.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestDestroy_DeletesTheEnvironmentWhenUninstallFails(t *testing.T) {
	previous := config.ServersFolder.Value()
	_ = config.ServersFolder.Set(t.TempDir(), false)
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })

	root := filepath.Join(config.ServersFolder.Value(), "destroytest")
	require.NoError(t, os.MkdirAll(root, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "world.dat"), []byte("world"), 0644))

	p := CreateProgram()
	p.Identifier = "destroytest"
	p.RunningEnvironment = &skypanel.Environment{
		RootDirectory:  root,
		ConsoleBuffer:  skypanel.CreateCache(),
		ConsoleTracker: skypanel.CreateTracker(),
		Implementation: &fakeEnvironment{},
	}
	// a broken uninstall step: the server is being deleted anyway
	p.Uninstallation = []skypanel.ConditionalMetadataType{step("no-such-operation", "", nil)}

	assert.NoError(t, p.Destroy())
	_, err := os.Stat(root)
	assert.True(t, os.IsNotExist(err), "the server's folder must be deleted even when uninstall fails")
	assert.Contains(t, consoleText(p), "Failed to uninstall server")
}
