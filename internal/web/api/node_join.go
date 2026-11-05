package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Joining a node with one command. An administrator asks for a join code for a node the panel
// already knows (Nodes → node → Deploy node) and pastes the command it comes with on the new
// machine; the installer trades the code for what the daemon needs to talk to the panel (its
// client id and secret, its ports) and sets the machine up as that node. The code works once
// and for an hour, and only its hash is kept, in memory: a restart of the panel drops it.

const (
	nodeJoinCodeLife = time.Hour
	// nodeJoinInstaller is the installer the command runs: the repository's own, on the stable
	// branch, so a node gets the same installer and version as a panel installed today
	nodeJoinInstaller = "https://raw.githubusercontent.com/Aether-Panel/Panel/main/tools/install/install.sh"
	// NodeJoinPath is where the installer trades a join code; it is public, the code is the key
	NodeJoinPath = "/node-join"
)

var (
	errNodeJoinCode  = skypanel.CreateError("this join code does not exist, was already used or has expired: generate a new one in the panel", "ErrNodeJoinCode")
	errNodeJoinLocal = skypanel.CreateError("the panel's own node is installed with the panel and cannot be joined", "ErrNodeJoinLocal")
	errNodeJoinURL   = skypanel.CreateError("set the panel's address in Settings first: the new node reaches the panel there", "ErrNodeJoinURL")
)

// nodeJoinCodes are the pending join codes, by the hash of the code
type nodeJoinCodes struct {
	lock    sync.Mutex
	pending map[string]pendingNodeJoin
	now     func() time.Time
}

type pendingNodeJoin struct {
	nodeID  uint
	expires time.Time
}

var joinCodes = newNodeJoinCodes()

// nodeJoinAttempts caps how often one address may try a code, so codes cannot be guessed
var nodeJoinAttempts = services.NewAttemptLimiter(10, 15*time.Minute)

func newNodeJoinCodes() *nodeJoinCodes {
	return &nodeJoinCodes{pending: map[string]pendingNodeJoin{}, now: time.Now}
}

func hashJoinCode(code string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

// issue makes a new code for the node; an earlier code for it stops working
func (j *nodeJoinCodes) issue(nodeID uint) (string, time.Time, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	code := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))

	j.lock.Lock()
	defer j.lock.Unlock()
	now := j.now()
	for hash, p := range j.pending {
		if p.nodeID == nodeID || now.After(p.expires) {
			delete(j.pending, hash)
		}
	}
	expires := now.Add(nodeJoinCodeLife)
	j.pending[hashJoinCode(code)] = pendingNodeJoin{nodeID: nodeID, expires: expires}
	return code, expires, nil
}

// redeem uses a code up: it gives the node it was for, once
func (j *nodeJoinCodes) redeem(code string) (uint, bool) {
	wanted := hashJoinCode(code)
	j.lock.Lock()
	defer j.lock.Unlock()
	for hash, p := range j.pending {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(wanted)) != 1 {
			continue
		}
		delete(j.pending, hash)
		return p.nodeID, !j.now().After(p.expires)
	}
	return 0, false
}

// nodeJoinCodeView is the answer to an administrator asking for a code
type nodeJoinCodeView struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
	// PanelURL is the address the new node will reach the panel at
	PanelURL string `json:"panelUrl"`
	// Command is what to paste on the new machine, as root
	Command string `json:"command"`
} // @name NodeJoinCode

// nodeJoinView is what the installer gets for a code: everything the daemon needs
type nodeJoinView struct {
	NodeID       uint   `json:"nodeId"`
	Name         string `json:"name"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	// Port is the node's private port: the one the panel connects to and the daemon listens on
	Port     uint16 `json:"port"`
	SFTPPort uint16 `json:"sftpPort"`
} // @name NodeJoin

// joinPanelURL is the panel's address for a new node: the one set in Settings. A local one
// (the default http://localhost:8080) cannot be reached from another machine.
func joinPanelURL() (string, error) {
	address := services.PanelURL()
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errNodeJoinURL
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return "", errNodeJoinURL
	}
	return address, nil
}

// @Summary Make a join code for a node
// @Description A one-time code, valid for an hour, and the command that installs this node on a new machine with it. A new code replaces the node's previous one.
// @Success 200 {object} nodeJoinCodeView
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Param id path string true "Node Id"
// @Tags Nodes
// @Router /api/nodes/{id}/join [post]
// @Security OAuth2Application[nodes.deploy]
func createNodeJoinCode(c *gin.Context) {
	id, ok := validateID(c)
	if !ok {
		return
	}
	ns := &services.Node{DB: middleware.GetDatabase(c)}
	node, err := ns.Get(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.HandleError(c, skypanel.ErrNodeInvalid, http.StatusNotFound)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if node.ID == models.LocalNode.ID {
		response.HandleError(c, errNodeJoinLocal, http.StatusBadRequest)
		return
	}
	panelURL, err := joinPanelURL()
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	code, expires, err := joinCodes.issue(node.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, nodeJoinCodeView{
		Code:      code,
		ExpiresAt: expires,
		PanelURL:  panelURL,
		Command:   fmt.Sprintf("curl -fsSL %s | sudo bash -s -- --node --panel %s --join %s", nodeJoinInstaller, panelURL, code),
	})
}

// @Summary Join a node with a code
// @Description Public: the installer of a new node trades its join code for the node's credentials and ports. A code works once; 10 tries per address every 15 minutes.
// @Success 200 {object} nodeJoinView
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 429 {object} skypanel.ErrorResponse
// @Param body body object true "{\"code\": \"...\"}"
// @Tags Nodes
// @Router /api/node-join [post]
func joinNode(c *gin.Context) {
	if !nodeJoinAttempts.Take(c.ClientIP()) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, &skypanel.ErrorResponse{Error: skypanel.ErrTooManyAttempts})
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := c.BindJSON(&request); err != nil {
		return
	}
	nodeID, ok := joinCodes.redeem(request.Code)
	if !ok {
		response.HandleError(c, errNodeJoinCode, http.StatusNotFound)
		return
	}
	node, err := (&services.Node{DB: middleware.GetDatabase(c)}).Get(nodeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// the node was deleted after the code was made
		response.HandleError(c, errNodeJoinCode, http.StatusNotFound)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	logging.Info.Printf("Node %q (%d) was joined with a join code from %s", node.Name, node.ID, c.ClientIP())
	c.JSON(http.StatusOK, nodeJoinView{
		NodeID:       node.ID,
		Name:         node.Name,
		ClientID:     fmt.Sprintf(".node_%d", node.ID),
		ClientSecret: node.Secret,
		Port:         node.PrivatePort,
		SFTPPort:     node.SFTPPort,
	})
}
