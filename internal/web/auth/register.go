package auth

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gopkg.in/go-playground/validator.v9"
)

// @Summary Register a new user
// @Description Creates a new user account if registration is enabled
// @Accept json
// @Produce json
// @Param request body registerRequestData true "Registration Request"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} skypanel.ErrorResponse
// @Tags Auth
// @Router /auth/register [post]
func RegisterPost(c *gin.Context) {
	if !config.RegistrationEnabled.Value() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	request := &registerRequestData{}
	err := c.BindJSON(request)

	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	validate := validator.New()
	err = validate.Struct(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if us.IsSecurePassword(request.Password) != nil {
		response.HandleError(c, skypanel.ErrPasswordRequirements, http.StatusBadRequest)
		return
	}

	user := &models.User{Username: request.Username, Email: request.Email}
	err = user.SetPassword(request.Password)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = us.Create(user)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	ps := &services.Permission{DB: db}
	perms, err := ps.GetForUserAndServer(user.ID, "")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// perms.ViewServer = true
	perms.Scopes = []*scopes.Scope{scopes.ScopeLogin}

	err = ps.UpdatePermissions(perms)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "accountCreation", nil, true)
	if err != nil {
		logging.Error.Printf("Error sending email: %s", err.Error())
	}

	createSession(c, user)
}

type registerRequestData struct {
	Username string `json:"username" validate:"required,printascii,min=5,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
