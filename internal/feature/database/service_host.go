package database

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"gorm.io/gorm"
)

type DatabaseHostRepo struct {
	DB *gorm.DB
}

func (dhs *DatabaseHostRepo) GetAll() ([]*domain.DatabaseHost, error) {
	var hosts []*domain.DatabaseHost

	res := dhs.DB.Find(&hosts)

	if res.Error != nil {
		return nil, res.Error
	}

	return hosts, nil
}

func (dhs *DatabaseHostRepo) Get(id uint) (*domain.DatabaseHost, error) {
	model := &domain.DatabaseHost{}

	res := dhs.DB.First(model, id)
	return model, res.Error
}

func (dhs *DatabaseHostRepo) Update(model *domain.DatabaseHost) error {
	res := dhs.DB.Save(model)
	return res.Error
}

func (dhs *DatabaseHostRepo) Delete(id uint) error {
	model := &domain.DatabaseHost{
		ID: id,
	}

	res := dhs.DB.Delete(model)
	return res.Error
}

func (dhs *DatabaseHostRepo) Create(host *domain.DatabaseHost) error {
	res := dhs.DB.Create(host)
	return res.Error
}
