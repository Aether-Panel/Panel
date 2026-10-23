package servers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/stretchr/testify/assert"
)

func newCrashTestServer(t *testing.T) *Server {
	p, _ := newBackupTestServer(t)
	previous := config.ServersFolder.Value()
	_ = config.ServersFolder.Set(t.TempDir(), false)
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })
	p.Identifier = "crashtest"
	return p
}

func TestAfterExit_ACrashIsRecordedWithTheLastConsoleLines(t *testing.T) {
	p := newCrashTestServer(t)
	p.Execution.ExpectedExitCode = 0
	for i := 0; i < 80; i++ {
		_, _ = p.RunningEnvironment.ConsoleBuffer.Write([]byte("line " + strings.Repeat("x", i%3) + "\n"))
	}
	_, _ = p.RunningEnvironment.ConsoleBuffer.Write([]byte("java.lang.OutOfMemoryError: Java heap space\n"))

	p.afterExit(137)

	reports, err := p.CrashReports()
	if !assert.NoError(t, err) || !assert.Len(t, reports, 1) {
		return
	}
	report := reports[0]
	assert.Equal(t, 137, report.ExitCode)
	assert.False(t, report.Restarted, "autorecover is off")
	assert.LessOrEqual(t, len(report.Lines), crashReportLines)
	assert.Contains(t, strings.Join(report.Lines, "\n"), "OutOfMemoryError")

	info, err := os.Stat(filepath.Join(config.ServersFolder.Value(), "crashtest.crashes"))
	if assert.NoError(t, err) {
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
}

func TestAfterExit_AStopSomeoneAskedForIsNotACrash(t *testing.T) {
	p := newCrashTestServer(t)
	p.stopRequested.Store(true)

	p.afterExit(143)

	reports, _ := p.CrashReports()
	assert.Empty(t, reports)
	assert.False(t, p.stopRequested.Load(), "the flag is used once")
}

func TestAfterExit_AGracefulExitIsNotACrash(t *testing.T) {
	p := newCrashTestServer(t)
	p.CrashCounter = 2

	p.afterExit(0)

	reports, _ := p.CrashReports()
	assert.Empty(t, reports)
	assert.Equal(t, 0, p.CrashCounter)
}

func TestCrashReports_KeepTheNewestTen(t *testing.T) {
	p := newCrashTestServer(t)
	for i := 0; i < maxCrashReports+3; i++ {
		p.recordCrash(i+1, false)
	}
	reports, _ := p.CrashReports()
	if assert.Len(t, reports, maxCrashReports) {
		assert.Equal(t, maxCrashReports+3, reports[0].ExitCode, "newest first")
	}
}

func TestNotifications_DefaultsValidationAndPrivateFile(t *testing.T) {
	p := newCrashTestServer(t)

	n, err := p.Notifications()
	assert.NoError(t, err)
	assert.Equal(t, DefaultNotifications(), n)

	assert.ErrorIs(t, p.SetNotifications(Notifications{DiscordWebhook: "http://169.254.169.254/latest"}), services.ErrInvalidDiscordWebhook)
	assert.ErrorIs(t, p.SetNotifications(Notifications{DiscordWebhook: "https://discord.com.evil.example/api/webhooks/1/x"}), services.ErrInvalidDiscordWebhook)

	hook := "https://discord.com/api/webhooks/123456/abc-DEF_9"
	assert.NoError(t, p.SetNotifications(Notifications{DiscordWebhook: "  " + hook + " ", Crash: true}))
	n, _ = p.Notifications()
	assert.Equal(t, hook, n.DiscordWebhook)
	assert.True(t, n.Crash)
	assert.False(t, n.Backup)

	info, err := os.Stat(filepath.Join(config.ServersFolder.Value(), "crashtest.notify"))
	if assert.NoError(t, err) {
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}

	assert.NoError(t, p.SetNotifications(Notifications{}), "an empty webhook turns notifications off")
}
