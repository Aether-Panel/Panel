package services

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// MySQL caps schema names at 64 characters
const mysqlMaxDatabaseNameLength = 64

// buildDatabaseName namespaces a user-requested database name under its server
// (s_<serverId>_<name>), so one tenant can never claim or drop another tenant's
// database or a system schema on a shared host.
func buildDatabaseName(serverID, requested string) (string, error) {
	if !validIdentifier.MatchString(serverID) || !validIdentifier.MatchString(requested) {
		return "", skypanel.ErrInvalidDatabaseName
	}

	name := fmt.Sprintf("s_%s_%s", serverID, requested)
	if len(name) > mysqlMaxDatabaseNameLength {
		return "", skypanel.ErrInvalidDatabaseName
	}
	return name, nil
}

// createDatabaseStatement deliberately omits IF NOT EXISTS: colliding with a schema
// the panel does not track must fail loudly instead of granting the caller
// privileges over someone else's database.
func createDatabaseStatement(name string) string {
	return fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name)
}

var (
	validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	validPassword   = regexp.MustCompile(`^[a-zA-Z0-9_!@#$%^&*]+$`)
)

type Database struct {
	DB *gorm.DB
}

func (ds *Database) GetAllForServer(serverID string) ([]*models.Database, error) {
	var databases []*models.Database

	res := ds.DB.Where("server_id = ?", serverID).Preload("DatabaseHost").Find(&databases)

	if res.Error != nil {
		return nil, res.Error
	}

	return databases, nil
}

func (ds *Database) Get(id uint) (*models.Database, error) {
	model := &models.Database{}

	res := ds.DB.Preload("DatabaseHost").First(model, id)
	return model, res.Error
}

func (ds *Database) Delete(id uint) error {
	model, err := ds.Get(id)
	if err != nil {
		return err
	}

	// Eliminar usuario y base de datos de MySQL
	err = ds.deleteFromMySQL(model)
	if err != nil {
		return err
	}

	// Eliminar de la base de datos del panel
	res := ds.DB.Delete(&models.Database{}, id)
	return res.Error
}

// UsableHosts lists the database hosts a server may create databases on: the ones serving
// its node (or every node) that still have room
func (ds *Database) UsableHosts(serverID string) ([]*models.DatabaseHost, error) {
	var hosts []*models.DatabaseHost
	if err := ds.DB.Order("name").Find(&hosts).Error; err != nil {
		return nil, err
	}
	usable := make([]*models.DatabaseHost, 0, len(hosts))
	for _, h := range hosts {
		err := ds.checkHostAccepts(h, serverID)
		if err == nil {
			usable = append(usable, h)
			continue
		}
		if !errors.Is(err, skypanel.ErrDatabaseHostFull) && !errors.Is(err, skypanel.ErrDatabaseHostOtherNode) {
			return nil, err
		}
	}
	return usable, nil
}

// checkHostAccepts applies the host's limits: MaxDatabases (0 is no limit) and, when the
// host is tied to a node, that the server runs on that node
func (ds *Database) checkHostAccepts(host *models.DatabaseHost, serverID string) error {
	if host.NodeID != nil {
		var server models.Server
		if err := ds.DB.Where("identifier = ?", serverID).First(&server).Error; err != nil {
			return err
		}
		if server.NodeID != *host.NodeID {
			return skypanel.ErrDatabaseHostOtherNode
		}
	}
	if host.MaxDatabases > 0 {
		var count int64
		if err := ds.DB.Model(&models.Database{}).Where("database_host_id = ?", host.ID).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(host.MaxDatabases) {
			return skypanel.ErrDatabaseHostFull
		}
	}
	return nil
}

// LeftoverDatabase is a MySQL database, with its user, that a server delete could not drop
// from its host: the server is deleted anyway, and these stay on the host until someone
// removes them by hand
type LeftoverDatabase struct {
	HostID   uint   `json:"hostId"`
	HostName string `json:"hostName"`
	Database string `json:"database"`
	Username string `json:"username"`
} // @name LeftoverDatabase

