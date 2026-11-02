package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeNode answers every call with the status statusFor gives it and records the calls
type fakeNode struct {
	calls     []string
	statusFor func(call string) int
}

func (f *fakeNode) call(method, path string, body io.ReadCloser, _ http.Header) (*http.Response, error) {
	if body != nil {
		_ = body.Close()
	}
	call := method + " " + path
	f.calls = append(f.calls, call)
	status := http.StatusNoContent
	if f.statusFor != nil {
		status = f.statusFor(call)
	}
	if status == 0 {
		return nil, errors.New("node unreachable")
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("node says no"))}, nil
}

func TestBuildOriginArchive_RenamesTheArchiveOnceComplete(t *testing.T) {
	node := &fakeNode{}

	require.NoError(t, buildOriginArchive(node.call, "srv01"))

	assert.Equal(t, []string{
		"DELETE /daemon/server/srv01/file/transfer.tar.gz",
		"DELETE /daemon/server/srv01/file/.transfer-partial.tar.gz",
		"POST /daemon/server/srv01/archive/.transfer-partial.tar.gz",
		"POST /daemon/server/srv01/file/.transfer-partial.tar.gz?destination=transfer.tar.gz",
	}, node.calls, "the download only finds transfer.tar.gz once it is complete")
}

func TestBuildOriginArchive_AFailedArchiveIsRemoved(t *testing.T) {
	node := &fakeNode{statusFor: func(call string) int {
		if strings.HasPrefix(call, "POST /daemon/server/srv01/archive/") {
			return http.StatusInsufficientStorage
		}
		return http.StatusNoContent
	}}

	err := buildOriginArchive(node.call, "srv01")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "node says no")
	assert.NotContains(t, node.calls, "POST /daemon/server/srv01/file/.transfer-partial.tar.gz?destination=transfer.tar.gz", "half an archive is never renamed")
	assert.Equal(t, "DELETE /daemon/server/srv01/file/.transfer-partial.tar.gz", node.calls[len(node.calls)-1])
}

func TestBuildOriginArchive_AFailedRenameFails(t *testing.T) {
	node := &fakeNode{statusFor: func(call string) int {
		if strings.Contains(call, "?destination=") {
			return 0
		}
		return http.StatusNoContent
	}}

	assert.Error(t, buildOriginArchive(node.call, "srv01"))
	assert.Equal(t, "DELETE /daemon/server/srv01/file/.transfer-partial.tar.gz", node.calls[len(node.calls)-1])
}

func TestValidateExTransfer_RefusesAnotherProtocolVersion(t *testing.T) {
	db := setupExTransferTest(t)
	destKey, _ := newExTestKey(t)
	session := newExTestSession(t, db, models.StatusCreated, destKey, nil)

	body, _ := json.Marshal(ExValidateReq{Token: "whatever", TargetPublicKey: "key", ProtocolVer: "2.0"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("db", db)
	validateExTransfer(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "UNSUPPORTED_PROTOCOL")
	assert.Equal(t, models.StatusCreated, reloadExSession(t, db, session).Status, "the session is left alone")
}
