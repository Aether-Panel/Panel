package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSessionTestDB(t *testing.T) *gorm.DB {
	dbPath := filepath.Join(t.TempDir(), "session_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = db.AutoMigrate(&domain.User{}, &domain.Role{}, &domain.Permissions{}, &domain.Client{}, &domain.Server{}, &domain.Session{}, &domain.Node{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return db
}

func createTestUser(t *testing.T, db *gorm.DB) *domain.User {
	user := &domain.User{
		Username:       "session-test-user",
		Email:          "session-test@example.com",
		HashedPassword: "dummy_hashed_password",
	}
	if !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}
	return user
}

func TestHashToken(t *testing.T) {
	token := "some-session-token-value"

	first, err := HashToken(token)
	assert.NoError(t, err)
	second, err := HashToken(token)
	assert.NoError(t, err)

	// Deterministic: same input always yields the same hash
	assert.Equal(t, first, second)

	// A different token must produce a different hash
	other, err := HashToken("different-token")
	assert.NoError(t, err)
	assert.NotEqual(t, first, other)

	// Length of a sha256 hex digest is 64 characters
	assert.Len(t, first, 64)
}

func TestSession_CreateAndValidateForUser(t *testing.T) {
	db := setupSessionTestDB(t)
	user := createTestUser(t, db)

	ss := &Session{DB: db}

	token, err := ss.CreateForUser(user)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate must succeed with the plaintext token
	session, err := ss.Validate(token)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	if user.ID != 0 {
		assert.Equal(t, user.ID, *session.UserID)
	}
	assert.WithinDuration(t, time.Now().Add(SessionLength), session.ExpirationTime, time.Minute)
}

func TestSession_CreateAndValidateForClient(t *testing.T) {
	db := setupSessionTestDB(t)
	user := createTestUser(t, db)

	client := &domain.Client{
		ClientID:           "test-client-id",
		HashedClientSecret: "dummy_hashed_secret",
		UserID:             user.ID,
		Name:               "test-client",
	}
	if !assert.NoError(t, db.Create(client).Error) {
		t.FailNow()
	}

	ss := &Session{DB: db}

	token, err := ss.CreateForClient(client)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	session, err := ss.Validate(token)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	if client.ID != 0 {
		assert.Equal(t, client.ID, *session.ClientID)
	}
}

func TestSession_ValidateExpiredReturnsError(t *testing.T) {
	db := setupSessionTestDB(t)
	user := createTestUser(t, db)

	ss := &Session{DB: db}

	token, err := ss.CreateForUser(user)
	assert.NoError(t, err)

	// Force the session to already be expired
	db.Model(&domain.Session{}).Where("token = ?", func() string {
		h, _ := HashToken(token)
		return h
	}()).Update("expiration_time", time.Now().Add(-time.Hour))

	_, err = ss.Validate(token)
	assert.Error(t, err)
}

func TestSession_SlidingExpirationRenewsWhenHalfElapsed(t *testing.T) {
	db := setupSessionTestDB(t)
	user := createTestUser(t, db)

	ss := &Session{DB: db}

	token, err := ss.CreateForUser(user)
	assert.NoError(t, err)
	hashed, err := HashToken(token)
	assert.NoError(t, err)

	// Simulate that more than half the session lifetime has elapsed: expiration
	// is < SessionLength away from now (but still in the future so Validate
	// matches). Sliding expiration must renew the clock forward by SessionLength.
	past := time.Now().Add(10 * time.Minute)
	err = db.Model(&domain.Session{}).Where("token = ?", hashed).Update("expiration_time", past).Error
	assert.NoError(t, err)

	session, err := ss.Validate(token)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.True(t, session.ExpirationTime.After(time.Now().Add(SessionLength/2)))
}

func TestSession_ValidateDoesNotRenewEarly(t *testing.T) {
	db := setupSessionTestDB(t)
	user := createTestUser(t, db)

	ss := &Session{DB: db}

	token, err := ss.CreateForUser(user)
	assert.NoError(t, err)

	// Fresh session: less than half lifetime elapsed, so expiration is NOT rewritten.
	fresh, err := ss.Validate(token)
	assert.NoError(t, err)

	var stored domain.Session
	err = db.First(&stored, "token = ?", func() string {
		h, _ := HashToken(token)
		return h
	}()).Error
	assert.NoError(t, err)

	assert.Equal(t, stored.ExpirationTime, fresh.ExpirationTime)
}

func TestSession_ExpireRemovesSession(t *testing.T) {
	db := setupSessionTestDB(t)
	user := createTestUser(t, db)

	ss := &Session{DB: db}

	token, err := ss.CreateForUser(user)
	assert.NoError(t, err)

	err = ss.Expire(token)
	assert.NoError(t, err)

	_, err = ss.Validate(token)
	assert.Error(t, err)
}

func TestSession_ValidateNode(t *testing.T) {
	db := setupSessionTestDB(t)
	ss := &Session{DB: db}

	// LocalNode path: secret matches the in-memory local node
	local, err := ss.ValidateNode(domain.LocalNode.Secret)
	assert.NoError(t, err)
	assert.NotNil(t, local)

	// A random secret should not match unless present in the DB
	_, err = ss.ValidateNode("definitely-not-a-valid-secret")
	assert.Error(t, err)
}
