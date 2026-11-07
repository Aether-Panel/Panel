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

// SFTPRecheck is an SFTPAuthorization that can say whether an open connection may stay open.
// The handshake is the only other check, so without this a user who is deleted or loses
// access keeps the connection they already have. The SFTP server calls it periodically with
// the permissions the handshake returned and closes the connection when allowed is false.
// An error means the answer is unknown (database or panel unreachable): the connection stays.
type SFTPRecheck interface {
	Recheck(perms *ssh.Permissions) (allowed bool, err error)
}
