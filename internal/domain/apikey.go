package domain

import "time"

type APIKey struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	HashedKey   string    `gorm:"size:255;not null" json:"-"`
	Prefix      string    `gorm:"size:8;not null" json:"prefix"`      // para identificación (ej: "ak_a1b2c3")
	Permissions []string  `gorm:"serializer:json" json:"permissions"` // e.g. ["provision", "terminate"]
	CreatedAt   time.Time `json:"created_at"`
}
