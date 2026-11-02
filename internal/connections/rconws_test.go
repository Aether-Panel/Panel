package connections

import (
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startRCONWSServer answers on wss://host:port/secret and passes on every command it gets
func startRCONWSServer(t *testing.T) (messages chan rconwsMessage, host, port string) {
	messages = make(chan rconwsMessage, 16)
	upgrader := websocket.Upgrader{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/secret" {
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			var m rconwsMessage
			if err := conn.ReadJSON(&m); err != nil {
				return
			}
			messages <- m
		}
	}))
	t.Cleanup(srv.Close)

	previous := rconwsDialer
	rconwsDialer = &websocket.Dialer{TLSClientConfig: srv.Client().Transport.(*http.Transport).TLSClientConfig}
	t.Cleanup(func() { rconwsDialer = previous })

	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err)
	return messages, host, port
}

func TestRCONWSConnection_SendsCommands(t *testing.T) {
	messages, host, port := startRCONWSServer(t)
	tc := &RCONWSConnection{IP: host, Port: port, Password: "secret"}
	tc.Start()
	defer func() { _ = tc.Close() }()
	waitForConnection(t, func() bool { return tc.readyConnection() != nil })

	n, err := tc.Write([]byte("say hi"))

	assert.NoError(t, err)
	assert.Equal(t, len("say hi"), n)
	select {
	case m := <-messages:
		assert.Equal(t, "say hi", m.Message)
		assert.Equal(t, 1, m.Identifier)
	case <-time.After(5 * time.Second):
		t.Fatal("the command never arrived")
	}
}

func TestRCONWSConnection_CloseStopsTheReconnector(t *testing.T) {
	_, host, port := startRCONWSServer(t)
	before := runtime.NumGoroutine()

	tc := &RCONWSConnection{IP: host, Port: port, Password: "secret"}
	tc.Start()
	waitForConnection(t, func() bool { return tc.readyConnection() != nil })
	// commands and Close come from other goroutines than the reconnector; run with -race
	go func() { _, _ = tc.Write([]byte("say bye")) }()
	assert.NoError(t, tc.Close())

	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before)
}
