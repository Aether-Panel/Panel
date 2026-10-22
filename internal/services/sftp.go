package services

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"golang.org/x/crypto/ssh"
	"strings"
)

type DatabaseSFTPAuthorization struct {
}

func (s *DatabaseSFTPAuthorization) Validate(username, password string) (perms *ssh.Permissions, err error) {
	parts := strings.Split(username, "#")
	if len(parts) != 2 {
		return nil, errors.New("incorrect username or password")
	}

	email := parts[0]
	serverID := parts[1]

	db, err := database.GetConnection()
	if err != nil {
		return nil, skypanel.ErrDatabaseNotAvailable
	}

	// same budget as the panel login, so SFTP cannot be used to guess the password instead
	limitKey := "password:" + email
	if !LoginAttempts.Allowed(limitKey) {
		return nil, skypanel.ErrTooManyAttempts
	}

	us := &User{DB: db}
	user, err := us.GetByEmail(email)
	if user == nil || err != nil || !us.IsValidCredentials(user, password) {
		LoginAttempts.Record(limitKey)
		return nil, errors.New("incorrect username or password")
	}
	LoginAttempts.Reset(limitKey)

	ss := &Permission{DB: db}
	allowed, err := ss.HasPermission(user.ID, serverID, scopes.ScopeServerSftp)
	if err != nil || !allowed {
		return nil, errors.New("incorrect username or password")
	}

	perms = &ssh.Permissions{}
	perms.Extensions = make(map[string]string)
	perms.Extensions["server_id"] = serverID
	return perms, nil
}
