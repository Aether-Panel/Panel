package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// transferLogFormat is how the panel log names a transfer step or failure of one server
const transferLogFormat = "Transfer of %s: %s"

type TransferRequest struct {
	NodeID *uint `json:"nodeId" binding:"required"`
}

// @Summary Transfer server
// @Description Transfer a server from one node to another, with its backups and its sub-servers. When a step fails the transfer is undone and the server stays on its node.
// @Accept json
// @Produce json
// @Param id path string true "Server ID"
// @Param request body TransferRequest true "Target Node ID"
// @Success 202 {object} string "Transfer started"
// @Failure 400 {object} skypanel.ErrorResponse "A sub-server moves with its parent"
// @Failure 409 {object} skypanel.ErrorResponse "Ports of the server are taken on the target node, it is already being transferred, or a backup of it is being written"
// @Tags Servers
// @Router /api/servers/{id}/transfer [post]
// @Security OAuth2Application[server.data.edit.admin]
func transferServer(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)

	var req TransferRequest
	if err := c.BindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// a sub-server lives on its parent's node: the parent's limits there are what its
	// sub-servers leave free, and the panel updates them on the sub-server's node
	if hasParentServer(server) {
		response.HandleError(c, skypanel.ErrTransferSubServer, http.StatusBadRequest)
		return
	}

	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}

	// Get target node
	targetNode, err := ns.Get(*req.NodeID)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if targetNode.ID == server.NodeID {
		response.HandleError(c, errors.New("cannot transfer to the same node"), http.StatusBadRequest)
		return
	}

	children, err := subServersToMove(db, server, targetNode)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	family := append([]*models.Server{server}, children...)

	// two transfers of the same servers would copy, stop and delete them under each other
	ids := familyIDs(family)
	claimed, err := claimTransfer(db, ids)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if !claimed {
		response.HandleError(c, skypanel.ErrTransferRunning, http.StatusConflict)
		return
	}
	started := false
	defer func() {
		if !started {
			releaseTransfer(ids)
		}
	}()

	if !transferCanStart(c, db, ns, family, targetNode) {
		return
	}

	var by *models.User
	if v, ok := c.Get("user"); ok {
		by, _ = v.(*models.User)
	}
	// kept until the transfer ends, so a panel that restarts meanwhile can undo or finish it
	record, err := recordNodeTransfer(db, family, targetNode, by)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	started = true
	go performInternalTransfer(record, family, targetNode, db, by)

	c.JSON(http.StatusAccepted, gin.H{"msg": "Transfer started"})
}

// transferCanStart checks what the target node and the old node must allow before the servers
// move, and writes the refusal itself
func transferCanStart(c *gin.Context, db *gorm.DB, ns *services.Node, family []*models.Server, targetNode *models.Node) bool {
	// the servers keep their ports on the new node; one taken there would only show once the
	// moved server failed to start
	held, err := portsHeldOnNode(db, ns, targetNode, family[0].Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return false
	}
	if taken := familyPortsTaken(family, held); len(taken) > 0 {
		response.HandleError(c, skypanel.ErrTransferPortsTaken(taken), http.StatusConflict)
		return false
	}

	// a backup being written would not be finished when the backups are copied, and would be
	// lost with the old node's copy
	callerFor := func(s *models.Server) nodeCaller { return nodeCallerFor(ns, &s.Node) }
	if firstWithBackupRunning(family, callerFor) != "" {
		response.HandleError(c, skypanel.ErrTransferBackupRunning, http.StatusConflict)
		return false
	}
	return true
}

// subServersToMove lists the Splitter sub-servers that go with the server: they live on their
// parent's node and take their resources out of its plan there. One already on the target
// node stays where it is.
func subServersToMove(db *gorm.DB, server *models.Server, targetNode *models.Node) ([]*models.Server, error) {
	var children []*models.Server
	if err := db.Preload("Node").Where(parentServerIDQuery, server.Identifier).Find(&children).Error; err != nil {
		return nil, err
	}
	moving := make([]*models.Server, 0, len(children))
	for _, child := range children {
		if child.NodeID != targetNode.ID {
			moving = append(moving, child)
		}
	}
	return moving, nil
}

