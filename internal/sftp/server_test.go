package sftp

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

// fakeRecheck answers each check with the next of its answers, then repeats the last one
type fakeRecheck struct {
	mu      sync.Mutex
	answers []recheckAnswer
	calls   int
	seen    *ssh.Permissions
}

type recheckAnswer struct {
	allowed bool
	err     error
}

func (f *fakeRecheck) Recheck(perms *ssh.Permissions) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen = perms
	a := f.answers[min(f.calls, len(f.answers)-1)]
	f.calls++
	return a.allowed, a.err
}

func (f *fakeRecheck) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// watch runs watchAccess on a fake connection and returns a channel that gets a value when
// the connection is closed, and the function that ends the watch
func watch(checker *fakeRecheck, perms *ssh.Permissions) (<-chan struct{}, func()) {
	closed := make(chan struct{}, 1)
	done := make(chan struct{})
	go watchAccess(done, checker, perms, time.Millisecond, func() error {
		closed <- struct{}{}
		return nil
	}, "alice@example.com#srv1")
	return closed, func() { close(done) }
}

// The connection is checked in isolation here: the watcher gets the connection's Close, so a
// real SSH connection adds nothing to what is tested
func TestWatchAccess_ClosesTheConnectionOnANo(t *testing.T) {
	checker := &fakeRecheck{answers: []recheckAnswer{{allowed: true}, {allowed: true}, {allowed: false}}}
	perms := &ssh.Permissions{Extensions: map[string]string{"server_id": "srv1"}}
	closed, stop := watch(checker, perms)
	defer stop()

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the connection must be closed once access is gone")
	}
	assert.Equal(t, 3, checker.callCount(), "no more checks after closing")
	assert.Same(t, perms, checker.seen, "each check gets what the handshake returned")
}

func TestWatchAccess_KeepsTheConnectionWhileAllowedOrUnknown(t *testing.T) {
	checker := &fakeRecheck{answers: []recheckAnswer{
		{allowed: true},
		{err: errors.New("panel unreachable")},
		{err: errors.New("panel unreachable")},
		{allowed: true},
		{err: errors.New("database is down")},
	}}
	closed, stop := watch(checker, &ssh.Permissions{})

	deadline := time.Now().Add(5 * time.Second)
	for checker.callCount() < 10 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stop()

	assert.GreaterOrEqual(t, checker.callCount(), 10, "the checks go on after an error")
	select {
	case <-closed:
		t.Fatal("an error is no answer: the connection must stay open")
	default:
	}
}

func TestWatchAccess_StopsWithTheConnection(t *testing.T) {
	checker := &fakeRecheck{answers: []recheckAnswer{{allowed: true}}}
	closed, stop := watch(checker, &ssh.Permissions{})
	stop()
	time.Sleep(20 * time.Millisecond)
	calls := checker.callCount()
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, calls, checker.callCount(), "a closed connection is not checked any more")
	assert.Empty(t, closed)
}

type panickingRecheck struct{}

func (panickingRecheck) Recheck(*ssh.Permissions) (bool, error) { panic("broken check") }

// a panic in a check ends the watch, not the daemon
func TestWatchAccess_SurvivesAPanickingCheck(t *testing.T) {
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		watchAccess(make(chan struct{}), panickingRecheck{}, &ssh.Permissions{}, time.Millisecond, func() error { return nil }, "alice@example.com#srv1")
	}()

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch must end after the panic")
	}
}
