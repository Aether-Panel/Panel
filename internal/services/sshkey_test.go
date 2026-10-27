package services

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func setupSSHKeyTestDB(t *testing.T) (*gorm.DB, *models.User) {
	db := setupOtpTestDB(t)
	if !assert.NoError(t, db.AutoMigrate(&models.SSHKey{})) {
		t.FailNow()
	}
	return db, createOtpTestUser(t, db, "", false)
}

// newSSHKeyLine makes an authorized_keys line for a fresh key of the given kind
func newSSHKeyLine(t *testing.T, kind string, comment string) (string, ssh.PublicKey) {
	var raw interface{}
	switch kind {
	case "ed25519":
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		assert.NoError(t, err)
		raw = pub
	case "ecdsa":
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		assert.NoError(t, err)
		raw = &priv.PublicKey
	case "rsa1024":
		priv, err := rsa.GenerateKey(rand.Reader, 1024)
		assert.NoError(t, err)
		raw = &priv.PublicKey
	}
	key, err := ssh.NewPublicKey(raw)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	if comment != "" {
		line += " " + comment
	}
	return line, key
}

func TestSSHKeyAdd_StoresTheKeyAndNamesItFromTheComment(t *testing.T) {
	db, user := setupSSHKeyTestDB(t)
	ks := &SSHKey{DB: db}
	line, key := newSSHKeyLine(t, "ed25519", "ruben@laptop")

	stored, err := ks.Add(user.ID, "", line)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "ruben@laptop", stored.Name)
	assert.Equal(t, ssh.FingerprintSHA256(key), stored.Fingerprint)
	assert.Equal(t, ssh.KeyAlgoED25519, stored.Type)

	keys, err := ks.GetForUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, keys, 1)
}

func TestSSHKeyAdd_RefusesBadInput(t *testing.T) {
	db, user := setupSSHKeyTestDB(t)
	ks := &SSHKey{DB: db}
	weak, _ := newSSHKeyLine(t, "rsa1024", "")
	good, _ := newSSHKeyLine(t, "ecdsa", "")

	cases := map[string]struct {
		name, line string
		want       error
	}{
		"garbage":       {"laptop", "not a key", skypanel.ErrSSHKeyInvalid},
		"private key":   {"laptop", "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----", skypanel.ErrSSHKeyIsPrivate},
		"short rsa":     {"laptop", weak, skypanel.ErrSSHKeyWeak},
		"with options":  {"laptop", `command="ls" ` + good, skypanel.ErrSSHKeyInvalid},
		"no name":       {"", good, skypanel.ErrSSHKeyName},
		"name too long": {strings.Repeat("a", 101), good, skypanel.ErrSSHKeyName},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ks.Add(user.ID, tc.name, tc.line)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestSSHKeyAdd_RefusesTheSameKeyTwice(t *testing.T) {
	db, user := setupSSHKeyTestDB(t)
	ks := &SSHKey{DB: db}
	line, _ := newSSHKeyLine(t, "ed25519", "")

	_, err := ks.Add(user.ID, "laptop", line)
	assert.NoError(t, err)
	_, err = ks.Add(user.ID, "again", line)
	assert.ErrorIs(t, err, skypanel.ErrSSHKeyExists)
}

func TestSSHKeyFindForLogin_OnlyTheUsersOwnKey(t *testing.T) {
	db, user := setupSSHKeyTestDB(t)
	ks := &SSHKey{DB: db}
	line, key := newSSHKeyLine(t, "ed25519", "")
	_, other := newSSHKeyLine(t, "ed25519", "")
	_, err := ks.Add(user.ID, "laptop", line)
	assert.NoError(t, err)

	found, err := ks.FindForLogin(user.ID, key)
	assert.NoError(t, err)
	assert.NotNil(t, found)

	found, err = ks.FindForLogin(user.ID, other)
	assert.NoError(t, err)
	assert.Nil(t, found, "a key the user never added does not sign in")

	found, err = ks.FindForLogin(user.ID+1, key)
	assert.NoError(t, err)
	assert.Nil(t, found, "another user's key does not sign in")
}

func TestSSHKeyDelete_OnlyTheOwnerCanDelete(t *testing.T) {
	db, user := setupSSHKeyTestDB(t)
	ks := &SSHKey{DB: db}
	line, _ := newSSHKeyLine(t, "ed25519", "")
	stored, err := ks.Add(user.ID, "laptop", line)
	if !assert.NoError(t, err) {
		return
	}

	assert.ErrorIs(t, ks.Delete(user.ID+1, stored.ID), gorm.ErrRecordNotFound)
	assert.NoError(t, ks.Delete(user.ID, stored.ID))
	keys, _ := ks.GetForUser(user.ID)
	assert.Empty(t, keys)
}
