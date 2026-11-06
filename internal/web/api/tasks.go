package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

type userTaskOperation struct {
	// keys are the arguments the step may carry besides its type
	keys []string
	// scopes are what the user must hold on the server, the same ones the button for that action needs
	scopes []*scopes.Scope
}

// userTaskOperations is what a task made by someone without admin may do. Everything else
// (shell commands, downloads, file writes) stays with administrators, since a task runs on
// the node with nobody watching and those reach beyond the game. A backup step never takes
// taskId: only the scheduler sets it, or a user could name another task and prune its copies
// in the bucket.
var userTaskOperations = map[string]userTaskOperation{
	"stdin":   {keys: []string{"command"}, scopes: []*scopes.Scope{scopes.ScopeServerSendCommand}},
	"sleep":   {keys: []string{"duration"}},
	"start":   {scopes: []*scopes.Scope{scopes.ScopeServerStart}},
	"stop":    {scopes: []*scopes.Scope{scopes.ScopeServerStop}},
	"restart": {scopes: []*scopes.Scope{scopes.ScopeServerStart, scopes.ScopeServerStop}},
	"backup":  {keys: []string{"keep", "offsite", "offsiteKeep"}, scopes: []*scopes.Scope{scopes.ScopeServerBackupCreate}},
}

const (
	maxUserTaskSteps       = 10
	maxUserTaskSleep       = time.Hour
	maxUserTaskCommand     = 512
	maxUserTaskName        = 64
	maxUserTaskDescription = 256
	maxTaskBody            = 64 << 10
)

// cron descriptors a user may pick; @every is left out because it can run every second
var userCronDescriptors = map[string]bool{
	"@yearly": true, "@annually": true, "@monthly": true, "@weekly": true,
	"@daily": true, "@midnight": true, "@hourly": true,
}

// validateUserCron allows a standard five-field schedule or a descriptor, optionally behind
// CRON_TZ= or TZ=. Leaving out the seconds field keeps a task to at most once a minute.
func validateUserCron(spec string) error {
	if spec == "" {
		return nil
	}
	fields := strings.Fields(spec)
	if len(fields) > 0 && (strings.HasPrefix(fields[0], "CRON_TZ=") || strings.HasPrefix(fields[0], "TZ=")) {
		fields = fields[1:]
	}
	if len(fields) == 1 && userCronDescriptors[fields[0]] {
		return nil
	}
	if len(fields) != 5 {
		return errors.New("the schedule must have five fields: minute, hour, day of month, month and day of week")
	}
	return nil
}

// validateUserTask checks a task sent by someone without admin: only the steps above, with
// the permissions each one needs, and a schedule that runs at most once a minute
func validateUserTask(task *skypanel.Task, granted []*scopes.Scope) error {
	if len(task.Name) > maxUserTaskName {
		return fmt.Errorf("the task name can be at most %d characters", maxUserTaskName)
	}
	if len(task.Description) > maxUserTaskDescription {
		return fmt.Errorf("the task description can be at most %d characters", maxUserTaskDescription)
	}
	if err := validateUserCron(task.CronSchedule); err != nil {
		return err
	}
	if len(task.Operations) == 0 || len(task.Operations) > maxUserTaskSteps {
		return fmt.Errorf("a task needs between 1 and %d steps", maxUserTaskSteps)
	}

	for _, op := range task.Operations {
		if err := validateUserTaskOperation(op, granted); err != nil {
			return err
		}
	}
	return nil
}

// validateUserTaskOperation checks one step: its type, arguments and the permissions it needs
func validateUserTaskOperation(op skypanel.ConditionalMetadataType, granted []*scopes.Scope) error {
	allowed, ok := userTaskOperations[op.Type]
	if !ok {
		return fmt.Errorf("only administrators can use %q steps", op.Type)
	}
	if op.If != "" {
		return errors.New("only administrators can use conditions")
	}
	for key := range op.Metadata {
		if !slices.Contains(allowed.keys, key) {
			return fmt.Errorf("%q steps do not take %q", op.Type, key)
		}
	}
	for _, scope := range allowed.scopes {
		if !scopes.ContainsScope(granted, scope) {
			return fmt.Errorf("you need the %s permission for %q steps", scope.Value, op.Type)
		}
	}
	return validateUserTaskStep(op)
}

func validateUserTaskStep(op skypanel.ConditionalMetadataType) error {
	switch op.Type {
	case "stdin":
		command, err := cast.ToStringE(op.Metadata["command"])
		if err != nil || strings.TrimSpace(command) == "" {
			return errors.New("a command step needs the command to send")
		}
		if len(command) > maxUserTaskCommand || strings.ContainsAny(command, "\r\n") {
			return fmt.Errorf("a command must be a single line of at most %d characters", maxUserTaskCommand)
		}
	case "sleep":
		duration, err := cast.ToDurationE(op.Metadata["duration"])
		if err != nil || duration <= 0 || duration > maxUserTaskSleep {
			return errors.New("a wait step can last up to one hour")
		}
	case "backup":
		if _, err := servers.AutoBackupKeep(op.Metadata["keep"]); err != nil {
			return err
		}
		if _, err := servers.ParseOffsiteFrequency(op.Metadata["offsite"]); err != nil {
			return err
		}
		if _, err := servers.OffsiteKeep(op.Metadata["offsiteKeep"]); err != nil {
			return err
		}
	}
	return nil
}

