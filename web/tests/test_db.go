package main

import (
	"fmt"
	"log"

	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/SkyPanel/SkyPanel/v3/database"
	"github.com/SkyPanel/SkyPanel/v3/models"
)

func main() {
	// 1. Cargar la configuración real
	err := config.LoadConfigFile("config.json")
	if err != nil {
		log.Fatalf("Error cargando config: %v", err)
	}

	fmt.Printf("Usando Dialecto: %s\n", config.DatabaseDialect.Value())
	fmt.Printf("Usando URL: %s\n", config.DatabaseUrl.Value())

	// 2. Usar la conexión oficial del proyecto
	db, err := database.GetConnection()
	if err != nil {
		log.Fatalf("Error conectando a MariaDB: %v", err)
	}

	// 3. Listar servidores
	var servers []models.Server
	err = db.Find(&servers).Error
	if err != nil {
		log.Fatalf("Error consultando tabla servers: %v", err)
	}

	fmt.Println("\n--- SERVIDORES EN TU BASE DE DATOS MARIADB ---")
	if len(servers) == 0 {
		fmt.Println("No se encontraron servidores en la base de datos.")
	}
	for _, s := range servers {
		fmt.Printf("- ID: %s | Nombre: %s\n", s.Identifier, s.Name)
	}
}
