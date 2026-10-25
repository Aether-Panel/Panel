package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
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
	server := c.MustGet("server").(*models.Server)

	var req TransferRequest
	if err := c.BindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}

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

func performInternalTransfer(server *models.Server, targetNode *models.Node, db *gorm.DB) {
	logging.Info.Printf("Starting transfer of server %s to node %s", server.Identifier, targetNode.Name)

	ns := &services.Node{DB: db}

	// 1+2. Copy the server definition from the source node to the target node
	if !copyDefinitionToTarget(ns, server, targetNode) {
		return
	}

	// 3+4. Stop the server and archive its files on the source node
	if !stopAndArchiveOnSource(ns, server) {
		return
	}

	// 5. Download from Source and stream to Target Node via io.Pipe
	if !streamArchiveToTarget(ns, server, targetNode) {
		return
	}

	// 6. Extract on target node
	if !extractArchiveOnTarget(ns, server, targetNode) {
		return
	}

	// Clean up transfer file on target node
	_, _ = ns.CallNode(targetNode, "DELETE", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)

	// 7. Change DB record
	logging.Info.Printf("Updating database for %s", server.Identifier)

	oldNode := server.Node
	if !moveServerRecordToNode(db, server, targetNode) {
		return
	}

	// 8. Ask source node to delete the server
	_, _ = ns.CallNode(&oldNode, "DELETE", fmt.Sprintf("/daemon/server/%s", server.Identifier), nil, nil)

	logging.Info.Printf("Transfer of %s completed successfully", server.Identifier)
}

// copyDefinitionToTarget reads the server definition from the source node and
// creates the server with it on the target node.
func copyDefinitionToTarget(ns *services.Node, server *models.Server, targetNode *models.Node) bool {
	// 1. Get the server definition from the source node
	logging.Info.Printf("Getting server definition from source node")
	defRes, err := ns.CallNode(&server.Node, "GET", fmt.Sprintf("/daemon/server/%s/definition", server.Identifier), nil, nil)
	if err != nil || defRes.StatusCode != 200 {
		logging.Error.Printf("Failed to get server definition for %s: %v", server.Identifier, err)
		return false
	}
	defer defRes.Body.Close()

	// 2. Create the server on the target node
	logging.Info.Printf("Creating server on target node")
	headers := http.Header{}
	headers.Set(headerContentType, "application/json")
	createRes, err := ns.CallNode(targetNode, "PUT", fmt.Sprintf("/daemon/server/%s", server.Identifier), defRes.Body, headers)
	if err != nil || createRes.StatusCode != 200 {
		var errorBody string
		if createRes != nil && createRes.Body != nil {
			if bodyBytes, errRead := io.ReadAll(createRes.Body); errRead == nil {
				errorBody = string(bodyBytes)
			}
		}
		logging.Error.Printf("Failed to create server on target node for %s: err=%v, status=%d, body=%s",
			server.Identifier, err, statusCodeOf(createRes), errorBody)
		return false
	}
	defer createRes.Body.Close()
	return true
}

// statusCodeOf is the response's status code, or 0 when there is no response.
func statusCodeOf(res *http.Response) int {
	if res != nil {
		return res.StatusCode
	}
	return 0
}

// isDaemonSuccess reports whether a daemon action answered with 200 or 204.
// A missing response without an error counts as success, as it always has.
func isDaemonSuccess(res *http.Response, err error) bool {
	return err == nil && (res == nil || res.StatusCode == http.StatusNoContent || res.StatusCode == http.StatusOK)
}

// readFailureBody reads and closes the body of a failed daemon response.
func readFailureBody(res *http.Response) string {
	if res == nil {
		return "unknown error"
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return string(body)
}

// stopAndArchiveOnSource stops the server on the source node and packs its
// files into transfer.tar.gz.
func stopAndArchiveOnSource(ns *services.Node, server *models.Server) bool {
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
	headersArch.Set(headerContentType, "application/json")
	res, err := ns.CallNode(&server.Node, "POST", fmt.Sprintf("/daemon/server/%s/archive/transfer.tar.gz", server.Identifier), io.NopCloser(archiveBody), headersArch)
	if !isDaemonSuccess(res, err) {
		logging.Error.Printf("Failed to archive server %s: %v - %s", server.Identifier, err, readFailureBody(res))
		return false
	}
	if res != nil && res.Body != nil {
		res.Body.Close()
	}
	return true
}

// streamArchiveToTarget downloads transfer.tar.gz from the source node and
// uploads it to the target node through a pipe, without buffering it.
func streamArchiveToTarget(ns *services.Node, server *models.Server, targetNode *models.Node) bool {
	logging.Info.Printf("Streaming archive for %s from source to target node", server.Identifier)
	downloadRes, err := ns.CallNode(&server.Node, "GET", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), nil, nil)
	if err != nil || downloadRes.StatusCode != 200 {
		logging.Error.Printf("Failed to start download for %s: %s or status", server.Identifier, err)
		return false
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
	headersTransfer.Set(headerContentType, writer.FormDataContentType())

	uploadRes, err := ns.CallNode(targetNode, "PUT", fmt.Sprintf("/daemon/server/%s/file/transfer.tar.gz", server.Identifier), pr, headersTransfer)
	if err != nil || uploadRes.StatusCode != http.StatusNoContent {
		// uploadRes is nil when the call itself failed
		var body []byte
		if uploadRes != nil {
			body, _ = io.ReadAll(uploadRes.Body)
		}
		logging.Error.Printf("Failed to upload archive to target node for %s: %v %s", server.Identifier, err, string(body))
		return false
	}
	if uploadRes.Body != nil {
		uploadRes.Body.Close()
	}
	return true
}

// extractArchiveOnTarget unpacks transfer.tar.gz into the server's folder on
// the target node.
func extractArchiveOnTarget(ns *services.Node, server *models.Server, targetNode *models.Node) bool {
	logging.Info.Printf("Extracting files on target node for %s", server.Identifier)
	extractRes, err := ns.CallNode(targetNode, "POST", fmt.Sprintf("/daemon/server/%s/extract/transfer.tar.gz?destination=.", server.Identifier), nil, nil)
	if !isDaemonSuccess(extractRes, err) {
		logging.Error.Printf("Failed to extract on target node for %s: %v - %s", server.Identifier, err, readFailureBody(extractRes))
		return false
	}
	if extractRes.Body != nil {
		extractRes.Body.Close()
	}
	return true
}

// moveServerRecordToNode points the server's database record at the target
// node.
func moveServerRecordToNode(db *gorm.DB, server *models.Server, targetNode *models.Node) bool {
	server.NodeID = targetNode.ID
	server.Node = *targetNode

	ss := &services.Server{DB: db}
	if err := ss.Update(server); err != nil {
		logging.Error.Printf("Failed to update server config in DB: %v", err)
		return false
	}

	// ss.Update explicitly omits 'node_id' for security, and gorm has it marked as '<-:create' only.
	// We MUST force update it here via raw SQL.
	var rawNodeID *uint
	if targetNode.ID != 0 && !targetNode.IsLocal() {
		rawNodeID = &targetNode.ID
	}
	if err := db.Exec("UPDATE servers SET node_id = ? WHERE identifier = ?", rawNodeID, server.Identifier).Error; err != nil {
		logging.Error.Printf("Failed to update server node_id in DB via raw SQL: %v", err)
		return false
	}
	server.RawNodeID = rawNodeID
	return true
}
