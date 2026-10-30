package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Why a port cannot be used
const (
	portBusyRange     = "range"     // below 1024
	portBusyPanel     = "panel"     // the panel itself: web, SFTP, its database, Adminer
	portBusyServer    = "server"    // another server on the node
	portBusyContainer = "container" // another container running on the node
)

type portCheckRequest struct {
	// Ports maps each port variable of the form to the port it asks for
	Ports map[string]uint16 `json:"ports"`
	// Exclude is the server being edited: its own ports are not taken
	Exclude string `json:"exclude"`
}

type portCheckResult struct {
	Busy   bool   `json:"busy"`
	Reason string `json:"reason,omitempty"`
	// By names the server or container that holds the port
	By string `json:"by,omitempty"`
	// Suggestion is the next free port after the one asked for
	Suggestion uint16 `json:"suggestion,omitempty"`
}

type portHolder struct {
	reason, by string
}

// checkNodePorts tells the new-server wizard, before it creates anything, which of its ports
// are already taken on the node and by whom, with a free one to use instead. A port taken only
// shows when the server starts otherwise, as a conflict nobody saw coming.
func checkNodePorts(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}

	id, ok := validateID(c)
	if !ok {
		return
	}
	node, err := ns.Get(id)
	if response.HandleError(c, err, http.StatusNotFound) {
		return
	}

	var req portCheckRequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	held, err := portsHeldOnNode(db, ns, node, req.Exclude)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	asked := make(map[uint16]bool, len(req.Ports))
	for _, p := range req.Ports {
		asked[p] = true
	}

	results := make(map[string]portCheckResult, len(req.Ports))
	for variable, p := range req.Ports {
		if p == 0 {
			continue
		}
		result := portCheckResult{}
		if p < 1024 {
			result = portCheckResult{Busy: true, Reason: portBusyRange}
		} else if h, taken := held[p]; taken {
			result = portCheckResult{Busy: true, Reason: h.reason, By: h.by}
		}
		if result.Busy {
			result.Suggestion = nextPortFree(max(p, 1023), held, asked)
		}
		results[variable] = result
	}
	c.JSON(http.StatusOK, gin.H{"ports": results})
}

// portsHeldOnNode collects every port that is not free on the node: the node's own, the ones
// the panel gave to its servers, and the ones published by containers running there.
func portsHeldOnNode(db *gorm.DB, ns *services.Node, node *models.Node, exclude string) (map[uint16]portHolder, error) {
	held := make(map[uint16]portHolder)
	for _, p := range []uint16{node.PublicPort, node.PrivatePort, node.SFTPPort} {
		if p != 0 {
			held[p] = portHolder{reason: portBusyPanel}
		}
	}

	// the panel's own node keeps its servers without a node id
	query := db.Model(&models.Server{})
	if node.ID == 0 {
		query = query.Where("node_id IS NULL")
	} else {
		query = query.Where("node_id = ?", node.ID)
	}
	var onNode []*models.Server
	if err := query.Find(&onNode).Error; err != nil {
		return nil, err
	}
	names := make(map[string]string, len(onNode))
	for _, s := range onNode {
		names[s.Identifier] = s.Name
		if s.Identifier == exclude {
			continue
		}
		for _, p := range append([]uint16{s.Port}, s.Ports...) {
			if p != 0 {
				held[p] = portHolder{reason: portBusyServer, by: s.Name}
			}
		}
	}

	for p, container := range publishedPortsOnNode(ns, node) {
		if container == exclude {
			continue
		}
		if _, known := held[p]; known {
			continue
		}
		switch {
		case names[container] != "":
			held[p] = portHolder{reason: portBusyServer, by: names[container]}
		case strings.HasPrefix(container, "skypanel"):
			held[p] = portHolder{reason: portBusyPanel, by: container}
		default:
			held[p] = portHolder{reason: portBusyContainer, by: container}
		}
	}
	return held, nil
}

// publishedPortsOnNode asks the node which host ports its running containers publish; a node
// that cannot answer (an older one, or one without Docker) leaves the check to the panel's records.
func publishedPortsOnNode(ns *services.Node, node *models.Node) map[uint16]string {
	res, err := ns.CallNode(node, http.MethodGet, "/daemon/ports", nil, nil)
	defer utils.CloseResponse(res)
	if err != nil || res.StatusCode != http.StatusOK {
		return nil
	}
	var body struct {
		Ports map[uint16]string `json:"ports"`
	}
	if json.NewDecoder(res.Body).Decode(&body) != nil {
		return nil
	}
	return body.Ports
}

// nextPortFree is the first port after from that nobody holds and the form does not ask for
func nextPortFree(from uint16, held map[uint16]portHolder, asked map[uint16]bool) uint16 {
	for p := int(from) + 1; p <= 65535; p++ {
		if _, taken := held[uint16(p)]; !taken && !asked[uint16(p)] {
			return uint16(p)
		}
	}
	return 0
}
