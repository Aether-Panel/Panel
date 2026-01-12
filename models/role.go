package models

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Role struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;not null;size:100;uniqueIndex;unique" json:"name" validate:"required,printascii,max=100,min=1"`
	Description string    `gorm:"column:description;size:500" json:"description"`
	RawScopes   string    `gorm:"column:scopes;not null;size:2000;default:''" json:"-"`
	Scopes      []string  `gorm:"-" json:"scopes"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (r *Role) BeforeSave(*gorm.DB) error {
	// Convertir Scopes a string separado por comas
	if len(r.Scopes) > 0 {
		r.RawScopes = strings.Join(r.Scopes, ",")
	} else {
		r.RawScopes = ""
	}
	
	// Validar
	err := validator.New().Struct(r)
	if err != nil {
		return SkyPanel.GenerateValidationMessage(err)
	}
	return nil
}

func (r *Role) AfterFind(*gorm.DB) error {
	// Convertir RawScopes a array
	r.Scopes = make([]string, 0)
	if r.RawScopes != "" {
		r.Scopes = strings.Split(r.RawScopes, ",")
		// Limpiar espacios en blanco
		for i, scope := range r.Scopes {
			r.Scopes[i] = strings.TrimSpace(scope)
		}
	}
	return nil
}

