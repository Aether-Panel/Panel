package services

import (
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func TestAttemptLimiter_BlocksAfterMaxAndRecoversAfterWindow(t *testing.T) {
	now := time.Now()
	l := NewAttemptLimiter(3, 10*time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		assert.True(t, l.Allowed("User@Example.com"))
		l.Record("user@example.com ")
	}
	assert.False(t, l.Allowed("user@example.com"), "keys ignore case and surrounding spaces")
	assert.True(t, l.Allowed("other@example.com"), "other accounts are not affected")

	now = now.Add(11 * time.Minute)
	assert.True(t, l.Allowed("user@example.com"), "old attempts expire")
}

func TestAttemptLimiter_ResetClearsTheKey(t *testing.T) {
	l := NewAttemptLimiter(1, time.Hour)
	l.Record("a@example.com")
	assert.False(t, l.Allowed("a@example.com"))
	l.Reset("a@example.com")
	assert.True(t, l.Allowed("a@example.com"))
}

func TestValidateLogin_StopsPasswordGuessing(t *testing.T) {
	db := setupSessionTestDB(t)
	user := createTestUser(t, db)
	assert.NoError(t, user.SetPassword("the-right-password"))
	assert.NoError(t, db.Save(user).Error)
	t.Cleanup(func() { LoginAttempts.Reset("password:" + user.Email) })
	us := &User{DB: db}

	for i := 0; i < 10; i++ {
		_, _, err := us.ValidateLogin(user.Email, "wrong")
		assert.ErrorIs(t, err, skypanel.ErrInvalidCredentials)
	}

	_, _, err := us.ValidateLogin(user.Email, "the-right-password")
	assert.ErrorIs(t, err, skypanel.ErrTooManyAttempts, "even the right password waits once the limit is hit")

	LoginAttempts.Reset("password:" + user.Email)
	_, _, err = us.ValidateLogin(user.Email, "the-right-password")
	assert.NoError(t, err)
}
