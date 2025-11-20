package models

import (
	"github.com/SkyPanel/SkyPanel/v3/scopes"
)

type PermissionView struct {
	ServerIdentifier string `json:"serverIdentifier,omitempty"`

	Scopes []*scopes.Scope `json:"scopes"`
} //@name Permissions

func FromPermission(p *Permissions) *PermissionView {
	model := &PermissionView{
		Scopes: p.Scopes,
	}

	if model.Scopes == nil {
		model.Scopes = make([]*scopes.Scope, 0)
	}

	return model
}

type UserPermissionsView struct {
	Username string          `json:"username,omitempty"`
	Email    string          `json:"email"`
	Scopes   []*scopes.Scope `json:"scopes"`
}
