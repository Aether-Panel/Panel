package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	uuid "github.com/gofrs/uuid/v5"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// errClientScopes refuses a client limited to something other than server permissions
var errClientScopes = skypanel.CreateError("a client can only be limited to server permissions it can hold", "ErrClientScopes")

// emailSendErrorFmt logs a mail that could not be sent
const emailSendErrorFmt = "Error sending email: %s\n"

func registerSelf(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeLogin), getSelf)
	g.Handle("PUT", "", middleware.RequiresPermission(scopes.ScopeSelfEdit), updateSelf)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "PUT"))

	g.Handle("GET", "/otp", middleware.RequiresPermission(scopes.ScopeSelfEdit), getOtpStatus)
	g.Handle("POST", "/otp", middleware.RequiresPermission(scopes.ScopeSelfEdit), startOtpEnroll)
	g.Handle("PUT", "/otp", middleware.RequiresPermission(scopes.ScopeSelfEdit), validateOtpEnroll)
	g.Handle("OPTIONS", "/otp", response.CreateOptions("GET", "POST", "PUT"))

	g.Handle("POST", "/otp/recovery", middleware.RequiresPermission(scopes.ScopeSelfEdit), regenerateOtpRecoveryCodes)
	g.Handle("DELETE", "/otp/:token", middleware.RequiresPermission(scopes.ScopeSelfEdit), disableOtp)
	g.Handle("OPTIONS", "/otp/:token", response.CreateOptions("POST", "DELETE"))

	g.Handle("GET", "/oauth2", middleware.RequiresPermission(scopes.ScopeSelfClients), getPersonalOAuth2Clients)
	g.Handle("POST", "/oauth2", middleware.RequiresPermission(scopes.ScopeSelfClients), createPersonalOAuth2Client)
	g.Handle("OPTIONS", "/oauth2", response.CreateOptions("GET", "POST"))

	g.Handle("DELETE", "/oauth2/:clientID", middleware.RequiresPermission(scopes.ScopeSelfClients), deletePersonalOAuth2Client)
	g.Handle("OPTIONS", "/oauth2/:clientID", response.CreateOptions("DELETE"))

	g.Handle("GET", "/sshkeys", middleware.RequiresPermission(scopes.ScopeSelfEdit), getSSHKeys)
	g.Handle("POST", "/sshkeys", middleware.RequiresPermission(scopes.ScopeSelfEdit), addSSHKey)
	g.Handle("OPTIONS", "/sshkeys", response.CreateOptions("GET", "POST"))

	g.Handle("DELETE", "/sshkeys/:id", middleware.RequiresPermission(scopes.ScopeSelfEdit), deleteSSHKey)
	g.Handle("OPTIONS", "/sshkeys/:id", response.CreateOptions("DELETE"))

	registerGitHub(g.Group("/github"))
}

// @Summary Get your user info
// @Description Gets the user information of the current user
// @Success 200 {object} models.UserView
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Self
// @Router /api/self [get]
// @Security OAuth2Application[login]
func getSelf(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	// If user has a role_id but Role is not loaded (e.g. role was assigned after session was created),
	// reload it from DB so the scopes appear correctly in the response
	if user.RoleID != nil && user.Role.ID == 0 {
		db := middleware.GetDatabase(c)
		rs := &services.Role{DB: db}
		if role, err := rs.Get(*user.RoleID); err == nil {
			user.Role = *role
		}
	}

	c.JSON(http.StatusOK, models.FromUser(user))
}

// @Summary Update your user
// @Description Update user information for your current user
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param user body models.UserView true "User information"
// @Tags Self
// @Router /api/self [PUT]
// @Security OAuth2Application[self.edit]
func updateSelf(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	user := c.MustGet("user").(*models.User)

	var viewModel models.UserView
	if err := c.BindJSON(&viewModel); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if err := viewModel.Valid(true); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if viewModel.Password == "" {
		response.HandleError(c, skypanel.ErrFieldRequired("password"), http.StatusBadRequest)
		return
	}

	if !us.IsValidCredentials(user, viewModel.Password) {
		response.HandleError(c, skypanel.ErrInvalidCredentials, http.StatusInternalServerError)
		return
	}

	var oldEmail string
	if user.Email != viewModel.Email {
		oldEmail = user.Email
	}

	viewModel.CopyToModel(user)

	passwordChanged := false
	if viewModel.NewPassword != "" {
		if us.IsSecurePassword(viewModel.NewPassword) != nil {
			response.HandleError(c, skypanel.ErrPasswordRequirements, http.StatusBadRequest)
			return
		}

		passwordChanged = true
		err := user.SetPassword(viewModel.NewPassword)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	if err := us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if passwordChanged {
		// keep this session, end the others that were opened with the old password
		ss := &services.Session{DB: db}
		if err := ss.ExpireAllForUser(user.ID, middleware.GetToken(c)); err != nil {
			logging.Error.Printf("Error ending sessions after password change: %s\n", err)
		}
	}

	if oldEmail != "" {
		err := services.GetEmailService().SendEmail(oldEmail, "emailChanged", map[string]interface{}{
			"NEW_EMAIL": user.Email,
		}, true)
		if err != nil {
			logging.Error.Printf(emailSendErrorFmt, err)
		}
	}

	if passwordChanged {
		err := services.GetEmailService().SendEmail(user.Email, "passwordChanged", nil, true)
		if err != nil {
			logging.Error.Printf(emailSendErrorFmt, err)
		}
	}

	c.Status(http.StatusNoContent)
}

func getOtpStatus(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	user := c.MustGet("user").(*models.User)

	otpEnabled, err := us.GetOtpStatus(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"otpEnabled": otpEnabled,
	})
}

