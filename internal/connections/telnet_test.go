package connections

import (
	"bufio"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startTelnetServer accepts connections and passes on every line they send
func startTelnetServer(t *testing.T) (lines chan string, host, port string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	lines = make(chan string, 16)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				scanner := bufio.NewScanner(c)
				for scanner.Scan() {
					lines <- scanner.Text()
				}
			}(conn)
		}
	}()

	host, port, err = net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return lines, host, port
}

func nextLine(t *testing.T, lines chan string) string {
	select {
	case line := <-lines:
		return line
	case <-time.After(5 * time.Second):
		t.Fatal("nothing was sent")
		return ""
	}
}

func waitForConnection(t *testing.T, ready func() bool) {
	deadline := time.Now().Add(5 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal("never connected")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTelnetConnection_SendsThePasswordThenCommands(t *testing.T) {
	lines, host, port := startTelnetServer(t)
	tc := &TelnetConnection{IP: host, Port: port, Password: "secret"}
	tc.Start()
	defer func() { _ = tc.Close() }()
	waitForConnection(t, func() bool { return tc.readyConnection() != nil })

	n, err := tc.Write([]byte("say hi\n"))

	assert.NoError(t, err)
	assert.Equal(t, len("say hi\n"), n)
	assert.Equal(t, "secret", nextLine(t, lines))
	assert.Equal(t, "say hi", nextLine(t, lines))
}

func TestTelnetConnection_CloseStopsTheReconnector(t *testing.T) {
	_, host, port := startTelnetServer(t)
	before := runtime.NumGoroutine()

	tc := &TelnetConnection{IP: host, Port: port}
	tc.Start()
	waitForConnection(t, func() bool { return tc.readyConnection() != nil })
	// commands and Close come from other goroutines than the reconnector; run with -race
	go func() { _, _ = tc.Write([]byte("say bye\n")) }()
	assert.NoError(t, tc.Close())

	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before)
}

func TestTelnetConnection_ClosedWhileConnectingKeepsNoSocket(t *testing.T) {
	_, host, port := startTelnetServer(t)
	tc := &TelnetConnection{IP: host, Port: port}
	tc.Start()
	// closed before the reconnector, which waits a second for the prompt, takes the socket
	assert.NoError(t, tc.Close())

	time.Sleep(2 * time.Second)
	assert.Nil(t, tc.readyConnection(), "a closed connection does not come back")
}
