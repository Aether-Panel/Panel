package models

import "time"

// SSHKey is a public key a user signs in to SFTP with instead of their password.
type SSHKey struct {
	ID     uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID uint `gorm:"column:user_id;not null;uniqueIndex:idx_ssh_keys_user_fingerprint" json:"-"`
	User   User `gorm:"ASSOCIATION_SAVE_REFERENCE:false" json:"-" validate:"-"`

	Name string `gorm:"column:name;not null;size:100" json:"name"`
	// Fingerprint is the SHA256 fingerprint, the same text `ssh-keygen -l` prints
	Fingerprint string `gorm:"column:fingerprint;not null;size:100;uniqueIndex:idx_ssh_keys_user_fingerprint" json:"fingerprint"`
	// PublicKey is the key in authorized_keys format, without its comment
	PublicKey string `gorm:"column:public_key;not null;type:text" json:"-"`
	Type      string `gorm:"column:type;not null;size:50" json:"type"`

	CreatedAt time.Time `json:"createdAt"`
}

func (SSHKey) TableName() string {
	return "ssh_keys"
}
