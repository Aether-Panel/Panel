package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/oauth2"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SFTPCheckRequest is a node asking whether the user of an open SFTP connection may keep it
type SFTPCheckRequest struct {
	// Username is the SFTP login the connection signed in with, "email#serverID"
	Username string `json:"username"`
	// KeyFingerprint is the SHA256 fingerprint of the SSH key the connection signed in with,
	// empty for a password: the key must still be on the account
	KeyFingerprint string `json:"keyFingerprint,omitempty"`
} // @name SFTPCheckRequest

// sftpCheckRequestLimit bounds the body: one login name
const sftpCheckRequestLimit = 4 * 1024

// @Summary Check an open SFTP connection
// @Description For nodes: whether the user of an SFTP connection the node already let in may still use SFTP on that server, which must be on this node. The node asks every 30 seconds for each open connection, with the fingerprint of the SSH key it signed in with (if any), and closes it on a 400 "no access". It signs in with its secret, as for the OAuth2 password check. Nothing is counted against the attempt limits: the node already checked the password or key.
// @Accept json
// @Produce json
// @Param request body SFTPCheckRequest true "SFTP login to check"
// @Success 200 {object} oauth2.TokenResponse
// @Failure 400 {object} oauth2.ErrorResponse
// @Failure 401 {object} oauth2.ErrorResponse
// @Failure 500 {object} oauth2.ErrorResponse
// @Tags Auth
// @Router /auth/sftp-check [post]
func SFTPCheckPost(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	db := middleware.GetDatabase(c)

	node, ok := requestingNode(c, db)
	if !ok {
		return
	}

	var request SFTPCheckRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, sftpCheckRequestLimit)
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, &oauth2.ErrorResponse{Error: "invalid_request", ErrorDescription: "bad request"})
		return
	}

	scope, err := checkSFTPAccess(db, node, request.Username, request.KeyFingerprint)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, &oauth2.TokenResponse{Scope: scope})
	case errors.Is(err, errSFTPNoAccess):
		// the node closes the connection only on this exact answer
		c.AbortWithStatusJSON(http.StatusBadRequest, &oauth2.ErrorResponse{Error: "invalid_request", ErrorDescription: oauth2.SFTPNoAccess})
	default:
		// the node keeps the connection on anything but a 400, so a database error must not
		// look like a refusal
		logging.Error.Printf("Could not check an open SFTP connection: %s", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, &oauth2.ErrorResponse{Error: "server_error"})
	}
}

// errSFTPNoAccess is a definite no: the node closes the connection
var errSFTPNoAccess = errors.New("no access")

// checkSFTPAccess applies the SFTP sign-in rule, without the password or key, to a connection
// a node already let in: the server must still be on that node and the account must still
// exist, hold the login scope and SFTP on the server, and, for a connection opened with an SSH
// key, still have that key. It returns the granted scope, errSFTPNoAccess, or another error
// when the database could not answer.
func checkSFTPAccess(db *gorm.DB, node *models.Node, username, keyFingerprint string) (string, error) {
	email, serverID, found := strings.Cut(username, "#")
	if !found || email == "" || serverID == "" || strings.Contains(serverID, "#") {
		return "", errSFTPNoAccess
	}

	server, err := (&services.Server{DB: db}).Get(serverID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", errSFTPNoAccess
	}
	if err != nil {
		return "", err
	}
	if server.NodeID != node.ID {
		return "", errSFTPNoAccess
	}

	user, err := (&services.User{DB: db}).GetByEmail(email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", errSFTPNoAccess
	}
	if err != nil {
		return "", err
	}

	if keyFingerprint != "" {
		kept, err := (&services.SSHKey{DB: db}).HasFingerprint(user.ID, keyFingerprint)
		if err != nil {
			return "", err
		}
		if !kept {
			return "", errSFTPNoAccess
		}
	}

	ps := &services.Permission{DB: db}
	allowed, err := ps.MaySignIn(user.ID)
	if err == nil && allowed {
		allowed, err = ps.HasPermission(user.ID, server.Identifier, scopes.ScopeServerSftp)
	}
	if err != nil {
		return "", err
	}
	if !allowed {
		return "", errSFTPNoAccess
	}
	return server.Identifier + ":" + scopes.ScopeServerSftp.String(), nil
}
