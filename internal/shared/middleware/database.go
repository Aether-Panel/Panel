package middleware

import (
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
)

func NeedsDatabase(c *gin.Context) {
	db, err := database.GetConnection()

	if err != nil {
		logging.Error.Printf("Database not available: %s", err)
		err = skypanel.ErrDatabaseNotAvailable
	}

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Set("db", db)
}

func GetDatabase(c *gin.Context) *gorm.DB {
	db, exist := c.Get("db")
	if !exist {
		return nil
	}
	casted, ok := db.(*gorm.DB)
	if !ok {
		return nil
	}
	return casted
}

func HasTransaction(c *gin.Context) {
	db := GetDatabase(c)

	if db == nil {
		NeedsDatabase(c)
		db = GetDatabase(c)
		if db == nil {
			response.HandleError(c, skypanel.ErrDatabaseNotAvailable, http.StatusInternalServerError)
			return
		}
	}

	c.Set("noTransactionDb", db)

	_ = db.Transaction(func(trans *gorm.DB) error {
		c.Set("db", trans)

		c.Next()

		if len(c.Errors) > 0 {
			logging.Error.Printf("Transaction rollback due to c.Errors: %+v", c.Errors)
			return errors.New("error in transaction")
		} else if c.Writer.Status() >= 400 {
			return fmt.Errorf("bad status code %d", c.Writer.Status())
		}
		return nil
	})
}
