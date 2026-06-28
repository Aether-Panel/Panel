package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&User{}, &Node{}, &Server{}, &ExTransferSession{}, &ExTransferLog{})
	assert.NoError(t, err)

	return db
}

func TestExTransferSession_CreateAndTransition(t *testing.T) {
	db := setupTestDB(t)

	// Setup dummy user and server
	user := User{
		Username:       "testuser",
		Email:          "test@example.com",
		HashedPassword: "dummy_hashed_password",
	}
	err := db.Create(&user).Error
	assert.NoError(t, err)

	node := Node{
		Name:        "localNode",
		PublicHost:  "127.0.0.1",
		PrivateHost: "127.0.0.1",
		PublicPort:  8080,
		PrivatePort: 8080,
		SFTPPort:    5657,
		Secret:      "dummy_secret",
	}
	err = db.Create(&node).Error
	assert.NoError(t, err)

	server := Server{
		Identifier: "testserver-uuid",
		Name:       "testserver",
		NodeID:     node.ID,
		Type:       "minecraft",
	}
	err = db.Create(&server).Error
	assert.NoError(t, err)

	// Create a new transfer session
	sessionUUID := "session-uuid-123"
	expiresAt := time.Now().Add(1 * time.Hour)
	session := ExTransferSession{
		SessionUUID:     sessionUUID,
		ServerID:        server.Identifier,
		UserID:          user.ID,
		TokenHash:       "dummyhash123",
		Status:          StatusCreated,
		ProtocolVersion: "1.0",
		ExpiresAt:       expiresAt,
	}

	err = db.Create(&session).Error
	assert.NoError(t, err)

	// Fetch it
	var fetchedSession ExTransferSession
	err = db.First(&fetchedSession, "session_uuid = ?", sessionUUID).Error
	assert.NoError(t, err)
	assert.Equal(t, StatusCreated, fetchedSession.Status)
	assert.Equal(t, "dummyhash123", fetchedSession.TokenHash)

	// Update status to Validated
	err = db.Model(&fetchedSession).Update("status", StatusValidated).Error
	assert.NoError(t, err)

	var updatedSession ExTransferSession
	err = db.First(&updatedSession, "session_uuid = ?", sessionUUID).Error
	assert.NoError(t, err)
	assert.Equal(t, StatusValidated, updatedSession.Status)
}
