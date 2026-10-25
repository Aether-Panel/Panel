package services

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"sync"
	"time"
)

type TokenService interface {
	GetKeyFunc() jwt.Keyfunc
	GetTokenStore() jwkset.Storage
	GenerateRequest(node *models.Node) (string, error)
	ValidateRequest(string) error
}

type tokenService struct{}

var externalService keyfunc.Keyfunc
var tokenServiceLocker sync.Mutex
var tokenStore jwkset.Storage
var privateKey crypto.PrivateKey

const keyID = "SkyPanel"

func NewTokenService() (TokenService, error) {
	tokenServiceLocker.Lock()
	defer tokenServiceLocker.Unlock()

	if config.PanelEnabled.Value() {
		if tokenStore == nil {
			key := config.PrivateKey.Value()

			var randData []byte
			var err error
			if key == "" {
				randData = make([]byte, ed25519.SeedSize)
				_, err = rand.Read(randData)
				if err != nil {
					return nil, err
				}
				enc := base64.StdEncoding.EncodeToString(randData)
				_ = config.PrivateKey.Set(enc, true)
			} else {
				randData, err = base64.StdEncoding.DecodeString(key)
				if err != nil {
					return nil, err
				}
			}

			// Create a cryptographic key.
			priv := ed25519.NewKeyFromSeed(randData)
			pub := priv.Public()
			privateKey = priv

			// Turn the key into a JWK.
			options := jwkset.JWKOptions{
				Marshal: jwkset.JWKMarshalOptions{
					Private: false,
				},
				Metadata: jwkset.JWKMetadataOptions{
					KID: keyID,
				},
			}

			privateJwk, err := jwkset.NewJWKFromKey(pub, options)
			if err != nil {
				return nil, err
			}

			tokenStore = jwkset.NewMemoryStorage()
			err = tokenStore.KeyWrite(context.Background(), privateJwk)
			if err != nil {
				return nil, err
			}
		}

		if externalService == nil {
			var err error
			externalService, err = keyfunc.New(keyfunc.Options{
				Storage: tokenStore,
			})
			if err != nil {
				return nil, err
			}
		}
	} else if externalService == nil {
		var err error
		externalService, err = keyfunc.NewDefault([]string{config.TokenPublicURL.Value()})
		if err != nil {
			return nil, err
		}
	}

	return &tokenService{}, nil
}

// requestTokenLifetime bounds how long a token sent to a node stays usable. Every token is
// used right after it is created, so this only needs to cover clock differences between
// the panel and its nodes.
const requestTokenLifetime = 15 * time.Minute

// nodeAudience derives a token audience from a node secret. Both sides already share
// that secret (the panel stores it, the daemon has it as daemon.auth.clientSecret),
// and hashing it keeps the secret itself out of the (signed, not encrypted) token.
func nodeAudience(secret string) string {
	sum := sha256.Sum256([]byte("skypanel-daemon-aud:" + secret))
	return hex.EncodeToString(sum[:])
}

// requestClaims builds the claims for a panel→daemon token: short-lived and bound to
// the one node it is meant for, so a captured token cannot replay against other nodes.
func requestClaims(node *models.Node) jwt.MapClaims {
	now := time.Now()

	secret := models.LocalNode.Secret
	if node != nil && !node.IsLocal() {
		secret = node.Secret
	}

	return jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(requestTokenLifetime).Unix(),
		"aud": nodeAudience(secret),
	}
}

func (ts *tokenService) GenerateRequest(node *models.Node) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, requestClaims(node))
	token.Header[jwkset.HeaderKID] = keyID

	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", err
	}
	return signed, nil
}

// requestParseOptions are the checks every panel→daemon token must pass: only the
// panel's EdDSA signature and a mandatory, bounded expiry.
func requestParseOptions() []jwt.ParserOption {
	return []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(time.Minute),
	}
}

// checkRequestAudience confirms the token was minted for this node and not captured
// from traffic to another one.
func checkRequestAudience(claims jwt.Claims) error {
	auds, err := claims.GetAudience()
	if err != nil || len(auds) == 0 {
		return jwt.ErrTokenRequiredClaimMissing
	}

	// a combined panel+daemon install answers as the local node; a deployed daemon
	// answers as the node secret it was provisioned with
	expected := []string{nodeAudience(models.LocalNode.Secret)}
	if s := config.ClientSecret.Value(); s != "" {
		expected = append(expected, nodeAudience(s))
	}

	for _, aud := range auds {
		for _, want := range expected {
			if aud == want {
				return nil
			}
		}
	}
	return jwt.ErrTokenInvalidAudience
}

func (ts *tokenService) ValidateRequest(token string) error {
	parsed, err := jwt.Parse(token, ts.GetKeyFunc(), requestParseOptions()...)
	if err != nil {
		return err
	}
	if !parsed.Valid {
		return jwt.ErrTokenSignatureInvalid
	}
	return checkRequestAudience(parsed.Claims)
}

func (ts *tokenService) GetKeyFunc() jwt.Keyfunc {
	return externalService.Keyfunc
}

func (ts *tokenService) GetTokenStore() jwkset.Storage {
	return tokenStore
}
