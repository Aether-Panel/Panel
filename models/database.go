package models

import (
	"time"

	"github.com/SkyPanel/SkyPanel/v3"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
)

type Database struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	ServerID         string `gorm:"column:server_id;not null;size:20;index" json:"server_id" validate:"required"`
	DatabaseHostID   uint   `gorm:"column:database_host_id;not null;index" json:"database_host_id" validate:"required"`
	DatabaseName     string `gorm:"column:database_name;not null;size:100" json:"database_name" validate:"required"`
	Username         string `gorm:"column:username;not null;size:100" json:"username" validate:"required"`
	Password         string `gorm:"column:password;not null;size:255" json:"password" validate:"required"`
	RemoteConnection string `gorm:"column:remote_connection;not null;size:255" json:"remote_connection"`
	MaxConnections   int    `gorm:"column:max_connections;default:0" json:"max_connections"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relaciones
	Server       *Server       `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE;" json:"-"`
	DatabaseHost *DatabaseHost `gorm:"foreignKey:DatabaseHostID;constraint:OnDelete:CASCADE;" json:"-"`
}

func (d *Database) IsValid() (err error) {
	err = validator.New().Struct(d)
	if err != nil {
		err = SkyPanel.GenerateValidationMessage(err)
	}
	return
}

func (d *Database) BeforeSave(*gorm.DB) (err error) {
	err = d.IsValid()
	return
}

func (d *Database) TableName() string {
	return "databases"
}
