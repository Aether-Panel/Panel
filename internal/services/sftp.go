package services

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"strings"
)

type DatabaseSFTPAuthorization struct {
}

// errSFTPRefused is the one answer every failed sign-in gets, so it reveals nothing
var errSFTPRefused = errors.New("incorrect username or password")

// splitSFTPUsername reads "email#serverID", the username SFTP clients sign in with
func splitSFTPUsername(username string) (email, serverID string, ok bool) {
	parts := strings.Split(username, "#")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (s *DatabaseSFTPAuthorization) Validate(username, password string) (perms *ssh.Permissions, err error) {
	email, serverID, ok := splitSFTPUsername(username)
	if !ok {
		return nil, errSFTPRefused
	}

	db, err := database.GetConnection()
	if err != nil {
		return nil, skypanel.ErrDatabaseNotAvailable
	}

	// same budget as the panel login, so SFTP cannot be used to guess the password instead
	limitKey := "password:" + email
	if !LoginAttempts.Take(limitKey) {
		return nil, skypanel.ErrTooManyAttempts
	}

	us := &User{DB: db}
	user, err := us.GetByEmail(email)
	if user == nil || err != nil || !us.IsValidCredentials(user, password) {
		return nil, errSFTPRefused
	}
	LoginAttempts.Reset(limitKey)

	return sftpPermissions(db, user.ID, serverID)
}

// ValidateKey signs in with a public key the user added to their account, in place of
// the password. The username is the same "email#serverID".
func (s *DatabaseSFTPAuthorization) ValidateKey(username string, key ssh.PublicKey) (perms *ssh.Permissions, err error) {
	email, serverID, ok := splitSFTPUsername(username)
	if !ok {
		return nil, errSFTPRefused
	}

	db, err := database.GetConnection()
	if err != nil {
		return nil, skypanel.ErrDatabaseNotAvailable
	}

	us := &User{DB: db}
	user, err := us.GetByEmail(email)
	if user == nil || err != nil {
		return nil, errSFTPRefused
	}
	ks := &SSHKey{DB: db}
	stored, err := ks.FindForLogin(user.ID, key)
	if stored == nil || err != nil {
		return nil, errSFTPRefused
	}

	return sftpPermissions(db, user.ID, serverID)
}

// sftpPermissions lets a signed-in user into the server's files when they hold its SFTP scope
func sftpPermissions(db *gorm.DB, userID uint, serverID string) (*ssh.Permissions, error) {
	ps := &Permission{DB: db}
	allowed, err := ps.HasPermission(userID, serverID, scopes.ScopeServerSftp)
	if err != nil || !allowed {
		return nil, errSFTPRefused
	}

	perms := &ssh.Permissions{}
	perms.Extensions = make(map[string]string)
	perms.Extensions["server_id"] = serverID
	return perms, nil
}
