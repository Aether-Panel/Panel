package services

import (
	"strings"
	"sync"
	"time"
)

// AttemptLimiter throttles guessing per account. It is keyed by the account (email), not
// by client IP, so it behaves the same behind a reverse proxy, where every client shares
// the proxy's address. State lives in memory and resets when the panel restarts.
type AttemptLimiter struct {
	max    int
	window time.Duration

	lock     sync.Mutex
	attempts map[string][]time.Time
	now      func() time.Time
}

// limiterSweepSize is how many keys trigger a sweep of expired entries, so addresses that
// are tried once and never again do not pile up
const limiterSweepSize = 10000

var (
	// LoginAttempts covers passwords (panel, OAuth2 and SFTP) and 2FA codes
	LoginAttempts = NewAttemptLimiter(10, 15*time.Minute)
	// PasswordResetRequests stops the reset form from being used to flood an inbox
	PasswordResetRequests = NewAttemptLimiter(5, time.Hour)
)

func NewAttemptLimiter(maxAttempts int, window time.Duration) *AttemptLimiter {
	return &AttemptLimiter{
		max:      maxAttempts,
		window:   window,
		attempts: make(map[string][]time.Time),
		now:      time.Now,
	}
}

// Allowed reports whether another attempt may be made for the key
func (l *AttemptLimiter) Allowed(key string) bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	return len(l.recent(normalizeAttemptKey(key))) < l.max
}

// Record counts an attempt (a failed login, or a reset email sent) for the key
func (l *AttemptLimiter) Record(key string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	key = normalizeAttemptKey(key)
	l.attempts[key] = append(l.recent(key), l.now())

	if len(l.attempts) > limiterSweepSize {
		for k := range l.attempts {
			if len(l.recent(k)) == 0 {
				delete(l.attempts, k)
			}
		}
	}
}

// Reset forgets the key, for example after a successful login
func (l *AttemptLimiter) Reset(key string) {
	l.lock.Lock()
	defer l.lock.Unlock()
	delete(l.attempts, normalizeAttemptKey(key))
}

// recent drops attempts older than the window; callers must hold the lock
func (l *AttemptLimiter) recent(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	kept := l.attempts[key][:0]
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.attempts, key)
		return nil
	}
	l.attempts[key] = kept
	return kept
}

func normalizeAttemptKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
