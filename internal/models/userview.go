package models

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gopkg.in/go-playground/validator.v9"
)

type UserView struct {
	ID        uint            `json:"id,omitempty"`
	Username  string          `json:"username,omitempty"`
	Email     string          `json:"email,omitempty"`
	OtpActive bool            `json:"otpActive"`
	RoleID    *uint           `json:"roleId,omitempty"`
	Scopes    []*scopes.Scope `json:"scopes,omitempty"`
	//ONLY SHOW WHEN COPYING
	Password    string `json:"password,omitempty"`
	NewPassword string `json:"newPassword,omitempty"`
} //@name User

func FromUser(model *User) *UserView {
	view := &UserView{
		ID:        model.ID,
		Username:  model.Username,
		Email:     model.Email,
		OtpActive: model.OtpActive,
		RoleID:    model.RoleID,
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
	if model.RoleID != nil && model.Role.ID != 0 {
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

	newModel.RoleID = model.RoleID
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
		return skypanel.ErrFieldRequired("username")
	}

	if validate.Var(model.Username, "omitempty,printascii") != nil {
		return skypanel.ErrFieldMustBePrintable("username")
	}

	if validate.Var(model.Username, "omitempty,min=1,max=100") != nil {
		return skypanel.ErrFieldLength("username", 1, 100)
	}

	return nil
}

func (model *UserView) EmailValid(allowEmpty bool) error {
	validate := validator.New()

	if !allowEmpty && validate.Var(model.Email, "required") != nil {
		return skypanel.ErrFieldRequired("email")
	}

	if validate.Var(model.Email, "omitempty,email,max=255") != nil {
		return skypanel.ErrFieldNotEmail("email")
	}

	return nil
}