// performInternalTransfer moves the server and its sub-servers (family, the server first) to the
// target node. Nothing in the panel changes until all of them are on the new node with their
// files and backups; when a step fails before that, everything is undone and they stay where
// they were. How far it got is kept in record until it ends.
func performInternalTransfer(record *models.NodeTransfer, family []*models.Server, targetNode *models.Node, db *gorm.DB, by *models.User) {
	server := family[0]
	logging.Info.Printf("Starting transfer of server %s to node %s", server.Identifier, targetNode.Name)
	defer endNodeTransfer(db, record, familyIDs(family))

	ns := &services.Node{DB: db}
	run := &transferRun{db: db, record: record}
	for _, s := range family {
		// the panel drops the records of backups that failed, so none is left pointing at a
		// backup the old node never had
		syncAutomaticBackups(context.Background(), db, s)
		m := newServerMove(ns, s, targetNode)
		m.persist = run.save
		run.moves = append(run.moves, m)
	}
	run.save()

	commit := func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			for _, s := range family {
				if err := moveServerRecordToNode(tx, s.Identifier, targetNode); err != nil {
					return err
				}
			}
			// in the same transaction: a panel that restarts finds the servers moved exactly
			// when the record says so
			return tx.Model(&models.NodeTransfer{}).Where("id = ?", record.ID).Update("committed", true).Error
		})
	}

	if err := moveServers(run.moves, commit); err != nil {
		logging.Error.Printf("Transfer of %s to node %s failed and was undone, it stays on node %s: %s",
			server.Identifier, targetNode.Name, server.Node.Name, err)
		// the owner sees it in the server's activity, not only whoever reads the panel log
		for _, s := range family {
			recordTransferActivity(db, s.Identifier, activityTransferFailed, err.Error(), by)
		}
		return
	}

	for _, s := range family {
		s.NodeID = targetNode.ID
		s.Node = *targetNode
		// its subdomain follows it to the new node's address
		scheduleSubdomainRefresh(s.Identifier)
	}
	// the transfer is done either way: a server that does not start is reported, not moved back
	for id, err := range resumeMovedServers(run.moves) {
		logging.Error.Printf(transferLogFormat, id, err)
		recordTransferActivity(db, id, activityTransferStartFailed, err.Error(), by)
	}
	logging.Info.Printf("Transfer of %s completed successfully", server.Identifier)
}

// transferRun is a node transfer being made and its record
type transferRun struct {
	db     *gorm.DB
	record *models.NodeTransfer
	moves  []*serverMove
}

// save writes how far each server got, for a panel that restarts before the transfer ends
func (r *transferRun) save() {
	r.record.Moves = make([]models.NodeTransferMove, 0, len(r.moves))
	for _, m := range r.moves {
		r.record.Moves = append(r.record.Moves, m.state())
	}
	if err := r.db.Model(r.record).Select("moves").Updates(r.record).Error; err != nil {
		logging.Error.Printf("Transfer of %s: could not save how far it got: %s", r.record.ServerID, err)
	}
}

// recordNodeTransfer stores the transfer about to start
func recordNodeTransfer(db *gorm.DB, family []*models.Server, targetNode *models.Node, by *models.User) (*models.NodeTransfer, error) {
	record := &models.NodeTransfer{ServerID: family[0].Identifier, TargetNodeID: targetNode.ID}
	if by != nil {
		id := by.ID
		record.UserID = &id
	}
	for _, s := range family {
		record.Moves = append(record.Moves, models.NodeTransferMove{ServerID: s.Identifier, SourceNodeID: s.NodeID, Step: stepWaiting})
	}
	return record, db.Create(record).Error
}

// endNodeTransfer forgets a transfer that ended, one way or the other
func endNodeTransfer(db *gorm.DB, record *models.NodeTransfer, ids []string) {
	if err := db.Delete(&models.NodeTransfer{}, record.ID).Error; err != nil {
		logging.Error.Printf("Transfer of %s: could not remove its record: %s", record.ServerID, err)
	}
	releaseTransfer(ids)
}

// RecoverNodeTransfers ends the node transfers a stop of the panel cut short. One that had not
// pointed the panel at the new node yet is undone like a failed transfer: the copies on the
// new node go, and the servers get their tasks back and run again on their old node if they
// ran before. One that had is finished: the old copies go, and the servers get their tasks
// back and start on the new node if they ran before.
func RecoverNodeTransfers() {
	db, err := database.GetConnection()
	if err != nil {
		logging.Error.Printf("Could not look for interrupted node transfers: %s", err)
		return
	}
	ns := &services.Node{DB: db}
	recoverNodeTransfers(db, func(id uint) (nodeCaller, error) {
		node, err := ns.Get(id)
		if err != nil {
			return nil, err
		}
		return nodeCallerFor(ns, node), nil
	})
}

