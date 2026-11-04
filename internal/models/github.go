package models

import "time"

// GitHubAccount is the GitHub account a panel user connected, to install their repositories
// into servers. Each user has at most one; the token is sealed (services.SealGitHubToken).
type GitHubAccount struct {
	UserID    uint   `gorm:"column:user_id;primaryKey;autoIncrement:false" json:"-"`
	Login     string `gorm:"column:login;not null;size:100" json:"login"`
	AvatarURL string `gorm:"column:avatar_url;size:255" json:"avatarUrl"`
	Token     string `gorm:"column:token;not null;type:text" json:"-"`
	Scopes    string `gorm:"column:scopes;size:255" json:"-"`

	CreatedAt time.Time `json:"connectedAt"`
	UpdatedAt time.Time `json:"-"`
}

func (GitHubAccount) TableName() string {
	return "github_accounts"
}
