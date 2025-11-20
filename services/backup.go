package services

import (
	"github.com/SkyPanel/SkyPanel/v3/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Backup struct {
	DB *gorm.DB
}

func (bs *Backup) GetAllForServer(serverID string) ([]*models.Backup, error) {
	var records []*models.Backup
	err := bs.DB.Where(&models.Backup{ServerID: serverID}).Find(&records).Error
	return records, err
}

func (bs *Backup) Get(serverId string, id uint) (*models.Backup, error) {
	var record *models.Backup
	err := bs.DB.Where(&models.Backup{ServerID: serverId, ID: id}).First(&record).Error
	return record, err
}

func (bs *Backup) Create(model *models.Backup) error {
	return bs.DB.Create(model).Error
}

func (bs *Backup) Update(model *models.Backup) error {
	return bs.DB.Omit(clause.Associations).Save(model).Error
}

func (bs *Backup) Delete(id uint) error {
	return bs.DB.Delete(&models.Backup{
		ID: id,
	}).Error
}