// DropAllForServer drops every database and user of a server from its MySQL host, before
// the server's rows go. It is best effort: a host that cannot be reached must not stop the
// server from being deleted, so what could not be dropped is logged and returned, for the
// person deleting the server to be told. err is only for the databases that could not be listed.
func (ds *Database) DropAllForServer(serverID string) ([]LeftoverDatabase, error) {
	databases, err := ds.GetAllForServer(serverID)
	if err != nil {
		return nil, err
	}
	var leftovers []LeftoverDatabase
	for _, d := range databases {
		if err := ds.deleteFromMySQL(d); err != nil {
			left := LeftoverDatabase{HostID: d.DatabaseHostID, Database: d.DatabaseName, Username: d.Username}
			if d.DatabaseHost != nil {
				left.HostName = d.DatabaseHost.Name
			}
			logging.Error.Printf("server %s: database %s and its user %s could not be dropped from host %d (%s), remove them by hand: %s", serverID, d.DatabaseName, d.Username, d.DatabaseHostID, left.HostName, err)
			leftovers = append(leftovers, left)
		}
	}
	return leftovers, nil
}

// mysqlAccessDenied are the MySQL error numbers that mean the host refused the panel's
// account or one of its privileges
var mysqlAccessDenied = map[uint16]bool{
	1044: true, // ER_DBACCESS_DENIED_ERROR
	1045: true, // ER_ACCESS_DENIED_ERROR
	1142: true, // ER_TABLEACCESS_DENIED_ERROR
	1227: true, // ER_SPECIFIC_ACCESS_DENIED_ERROR
	1370: true, // ER_PROCACCESS_DENIED_ERROR
}

func isMySQLAccessDenied(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && mysqlAccessDenied[me.Number]
}

// mysqlStepError says which step on a MySQL host failed, with the driver's error. Only an
// access-denied answer from MySQL mentions the privilege the host account needs: a host that
// is down or unreachable must not read as a permissions problem.
func mysqlStepError(step, privilege string, err error) error {
	if isMySQLAccessDenied(err) {
		return fmt.Errorf("%s: MySQL denied it, the database host's account needs the %s privilege: %w", step, privilege, err)
	}
	return fmt.Errorf("%s: %w", step, err)
}

// ensureDatabaseNameAvailable refuses names already tracked on that host, no matter
// which server owns them.
func (ds *Database) ensureDatabaseNameAvailable(hostID uint, name string) error {
	var count int64
	err := ds.DB.Model(&models.Database{}).
		Where("database_host_id = ? AND database_name = ?", hostID, name).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return skypanel.ErrDatabaseNameTaken
	}
	return nil
}

func (ds *Database) Create(database *models.Database) error {
	// Obtener el database host
	host := &models.DatabaseHost{}
	err := ds.DB.First(host, database.DatabaseHostID).Error
	if err != nil {
		return err
	}
	if err = ds.checkHostAccepts(host, database.ServerID); err != nil {
		return err
	}

	// El nombre físico queda bajo el namespace del servidor y debe ser único en el host
	name, err := buildDatabaseName(database.ServerID, database.DatabaseName)
	if err != nil {
		return err
	}
	database.DatabaseName = name

	if err = ds.ensureDatabaseNameAvailable(database.DatabaseHostID, name); err != nil {
		return err
	}

	// Generar usuario y contraseña aleatorios
	database.Username = generateRandomUsername()
	database.Password = generateRandomPassword()
	database.RemoteConnection = fmt.Sprintf("%s:%d", host.Host, host.Port)

	// Crear en MySQL
	err = ds.createInMySQL(database, host)
	if err != nil {
		return err
	}

	// Guardar en la base de datos del panel
	res := ds.DB.Create(database)
	return res.Error
}

