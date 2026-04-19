package models

import (
	"errors"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/gofrs/uuid/v5"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
)

type Node struct {
	ID          uint   `json:"-"`
	Name        string `gorm:"column:name;not null;size:100;uniqueIndex;unique" json:"-" validate:"required,printascii"`
	PublicHost  string `gorm:"column:public_host;not null;size:100" json:"-" validate:"required,ip|fqdn|hostname"`
	PrivateHost string `gorm:"column:private_host;not null;size:100" json:"-" validate:"required,ip|fqdn|hostname"`
	PublicPort  uint16 `gorm:"column:public_port;not null;default:8080" json:"-" validate:"required,min=1,max=65535,nefield=SFTPPort"`
	PrivatePort uint16 `gorm:"column:private_port;not null;default:8080" json:"-" validate:"required,min=1,max=65535,nefield=SFTPPort"`
	SFTPPort    uint16 `gorm:"column:sftp_port;not null;default:5657" json:"-" validate:"required,min=1,max=65535,nefield=PublicPort,nefield=PrivatePort"`

	Secret string `gorm:"column:secret;not null;size=36" json:"-" validate:"required"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`

	Local bool `gorm:"-" json:"-"`
}

func (n *Node) IsValid() (err error) {
	err = validator.New().Struct(n)
	if err != nil {
		err = SkyPanel.GenerateValidationMessage(err)
	}
	return
}

func (n *Node) BeforeSave(*gorm.DB) (err error) {
	err = n.IsValid()
	if err != nil {
		return err
	}
	if n.IsLocal() {
		return errors.New("cannot save local node")
	}
	return
}

func (n *Node) IsLocal() bool {
	nodeIP := config.NodeIP.Value()

	// If the node IP matches our explicitly configured local NodeIP, it is ALWAYS local to us
	if nodeIP != "" && nodeIP != "0.0.0.0" {
		if n.PublicHost == nodeIP || n.PrivateHost == nodeIP {
			return true
		}
	}

	if n.Local {
		if nodeIP != "" && nodeIP != "0.0.0.0" {
			// If we have a specific NodeIP, but the LocalNode's host (set via MasterUrl)
			// doesn't match our NodeIP, AND MasterUrl isn't generic localhost,
			// it means LocalNode represents the Master Panel, and WE are a secondary node!
			if n.PublicHost != "0.0.0.0" && n.PublicHost != "127.0.0.1" && n.PublicHost != "localhost" && n.PublicHost != nodeIP {
				return false
			}
		}
		return true // Otherwise, fallback to assuming we are the primary local node
	}
	
	// Fallback for older configs
	return n.PublicHost == nodeIP || n.PrivateHost == nodeIP
}

var LocalNode = &Node{
	ID:          0,
	Name:        "LocalNode",
	PublicHost:  "0.0.0.0",
	PrivateHost: "0.0.0.0",
	PublicPort:  8080,
	PrivatePort: 8080,
	SFTPPort:    5657,
	Local:       true,
}

func init() {
	u, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	LocalNode.Secret = strings.Replace(u.String(), "-", "", -1)
}
