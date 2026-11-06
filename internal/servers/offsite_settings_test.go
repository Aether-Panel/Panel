package servers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// usePeriodicGlobals sets the panel-wide periodic interval and copies to keep for this test
func usePeriodicGlobals(t *testing.T, hours, keep int) {
	previousHours, previousKeep := config.BackupsS3PeriodicHours.Value(), config.BackupsS3PeriodicKeep.Value()
	require.NoError(t, config.BackupsS3PeriodicHours.Set(hours, false))
	require.NoError(t, config.BackupsS3PeriodicKeep.Set(keep, false))
	t.Cleanup(func() {
		_ = config.BackupsS3PeriodicHours.Set(previousHours, false)
		_ = config.BackupsS3PeriodicKeep.Set(previousKeep, false)
	})
}

func intPtr(v int) *int { return &v }

func TestOffsiteSettings_DefaultsAndOverride(t *testing.T) {
	p := newCrashTestServer(t)
	usePeriodicGlobals(t, 24, 4)

	s, err := p.OffsiteSettings()
	require.NoError(t, err)
	assert.Equal(t, OffsiteSettings{}, s, "a server without its own setting follows the panel's")
	hours, keep := p.EffectivePeriodic()
	assert.Equal(t, 24, hours)
	assert.Equal(t, 4, keep)

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicHours: intPtr(0)}))
	hours, keep = p.EffectivePeriodic()
	assert.Equal(t, 0, hours, "0 turns the periodic copy off for this server only")
	assert.Equal(t, 4, keep, "what it leaves unset still follows the panel")
	info, err := os.Stat(filepath.Join(config.ServersFolder.Value(), p.ID()+".offsite"))
	if assert.NoError(t, err) {
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicHours: intPtr(168), PeriodicKeep: intPtr(10)}))
	s, err = p.OffsiteSettings()
	require.NoError(t, err)
	assert.Equal(t, OffsiteSettings{PeriodicHours: intPtr(168), PeriodicKeep: intPtr(10)}, s)
	hours, keep = p.EffectivePeriodic()
	assert.Equal(t, 168, hours)
	assert.Equal(t, 10, keep)

	assert.ErrorIs(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicHours: intPtr(25)}), skypanel.ErrOffsitePeriodicHours)
	assert.ErrorIs(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicKeep: intPtr(0)}), skypanel.ErrOffsiteKeep)
	assert.ErrorIs(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicKeep: intPtr(51)}), skypanel.ErrOffsiteKeep)
	s, _ = p.OffsiteSettings()
	assert.Equal(t, OffsiteSettings{PeriodicHours: intPtr(168), PeriodicKeep: intPtr(10)}, s, "a refused value saves nothing")

	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{}))
	hours, keep = p.EffectivePeriodic()
	assert.Equal(t, 24, hours, "null goes back to the panel's value")
	assert.Equal(t, 4, keep)
}

func TestOffsiteSettings_AGlobalValueEditedByHandIsNotUsed(t *testing.T) {
	p := newCrashTestServer(t)
	usePeriodicGlobals(t, 25, 0)

	hours, keep := p.EffectivePeriodic()
	assert.Equal(t, 0, hours, "an interval the panel does not offer leaves the periodic copy off")
	assert.Equal(t, 4, keep)
}

func TestOffsiteStatus_ReportsWhatTheServerUses(t *testing.T) {
	fake := useFakeBucket(t)
	p := newCrashTestServer(t)
	usePeriodicGlobals(t, 48, 6)
	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicKeep: intPtr(2)}))

	status, err := p.OffsiteStatus()
	require.NoError(t, err)
	assert.Nil(t, status.PeriodicHours)
	assert.Equal(t, intPtr(2), status.PeriodicKeep)
	assert.Equal(t, 48, status.EffectiveHours)
	assert.Equal(t, 2, status.EffectiveKeep)
	assert.True(t, status.Configured)
	assert.Nil(t, status.LastPeriodic, "no periodic copy made yet")

	last := time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC)
	periodic.Lock()
	periodic.states[p.ID()] = &periodicState{last: last}
	periodic.Unlock()
	t.Cleanup(func() {
		periodic.Lock()
		delete(periodic.states, p.ID())
		periodic.Unlock()
	})
	fake.disabled.Store(true)

	status, err = p.OffsiteStatus()
	require.NoError(t, err)
	assert.False(t, status.Configured)
	if assert.NotNil(t, status.LastPeriodic) {
		assert.Equal(t, "2026-11-05T03:00:00Z", *status.LastPeriodic)
	}
}

func TestLastPeriodic_UnknownServer(t *testing.T) {
	_, ok := LastPeriodic("never-seen")
	assert.False(t, ok)
}

func TestDeletingAServerRemovesItsOffsiteFile(t *testing.T) {
	s := newTestScheduler(t)
	p, _ := newBackupTestServer(t)
	p.Identifier = "tasktest"
	p.Scheduler = s
	folder := config.ServersFolder.Value()
	p.RunningEnvironment.RootDirectory = filepath.Join(folder, p.ID())
	require.NoError(t, os.MkdirAll(p.RunningEnvironment.RootDirectory, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(folder, p.ID()+".json"), []byte("{}"), 0600))
	require.NoError(t, p.SetOffsiteSettings(OffsiteSettings{PeriodicHours: intPtr(12)}))
	require.FileExists(t, filepath.Join(folder, p.ID()+".offsite"))

	previous := GetAll()
	addToCache(p)
	t.Cleanup(func() {
		allServersLock.Lock()
		allServers = previous
		allServersLock.Unlock()
	})

	require.NoError(t, Delete(p.ID()))

	assert.NoFileExists(t, filepath.Join(folder, p.ID()+".offsite"))
}
