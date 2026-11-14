package curseforge

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderForInstallerJar(t *testing.T) {
	// "neoforgedl" matched no jar, and Run had no case for it: NeoForge packs were refused
	assert.Equal(t, "neoforge", loaderForInstallerJar("neoforge-21.1.77-installer.jar"))
	assert.Equal(t, "forge", loaderForInstallerJar("forge-1.20.1-47.2.0-installer.jar"))
}

// fakeInstaller runs the fabric installer: it records the command and leaves the launcher
// the real installer writes
type fakeInstaller struct {
	root     string
	commands []string
}

func (f *fakeInstaller) ExecuteAsyncImpl(_ *skypanel.Environment, steps skypanel.ExecutionData) error {
	f.commands = append(f.commands, steps.Command)
	if err := os.WriteFile(filepath.Join(f.root, "fabric-server-launch.jar"), []byte("launcher"), 0644); err != nil {
		return err
	}
	steps.Callback(0)
	return nil
}
func (f *fakeInstaller) KillImpl(*skypanel.Environment) error { return nil }
func (f *fakeInstaller) GetStatsImpl(*skypanel.Environment) (*skypanel.ServerStats, error) {
	return &skypanel.ServerStats{}, nil
}
func (f *fakeInstaller) SendCodeImpl(*skypanel.Environment, int) error     { return nil }
func (f *fakeInstaller) GetUIDImpl(*skypanel.Environment) int              { return os.Getuid() }
func (f *fakeInstaller) GetGidImpl(*skypanel.Environment) int              { return os.Getgid() }
func (f *fakeInstaller) IsRunningImpl(*skypanel.Environment) (bool, error) { return false, nil }

// fabricServer answers the improved launcher with improvedStatus and counts the requests
// for the classic installer
func fabricServer(t *testing.T, improvedStatus int) *atomic.Int32 {
	installerHits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/improved":
			w.WriteHeader(improvedStatus)
			_, _ = w.Write([]byte("improved"))
		case "/installer":
			installerHits.Add(1)
			_, _ = w.Write([]byte("installer"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	improved, installer := ImprovedFabricInstallerURL, FabricInstallerURL
	ImprovedFabricInstallerURL, FabricInstallerURL = server.URL+"/improved", server.URL+"/installer"
	t.Cleanup(func() { ImprovedFabricInstallerURL, FabricInstallerURL = improved, installer })
	return installerHits
}

func fabricEnv(t *testing.T) (*skypanel.Environment, *fakeInstaller) {
	root := t.TempDir()
	impl := &fakeInstaller{root: root}
	return &skypanel.Environment{
		RootDirectory:  root,
		ConsoleBuffer:  skypanel.CreateCache(),
		ConsoleTracker: skypanel.CreateTracker(),
		Implementation: impl,
	}, impl
}

func TestInstallFabric_UsesTheImprovedLauncher(t *testing.T) {
	installerHits := fabricServer(t, http.StatusOK)
	env, impl := fabricEnv(t)

	require.NoError(t, installFabric(env, map[string]string{"mcVersion": "1.20.1", "version": "0.15.0"}, "java"))

	content, err := os.ReadFile(filepath.Join(env.RootDirectory, serverJarName))
	require.NoError(t, err)
	assert.Equal(t, "improved", string(content))
	assert.Zero(t, installerHits.Load())
	assert.Empty(t, impl.commands)
}

func TestInstallFabric_FallsBackToTheInstallerOn404(t *testing.T) {
	installerHits := fabricServer(t, http.StatusNotFound)
	env, impl := fabricEnv(t)

	require.NoError(t, installFabric(env, map[string]string{"mcVersion": "1.20.1", "version": "0.15.0"}, "java"))

	assert.Equal(t, int32(1), installerHits.Load(), "a 404 on the improved launcher must fall back to the installer")
	if assert.Len(t, impl.commands, 1) {
		assert.Equal(t, "java -jar fabric-installer.jar server -mcversion 1.20.1 -loader 0.15.0 -downloadMinecraft", impl.commands[0])
	}
	content, err := os.ReadFile(filepath.Join(env.RootDirectory, serverJarName))
	require.NoError(t, err)
	assert.Equal(t, "launcher", string(content))
	_, err = os.Stat(filepath.Join(env.RootDirectory, "fabric-installer.jar"))
	assert.True(t, os.IsNotExist(err), "the installer is removed afterwards")
}

func TestInstallFabric_OtherErrorsAreReturned(t *testing.T) {
	installerHits := fabricServer(t, http.StatusServiceUnavailable)
	env, impl := fabricEnv(t)

	assert.Error(t, installFabric(env, map[string]string{"mcVersion": "1.20.1", "version": "0.15.0"}, "java"))
	assert.Zero(t, installerHits.Load(), "only a 404 means the improved launcher does not exist")
	assert.Empty(t, impl.commands)
}
