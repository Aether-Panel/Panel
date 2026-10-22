package connections

import (
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/gorcon/rcon"
	"io"
	"sync"
	"time"
)

type RCONConnection struct {
	io.WriteCloser
	IP         string
	Port       string
	Password   string
	Reconnect  bool
	connection *rcon.Conn
	ready      bool
	closer     chan error
	lock       sync.Mutex
}

func (tc *RCONConnection) Write(p []byte) (n int, err error) {
	conn := tc.readyConnection()
	if conn == nil {
		time.Sleep(1 * time.Second)
		conn = tc.readyConnection()
		if conn == nil {
			return 0, errors.New("rcon not available")
		}
	}

	var res string
	res, err = conn.Execute(string(p))
	if err != nil {
		// drop this connection so the reconnector opens a new one
		tc.signalClose(err)
	}
	logging.Debug.Printf("RCON Response: %s\n", res)
	return len(p), err
}

func (tc *RCONConnection) Start() {
	tc.lock.Lock()
	tc.Reconnect = true
	if tc.IP == "" {
		tc.IP = "127.0.0.1"
	}
	tc.closer = make(chan error, 1)
	tc.lock.Unlock()

	go tc.reconnector()
}

func (tc *RCONConnection) Close() error {
	tc.lock.Lock()
	tc.Reconnect = false
	conn := tc.connection
	tc.lock.Unlock()

	// wake the reconnector up so it can exit
	tc.signalClose(nil)
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func (tc *RCONConnection) readyConnection() *rcon.Conn {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	if !tc.ready {
		return nil
	}
	return tc.connection
}

func (tc *RCONConnection) shouldReconnect() bool {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	return tc.Reconnect
}

// signalClose tells recon to drop the current connection; it never blocks
func (tc *RCONConnection) signalClose(err error) {
	tc.lock.Lock()
	closer := tc.closer
	tc.lock.Unlock()

	select {
	case closer <- err:
	default:
	}
}

func (tc *RCONConnection) setReady(conn *rcon.Conn, ready bool) {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	if conn != nil {
		tc.connection = conn
	}
	tc.ready = ready
}

func (tc *RCONConnection) reconnector() {
	init := true
	for tc.shouldReconnect() {
		tc.recon(init)
		if init {
			init = false
		}
	}
}

func (tc *RCONConnection) recon(init bool) {
	tc.setReady(nil, false)
	if !init {
		time.Sleep(5 * time.Second)
	}
	if !tc.shouldReconnect() {
		return
	}

	tc.lock.Lock()
	address := fmt.Sprintf("%s:%s", tc.IP, tc.Port)
	closer := tc.closer
	tc.lock.Unlock()

	conn, err := rcon.Dial(address, tc.Password)
	if err != nil {
		logging.Debug.Printf("Error connecting to RCON: %s", err.Error())
		return
	}
	tc.setReady(conn, true)
	defer utils.Close(conn)
	<-closer
	tc.setReady(nil, false)
}
