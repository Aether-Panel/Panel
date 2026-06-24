package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/models"
	"gorm.io/gorm"
)

type APIKeyService struct {
	DB *gorm.DB
}

// GenerateKey generates a new API key and returns the unhashed token and the model to save.
func (s *APIKeyService) GenerateKey(name string, permissions []string) (string, *models.APIKey, error) {
	// Generate 24 random bytes
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, err
	}

	// Create token: ak_ + 5 hex chars prefix + _ + 43 hex chars
	fullHex := hex.EncodeToString(randomBytes) // 48 chars
	prefix := "ak_" + fullHex[:5] // exactly 8 chars
	token := prefix + "_" + fullHex[5:]

	// Hash the token for storage
	hasher := sha256.New()
	hasher.Write([]byte(token))
	hashedKey := hex.EncodeToString(hasher.Sum(nil))

	apiKey := &models.APIKey{
		Name:        name,
		Prefix:      prefix,
		HashedKey:   hashedKey,
		Permissions: permissions,
	}

	err := s.DB.Create(apiKey).Error
	if err != nil {
		return "", nil, err
	}

	return token, apiKey, nil
}

// ValidateKey validates an API key and returns it if valid.
func (s *APIKeyService) ValidateKey(token string) (*models.APIKey, error) {
	parts := strings.Split(token, "_")
	if len(parts) != 3 || parts[0] != "ak" {
		return nil, errors.New("invalid API key format")
	}
	prefix := parts[0] + "_" + parts[1]

	hasher := sha256.New()
	hasher.Write([]byte(token))
	hashedKey := hex.EncodeToString(hasher.Sum(nil))

	var apiKey models.APIKey
	err := s.DB.Where("prefix = ? AND hashed_key = ?", prefix, hashedKey).First(&apiKey).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid API key")
		}
		return nil, err
	}

	return &apiKey, nil
}

func (s *APIKeyService) GetAll() ([]*models.APIKey, error) {
	var keys []*models.APIKey
	err := s.DB.Find(&keys).Error
	return keys, err
}

func (s *APIKeyService) Delete(id uint) error {
	return s.DB.Delete(&models.APIKey{}, id).Error
}