// recoverNodeTransfers is RecoverNodeTransfers with the calls to the nodes given
func recoverNodeTransfers(db *gorm.DB, callerFor func(nodeID uint) (nodeCaller, error)) {
	var records []models.NodeTransfer
	if err := db.Find(&records).Error; err != nil {
		logging.Error.Printf("Could not look for interrupted node transfers: %s", err)
		return
	}
	for i := range records {
		recoverNodeTransfer(db, &records[i], callerFor)
	}
}

func recoverNodeTransfer(db *gorm.DB, record *models.NodeTransfer, callerFor func(nodeID uint) (nodeCaller, error)) {
	logging.Info.Printf("Ending the transfer of %s that a panel restart interrupted", record.ServerID)
	reach := func(nodeID uint) nodeCaller {
		call, err := callerFor(nodeID)
		if err != nil {
			// its steps fail and are logged one by one
			return func(string, string, io.ReadCloser, http.Header) (*http.Response, error) { return nil, err }
		}
		return call
	}

	target := reach(record.TargetNodeID)
	moves := make([]*serverMove, 0, len(record.Moves))
	ids := make([]string, 0, len(record.Moves))
	for _, state := range record.Moves {
		moves = append(moves, &serverMove{
			id:            state.ServerID,
			sourceNodeID:  state.SourceNodeID,
			source:        reach(state.SourceNodeID),
			target:        target,
			step:          state.Step,
			wasRunning:    state.WasRunning,
			copiedBackups: state.CopiedBackups,
		})
		ids = append(ids, state.ServerID)
	}

	var by *models.User
	if record.UserID != nil {
		user := &models.User{}
		if err := db.First(user, *record.UserID).Error; err == nil {
			by = user
		}
	}

	if record.Committed {
		for _, m := range moves {
			m.finish()
			scheduleSubdomainRefresh(m.id)
		}
		for id, err := range resumeMovedServers(moves) {
			logging.Error.Printf(transferLogFormat, id, err)
			recordTransferActivity(db, id, activityTransferStartFailed, err.Error(), by)
		}
	} else {
		// what was done for each server is not known for sure: everything that may have been
		// is undone, and undoing what was not is harmless
		for _, m := range moves {
			m.createdOnTarget = true
			m.archived = true
			m.pausedSource = true
		}
		undoMoves(moves)
		for _, m := range moves {
			recordTransferActivity(db, m.id, activityTransferFailed, "the panel restarted while "+m.step+": the transfer was undone", by)
		}
	}
	endNodeTransfer(db, record, ids)
}

// The activity log entries of a node transfer that the request itself cannot record, because
// they happen after it was answered
const (
	activityTransferFailed      = "server.transferFailed"
	activityTransferStartFailed = "server.transferStartFailed"
)

// recordTransferActivity adds what happened to a transfer to the server's activity log, under
// the name of who asked for it
func recordTransferActivity(db *gorm.DB, serverID, action, detail string, by *models.User) {
	entry := &models.ServerActivity{ServerID: serverID, Action: action, Detail: detail}
	if by != nil {
		id := by.ID
		entry.UserID = &id
		entry.Username = by.Username
	}
	sa := &services.ServerActivity{DB: db}
	if err := sa.Record(entry); err != nil {
		logging.Error.Printf("Failed to record activity %s on server %s: %s", action, serverID, err)
	}
}

// resumeMovedServers gives the servers their tasks back on the new node and starts there the
// ones that were running before the transfer. It returns why each one that did not start
// failed; tasks that could not be resumed are logged.
func resumeMovedServers(moves []*serverMove) map[string]error {
	failed := make(map[string]error)
	for _, m := range moves {
		m.undoStep(m.target, http.MethodPost, daemonServerPrefix+m.id+schedulerResume, "resuming its tasks on the new node")
		if !m.wasRunning {
			continue
		}
		// a panel that restarted after starting it finds it running already
		if running, err := runningOn(m.target, m.id); err == nil && running {
			continue
		}
		res, err := m.target(http.MethodPost, daemonServerPrefix+m.id+startAndWait, nil, nil)
		if err = daemonStepError("starting it on the new node", res, err); err != nil {
			failed[m.id] = err
		}
	}
	return failed
}

