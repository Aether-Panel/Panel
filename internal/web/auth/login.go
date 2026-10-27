package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// @Summary User Login
// @Description Authenticates a user and starts a session
// @Accept json
// @Produce json
// @Param request body LoginRequestData true "Login Request"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} skypanel.ErrorResponse
// @Tags Auth
// @Router /auth/login [post]
func LoginPost(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	request := &LoginRequestData{}

	err := c.BindJSON(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if err := verifyCaptcha(c, request.TurnstileToken, request.CaptchaToken); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	user, otpNeeded, err := us.ValidateLogin(request.Email, request.Password)
	if errors.Is(err, skypanel.ErrTooManyAttempts) {
		response.HandleError(c, err, http.StatusTooManyRequests)
		return
	}
	if err != nil && user != nil && user.ID != 0 {
		// Only a known account is recorded: an unknown address would mean
		// storing input the caller chose, and a public form can be spammed.
		middleware.RecordActivity(c, models.ActivityLoginFailed, user)
	}
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if otpNeeded {
		userSession := sessions.Default(c)
		userSession.Set("user", user.Email)
		userSession.Set("time", time.Now().Unix())
		err = userSession.Save()
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		c.JSON(http.StatusOK, &LoginResponse{
			OtpNeeded: true,
		})
		return
	}

	createSession(c, user)
}

// @Summary OTP Verification
// @Description Verifies an OTP code for a login attempt
// @Accept json
// @Produce json
// @Param request body OtpRequestData true "OTP Request"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} skypanel.ErrorResponse
// @Tags Auth
// @Router /auth/otp [post]
func OtpPost(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	request := &OtpRequestData{}

	err := c.BindJSON(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	userSession := sessions.Default(c)
	email, _ := userSession.Get("user").(string)
	timestamp, _ := userSession.Get("time").(int64)

	if email == "" {
		response.HandleError(c, skypanel.ErrInvalidSession, http.StatusBadRequest)
		return
	}

	if timestamp < time.Now().Unix()-300 {
		userSession.Clear()
		_ = userSession.Save()
		response.HandleError(c, skypanel.ErrSessionExpired, http.StatusBadRequest)
		return
	}

	user, err := us.ValidOtp(email, request.Token)
	if errors.Is(err, skypanel.ErrTooManyAttempts) {
		response.HandleError(c, err, http.StatusTooManyRequests)
		return
	}
	if err != nil && user != nil && user.ID != 0 {
		middleware.RecordActivity(c, models.ActivityLoginFailed, user)
	}
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	createSession(c, user)
}

func createSession(c *gin.Context, user *models.User) {
	data, ok := openSession(c, user, models.ActivityLogin)
	if ok {
		c.JSON(http.StatusOK, data)
	}
}

// userScopes is every panel-wide scope the user holds, their own plus their role's.
func userScopes(db *gorm.DB, user *models.User) ([]*scopes.Scope, error) {
	ps := &services.Permission{DB: db}
	perms, err := ps.GetForUserAndServer(user.ID, "")
	if err != nil {
		return nil, err
	}

	allScopes := perms.Scopes
	if user.RoleID != nil {
		rs := &services.Role{DB: db}
		role, err := rs.Get(*user.RoleID)
		if err == nil && role != nil {
			for _, s := range role.Scopes {
				allScopes = scopes.AddScope(allScopes, scopes.GetScope(s))
			}
		}
	}
	return allScopes, nil
}

// openSession signs the user in: it checks they may log in, sets the session cookie and
// records the event. ok is false once an error response has been written.
func openSession(c *gin.Context, user *models.User, event models.ActivityEvent) (*LoginResponse, bool) {
	db := middleware.GetDatabase(c)
	ss := &services.Session{DB: db}

	allScopes, err := userScopes(db, user)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return nil, false
	}

	if !scopes.ContainsScope(allScopes, scopes.ScopeLogin) {
		response.HandleError(c, skypanel.ErrLoginNotPermitted, http.StatusForbidden)
		return nil, false
	}

	session, err := ss.CreateForUser(user)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return nil, false
	}

	middleware.RecordActivity(c, event, user)

	data := &LoginResponse{}
	data.Scopes = allScopes
	data.Session = session // Expose session token for external App auth

	maxAge := int(services.SessionLength / time.Second)

	// Always set Secure=true for session cookie to prevent transmission over HTTP
	middleware.SetAuthCookie(c, session, maxAge)
	return data, true
}

type LoginRequestData struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	TurnstileToken string `json:"turnstileToken"`
	// CaptchaToken answers the built-in anti-bot check (see GET /auth/challenge)
	CaptchaToken string `json:"captchaToken"`
}

type LoginResponse struct {
	Scopes    []*scopes.Scope `json:"scopes,omitempty"`
	OtpNeeded bool            `json:"otpNeeded,omitempty"`
	Session   string          `json:"session,omitempty"`
}

type OtpRequestData struct {
	Token string `json:"token"`
}
