package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ErrConfirmationInvalid is returned by Resume when the token is unknown, expired,
// already used, or bound to another user, server or set of calls.
var ErrConfirmationInvalid = errors.New("this confirmation has expired or is not valid, ask again")

// ConfirmationTTL is how long a pending confirmation waits for the user.
const ConfirmationTTL = 10 * time.Minute

// MaxPendingConfirmations caps the entries kept in memory across all users.
const MaxPendingConfirmations = 1000

// pendingCall is a call waiting for the user, with the hash of what was shown.
type pendingCall struct {
	index int // position of the call in its round
	call  FunctionCall
	step  Step
	hash  string
}

// confirmationEntry is everything needed to continue the conversation once the
// user has decided.
type confirmationEntry struct {
	token     string
	userID    uint
	serverID  string
	expiresAt time.Time

	system    string
	history   []Message           // up to and including the model message with the calls
	responses []*FunctionResponse // the round's answers in call order; nil where pending
	pending   []pendingCall
	rounds    int
	calls     int
}

type ownerKey struct {
	userID   uint
	serverID string
}

// ConfirmationStore keeps the pending confirmations in memory. Entries are single
// use, expire after ConfirmationTTL, and each user has at most one per server: a
// new one replaces the old. A panel restart drops them all.
type ConfirmationStore struct {
	mu      sync.Mutex
	entries map[string]*confirmationEntry
	owners  map[ownerKey]string
	now     func() time.Time
}

// NewConfirmationStore returns an empty store.
func NewConfirmationStore() *ConfirmationStore {
	return &ConfirmationStore{
		entries: map[string]*confirmationEntry{},
		owners:  map[ownerKey]string{},
		now:     time.Now,
	}
}

// put stores e under a new random token, replacing the owner's previous entry.
func (s *ConfirmationStore) put(e *confirmationEntry) (string, time.Time, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.sweepLocked(now)
	key := ownerKey{e.userID, e.serverID}
	if old, ok := s.owners[key]; ok {
		delete(s.entries, old)
	}
	if len(s.entries) >= MaxPendingConfirmations {
		s.evictOldestLocked()
	}
	e.token = token
	e.expiresAt = now.Add(ConfirmationTTL)
	s.entries[token] = e
	s.owners[key] = token
	return token, e.expiresAt, nil
}

// take removes and returns the entry for token when it is still valid and bound to
// this user and server. A mismatching user or server leaves the entry in place, so
// someone else holding the token cannot burn the owner's confirmation.
func (s *ConfirmationStore) take(token string, userID uint, serverID string) (*confirmationEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[token]
	if !ok {
		return nil, ErrConfirmationInvalid
	}
	if !s.now().Before(e.expiresAt) {
		s.removeLocked(e)
		return nil, ErrConfirmationInvalid
	}
	if e.userID != userID || e.serverID != serverID {
		return nil, ErrConfirmationInvalid
	}
	s.removeLocked(e)
	return e, nil
}

// Len reports how many entries are stored (expired ones included until swept).
func (s *ConfirmationStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *ConfirmationStore) removeLocked(e *confirmationEntry) {
	delete(s.entries, e.token)
	key := ownerKey{e.userID, e.serverID}
	if s.owners[key] == e.token {
		delete(s.owners, key)
	}
}

func (s *ConfirmationStore) sweepLocked(now time.Time) {
	for _, e := range s.entries {
		if !now.Before(e.expiresAt) {
			s.removeLocked(e)
		}
	}
}

func (s *ConfirmationStore) evictOldestLocked() {
	var oldest *confirmationEntry
	for _, e := range s.entries {
		if oldest == nil || e.expiresAt.Before(oldest.expiresAt) {
			oldest = e
		}
	}
	if oldest != nil {
		s.removeLocked(oldest)
	}
}

// callHash binds a pending call to exactly what was described to the user: the
// tool name and its arguments as canonical JSON (encoding/json sorts map keys).
func callHash(call FunctionCall) (string, error) {
	data, err := json.Marshal(struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	}{call.Name, call.Args})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
