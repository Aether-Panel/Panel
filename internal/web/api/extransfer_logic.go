package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	transferProgressMutex sync.RWMutex
	transferProgress      = make(map[string]string)
	externalHTTPClient    = utils.NewRestrictedHTTPClient()
)

// serverIdentifierQuery looks a server up by its identifier
const serverIdentifierQuery = "identifier = ?"

// invalidJSONPayloadMsg answers a request whose body is not valid JSON
const invalidJSONPayloadMsg = "Invalid JSON payload"

func sendWebhookReport(serverName, status, details string, isError bool) {
	webhookURL := config.DiscordWebhookExTransfer.Value()
	if webhookURL == "" {
		return
	}

	ds := &services.DiscordService{}
	color := 0x00FF00 // Green for success
	if isError {
		color = 0xFF0000 // Red for error
	}

	title := fmt.Sprintf("ExTransfer: %s", status)
	fields := []services.DiscordEmbedField{
		{Name: "Servidor", Value: serverName, Inline: true},
		{Name: "Estado", Value: status, Inline: true},
	}
	if details != "" {
		fields = append(fields, services.DiscordEmbedField{Name: "Detalles", Value: details, Inline: false})
	}

	err := ds.SendWebhookToURL(webhookURL, title, "Reporte de transferencia externa", color, fields)
	if err != nil {
		logging.Error.Printf("Failed to send ExTransfer webhook: %v", err)
	}
}

func setTransferProgress(serverID, status string) {
	transferProgressMutex.Lock()
	defer transferProgressMutex.Unlock()
	if status == "" {
		delete(transferProgress, serverID)
	} else {
		transferProgress[serverID] = status
	}
}

func getTransferProgress(serverID string) string {
	transferProgressMutex.RLock()
	defer transferProgressMutex.RUnlock()
	return transferProgress[serverID]
}

var ExTransferSalt = "AETHER_FEDERATED_SALT_v1"

var (
	exTransferKeyOnce sync.Once
	exTransferPrivKey ed25519.PrivateKey
)

// exTransferPrivateKey returns this panel's signing key for federated transfers. The other
// panel stores the matching public key in the session, so the key must survive restarts: it
// is generated once and saved in the config file, like panel.token.
func exTransferPrivateKey() ed25519.PrivateKey {
	exTransferKeyOnce.Do(func() {
		if seed, err := base64.StdEncoding.DecodeString(config.ExTransferKey.Value()); err == nil && len(seed) == ed25519.SeedSize {
			exTransferPrivKey = ed25519.NewKeyFromSeed(seed)
			return
		}

		seed := make([]byte, ed25519.SeedSize)
		if _, err := rand.Read(seed); err != nil {
			panic(err)
		}
		if err := config.ExTransferKey.Set(base64.StdEncoding.EncodeToString(seed), true); err != nil {
			logging.Error.Printf("Could not save the federated transfer key, transfers will not survive a restart: %s", err)
		}
		exTransferPrivKey = ed25519.NewKeyFromSeed(seed)
	})
	return exTransferPrivKey
}

func exTransferPublicKey() ed25519.PublicKey {
	return exTransferPrivateKey().Public().(ed25519.PublicKey)
}

type ExTransferAPIError struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
	Retryable bool   `json:"retryable"`
}

type ExValidateReq struct {
	Token           string `json:"token" binding:"required"`
	TargetPublicKey string `json:"target_public_key" binding:"required"`
	ProtocolVer     string `json:"protocol_version" binding:"required"`
}

type ExConsumeReq struct {
	SessionID string `json:"session_id" binding:"required"`
	Signature string `json:"signature" binding:"required"`
}

func RegisterExTransferRoutes(rg *gin.RouterGroup) {
	// Origin endpoints
	// Origin endpoints are mounted per-server in servers.go

	// Destination interacting with Origin
	rg.POST("/validate", validateExTransfer)
	rg.POST("/consume", consumeExTransfer)
	rg.POST("/heartbeat", heartbeatExTransfer)
	rg.POST("/confirm", confirmExTransfer)
	rg.GET("/download", downloadExTransfer)

	// Destination giving up on the transfer
	rg.POST("/cancel", cancelExTransfer)
}

func WellKnownAetherPanel(c *gin.Context) {
	pubKeyB64 := base64.StdEncoding.EncodeToString(exTransferPublicKey())
	c.JSON(200, gin.H{
		"panel_name":          "SkyPanel",
		"version":             "1.0",
		"protocols_supported": []string{"1.0"},
		"public_key":          pubKeyB64,
		"tls_fingerprint":     "AUTO_GENERATED",
		"capabilities": map[string]interface{}{
			"virtualization": []string{"docker"},
			"os":             "linux",
		},
		"timestamp": time.Now().Unix(),
	})
}

// @Summary Create federated transfer session
// @Description Create a new federated transfer session for a server
// @Produce json
// @Param serverId path string true "Server ID"
// @Success 200 {object} map[string]interface{} "Session created"
// @Failure 500 {object} map[string]string "Failed to create session"
// @Tags Federated Transfer
// @Router /api/servers/{serverId}/extransfer/create [post]
// @Security OAuth2Application[server.edit.admin]
func CreateExTransfer(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)
	user := c.MustGet("user").(*models.User)
	db := middleware.GetDatabase(c)

	// Gen token
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	rawToken := hex.EncodeToString(b)

	mac := hmac.New(sha256.New, []byte(ExTransferSalt))
	mac.Write([]byte(rawToken))
	hashedToken := hex.EncodeToString(mac.Sum(nil))

	session := models.ExTransferSession{
		SessionUUID: uuid.NewString(),
		ServerID:    server.Identifier,
		UserID:      user.ID,
		TokenHash:   hashedToken,
		Status:      models.StatusCreated,
		ExpiresAt:   time.Now().Add(15 * time.Minute),
	}

	if err := db.Create(&session).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(200, gin.H{
		"session_id": session.SessionUUID,
		"token":      rawToken,
		"expires_in": 900,
	})
}

