package models

import "time"

type APIKey struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	HashedKey   string    `gorm:"size:255;not null" json:"-"`
	Prefix      string    `gorm:"size:8;not null" json:"prefix"`      // para identificación (ej: "ak_a1b2c3")
	Permissions []string  `gorm:"serializer:json" json:"permissions"` // e.g. ["provision", "terminate"]
	CreatedAt   time.Time `json:"created_at"`
}

// What a billing API key may do on /api/v1. Every route names one of these; ping needs none.
const (
	APIKeyCreate        = "create"
	APIKeyTerminate     = "terminate"
	APIKeySuspend       = "suspend" // suspend and unsuspend
	APIKeyChangePackage = "change-package"
	APIKeySSO           = "sso"

	// APIKeyAll is what every key made before keys could be limited holds (the panel always
	// created them with it): every action, so existing integrations keep working
	APIKeyAll = "provision"
)

// APIKeyActions lists the actions an administrator can pick for a new key
var APIKeyActions = []string{APIKeyCreate, APIKeyTerminate, APIKeySuspend, APIKeyChangePackage, APIKeySSO}

// Allows says whether the key may perform the action. A key with no permissions at all, or
// with "provision", predates limited keys and may do everything.
func (k *APIKey) Allows(action string) bool {
	if len(k.Permissions) == 0 {
		return true
	}
	for _, p := range k.Permissions {
		if p == APIKeyAll || p == action {
			return true
		}
	}
	return false
}
