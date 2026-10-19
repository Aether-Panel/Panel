package models

import (
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// ActivityEvent names an access event kept in the panel activity feed.
type ActivityEvent = string

const (
	// ActivityLogin is a successful sign-in (password or OTP).
	ActivityLogin ActivityEvent = "login"
	// ActivityLoginFailed is a rejected sign-in attempt.
	ActivityLoginFailed ActivityEvent = "login_failed"
	// ActivityLogout is a session invalidation.
	ActivityLogout ActivityEvent = "logout"
	// ActivityRegister is an account creation (self-service or by an operator).
	ActivityRegister ActivityEvent = "register"
	// ActivityPasswordReset is a completed password reset.
	ActivityPasswordReset ActivityEvent = "password_reset"
)

// ActivityLog is one recorded access event. The username and email are copied
// (not only referenced) so a record survives the account being renamed or
// deleted.
type ActivityLog struct {
	ID     uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Event  string `gorm:"column:event;not null;size:32;index" json:"event"`
	UserID *uint  `gorm:"column:user_id;index" json:"-"`

	Username string `gorm:"column:username;not null;size:100" json:"-"`
	Email    string `gorm:"column:email;size:255" json:"-"`

	IP        string `gorm:"column:ip;size:64" json:"-"`
	UserAgent string `gorm:"column:user_agent;size:255" json:"-"`

	CreatedAt time.Time `gorm:"column:created_at;index" json:"-"`
}

// ActivityView is the API representation of an event. The user agent is only
// exposed as a short summary: the feed says where the sign-in came from without
// echoing a full browser string.
type ActivityView struct {
	ID        uint      `json:"id"`
	Event     string    `json:"event"`
	Username  string    `json:"username,omitempty"`
	Email     string    `json:"email,omitempty"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"userAgent,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
} // @name Activity

func FromActivity(model *ActivityLog) *ActivityView {
	return &ActivityView{
		ID:        model.ID,
		Event:     model.Event,
		Username:  model.Username,
		Email:     model.Email,
		IP:        model.IP,
		UserAgent: summariseUserAgent(model.UserAgent),
		CreatedAt: model.CreatedAt,
	}
}

// summariseUserAgent reduces a browser string to "browser on platform" so the
// feed stays readable and does not store a full fingerprint.
func summariseUserAgent(userAgent string) string {
	if userAgent == "" {
		return ""
	}

	client := ""
	switch {
	case strings.Contains(userAgent, "Edg/"):
		client = "Edge"
	case strings.Contains(userAgent, "OPR/"), strings.Contains(userAgent, "Opera"):
		client = "Opera"
	case strings.Contains(userAgent, "Firefox/"):
		client = "Firefox"
	case strings.Contains(userAgent, "Chrome/"):
		client = "Chrome"
	case strings.Contains(userAgent, "Safari/"):
		client = "Safari"
	case strings.Contains(userAgent, "curl/"), strings.Contains(userAgent, "Go-http-client"):
		client = "CLI"
	}

	platform := ""
	switch {
	case strings.Contains(userAgent, "Windows"):
		platform = "Windows"
	case strings.Contains(userAgent, "Android"):
		platform = "Android"
	case strings.Contains(userAgent, "iPhone"), strings.Contains(userAgent, "iPad"):
		platform = "iOS"
	case strings.Contains(userAgent, "Mac OS X"), strings.Contains(userAgent, "Macintosh"):
		platform = "macOS"
	case strings.Contains(userAgent, "Linux"):
		platform = "Linux"
	}

	switch {
	case client != "" && platform != "":
		return client + " · " + platform
	case client != "":
		return client
	default:
		return platform
	}
}

// ActivitySearch is the query accepted by GET /api/activity.
type ActivitySearch struct {
	Limit uint `form:"limit"`
	Page  uint `form:"page"`
	Event string `form:"event"`
} // @name ActivitySearch

type ActivitySearchResponse struct {
	Activity []*ActivityView `json:"activity"`
	*skypanel.Metadata
} // @name ActivitySearchResponse
