package permission

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PermissionRepo struct {
	DB *gorm.DB
}

func (ps *PermissionRepo) GetForUser(id uint) ([]*domain.Permissions, error) {
	var allPerms []*domain.Permissions
	permissions := &domain.Permissions{
		UserID: &id,
	}

	err := ps.DB.Preload(clause.Associations).Where(permissions).Find(&allPerms).Error

	return allPerms, err
}

func (ps *PermissionRepo) GetForServer(serverID string) ([]*domain.Permissions, error) {
	var allPerms []*domain.Permissions
	permissions := &domain.Permissions{
		ServerIdentifier: &serverID,
	}

	err := ps.DB.Preload(clause.Associations).Where(permissions).Find(&allPerms).Error

	return allPerms, err
}

func (ps *PermissionRepo) GetForUserAndServer(userID uint, serverID string) (*domain.Permissions, error) {
	var id *string

	if serverID != "" {
		id = &serverID
	}

	permissions := &domain.Permissions{
		UserID:           &userID,
		ServerIdentifier: id,
	}

	query := ps.DB.Preload(clause.Associations)
	if id != nil {
		query = query.Where("user_id = ? AND server_identifier = ?", userID, *id)
	} else {
		query = query.Where("user_id = ? AND server_identifier IS NULL", userID)
	}

	err := query.First(permissions).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return permissions, nil
	}

	return permissions, err
}

func (ps *PermissionRepo) HasPermission(userID uint, serverID string, permission *scopes.Scope) (bool, error) {
	var query *gorm.DB

	if serverID != "" {
		query = ps.DB.Preload(clause.Associations).
			Where("user_id = ? AND server_identifier = ?", userID, serverID).
			Or("user_id = ? AND server_identifier IS NULL", userID)
	} else {
		query = ps.DB.Preload(clause.Associations).Where("user_id = ? AND server_identifier IS NULL", userID)
	}

	var perms []*domain.Permissions

	err := query.Find(&perms).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	for _, perm := range perms {
		if scopes.ContainsScope(perm.Scopes, permission) {
			return true, nil
		}
	}

	return false, nil
}

func (ps *PermissionRepo) GetForClient(id uint) ([]*domain.Permissions, error) {
	var allPerms []*domain.Permissions
	permissions := &domain.Permissions{
		ClientID: &id,
	}

	err := ps.DB.Preload(clause.Associations).Where(permissions).Find(&allPerms).Error

	return allPerms, err
}

func (ps *PermissionRepo) GetForClientAndServer(id uint, serverID *string) (*domain.Permissions, error) {
	permissions := &domain.Permissions{
		ClientID:         &id,
		ServerIdentifier: serverID,
	}

	err := ps.DB.Preload(clause.Associations).Where(permissions).FirstOrCreate(permissions).Error

	return permissions, err
}

func (ps *PermissionRepo) UpdatePermissions(perms *domain.Permissions) error {
	// update oauth2 with new information
	// TODO: THIS NUKES STUFF IF YOU REMOVE GLOBAL PERMS........
	/*if perms.ShouldDelete() {
		return ps.Remove(perms)
	} else {
		return ps.DB.Save(perms).Error
	}*/

	return ps.DB.Omit(clause.Associations).Save(perms).Error
}

func (ps *PermissionRepo) Remove(perms *domain.Permissions) error {
	// update oauth2 with new information

	return ps.DB.Omit(clause.Associations).Delete(perms).Error
}
