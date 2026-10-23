package services

import (
	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_tokenService_GenerateRequest(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "TestGenerate",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := NewTokenService()
			if !assert.NoError(t, err) {
				return
			}
			got, err := ts.GenerateRequest()
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			// now check if we can decrypt it
			err = ts.ValidateRequest(got)
			if !assert.NoError(t, err) {
				return
			}
		})
	}
}

func signTestToken(t *testing.T, claims jwt.MapClaims) string {
	token := jwt.New(jwt.SigningMethodEdDSA)
	token.Header[jwkset.HeaderKID] = keyID
	token.Claims = claims
	signed, err := token.SignedString(privateKey)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return signed
}

func Test_tokenService_RequestTokensExpire(t *testing.T) {
	ts, err := NewTokenService()
	if !assert.NoError(t, err) {
		return
	}

	signed, err := ts.GenerateRequest()
	if !assert.NoError(t, err) {
		return
	}
	parsed, _, err := jwt.NewParser().ParseUnverified(signed, jwt.MapClaims{})
	if !assert.NoError(t, err) {
		return
	}
	exp, err := parsed.Claims.GetExpirationTime()
	if assert.NoError(t, err) && assert.NotNil(t, exp) {
		assert.WithinDuration(t, time.Now().Add(requestTokenLifetime), exp.Time, time.Minute)
	}

	expired := signTestToken(t, jwt.MapClaims{"exp": time.Now().Add(-time.Hour).Unix()})
	assert.Error(t, ts.ValidateRequest(expired), "an expired token must be rejected")

	// tokens from panels that do not send exp yet keep working
	legacy := signTestToken(t, jwt.MapClaims{})
	assert.NoError(t, ts.ValidateRequest(legacy))
}
