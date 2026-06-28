package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestNodesAPI(t *testing.T) {
	session, err := createSessionAdmin()
	if !assert.NoError(t, err) {
		return
	}

	var createdNodeId uint

	t.Run("CreateNode", func(t *testing.T) {
		response := CallAPI("POST", "/api/nodes", map[string]interface{}{
			"name":        "TestNode2",
			"description": "Test node description",
			"publicHost":  "127.0.0.1",
			"privateHost": "127.0.0.1",
			"publicPort":  8080,
			"privatePort": 8080,
			"sftpPort":    5657,
		}, session)

		if !assert.Equal(t, http.StatusOK, response.Code) {
			return
		}

		var node models.NodeView
		err := json.NewDecoder(response.Body).Decode(&node)
		assert.NoError(t, err)
		createdNodeId = node.Id
		assert.NotZero(t, createdNodeId)
	})

	t.Run("ListNodes", func(t *testing.T) {
		response := CallAPI("GET", "/api/nodes", nil, session)
		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("DeleteNode", func(t *testing.T) {
		if createdNodeId != 0 {
			url := fmt.Sprintf("/api/nodes/%d", createdNodeId)
			response := CallAPI("DELETE", url, nil, session)
			assert.Equal(t, http.StatusNoContent, response.Code)
		}
	})
}
