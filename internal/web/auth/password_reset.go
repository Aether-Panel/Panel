package auth

import (
	"net/http"

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

	token, err := us.CreatePasswordResetToken(request.Email)
	if err != nil {
		logging.Info.Printf("Password reset requested for unknown user: %s", request.Email)
		c.Status(http.StatusNoContent)
		return
	}

	resetLink := panelBaseURL(c) + "/reset-password?token=" + token
	err = services.GetEmailService().SendEmail(request.Email, "passwordReset", map[string]interface{}{
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

	user, err := us.ConsumePasswordResetToken(request.Token)
	if err != nil {
		response.HandleError(c, skypanel.ErrTokenInvalid, http.StatusBadRequest)
		return
	}

	if us.IsSecurePassword(request.Password) != nil {
		response.HandleError(c, skypanel.ErrPasswordRequirements, http.StatusBadRequest)
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

	err = services.GetEmailService().SendEmail(user.Email, "passwordChanged", nil, true)
	if err != nil {
		logging.Error.Printf("Error sending password changed email: %s", err.Error())
	}

	c.Status(http.StatusNoContent)
}

type forgotPasswordRequestData struct {
	Email string `json:"email" validate:"required,email"`
}

type resetPasswordRequestData struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required"`
}

func panelBaseURL(c *gin.Context) string {
	host := c.Request.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}

	if host != "" {
		scheme := "http"
		if proto := c.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else if c.Request.TLS != nil {
			scheme = "https"
		}
		return scheme + "://" + host
	}

	return config.MasterURL.Value()
}
