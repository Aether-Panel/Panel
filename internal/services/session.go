package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	uuid "github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

// SessionLength is the lifetime of a session. With sliding expiration (see
// Validate) the clock is renewed on activity, so a session only expires after
// this much time has passed without any authenticated request.
const SessionLength = time.Hour

// sessionPruneEvery is how often creating a session also deletes the expired ones. Validate
// already ignores them, but nothing else removes the rows, so every login left one behind.
const sessionPruneEvery = time.Hour

// lastSessionPrune is the unix time of the last clean-up, shared by every request so the
// pass runs at most once per sessionPruneEvery
var lastSessionPrune atomic.Int64

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

	err = ss.create(session)
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

	err = ss.create(session)
	return sessionToken, err
}

// create stores the session and, at most once per sessionPruneEvery, deletes the expired
// ones. Sessions are only made here, so this keeps the table from growing with every login.
func (ss *Session) create(session *models.Session) error {
	if err := ss.DB.Create(session).Error; err != nil {
		return err
	}

	now := time.Now()
	last := lastSessionPrune.Load()
	if now.Sub(time.Unix(last, 0)) >= sessionPruneEvery && lastSessionPrune.CompareAndSwap(last, now.Unix()) {
		// a failed clean-up must not fail the login: the rows just wait for the next pass
		if err := ss.PruneExpired(now); err != nil {
			logging.Error.Printf("Error deleting expired sessions: %s", err)
		}
	}
	return nil
}

// PruneExpired deletes the sessions that expired by now; Validate would refuse them anyway
func (ss *Session) PruneExpired(now time.Time) error {
	return ss.DB.Where("expiration_time <= ?", now).Delete(&models.Session{}).Error
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

// ExpireAllForUser ends every session of the user, except the one keepToken belongs to
// (pass "" to end them all). Used when the password changes so old logins stop working.
func (ss *Session) ExpireAllForUser(userID uint, keepToken string) error {
	query := ss.DB.Where("user_id = ?", userID)
	if keepToken != "" {
		hashed, err := HashToken(keepToken)
		if err != nil {
			return err
		}
		query = query.Where("token <> ?", hashed)
	}
	return query.Delete(&models.Session{}).Error
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
