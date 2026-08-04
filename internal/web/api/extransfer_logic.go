package api

import (
	"bytes"
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
)

var (
	transferProgressMutex sync.RWMutex
	transferProgress      = make(map[string]string)
	externalHTTPClient    = utils.NewRestrictedHTTPClient()
)

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
	ExTransferPrivateKey ed25519.PrivateKey
	ExTransferPublicKey  ed25519.PublicKey
)

func init() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	ExTransferPrivateKey = priv
	ExTransferPublicKey = pub
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

	// User cancellation
	rg.POST("/cancel", cancelExTransfer)
}

func WellKnownAetherPanel(c *gin.Context) {
	pubKeyB64 := base64.StdEncoding.EncodeToString(ExTransferPublicKey)
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
		c.JSON(400, ExTransferAPIError{ErrorCode: "BAD_REQUEST", Message: "Invalid JSON payload", Retryable: false})
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
		if err := tx.Raw("SELECT * FROM ex_transfer_sessions WHERE token_hash = ? FOR UPDATE", hashedToken).Scan(&session).Error; err != nil {
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
		c.JSON(400, ExTransferAPIError{ErrorCode: "BAD_REQUEST", Message: "Invalid JSON payload", Retryable: false})
		return
	}

	db := middleware.GetDatabase(c)
	var session models.ExTransferSession

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw("SELECT * FROM ex_transfer_sessions WHERE session_uuid = ? FOR UPDATE", req.SessionID).Scan(&session).Error; err != nil {
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

// @Summary Heartbeat for federated transfer
// @Description Check if the transfer session is still alive
// @Produce json
// @Success 200 {object} map[string]string "Alive"
// @Tags Federated Transfer
// @Router /api/extransfer/heartbeat [post]
func heartbeatExTransfer(c *gin.Context) {
	c.JSON(200, gin.H{"status": "alive"})
}

// @Summary Confirm federated transfer
// @Description Confirm that the transfer was successful
// @Produce json
// @Success 200 {object} map[string]string "Consumed"
// @Tags Federated Transfer
// @Router /api/extransfer/confirm [post]
func confirmExTransfer(c *gin.Context) {
	c.JSON(200, gin.H{"status": "consumed"})
}

// @Summary Cancel federated transfer
// @Description Cancel the transfer session
// @Produce json
// @Success 200 {object} map[string]string "Cancelled"
// @Tags Federated Transfer
// @Router /api/extransfer/cancel [post]
func cancelExTransfer(c *gin.Context) {
	c.JSON(200, gin.H{"status": "cancelled"})
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
	var session models.ExTransferSession
	if err := db.Where("session_uuid = ?", sessionID).First(&session).Error; err != nil {
		c.JSON(404, ExTransferAPIError{ErrorCode: "NOT_FOUND", Message: "Session not found", Retryable: false})
		return
	}

	if session.Status != models.StatusMigrating {
		c.JSON(403, ExTransferAPIError{ErrorCode: "INVALID_STATE", Message: "Session not in migrating state", Retryable: false})
		return
	}

	// Verify signature
	pubKey, errDec := base64.StdEncoding.DecodeString(session.DestPublicKey)
	sig, errSig := base64.StdEncoding.DecodeString(signature)
	if errDec != nil || errSig != nil || len(pubKey) != ed25519.PublicKeySize {
		c.JSON(400, ExTransferAPIError{ErrorCode: "INVALID_SIGNATURE_FORMAT", Message: "Invalid signature format", Retryable: false})
		return
	}

	message := "DOWNLOAD:" + sessionID
	if !ed25519.Verify(pubKey, []byte(message), sig) {
		c.JSON(403, ExTransferAPIError{ErrorCode: "INVALID_SIGNATURE", Message: "Invalid signature", Retryable: false})
		return
	}

	// Get server
	var server models.Server
	if err := db.Preload("Node").Where("identifier = ?", session.ServerID).First(&server).Error; err != nil {
		c.JSON(500, ExTransferAPIError{ErrorCode: "INTERNAL_ERROR", Message: "Failed to get server", Retryable: false})
		return
	}

	ns := &services.Node{DB: db}
	downloadRes, err := ns.CallNode(&server.Node, "GET", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)
	if err != nil || downloadRes.StatusCode != 200 {
		c.JSON(500, ExTransferAPIError{ErrorCode: "DAEMON_ERROR", Message: "Failed to get file from daemon", Retryable: true})
		return
	}
	defer downloadRes.Body.Close()

	c.Header("Content-Disposition", "attachment; filename=transfer.tar.gz")
	c.Header("Content-Type", "application/octet-stream")

	_, err = io.Copy(c.Writer, downloadRes.Body)
	if err != nil {
		logging.Error.Printf("Error streaming file to client: %v", err)
	}
}

func StartDataPlaneMigration(session models.ExTransferSession, db *gorm.DB) {
	logging.Info.Printf("Starting data plane migration for session %s", session.SessionUUID)

	var server models.Server
	if err := db.Preload("Node").Where("identifier = ?", session.ServerID).First(&server).Error; err != nil {
		logging.Error.Printf("Failed to get server for migration: %v", err)
		return
	}

	ns := &services.Node{DB: db}

	// Clean up existing archive if it exists from previous attempts
	_, _ = ns.CallNode(&server.Node, "DELETE", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)

	// Archive files
	archiveBody := bytes.NewReader([]byte(`["."] `))
	headersArch := http.Header{}
	headersArch.Set("Content-Type", "application/json")
	res, err := ns.CallNode(&server.Node, "POST", fmt.Sprintf("/daemon/server/%s/archive/transfer.tar.gz", server.Identifier), io.NopCloser(archiveBody), headersArch)
	if err != nil || (res != nil && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK) {
		logging.Error.Printf("Failed to archive server %s: %v", server.Identifier, err)
		db.Model(&session).Update("status", models.StatusFailed)
		sendWebhookReport(server.Name, "Fallida (Origen)", "Fallo al comprimir los archivos en el servidor de origen.", true)
		return
	}

	logging.Info.Printf("Archive created for session %s", session.SessionUUID)
}

type ExPullReq struct {
	OriginURL string `json:"origin_url" binding:"required"`
	Token     string `json:"token" binding:"required"`
}

func pullExTransfer(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)
	var req ExPullReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON payload"})
		return
	}

	req.Token = strings.TrimSpace(req.Token)
	req.OriginURL = strings.TrimSpace(req.OriginURL)

	db := middleware.GetDatabase(c)

	setTransferProgress(server.Identifier, "Iniciando...")
	go performPullTransferAsync(server, req.OriginURL, req.Token, db)

	c.JSON(202, gin.H{"status": "ACCEPTED", "message": "Pull process initiated"})
}

