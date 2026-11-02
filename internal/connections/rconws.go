package connections

import (
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/query"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"io"
	"net"
	"net/url"
	"sync"
	"time"
)

type RCONWSConnection struct {
	io.WriteCloser
	IP        string
	Port      string
	Password  string
	NoTLS     bool
	Reconnect bool
	// Environment      skypanel.Environment
	connection *websocket.Conn
	ready      bool
	identifier int
	// identifierLocker lets one command at a time be written: a websocket takes one writer
	identifierLocker sync.Mutex
	// lock guards Reconnect, connection and ready: commands, Close and the reconnector run on
	// different goroutines, as with RCON
	lock sync.Mutex
}

type rconwsMessage struct {
	Message    string `json:"Message"`
	Identifier int    `json:"Identifier"`
	Stacktrace string `json:"Stacktrace"`
}

// rconwsDialer opens the connection; a variable so tests can trust their own certificate
var rconwsDialer = websocket.DefaultDialer

// rconwsURL is the address of the game's RCON websocket, which takes the password as the
// path. The password is escaped: a "/", "?", "#" or "%" in it would otherwise end the path or
// start a query, and the game would get another password. It is wss:// unless the template's
// console says "secure": false (NoTLS), for games that serve plain ws:// such as Rust.
func rconwsURL(addr *net.TCPAddr, password string, secure bool) string {
	scheme := "ws"
	if secure {
		scheme = "wss"
	}
	return fmt.Sprintf("%s://%s/%s", scheme, addr, url.PathEscape(password))
}

func (tc *RCONWSConnection) Write(p []byte) (n int, err error) {
	conn := tc.readyConnection()
	if conn == nil {
		time.Sleep(1 * time.Second)
		conn = tc.readyConnection()
		if conn == nil {
			return 0, errors.New("rconws not available")
		}
	}
	tc.identifierLocker.Lock()
	defer tc.identifierLocker.Unlock()
	tc.identifier++
	return len(p), conn.WriteJSON(&rconwsMessage{
		Message:    string(p),
		Identifier: tc.identifier,
	})
}

func (tc *RCONWSConnection) Start() {
	tc.lock.Lock()
	tc.Reconnect = true
	// no address, or every address, means the game itself: this machine, or the host when
	// the panel runs in Docker, as the game queries resolve it
	tc.IP = query.HostOrLocal(tc.IP)
	tc.lock.Unlock()

	go tc.reconnector()
}

func (tc *RCONWSConnection) Close() error {
	tc.lock.Lock()
	tc.Reconnect = false
	conn := tc.connection
	tc.lock.Unlock()

	if conn == nil {
		return nil
	}
	// the reconnector closes a broken connection too; whichever goes second is not an error
	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func (tc *RCONWSConnection) readyConnection() *websocket.Conn {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	if !tc.ready {
		return nil
	}
	return tc.connection
}

func (tc *RCONWSConnection) shouldReconnect() bool {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	return tc.Reconnect
}

// attach makes conn the one commands go to. It refuses it when the connection was closed
// while it was being opened, so a stopped server does not keep a socket open.
func (tc *RCONWSConnection) attach(conn *websocket.Conn) bool {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	if !tc.Reconnect {
		return false
	}
	tc.connection = conn
	tc.ready = true
	return true
}

func (tc *RCONWSConnection) setNotReady() {
	tc.lock.Lock()
	tc.ready = false
	tc.lock.Unlock()
}

func (tc *RCONWSConnection) reconnector() {
	init := true
	for tc.shouldReconnect() {
		tc.setNotReady()
		if !init {
			time.Sleep(5 * time.Second)
		} else {
			init = false
		}
		if !tc.shouldReconnect() {
			return
		}

		tc.lock.Lock()
		ipAddr := &net.TCPAddr{
			IP:   net.ParseIP(tc.IP),
			Port: cast.ToInt(tc.Port),
		}
		password := tc.Password
		secure := !tc.NoTLS
		tc.lock.Unlock()

		conn, _, err := rconwsDialer.Dial(rconwsURL(ipAddr, password, secure), nil)
		if err != nil {
			logging.Debug.Printf("Error waiting for RCON WS socket: %s", err.Error())
			continue
		}

		if !tc.attach(conn) {
			_ = conn.Close()
			return
		}
		listening := true
		for listening {
			_, _, err = conn.ReadMessage()
			if err != nil {
				listening = false
			}
		}
		_ = conn.Close()
	}
}
