package models

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
	"time"
)

type DatabaseHost struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Name         string `gorm:"column:name;not null;size:255" json:"name" validate:"required"`
	Host         string `gorm:"column:host;not null;size:255" json:"host" validate:"required,ip|fqdn|hostname"`
	Port         uint16 `gorm:"column:port;not null;default:3306" json:"port" validate:"required,min=1,max=65535"`
	Username     string `gorm:"column:username;not null;size:100" json:"username" validate:"required"`
	Password     string `gorm:"column:password;not null;size:255" json:"password" validate:"required"`
	MaxDatabases int    `gorm:"column:max_databases;default:0" json:"max_databases"`
	NodeID       *uint  `gorm:"column:node_id;default:null" json:"node_id"`
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (dh *DatabaseHost) IsValid() (err error) {
	err = validator.New().Struct(dh)
	if err != nil {
		err = SkyPanel.GenerateValidationMessage(err)
	}
	return
}

func (dh *DatabaseHost) BeforeSave(*gorm.DB) (err error) {
	err = dh.IsValid()
	return
}

func (dh *DatabaseHost) TableName() string {
	return "database_hosts"
}