// @Summary Validate federated transfer token
// @Description Validate a token received from another panel and establish a session
// @Accept json
// @Produce json
// @Param request body ExValidateReq true "Validation Request"
// @Success 200 {object} map[string]interface{} "Validation successful"
// @Failure 400 {object} ExTransferAPIError "Bad request"
// @Failure 403 {object} ExTransferAPIError "Forbidden"
// @Tags Federated Transfer
// @Router /api/extransfer/validate [post]
func validateExTransfer(c *gin.Context) {
	var req ExValidateReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, ExTransferAPIError{ErrorCode: "BAD_REQUEST", Message: invalidJSONPayloadMsg, Retryable: false})
		return
	}

	req.Token = strings.TrimSpace(req.Token) // se agrego prueba local

	mac := hmac.New(sha256.New, []byte(ExTransferSalt))
	mac.Write([]byte(req.Token))
	hashedToken := hex.EncodeToString(mac.Sum(nil))

	db := middleware.GetDatabase(c)
	var nonce string
	var sessionID string

	err := db.Transaction(func(tx *gorm.DB) error {
		var session models.ExTransferSession
		if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Where("token_hash = ?", hashedToken).Find(&session).Error; err != nil {
			return err
		}

		if session.ID == 0 {
			return errors.New("not_found")
		}

		if session.Status != models.StatusCreated && session.Status != models.StatusFailed {
			return errors.New("invalid_state")
		}
		if time.Now().After(session.ExpiresAt) {
			return errors.New("expired")
		}

		nonce = uuid.NewString()
		exp := time.Now().Add(3 * time.Minute)
		session.CurrentNonce = nonce
		session.NonceExpiresAt = &exp
		session.DestPublicKey = req.TargetPublicKey
		session.Status = models.StatusValidated
		sessionID = session.SessionUUID

		return tx.Save(&session).Error
	})

	if err != nil {
		c.JSON(403, ExTransferAPIError{ErrorCode: "TOKEN_ERROR", Message: err.Error(), Retryable: false})
		return
	}

	// Returning Server Capabilities to Destination
	c.JSON(200, gin.H{
		"session_id": sessionID,
		"nonce":      nonce,
		"capabilities_required": gin.H{
			"virtualization": "docker",
			"cpu_cores_min":  1,
			"ram_mb_min":     1024,
		},
	})
}

// @Summary Consume federated transfer
// @Description Start the migration after validating signature
// @Accept json
// @Produce json
// @Param request body ExConsumeReq true "Consume Request"
// @Success 202 {object} map[string]interface{} "Migration started"
// @Failure 400 {object} ExTransferAPIError "Bad request"
// @Failure 403 {object} ExTransferAPIError "Forbidden"
// @Tags Federated Transfer
// @Router /api/extransfer/consume [post]
func consumeExTransfer(c *gin.Context) {
	var req ExConsumeReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, ExTransferAPIError{ErrorCode: "BAD_REQUEST", Message: invalidJSONPayloadMsg, Retryable: false})
		return
	}

	db := middleware.GetDatabase(c)
	var session models.ExTransferSession

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Where("session_uuid = ?", req.SessionID).Find(&session).Error; err != nil {
			return err
		}

		if session.Status != models.StatusValidated {
			return errors.New("invalid_state")
		}
		if time.Now().After(*session.NonceExpiresAt) {
			return errors.New("nonce_expired")
		}

		// Verify signature
		pubKey, errDec := base64.StdEncoding.DecodeString(session.DestPublicKey)
		sig, errSig := base64.StdEncoding.DecodeString(req.Signature)
		if errDec != nil || errSig != nil || len(pubKey) != ed25519.PublicKeySize {
			return errors.New("invalid_signature_format")
		}

		message := session.CurrentNonce + req.SessionID
		if !ed25519.Verify(pubKey, []byte(message), sig) {
			return errors.New("invalid_signature")
		}

		session.Status = models.StatusMigrating
		// leave the destination time for its first heartbeat, however late it consumed
		session.ExpiresAt = extendedExTransferExpiry(&session, time.Now())
		return tx.Save(&session).Error
	})

	if err != nil {
		c.JSON(403, ExTransferAPIError{ErrorCode: "CONSUME_ERROR", Message: err.Error(), Retryable: false})
		return
	}

	// Trigger async migration process
	go StartDataPlaneMigration(session, db)

	c.JSON(202, gin.H{"status": "MIGRATING", "message": "Stream initialized"})
}

const (
	// exTransferHeartbeatExtension is how far each heartbeat pushes the session's expiry
	exTransferHeartbeatExtension = 5 * time.Minute
	// exTransferMaxLifetime caps how long heartbeats can keep a session alive after it was created
	exTransferMaxLifetime = 6 * time.Hour
	// exTransferHeartbeatEvery is how often the destination tells the origin it is still working
	exTransferHeartbeatEvery = 20 * time.Second
	// exTransferRecentOutcome is how long the panel keeps showing a finished session's outcome
	exTransferRecentOutcome = 15 * time.Minute

	// progress values the destination reports to its own UI besides the step texts
	exTransferProgressDone              = "DONE"
	exTransferProgressCancelled         = "CANCELLED"
	exTransferProgressCancelledByOrigin = "CANCELLED_BY_ORIGIN"
)

// ExSignedReq is what the destination sends to heartbeat, confirm or cancel a session: the
// session and its signature over "<ACTION>:" + session_id, made with the key it gave at validate.
type ExSignedReq struct {
	SessionID string `json:"session_id" binding:"required"`
	Signature string `json:"signature" binding:"required"`
	Reason    string `json:"reason,omitempty"`
}

// exTransferError is a refusal sent back to the other panel, with its HTTP status.
type exTransferError struct {
	status int
	body   ExTransferAPIError
}

func (e *exTransferError) Error() string { return e.body.Message }

func newExTransferError(status int, code, message string) *exTransferError {
	return &exTransferError{status: status, body: ExTransferAPIError{ErrorCode: code, Message: message, Retryable: status >= 500}}
}

var (
	errExSessionNotFound    = newExTransferError(http.StatusNotFound, "NOT_FOUND", "Session not found")
	errExSessionExpired     = newExTransferError(http.StatusForbidden, "SESSION_EXPIRED", "Session expired")
	errExNotMigrating       = newExTransferError(http.StatusForbidden, "INVALID_STATE", "Session not in migrating state")
	errExNotCancellable     = newExTransferError(http.StatusForbidden, "INVALID_STATE", "Session can no longer be cancelled")
	errExInvalidSignature   = newExTransferError(http.StatusForbidden, "INVALID_SIGNATURE", "Invalid signature")
	errExBadSignatureFormat = newExTransferError(http.StatusBadRequest, "INVALID_SIGNATURE_FORMAT", "Invalid signature format")
)

