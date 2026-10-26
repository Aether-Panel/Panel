package skypanel

import (
	"golang.org/x/crypto/ssh"
)

type SFTPAuthorization interface {
	Validate(username, password string) (perms *ssh.Permissions, err error)
}

// SFTPKeyAuthorization is an SFTPAuthorization that also signs users in with a public key
// stored in their account. The SFTP server offers key sign-in only when its authorization
// implements it.
type SFTPKeyAuthorization interface {
	ValidateKey(username string, key ssh.PublicKey) (perms *ssh.Permissions, err error)
}
