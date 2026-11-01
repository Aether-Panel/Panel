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

// Delete removes the client and the sessions its credentials opened: a leaked token kept
// working (each call renewed it) after its owner deleted the client
func (s *OAuth2) Delete(clientID string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		client := &models.Client{}
		if err := tx.Where(&models.Client{ClientID: clientID}).First(client).Error; err != nil {
			return err
		}
		if err := tx.Where("client_id = ?", client.ID).Delete(&models.Session{}).Error; err != nil {
			return err
		}
		return tx.Delete(client).Error
	})
}
