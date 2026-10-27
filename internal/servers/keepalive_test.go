package servers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// afterExit stops the keep-alive sender; it used to block for ever on an unbuffered channel
// when keep-alive had been turned off since the last run
func TestStopKeepAlive_NeverBlocks(t *testing.T) {
	p, _ := newBackupTestServer(t)

	done := make(chan struct{})
	go func() {
		p.stopKeepAlive() // nothing running
		p.KeepAlive.Frequency, p.KeepAlive.Command = "1h", "say hi"
		p.startKeepAlive()
		p.startKeepAlive() // a second run replaces the first sender
		p.stopKeepAlive()
		p.stopKeepAlive()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stopping keep-alive blocked")
	}
	assert.Nil(t, p.keepAliveStop)
}

func TestInstall_OnlyOneAtATime(t *testing.T) {
	p, _ := newBackupTestServer(t)
	p.installRunning.Store(true)
	assert.Error(t, p.Install(), "a second install is refused while one runs")
}
