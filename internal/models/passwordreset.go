package models

import (
	"fmt"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"golang.org/x/crypto/blake2b"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
)

type PasswordReset struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	UserID    uint      `gorm:"column:user_id;not null;index" json:"-"`
	User      User      `gorm:"ASSOCIATION_SAVE_REFERENCE:false" json:"-" validate:"-"`
	TokenHash string    `gorm:"column:token;not null;size:64;uniqueIndex" json:"-"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null" json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (pr *PasswordReset) SetToken(token string) error {
	hash, err := blake2b.New256(nil)
	if err != nil {
		return err
	}

	_, err = hash.Write([]byte(token))
	if err != nil {
		return err
	}

	pr.TokenHash = fmt.Sprintf("%x", hash.Sum(nil))
	return nil
}

func (pr *PasswordReset) IsValid() (err error) {
	err = validator.New().Struct(pr)
	if err != nil {
		err = skypanel.GenerateValidationMessage(err)
	}
	return
}

func (pr *PasswordReset) BeforeSave(*gorm.DB) error {
	return pr.IsValid()
}