func (ds *Database) createInMySQL(database *models.Database, host *models.DatabaseHost) error {
	if !validIdentifier.MatchString(database.DatabaseName) || !validIdentifier.MatchString(database.Username) || !validPassword.MatchString(database.Password) {
		return fmt.Errorf("invalid characters in database credentials")
	}

	databaseName := sanitizeIdentifier(database.DatabaseName)

	// Conectar a MySQL usando Config para escapar caracteres especiales
	cfg := mysql.NewConfig()
	cfg.User = host.Username
	cfg.Passwd = host.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", host.Host, host.Port)
	cfg.ParseTime = true
	cfg.InterpolateParams = true
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 5 * time.Second
	cfg.WriteTimeout = 5 * time.Second

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %w", err)
	}
	defer db.Close()

	// Verificar conexión
	err = db.Ping()
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL host (check credentials and host permissions): %w", err)
	}

	// Crear la base de datos (sin IF NOT EXISTS: chocar con un schema existente debe fallar)
	_, err = db.Exec(createDatabaseStatement(databaseName)) /* #nosec G201 */ // NOSONAR
	if err != nil {
		return fmt.Errorf("failed to create database (ensure user has CREATE permissions and the name is free): %w", err)
	}

	// From here a failure undoes what was created: a schema left behind with no panel row
	// would make every retry with the same name fail at CREATE DATABASE.
	created := false
	defer func() {
		if created {
			return
		}
		_, _ = db.Exec(fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", database.Username)) /* #nosec G201 */ // NOSONAR
		_, _ = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", databaseName))       /* #nosec G201 */ // NOSONAR
	}()

	// Crear el usuario (permitir conexión desde cualquier host)
	_, err = db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", database.Username, database.Password)) /* #nosec G201 */ // NOSONAR
	if err != nil {
		return fmt.Errorf("failed to create MySQL user (ensure user has CREATE USER permissions): %w", err)
	}

	// Otorgar permisos al usuario sobre la base de datos
	_, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'", databaseName, database.Username)) /* #nosec G201 */ // NOSONAR
	if err != nil {
		return fmt.Errorf("failed to grant privileges (ensure user has GRANT OPTION): %w", err)
	}

	// Aplicar cambios
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return fmt.Errorf("failed to flush privileges: %w", err)
	}

	created = true
	return nil
}

func (ds *Database) deleteFromMySQL(database *models.Database) error {
	if !validIdentifier.MatchString(database.DatabaseName) || !validIdentifier.MatchString(database.Username) {
		return fmt.Errorf("invalid characters in database credentials")
	}

	databaseName := sanitizeIdentifier(database.DatabaseName)

	// Obtener el database host
	host := &models.DatabaseHost{}
	err := ds.DB.First(host, database.DatabaseHostID).Error
	if err != nil {
		return err
	}

	// Conectar a MySQL usando Config
	cfg := mysql.NewConfig()
	cfg.User = host.Username
	cfg.Passwd = host.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", host.Host, host.Port)
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 5 * time.Second
	cfg.WriteTimeout = 5 * time.Second

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("invalid MySQL host settings: %w", err)
	}
	defer db.Close()

	// connect first, so a host that is down says so instead of failing the first DROP
	if err = db.Ping(); err != nil {
		if isMySQLAccessDenied(err) {
			return fmt.Errorf("could not connect to the MySQL host: it refused the database host's username or password: %w", err)
		}
		return fmt.Errorf("could not connect to the MySQL host: %w", err)
	}

	_, err = db.Exec(fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", database.Username)) /* #nosec G201 */ // NOSONAR
	if err != nil {
		return mysqlStepError("failed to drop user "+database.Username, "CREATE USER", err)
	}

	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", databaseName)) /* #nosec G201 */ // NOSONAR
	if err != nil {
		return mysqlStepError("failed to drop database "+databaseName, "DROP", err)
	}

	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return mysqlStepError("failed to flush privileges", "RELOAD", err)
	}

	return nil
}

func sanitizeIdentifier(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func generateRandomUsername() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 10)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return "db_" + string(b)
}

func generateRandomPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, 16)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