// How the daemon pauses and resumes a server's tasks
const (
	schedulerPause  = "/scheduler/pause"
	schedulerResume = "/scheduler/resume"
)

// The steps of a server's transfer, as the record keeps them and the activity log reads them
const (
	stepWaiting        = "waiting for its turn"
	stepPausingTasks   = "pausing its tasks"
	stepCreating       = "creating it on the new node"
	stepStopping       = "stopping it"
	stepCopyingFiles   = "copying its files"
	stepCopyingBackups = "copying its backups"
	stepCopied         = "waiting for the panel to point at the new node"
)

// startAndWait starts a server and answers once it started, or why it could not
const startAndWait = "/start?wait=true"

var (
	transfersMu sync.Mutex

	// transfersRunning holds the servers being moved to another node right now
	transfersRunning = make(map[string]bool)
)

// familyIDs are the identifiers of the servers moved together
func familyIDs(family []*models.Server) []string {
	ids := make([]string, 0, len(family))
	for _, s := range family {
		ids = append(ids, s.Identifier)
	}
	return ids
}

// claimTransfer marks the servers as being moved, all of them or none: false when one of them
// already is, here or in a transfer recorded before the panel restarted. A nil db only looks
// here.
func claimTransfer(db *gorm.DB, ids []string) (bool, error) {
	transfersMu.Lock()
	defer transfersMu.Unlock()
	for _, id := range ids {
		if transfersRunning[id] {
			return false, nil
		}
	}
	if db != nil {
		recorded, err := serversInRecordedTransfers(db)
		if err != nil {
			return false, err
		}
		for _, id := range ids {
			if recorded[id] {
				return false, nil
			}
		}
	}
	for _, id := range ids {
		transfersRunning[id] = true
	}
	return true, nil
}

// serversInRecordedTransfers are the servers of every transfer that has not ended
func serversInRecordedTransfers(db *gorm.DB) (map[string]bool, error) {
	var records []models.NodeTransfer
	if err := db.Find(&records).Error; err != nil {
		return nil, err
	}
	held := make(map[string]bool)
	for _, r := range records {
		held[r.ServerID] = true
		for _, m := range r.Moves {
			held[m.ServerID] = true
		}
	}
	return held, nil
}

// releaseTransfer lets the servers be moved again
func releaseTransfer(ids []string) {
	transfersMu.Lock()
	defer transfersMu.Unlock()
	for _, id := range ids {
		delete(transfersRunning, id)
	}
}

// firstWithBackupRunning is the first server whose node is writing a backup or restore of it,
// "" when none is. A node that cannot tell is not taken as busy: the transfer finds out about a
// node it cannot reach by itself.
func firstWithBackupRunning(family []*models.Server, callerFor func(*models.Server) nodeCaller) string {
	for _, s := range family {
		if busy, err := backupRunningOn(callerFor(s), s.Identifier); err == nil && busy {
			return s.Identifier
		}
	}
	return ""
}

// backupRunningOn asks the node whether a backup, restore point or restore of the server is
// being written
func backupRunningOn(call nodeCaller, id string) (bool, error) {
	res, err := call(http.MethodGet, daemonServerPrefix+id+"/backup/busy", nil, nil)
	defer utils.CloseResponse(res)
	if err != nil {
		return false, err
	}
	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("the node answered %d", res.StatusCode)
	}
	var answer struct {
		Busy bool `json:"busy"`
	}
	if err = json.NewDecoder(res.Body).Decode(&answer); err != nil {
		return false, err
	}
	return answer.Busy, nil
}

// moveServers copies every server in full (definition, files and backups) to the new node,
// then commit points the panel at it, and only then are the servers removed from their old
// node. When a step before that fails, what was done for every server is undone.
func moveServers(moves []*serverMove, commit func() error) error {
	for _, m := range moves {
		if err := m.prepare(); err != nil {
			undoMoves(moves)
			return fmt.Errorf("server %s, %w", m.id, err)
		}
	}
	if err := commit(); err != nil {
		undoMoves(moves)
		return fmt.Errorf("pointing the panel at the new node: %w", err)
	}
	for _, m := range moves {
		m.finish()
	}
	return nil
}

// undoMoves undoes the servers newest first; one that was not reached has nothing to undo
func undoMoves(moves []*serverMove) {
	for i := len(moves) - 1; i >= 0; i-- {
		moves[i].rollback()
	}
}

