package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFromActivity_CopiesWhatTheFeedShows(t *testing.T) {
	userID := uint(7)
	created := time.Now().Add(-time.Minute)
	model := &ActivityLog{
		ID:        3,
		Event:     ActivityLogin,
		UserID:    &userID,
		Username:  "operator",
		Email:     "operator@example.com",
		IP:        "203.0.113.7",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		CreatedAt: created,
	}

	view := FromActivity(model)

	assert.Equal(t, uint(3), view.ID)
	assert.Equal(t, ActivityLogin, view.Event)
	assert.Equal(t, "operator", view.Username)
	assert.Equal(t, "operator@example.com", view.Email)
	assert.Equal(t, "203.0.113.7", view.IP)
	assert.Equal(t, "Chrome · Windows", view.UserAgent)
	assert.Equal(t, created, view.CreatedAt)
}

func TestFromActivity_SummarisesUserAgent(t *testing.T) {
	cases := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "chrome on windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want:      "Chrome · Windows",
		},
		{
			name:      "firefox on macos",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
			want:      "Firefox · macOS",
		},
		{
			name:      "safari on iphone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
			want:      "Safari · iOS",
		},
		{
			name:      "curl",
			userAgent: "curl/8.4.0",
			want:      "CLI",
		},
		{
			name:      "unknown client on linux",
			userAgent: "somebot/1.0 (Linux)",
			want:      "Linux",
		},
		{
			name:      "nothing to say",
			userAgent: "",
			want:      "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, FromActivity(&ActivityLog{UserAgent: testCase.userAgent}).UserAgent)
		})
	}
}
