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

	us := &User{DB: db}
	user, err := us.GetByEmail(email)

	// same budget as the panel login, so SFTP cannot be used to guess the password instead
	limitKey := attemptKey("password", email, user)
	if !LoginAttempts.Take(limitKey) {
		return nil, skypanel.ErrTooManyAttempts
	}

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

	return validateSFTPKey(db, email, serverID, key)
}

// validateSFTPKey checks a key login against the database. Failed keys count against the
// account's key budget, "sshkey:user:<id>", the one the panel's /auth/sftp-key check for
// remote nodes uses, so a key login has the same limit on every node. It is apart from the
// password budget: an SSH client offers each of its keys in turn, and those misses must not
// lock the owner out of signing in with their password.
func validateSFTPKey(db *gorm.DB, email, serverID string, key ssh.PublicKey) (*ssh.Permissions, error) {
	us := &User{DB: db}
	user, err := us.GetByEmail(email)
	if user == nil || err != nil {
		return nil, errSFTPRefused
	}

	limitKey := attemptKey("sshkey", email, user)
	if !LoginAttempts.Take(limitKey) {
		return nil, skypanel.ErrTooManyAttempts
	}

	ks := &SSHKey{DB: db}
	stored, err := ks.FindForLogin(user.ID, key)
	if stored == nil || err != nil {
		return nil, errSFTPRefused
	}

	perms, err := sftpPermissions(db, user.ID, serverID)
	if err != nil {
		return nil, err
	}
	LoginAttempts.Reset(limitKey)
	return perms, nil
}

// sftpPermissions lets a user into the server's files when they may still sign in to the
// panel and hold the server's SFTP scope
func sftpPermissions(db *gorm.DB, userID uint, serverID string) (*ssh.Permissions, error) {
	ps := &Permission{DB: db}
	allowed, err := ps.MaySignIn(userID)
	if err == nil && allowed {
		allowed, err = ps.HasPermission(userID, serverID, scopes.ScopeServerSftp)
	}
	if err != nil || !allowed {
		return nil, errSFTPRefused
	}

	perms := &ssh.Permissions{}
	perms.Extensions = make(map[string]string)
	perms.Extensions["server_id"] = serverID
	return perms, nil
}