// writeExTransferError answers err to the other panel and reports whether there was one.
func writeExTransferError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	var exErr *exTransferError
	if errors.As(err, &exErr) {
		c.JSON(exErr.status, exErr.body)
		return true
	}
	logging.Error.Printf("ExTransfer request failed: %v", err)
	c.JSON(http.StatusInternalServerError, ExTransferAPIError{ErrorCode: "INTERNAL_ERROR", Message: "Internal error", Retryable: true})
	return true
}

func bindExSignedReq(c *gin.Context) (ExSignedReq, bool) {
	var req ExSignedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ExTransferAPIError{ErrorCode: "BAD_REQUEST", Message: invalidJSONPayloadMsg, Retryable: false})
		return req, false
	}
	if r := []rune(strings.TrimSpace(req.Reason)); len(r) > 500 {
		req.Reason = string(r[:500])
	} else {
		req.Reason = string(r)
	}
	return req, true
}

// findSignedExTransferSession loads the session and checks that the destination signed action
// for it. Pass a query with a locking clause to lock the row inside a transaction.
func findSignedExTransferSession(q *gorm.DB, sessionID, action, signature string) (*models.ExTransferSession, error) {
	var session models.ExTransferSession
	if err := q.Where("session_uuid = ?", sessionID).Limit(1).Find(&session).Error; err != nil {
		return nil, err
	}
	if session.ID == 0 {
		return nil, errExSessionNotFound
	}

	pubKey, errDec := base64.StdEncoding.DecodeString(session.DestPublicKey)
	sig, errSig := base64.StdEncoding.DecodeString(signature)
	if errDec != nil || errSig != nil || len(pubKey) != ed25519.PublicKeySize {
		return nil, errExBadSignatureFormat
	}
	if !ed25519.Verify(pubKey, []byte(action+":"+session.SessionUUID), sig) {
		return nil, errExInvalidSignature
	}
	return &session, nil
}

func lockedForUpdate(tx *gorm.DB) *gorm.DB {
	return tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate})
}

// extendedExTransferExpiry is the expiry a sign of life at now gives the session: never earlier
// than it already is and never later than exTransferMaxLifetime after it was created.
func extendedExTransferExpiry(session *models.ExTransferSession, now time.Time) time.Time {
	next := now.Add(exTransferHeartbeatExtension)
	if limit := session.CreatedAt.Add(exTransferMaxLifetime); next.After(limit) {
		next = limit
	}
	if next.Before(session.ExpiresAt) {
		return session.ExpiresAt
	}
	return next
}

func logExTransfer(tx *gorm.DB, sessionID, action, ip, details string, isError bool) error {
	return tx.Omit(clause.Associations).Create(&models.ExTransferLog{
		SessionID: sessionID,
		Action:    action,
		IPAddress: ip,
		IsError:   isError,
		Details:   details,
	}).Error
}

// reportExTransfer sends the webhook report for a server of this panel without holding up the request.
func reportExTransfer(db *gorm.DB, serverID, status, details string, isError bool) {
	name := serverID
	var names []string
	if err := db.Model(&models.Server{}).Where(serverIdentifierQuery, serverID).Limit(1).Pluck("name", &names).Error; err == nil && len(names) == 1 && names[0] != "" {
		name = names[0]
	}
	go sendWebhookReport(name, status, details, isError)
}

// removeOriginArchive deletes the archive a finished or cancelled session left in the server's
// files. It is a variable so tests can keep away from the node.
var removeOriginArchive = func(db *gorm.DB, serverID string) {
	var server models.Server
	if err := db.Preload("Node").Where(serverIdentifierQuery, serverID).First(&server).Error; err != nil {
		return
	}
	ns := &services.Node{DB: db}
	res, err := ns.CallNode(&server.Node, "DELETE", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)
	if err != nil {
		logging.Error.Printf("Could not remove the transfer archive of %s: %v", serverID, err)
		return
	}
	if res.Body != nil {
		_ = res.Body.Close()
	}
}

// downloads being streamed per session, so a cancel can cut them off
type originDownload struct{ cancel context.CancelFunc }

var (
	originDownloadsMu sync.Mutex
	originDownloads   = make(map[string]map[*originDownload]struct{})
)

func trackOriginDownload(sessionID string, cancel context.CancelFunc) (untrack func()) {
	d := &originDownload{cancel: cancel}
	originDownloadsMu.Lock()
	if originDownloads[sessionID] == nil {
		originDownloads[sessionID] = make(map[*originDownload]struct{})
	}
	originDownloads[sessionID][d] = struct{}{}
	originDownloadsMu.Unlock()

	return func() {
		originDownloadsMu.Lock()
		delete(originDownloads[sessionID], d)
		if len(originDownloads[sessionID]) == 0 {
			delete(originDownloads, sessionID)
		}
		originDownloadsMu.Unlock()
	}
}

func stopOriginDownloads(sessionID string) {
	originDownloadsMu.Lock()
	defer originDownloadsMu.Unlock()
	for d := range originDownloads[sessionID] {
		d.cancel()
	}
}

// @Summary Heartbeat for federated transfer
// @Description The destination says it is still transferring. Keeps the session from expiring (up to 6 hours after it was created) and answers its status, so the destination stops when it was cancelled.
// @Accept json
// @Produce json
// @Param request body ExSignedReq true "Session and signature of 'HEARTBEAT:' + session_id"
// @Success 200 {object} map[string]interface{} "Session status"
// @Failure 400 {object} ExTransferAPIError "Bad request"
// @Failure 403 {object} ExTransferAPIError "Forbidden"
// @Failure 404 {object} ExTransferAPIError "Not found"
// @Tags Federated Transfer
// @Router /api/extransfer/heartbeat [post]
func heartbeatExTransfer(c *gin.Context) {
	req, ok := bindExSignedReq(c)
	if !ok {
		return
	}

	db := middleware.GetDatabase(c)
	var status models.TransferStatus
	var expiresAt time.Time

	err := db.Transaction(func(tx *gorm.DB) error {
		session, err := findSignedExTransferSession(lockedForUpdate(tx), req.SessionID, "HEARTBEAT", req.Signature)
		if err != nil {
			return err
		}
		status, expiresAt = session.Status, session.ExpiresAt

		switch session.Status {
		case models.StatusCancelled, models.StatusFailed:
			// answered so the destination stops
			return nil
		case models.StatusMigrating:
		default:
			return errExNotMigrating
		}

		now := time.Now()
		if now.After(session.ExpiresAt) {
			return errExSessionExpired
		}
		expiresAt = extendedExTransferExpiry(session, now)
		return tx.Model(session).Updates(map[string]interface{}{
			"last_heartbeat_at": now,
			"expires_at":        expiresAt,
		}).Error
	})
	if writeExTransferError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status, "expires_at": expiresAt})
}

