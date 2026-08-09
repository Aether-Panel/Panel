package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TransferRequest struct {
	NodeID *uint `json:"nodeId" binding:"required"`
}

// @Summary Transfer server
// @Description Transfer a server from one node to another
// @Accept json
// @Produce json
// @Param id path string true "Server ID"
// @Param request body TransferRequest true "Target Node ID"
// @Success 202 {object} string "Transfer started"
// @Failure 400 {object} skypanel.ErrorResponse
// @Tags Servers
// @Router /api/servers/{id}/transfer [post]
// @Security OAuth2Application[server.edit.admin]
func transferServer(c *gin.Context) {
	server := c.MustGet("server").(*domain.Server)

	var req TransferRequest
	if err := c.BindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	db := middleware.GetDatabase(c)
	ns := &node.NodeRepo{DB: db}

	// Get target node
	targetNode, err := ns.Get(*req.NodeID)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if targetNode.ID == server.NodeID {
		response.HandleError(c, errors.New("cannot transfer to the same node"), http.StatusBadRequest)
		return
	}

	go performInternalTransfer(server, targetNode, db)

	c.JSON(http.StatusAccepted, gin.H{"msg": "Transfer started"})
}

func performInternalTransfer(server *domain.Server, targetNode *domain.Node, db *gorm.DB) {
	logging.Info.Printf("Starting transfer of server %s to node %s", server.Identifier, targetNode.Name)

	ns := &node.NodeRepo{DB: db}

	// 1. Get the server definition from the source node
	logging.Info.Printf("Getting server definition from source node")
	defRes, err := ns.CallNode(&server.Node, "GET", fmt.Sprintf("/daemon/server/%s/definition", server.Identifier), nil, nil)
	if err != nil || defRes.StatusCode != 200 {
		logging.Error.Printf("Failed to get server definition for %s: %v", server.Identifier, err)
		return
	}
	defer defRes.Body.Close()

	// 2. Create the server on the target node
	logging.Info.Printf("Creating server on target node")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	createRes, err := ns.CallNode(targetNode, "PUT", fmt.Sprintf("/daemon/server/%s", server.Identifier), defRes.Body, headers)
	if err != nil || createRes.StatusCode != 200 {
		var errorBody string
		if createRes != nil && createRes.Body != nil {
			if bodyBytes, errRead := io.ReadAll(createRes.Body); errRead == nil {
				errorBody = string(bodyBytes)
			}
		}
		logging.Error.Printf("Failed to create server on target node for %s: err=%v, status=%d, body=%s",
			server.Identifier, err, func() int {
				if createRes != nil {
					return createRes.StatusCode
				}
				return 0
			}(), errorBody)
		return
	}
	defer createRes.Body.Close()

	// 3. Stop the server on Source Node
	logging.Info.Printf("Stopping server %s on source node", server.Identifier)
	_, _ = ns.CallNode(&server.Node, "POST", fmt.Sprintf("/daemon/server/%s/power/stop", server.Identifier), nil, nil)
	time.Sleep(5 * time.Second) // Give it some time to stop
	_, _ = ns.CallNode(&server.Node, "POST", fmt.Sprintf("/daemon/server/%s/power/kill", server.Identifier), nil, nil)

	// Clean up old transfer files if they existed
	_, _ = ns.CallNode(&server.Node, "DELETE", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)

	// 4. Archive files on Source Node
	logging.Info.Printf("Archiving files for %s on source node", server.Identifier)
	archiveBody := bytes.NewReader([]byte(`["*"]`))
	headersArch := http.Header{}
	headersArch.Set("Content-Type", "application/json")
	res, err := ns.CallNode(&server.Node, "POST", fmt.Sprintf("/daemon/server/%s/archive/transfer.tar.gz", server.Identifier), io.NopCloser(archiveBody), headersArch)
	if err != nil || (res != nil && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK) {
		errorMsg := "unknown error"
		if res != nil {
			body, _ := io.ReadAll(res.Body)
			errorMsg = string(body)
			res.Body.Close()
		}
		logging.Error.Printf("Failed to archive server %s: %v - %s", server.Identifier, err, errorMsg)
		return
	}
	if res != nil && res.Body != nil {
		res.Body.Close()
	}

	// 5. Download from Source and stream to Target Node via io.Pipe
	logging.Info.Printf("Streaming archive for %s from source to target node", server.Identifier)
	downloadRes, err := ns.CallNode(&server.Node, "GET", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)
	if err != nil || downloadRes.StatusCode != 200 {
		logging.Error.Printf("Failed to start download for %s: %s or status", server.Identifier, err)
		return
	}
	defer downloadRes.Body.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		part, err := writer.CreateFormFile("file", "transfer.tar.gz")
		if err != nil {
			logging.Error.Printf("Error creating form file: %v", err)
			return
		}

		_, err = io.Copy(part, downloadRes.Body)
		if err != nil {
			logging.Error.Printf("Error streaming body: %v", err)
		}
	}()

	headersTransfer := http.Header{}
	headersTransfer.Set("Content-Type", writer.FormDataContentType())

	uploadRes, err := ns.CallNode(targetNode, "PUT", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), pr, headersTransfer)
	if err != nil || uploadRes.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(uploadRes.Body)
		logging.Error.Printf("Failed to upload archive to target node for %s: %v %s", server.Identifier, err, string(body))
		return
	}
	if uploadRes.Body != nil {
		uploadRes.Body.Close()
	}

	// 6. Extract on target node
	logging.Info.Printf("Extracting files on target node for %s", server.Identifier)
	extractRes, err := ns.CallNode(targetNode, "POST", fmt.Sprintf("/daemon/server/%s/extract/transfer.tar.gz?destination=.", server.Identifier), nil, nil)
	if err != nil || (extractRes != nil && extractRes.StatusCode != http.StatusNoContent && extractRes.StatusCode != http.StatusOK) {
		errorMsg := "unknown error"
		if extractRes != nil {
			body, _ := io.ReadAll(extractRes.Body)
			errorMsg = string(body)
			extractRes.Body.Close()
		}
		logging.Error.Printf("Failed to extract on target node for %s: %v - %s", server.Identifier, err, errorMsg)
		return
	}
	if extractRes.Body != nil {
		extractRes.Body.Close()
	}

	// Clean up transfer file on target node
	_, _ = ns.CallNode(targetNode, "DELETE", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)

	// 7. Change DB record
	logging.Info.Printf("Updating database for %s", server.Identifier)

	oldNode := server.Node
	server.NodeID = targetNode.ID
	server.Node = *targetNode

	ss := &ServerRepo{DB: db}
	if err := ss.Update(server); err != nil {
		logging.Error.Printf("Failed to update server config in DB: %v", err)
		return
	}

	// ss.Update explicitly omits 'node_id' for security, and gorm has it marked as '<-:create' only.
	// We MUST force update it here via raw SQL.
	var rawNodeID *uint
	if targetNode.ID != 0 && !targetNode.IsLocal() {
		rawNodeID = &targetNode.ID
	}
	if err := db.Exec("UPDATE servers SET node_id = ? WHERE identifier = ?", rawNodeID, server.Identifier).Error; err != nil {
		logging.Error.Printf("Failed to update server node_id in DB via raw SQL: %v", err)
		return
	}
	server.RawNodeID = rawNodeID

	// 8. Ask source node to delete the server
	_, _ = ns.CallNode(&oldNode, "DELETE", fmt.Sprintf("/daemon/server/%s", server.Identifier), nil, nil)

	logging.Info.Printf("Transfer of %s completed successfully", server.Identifier)
}
