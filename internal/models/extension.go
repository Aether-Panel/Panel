package models

import "time"

// Extension is an add-on installed by an administrator: pages the panel shows in a
// sandboxed frame, and a webhook that receives the panel's events. Its files live in
// config.ExtensionsFolder/<id>; Manifest is the extension.json it was installed from.
type Extension struct {
	ID          string `gorm:"column:id;primaryKey;size:40" json:"id"`
	Name        string `gorm:"column:name;not null;size:60" json:"name"`
	Version     string `gorm:"column:version;not null;size:20" json:"version"`
	Author      string `gorm:"column:author;size:60" json:"author"`
	Description string `gorm:"column:description;size:300" json:"description"`
	Enabled     bool   `gorm:"column:enabled;not null;default:false" json:"enabled"`
	Manifest    string `gorm:"column:manifest;not null;type:text" json:"-"`

	// WebhookURL is where events are sent: the manifest's, unless the administrator changed it
	WebhookURL string `gorm:"column:webhook_url;size:500" json:"webhookUrl"`
	// Secret signs every event (X-Aether-Signature), so the extension can check it came from here
	Secret string `gorm:"column:secret;not null;size:64" json:"secret"`

	CreatedAt time.Time `json:"installedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Extension) TableName() string {
	return "extensions"
}