// @Summary Confirm federated transfer
// @Description The destination received and unpacked the data. Completes the session; it cannot be downloaded again. Confirming twice is harmless.
// @Accept json
// @Produce json
// @Param request body ExSignedReq true "Session and signature of 'CONFIRM:' + session_id"
// @Success 200 {object} map[string]string "Completed"
// @Failure 400 {object} ExTransferAPIError "Bad request"
// @Failure 403 {object} ExTransferAPIError "Forbidden"
// @Failure 404 {object} ExTransferAPIError "Not found"
// @Tags Federated Transfer
// @Router /api/extransfer/confirm [post]
func confirmExTransfer(c *gin.Context) {
	req, ok := bindExSignedReq(c)
	if !ok {
		return
	}

	db := middleware.GetDatabase(c)
	var serverID string
	completed := false

	err := db.Transaction(func(tx *gorm.DB) error {
		session, err := findSignedExTransferSession(lockedForUpdate(tx), req.SessionID, "CONFIRM", req.Signature)
		if err != nil {
			return err
		}
		if session.Status == models.StatusCompleted {
			return nil
		}
		if session.Status != models.StatusMigrating {
			return errExNotMigrating
		}
		now := time.Now()
		if now.After(session.ExpiresAt) {
			return errExSessionExpired
		}

		if err := tx.Model(session).Updates(map[string]interface{}{
			"status":            models.StatusCompleted,
			"last_heartbeat_at": now,
		}).Error; err != nil {
			return err
		}
		serverID, completed = session.ServerID, true
		return logExTransfer(tx, session.SessionUUID, "CONFIRM", c.ClientIP(), "The destination confirmed it received the server", false)
	})
	if writeExTransferError(c, err) {
		return
	}

	if completed {
		reportExTransfer(db, serverID, "Completada (Origen)", "El panel de destino confirmó que recibió los datos del servidor.", false)
		go removeOriginArchive(db, serverID)
	}
	c.JSON(http.StatusOK, gin.H{"status": models.StatusCompleted})
}

// @Summary Cancel federated transfer
// @Description The destination gives up on the transfer. The session is cancelled and cannot be used again. Cancelling twice is harmless.
// @Accept json
// @Produce json
// @Param request body ExSignedReq true "Session, signature of 'CANCEL:' + session_id and an optional reason"
// @Success 200 {object} map[string]string "Cancelled"
// @Failure 400 {object} ExTransferAPIError "Bad request"
// @Failure 403 {object} ExTransferAPIError "Forbidden"
// @Failure 404 {object} ExTransferAPIError "Not found"
// @Tags Federated Transfer
// @Router /api/extransfer/cancel [post]
func cancelExTransfer(c *gin.Context) {
	req, ok := bindExSignedReq(c)
	if !ok {
		return
	}

	db := middleware.GetDatabase(c)
	var serverID, sessionID string
	cancelled := false

	err := db.Transaction(func(tx *gorm.DB) error {
		session, err := findSignedExTransferSession(lockedForUpdate(tx), req.SessionID, "CANCEL", req.Signature)
		if err != nil {
			return err
		}
		switch session.Status {
		case models.StatusCancelled:
			return nil
		case models.StatusValidated, models.StatusMigrating, models.StatusFailed:
		default:
			return errExNotCancellable
		}
		if time.Now().After(session.ExpiresAt) {
			return errExSessionExpired
		}

		if err := tx.Model(session).Update("status", models.StatusCancelled).Error; err != nil {
			return err
		}
		serverID, sessionID, cancelled = session.ServerID, session.SessionUUID, true
		details := "The destination cancelled the transfer"
		if req.Reason != "" {
			details += ": " + req.Reason
		}
		return logExTransfer(tx, session.SessionUUID, "CANCEL", c.ClientIP(), details, true)
	})
	if writeExTransferError(c, err) {
		return
	}

	if cancelled {
		stopOriginDownloads(sessionID)
		details := "El panel de destino canceló la transferencia."
		if req.Reason != "" {
			details += " Motivo: " + req.Reason
		}
		reportExTransfer(db, serverID, "Cancelada (Destino)", details, true)
		go removeOriginArchive(db, serverID)
	}
	c.JSON(http.StatusOK, gin.H{"status": models.StatusCancelled})
}

// @Summary Download federated transfer data
// @Description Download the server archive for the transfer session
// @Accept json
// @Produce octet-stream
// @Param session_id query string true "Session ID"
// @Param signature query string true "Signature of 'DOWNLOAD:' + session_id"
// @Success 200 {file} binary "Server archive"
// @Failure 400 {object} ExTransferAPIError "Bad request"
// @Failure 403 {object} ExTransferAPIError "Forbidden"
// @Failure 404 {object} ExTransferAPIError "Not found"
// @Tags Federated Transfer
// @Router /api/extransfer/download [get]
func downloadExTransfer(c *gin.Context) {
	sessionID := c.Query("session_id")
	signature := c.Query("signature")

	if sessionID == "" || signature == "" {
		c.JSON(400, ExTransferAPIError{ErrorCode: "BAD_REQUEST", Message: "Missing session_id or signature", Retryable: false})
		return
	}

	db := middleware.GetDatabase(c)
	session, err := findSignedExTransferSession(db, sessionID, "DOWNLOAD", signature)
	if writeExTransferError(c, err) {
		return
	}

	// tracked before the state check: a cancel that lands after it cuts the stream off
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	defer trackOriginDownload(session.SessionUUID, cancel)()

	var current models.ExTransferSession
	if err := db.Select("status", "expires_at").Where("id = ?", session.ID).First(&current).Error; err != nil {
		writeExTransferError(c, err)
		return
	}
	if current.Status != models.StatusMigrating {
		writeExTransferError(c, errExNotMigrating)
		return
	}
	if time.Now().After(current.ExpiresAt) {
		writeExTransferError(c, errExSessionExpired)
		return
	}

	// Get server
	var server models.Server
	if err := db.Preload("Node").Where(serverIdentifierQuery, session.ServerID).First(&server).Error; err != nil {
		c.JSON(500, ExTransferAPIError{ErrorCode: "INTERNAL_ERROR", Message: "Failed to get server", Retryable: false})
		return
	}

	ns := &services.Node{DB: db}
	downloadRes, err := ns.CallNodeContext(ctx, &server.Node, "GET", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)
	if err != nil || downloadRes.StatusCode != 200 {
		if err == nil {
			_ = downloadRes.Body.Close()
		}
		c.JSON(500, ExTransferAPIError{ErrorCode: "DAEMON_ERROR", Message: "Failed to get file from daemon", Retryable: true})
		return
	}
	defer downloadRes.Body.Close()

	c.Header("Content-Disposition", "attachment; filename=transfer.tar.gz")
	c.Header(headerContentType, "application/octet-stream")

	_, err = io.Copy(c.Writer, downloadRes.Body)
	if err != nil {
		logging.Error.Printf("Error streaming file to client: %v", err)
	}
}