// serverPortsTaken lists the server's ports that something already holds on the target node,
// in order. Its own ports below 1024 were allowed where it is, so only holders count.
func serverPortsTaken(server *models.Server, held map[uint16]portHolder) []uint16 {
	taken := make([]uint16, 0)
	for _, p := range append([]uint16{server.Port}, server.Ports...) {
		if _, busy := held[p]; p != 0 && busy && !slices.Contains(taken, p) {
			taken = append(taken, p)
		}
	}
	slices.Sort(taken)
	return taken
}

// familyPortsTaken is serverPortsTaken for a server and the sub-servers moving with it
func familyPortsTaken(family []*models.Server, held map[uint16]portHolder) []uint16 {
	taken := make([]uint16, 0)
	for _, s := range family {
		for _, p := range serverPortsTaken(s, held) {
			if !slices.Contains(taken, p) {
				taken = append(taken, p)
			}
		}
	}
	slices.Sort(taken)
	return taken
}

// daemonBackupFile is where a node receives, and gives back, the backups a transfer copies
const daemonBackupFile = "/backup/file"

// serverMove is one server of a node transfer, with what has been done for it so far, so a
// failed transfer undoes exactly that
type serverMove struct {
	id           string
	sourceNodeID uint
	source       nodeCaller
	target       nodeCaller

	// step is the last step begun, for the record and the activity log
	step            string
	pausedSource    bool
	createdOnTarget bool
	wasRunning      bool
	archived        bool

	// copiedBackups are the backups this transfer wrote, or is writing, on the new node
	copiedBackups []string

	// persist saves how far the server got; nil saves nothing
	persist func()
}

func newServerMove(ns *services.Node, s *models.Server, targetNode *models.Node) *serverMove {
	// a copy: the server's own node changes once it is moved
	sourceNode := s.Node
	return &serverMove{
		id:           s.Identifier,
		sourceNodeID: s.NodeID,
		source:       nodeCallerFor(ns, &sourceNode),
		target:       nodeCallerFor(ns, targetNode),
		step:         stepWaiting,
	}
}

// state is what the record keeps of the server's move
func (m *serverMove) state() models.NodeTransferMove {
	return models.NodeTransferMove{
		ServerID:      m.id,
		SourceNodeID:  m.sourceNodeID,
		Step:          m.step,
		WasRunning:    m.wasRunning,
		CopiedBackups: slices.Clone(m.copiedBackups),
	}
}

func (m *serverMove) save() {
	if m.persist != nil {
		m.persist()
	}
}

// reach begins a step
func (m *serverMove) reach(step string) {
	logging.Info.Printf(transferLogFormat, m.id, step)
	m.step = step
	m.save()
}

// nodeCallerFor makes the calls to one node
func nodeCallerFor(ns *services.Node, node *models.Node) nodeCaller {
	return func(method, path string, body io.ReadCloser, headers http.Header) (*http.Response, error) {
		return ns.CallNode(node, method, path, body, headers)
	}
}

// prepare puts a full copy of the server on the new node; the old node keeps its own until
// the panel points at the new one. Its tasks are paused on both nodes meanwhile: a task run
// on the old node could start the server or back it up in the middle of the copy, and one on
// the new node would act on a copy not finished yet.
func (m *serverMove) prepare() error {
	m.reach(stepPausingTasks)
	m.pausedSource = true
	if err := pauseTasks(m.source, m.id, "the old node"); err != nil {
		return err
	}
	m.reach(stepCreating)
	if err := m.copyDefinition(); err != nil {
		return err
	}
	if err := pauseTasks(m.target, m.id, "the new node"); err != nil {
		return err
	}
	if err := m.copyTasks(); err != nil {
		return err
	}
	if err := m.copyOffsite(); err != nil {
		return err
	}
	m.reach(stepStopping)
	if err := m.stop(); err != nil {
		return fmt.Errorf("stopping it: %w", err)
	}
	// a backup that began after the transfer was asked for is not finished yet
	if busy, err := backupRunningOn(m.source, m.id); err == nil && busy {
		return errors.New("a backup or restore of it is being written: try again once it is done")
	}
	m.reach(stepCopyingFiles)
	m.archived = true
	if err := buildOriginArchive(m.source, m.id); err != nil {
		return err
	}
	if err := m.streamArchive(); err != nil {
		return err
	}
	if err := m.extractArchive(); err != nil {
		return err
	}
	m.reach(stepCopyingBackups)
	if err := m.copyBackups(); err != nil {
		return err
	}
	m.reach(stepCopied)
	return nil
}

