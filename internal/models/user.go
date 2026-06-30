package models

import (
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
)

type User struct {
	ID             uint   `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	Username       string `gorm:"column:username;not null;size:100;uniqueIndex;unique" json:"-" validate:"required,printascii,max=100,min=1"`
	Email          string `gorm:"column:email;not null;size:255;uniqueIndex;unique" json:"-" validate:"required,email,max=255"`
	HashedPassword string `gorm:"column:password;NOT NULL;size:200" json:"-" validate:"required,max=200"`
	OtpSecret      string `gorm:"column:otp_secret;size:32" json:"-"`
	OtpActive      bool   `gorm:"column:otp_active;not null;DEFAULT:0" json:"-"`

	RoleID      *uint         `gorm:"column:role_id;index" json:"-"`
	Role        Role          `gorm:"ASSOCIATION_SAVE_REFERENCE:false" json:"-" validate:"-"`
	Permissions []Permissions `gorm:"foreignKey:UserID" json:"-"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (u *User) SetPassword(pw string) error {
	res, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)

	if err == nil {
		u.HashedPassword = string(res)
	}

	return err
}

func (u *User) IsValid() (err error) {
	err = validator.New().Struct(u)
	if err != nil {
		err = skypanel.GenerateValidationMessage(err)
	}
	return
}

func (u *User) BeforeSave(*gorm.DB) (err error) {
	err = u.IsValid()
	return
}
