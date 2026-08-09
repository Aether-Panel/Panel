package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	uuid "github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

// SessionLength is the lifetime of a session. With sliding expiration (see
// Validate) the clock is renewed on activity, so a session only expires after
// this much time has passed without any authenticated request.
const SessionLength = time.Hour

type Session struct {
	DB *gorm.DB
}

func (ss *Session) CreateForUser(user *models.User) (string, error) {
	token, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	sessionToken := token.String()

	res, err := HashToken(sessionToken)
	if err != nil {
		return "", err
	}

	session := &models.Session{
		Token:          res,
		ExpirationTime: time.Now().Add(SessionLength),
		UserID:         &user.ID,
	}

	err = ss.DB.Create(session).Error
	return sessionToken, err
}

func (ss *Session) CreateForClient(client *models.Client) (string, error) {
	token, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	sessionToken := token.String()

	res, err := HashToken(sessionToken)
	if err != nil {
		return "", err
	}

	session := &models.Session{
		Token:          res,
		ExpirationTime: time.Now().Add(SessionLength),
		ClientID:       &client.ID,
		UserID:         &client.UserID,
	}

	err = ss.DB.Create(session).Error
	return sessionToken, err
}

func (ss *Session) Validate(token string) (*models.Session, error) {
	hashed, err := HashToken(token)
	if err != nil {
		return nil, err
	}

	session := &models.Session{Token: hashed}
	query := ss.DB.Preload("Client").Preload("User.Permissions").Preload("User.Role").Preload("Server")
	query = query.Where("expiration_time > ?", time.Now())
	query = query.Where("user_id IS NOT NULL OR client_id IS NOT NULL")
	query = query.Where(session)

	err = query.First(session).Error
	if err != nil {
		return nil, err
	}

	// Sliding expiration: renew the session while the user is active so it only
	// expires after a stretch of true inactivity. To avoid a write on every
	// request we only refresh once more than half the lifetime has elapsed.
	if time.Until(session.ExpirationTime) < SessionLength/2 {
		newExpiration := time.Now().Add(SessionLength)
		if upErr := ss.DB.Model(&models.Session{}).
			Where("token = ?", session.Token).
			Update("expiration_time", newExpiration).Error; upErr != nil {
			return nil, upErr
		}
		session.ExpirationTime = newExpiration
	}

	return session, nil
}

func (ss *Session) ValidateNode(token string) (*models.Node, error) {
	if models.LocalNode != nil && models.LocalNode.Secret == token {
		return models.LocalNode, nil
	}

	node := &models.Node{Secret: token}
	err := ss.DB.Where(node).First(node).Error
	return node, err
}

func (ss *Session) Expire(token string) error {
	hashed, err := HashToken(token)
	if err != nil {
		return err
	}

	session := &models.Session{Token: hashed}
	err = ss.DB.Where(session).Delete(session).Error
	if err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

func HashToken(source string) (result string, err error) {
	h := sha256.New()
	_, err = h.Write([]byte(source))
	if err != nil {
		return
	}
	bs := h.Sum(nil)
	builder := &strings.Builder{}
	_, err = hex.NewEncoder(builder).Write(bs)
	result = builder.String()
	return
}
