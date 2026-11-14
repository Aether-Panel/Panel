package services

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
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

// Delete removes the host, or returns skypanel.ErrDatabaseHostInUse while it still has
// databases: removing it would drop their rows (the foreign key cascades) and leave the
// databases and their users on the MySQL server with nothing in the panel to delete them.
func (dhs *DatabaseHost) Delete(id uint) error {
	var databases int64
	if err := dhs.DB.Model(&models.Database{}).Where("database_host_id = ?", id).Count(&databases).Error; err != nil {
		return err
	}
	if databases > 0 {
		return skypanel.ErrDatabaseHostInUse
	}

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
