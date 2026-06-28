package services

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"gorm.io/gorm"
)

type DatabaseHost struct {
	DB *gorm.DB
}

func (dhs *DatabaseHost) GetAll() ([]*models.DatabaseHost, error) {
	var hosts []*models.DatabaseHost

	res := dhs.DB.Find(&hosts)

	if res.Error != nil {
		return nil, res.Error
	}

	return hosts, nil
}

func (dhs *DatabaseHost) Get(id uint) (*models.DatabaseHost, error) {
	model := &models.DatabaseHost{}

	res := dhs.DB.First(model, id)
	return model, res.Error
}

func (dhs *DatabaseHost) Update(model *models.DatabaseHost) error {
	res := dhs.DB.Save(model)
	return res.Error
}

func (dhs *DatabaseHost) Delete(id uint) error {
	model := &models.DatabaseHost{
		ID: id,
	}

	res := dhs.DB.Delete(model)
	return res.Error
}

func (dhs *DatabaseHost) Create(host *models.DatabaseHost) error {
	res := dhs.DB.Create(host)
	return res.Error
}

