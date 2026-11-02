package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/oauth2"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// SFTPKeyRequest is a node asking whether a public key offered to its SFTP server may sign in
type SFTPKeyRequest struct {
	// Username is the SFTP login, "email#serverID"
	Username string `json:"username"`
	// PublicKey is the key the client offered, one authorized_keys line
	PublicKey string `json:"publicKey"`
} // @name SFTPKeyRequest

// sftpKeyRequestLimit bounds the body: one login name and one public key
const sftpKeyRequestLimit = 16 * 1024

// sftpKeyNoAccess is the one refusal every failed check gets, so the answer does not tell
// whether the account, the server or the key was wrong
const sftpKeyNoAccess = "no access"

// @Summary Check an SFTP key login
// @Description For nodes: whether a public key offered to the node's SFTP server is one of the user's SSH keys, and the user may still sign in and use SFTP on that server, which must be on this node. The node signs in with its secret, as for the OAuth2 password check. Only the granted scope is answered, never a session.
// @Accept json
// @Produce json
// @Param request body SFTPKeyRequest true "SFTP login to check"
// @Success 200 {object} oauth2.TokenResponse
// @Failure 400 {object} oauth2.ErrorResponse
// @Failure 401 {object} oauth2.ErrorResponse
// @Failure 429 {object} oauth2.ErrorResponse
// @Tags Auth
// @Router /auth/sftp-key [post]
func SFTPKeyPost(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	db := middleware.GetDatabase(c)

	node, ok := requestingNode(c, db)
	if !ok {
		return
	}

	var request SFTPKeyRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, sftpKeyRequestLimit)
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, &oauth2.ErrorResponse{Error: "invalid_request", ErrorDescription: "bad request"})
		return
	}

	scope, status := checkSFTPKey(db, node, &request)
	switch status {
	case http.StatusOK:
		c.JSON(http.StatusOK, &oauth2.TokenResponse{Scope: scope})
	case http.StatusTooManyRequests:
		c.AbortWithStatusJSON(status, &oauth2.ErrorResponse{Error: "invalid_request", ErrorDescription: "too many failed attempts, wait a few minutes"})
	default:
		c.AbortWithStatusJSON(status, &oauth2.ErrorResponse{Error: "invalid_request", ErrorDescription: sftpKeyNoAccess})
	}
}

// requestingNode finds the node from the secret it signs in with. It answers 401 itself and
// returns false when the request carries no node's secret.
func requestingNode(c *gin.Context, db *gorm.DB) (*models.Node, bool) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	secret, found := strings.CutPrefix(header, "Bearer ")
	secret = strings.TrimSpace(secret)
	// an empty secret must never reach the lookup: a query on an empty field matches any node
	if found && secret != "" {
		node, err := (&services.Session{DB: db}).ValidateNode(secret)
		if err == nil {
			return node, true
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.Error.Printf("Could not check the node asking for an SFTP key login: %s", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, &oauth2.ErrorResponse{Error: "server_error"})
			return nil, false
		}
	}
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatusJSON(http.StatusUnauthorized, &oauth2.ErrorResponse{Error: "invalid_client"})
	return nil, false
}

// sftpKeyAttemptKey is the attempt budget of key logins for one account. It is apart from the
// password budget: an SSH client offers each of its keys in turn, and those misses must not
// lock the owner out of signing in with their password.
func sftpKeyAttemptKey(userID uint) string {
	return fmt.Sprintf("sshkey:user:%d", userID)
}

// checkSFTPKey applies the rules of an SFTP key login on the panel's own node
// (services.DatabaseSFTPAuthorization.ValidateKey) for a remote node: the server must be on
// the asking node, the key one of the user's, and the user must still hold the login scope and
// SFTP on the server. It returns the granted scope and http.StatusOK, or the status to refuse with.
func checkSFTPKey(db *gorm.DB, node *models.Node, request *SFTPKeyRequest) (string, int) {
	parts := strings.Split(request.Username, "#")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", http.StatusBadRequest
	}
	email, serverID := parts[0], parts[1]

	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(request.PublicKey))
	if err != nil {
		return "", http.StatusBadRequest
	}

	// a node only checks logins for the servers it runs
	server, err := (&services.Server{DB: db}).Get(serverID)
	if err != nil || server.NodeID != node.ID {
		return "", http.StatusBadRequest
	}

	user, err := (&services.User{DB: db}).GetByEmail(email)
	if err != nil || user == nil {
		return "", http.StatusBadRequest
	}

	limitKey := sftpKeyAttemptKey(user.ID)
	if !services.LoginAttempts.Take(limitKey) {
		return "", http.StatusTooManyRequests
	}

	stored, err := (&services.SSHKey{DB: db}).FindForLogin(user.ID, key)
	if err != nil || stored == nil {
		return "", http.StatusBadRequest
	}

	ps := &services.Permission{DB: db}
	allowed, err := ps.MaySignIn(user.ID)
	if err == nil && allowed {
		allowed, err = ps.HasPermission(user.ID, server.Identifier, scopes.ScopeServerSftp)
	}
	if err != nil || !allowed {
		return "", http.StatusBadRequest
	}

	services.LoginAttempts.Reset(limitKey)
	return server.Identifier + ":" + scopes.ScopeServerSftp.String(), http.StatusOK
}
