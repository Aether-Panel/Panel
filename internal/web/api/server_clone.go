package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// maxCloneNameLength matches the size of the servers.name column
const maxCloneNameLength = 40

// clonePortSearch is how far above each of the original's ports the copy's port is looked
// for (25565 → 25566), before any free port on the node is taken
const clonePortSearch = 200

// maxDefinitionSize bounds the server definition read from the node
const maxDefinitionSize = 8 << 20

var (
	errCloneNoFreePort = errors.New("there is no free port left on this node for the copy")
	errCloneNotOnNode  = errors.New("the server's files are not on its node, so it cannot be copied")
	errCloneNotMember  = errors.New("you can only copy servers you have access to")
	errCloneSuspended  = errors.New("a suspended server cannot be copied")
)

// registerServerClone adds the route that duplicates a server
func registerServerClone(g *gin.RouterGroup) {
	g.POST("/:serverId/clone", requiresExperimental(config.ExperimentalServerClone), middleware.RequiresPermission(scopes.ScopeServerCreate), middleware.ResolveServerPanel, cloneServer)
	g.OPTIONS("/:serverId/clone", response.CreateOptions("POST"))
}

type cloneServerRequest struct {
	Name      string `json:"name"`
	CopyFiles *bool  `json:"copyFiles"`
	// Label is the copy's name as the activity log shows it on the original
	Label string `json:"label"`
}

type cloneServerResponse struct {
	ID   string `json:"id"`
	Port uint16 `json:"port,omitempty"`
}

// normalize trims the name and applies the defaults; the error explains what to fix
func (r *cloneServerRequest) normalize() (name string, copyFiles bool, err error) {
	name = strings.TrimSpace(r.Name)
	switch {
	case name == "":
		return "", false, errors.New("the copy needs a name")
	case utf8.RuneCountInString(name) > maxCloneNameLength:
		return "", false, fmt.Errorf("the name can be at most %d characters long", maxCloneNameLength)
	}
	for _, ch := range name {
		// the same rule servers.name follows (printascii)
		if ch < 0x20 || ch > 0x7e {
			return "", false, errors.New("the name can only use letters without accents, digits, spaces and common symbols")
		}
	}
	return name, r.CopyFiles == nil || *r.CopyFiles, nil
}

