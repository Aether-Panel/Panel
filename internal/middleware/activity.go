package middleware

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
)

// RecordActivity appends an access event to the panel activity feed, taking the
// address and client string from the request. It is deliberately best effort:
// the record is evidence *about* an action, so failing to write it must never
// fail the action itself.
func RecordActivity(c *gin.Context, event models.ActivityEvent, user *models.User) {
	if user == nil {
		return
	}

	db := GetDatabase(c)
	if db == nil {
		return
	}

	activity := &services.Activity{DB: db}
	err := activity.Record(
		event,
		&user.ID,
		user.Username,
		user.Email,
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		logging.Error.Printf("Failed to record activity event %s: %s", event, err.Error())
	}
}
