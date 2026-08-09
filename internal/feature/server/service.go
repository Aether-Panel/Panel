package server

import (
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ServerRepo struct {
	DB *gorm.DB
}

type ServerSearch struct {
	Username string
	NodeID   uint
	NodeName string
	Name     string
	PageSize uint
	Page     uint
}

func (ss *ServerRepo) Search(searchCriteria ServerSearch) (records []*domain.Server, total int64, err error) {
	query := ss.DB

	if searchCriteria.NodeID != 0 {
		query = query.Where(&domain.Server{NodeID: searchCriteria.NodeID})
	} else if searchCriteria.NodeName != "" {
		if searchCriteria.NodeName == "LocalNode" {
			query = query.Where(&domain.Server{NodeID: 0, RawNodeID: nil})
		} else {
			query = query.Joins("JOIN nodes n ON servers.node_id = n.id AND n.name = ?", searchCriteria.NodeName)
		}
	}

	if searchCriteria.Username != "" {
		query = query.Joins("JOIN permissions p ON servers.identifier = p.server_identifier")
		query = query.Joins("JOIN users ON p.user_id = users.id")
		query = query.Where("users.username = ?", searchCriteria.Username)
	}

	nameFilter := strings.ReplaceAll(searchCriteria.Name, "*", "%")

	if nameFilter != "" && nameFilter != "%" {
		query = query.Where("name LIKE ?", nameFilter)
	}

	err = query.Model(&records).Count(&total).Error

	if err != nil {
		return nil, 0, err
	}

	err = query.Preload(clause.Associations).Offset(int((searchCriteria.Page - 1) * searchCriteria.PageSize)).Limit(int(searchCriteria.PageSize)).Order("servers.name").Find(&records).Error

	return
}

func (ss *ServerRepo) Get(id string) (*domain.Server, error) {
	if id == "" {
		return nil, gorm.ErrRecordNotFound
	}
	model := &domain.Server{
		Identifier: id,
	}

	err := ss.DB.Preload(clause.Associations).First(model).Error
	if err != nil {
		return nil, err
	}

	return model, nil
}

func (ss *ServerRepo) Update(model *domain.Server) error {
	res := ss.DB.Omit(clause.Associations).Omit("node_id").Save(model)
	return res.Error
}

// Delete a server by ID, This is _not_ ran in a transaction automatically to allow for more flexibility
// Callers should set the DB to be a transaction if needed
// (Because Gorm V2 has removed `RollbackUnlessCommitted1)
func (ss *ServerRepo) Delete(id string) error {
	model := &domain.Server{
		Identifier: id,
	}

	err := ss.DB.Delete(domain.Permissions{}, "server_identifier = ?", id).Error
	if err != nil {
		return err
	}

	err = ss.DB.Delete(domain.Client{}, "server_id = ?", id).Error
	if err != nil {
		return err
	}

	err = ss.DB.Delete(domain.Backup{}, "server_id = ?", id).Error
	if err != nil {
		return err
	}

	err = ss.DB.Delete(domain.Database{}, "server_id = ?", id).Error
	if err != nil {
		return err
	}

	err = ss.DB.Delete(domain.UptimeStatus{}, "server_id = ?", id).Error
	if err != nil {
		return err
	}

	err = ss.DB.Delete(model).Error
	if err != nil {
		return err
	}

	return nil
}

func (ss *ServerRepo) Create(model *domain.Server) error {
	if model.Identifier == "" {
		uniqueID, err := uuid.NewV4()
		if err != nil {
			return err
		}
		generatedID := strings.ToUpper(uniqueID.String())[0:8]
		model.Identifier = generatedID
	}

	res := ss.DB.Omit(clause.Associations).Create(model)
	if res.Error != nil {
		return res.Error
	}
	return nil
}
