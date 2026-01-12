package services

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/models"
	"gorm.io/gorm"
)

type Role struct {
	DB *gorm.DB
}

func (rs *Role) Get(id uint) (*models.Role, error) {
	role := &models.Role{
		ID: id,
	}
	err := rs.DB.First(role).Error
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (rs *Role) List() ([]*models.Role, error) {
	var roles []*models.Role
	err := rs.DB.Order("name ASC").Find(&roles).Error
	return roles, err
}

func (rs *Role) Create(role *models.Role) error {
	return rs.DB.Create(role).Error
}

func (rs *Role) Update(role *models.Role) error {
	return rs.DB.Save(role).Error
}

func (rs *Role) Delete(id uint) error {
	// Verificar si hay usuarios usando este rol
	var count int64
	rs.DB.Model(&models.User{}).Where("role_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("cannot delete role: users are assigned to this role")
	}
	
	return rs.DB.Delete(&models.Role{}, id).Error
}

func (rs *Role) GetByName(name string) (*models.Role, error) {
	role := &models.Role{
		Name: name,
	}
	err := rs.DB.Where(role).First(role).Error
	if err != nil {
		return nil, err
	}
	return role, nil
}

