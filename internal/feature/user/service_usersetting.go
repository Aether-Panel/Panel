package user

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserSettingRepo struct {
	DB *gorm.DB
}

func (uss *UserSettingRepo) GetAllForUser(userID uint) (domain.UserSettingsView, error) {
	var records []*domain.UserSetting

	query := uss.DB

	query = query.Where(&domain.UserSetting{UserID: userID})

	err := query.Model(&records).Error
	if err != nil {
		return nil, err
	}

	err = query.Find(&records).Error
	if err != nil {
		return nil, err
	}

	return domain.FromUserSettings(records), nil
}

func (uss *UserSettingRepo) Update(model *domain.UserSetting) error {
	search := &domain.UserSetting{
		Key:    model.Key,
		UserID: model.UserID,
	}

	err := uss.DB.Where(search).First(search).Error

	if err != nil && gorm.ErrRecordNotFound != err {
		return err
	}

	if err != nil {
		err = uss.DB.Omit(clause.Associations).Create(model).Error
	} else {
		err = uss.DB.Omit(clause.Associations).Save(model).Error
	}

	return err
}
