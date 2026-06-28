package tests

import (
	"net/http"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestExTransferAPI(t *testing.T) {
	t.Run("ValidateNoToken", func(t *testing.T) {
		response := CallAPI("POST", "/api/extransfer/validate", map[string]interface{}{
			"token": "",
		}, "")
		
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("ConsumeNoSession", func(t *testing.T) {
		response := CallAPI("POST", "/api/extransfer/consume", map[string]interface{}{
			"session_id": "",
		}, "")
		
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}
