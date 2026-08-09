package backup

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BackupRepo struct {
	DB *gorm.DB
}

func (bs *BackupRepo) GetAllForServer(serverID string) ([]*domain.Backup, error) {
	var records []*domain.Backup
	err := bs.DB.Where(&domain.Backup{ServerID: serverID}).Find(&records).Error
	return records, err
}

func (bs *BackupRepo) Get(serverID string, id uint) (*domain.Backup, error) {
	var record *domain.Backup
	err := bs.DB.Where(&domain.Backup{ServerID: serverID, ID: id}).First(&record).Error
	return record, err
}

func (bs *BackupRepo) Create(model *domain.Backup) error {
	return bs.DB.Create(model).Error
}

func (bs *BackupRepo) Update(model *domain.Backup) error {
	return bs.DB.Omit(clause.Associations).Save(model).Error
}

func (bs *BackupRepo) Delete(id uint) error {
	return bs.DB.Delete(&domain.Backup{
		ID: id,
	}).Error
}
