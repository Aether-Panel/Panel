package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clientIPSeen is the address a handler gets for a request from remote carrying xff
func clientIPSeen(t *testing.T, configured []string, remote, xff string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(trustedProxyList(configured)))
	var seen string
	router.GET("/", func(c *gin.Context) { seen = c.ClientIP() })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	router.ServeHTTP(httptest.NewRecorder(), req)
	return seen
}

func TestClientIP_BehindTheInstallersNginx(t *testing.T) {
	// Nginx on the host reaches the container through the Docker gateway
	assert.Equal(t, "203.0.113.7", clientIPSeen(t, nil, "172.18.0.1:51000", "203.0.113.7"))
}

func TestClientIP_BehindCloudflareAndNginx(t *testing.T) {
	// Cloudflare adds the visitor, Nginx adds the Cloudflare edge it saw
	assert.Equal(t, "203.0.113.7", clientIPSeen(t, nil, "172.18.0.1:51000", "203.0.113.7, 2606:4700:3031::6815:2603"))
	assert.Equal(t, "203.0.113.7", clientIPSeen(t, nil, "172.18.0.1:51000", "203.0.113.7, 162.158.10.20"))
}

func TestClientIP_AWrittenHeaderCannotFakeTheAddress(t *testing.T) {
	// the visitor writes an address; Nginx appends the one it really saw, which wins
	assert.Equal(t, "198.51.100.9", clientIPSeen(t, nil, "172.18.0.1:51000", "1.2.3.4, 198.51.100.9"))
	// straight to the panel from the internet: the header is ignored
	assert.Equal(t, "198.51.100.9", clientIPSeen(t, nil, "198.51.100.9:40000", "1.2.3.4"))
}

func TestClientIP_ConfiguredProxiesAreAdded(t *testing.T) {
	assert.Equal(t, "203.0.113.7", clientIPSeen(t, []string{"198.51.100.0/24"}, "198.51.100.2:40000", "203.0.113.7"))
	assert.Equal(t, "198.51.100.2", clientIPSeen(t, nil, "198.51.100.2:40000", "203.0.113.7"), "not trusted unless listed")
}

func TestTrustedProxyList_NoDuplicates(t *testing.T) {
	list := trustedProxyList([]string{"172.18.0.1", "10.0.0.0/8", ""})
	assert.Contains(t, list, "172.18.0.1")
	seen := map[string]int{}
	for _, p := range list {
		seen[p]++
	}
	for p, n := range seen {
		assert.Equal(t, 1, n, p)
	}
	assert.NotContains(t, list, "")
}

func TestAlwaysTrustedProxies_EveryLineIsARange(t *testing.T) {
	require.NotEmpty(t, alwaysTrustedProxies)
	for _, proxy := range alwaysTrustedProxies {
		_, _, err := net.ParseCIDR(proxy)
		assert.NoError(t, err, proxy)
	}
	assert.Contains(t, alwaysTrustedProxies, "172.16.0.0/12", "the Docker gateway range")
	assert.Contains(t, alwaysTrustedProxies, "2606:4700::/32", "a Cloudflare range")
}
