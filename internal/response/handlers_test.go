package response

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func handle(err error, status int) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	HandleError(c, err, status)
	return w
}

func TestHandleError_HidesUnexpectedInternalErrors(t *testing.T) {
	res := handle(errors.New(`Error 1146 (42S02): Table 'skypanel.servers' doesn't exist`), http.StatusInternalServerError)

	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.NotContains(t, res.Body.String(), "skypanel.servers", "database details stay in the log")
	assert.Contains(t, res.Body.String(), "ErrInternal")
}

func TestHandleError_KeepsMessagesMeantForTheUser(t *testing.T) {
	res := handle(errors.New("cannot delete server because it has active child servers"), http.StatusBadRequest)
	assert.Contains(t, res.Body.String(), "active child servers", "client errors explain what to fix")

	res = handle(skypanel.ErrDatabaseNotAvailable, http.StatusInternalServerError)
	assert.Contains(t, res.Body.String(), "ErrDatabaseNotAvailable", "the panel's own errors are safe to show")
}
