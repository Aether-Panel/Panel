package models

import (
	"regexp"
	"strings"

	"github.com/gofrs/uuid/v5"
)

// serverIdentifierPattern restricts server identifiers to a URL- and
// path-safe character set. Identifiers are embedded verbatim into daemon URLs
// such as /daemon/server/{identifier}/start, so allowing arbitrary printable
// characters (as the old `printascii` validation did) was a path/route
// injection risk. The pattern keeps backwards compatibility with the historical
// 8-character hexadecimal identifiers while allowing up to 20 characters.
var serverIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,19}$`)

// IsValidServerIdentifier reports whether id is a safe, well-formed server
// identifier. It should be used at every trust boundary where a caller can
// choose an identifier (e.g. server creation).
func IsValidServerIdentifier(id string) bool {
	return serverIdentifierPattern.MatchString(id)
}

// GenerateServerIdentifier returns a new, path-safe server identifier.
//
// It is derived from a random (UUIDv4) value and rendered as 12 uppercase
// hexadecimal characters (48 bits of entropy). This is longer than the legacy
// 8-character scheme (32 bits) which makes collisions, including the
// birthday-bound collisions possible with many servers, practically impossible.
// Note: it deliberately does NOT use the leading bits of a UUIDv7, since those
// encode a timestamp and would collide for servers created in the same
// millisecond.
func GenerateServerIdentifier() (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	compact := strings.ReplaceAll(id.String(), "-", "")
	return strings.ToUpper(compact[:12]), nil
}

// GenerateInternalID returns a new, time-ordered internal identifier (UUIDv7)
// suitable for use as a stable, index-friendly internal primary identity.
func GenerateInternalID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
