package models

import (
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"gorm.io/gorm"
)

type Permissions struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"-"`

	//owners of this permission set
	UserId *uint `gorm:"column:user_id;index" json:"-"`
	User   User  `gorm:"ASSOCIATION_SAVE_REFERENCE:false" json:"-" validate:"-"`

	ClientID *uint  `gorm:"column:client_id;index" json:"-"`
	Client   Client `gorm:"ASSOCIATION_SAVE_REFERENCE:false" json:"-" validate:"-"`

	//if this set is for a server, what server
	ServerIdentifier *string `gorm:"column:server_identifier;size:20;index" json:"-"`
	Server           Server  `gorm:"ASSOCIATION_SAVE_REFERENCE:false" json:"-" validate:"-"`

	RawScopes string          `gorm:"column:scopes;not null;size:1000;default:''" json:"-" validate:"required"`
	Scopes    []*scopes.Scope `gorm:"-" json:"-"`
}

func (p *Permissions) BeforeSave(*gorm.DB) error {
	if p.ServerIDentifier != nil && *p.ServerIDentifier == "" {
		p.ServerIDentifier = nil
	}

	if p.ServerIDentifier != nil {
		//ensure they have the view, because we're saving them back in
		p.Scopes = scopes.AddScope(p.Scopes, scopes.ScopeServerView)
	}

	tmp := make([]string, len(p.Scopes))
	for k, v := range p.Scopes {
		tmp[k] = v.String()
	}
	p.RawScopes = strings.Join(tmp, ",")
	return nil
}

func (p *Permissions) AfterFind(*gorm.DB) error {
	p.Scopes = make([]*scopes.Scope, 0)
	if p.RawScopes != "" {
		for _, v := range strings.Split(p.RawScopes, ",") {
			//we can just simply blindly assign it, because the checks we do are just making these strings anyways...
			p.Scopes = append(p.Scopes, scopes.GetScope(v))
		}
	}

	return nil
}

func (p *Permissions) ShouldDelete() bool {
	if p.ServerIDentifier == nil {
		return false
	}
	return len(p.Scopes) == 0
}
