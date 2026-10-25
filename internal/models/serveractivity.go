package models

import "time"

// ServerActivityDetailMax caps how much context one activity entry keeps
const ServerActivityDetailMax = 300

// ServerActivity is one change made to a server through the panel: who did it, what it was
// and when. The username is copied so the entry still reads right after the account is
// renamed or deleted.
type ServerActivity struct {
	ID       uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID string `gorm:"column:server_id;size:20;not null;index" json:"-"`
	UserID   *uint  `gorm:"column:user_id" json:"-"`
	Username string `gorm:"column:username;size:100;not null;default:''" json:"-"`

	// Action is a short key such as "server.start", "file.write" or "backup.restore"
	Action string `gorm:"column:action;size:48;not null" json:"-"`
	Detail string `gorm:"column:detail;size:300;not null;default:''" json:"-"`
	IP     string `gorm:"column:ip;size:64;not null;default:''" json:"-"`

	CreatedAt time.Time `gorm:"column:created_at;index" json:"-"`
}

// ServerActivityView is the API representation of an entry. The IP is only filled in for
// callers who may see it.
type ServerActivityView struct {
	ID        uint      `json:"id"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	Username  string    `json:"username"`
	UserID    *uint     `json:"userId"`
	IP        string    `json:"ip,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
} // @name ServerActivity

// FromServerActivity converts an entry for the API, keeping the IP only when asked to
func FromServerActivity(model *ServerActivity, withIP bool) *ServerActivityView {
	view := &ServerActivityView{
		ID:        model.ID,
		Action:    model.Action,
		Detail:    model.Detail,
		Username:  model.Username,
		UserID:    model.UserID,
		CreatedAt: model.CreatedAt,
	}
	if withIP {
		view.IP = model.IP
	}
	return view
}

// ServerActivityResponse is one page of a server's activity, newest first. NextCursor is
// the id to pass as "before" for the next page, or null on the last one.
type ServerActivityResponse struct {
	Items      []*ServerActivityView `json:"items"`
	NextCursor *uint                 `json:"nextCursor"`
} // @name ServerActivityResponse
