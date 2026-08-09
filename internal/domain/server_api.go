package domain

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type ServerCreation struct {
	skypanel.Server

	NodeID uint     `json:"node"`
	Users  []string `json:"users"`
	Name   string   `json:"name"`

	// Splitter Fields
	ParentServerID *string `json:"parent_server_id,omitempty"`
	TotalCPU       int     `json:"total_cpu,omitempty"`
	TotalMemory    int64   `json:"total_memory,omitempty"`
	TotalDisk      int64   `json:"total_disk,omitempty"`
} // @name CreatedServer

type GetServerResponse struct {
	Server *ServerView     `json:"server"`
	Perms  *PermissionView `json:"permissions"`
} // @name GetServer

type CreateServerResponse struct {
	ID string `json:"id"`
} // @name CreatedServerId

type ServerSearchResponse struct {
	Servers []*ServerView `json:"servers"`
	*skypanel.Metadata
} // @name ServerSearchResults

type ServerWithName struct {
	skypanel.Server
	Name string `json:"name"`
} // @name NamedServer