// @Summary Clone a server
// @Description Creates a copy of the server on the same node: same template, settings, resources and users, with new ports. With copyFiles (default) the node also copies its files and world in the background.
// @Success 200 {object} cloneServerResponse
// @Param id path string true "Server ID"
// @Tags Servers
// @Router /api/servers/{id}/clone [post]
// @Security OAuth2Application[server.create]
func cloneServer(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	ps := &services.Permission{DB: db}
	source := getServerFromGin(c)
	user := c.MustGet("user").(*models.User)

	var req cloneServerRequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	name, copyFiles, err := req.normalize()
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	admin, err := authorizeClone(ps, user, source)
	if errors.Is(err, errCloneNotMember) || errors.Is(err, errCloneSuspended) {
		response.HandleError(c, err, http.StatusForbidden)
		return
	} else if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	node, err := ns.Get(source.NodeID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	definition, err := fetchServerDefinition(c, ns, node, source.Identifier)
	if response.HandleError(c, err, http.StatusConflict) {
		return
	}

	newID, err := models.GenerateServerIdentifier()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	postBody := newCloneCreation(definition, source, newID, name)

	if !admin {
		if err := restrictSubServerCreation(ps, user, postBody); response.HandleError(c, err, http.StatusForbidden) {
			return
		}
	}

	mapping, err := assignClonePorts(db, node, postBody.Variables)
	if response.HandleError(c, err, http.StatusConflict) {
		return
	}
	remapPortBindings(&postBody.Environment, mapping)

	server, _, err := validateAndBuildServerCreation(c, db, &services.User{DB: db}, postBody)
	if err != nil {
		return
	}
	server.TotalCPU, server.TotalMemory, server.TotalDisk = source.TotalCPU, source.TotalMemory, source.TotalDisk
	server.PortNotes = remapPortNotes(source.PortNotes, mapping)

	var parentCPU int
	var parentMemory, parentDisk int64
	err = db.Transaction(func(tx *gorm.DB) error {
		if server.ParentServerID != nil && *server.ParentServerID != "" {
			if err := checkParentServerLimits(tx, server, &parentCPU, &parentMemory, &parentDisk); err != nil {
				return err
			}
		}
		if err := tx.Create(server).Error; err != nil {
			return err
		}
		return copyServerPermissions(tx, source.Identifier, server.Identifier)
	})
	if response.HandleError(c, err, http.StatusUnprocessableEntity) {
		return
	}

	if err := notifyNodeOfServerCreation(c, ns, node, postBody, server); err != nil {
		rollbackClone(db, ns, node, server)
		return
	}
	if server.ParentServerID != nil && *server.ParentServerID != "" {
		updateParentServerLimitsOnNode(c, db, ns, node, server, parentCPU, parentMemory, parentDisk)
	}

	if copyFiles {
		if err := startCloneCopy(c, ns, node, server.Identifier, source); err != nil {
			rollbackClone(db, ns, node, server)
			response.HandleError(c, err, http.StatusConflict)
			return
		}
	}

	c.JSON(http.StatusOK, &cloneServerResponse{ID: server.Identifier, Port: server.Port})
}

// authorizeClone tells whether the user is a panel admin and refuses the copy to anyone else
// who has no access to the original or whose original is suspended
func authorizeClone(ps *services.Permission, user *models.User, source *models.Server) (bool, error) {
	if scopes.ContainsScope(getUserScopesForSearch(user, ps), scopes.ScopeAdmin) {
		return true, nil
	}
	// server.create is a panel-wide scope, so it alone does not open someone else's server
	perms, err := ps.GetForUserAndServer(user.ID, source.Identifier)
	if err != nil {
		return false, err
	}
	if perms.ID == 0 {
		return false, errCloneNotMember
	}
	// the copy is not suspended, so it must not become a way around a suspension
	if source.Suspended {
		return false, errCloneSuspended
	}
	return false, nil
}

// fetchServerDefinition reads a server's definition from its node, the same data
// GET /api/servers/:id/definition returns
func fetchServerDefinition(c *gin.Context, ns *services.Node, node *models.Node, id string) (*skypanel.Server, error) {
	ctx, cancel := nodeCallContext(c)
	defer cancel()
	res, err := ns.CallNodeContext(ctx, node, http.MethodGet, "/daemon/server/"+id+"/definition", nil, nil)
	defer utils.CloseResponse(res)
	if err != nil {
		return nil, fmt.Errorf("the node of this server is not reachable: %w", err)
	}
	switch {
	case res.StatusCode == http.StatusNotFound:
		return nil, errCloneNotOnNode
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("the node answered %d when the server was read", res.StatusCode)
	}

	definition := &skypanel.Server{}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxDefinitionSize)).Decode(definition); err != nil {
		return nil, fmt.Errorf("the node sent a server definition that could not be read: %w", err)
	}
	return definition, nil
}

// newCloneCreation describes the copy: the original's definition under a new id and name, on
// the same node and parent. Users are copied afterwards with their own scopes; the subdomain,
// billing link and suspension of the original are not part of it.
func newCloneCreation(definition *skypanel.Server, source *models.Server, newID, name string) *models.ServerCreation {
	postBody := &models.ServerCreation{
		Server: *definition,
		NodeID: source.NodeID,
		Name:   name,
	}
	postBody.Identifier = newID
	if source.ParentServerID != nil && *source.ParentServerID != "" {
		parent := *source.ParentServerID
		postBody.ParentServerID = &parent
	}
	variables := make(map[string]skypanel.Variable, len(definition.Variables))
	for k, v := range definition.Variables {
		variables[k] = v
	}
	postBody.Variables = variables
	return postBody
}

