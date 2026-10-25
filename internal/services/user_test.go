package services

import (
	"strings"
	"testing"
)

func TestIsSecurePassword(t *testing.T) {
	us := &User{}
	cases := []struct {
		name     string
		password string
		ok       bool
	}{
		{"too short", "1234567", false},
		{"shortest allowed", "12345678", true},
		{"bcrypt's limit", strings.Repeat("a", 72), true},
		{"over bcrypt's limit", strings.Repeat("a", 73), false},
		// 25 three-byte characters are 75 bytes, over the limit though only 25 letters
		{"multi-byte over the limit", strings.Repeat("€", 25), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := us.IsSecurePassword(c.password); (err == nil) != c.ok {
				t.Fatalf("IsSecurePassword(%d bytes) = %v, want ok=%v", len(c.password), err, c.ok)
			}
		})
	}
}