func StartDataPlaneMigration(session models.ExTransferSession, db *gorm.DB) {
	logging.Info.Printf("Starting data plane migration for session %s", session.SessionUUID)

	var server models.Server
	if err := db.Preload("Node").Where(serverIdentifierQuery, session.ServerID).First(&server).Error; err != nil {
		logging.Error.Printf("Failed to get server for migration: %v", err)
		return
	}

	ns := &services.Node{DB: db}

	// Clean up existing archive if it exists from previous attempts
	_, _ = ns.CallNode(&server.Node, "DELETE", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)

	// Archive files
	archiveBody := bytes.NewReader([]byte(`["*"]`))
	headersArch := http.Header{}
	headersArch.Set(headerContentType, contentTypeJSON)
	res, err := ns.CallNode(&server.Node, "POST", fmt.Sprintf("/daemon/server/%s/archive/transfer.tar.gz", server.Identifier), io.NopCloser(archiveBody), headersArch)
	if err != nil || (res != nil && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK) {
		logging.Error.Printf("Failed to archive server %s: %v", server.Identifier, err)
		// a session cancelled meanwhile stays cancelled
		db.Model(&models.ExTransferSession{}).Where("id = ? AND status = ?", session.ID, models.StatusMigrating).Update("status", models.StatusFailed)
		sendWebhookReport(server.Name, "Fallida (Origen)", "Fallo al comprimir los archivos en el servidor de origen.", true)
		return
	}

	// cancelled while the archive was being made: nobody will download it
	var current models.ExTransferSession
	if err := db.Select("status").Where("id = ?", session.ID).First(&current).Error; err == nil && current.Status != models.StatusMigrating {
		removeOriginArchive(db, server.Identifier)
		return
	}

	logging.Info.Printf("Archive created for session %s", session.SessionUUID)
}

type ExPullReq struct {
	OriginURL string `json:"origin_url" binding:"required"`
	Token     string `json:"token" binding:"required"`
}

// pulls running on this panel (destination side), so the owner can cancel them
type runningPull struct{ cancel context.CancelCauseFunc }

var (
	runningPullsMu sync.Mutex
	runningPulls   = make(map[string]*runningPull)

	errPullCancelledByOwner  = errors.New("cancelled by the server owner")
	errPullCancelledByOrigin = errors.New("cancelled by the origin panel")
)

// startPull registers a pull for the server, or reports false when one is already running.
func startPull(serverID string) (ctx context.Context, finish func(), ok bool) {
	runningPullsMu.Lock()
	defer runningPullsMu.Unlock()
	if runningPulls[serverID] != nil {
		return nil, nil, false
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	p := &runningPull{cancel: cancel}
	runningPulls[serverID] = p

	return ctx, func() {
		runningPullsMu.Lock()
		if runningPulls[serverID] == p {
			delete(runningPulls, serverID)
		}
		runningPullsMu.Unlock()
		cancel(nil)
	}, true
}

// cancelPull stops the server's running pull and reports whether there was one.
func cancelPull(serverID string) bool {
	runningPullsMu.Lock()
	defer runningPullsMu.Unlock()
	p := runningPulls[serverID]
	if p == nil {
		return false
	}
	p.cancel(errPullCancelledByOwner)
	return true
}

func isPullRunning(serverID string) bool {
	runningPullsMu.Lock()
	defer runningPullsMu.Unlock()
	return runningPulls[serverID] != nil
}

func pullExTransfer(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)
	var req ExPullReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": invalidJSONPayloadMsg})
		return
	}

	req.Token = strings.TrimSpace(req.Token)
	req.OriginURL = strings.TrimSpace(req.OriginURL)

	db := middleware.GetDatabase(c)

	ctx, finish, ok := startPull(server.Identifier)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "A transfer is already running for this server"})
		return
	}

	setTransferProgress(server.Identifier, "Iniciando...")
	go func() {
		defer finish()
		performPullTransferAsync(ctx, server, req.OriginURL, req.Token, db)
	}()

	c.JSON(202, gin.H{"status": "ACCEPTED", "message": "Pull process initiated"})
}

func getExTransferStatus(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)
	res := gin.H{
		"status":  getTransferProgress(server.Identifier),
		"pulling": isPullRunning(server.Identifier),
	}
	if origin := originSessionSummary(middleware.GetDatabase(c), server.Identifier); origin != nil {
		res["origin"] = origin
	}
	c.JSON(200, res)
}

// originSessionSummary describes the server's newest session created by this panel, while it is
// live or shortly after it ended, so the owner can follow and cancel it.
func originSessionSummary(db *gorm.DB, serverID string) gin.H {
	if db == nil {
		return nil
	}
	var session models.ExTransferSession
	if err := db.Where("server_id = ?", serverID).Order("id DESC").Limit(1).Find(&session).Error; err != nil || session.ID == 0 {
		return nil
	}

	now := time.Now()
	expired := now.After(session.ExpiresAt)
	switch session.Status {
	case models.StatusCreated, models.StatusValidated, models.StatusMigrating:
		if expired && now.Sub(session.ExpiresAt) > exTransferRecentOutcome {
			return nil
		}
	default:
		if now.Sub(session.UpdatedAt) > exTransferRecentOutcome {
			return nil
		}
	}

	return gin.H{
		"session_id":        session.SessionUUID,
		"status":            session.Status,
		"expired":           expired,
		"expires_at":        session.ExpiresAt,
		"last_heartbeat_at": session.LastHeartbeatAt,
		"updated_at":        session.UpdatedAt,
	}
}