// usedPortsOnNode lists every port the panel has given a server on the node
func usedPortsOnNode(db *gorm.DB, nodeID uint) (map[uint16]bool, error) {
	query := db.Model(&models.Server{})
	if nodeID == 0 {
		query = query.Where("node_id IS NULL")
	} else {
		query = query.Where("node_id = ?", nodeID)
	}
	var onNode []*models.Server
	if err := query.Find(&onNode).Error; err != nil {
		return nil, err
	}
	used := make(map[uint16]bool)
	for _, s := range onNode {
		used[s.Port] = true
		for _, p := range s.Ports {
			used[p] = true
		}
	}
	delete(used, 0)
	return used, nil
}

// assignClonePorts gives every port variable of the copy (port, port2…, query ports) a port
// that no server on the node uses, preferring the next ones after the original's. Variables
// sharing a port keep sharing the new one. It returns old → new.
func assignClonePorts(db *gorm.DB, node *models.Node, variables map[string]skypanel.Variable) (map[uint16]uint16, error) {
	taken, err := usedPortsOnNode(db, node.ID)
	if err != nil {
		return nil, err
	}
	for _, p := range []uint16{node.PublicPort, node.PrivatePort, node.SFTPPort} {
		taken[p] = true
	}

	keys := make([]string, 0, len(variables))
	for k, v := range variables {
		if servers.IsPortVariable(k) && servers.PortValue(v.Value) > 0 {
			keys = append(keys, k)
			// the original keeps its ports, even those the panel does not track
			taken[servers.PortValue(v.Value)] = true
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "port" || keys[j] == "port" {
			return keys[i] == "port"
		}
		return keys[i] < keys[j]
	})

	mapping := make(map[uint16]uint16, len(keys))
	for _, k := range keys {
		v := variables[k]
		old := servers.PortValue(v.Value)
		next, ok := mapping[old]
		if !ok {
			next = nextFreePort(db, node.ID, old, taken)
			if next == 0 {
				return nil, errCloneNoFreePort
			}
			taken[next] = true
			mapping[old] = next
		}
		v.Value = portValueLike(v.Value, next)
		variables[k] = v
	}

	if err := validatePortsAgainstNode(db, node.ID, "", collectPortsFromVariables(variables)); err != nil {
		return nil, err
	}
	return mapping, nil
}

// nextFreePort looks just above after first, then anywhere on the node
func nextFreePort(db *gorm.DB, nodeID uint, after uint16, taken map[uint16]bool) uint16 {
	start := max(int(after)+1, 1024)
	for p := start; p <= int(after)+clonePortSearch && p <= 65535; p++ {
		if !taken[uint16(p)] {
			return uint16(p)
		}
	}
	for range 50 {
		if p := pickFreePort(db, nodeID, 0, 0); p != 0 && !taken[p] {
			return p
		}
	}
	return 0
}

// portValueLike stores a port the way the original value was stored (text or number)
func portValueLike(original interface{}, port uint16) interface{} {
	if _, isText := original.(string); isText {
		return strconv.Itoa(int(port))
	}
	return int(port)
}

// remapPortBindings changes ports written as plain numbers in a docker environment's
// portBindings ("0.0.0.0:25565:25565/tcp"); bindings that use ${port} follow the variables by
// themselves
func remapPortBindings(environment *skypanel.MetadataType, mapping map[uint16]uint16) {
	if len(mapping) == 0 || environment.Metadata == nil {
		return
	}
	bindings, ok := environment.Metadata["portBindings"].([]interface{})
	if !ok {
		return
	}
	for i, b := range bindings {
		if spec, ok := b.(string); ok {
			bindings[i] = remapBindingSpec(spec, mapping)
		}
	}
}

