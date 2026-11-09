package stats

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceIDIsStableAndHidesTheKey(t *testing.T) {
	key := "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
	id := instanceID(key)

	assert.Len(t, id, 32)
	assert.Equal(t, id, instanceID(key), "the same key gives the same id")
	assert.NotEqual(t, id, instanceID(key+"0"), "another key gives another id")
	assert.NotContains(t, key, id)
}

func TestSendPostsTheReport(t *testing.T) {
	var got report
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.True(t, strings.HasPrefix(r.Header.Get("User-Agent"), "AetherPanel/"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	want := report{ID: instanceID("key"), Version: "2.2.0", Servers: 7}
	require.NoError(t, send(srv.Client(), srv.URL, want))
	assert.Equal(t, want, got)
}

func TestSendReportsARejectedPing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	assert.Error(t, send(srv.Client(), srv.URL, report{ID: "x", Version: "2.2.0"}))
}