// pauseTasks keeps the server's tasks from running on one of its nodes
func pauseTasks(call nodeCaller, id, where string) error {
	res, err := call(http.MethodPost, daemonServerPrefix+id+schedulerPause, nil, nil)
	return daemonStepError("pausing its tasks on "+where, res, err)
}

// copyTasks copies the server's scheduled tasks to the new node: they are kept apart from its
// definition, so creating it there does not bring them
func (m *serverMove) copyTasks() error {
	res, err := m.source(http.MethodGet, daemonServerPrefix+m.id+"/tasks", nil, nil)
	if err != nil {
		return fmt.Errorf("reading its tasks: %w", err)
	}
	defer utils.CloseResponse(res)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("reading its tasks: the node answered %d: %s", res.StatusCode, readFailureBody(res))
	}
	var tasks skypanel.ServerTasks
	if err = json.NewDecoder(res.Body).Decode(&tasks); err != nil {
		return fmt.Errorf("reading its tasks: %w", err)
	}
	headers := http.Header{}
	headers.Set(headerContentType, contentTypeJSON)
	for id, task := range tasks.Tasks {
		body, err := json.Marshal(task.Task)
		if err != nil {
			return err
		}
		answer, err := m.target(http.MethodPut, daemonServerPrefix+m.id+"/tasks/"+url.PathEscape(id), io.NopCloser(bytes.NewReader(body)), headers)
		if err = daemonStepError("copying the task "+id, answer, err); err != nil {
			return err
		}
	}
	return nil
}

// copyOffsite copies the server's own periodic off-site copy setting to the new node: like its
// tasks, it lives apart from its definition. An old node without the setting answers 404, and
// one that has nothing set leaves the new node on the panel's setting: nothing to copy.
func (m *serverMove) copyOffsite() error {
	res, err := m.source(http.MethodGet, daemonServerPrefix+m.id+"/offsite", nil, nil)
	if err != nil {
		return fmt.Errorf("reading its off-site settings: %w", err)
	}
	defer utils.CloseResponse(res)
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("reading its off-site settings: the node answered %d: %s", res.StatusCode, readFailureBody(res))
	}
	// only what the server set: the rest of the answer is what the old node uses and knows
	var settings servers.OffsiteSettings
	if err = json.NewDecoder(res.Body).Decode(&settings); err != nil {
		return fmt.Errorf("reading its off-site settings: %w", err)
	}
	if settings.PeriodicHours == nil && settings.PeriodicKeep == nil {
		return nil
	}
	body, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	headers := http.Header{}
	headers.Set(headerContentType, contentTypeJSON)
	answer, err := m.target(http.MethodPut, daemonServerPrefix+m.id+"/offsite", io.NopCloser(bytes.NewReader(body)), headers)
	return daemonStepError("copying its off-site settings", answer, err)
}

// copyDefinition reads the server definition from the old node and creates the server with
// it on the new node
func (m *serverMove) copyDefinition() error {
	def, err := m.source(http.MethodGet, daemonServerPrefix+m.id+"/definition", nil, nil)
	if err != nil {
		return fmt.Errorf("reading its definition: %w", err)
	}
	defer utils.CloseResponse(def)
	if def.StatusCode != http.StatusOK {
		return fmt.Errorf("reading its definition: the node answered %d: %s", def.StatusCode, readFailureBody(def))
	}

	headers := http.Header{}
	headers.Set(headerContentType, contentTypeJSON)
	res, err := m.target(http.MethodPut, daemonServerPrefix+m.id, def.Body, headers)
	// a node that answered with an error removed what it began itself; one that could not
	// be heard may have created it
	m.createdOnTarget = err != nil || isDaemonSuccess(res, err)
	return daemonStepError("creating it on the new node", res, err)
}

// transferKillWait is how long a killed server is given to go down before the transfer gives up
var transferKillWait = 2 * time.Second

