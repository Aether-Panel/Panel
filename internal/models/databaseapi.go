package models

type DatabaseView struct {
	ID               uint   `json:"id"`
	ServerID         string `json:"server_id"`
	DatabaseHostID   uint   `json:"database_host_id"`
	DatabaseName     string `json:"database_name"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	RemoteConnection string `json:"remote_connection"`
	MaxConnections   int    `json:"max_connections"`
	Host             string `json:"host"`
	Port             uint16 `json:"port"`
	HostName         string `json:"host_name"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
} // @name DatabaseView

type DatabaseCreate struct {
	DatabaseHostID uint   `json:"database_host_id" binding:"required"`
	DatabaseName   string `json:"database_name" binding:"required"`
} // @name DatabaseCreate