// @Summary Create or edit server task
// @Description Creates the task, or replaces the one with the same id
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Param taskId path string true "Task ID"
// @Param task body skypanel.Task true "Task definition"
// @Tags Daemon Server
// @Router /api/servers/{id}/tasks/{taskId} [put]
// @Security OAuth2Application[server.tasks.create, server.tasks.edit]
func putServerTask(c *gin.Context) {
	if !callerIsAdmin(c) {
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxTaskBody+1))
		if response.HandleError(c, err, http.StatusBadRequest) {
			return
		}
		if len(body) > maxTaskBody {
			response.HandleError(c, errors.New("the task is too large"), http.StatusRequestEntityTooLarge)
			return
		}

		var task skypanel.Task
		if response.HandleError(c, json.Unmarshal(body, &task), http.StatusBadRequest) {
			return
		}
		if response.HandleError(c, validateUserTask(&task, callerScopes(c)), http.StatusBadRequest) {
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
	}

	proxyServerRequest(c)
}

// syncAutomaticBackupsLock keeps two requests for the same list from adding a backup twice
var syncAutomaticBackupsLock sync.Mutex

// syncAutomaticBackups brings the panel's list in line with the node: the backups of scheduled
// tasks, restore points and periodic off-site copies (which only the bucket keeps) are added,
// ones removed by retention are dropped, and every backup gets its size once its archive is
// written. Backups made by hand are recorded when they are made,
// so only automatic ones are added, and one made by hand is only removed when the node reports
// that its archive could not be written. It is best effort: when the node cannot be reached the
// stored list is shown as it is.
func syncAutomaticBackups(ctx context.Context, db *gorm.DB, server *models.Server) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ns := &services.Node{DB: db}
	res, err := ns.CallNodeContext(ctx, &server.Node, http.MethodGet, "/daemon/server/"+server.Identifier+"/backup", nil, nil)
	defer utils.CloseResponse(res)
	if err != nil || res.StatusCode != http.StatusOK {
		return
	}

	var onNode []skypanel.BackupFile
	if err := json.NewDecoder(res.Body).Decode(&onNode); err != nil {
		return
	}

	syncAutomaticBackupsLock.Lock()
	defer syncAutomaticBackupsLock.Unlock()

	bs := &services.Backup{DB: db}
	failed, err := applyNodeBackups(bs, server, onNode)
	if err != nil {
		return
	}
	// the node keeps a mark for each failed backup until it is told the panel dropped it
	for _, name := range failed {
		res, err := ns.CallNodeContext(ctx, &server.Node, http.MethodDelete, daemonServerPrefix+server.Identifier+"/backup"+fileNameParam+name, nil, nil)
		utils.CloseResponse(res)
		if err != nil {
			logging.Error.Printf("Error clearing the failed backup %s on the node: %s", name, err)
		}
	}
}

// applyNodeBackups updates the panel's records of the server's backups from the node's list
// and returns the backups the node reports as failed, whose record it removed
func applyNodeBackups(bs *services.Backup, server *models.Server, onNode []skypanel.BackupFile) ([]string, error) {
	records, err := bs.GetAllForServer(server.Identifier)
	if err != nil {
		return nil, err
	}

	present := make(map[string]skypanel.BackupFile, len(onNode))
	finished := make([]skypanel.BackupFile, 0, len(onNode))
	failed := make([]string, 0)
	for _, f := range onNode {
		if f.Location == servers.BackupFailedLocation {
			failed = append(failed, f.FileName)
			continue
		}
		present[f.FileName] = f
		finished = append(finished, f)
	}
	isFailed := make(map[string]bool, len(failed))
	for _, name := range failed {
		isFailed[name] = true
	}

	known := make(map[string]bool, len(records))
	for _, r := range records {
		known[r.FileName] = true
		file, onDisk := present[r.FileName]
		// a backup made by hand is recorded when it is asked for, before its archive is
		// written; when writing it failed there is no file behind the record
		if (isSystemBackup(r.FileName) && !onDisk) || isFailed[r.FileName] {
			if err := bs.Delete(r.ID); err != nil {
				logging.Error.Printf("Error removing the record of deleted backup %s: %s", r.FileName, err)
			}
			continue
		}
		if onDisk && (file.Size != r.FileSize || file.Location != r.Location) {
			r.FileSize = file.Size
			r.Location = file.Location
			if err := bs.Update(r); err != nil {
				logging.Error.Printf("Error recording the size of backup %s: %s", r.FileName, err)
			}
		}
	}
	addSystemBackups(bs, server, finished, known)
	return failed, nil
}

// addSystemBackups records the backups the node made by itself that the panel does not list yet
func addSystemBackups(bs *services.Backup, server *models.Server, onNode []skypanel.BackupFile, known map[string]bool) {
	for _, f := range onNode {
		if !isSystemBackup(f.FileName) || known[f.FileName] {
			continue
		}
		name := "Automatic backup"
		switch {
		case strings.HasPrefix(f.FileName, servers.RestorePointPrefix):
			name = "Restore point"
		case strings.HasPrefix(f.FileName, servers.OffsiteBackupPrefix):
			name = "Periodic backup (S3)"
		}
		record := &models.Backup{Name: name, FileName: f.FileName, FileSize: f.Size, Location: f.Location, ServerID: server.Identifier, CreatedAt: f.ModTime}
		if err := bs.Create(record); err != nil {
			logging.Error.Printf("Error recording automatic backup %s: %s", f.FileName, err)
		}
	}
}

// isSystemBackup tells backups the node makes by itself (scheduled tasks, restore points, the
// periodic off-site copy), which the panel adds and removes as they come and go, from those made
// by hand. The node lists the periodic copy with Location "offsite" once only the bucket has it,
// so it stays recorded until the retention removes it there.
func isSystemBackup(fileName string) bool {
	return strings.HasPrefix(fileName, servers.AutoBackupPrefix) ||
		strings.HasPrefix(fileName, servers.RestorePointPrefix) ||
		strings.HasPrefix(fileName, servers.OffsiteBackupPrefix)
}
