package models

import (
	"time"
)

type TransferStatus string

const (
	StatusCreated   TransferStatus = "CREATED"
	StatusValidated TransferStatus = "VALIDATED"
	StatusMigrating TransferStatus = "MIGRATING"
	StatusConsumed  TransferStatus = "CONSUMED"
	StatusCompleted TransferStatus = "COMPLETED"
	StatusFailed    TransferStatus = "FAILED"
	StatusCancelled TransferStatus = "CANCELLED"
)

// ExTransferSession represents a federated transfer session between two panels.
type ExTransferSession struct {
	ID              uint           `gorm:"primaryKey" json:"-"`
	SessionUUID     string         `gorm:"type:char(36);uniqueIndex;not null" json:"session_id"`
	ServerID        string         `gorm:"size:36;index;not null" json:"server_id"`
	UserID          uint           `gorm:"index;not null" json:"user_id"`
	Server          Server         `gorm:"foreignKey:ServerID;references:Identifier;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	User            User           `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	TokenHash       string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`
	Status          TransferStatus `gorm:"type:varchar(20);not null;default:'CREATED'" json:"status"`
	DestHost        string         `gorm:"type:varchar(255)" json:"dest_host,omitempty"`
	DestPublicKey   string         `gorm:"type:text" json:"dest_public_key,omitempty"`
	CurrentNonce    string         `gorm:"type:varchar(64)" json:"-"`
	NonceExpiresAt  *time.Time     `json:"-"`
	ProtocolVersion string         `gorm:"type:varchar(10);default:'1.0'" json:"protocol_version"`
	Payload         string         `gorm:"type:text" json:"-"` 
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	ExpiresAt       time.Time      `gorm:"index;not null" json:"expires_at"`
}

type ExTransferLog struct {
	ID        uint   `gorm:"primaryKey"`
	SessionID string            `gorm:"type:char(36);index;not null"`
	Session   ExTransferSession `gorm:"foreignKey:SessionID;references:SessionUUID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Action    string            `gorm:"type:varchar(50);not null"`
	IPAddress string            `gorm:"type:varchar(45);not null"`
	IsError   bool              `gorm:"not null;default:false"`
	Details   string `gorm:"type:text"`
	CreatedAt time.Time
}
