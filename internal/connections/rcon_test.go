package connections

import (
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/gorcon/rcon"
	"github.com/gorcon/rcon/rcontest"
	"github.com/stretchr/testify/assert"
)

func startRCONServer(t *testing.T) (*rcontest.Server, string, string) {
	server := rcontest.NewServer(
		rcontest.SetSettings(rcontest.Settings{Password: "secret"}),
		rcontest.SetCommandHandler(func(c *rcontest.Context) {
			_, _ = rcon.NewPacket(rcon.SERVERDATA_RESPONSE_VALUE, c.Request().ID, "ok").WriteTo(c.Conn())
		}),
	)
	t.Cleanup(server.Close)

	host, port, err := net.SplitHostPort(server.Addr())
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return server, host, port
}

func waitUntilReady(t *testing.T, tc *RCONConnection) {
	deadline := time.Now().Add(5 * time.Second)
	for tc.readyConnection() == nil {
		if time.Now().After(deadline) {
			t.Fatal("rcon never connected")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRCONConnection_SendsCommands(t *testing.T) {
	_, host, port := startRCONServer(t)
	tc := &RCONConnection{IP: host, Port: port, Password: "secret"}
	tc.Start()
	waitUntilReady(t, tc)

	n, err := tc.Write([]byte("say hi"))

	assert.NoError(t, err)
	assert.Equal(t, len("say hi"), n)
	assert.NoError(t, tc.Close())
}

func TestRCONConnection_CloseStopsTheReconnector(t *testing.T) {
	_, host, port := startRCONServer(t)
	before := runtime.NumGoroutine()

	tc := &RCONConnection{IP: host, Port: port, Password: "secret"}
	tc.Start()
	waitUntilReady(t, tc)
	_ = tc.Close()

	// the reconnector used to wait forever on a nil channel, leaking one goroutine per stop
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before)
}

func TestRCONConnection_ReconnectsAfterABrokenConnection(t *testing.T) {
	_, host, port := startRCONServer(t)
	tc := &RCONConnection{IP: host, Port: port, Password: "secret"}
	tc.Start()
	waitUntilReady(t, tc)
	defer func() { _ = tc.Close() }()

	// break the connection under the reconnector, as a game server restart would
	_ = tc.readyConnection().Close()

	done := make(chan error, 1)
	go func() {
		_, err := tc.Write([]byte("say hi"))
		done <- err
	}()
	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Write blocked after the connection broke")
	}

	// a new connection is opened and commands work again
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := tc.Write([]byte("say hi")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("rcon never reconnected")
		}
		time.Sleep(200 * time.Millisecond)
	}
}
