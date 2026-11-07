package services

import (
	"errors"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// serverIDQuery picks the rows that belong to a server
const serverIDQuery = "server_id = ?"

type Server struct {
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

func (ss *Server) Search(searchCriteria ServerSearch) (records []*models.Server, total int64, err error) {
	query := ss.DB

	// explicit conditions: NodeID is not a column (gorm:"-") and a nil RawNodeID is a zero
	// value, so as struct conditions both were dropped and every server came back
	if searchCriteria.NodeID != 0 {
		query = query.Where("servers.node_id = ?", searchCriteria.NodeID)
	} else if searchCriteria.NodeName != "" {
		if searchCriteria.NodeName == "LocalNode" {
			query = query.Where("servers.node_id IS NULL")
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

func (ss *Server) Get(id string) (*models.Server, error) {
	if id == "" {
		return nil, gorm.ErrRecordNotFound
	}
	model := &models.Server{
		Identifier: id,
	}

	err := ss.DB.Preload(clause.Associations).First(model).Error
	if err != nil {
		return nil, err
	}

	return model, nil
}

// GetByInternalID looks a server up by its immutable internal UUIDv7 identity.
// It allows callers to reference a server independently of its user-facing
// Identifier, which is useful for future migrations and integrations.
func (ss *Server) GetByInternalID(internalID string) (*models.Server, error) {
	if internalID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	model := &models.Server{}
	err := ss.DB.Preload(clause.Associations).Where("internal_id = ?", internalID).First(model).Error
	if err != nil {
		return nil, err
	}

	return model, nil
}

func (ss *Server) Update(model *models.Server) error {
	res := ss.DB.Omit(clause.Associations).Omit("node_id").Save(model)
	return res.Error
}

// Delete a server by ID: its MySQL databases on their hosts, then every row that belongs to
// it, in one transaction so a refused step (a foreign key on MySQL) leaves nothing half
// deleted. When ss.DB is already a transaction, the row deletes run in a savepoint of it.
func (ss *Server) Delete(id string) error {
	// drop the MySQL databases first, while their rows still say where they are: their rows
	// going away left the schemas and users (with working passwords) on the host, and a new
	// server with this id failing to create them. An error here deletes nothing. Outside the
	// transaction: it talks to the hosts, which can take seconds each.
	if err := (&Database{DB: ss.DB}).DropAllForServer(id); err != nil {
		return err
	}

	return ss.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(models.Permissions{}, "server_identifier = ?", id).Error; err != nil {
			return err
		}
		if err := DeleteServerClients(tx, id); err != nil {
			return err
		}
		for _, owned := range []interface{}{models.Backup{}, models.Database{}, models.UptimeStatus{}} {
			if err := tx.Delete(owned, serverIDQuery, id).Error; err != nil {
				return err
			}
		}
		// sessions scoped to this server, so no orphaned rows survive it
		if err := tx.Delete(models.Session{}, "server_identifier = ?", id).Error; err != nil {
			return err
		}
		if err := DeleteServerTransfers(tx, id); err != nil {
			return err
		}
		return tx.Delete(&models.Server{Identifier: id}).Error
	})
}

// DeleteServerClients removes the server's OAuth clients with their permissions and the
// sessions made for them. Those go first: CreateForClient gives the sessions no
// server_identifier, so nothing else finds them, and where foreign keys are on both would
// block the clients' delete.
func DeleteServerClients(db *gorm.DB, serverID string) error {
	clientIDs := func() *gorm.DB {
		return db.Model(&models.Client{}).Select("id").Where(serverIDQuery, serverID)
	}
	if err := db.Where("client_id IN (?)", clientIDs()).Delete(&models.Permissions{}).Error; err != nil {
		return err
	}
	if err := db.Where("client_id IN (?)", clientIDs()).Delete(&models.Session{}).Error; err != nil {
		return err
	}
	return db.Delete(models.Client{}, serverIDQuery, serverID).Error
}

// DeleteServerTransfers removes the server's federated transfer sessions and their logs
func DeleteServerTransfers(db *gorm.DB, serverID string) error {
	return deleteTransfers(db, serverIDQuery, serverID)
}

// deleteTransfers removes the federated transfer sessions matching query and their logs.
// Logs go first: they point at the session, and SQLite runs without the cascade.
func deleteTransfers(db *gorm.DB, query string, arg interface{}) error {
	sessions := db.Model(&models.ExTransferSession{}).Select("session_uuid").Where(query, arg)
	if err := db.Where("session_id IN (?)", sessions).Delete(&models.ExTransferLog{}).Error; err != nil {
		return err
	}
	return db.Where(query, arg).Delete(&models.ExTransferSession{}).Error
}

func (ss *Server) Create(model *models.Server) error {
	if model.Identifier == "" {
		// Generate a unique identifier, retrying on the (now extremely unlikely)
		// event of a collision instead of returning a raw primary-key error.
		const maxAttempts = 5
		for attempt := 0; attempt < maxAttempts; attempt++ {
			generated, err := models.GenerateServerIdentifier()
			if err != nil {
				return err
			}

			var count int64
			if err := ss.DB.Model(&models.Server{}).Where("identifier = ?", generated).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				model.Identifier = generated
				break
			}
		}

		if model.Identifier == "" {
			return errors.New("could not generate a unique server identifier")
		}
	}

	if model.InternalID == "" {
		internalID, err := models.GenerateInternalID()
		if err != nil {
			return err
		}
		model.InternalID = internalID
	}

	res := ss.DB.Omit(clause.Associations).Create(model)
	if res.Error != nil {
		return res.Error
	}
	return nil
}
