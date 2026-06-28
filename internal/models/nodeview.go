package models

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gopkg.in/go-playground/validator.v9"
	"net/url"
)

type NodeView struct {
	ID          uint   `json:"id"`
	Name        string `json:"name,omitempty"`
	PublicHost  string `json:"publicHost,omitempty"`
	PrivateHost string `json:"privateHost,omitempty"`
	PublicPort  uint16 `json:"publicPort,omitempty"`
	PrivatePort uint16 `json:"privatePort,omitempty"`
	SFTPPort    uint16 `json:"sftpPort,omitempty"`
	Local       bool   `json:"isLocal"`
} //@name Node

type NodesView []*NodeView //@name Nodes

func FromNode(n *Node) *NodeView {
	return &NodeView{
		ID:          n.ID,
		Name:        n.Name,
		PublicHost:  n.PublicHost,
		PrivateHost: n.PrivateHost,
		PublicPort:  n.PublicPort,
		PrivatePort: n.PrivatePort,
		SFTPPort:    n.SFTPPort,
		Local:       n.IsLocal(),
	}
}

func FromNodes(n []*Node) *NodesView {
	result := make(NodesView, len(n))

	for k, v := range n {
		result[k] = FromNode(v)
	}

	return &result
}

func (n *NodeView) CopyToModel(newModel *Node) {
	if n.Name != "" {
		newModel.Name = n.Name
	}

	if n.PublicHost != "" {
		newModel.PublicHost = n.PublicHost
	}

	if n.PrivateHost != "" {
		newModel.PrivateHost = n.PrivateHost
	}

	if n.PublicPort > 0 {
		newModel.PublicPort = n.PublicPort
	}

	if n.PrivatePort > 0 {
		newModel.PrivatePort = n.PrivatePort
	}

	if n.SFTPPort > 0 {
		newModel.SFTPPort = n.SFTPPort
	}
}

func (n *NodeView) Valid(allowEmpty bool) error {
	validate := validator.New()

	if !allowEmpty && validate.Var(n.Name, "required") != nil {
		return skypanel.ErrFieldRequired("name")
	}

	if validate.Var(n.Name, "omitempty,printascii") != nil {
		return skypanel.ErrFieldMustBePrintable("name")
	}

	testName := url.QueryEscape(n.Name)
	if testName != n.Name {
		return skypanel.ErrFieldHasURICharacters("name")
	}

	if !allowEmpty && validate.Var(n.PublicHost, "required") != nil {
		return skypanel.ErrFieldMustBePrintable("publicHost")
	}

	if validate.Var(n.PublicHost, "omitempty,ip|fqdn") != nil {
		return skypanel.ErrFieldIsInvalidHost("publicHost")
	}

	if !allowEmpty && validate.Var(n.PrivateHost, "required") != nil {
		return skypanel.ErrFieldMustBePrintable("privateHost")
	}

	if validate.Var(n.PrivateHost, "omitempty,ip_addr|fqdn") != nil {
		return skypanel.ErrFieldIsInvalidHost("privateHost")
	}

	if allowEmpty {
		if validate.Var(n.PublicPort, "min=0,max=65535") != nil {
			return skypanel.ErrFieldTooLarge("publicPort", 65535)
		}

		if validate.Var(n.PrivatePort, "min=0,max=65535") != nil {
			return skypanel.ErrFieldTooLarge("privatePort", 65535)
		}

		if validate.Var(n.SFTPPort, "min=0,max=65535") != nil {
			return skypanel.ErrFieldTooLarge("sftpPort", 65535)
		}
	} else {
		if validate.Var(n.PublicPort, "min=1,max=65535") != nil {
			return skypanel.ErrFieldNotBetween("publicPort", 1, 65535)
		}

		if validate.Var(n.PrivatePort, "min=1,max=65535") != nil {
			return skypanel.ErrFieldNotBetween("privatePort", 1, 65535)
		}

		if validate.Var(n.SFTPPort, "min=1,max=65535") != nil {
			return skypanel.ErrFieldNotBetween("sftpPort", 1, 65535)
		}
	}

	if n.SFTPPort != 0 && n.SFTPPort == n.PublicPort {
		return skypanel.ErrFieldEqual("sftpPort", "publicPort")
	}

	if n.SFTPPort != 0 && n.SFTPPort == n.PrivatePort {
		return skypanel.ErrFieldEqual("sftpPort", "privatePort")
	}

	return nil
}
