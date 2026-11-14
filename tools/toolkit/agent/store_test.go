package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmationStore_TakeIsSingleUseAndBound(t *testing.T) {
	s := NewConfirmationStore()
	token, expiresAt, err := s.put(&confirmationEntry{userID: 1, serverID: "a"})
	require.NoError(t, err)
	assert.Len(t, token, 64)
	assert.WithinDuration(t, time.Now().Add(ConfirmationTTL), expiresAt, time.Minute)

	_, err = s.take(token, 2, "a")
	assert.ErrorIs(t, err, ErrConfirmationInvalid)
	_, err = s.take(token, 1, "b")
	assert.ErrorIs(t, err, ErrConfirmationInvalid)
	e, err := s.take(token, 1, "a")
	require.NoError(t, err)
	assert.Equal(t, "a", e.serverID)
	_, err = s.take(token, 1, "a")
	assert.ErrorIs(t, err, ErrConfirmationInvalid)
	assert.Zero(t, s.Len())
}

func TestConfirmationStore_SweepsExpiredEntries(t *testing.T) {
	now := time.Now()
	s := NewConfirmationStore()
	s.now = func() time.Time { return now }
	_, _, err := s.put(&confirmationEntry{userID: 1, serverID: "a"})
	require.NoError(t, err)
	now = now.Add(ConfirmationTTL)
	_, _, err = s.put(&confirmationEntry{userID: 2, serverID: "a"})
	require.NoError(t, err)
	assert.Equal(t, 1, s.Len())
}

func TestConfirmationStore_CapsTheEntries(t *testing.T) {
	now := time.Now()
	s := NewConfirmationStore()
	s.now = func() time.Time { return now }
	first, _, err := s.put(&confirmationEntry{userID: 0, serverID: "a"})
	require.NoError(t, err)
	for i := 1; i < MaxPendingConfirmations+5; i++ {
		now = now.Add(time.Millisecond)
		_, _, err := s.put(&confirmationEntry{userID: uint(i), serverID: "a"})
		require.NoError(t, err)
	}
	assert.Equal(t, MaxPendingConfirmations, s.Len())
	_, err = s.take(first, 0, "a")
	assert.ErrorIs(t, err, ErrConfirmationInvalid, "the oldest entry was evicted")
}

func TestCallHash_IgnoresKeyOrderButNotValues(t *testing.T) {
	a, err := callHash(FunctionCall{Name: "edit_file", Args: map[string]any{"path": "a", "old": "x", "new": "y"}})
	require.NoError(t, err)
	b, err := callHash(FunctionCall{ID: "other", Name: "edit_file", Args: map[string]any{"new": "y", "old": "x", "path": "a"}})
	require.NoError(t, err)
	c, err := callHash(FunctionCall{Name: "edit_file", Args: map[string]any{"path": "b", "old": "x", "new": "y"}})
	require.NoError(t, err)
	d, err := callHash(FunctionCall{Name: "write_file", Args: map[string]any{"path": "a", "old": "x", "new": "y"}})
	require.NoError(t, err)
	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
	assert.NotEqual(t, a, d)
}
