package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"
)

// SSOLinkLifetime is how long a sign-in link from the billing system stays usable. The
// billing system sends the browser straight to it, so a minute is plenty.
const SSOLinkLifetime = time.Minute

// SSOLinks holds the one-time sign-in links handed to the billing system (WHMCS, Paymenter)
// for its "go to the panel" button. Only a hash of each token is kept, in memory: a link
// left unused simply expires, and a panel restart forgets every pending one.
var SSOLinks = &ssoLinkStore{links: map[string]ssoLink{}}

type ssoLink struct {
	userID   uint
	redirect string
	expires  time.Time
}

type ssoLinkStore struct {
	mu    sync.Mutex
	links map[string]ssoLink
}

// Create returns a new token that signs userID in once and then sends them to redirect.
func (s *ssoLinkStore) Create(userID uint, redirect string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, l := range s.links {
		if now.After(l.expires) {
			delete(s.links, k)
		}
	}
	s.links[hashSSOToken(token)] = ssoLink{userID: userID, redirect: redirect, expires: now.Add(SSOLinkLifetime)}
	return token, nil
}

// Consume spends the token: the first call within its lifetime gets the user and where to
// send them, every later call (or a late one) gets ok false.
func (s *ssoLinkStore) Consume(token string) (userID uint, redirect string, ok bool) {
	key := hashSSOToken(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	link, found := s.links[key]
	delete(s.links, key)
	if !found || time.Now().After(link.expires) {
		return 0, "", false
	}
	return link.userID, link.redirect, true
}

func hashSSOToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
