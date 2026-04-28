package api

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/middleware"
	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ExTransferSalt = "AETHER_FEDERATED_SALT_v1"

type ExTransferApiError struct {
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

func RegisterExTransferRoutes(rg *gin.RouterGroup) {
	// Origin endpoints
	// Origin endpoints are mounted per-server in servers.go
	
	// Destination interacting with Origin
	rg.POST("/validate", validateExTransfer)
	rg.POST("/consume", consumeExTransfer)
	rg.POST("/heartbeat", heartbeatExTransfer)
	rg.POST("/confirm", confirmExTransfer)

	// User cancellation
	rg.POST("/cancel", cancelExTransfer)
}

func WellKnownAetherPanel(c *gin.Context) {
	c.JSON(200, gin.H{
		"panel_name": "SkyPanel",
		"version": "1.0",
		"protocols_supported": []string{"1.0"},
		"public_key": "YOUR_ED25519_PUB_KEY", // Replace with real key
		"tls_fingerprint": "AUTO_GENERATED",
		"capabilities": map[string]interface{}{
			"virtualization": []string{"docker"},
			"os": "linux",
		},
		"timestamp": time.Now().Unix(),
	})
}

func CreateExTransfer(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)
	user := c.MustGet("user").(*models.User)
	db := middleware.GetDatabase(c)

	// Gen token
	b := make([]byte, 32)
	rand.Read(b)
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

func validateExTransfer(c *gin.Context) {
	var req ExValidateReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, ExTransferApiError{ErrorCode: "BAD_REQUEST", Message: "Invalid JSON payload", Retryable: false})
		return
	}

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
		c.JSON(403, ExTransferApiError{ErrorCode: "TOKEN_ERROR", Message: err.Error(), Retryable: false})
		return
	}

	// Returning Server Capabilities to Destination
	c.JSON(200, gin.H{
		"session_id": sessionID,
		"nonce":      nonce,
		"capabilities_required": gin.H{
			"virtualization": "docker",
			"cpu_cores_min": 1,
			"ram_mb_min": 1024,
		},
	})
}

func consumeExTransfer(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
		Signature string `json:"signature" binding:"required"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, ExTransferApiError{ErrorCode: "BAD_REQUEST", Message: "Invalid JSON payload", Retryable: false})
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
		c.JSON(403, ExTransferApiError{ErrorCode: "CONSUME_ERROR", Message: err.Error(), Retryable: false})
		return
	}

	// Trigger async migration process (Saga pattern coordination)
	// go StartDataPlaneMigration(session)

	c.JSON(202, gin.H{"status": "MIGRATING", "message": "Stream initialized"})
}

func heartbeatExTransfer(c *gin.Context) {
	c.JSON(200, gin.H{"status": "alive"})
}

func confirmExTransfer(c *gin.Context) {
	c.JSON(200, gin.H{"status": "consumed"})
}

func cancelExTransfer(c *gin.Context) {
	c.JSON(200, gin.H{"status": "cancelled"})
}
