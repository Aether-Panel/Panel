package models

import (
	"fmt"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
)

type Server struct {
	Name       string `gorm:"column:name;not null;size:40" json:"-" validate:"required,printascii"`
	Identifier string `gorm:"column:identifier;primaryKey;size:20" json:"-" validate:"required,printascii"`

	// InternalID is the immutable, time-ordered internal identity (UUIDv7) of the
	// server. Unlike Identifier — the user-facing key that is embedded in daemon
	// URLs and referenced by several tables — InternalID never changes and is
	// meant to become the canonical internal identity going forward.
	//
	// It is generated automatically on create and is also surfaced read-only to
	// clients as `internalId` (see ServerView) so frontend and integrations can
	// adopt it, but it must never be used as the public/legacy key.
	InternalID string `gorm:"column:internal_id;size:36;not null;uniqueIndex" json:"-"`

	RawNodeID *uint `gorm:"column:node_id;index;->;<-:create" json:"-" validate:"-"`
	NodeID    uint  `gorm:"-" json:"-" validate:"-"`
	Node      Node  `gorm:"foreignKey:RawNodeID;->;<-:create" json:"-" validate:"-"`

	IP   string `gorm:"" json:"-" validate:"omitempty,ip|fqdn"`
	Port uint16 `gorm:"" json:"-" validate:"omitempty"`

	// Ports holds the full list of ports assigned to the server (primary first).
	// Port mirrors the first entry and is kept for backwards compatibility.
	Ports []uint16 `gorm:"serializer:json" json:"-" validate:"-"`

	// PortNotes stores user-defined labels for each port (key: port number as string).
	PortNotes map[string]string `gorm:"serializer:json" json:"-" validate:"-"`

	Type string `gorm:"NOT NULL;default='generic'" json:"-" validate:"required,printascii"`
	Icon string `gorm:"" json:"-"`

	// Server Splitter Hierarchical relation
	ParentServerID *string `gorm:"column:parent_server_id;size:20;index" json:"parent_server_id,omitempty"`
	ParentServer   *Server `gorm:"foreignKey:ParentServerID;references:Identifier" json:"-" validate:"-"`

	// Total Max Resources for the server
	TotalCPU    int   `gorm:"column:total_cpu;default:0" json:"total_cpu"`
	TotalMemory int64 `gorm:"column:total_memory;default:0" json:"total_memory"`
	TotalDisk   int64 `gorm:"column:total_disk;default:0" json:"total_disk"`

	// Server State
	Suspended bool `gorm:"column:suspended;default:false" json:"suspended"`

	// External integration ID (e.g. Paymenter service ID)
	ExternalID string `gorm:"column:external_id;size:64;index" json:"-"`

	// Subdomain is the server's own name under the panel's DNS base domain (survival →
	// survival.play.example.com); empty when it has none. Two servers never share one: the
	// panel checks that before creating the records (a unique index would refuse the many
	// servers that have none).
	Subdomain string `gorm:"column:subdomain;size:63;index" json:"-" validate:"-"`

	// DNSRecordIDs are the Cloudflare records created for Subdomain, so they can be removed
	DNSRecordIDs []string `gorm:"column:dns_record_ids;serializer:json" json:"-" validate:"-"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (s *Server) IsValid() (err error) {
	err = validator.New().Struct(s)
	if err != nil {
		err = skypanel.GenerateValidationMessage(err)
	}
	return
}

func (s *Server) BeforeSave(*gorm.DB) (err error) {
	err = s.IsValid()
	if s.NodeID == 0 || s.Node.IsLocal() {
		s.RawNodeID = nil
	} else {
		s.RawNodeID = &s.NodeID
	}
	return
}

// BeforeCreate validates the identifier and guarantees every server gets an
// internal UUIDv7 identity, even when it is inserted outside the
// ServerService.Create path (for example directly through the ORM).
func (s *Server) BeforeCreate(*gorm.DB) (err error) {
	if !IsValidServerIdentifier(s.Identifier) {
		return fmt.Errorf("invalid server identifier %q: only letters, digits, '-' and '_' are allowed (1-20 characters)", s.Identifier)
	}
	if s.InternalID == "" {
		s.InternalID, err = GenerateInternalID()
	}
	return
}

func (s *Server) AfterFind(*gorm.DB) (err error) {
	if s.RawNodeID == nil || *s.RawNodeID == LocalNode.ID {
		s.Node = *LocalNode
		s.NodeID = LocalNode.ID
	} else {
		s.NodeID = *s.RawNodeID
	}
	return
}
