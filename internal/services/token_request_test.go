package services

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func makeRequestTestKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return pub, priv
}

func signRequestTestToken(t *testing.T, priv ed25519.PrivateKey, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(priv)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return signed
}

func parseRequestTestToken(signed string, pub ed25519.PublicKey) (*jwt.Token, error) {
	keyFunc := func(*jwt.Token) (interface{}, error) { return pub, nil }
	return jwt.Parse(signed, keyFunc, requestParseOptions()...)
}

func TestRequestParseOptions_RejectTokenWithoutExp(t *testing.T) {
	pub, priv := makeRequestTestKey(t)

	signed := signRequestTestToken(t, priv, jwt.MapClaims{"iat": time.Now().Unix()})
	_, err := parseRequestTestToken(signed, pub)
	assert.Error(t, err)

	signed = signRequestTestToken(t, priv, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	_, err = parseRequestTestToken(signed, pub)
	assert.NoError(t, err)
}

func TestRequestParseOptions_RejectNonEdDSATokens(t *testing.T) {
	secret := []byte("shared-hmac-secret")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	signed, err := token.SignedString(secret)
	assert.NoError(t, err)

	keyFunc := func(*jwt.Token) (interface{}, error) { return secret, nil }
	_, err = jwt.Parse(signed, keyFunc, requestParseOptions()...)
	assert.Error(t, err)
}

func TestCheckRequestAudience_MatchesConfiguredNodeSecret(t *testing.T) {
	_ = config.ClientSecret.Set("node-secret-abc", false)
	t.Cleanup(func() { _ = config.ClientSecret.Set("", false) })

	pub, priv := makeRequestTestKey(t)

	good := signRequestTestToken(t, priv, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute).Unix(),
		"aud": nodeAudience("node-secret-abc"),
	})
	parsed, err := parseRequestTestToken(good, pub)
	assert.NoError(t, err)
	assert.NoError(t, checkRequestAudience(parsed.Claims))

	// a token minted for another node must not be replayable here
	wrong := signRequestTestToken(t, priv, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute).Unix(),
		"aud": nodeAudience("some-other-node"),
	})
	parsed, err = parseRequestTestToken(wrong, pub)
	assert.NoError(t, err)
	assert.Error(t, checkRequestAudience(parsed.Claims))

	// a token without an audience is no longer trusted anywhere
	missing := signRequestTestToken(t, priv, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	parsed, err = parseRequestTestToken(missing, pub)
	assert.NoError(t, err)
	assert.Error(t, checkRequestAudience(parsed.Claims))
}

func TestCheckRequestAudience_FallsBackToLocalNodeSecret(t *testing.T) {
	_ = config.ClientSecret.Set("", false)

	pub, priv := makeRequestTestKey(t)
	signed := signRequestTestToken(t, priv, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute).Unix(),
		"aud": nodeAudience(models.LocalNode.Secret),
	})
	parsed, err := parseRequestTestToken(signed, pub)
	assert.NoError(t, err)
	assert.NoError(t, checkRequestAudience(parsed.Claims))
}

func TestRequestClaims_BindAudienceToTargetNode(t *testing.T) {
	node := &models.Node{
		ID:          5,
		Name:        "remote-node",
		PublicHost:  "10.11.12.13",
		PrivateHost: "10.11.12.13",
		PublicPort:  8080,
		PrivatePort: 8080,
		SFTPPort:    5657,
		Secret:      "remote-node-secret",
	}

	claims := requestClaims(node)
	assert.Equal(t, nodeAudience("remote-node-secret"), claims["aud"])
	assert.NotNil(t, claims["exp"])

	// the local node signs for its own derived identity
	local := requestClaims(models.LocalNode)
	assert.Equal(t, nodeAudience(models.LocalNode.Secret), local["aud"])
}
