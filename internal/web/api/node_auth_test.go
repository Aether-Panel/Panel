package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeRefusedPanel(t *testing.T) {
	challenge := http.Header{"Www-Authenticate": {`Bearer error="invalid_token"`}}
	cases := []struct {
		name   string
		res    *http.Response
		refuse bool
	}{
		{"the node's 401", &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}}, true},
		{"a 403 asking for credentials", &http.Response{StatusCode: http.StatusForbidden, Header: challenge}, true},
		{"the node's own 403", &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}}, false},
		{"a refusal of the request", &http.Response{StatusCode: http.StatusConflict, Header: challenge}, false},
		{"success", &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}, false},
		{"no answer", nil, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.refuse, nodeRefusedPanel(c.res), c.name)
	}
}

// fakeRemoteNode is a node that answers every request with the given status and headers
func fakeRemoteNode(t *testing.T, status int, header http.Header) *models.Node {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			// the panel's probe for TLS
			w.WriteHeader(http.StatusOK)
			return
		}
		for k, v := range header {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":{"code":"ErrFromNode"}}`))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	return &models.Node{Name: "remote", PublicHost: "127.0.0.1", PrivateHost: "127.0.0.1", PublicPort: uint16(port), PrivatePort: uint16(port)}
}

func proxyTo(t *testing.T, node *models.Node) *httptest.ResponseRecorder {
	require.False(t, node.IsLocal(), "the test needs a remote node")
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/servers/srv1/file/world", nil)
	c.Params = gin.Params{{Key: "serverId", Value: "srv1"}}

	proxyHTTPRequest(c, "/daemon/server/srv1/file/world", &services.Node{}, node)
	c.Writer.WriteHeaderNow()
	return w
}

// A node that refuses the panel's token must not reach the browser as a 401: the web client
// takes any 401 for its own session ending and sends the user to the login page, again and
// again while the node's secret or clock is wrong.
func TestProxyHTTPRequest_NodeAuthFailureIsBadGateway(t *testing.T) {
	for _, header := range []http.Header{
		{"Www-Authenticate": {`Bearer error="invalid_token"`}},
		{},
	} {
		w := proxyTo(t, fakeRemoteNode(t, http.StatusUnauthorized, header))

		assert.Equal(t, http.StatusBadGateway, w.Code)
		assert.Empty(t, w.Header().Get("Www-Authenticate"), "the node's challenge is not passed on")
		var answer struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
		assert.Equal(t, "ErrNodeAuthFailed", answer.Error.Code)
	}

	w := proxyTo(t, fakeRemoteNode(t, http.StatusForbidden, http.Header{"Www-Authenticate": {"Basic"}}))
	assert.Equal(t, http.StatusBadGateway, w.Code, "a proxy in front of the node asking for credentials")
}

// The node's other answers, its own 403 included, still reach the browser as they are
func TestProxyHTTPRequest_PassesTheNodesOwnAnswers(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusConflict, http.StatusBadRequest} {
		w := proxyTo(t, fakeRemoteNode(t, status, http.Header{}))

		assert.Equal(t, status, w.Code)
		assert.Contains(t, w.Body.String(), "ErrFromNode")
	}
}
