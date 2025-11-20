package models

import (
	"time"

	"github.com/SkyPanel/SkyPanel/v3"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
)

type UptimeStatus struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID  string    `gorm:"column:server_id;not null;size:20;index" json:"-" validate:"required,printascii"`
	Server    Server    `gorm:"foreignKey:ServerID;->;<-:create" json:"-" validate:"-"`
	IsRunning bool      `gorm:"column:is_running;not null" json:"isRunning"`
	StartTime time.Time `gorm:"column:start_time;not null;index" json:"startTime"`
	EndTime   *time.Time `gorm:"column:end_time;index" json:"endTime,omitempty"`
	Duration  int64     `gorm:"column:duration;default:0" json:"duration"` // Duración en segundos

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (u *UptimeStatus) IsValid() (err error) {
	err = validator.New().Struct(u)
	if err != nil {
		err = SkyPanel.GenerateValidationMessage(err)
	}
	return
}

func (u *UptimeStatus) BeforeSave(*gorm.DB) (err error) {
	err = u.IsValid()
	return
}