// exTransferDownloadClient fetches the archive. Same restrictions as externalHTTPClient, but
// without its overall timeout, which would cut any download longer than 30 seconds; the
// transfer's context and heartbeats bound it instead.
var exTransferDownloadClient = func() *http.Client {
	client := utils.NewRestrictedHTTPClient()
	client.Timeout = 0
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = time.Minute
	}
	return client
}()

// originSession is the destination's handle on a session of the origin panel: it signs and
// sends the heartbeat, confirm and cancel calls.
type originSession struct {
	client *http.Client
	base   *url.URL
	id     string
	key    ed25519.PrivateKey
}

type originAnswer struct {
	Status    string `json:"status"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

func (o *originSession) post(ctx context.Context, path, action, reason string) (int, originAnswer, error) {
	var answer originAnswer
	sig := ed25519.Sign(o.key, []byte(action+":"+o.id))
	body, err := json.Marshal(ExSignedReq{SessionID: o.id, Signature: base64.StdEncoding.EncodeToString(sig), Reason: reason})
	if err != nil {
		return 0, answer, err
	}

	endpoint := o.base.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return 0, answer, err
	}
	req.Header.Set(headerContentType, contentTypeJSON)

	resp, err := o.client.Do(req)
	if err != nil {
		return 0, answer, err
	}
	defer resp.Body.Close()
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&answer)
	return resp.StatusCode, answer, nil
}

// heartbeat reports whether the origin still wants the transfer. Network trouble and server
// errors count as yes (the next beat tries again); a refusal or a cancelled session as no.
func (o *originSession) heartbeat(ctx context.Context) bool {
	code, answer, err := o.post(ctx, "/api/extransfer/heartbeat", "HEARTBEAT", "")
	switch {
	case err != nil || code >= 500:
		return true
	case code >= 400:
		logging.Info.Printf("Origin refused the heartbeat of session %s: %d %s", o.id, code, answer.ErrorCode)
		return false
	default:
		return answer.Status != string(models.StatusCancelled) && answer.Status != string(models.StatusFailed)
	}
}

// confirm tells the origin the data arrived, retrying a couple of times on network trouble.
func (o *originSession) confirm() error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		code, answer, err := o.post(ctx, "/api/extransfer/confirm", "CONFIRM", "")
		cancel()
		switch {
		case err != nil:
			lastErr = err
		case code >= 500:
			lastErr = fmt.Errorf("origin answered %d", code)
		case code >= 400:
			return fmt.Errorf("origin refused the confirmation: %d %s", code, answer.ErrorCode)
		default:
			return nil
		}
	}
	return lastErr
}

// notifyCancel tells the origin, best effort, that this panel gave up on the session.
func (o *originSession) notifyCancel(reason string) {
	if o == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	code, answer, err := o.post(ctx, "/api/extransfer/cancel", "CANCEL", reason)
	if err != nil || code >= 300 {
		logging.Info.Printf("Could not tell the origin that session %s was cancelled: %v %d %s", o.id, err, code, answer.ErrorCode)
	}
}

// keepOriginAlive heartbeats every interval until ctx ends. When the origin no longer wants
// the transfer it aborts the pull with errPullCancelledByOrigin.
func keepOriginAlive(ctx context.Context, o *originSession, every time.Duration, abort context.CancelCauseFunc) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !o.heartbeat(ctx) && ctx.Err() == nil {
				abort(errPullCancelledByOrigin)
				return
			}
		}
	}
}

func performPullTransferAsync(parent context.Context, server *models.Server, originURL, token string, db *gorm.DB) {
	ctx, abort := context.WithCancelCause(parent)
	defer abort(nil)

	var lastStep string
	sendStep := func(msg string) {
		lastStep = msg
		setTransferProgress(server.Identifier, msg)
		if strings.HasPrefix(msg, "ERROR:") {
			sendWebhookReport(server.Name, "Fallida", msg, true)
		}
	}
	defer func() {
		final := lastStep
		time.AfterFunc(10*time.Second, func() { clearTransferProgressIf(server.Identifier, final) })
	}()

	ns := &services.Node{DB: db}
	archivePath := fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier)
	var origin *originSession
	archiveTouched := false

	// fail ends the pull early: it removes the partial archive, tells the origin (unless the
	// origin is the one that cancelled) and reports a cancel or errMsg to the owner.
	fail := func(errMsg string) {
		if archiveTouched {
			deleteTransferArchive(ns, server, archivePath)
		}
		switch cause := context.Cause(ctx); {
		case errors.Is(cause, errPullCancelledByOrigin):
			logging.Info.Printf("Pull transfer for server %s was cancelled by the origin", server.Identifier)
			sendStep(exTransferProgressCancelledByOrigin)
			sendWebhookReport(server.Name, "Cancelada (Origen)", "El panel de origen canceló la transferencia.", true)
		case ctx.Err() != nil:
			logging.Info.Printf("Pull transfer for server %s was cancelled by its owner", server.Identifier)
			origin.notifyCancel("cancelled by the owner on the destination panel")
			sendStep(exTransferProgressCancelled)
			sendWebhookReport(server.Name, "Cancelada", "El propietario canceló la transferencia desde el panel de destino.", true)
		default:
			origin.notifyCancel(strings.TrimPrefix(errMsg, "ERROR: "))
			sendStep(errMsg)
		}
	}

	logging.Info.Printf("Starting pull transfer for server %s from %s", server.Identifier, originURL)
	sendStep("Validando conexión con el panel de origen...")

	baseURL, err := parseOriginURL(originURL)
	if err != nil {
		logging.Error.Printf("Invalid origin URL %s: %v", originURL, err)
		fail("ERROR: La URL de origen no es válida o apunta a una dirección no permitida")
		return
	}

	originAPI := func(path string) *url.URL {
		return baseURL.ResolveReference(&url.URL{Path: path})
	}
	postOrigin := func(path string, body interface{}) (*http.Response, error) {
		return postToOrigin(ctx, originAPI(path), body)
	}

	// 2. Validate token and get nonce
	validateRes, errMsg := validateOriginToken(postOrigin, token)
	if errMsg != "" {
		fail(errMsg)
		return
	}
	origin = &originSession{client: externalHTTPClient, base: baseURL, id: validateRes.SessionID, key: exTransferPrivateKey()}

	sendStep("Iniciando la transferencia en el servidor origen...")
	// 3. Consume transfer
	if errMsg := consumeOriginSession(postOrigin, validateRes); errMsg != "" {
		fail(errMsg)
		return
	}

	// 4. From here on the origin waits for heartbeats, and stops us through them
	beatsCtx, stopBeats := context.WithCancel(ctx)
	defer stopBeats()
	beatsDone := make(chan struct{})
	go func() {
		defer close(beatsDone)
		keepOriginAlive(beatsCtx, origin, exTransferHeartbeatEvery, abort)
	}()
	stopHeartbeats := func() {
		stopBeats()
		<-beatsDone
	}
	defer stopHeartbeats()

	sendStep("Esperando a que el origen comprima los archivos...")

	// 5. Download file — retry until origin is ready (archive may take time to create)
	dlSig := ed25519.Sign(exTransferPrivateKey(), []byte("DOWNLOAD:"+validateRes.SessionID))
	dlURL := *originAPI("/api/extransfer/download")
	dlURL.RawQuery = url.Values{
		"session_id": {validateRes.SessionID},
		"signature":  {base64.StdEncoding.EncodeToString(dlSig)},
	}.Encode()

	resp, errMsg := waitForOriginArchive(ctx, dlURL.String(), sendStep)
	if resp == nil {
		fail(errMsg)
		return
	}
	defer resp.Body.Close()

	sendStep("Subiendo archivos al daemon local (esto puede tardar)...")

	// 6. Stream to daemon
	archiveTouched = true
	if !uploadTransferArchive(ctx, ns, server, archivePath, resp.Body) {
		fail("ERROR: Error al subir archivo al daemon")
		return
	}

	if ctx.Err() != nil {
		fail("")
		return
	}

	sendStep("Descomprimiendo archivos...")
	// 7. Extract on daemon
	logging.Info.Printf("Extracting files on daemon for server %s", server.Identifier)
	if errMsg := extractTransferArchive(ctx, ns, server); errMsg != "" {
		fail(errMsg)
		return
	}

	sendStep("Limpiando archivos temporales...")
	// Clean up transfer file on daemon
	deleteTransferArchive(ns, server, archivePath)

	// 8. The data is here: close the session on the origin so it cannot be downloaded again
	stopHeartbeats()
	if err := origin.confirm(); err != nil {
		logging.Error.Printf("Could not confirm session %s to the origin: %v", validateRes.SessionID, err)
	}

	logging.Info.Printf("Pull transfer for server %s completed successfully", server.Identifier)
	sendWebhookReport(server.Name, "Completada", "La transferencia externa finalizó correctamente.", false)
	sendStep(exTransferProgressDone)
}

// originPoster sends a JSON body to one of the origin's federated transfer routes
type originPoster func(path string, body interface{}) (*http.Response, error)

func postToOrigin(ctx context.Context, target *url.URL, body interface{}) (*http.Response, error) {
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerContentType, contentTypeJSON)
	return externalHTTPClient.Do(req)
}

// originValidateResponse is the session the origin opens for a valid token
type originValidateResponse struct {
	SessionID string `json:"session_id"`
	Nonce     string `json:"nonce"`
}

// validateOriginToken presents the token to the origin; errMsg is what to report when the
// origin does not accept it
func validateOriginToken(postOrigin originPoster, token string) (validateRes originValidateResponse, errMsg string) {
	pubKeyB64 := base64.StdEncoding.EncodeToString(exTransferPublicKey())
	resp, err := postOrigin("/api/extransfer/validate", map[string]string{
		"token":             token,
		"target_public_key": pubKeyB64,
		"protocol_version":  "1.0",
	})
	if err != nil {
		logging.Error.Printf("Failed to call validate on origin: %v", err)
		return validateRes, "ERROR: Fallo de red al conectar con origen"
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		logging.Error.Printf("Origin validate failed with status %d: %s", resp.StatusCode, string(body))
		return validateRes, fmt.Sprintf("ERROR: Token inválido o expirado (Status %d)", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&validateRes); err != nil {
		logging.Error.Printf("Failed to decode validate response: %v", err)
		return validateRes, "ERROR: Respuesta inválida del origen"
	}

	if _, err := uuid.Parse(validateRes.SessionID); err != nil {
		logging.Error.Printf("Origin returned invalid session_id: %v", err)
		return validateRes, "ERROR: El origen devolvió un ID de sesión inválido"
	}
	return validateRes, ""
}

// consumeOriginSession asks the origin to start the transfer, signing its nonce; errMsg is
// what to report when it refuses
func consumeOriginSession(postOrigin originPoster, validateRes originValidateResponse) (errMsg string) {
	message := validateRes.Nonce + validateRes.SessionID
	sig := ed25519.Sign(exTransferPrivateKey(), []byte(message))
	resp, err := postOrigin("/api/extransfer/consume", map[string]string{
		"session_id": validateRes.SessionID,
		"signature":  base64.StdEncoding.EncodeToString(sig),
	})
	if err != nil {
		logging.Error.Printf("Failed to call consume on origin: %v", err)
		return "ERROR: Fallo al iniciar transferencia en origen"
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		logging.Error.Printf("Origin consume failed with status %d: %s", resp.StatusCode, string(body))
		return "ERROR: El origen rechazó la transferencia"
	}
	return ""
}

// waitForOriginArchive polls the origin until it serves the archive. A nil response means the
// pull stops with errMsg, which is empty when ctx was cancelled
func waitForOriginArchive(ctx context.Context, dlURL string, sendStep func(string)) (resp *http.Response, errMsg string) {
	const maxWait = 90 * time.Second
	const pollInterval = 5 * time.Second
	deadline := time.Now().Add(maxWait)

	for {
		select {
		case <-ctx.Done():
			return nil, ""
		case <-time.After(pollInterval):
		}
		sendStep("Descargando paquete de datos desde el origen...")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
		if err == nil {
			resp, err = exTransferDownloadClient.Do(req)
		}
		if err != nil {
			logging.Error.Printf("Failed to call download on origin: %v", err)
			return nil, "ERROR: Error de red al descargar paquete"
		}
		if resp.StatusCode == 200 {
			return resp, ""
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// the origin refuses the session (cancelled, failed or expired): waiting will not help
			logging.Error.Printf("Origin refused the download with status %d", resp.StatusCode)
			return nil, fmt.Sprintf("ERROR: El origen rechazó la descarga (Status %d)", resp.StatusCode)
		}
		if time.Now().After(deadline) {
			logging.Error.Printf("Origin download timed out after %s", maxWait)
			return nil, fmt.Sprintf("ERROR: El origen no preparó el paquete a tiempo (timeout %s)", maxWait)
		}
		logging.Info.Printf("Origin not ready yet (status %d), retrying...", resp.StatusCode)
	}
}

// uploadTransferArchive streams the downloaded archive to the local daemon
func uploadTransferArchive(ctx context.Context, ns *services.Node, server *models.Server, archivePath string, archive io.ReadCloser) bool {
	headersTransfer := http.Header{}
	headersTransfer.Set(headerContentType, "application/octet-stream")

	uploadRes, err := ns.CallNodeContext(ctx, &server.Node, "PUT", archivePath, archive, headersTransfer)
	if err != nil || uploadRes.StatusCode != http.StatusNoContent {
		var body []byte
		if uploadRes != nil && uploadRes.Body != nil {
			body, _ = io.ReadAll(io.LimitReader(uploadRes.Body, 64<<10))
			uploadRes.Body.Close()
		}
		logging.Error.Printf("Failed to upload archive to daemon: %v %s", err, string(body))
		return false
	}
	if uploadRes.Body != nil {
		uploadRes.Body.Close()
	}
	return true
}

// extractTransferArchive unpacks the archive into the server folder; errMsg is what to report
// when the daemon could not
func extractTransferArchive(ctx context.Context, ns *services.Node, server *models.Server) (errMsg string) {
	extractRes, err := ns.CallNodeContext(ctx, &server.Node, "POST", fmt.Sprintf("/daemon/server/%s/extract/transfer.tar.gz?destination=.", server.Identifier), nil, nil)
	if err != nil || (extractRes != nil && extractRes.StatusCode != http.StatusNoContent && extractRes.StatusCode != http.StatusOK) {
		var body string
		if extractRes != nil && extractRes.Body != nil {
			b, _ := io.ReadAll(io.LimitReader(extractRes.Body, 64<<10))
			body = string(b)
			extractRes.Body.Close()
		}
		logging.Error.Printf("Failed to extract on daemon: %v, body: %s", err, body)
		return fmt.Sprintf("ERROR: Error al descomprimir en el destino: %s", body)
	}
	if extractRes != nil && extractRes.Body != nil {
		extractRes.Body.Close()
	}
	return ""
}

// deleteTransferArchive removes the transfer archive from the server folder, best effort
func deleteTransferArchive(ns *services.Node, server *models.Server, archivePath string) {
	if res, err := ns.CallNode(&server.Node, "DELETE", archivePath, nil, nil); err == nil && res.Body != nil {
		_ = res.Body.Close()
	}
}

// clearTransferProgressIf clears the server's progress unless something newer replaced it.
func clearTransferProgressIf(serverID, expected string) {
	transferProgressMutex.Lock()
	defer transferProgressMutex.Unlock()
	if transferProgress[serverID] == expected {
		delete(transferProgress, serverID)
	}
}

func parseOriginURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	if err := utils.ValidateExternalURL(rawURL); err != nil {
		return nil, err
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}

// @Summary Cancel federated transfer from the panel
// @Description Cancels the server's live transfer sessions created by this panel (origin side) and stops a pull running on this panel (destination side)
// @Produce json
// @Param serverId path string true "Server ID"
// @Success 200 {object} map[string]interface{} "What was cancelled"
// @Failure 409 {object} ExTransferAPIError "No transfer in progress"
// @Tags Federated Transfer
// @Router /api/servers/{serverId}/extransfer/cancel [post]
// @Security OAuth2Application[server.edit.admin]
func cancelExTransferByUser(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)
	db := middleware.GetDatabase(c)

	who := "the server owner"
	if u, ok := c.Get("user"); ok {
		if user, ok := u.(*models.User); ok && user != nil {
			who = user.Username
		}
	}

	pullCancelled := cancelPull(server.Identifier)

	var cancelledSessions []string
	err := db.Transaction(func(tx *gorm.DB) error {
		var sessions []models.ExTransferSession
		if err := lockedForUpdate(tx).
			Where("server_id = ? AND status IN ? AND expires_at > ?", server.Identifier,
				[]models.TransferStatus{models.StatusCreated, models.StatusValidated, models.StatusMigrating, models.StatusFailed}, time.Now()).
			Find(&sessions).Error; err != nil {
			return err
		}
		for i := range sessions {
			if err := tx.Model(&sessions[i]).Update("status", models.StatusCancelled).Error; err != nil {
				return err
			}
			if err := logExTransfer(tx, sessions[i].SessionUUID, "CANCEL_BY_OWNER", c.ClientIP(), "Cancelled from the panel by "+who, true); err != nil {
				return err
			}
			cancelledSessions = append(cancelledSessions, sessions[i].SessionUUID)
		}
		return nil
	})
	if err != nil {
		logging.Error.Printf("Could not cancel the transfer sessions of %s: %v", server.Identifier, err)
		c.JSON(http.StatusInternalServerError, ExTransferAPIError{ErrorCode: "INTERNAL_ERROR", Message: "Could not cancel the transfer", Retryable: true})
		return
	}

	if !pullCancelled && len(cancelledSessions) == 0 {
		c.JSON(http.StatusConflict, ExTransferAPIError{ErrorCode: "NO_ACTIVE_TRANSFER", Message: "There is no transfer in progress for this server", Retryable: false})
		return
	}

	if len(cancelledSessions) > 0 {
		for _, id := range cancelledSessions {
			stopOriginDownloads(id)
		}
		go sendWebhookReport(server.Name, "Cancelada (Origen)", "El propietario canceló la transferencia desde el panel de origen.", true)
		go removeOriginArchive(db, server.Identifier)
	}

	c.JSON(http.StatusOK, gin.H{"cancelled_pull": pullCancelled, "cancelled_sessions": len(cancelledSessions)})
}
