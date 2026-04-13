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
	if n.Local {
		return true
	}
	// If the node IP matches our configured local IP, it's local to US
	// even if it's stored as a remote node for other panels.
	return n.PublicHost == config.NodeIP.Value() || n.PrivateHost == config.NodeIP.Value()
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
