package client

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"gorm.io/gorm"
)

type ClientRepo struct {
	DB *gorm.DB
}

// Get Gets a specific OAuth client, including the scopes it holds
func (s *ClientRepo) Get(clientID string) (*domain.Client, error) {
	client := &domain.Client{
		ClientID: clientID,
	}
	err := s.DB.Where(client).First(client).Error
	return client, err
}

// GetForUser Gets all clients for a user
func (s *ClientRepo) GetForUser(userID uint) ([]*domain.Client, error) {
	client := &domain.Client{
		UserID: userID,
	}
	var clients []*domain.Client
	err := s.DB.Where(client).Find(&clients).Error
	return clients, err
}

func (s *ClientRepo) Create(request *domain.Client) error {
	return s.DB.Create(request).Error
}

func (s *ClientRepo) Update(request *domain.Client) error {
	return s.DB.Save(request).Error
}

func (s *ClientRepo) Delete(clientID string) error {
	client := &domain.Client{
		ClientID: clientID,
	}
	return s.DB.Model(client).Delete(client, client).Error
}
