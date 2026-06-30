package models

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gopkg.in/go-playground/validator.v9"
)

type ServerView struct {
	Identifier     string           `json:"id,omitempty"`
	Name           string           `json:"name,omitempty"`
	NodeID         uint             `json:"nodeId"`
	Node           *NodeView        `json:"node,omitempty"`
	Data           interface{}      `json:"data,omitempty"`
	Users          []ServerUserView `json:"users,omitempty"`
	IP             string           `json:"ip,omitempty"`
	Port           uint16           `json:"port,omitempty"`
	Type           string           `json:"type"`
	Icon           string           `json:"icon,omitempty"`
	CanGetStatus   bool             `json:"canGetStatus,omitempty"`
	IsGhost        bool             `json:"isGhost,omitempty"`
	ParentServerID *string          `json:"parent_server_id,omitempty"`
	TotalCPU       int              `json:"total_cpu,omitempty"`
	TotalMemory    int64            `json:"total_memory,omitempty"`
	TotalDisk      int64            `json:"total_disk,omitempty"`
	Suspended      bool             `json:"suspended"`
} // @name ServerInfo

type ServerUserView struct {
	Username string   `json:"username"`
	Scopes   []string `json:"scopes"`
} // @name ServerUser

func FromServer(server *Server) *ServerView {
	model := &ServerView{
		Name:           server.Name,
		Identifier:     server.Identifier,
		NodeID:         server.NodeID,
		IP:             server.IP,
		Port:           server.Port,
		Type:           server.Type,
		Icon:           server.Icon,
		Node:           FromNode(&server.Node),
		ParentServerID: server.ParentServerID,
		TotalCPU:       server.TotalCPU,
		TotalMemory:    server.TotalMemory,
		TotalDisk:      server.TotalDisk,
		Suspended:      server.Suspended,
	}

	return model
}

func FromServers(servers []*Server) []*ServerView {
	result := make([]*ServerView, len(servers))

	for k, v := range servers {
		result[k] = FromServer(v)
	}

	return result
}

func (s *ServerView) Valid(allowEmpty bool) error {
	validate := validator.New()

	if !allowEmpty && validate.Var(s.Name, "required") != nil {
		return skypanel.ErrFieldRequired("name")
	}

	if !allowEmpty && validate.Var(s.Type, "required") != nil {
		return skypanel.ErrFieldRequired("type")
	}

	if validate.Var(s.Name, "omitempty,printascii") != nil {
		return skypanel.ErrFieldMustBePrintable("name")
	}

	if !allowEmpty && validate.Var(s.NodeID, "required,min:1") != nil {
		return skypanel.ErrFieldTooSmall("node", 1)
	}

	if validate.Var(s.IP, "omitempty,ip|fqdn") != nil {
		return skypanel.ErrFieldIsInvalidIP("ip")
	}

	return nil
}

func RemoveServerPrivateInfoFromAll(servers []*ServerView) []*ServerView {
	for k, v := range servers {
		servers[k] = RemoveServerPrivateInfo(v)
	}
	return servers
}

func RemoveServerPrivateInfo(server *ServerView) *ServerView {
	// SCRUB DATA FROM REGULAR USERS
	if server.Node != nil {
		server.Node.PrivateHost = ""
		server.Node.PrivatePort = 0
	}

	return server
}
