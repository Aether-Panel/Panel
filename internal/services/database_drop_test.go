package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/clause"
)

// A host where nothing listens: the drop fails at once, the server delete goes on, and the
// database is returned to be removed by hand
func TestDropAllForServer_AnUnreachableHostIsReported(t *testing.T) {
	db := setupDeleteCleanupDB(t, &models.DatabaseHost{}, &models.Database{})
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: "drop00001", Name: "drop", Type: "generic"}).Error)
	host := &models.DatabaseHost{Name: "paused-mariadb", Host: "127.0.0.1", Port: 1, Username: "root", Password: "unused"}
	require.NoError(t, db.Create(host).Error)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Database{
		ServerID: "drop00001", DatabaseHostID: host.ID, DatabaseName: "s_drop00001_world", Username: "u_world", Password: "unused",
	}).Error)

	leftovers, err := (&Database{DB: db}).DropAllForServer("drop00001")

	require.NoError(t, err, "a host that is down must not stop the delete")
	assert.Equal(t, []LeftoverDatabase{{HostID: host.ID, HostName: "paused-mariadb", Database: "s_drop00001_world", Username: "u_world"}}, leftovers)

	// the reason says the connection failed, not that a privilege is missing
	dropErr := (&Database{DB: db}).deleteFromMySQL(&models.Database{DatabaseHostID: host.ID, DatabaseName: "s_drop00001_world", Username: "u_world"})
	require.Error(t, dropErr)
	assert.Contains(t, dropErr.Error(), "could not connect to the MySQL host")
	assert.NotContains(t, strings.ToLower(dropErr.Error()), "permission")
	assert.NotContains(t, strings.ToLower(dropErr.Error()), "privilege")
}

func TestDropAllForServer_NothingToDrop(t *testing.T) {
	db := setupDeleteCleanupDB(t, &models.DatabaseHost{}, &models.Database{})

	leftovers, err := (&Database{DB: db}).DropAllForServer("none00001")

	require.NoError(t, err)
	assert.Empty(t, leftovers)
}

// Only MySQL saying access was denied mentions the privilege the host account needs
func TestMySQLStepError(t *testing.T) {
	denied := mysqlStepError("failed to drop user u_world", "CREATE USER", &mysql.MySQLError{Number: 1227, Message: "Access denied; you need (at least one of) the CREATE USER privilege(s)"})
	assert.Contains(t, denied.Error(), "needs the CREATE USER privilege")
	var me *mysql.MySQLError
	assert.True(t, errors.As(denied, &me), "the driver's error is kept")

	gone := mysqlStepError("failed to drop user u_world", "CREATE USER", mysql.ErrInvalidConn)
	assert.Equal(t, "failed to drop user u_world: invalid connection", gone.Error())

	other := mysqlStepError("failed to drop database s_x", "DROP", &mysql.MySQLError{Number: 1008, Message: "Can't drop database"})
	assert.NotContains(t, other.Error(), "privilege")
}

// Creating on a host where nothing listens says the connection failed, not that a privilege
// is missing
func TestCreateInMySQL_AnUnreachableHostIsNotAPermissionProblem(t *testing.T) {
	host := &models.DatabaseHost{Name: "paused-mariadb", Host: "127.0.0.1", Port: 1, Username: "root", Password: "unused"}

	err := (&Database{}).createInMySQL(&models.Database{DatabaseName: "s_new00001_world", Username: "u_world", Password: "Secret1"}, host)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not connect to the MySQL host")
	assert.NotContains(t, strings.ToLower(err.Error()), "permission")
	assert.NotContains(t, strings.ToLower(err.Error()), "privilege")
}
