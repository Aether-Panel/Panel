package daemon

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useStubBucket points the off-site settings at a bucket served by the test. It lists no
// object, or refuses every request while deny is set.
func useStubBucket(t *testing.T) *atomic.Bool {
	deny := &atomic.Bool{}
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		if deny.Load() {
			// AccessDenied is not retried, so the listing fails at once
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
			return
		}
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>backups</Name><Prefix></Prefix><KeyCount>0</KeyCount><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated></ListBucketResult>`))
	}))
	t.Cleanup(stub.Close)

	setString := func(entry interface {
		Value() string
		Set(string, bool) error
	}, value string) {
		previous := entry.Value()
		require.NoError(t, entry.Set(value, false))
		t.Cleanup(func() { _ = entry.Set(previous, false) })
	}
	setString(config.BackupsS3Endpoint, stub.URL)
	setString(config.BackupsS3Region, "us-east-1")
	setString(config.BackupsS3Bucket, "backups")
	setString(config.BackupsS3AccessKey, "key")
	setString(config.BackupsS3SecretKey, "secret")
	previous := config.BackupsS3Enabled.Value()
	require.NoError(t, config.BackupsS3Enabled.Set(true, false))
	t.Cleanup(func() { _ = config.BackupsS3Enabled.Set(previous, false) })
	return deny
}

// The panel keeps its off-site records when the node says the bucket could not be read
func TestListBackups_FlagsABucketThatCouldNotBeListed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, _ := newPluginTestServer(t, "")

	w := runFileHandler(listBackups, server, http.MethodGet, "/", "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, w.Header().Get(servers.BackupsOffsiteHeader), "no bucket is set up")

	deny := useStubBucket(t)
	w = runFileHandler(listBackups, server, http.MethodGet, "/", "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, w.Header().Get(servers.BackupsOffsiteHeader), "the bucket was listed")

	deny.Store(true)
	w = runFileHandler(listBackups, server, http.MethodGet, "/", "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, servers.BackupsOffsiteUnavailable, w.Header().Get(servers.BackupsOffsiteHeader))
	assert.JSONEq(t, `[]`, w.Body.String(), "the body keeps its shape")
}
