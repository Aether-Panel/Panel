package services

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

// Extensions of the ssh.Permissions a sign-in returns, read again by Recheck
const (
	sftpExtServerID = "server_id"
	sftpExtUserID   = "user_id"
	sftpExtEmail    = "email"
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

	return sftpPermissions(db, user, serverID)
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

	perms, err := sftpPermissions(db, user, serverID)
	if err != nil {
		return nil, err
	}
	LoginAttempts.Reset(limitKey)
	return perms, nil
}

// sftpPermissions lets a user into the server's files when they may still sign in to the
// panel and hold the server's SFTP scope. The user is kept in the permissions for Recheck.
func sftpPermissions(db *gorm.DB, user *models.User, serverID string) (*ssh.Permissions, error) {
	allowed, err := sftpAllowed(db, user.ID, serverID)
	if err != nil || !allowed {
		return nil, errSFTPRefused
	}

	perms := &ssh.Permissions{}
	perms.Extensions = map[string]string{
		sftpExtServerID: serverID,
		sftpExtUserID:   strconv.FormatUint(uint64(user.ID), 10),
		sftpExtEmail:    user.Email,
	}
	return perms, nil
}

// sftpAllowed is the access rule of SFTP, at sign-in and while connected: the login scope
// and the server's SFTP scope
func sftpAllowed(db *gorm.DB, userID uint, serverID string) (bool, error) {
	ps := &Permission{DB: db}
	allowed, err := ps.MaySignIn(userID)
	if err == nil && allowed {
		allowed, err = ps.HasPermission(userID, serverID, scopes.ScopeServerSftp)
	}
	return allowed, err
}

// Recheck says whether an open SFTP connection may stay open: the account still exists and
// still passes the sign-in rule for the server. A database error is not an answer.
func (s *DatabaseSFTPAuthorization) Recheck(perms *ssh.Permissions) (bool, error) {
	db, err := database.GetConnection()
	if err != nil {
		return false, skypanel.ErrDatabaseNotAvailable
	}
	return recheckSFTP(db, perms)
}

func recheckSFTP(db *gorm.DB, perms *ssh.Permissions) (bool, error) {
	if perms == nil {
		return false, nil
	}
	serverID := perms.Extensions[sftpExtServerID]
	userID, err := strconv.ParseUint(perms.Extensions[sftpExtUserID], 10, 32)
	if err != nil || serverID == "" {
		return false, nil
	}

	user, err := (&User{DB: db}).GetByID(uint(userID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// SQLite can hand a deleted user's id to a new account, which must not inherit the
	// connection; an email change also ends it, as the login name no longer matches
	if !strings.EqualFold(user.Email, perms.Extensions[sftpExtEmail]) {
		return false, nil
	}
	return sftpAllowed(db, user.ID, serverID)
}
