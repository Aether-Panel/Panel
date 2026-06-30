package services

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"gorm.io/gorm"
)

type OAuth2 struct {
	DB *gorm.DB
}

// Get Gets a specific OAuth client, including the scopes it holds
func (s *OAuth2) Get(clientID string) (*models.Client, error) {
	client := &models.Client{
		ClientID: clientID,
	}
	err := s.DB.Where(client).First(client).Error
	return client, err
}

// GetForUser Gets all clients for a user
func (s *OAuth2) GetForUser(userID uint) ([]*models.Client, error) {
	client := &models.Client{
		UserID: userID,
	}
	var clients []*models.Client
	err := s.DB.Where(client).Find(&clients).Error
	return clients, err
}

func (s *OAuth2) Create(request *models.Client) error {
	return s.DB.Create(request).Error
}

func (s *OAuth2) Update(request *models.Client) error {
	return s.DB.Save(request).Error
}

func (s *OAuth2) Delete(clientID string) error {
	client := &models.Client{
		ClientID: clientID,
	}
	return s.DB.Model(client).Delete(client, client).Error
}
