package services

import (
	"errors"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
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
	// No permitir borrar los roles predeterminados del sistema
	role, err := rs.Get(id)
	if err != nil {
		return err
	}
	if role.Name == "admin" {
		return errors.New("cannot delete the admin role")
	}
	if role.Name == "Administrador" || role.Name == "Usuario" {
		return errors.New("cannot delete a default role")
	}

	return rs.DB.Transaction(func(tx *gorm.DB) error {
		logging.Info.Printf("Attempting to delete role %d (%s)", id, role.Name)

		// Decouple users from this role
		err := tx.Model(&models.User{}).Where("role_id = ?", id).UpdateColumn("role_id", nil).Error
		if err != nil {
			logging.Error.Printf("Error decoupling users from role %d: %s", id, err.Error())
			return err
		}
		logging.Info.Printf("Users decoupled from role %d", id)

		// Delete the role
		err = tx.Delete(&models.Role{}, id).Error
		if err != nil {
			logging.Error.Printf("Error deleting role %d from database: %s", id, err.Error())
			return err
		}
		logging.Info.Printf("Role %d deleted successfully", id)
		return nil
	})
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