func startOtpEnroll(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	user := c.MustGet("user").(*models.User)

	secret, img, err := us.StartOtpEnroll(user.ID)
	if errors.Is(err, skypanel.ErrOtpAlreadyEnabled) {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": secret,
		"img":    img,
	})
}

func validateOtpEnroll(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	user := c.MustGet("user").(*models.User)

	request := &ValidateOtpRequest{}

	err := c.BindJSON(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	recoveryCodes, err := us.ValidateOtpEnroll(user.ID, request.Token)
	if errors.Is(err, skypanel.ErrInvalidCredentials) {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "otpEnabled", nil, true)
	if err != nil {
		logging.Error.Printf(emailSendErrorFmt, err)
	}
	c.JSON(http.StatusOK, gin.H{
		"recoveryCodes": recoveryCodes,
	})
}

func regenerateOtpRecoveryCodes(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	user := c.MustGet("user").(*models.User)

	request := &ValidateOtpRequest{}

	err := c.BindJSON(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	_, err = us.ValidOtp(user.Email, request.Token)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	recoveryCodes, err := us.RegenerateOtpRecoveryCodes(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "recoveryCodesRegenerated", nil, true)
	if err != nil {
		logging.Error.Printf(emailSendErrorFmt, err)
	}
	c.JSON(http.StatusOK, gin.H{
		"recoveryCodes": recoveryCodes,
	})
}

func disableOtp(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	user := c.MustGet("user").(*models.User)
	token := c.Param("token")

	_, err := us.ValidOtp(user.Email, token)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = us.DisableOtp(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "otpDisabled", nil, true)
	if err != nil {
		logging.Error.Printf(emailSendErrorFmt, err)
	}
	c.Status(http.StatusNoContent)
}

// @Summary Gets registered OAuth2 clients
// @Description Gets known OAuth2 clients the logged-in user has registered
// @Success 200 {object} []models.Client
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Self
// @Router /api/self/oauth2 [GET]
// @Security OAuth2Application[self.clients]
func getPersonalOAuth2Clients(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	db := middleware.GetDatabase(c)
	os := &services.OAuth2{DB: db}

	clients, err := os.GetForUser(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	views := make([]personalClientView, 0, len(clients))
	for _, client := range clients {
		views = append(views, viewPersonalClient(db, client))
	}
	c.JSON(http.StatusOK, views)
}

// personalClientView is an account client as the profile lists it: with the server and the
// permissions it is limited to, when it is
type personalClientView struct {
	ClientID    string   `json:"client_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ServerID    string   `json:"serverId,omitempty"`
	ServerName  string   `json:"serverName,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	// ClientSecret is only in the answer that creates the client
	ClientSecret string `json:"client_secret,omitempty"`
} // @name PersonalClient

func viewPersonalClient(db *gorm.DB, client *models.Client) personalClientView {
	view := personalClientView{ClientID: client.ClientID, Name: client.Name, Description: client.Description, Scopes: strings.Fields(client.Scopes)}
	if client.ServerID != nil && *client.ServerID != "" {
		view.ServerID = *client.ServerID
		var server models.Server
		if db.Select("name").Where("identifier = ?", view.ServerID).First(&server).Error == nil {
			view.ServerName = server.Name
		}
	}
	return view
}

// personalClientRequest creates an account client, optionally limited to one server and to
// some of the permissions its user has there
type personalClientRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ServerID    string   `json:"serverId"`
	Scopes      []string `json:"scopes"`
}

// clientLimits checks what a new client asks to be limited to: a server the user can see, and
// server permissions the user holds (there, or panel-wide without a server). A limit can only
// narrow the client; nothing here grants more than the user already has.
func clientLimits(db *gorm.DB, user *models.User, request personalClientRequest) (*string, string, error) {
	serverID := strings.TrimSpace(request.ServerID)
	held, err := middleware.UserScopesOn(db, user, serverID)
	if err != nil {
		return nil, "", err
	}
	if serverID != "" && !scopes.ContainsScope(held, scopes.ScopeServerView) {
		return nil, "", skypanel.ErrServerNotFound
	}
	var wanted []string
	for _, name := range request.Scopes {
		scope := scopes.GetScope(strings.TrimSpace(name))
		if scope == nil || !scope.ForServer {
			return nil, "", errClientScopes
		}
		if serverID != "" && !scopes.ContainsScope(held, scope) {
			return nil, "", skypanel.ErrNoPermission
		}
		if !containsString(wanted, scope.String()) {
			wanted = append(wanted, scope.String())
		}
	}
	// a client for one server still needs to see it to do anything there
	if serverID != "" && len(wanted) > 0 && !containsString(wanted, scopes.ScopeServerView.String()) {
		wanted = append(wanted, scopes.ScopeServerView.String())
	}
	if serverID == "" {
		return nil, strings.Join(wanted, " "), nil
	}
	return &serverID, strings.Join(wanted, " "), nil
}

// @Summary Create an account-level OAuth2 client
// @Success 200 {object} models.Client
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param client body models.Client false "Information for the client to create"
// @Tags Self
// @Router /api/self/oauth2 [POST]
// @Security OAuth2Application[self.clients]
func createPersonalOAuth2Client(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	db := middleware.GetDatabase(c)
	os := &services.OAuth2{DB: db}

	var request personalClientRequest
	err := c.BindJSON(&request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	serverID, limitedTo, err := clientLimits(db, user, request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, skypanel.ErrServerNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, skypanel.ErrNoPermission) {
			status = http.StatusForbidden
		}
		response.HandleError(c, err, status)
		return
	}

	id, err := uuid.NewV4()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	client := &models.Client{
		ClientID:    id.String(),
		UserID:      user.ID,
		Name:        request.Name,
		Description: request.Description,
		ServerID:    serverID,
		Scopes:      limitedTo,
	}

	secret, err := utils.GenerateRandomString(36)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	client.ClientSecret = secret

	err = client.SetClientSecret(secret)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = os.Create(client)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "oauthCreated", nil, true)
	if err != nil {
		logging.Error.Printf(emailSendErrorFmt, err)
	}

	// the secret only travels in this answer
	view := viewPersonalClient(db, client)
	view.ClientSecret = client.ClientSecret
	c.JSON(http.StatusOK, view)
}

// @Summary Deletes an account-level OAuth2 client
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path string true "Information for the client to create"
// @Tags Self
// @Router /api/self/oauth2/{id} [DELETE]
// @Security OAuth2Application[self.clients]
func deletePersonalOAuth2Client(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	clientID := c.Param("clientID")

	db := middleware.GetDatabase(c)
	os := &services.OAuth2{DB: db}

	client, err := os.Get(clientID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// ensure the client id is specific for this server, and this user
	if client.UserID != user.ID {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	err = os.Delete(client.ClientID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "oauthDeleted", nil, true)
	if err != nil {
		logging.Error.Printf(emailSendErrorFmt, err)
	}
	c.Status(http.StatusNoContent)
}

// @Summary Get your SSH keys
// @Description The public keys you can sign in to SFTP with instead of your password
// @Success 200 {object} []models.SSHKey
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Self
// @Router /api/self/sshkeys [GET]
// @Security OAuth2Application[self.edit]
func getSSHKeys(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	ks := &services.SSHKey{DB: middleware.GetDatabase(c)}
	keys, err := ks.GetForUser(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, keys)
}

type AddSSHKeyRequest struct {
	Name string `json:"name"`
	// PublicKey is one authorized_keys line, such as the contents of id_ed25519.pub
	PublicKey string `json:"publicKey"`
}

// @Summary Add an SSH key
// @Description Adds a public key to sign in to SFTP with. The username stays email#serverID.
// @Success 200 {object} models.SSHKey
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param key body AddSSHKeyRequest true "The key and a name for it"
// @Tags Self
// @Router /api/self/sshkeys [POST]
// @Security OAuth2Application[self.edit]
func addSSHKey(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	var request AddSSHKeyRequest
	if err := c.BindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	ks := &services.SSHKey{DB: middleware.GetDatabase(c)}
	key, err := ks.Add(user.ID, request.Name, request.PublicKey)
	var panelErr *skypanel.Error
	if errors.As(err, &panelErr) {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, key)
}

// @Summary Delete an SSH key
// @Success 204 {object} nil
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path int true "Key ID"
// @Tags Self
// @Router /api/self/sshkeys/{id} [DELETE]
// @Security OAuth2Application[self.edit]
func deleteSSHKey(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	id, err := cast.ToUintE(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	ks := &services.SSHKey{DB: middleware.GetDatabase(c)}
	if response.HandleError(c, ks.Delete(user.ID, id), http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

type ValidateOtpRequest struct {
	Token string `json:"token"`
}