// remapBindingSpec swaps every host:container part of a binding that is exactly an old port
func remapBindingSpec(spec string, mapping map[uint16]uint16) string {
	parts := strings.Split(spec, ":")
	for i, part := range parts {
		port, proto, hasProto := strings.Cut(part, "/")
		p, err := strconv.Atoi(port)
		if err != nil || p <= 0 || p > 65535 {
			continue
		}
		next, ok := mapping[uint16(p)]
		if !ok {
			continue
		}
		parts[i] = strconv.Itoa(int(next))
		if hasProto {
			parts[i] += "/" + proto
		}
	}
	return strings.Join(parts, ":")
}

// remapPortNotes moves the original's port labels to the copy's ports
func remapPortNotes(notes map[string]string, mapping map[uint16]uint16) map[string]string {
	if len(notes) == 0 {
		return nil
	}
	moved := make(map[string]string, len(notes))
	for port, note := range notes {
		p, err := strconv.Atoi(port)
		if err != nil {
			continue
		}
		if next, ok := mapping[uint16(p)]; ok && p > 0 && p <= 65535 {
			moved[strconv.Itoa(int(next))] = note
		}
	}
	return moved
}

// copyServerPermissions gives every user of the original the same access to the copy. Rows of
// OAuth clients (SFTP, API keys) belong to the original only.
func copyServerPermissions(tx *gorm.DB, sourceID, targetID string) error {
	var rows []*models.Permissions
	if err := tx.Where("server_identifier = ? AND user_id IS NOT NULL", sourceID).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		target := targetID
		userID := *row.UserID
		copied := &models.Permissions{
			UserID:           &userID,
			ServerIdentifier: &target,
			Scopes:           append([]*scopes.Scope(nil), row.Scopes...),
		}
		if err := tx.Omit(clause.Associations).Create(copied).Error; err != nil {
			return err
		}
	}
	return nil
}

// startCloneCopy asks the node to copy the original's files into the new server
func startCloneCopy(c *gin.Context, ns *services.Node, node *models.Node, newID string, source *models.Server) error {
	body, _ := json.Marshal(map[string]string{"sourceName": source.Name})
	ctx, cancel := nodeCallContext(c)
	defer cancel()
	res, err := ns.CallNodeContext(ctx, node, http.MethodPost, "/daemon/server/"+newID+"/clone-from/"+source.Identifier, io.NopCloser(bytes.NewReader(body)), http.Header{"Content-Type": {"application/json"}})
	defer utils.CloseResponse(res)
	if err != nil {
		return fmt.Errorf("the node of this server is not reachable: %w", err)
	}
	if res.StatusCode != http.StatusAccepted {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		logging.Error.Printf("Node refused to copy %s into %s: %d %s", source.Identifier, newID, res.StatusCode, string(detail))
		if res.StatusCode == http.StatusNotFound {
			return errCloneNotOnNode
		}
		return fmt.Errorf("the node could not start copying the files (%d)", res.StatusCode)
	}
	return nil
}

// rollbackClone removes a copy that could not be finished, from the node and the panel
func rollbackClone(db *gorm.DB, ns *services.Node, node *models.Node, server *models.Server) {
	// the node may not have it yet; it then answers 404, which is fine
	res, err := ns.CallNode(node, http.MethodDelete, "/daemon/server/"+server.Identifier, nil, nil)
	utils.CloseResponse(res)
	if err != nil {
		logging.Error.Printf("Error removing unfinished copy %s from its node: %s", server.Identifier, err)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("server_identifier = ?", server.Identifier).Delete(&models.Permissions{}).Error; err != nil {
			return err
		}
		return tx.Where("identifier = ?", server.Identifier).Delete(&models.Server{}).Error
	})
	if err != nil {
		logging.Error.Printf("Error removing unfinished copy %s: %s", server.Identifier, err)
	}
	if server.ParentServerID != nil && *server.ParentServerID != "" {
		updateParentServerLimitsAfterDeletion(db, ns, node, server)
	}
}
