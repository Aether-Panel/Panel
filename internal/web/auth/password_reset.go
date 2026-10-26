package auth

import (
	"net/http"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gopkg.in/go-playground/validator.v9"
)

func ForgotPasswordPost(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	request := &forgotPasswordRequestData{}
	err := c.BindJSON(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	validate := validator.New()
	err = validate.Struct(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// without it, the form could be used to flood any address with reset emails
	if err := verifyCaptcha(c, request.TurnstileToken, request.CaptchaToken); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// answer the same either way, so the limit does not reveal whether the account exists
	if !services.PasswordResetRequests.Take(request.Email) {
		logging.Info.Printf("Password reset limit reached for: %s", request.Email)
		c.Status(http.StatusNoContent)
		return
	}

	token, accountEmail, err := us.CreatePasswordResetToken(request.Email)
	if err != nil {
		logging.Info.Printf("Password reset requested for unknown user: %s", request.Email)
		c.Status(http.StatusNoContent)
		return
	}

	resetLink := panelBaseURL() + "/reset-password/?token=" + token
	err = services.GetEmailService().SendEmail(accountEmail, "passwordReset", map[string]interface{}{
		"RESET_LINK": resetLink,
	}, true)
	if err != nil {
		logging.Error.Printf("Error sending password reset email: %s", err.Error())
	}

	c.Status(http.StatusNoContent)
}

func ResetPasswordPost(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	request := &resetPasswordRequestData{}
	err := c.BindJSON(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	validate := validator.New()
	err = validate.Struct(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	// checked before the token is spent, so a too-short password can simply be retyped
	if us.IsSecurePassword(request.Password) != nil {
		response.HandleError(c, skypanel.ErrPasswordRequirements, http.StatusBadRequest)
		return
	}

	user, err := us.ConsumePasswordResetToken(request.Token)
	if err != nil {
		response.HandleError(c, skypanel.ErrTokenInvalid, http.StatusBadRequest)
		return
	}

	err = user.SetPassword(request.Password)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = us.Update(user)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// whoever knew the old password must not stay logged in
	ss := &services.Session{DB: db}
	if err = ss.ExpireAllForUser(user.ID, ""); err != nil {
		logging.Error.Printf("Error ending sessions after password reset: %s", err.Error())
	}

	err = services.GetEmailService().SendEmail(user.Email, "passwordChanged", nil, true)
	if err != nil {
		logging.Error.Printf("Error sending password changed email: %s", err.Error())
	}

	c.Status(http.StatusNoContent)
}

type forgotPasswordRequestData struct {
	Email          string `json:"email" validate:"required,email"`
	TurnstileToken string `json:"turnstileToken"`
	CaptchaToken   string `json:"captchaToken"`
}

type resetPasswordRequestData struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required"`
}

func panelBaseURL() string {
	// The link must point at the configured panel: building it from request headers lets
	// anyone send a reset email whose link delivers the token to their own domain.
	if masterURL := strings.TrimSuffix(config.MasterURL.Value(), "/"); config.MasterURL.IsSet() && masterURL != "" {
		return masterURL
	}
	// headers are the requester's to choose: a link built from them would carry the victim's
	// token to whatever host the attacker named, so the configured (or default) URL is used
	logging.Info.Printf("Master URL is not configured; the password reset link uses %s. Set it in the panel settings.", config.MasterURL.Value())
	return strings.TrimSuffix(config.MasterURL.Value(), "/")
}
