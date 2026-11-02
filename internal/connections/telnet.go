package connections

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/query"
	"github.com/spf13/cast"
	"io"
	"net"
	"sync"
	"time"
)

type TelnetConnection struct {
	io.WriteCloser
	IP         string
	Port       string
	Password   string
	Reconnect  bool
	connection *net.TCPConn
	ready      bool
	// lock guards the fields above: commands, Close and the reconnector run on different
	// goroutines, as with RCON
	lock sync.Mutex
}

func (tc *TelnetConnection) Write(p []byte) (n int, err error) {
	conn := tc.readyConnection()
	if conn == nil {
		time.Sleep(1 * time.Second)
		conn = tc.readyConnection()
		if conn == nil {
			return 0, errors.New("telnet not available")
		}
	}
	return conn.Write(p)
}

func (tc *TelnetConnection) Start() {
	tc.lock.Lock()
	tc.Reconnect = true
	// no address, or every address, means the game itself: this machine, or the host when
	// the panel runs in Docker, as the game queries resolve it
	tc.IP = query.HostOrLocal(tc.IP)
	tc.lock.Unlock()

	go tc.reconnector()
}

func (tc *TelnetConnection) Close() error {
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

func (tc *TelnetConnection) readyConnection() *net.TCPConn {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	if !tc.ready {
		return nil
	}
	return tc.connection
}

func (tc *TelnetConnection) shouldReconnect() bool {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	return tc.Reconnect
}

// attach makes conn the one commands go to. It refuses it when the connection was closed
// while it was being opened, so a stopped server does not keep a socket open.
func (tc *TelnetConnection) attach(conn *net.TCPConn) bool {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	if !tc.Reconnect {
		return false
	}
	tc.connection = conn
	tc.ready = true
	return true
}

func (tc *TelnetConnection) setNotReady() {
	tc.lock.Lock()
	tc.ready = false
	tc.lock.Unlock()
}

func (tc *TelnetConnection) reconnector() {
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
		tc.lock.Unlock()

		conn, err := net.DialTCP("tcp", nil, ipAddr)
		if err != nil {
			logging.Debug.Printf("Error waiting for TCP TELNET socket: %s", err.Error())
			continue
		}
		_ = conn.SetKeepAlive(true)

		// wait a second for the prompt for passwords/other delays
		time.Sleep(1 * time.Second)

		if password != "" {
			_, err = conn.Write([]byte(password + "\n"))
			if err != nil {
				logging.Debug.Printf("Error sending password for TCP TELNET socket: %s", err.Error())
				_ = conn.Close()
				continue
			}
		}
		if !tc.attach(conn) {
			_ = conn.Close()
			return
		}
		listening := true
		for listening {
			buf := make([]byte, 1024)
			_, err = conn.Read(buf)
			if err != nil {
				listening = false
			}
		}
		_ = conn.Close()
	}
}