func getExTransferStatus(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)
	status := getTransferProgress(server.Identifier)
	c.JSON(200, gin.H{"status": status})
}

func performPullTransferAsync(server *models.Server, originURL, token string, db *gorm.DB) {
	sendStep := func(msg string) {
		setTransferProgress(server.Identifier, msg)
		if strings.HasPrefix(msg, "ERROR:") {
			sendWebhookReport(server.Name, "Fallida", msg, true)
		}
	}
	defer func() {
		time.Sleep(10 * time.Second)
		sendStep("")
	}()

	logging.Info.Printf("Starting pull transfer for server %s from %s", server.Identifier, originURL)
	sendStep("Validando conexión con el panel de origen...")

	baseURL, err := parseOriginURL(originURL)
	if err != nil {
		logging.Error.Printf("Invalid origin URL %s: %v", originURL, err)
		sendStep("ERROR: La URL de origen no es válida o apunta a una dirección no permitida")
		return
	}

	originAPI := func(path string) *url.URL {
		return baseURL.ResolveReference(&url.URL{Path: path})
	}
	originValidate := originAPI("/api/extransfer/validate")
	originConsume := originAPI("/api/extransfer/consume")
	originDownload := originAPI("/api/extransfer/download")

	// 2. Validate token and get nonce
	pubKeyB64 := base64.StdEncoding.EncodeToString(ExTransferPublicKey)

	validateBody := map[string]string{
		"token":             token,
		"target_public_key": pubKeyB64,
		"protocol_version":  "1.0",
	}

	bodyBytes, _ := json.Marshal(validateBody)
	req := &http.Request{
		Method: "POST",
		URL:    originValidate,
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewBuffer(bodyBytes)),
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		logging.Error.Printf("Failed to call validate on origin: %v", err)
		sendStep("ERROR: Fallo de red al conectar con origen")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		logging.Error.Printf("Origin validate failed with status %d: %s", resp.StatusCode, string(body))
		sendStep(fmt.Sprintf("ERROR: Token inválido o expirado (Status %d)", resp.StatusCode))
		return
	}

	var validateRes struct {
		SessionID string `json:"session_id"`
		Nonce     string `json:"nonce"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&validateRes); err != nil {
		logging.Error.Printf("Failed to decode validate response: %v", err)
		sendStep("ERROR: Respuesta inválida del origen")
		return
	}

	if _, err := uuid.Parse(validateRes.SessionID); err != nil {
		logging.Error.Printf("Origin returned invalid session_id: %v", err)
		sendStep("ERROR: El origen devolvió un ID de sesión inválido")
		return
	}

	sendStep("Iniciando la transferencia en el servidor origen...")
	// 3. Consume transfer
	message := validateRes.Nonce + validateRes.SessionID
	sig := ed25519.Sign(ExTransferPrivateKey, []byte(message))
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	consumeBody := map[string]string{
		"session_id": validateRes.SessionID,
		"signature":  sigB64,
	}

	bodyBytes, _ = json.Marshal(consumeBody)
	req = &http.Request{
		Method: "POST",
		URL:    originConsume,
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewBuffer(bodyBytes)),
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = externalHTTPClient.Do(req)
	if err != nil {
		logging.Error.Printf("Failed to call consume on origin: %v", err)
		sendStep("ERROR: Fallo al iniciar transferencia en origen")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		logging.Error.Printf("Origin consume failed with status %d: %s", resp.StatusCode, string(body))
		sendStep("ERROR: El origen rechazó la transferencia")
		return
	}

	sendStep("Esperando a que el origen comprima los archivos...")

	// 5. Download file — retry until origin is ready (archive may take time to create)
	dlMessage := "DOWNLOAD:" + validateRes.SessionID
	dlSig := ed25519.Sign(ExTransferPrivateKey, []byte(dlMessage))
	dlSigB64 := base64.StdEncoding.EncodeToString(dlSig)

	dlURL := *originDownload
	dlURL.RawQuery = url.Values{
		"session_id": {validateRes.SessionID},
		"signature":  {dlSigB64},
	}.Encode()

	const maxWait = 90 * time.Second
	const pollInterval = 5 * time.Second
	deadline := time.Now().Add(maxWait)

	for {
		time.Sleep(pollInterval)
		sendStep("Descargando paquete de datos desde el origen...")
		req = &http.Request{
			Method: "GET",
			URL:    &dlURL,
			Header: http.Header{},
		}
		resp, err = externalHTTPClient.Do(req)
		if err != nil {
			logging.Error.Printf("Failed to call download on origin: %v", err)
			sendStep("ERROR: Error de red al descargar paquete")
			return
		}
		if resp.StatusCode == 200 {
			break
		}
		resp.Body.Close()
		if time.Now().After(deadline) {
			logging.Error.Printf("Origin download timed out after %s", maxWait)
			sendStep(fmt.Sprintf("ERROR: El origen no preparó el paquete a tiempo (timeout %s)", maxWait))
			return
		}
		logging.Info.Printf("Origin not ready yet (status %d), retrying...", resp.StatusCode)
	}
	defer resp.Body.Close()

	sendStep("Subiendo archivos al daemon local (esto puede tardar)...")

	// 6. Stream to daemon
	ns := &services.Node{DB: db}

	headersTransfer := http.Header{}
	headersTransfer.Set("Content-Type", "application/octet-stream")

	uploadRes, err := ns.CallNode(&server.Node, "PUT", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), resp.Body, headersTransfer)
	if err != nil || uploadRes.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(uploadRes.Body)
		logging.Error.Printf("Failed to upload archive to daemon: %v %s", err, string(body))
		sendStep("ERROR: Error al subir archivo al daemon")
		return
	}
	if uploadRes.Body != nil {
		uploadRes.Body.Close()
	}

	sendStep("Descomprimiendo archivos...")
	// 7. Extract on daemon
	logging.Info.Printf("Extracting files on daemon for server %s", server.Identifier)
	extractRes, err := ns.CallNode(&server.Node, "POST", fmt.Sprintf("/daemon/server/%s/extract/transfer.tar.gz?destination=.", server.Identifier), nil, nil)
	if err != nil || (extractRes != nil && extractRes.StatusCode != http.StatusNoContent && extractRes.StatusCode != http.StatusOK) {
		var body string
		if extractRes != nil && extractRes.Body != nil {
			b, _ := io.ReadAll(extractRes.Body)
			body = string(b)
		}
		logging.Error.Printf("Failed to extract on daemon: %v, body: %s", err, body)
		sendStep(fmt.Sprintf("ERROR: Error al descomprimir en el destino: %s", body))
		return
	}
	if extractRes != nil && extractRes.Body != nil {
		extractRes.Body.Close()
	}

	sendStep("Limpiando archivos temporales...")
	// Clean up transfer file on daemon
	_, _ = ns.CallNode(&server.Node, "DELETE", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)

	logging.Info.Printf("Pull transfer for server %s completed successfully", server.Identifier)
	sendWebhookReport(server.Name, "Completada", "La transferencia externa finalizó correctamente.", false)
	sendStep("DONE")
}

func parseOriginURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
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