// stop stops the server on its old node and waits for it; one that does not stop in time is
// killed. Archiving a running server copies a world that is still being written, and anything
// saved after the archive is lost, so the transfer only goes on once the node says it is
// stopped.
func (m *serverMove) stop() error {
	running, err := runningOn(m.source, m.id)
	if err != nil {
		return fmt.Errorf("could not read its status: %w", err)
	}
	if !running {
		return nil
	}
	m.wasRunning = true
	m.save()
	res, err := m.source(http.MethodPost, daemonServerPrefix+m.id+"/stop?wait=true", nil, nil)
	utils.CloseResponse(res)
	if err == nil {
		if running, err = runningOn(m.source, m.id); err == nil && !running {
			return nil
		}
	}
	res, err = m.source(http.MethodPost, daemonServerPrefix+m.id+"/kill", nil, nil)
	utils.CloseResponse(res)
	if err != nil {
		return fmt.Errorf("could not kill it: %w", err)
	}
	time.Sleep(transferKillWait)
	if running, err = runningOn(m.source, m.id); err != nil || running {
		return errors.New("it is still running")
	}
	return nil
}

// runningOn asks a node whether the server is running
func runningOn(call nodeCaller, id string) (bool, error) {
	res, err := call(http.MethodGet, daemonServerPrefix+id+"/status", nil, nil)
	defer utils.CloseResponse(res)
	if err != nil {
		return false, err
	}
	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("status %d", res.StatusCode)
	}
	var status skypanel.ServerRunning
	if err = json.NewDecoder(res.Body).Decode(&status); err != nil {
		return false, err
	}
	return status.Running, nil
}

// streamArchive downloads the archive from the old node and uploads it to the new one
// through a pipe, without buffering it
func (m *serverMove) streamArchive() error {
	download, err := m.source(http.MethodGet, originArchivePath(m.id, originArchive), nil, nil)
	if err != nil {
		return fmt.Errorf("downloading the archive: %w", err)
	}
	defer utils.CloseResponse(download)
	if download.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading the archive: the node answered %d: %s", download.StatusCode, readFailureBody(download))
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		part, err := writer.CreateFormFile("file", originArchive)
		if err == nil {
			_, err = io.Copy(part, download.Body)
		}
		if err == nil {
			err = writer.Close()
		}
		// a copy cut short fails the upload, instead of ending it as a smaller archive
		_ = pw.CloseWithError(err)
	}()

	headers := http.Header{}
	headers.Set(headerContentType, writer.FormDataContentType())
	res, err := m.target(http.MethodPut, originArchivePath(m.id, originArchive), pr, headers)
	// the copy must not wait forever for a node that stopped reading
	_ = pr.Close()
	return daemonStepError("uploading the archive", res, err)
}

// extractArchive unpacks the archive into the server's folder on the new node, then removes it
func (m *serverMove) extractArchive() error {
	res, err := m.target(http.MethodPost, fmt.Sprintf("%s%s/extract/%s?destination=.", daemonServerPrefix, m.id, originArchive), nil, nil)
	if err = daemonStepError("extracting the archive", res, err); err != nil {
		return err
	}
	callQuietly(m.target, http.MethodDelete, originArchivePath(m.id, originArchive))
	return nil
}

// copyBackups copies every backup the old node keeps for the server to the new one, so the
// panel's records, which follow the server, find their files there. A backup only kept
// off-site needs nothing: the bucket is the same wherever the server runs.
func (m *serverMove) copyBackups() error {
	res, err := m.source(http.MethodGet, daemonServerPrefix+m.id+"/backup", nil, nil)
	if err != nil {
		return fmt.Errorf("listing its backups: %w", err)
	}
	defer utils.CloseResponse(res)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("listing its backups: the node answered %d: %s", res.StatusCode, readFailureBody(res))
	}
	var backups []skypanel.BackupFile
	if err = json.NewDecoder(res.Body).Decode(&backups); err != nil {
		return fmt.Errorf("listing its backups: %w", err)
	}
	for _, b := range backups {
		// "local" and "both" have an archive on the node; "offsite" and "failed" do not
		if b.Location != "local" && b.Location != "both" {
			continue
		}
		if err = m.copyBackup(b); err != nil {
			return fmt.Errorf("copying the backup %s: %w", b.FileName, err)
		}
	}
	return nil
}

