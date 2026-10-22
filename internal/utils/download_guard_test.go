package utils

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestCheckDownloadAddress(t *testing.T) {
	t.Cleanup(func() { _ = config.SecurityBlockPrivateDownloads.Set(false, false) })

	for _, ip := range []string{"127.0.0.1", "::1", "169.254.169.254", "fe80::1", "0.0.0.0", "224.0.0.1"} {
		assert.Error(t, checkDownloadAddress(net.ParseIP(ip), nil), "%s must always be refused", ip)
	}
	assert.NoError(t, checkDownloadAddress(net.ParseIP("1.1.1.1"), nil))

	var warned net.IP
	assert.NoError(t, checkDownloadAddress(net.ParseIP("192.168.1.20"), func(ip net.IP) { warned = ip }), "LAN mirrors keep working by default")
	assert.Equal(t, "192.168.1.20", warned.String())

	_ = config.SecurityBlockPrivateDownloads.Set(true, false)
	assert.Error(t, checkDownloadAddress(net.ParseIP("10.0.0.5"), nil), "private addresses can be blocked too")
}

func TestDownloadFile_RefusesLoopback(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte("secret"))
	}))
	defer server.Close()

	err := DownloadFile(t.TempDir(), server.URL+"/file.txt", nil)

	assert.Error(t, err)
	assert.Zero(t, hits, "the request must not reach the local service")
}
