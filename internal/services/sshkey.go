package services

import (
	"errors"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// MaxSSHKeysPerUser keeps one account from storing an unbounded number of keys
const MaxSSHKeysPerUser = 25

// minRSABits refuses RSA keys too short to be safe, as OpenSSH itself does
const minRSABits = 2048

type SSHKey struct {
	DB *gorm.DB
}

// GetForUser lists the user's keys, newest first.
func (s *SSHKey) GetForUser(userID uint) ([]*models.SSHKey, error) {
	keys := make([]*models.SSHKey, 0)
	err := s.DB.Where(userIDQuery, userID).Order("id desc").Find(&keys).Error
	return keys, err
}

// Add stores a public key, given as one authorized_keys line, for the user.
func (s *SSHKey) Add(userID uint, name, line string) (*models.SSHKey, error) {
	key, comment, err := ParseSSHPublicKey(line)
	if err != nil {
		return nil, err
	}
	// without a name the key's own comment (usually user@host) names it
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.TrimSpace(comment)
	}
	if name == "" || len(name) > 100 {
		return nil, skypanel.ErrSSHKeyName
	}

	var count int64
	if err = s.DB.Model(&models.SSHKey{}).Where(userIDQuery, userID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count >= MaxSSHKeysPerUser {
		return nil, skypanel.ErrSSHKeyLimit
	}

	model := &models.SSHKey{
		UserID:      userID,
		Name:        name,
		Fingerprint: ssh.FingerprintSHA256(key),
		PublicKey:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
		Type:        key.Type(),
	}
	var existing int64
	err = s.DB.Model(&models.SSHKey{}).Where("user_id = ? AND fingerprint = ?", userID, model.Fingerprint).Count(&existing).Error
	if err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, skypanel.ErrSSHKeyExists
	}
	return model, s.DB.Create(model).Error
}

// Delete removes one of the user's keys; a key of someone else is reported as not found.
func (s *SSHKey) Delete(userID, id uint) error {
	res := s.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.SSHKey{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// FindForLogin returns the user's stored key matching the one offered, or nil.
func (s *SSHKey) FindForLogin(userID uint, offered ssh.PublicKey) (*models.SSHKey, error) {
	var key models.SSHKey
	err := s.DB.Where("user_id = ? AND fingerprint = ?", userID, ssh.FingerprintSHA256(offered)).First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// the fingerprint matched; compare the key itself so a hash collision can never sign in
	stored, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.PublicKey))
	if err != nil || string(stored.Marshal()) != string(offered.Marshal()) {
		return nil, nil
	}
	return &key, nil
}

// ParseSSHPublicKey reads one authorized_keys line ("ssh-ed25519 AAAA... comment") and
// refuses private keys, certificates and weak keys.
func ParseSSHPublicKey(line string) (ssh.PublicKey, string, error) {
	line = strings.TrimSpace(line)
	if strings.Contains(line, "PRIVATE KEY") {
		return nil, "", skypanel.ErrSSHKeyIsPrivate
	}
	key, comment, options, rest, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil || len(options) > 0 || len(strings.TrimSpace(string(rest))) > 0 {
		return nil, "", skypanel.ErrSSHKeyInvalid
	}
	switch key.Type() {
	case ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
		ssh.KeyAlgoSKED25519, ssh.KeyAlgoSKECDSA256:
	case ssh.KeyAlgoRSA:
		if bits, ok := rsaBits(key); !ok || bits < minRSABits {
			return nil, "", skypanel.ErrSSHKeyWeak
		}
	default:
		// DSA and certificates are not accepted
		return nil, "", skypanel.ErrSSHKeyInvalid
	}
	return key, comment, nil
}

func rsaBits(key ssh.PublicKey) (int, bool) {
	crypto, ok := key.(ssh.CryptoPublicKey)
	if !ok {
		return 0, false
	}
	type sized interface{ Size() int }
	rsa, ok := crypto.CryptoPublicKey().(sized)
	if !ok {
		return 0, false
	}
	return rsa.Size() * 8, true
}
