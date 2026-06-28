package models

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gopkg.in/go-playground/validator.v9"
)

type UserView struct {
	Id        uint            `json:"id,omitempty"`
	Username  string          `json:"username,omitempty"`
	Email     string          `json:"email,omitempty"`
	OtpActive bool            `json:"otpActive"`
	RoleId    *uint           `json:"roleId,omitempty"`
	Scopes    []*scopes.Scope `json:"scopes,omitempty"`
	//ONLY SHOW WHEN COPYING
	Password    string `json:"password,omitempty"`
	NewPassword string `json:"newPassword,omitempty"`
} //@name User

func FromUser(model *User) *UserView {
	view := &UserView{
		Id:        model.ID,
		Username:  model.Username,
		Email:     model.Email,
		OtpActive: model.OtpActive,
		RoleId:    model.RoleId,
		Scopes:    make([]*scopes.Scope, 0),
	}

	// Add individual permissions
	for _, p := range model.Permissions {
		if p.ServerIdentifier == nil || *p.ServerIdentifier == "" {
			for _, s := range p.Scopes {
				view.Scopes = scopes.AddScope(view.Scopes, s)
			}
		}
	}

	// Add role-based permissions
	if model.RoleId != nil && model.Role.ID != 0 {
		for _, s := range model.Role.Scopes {
			view.Scopes = scopes.AddScope(view.Scopes, scopes.GetScope(s))
		}
	}

	return view
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

	newModel.RoleId = model.RoleId
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

	if validate.Var(model.Username, "omitempty,min=1,max=100") != nil {
		return SkyPanel.ErrFieldLength("username", 1, 100)
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
