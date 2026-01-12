package models

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"gopkg.in/go-playground/validator.v9"
	"net/url"
)

type UserView struct {
	Id       uint   `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	OtpActive bool `json:"otpActive"`
	RoleId   *uint  `json:"roleId,omitempty"`
	//ONLY SHOW WHEN COPYING
	Password    string `json:"password,omitempty"`
	NewPassword string `json:"newPassword,omitempty"`
} //@name User

func FromUser(model *User) *UserView {
	return &UserView{
		Id:       model.ID,
		Username: model.Username,
		Email:    model.Email,
		OtpActive: model.OtpActive,
		RoleId:   model.RoleId,
	}
}

func FromUsers(users []*User) []*UserView {
	result := make([]*UserView, len(users))

	for k, v := range users {
		result[k] = FromUser(v)
	}

	return result
}

func (model *UserView) CopyToModel(newModel *User) {
	if model.Username != "" {
		newModel.Username = model.Username
	}

	if model.Email != "" {
		newModel.Email = model.Email
	}

	if model.Password != "" {
		_ = newModel.SetPassword(model.Password)
	}

	if model.RoleId != nil {
		newModel.RoleId = model.RoleId
	}
}

func (model *UserView) Valid(allowEmpty bool) error {

	userNameErr := model.UserNameValid(allowEmpty)
	if userNameErr != nil {
		return userNameErr
	}

	mailErr := model.EmailValid(allowEmpty)
	if mailErr != nil {
		return mailErr
	}

	return nil
}

func (model *UserView) UserNameValid(allowEmpty bool) error {
	validate := validator.New()

	if !allowEmpty && validate.Var(model.Username, "required") != nil {
		return SkyPanel.ErrFieldRequired("username")
	}

	if validate.Var(model.Username, "omitempty,printascii") != nil {
		return SkyPanel.ErrFieldMustBePrintable("username")
	}

	if validate.Var(model.Username, "omitempty,min=5,max=100") != nil {
		return SkyPanel.ErrFieldLength("username", 5, 100)
	}

	testName := url.QueryEscape(model.Username)
	if testName != model.Username {
		return SkyPanel.ErrFieldHasURICharacters("username")
	}

	return nil
}

func (model *UserView) EmailValid(allowEmpty bool) error {
	validate := validator.New()

	if !allowEmpty && validate.Var(model.Email, "required") != nil {
		return SkyPanel.ErrFieldRequired("email")
	}

	if validate.Var(model.Email, "omitempty,email,max=255") != nil {
		return SkyPanel.ErrFieldNotEmail("email")
	}

	return nil
}
