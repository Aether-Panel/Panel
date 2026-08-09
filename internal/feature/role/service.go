package role

import (
	"errors"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
	"gorm.io/gorm"
)

type RoleRepo struct {
	DB *gorm.DB
}

func (rs *RoleRepo) Get(id uint) (*domain.Role, error) {
	role := &domain.Role{
		ID: id,
	}
	err := rs.DB.First(role).Error
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (rs *RoleRepo) List() ([]*domain.Role, error) {
	var roles []*domain.Role
	err := rs.DB.Order("name ASC").Find(&roles).Error
	return roles, err
}

func (rs *RoleRepo) Create(role *domain.Role) error {
	return rs.DB.Create(role).Error
}

func (rs *RoleRepo) Update(role *domain.Role) error {
	return rs.DB.Save(role).Error
}

func (rs *RoleRepo) Delete(id uint) error {
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
		err := tx.Model(&domain.User{}).Where("role_id = ?", id).UpdateColumn("role_id", nil).Error
		if err != nil {
			logging.Error.Printf("Error decoupling users from role %d: %s", id, err.Error())
			return err
		}
		logging.Info.Printf("Users decoupled from role %d", id)

		// Delete the role
		err = tx.Delete(&domain.Role{}, id).Error
		if err != nil {
			logging.Error.Printf("Error deleting role %d from database: %s", id, err.Error())
			return err
		}
		logging.Info.Printf("Role %d deleted successfully", id)
		return nil
	})
}

func (rs *RoleRepo) GetByName(name string) (*domain.Role, error) {
	role := &domain.Role{
		Name: name,
	}
	err := rs.DB.Where(role).First(role).Error
	if err != nil {
		return nil, err
	}
	return role, nil
}
