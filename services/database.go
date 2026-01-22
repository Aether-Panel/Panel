package services

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/SkyPanel/SkyPanel/v3/models"
	"gorm.io/gorm"
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

func (ds *Database) Create(database *models.Database) error {
	// Obtener el database host
	host := &models.DatabaseHost{}
	err := ds.DB.First(host, database.DatabaseHostID).Error
	if err != nil {
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
	// Conectar a MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", host.Username, host.Password, host.Host, host.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	defer db.Close()

	// Crear la base de datos
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", database.DatabaseName))
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// Crear el usuario (permitir conexión desde cualquier host)
	_, err = db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", database.Username, database.Password))
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Otorgar permisos al usuario sobre la base de datos
	_, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'", database.DatabaseName, database.Username))
	if err != nil {
		return fmt.Errorf("failed to grant privileges: %w", err)
	}

	// Aplicar cambios
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return fmt.Errorf("failed to flush privileges: %w", err)
	}

	return nil
}

func (ds *Database) deleteFromMySQL(database *models.Database) error {
	// Obtener el database host
	host := &models.DatabaseHost{}
	err := ds.DB.First(host, database.DatabaseHostID).Error
	if err != nil {
		return err
	}

	// Conectar a MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", host.Username, host.Password, host.Host, host.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	defer db.Close()

	// Eliminar el usuario
	_, err = db.Exec(fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", database.Username))
	if err != nil {
		return fmt.Errorf("failed to drop user: %w", err)
	}

	// Eliminar la base de datos
	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", database.DatabaseName))
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	// Aplicar cambios
	_, err = db.Exec("FLUSH PRIVILEGES")
	if err != nil {
		return fmt.Errorf("failed to flush privileges: %w", err)
	}

	return nil
}

func generateRandomUsername() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 10)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return "db_" + string(b)
}

func generateRandomPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
