package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSSOLinks_WorkOnce(t *testing.T) {
	token, err := SSOLinks.Create(7, "/servers/abc/")
	if !assert.NoError(t, err) {
		return
	}

	userID, redirect, ok := SSOLinks.Consume(token)
	assert.True(t, ok)
	assert.Equal(t, uint(7), userID)
	assert.Equal(t, "/servers/abc/", redirect)

	_, _, ok = SSOLinks.Consume(token)
	assert.False(t, ok, "a link is good for one sign-in only")
}

func TestSSOLinks_ExpireAndRejectUnknownTokens(t *testing.T) {
	_, _, ok := SSOLinks.Consume("not-a-token")
	assert.False(t, ok)

	token, err := SSOLinks.Create(7, "/")
	if !assert.NoError(t, err) {
		return
	}
	SSOLinks.mu.Lock()
	link := SSOLinks.links[hashSSOToken(token)]
	link.expires = time.Now().Add(-time.Second)
	SSOLinks.links[hashSSOToken(token)] = link
	SSOLinks.mu.Unlock()

	_, _, ok = SSOLinks.Consume(token)
	assert.False(t, ok, "an expired link does not sign in")
}
