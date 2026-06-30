package models

type DatabaseHostView struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         uint16 `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"-"` // No enviar la contraseña por defecto
	MaxDatabases int    `json:"max_databases"`
	NodeID       *uint  `json:"node_id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
} // @name DatabaseHostView

type DatabaseHostCreate struct {
	Name         string `json:"name" binding:"required"`
	Host         string `json:"host" binding:"required"`
	Port         uint16 `json:"port" binding:"required"`
	Username     string `json:"username" binding:"required"`
	Password     string `json:"password" binding:"required"`
	MaxDatabases int    `json:"max_databases"`
	NodeID       *uint  `json:"node_id"`
} // @name DatabaseHostCreate

type DatabaseHostUpdate struct {
	Name         string `json:"name" binding:"required"`
	Host         string `json:"host" binding:"required"`
	Port         uint16 `json:"port" binding:"required"`
	Username     string `json:"username" binding:"required"`
	Password     string `json:"password"`
	MaxDatabases int    `json:"max_databases"`
	NodeID       *uint  `json:"node_id"`
} // @name DatabaseHostUpdate
