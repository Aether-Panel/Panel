package api

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// Accounts made through public registration wait for an administrator (panel.registrationApproval):
// they have no permission to sign in until approved. Rejecting one is deleting it.

func registerUserApproval(g *gin.RouterGroup) {
	// approving grants the sign-in permission, so it needs the right to change permissions
	g.Handle("POST", "/:id/approve", middleware.RequiresPermission(scopes.ScopeUserPermsEdit), approveUser)
	g.Handle("OPTIONS", "/:id/approve", response.CreateOptions("POST"))
}

// @Summary Approve a registered account
// @Description Lets an account made through public registration sign in, and sends it the welcome email
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Param id path uint true "User ID"
// @Tags Users
// @Router /api/users/{id}/approve [post]
// @Security OAuth2Application[users.perms.edit]
func approveUser(c *gin.Context) {
	db := middleware.GetDatabase(c)
	id, err := cast.ToUintE(c.Param("id"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	user, err := (&services.User{DB: db}).GetByID(id)
	if response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if !user.PendingApproval {
		// already approved (two admins, two tabs): nothing to do
		c.Status(http.StatusNoContent)
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		ps := &services.Permission{DB: tx}
		perms, err := ps.GetForUserAndServer(user.ID, "")
		if err != nil {
			return err
		}
		perms.Scopes = scopes.AddScope(perms.Scopes, scopes.ScopeLogin)
		if err := ps.UpdatePermissions(perms); err != nil {
			return err
		}
		return tx.Model(&models.User{}).Where("id = ?", user.ID).UpdateColumn("pending_approval", false).Error
	})
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if err := services.GetEmailService().SendEmail(user.Email, "accountCreation", nil, true); err != nil {
		logging.Error.Printf("Error sending email: %s", err.Error())
	}
	c.Status(http.StatusNoContent)
}