// copyBackup streams one backup from the old node into the new one
func (m *serverMove) copyBackup(b skypanel.BackupFile) error {
	name := url.QueryEscape(b.FileName)
	download, err := m.source(http.MethodGet, daemonServerPrefix+m.id+"/backup/download"+fileNameParam+name, nil, nil)
	if err != nil {
		return err
	}
	defer utils.CloseResponse(download)
	if download.StatusCode != http.StatusOK {
		return fmt.Errorf("the old node answered %d: %s", download.StatusCode, readFailureBody(download))
	}

	headers := http.Header{}
	headers.Set(headerContentType, "application/octet-stream")
	path := daemonServerPrefix + m.id + daemonBackupFile + fileNameParam + name + "&modTime=" + url.QueryEscape(b.ModTime.Format(time.RFC3339Nano))
	// recorded before it is written: a panel that restarts during the copy takes it back too
	m.copiedBackups = append(m.copiedBackups, b.FileName)
	m.save()
	res, err := m.target(http.MethodPut, path, download.Body, headers)
	if err == nil && res != nil && res.StatusCode == http.StatusConflict {
		// the new node already has this very backup, from an earlier stay of the server there:
		// it is not this transfer's to take back
		utils.CloseResponse(res)
		m.copiedBackups = m.copiedBackups[:len(m.copiedBackups)-1]
		m.save()
		return nil
	}
	return daemonStepError("writing it on the new node", res, err)
}

// rollback undoes what was done for the server, newest first: what it brought to the new node
// goes, and the server runs again on its old node if it was running. A step that cannot be
// undone is logged.
func (m *serverMove) rollback() {
	for _, name := range m.copiedBackups {
		m.undoStep(m.target, http.MethodDelete, daemonServerPrefix+m.id+daemonBackupFile+fileNameParam+url.QueryEscape(name),
			"removing the copy of backup "+name+" from the new node")
	}
	if m.createdOnTarget {
		// anything else in its backup folder there was not brought by this transfer
		m.undoStep(m.target, http.MethodDelete, daemonServerPrefix+m.id+"?keepBackups=true", "removing it from the new node")
	}
	if m.archived {
		for _, name := range []string{originArchive, originArchivePartial} {
			callQuietly(m.source, http.MethodDelete, originArchivePath(m.id, name))
		}
	}
	if m.pausedSource {
		m.undoStep(m.source, http.MethodPost, daemonServerPrefix+m.id+schedulerResume, "resuming its tasks on the old node")
	}
	if m.wasRunning {
		m.undoStep(m.source, http.MethodPost, daemonServerPrefix+m.id+startAndWait, "starting it again on the old node")
	}
}

// finish removes the server from its old node once the panel points at the new one; its
// backups there go too, they are on the new node now. A failure is only logged: the server
// already runs from the new node, and what is left on the old one is a stray copy.
func (m *serverMove) finish() {
	m.undoStep(m.source, http.MethodDelete, daemonServerPrefix+m.id, "removing it from the old node")
}

// undoStep makes a call that cleans up after the transfer and logs it when the node did not
// do it; a node that no longer has what is removed (404) is fine
func (m *serverMove) undoStep(call nodeCaller, method, path, step string) {
	res, err := call(method, path, nil, nil)
	if err == nil && res != nil && res.StatusCode == http.StatusNotFound {
		utils.CloseResponse(res)
		return
	}
	if err = daemonStepError(step, res, err); err != nil {
		logging.Error.Printf(transferLogFormat, m.id, err)
	}
}

// callQuietly makes a cleanup call whose failure does not matter, such as removing a file
// that may not be there
func callQuietly(call nodeCaller, method, path string) {
	res, err := call(method, path, nil, nil)
	if err == nil {
		utils.CloseResponse(res)
	}
}

// moveServerRecordToNode points the server's database record at the target node. The model
// only writes node_id on create, so it is set here with SQL, and only that column: the rest of
// the record may have been edited while the transfer ran.
func moveServerRecordToNode(db *gorm.DB, identifier string, targetNode *models.Node) error {
	var rawNodeID *uint
	if targetNode.ID != 0 && !targetNode.IsLocal() {
		id := targetNode.ID
		rawNodeID = &id
	}
	return db.Exec("UPDATE servers SET node_id = ? WHERE identifier = ?", rawNodeID, identifier).Error
}

// isDaemonSuccess reports whether a daemon action answered with 200 or 204.
// A missing response without an error counts as success, as it always has.
func isDaemonSuccess(res *http.Response, err error) bool {
	return err == nil && (res == nil || res.StatusCode == http.StatusNoContent || res.StatusCode == http.StatusOK)
}

// readFailureBody reads and closes the body of a failed daemon response.
func readFailureBody(res *http.Response) string {
	if res == nil {
		return "unknown error"
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return string(body)
}
