package models

import (
	"time"

	"github.com/SkyPanel/SkyPanel/v3"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
)

type Backup struct {
	ID       uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name     string `gorm:"NOT NULL;default='generic'" json:"name" validate:"required,printascii"`
	FileName string `gorm:"NOT NULL;default='generic'" json:"fileName" validate:"required,printascii"`
	//FileSize int64  `gorm:"NOT NULL;default:0" json:"fileSize"`

	ServerID string `gorm:"column:server_id;" json:"-" validate:"-"`
	Server   Server `gorm:"foreignKey:ServerID;->;<-:create" json:"-" validate:"-"`

	CreatedAt time.Time `json:"createdAt"`
}

func (s *Backup) IsValid() (err error) {
	err = validator.New().Struct(s)
	if err != nil {
		err = SkyPanel.GenerateValidationMessage(err)
	}
	return
}

func (s *Backup) BeforeSave(*gorm.DB) (err error) {
	err = s.IsValid()
	return
}
