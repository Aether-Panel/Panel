package skypanel

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 1024
)

type Tracker struct {
	sockets []*Socket
	locker  sync.Mutex
}

func CreateTracker() *Tracker {
	return &Tracker{sockets: make([]*Socket, 0)}
}

func (ws *Tracker) Register(conn *Socket) {
	ws.locker.Lock()
	defer ws.locker.Unlock()
	ws.sockets = append(ws.sockets, conn)
}

// Remove drops a socket from the tracker if it is present.
func (ws *Tracker) Remove(conn *Socket) {
	ws.locker.Lock()
	defer ws.locker.Unlock()
	for i, k := range ws.sockets {
		if k == conn {
			ws.sockets[i] = ws.sockets[len(ws.sockets)-1]
			ws.sockets[len(ws.sockets)-1] = nil
			ws.sockets = ws.sockets[:len(ws.sockets)-1]
			return
		}
	}
}

func (ws *Tracker) WriteMessage(msg Transmission) error {
	d, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	ws.locker.Lock()
	sockets := make([]*Socket, len(ws.sockets))
	copy(sockets, ws.sockets)
	ws.locker.Unlock()

	for _, conn := range sockets {
		go func(c *Socket) {
			if _, err := c.Write(d); err != nil {
				logging.Debug.Printf("websocket encountered error, dropping (%s)", err.Error())
				ws.Remove(c)
			}
		}(conn)
	}

	return nil
}

func (ws *Tracker) Write(source []byte) (n int, e error) {
	packet := ServerLogs{Logs: source}
	e = ws.WriteMessage(Transmission{
		Message: packet,
		Type:    MessageTypeLog,
	})
	n = len(source)
	return
}

func Create(ws *websocket.Conn) *Socket {
	return &Socket{conn: ws}
}

type Socket struct {
	conn     *websocket.Conn
	locker   sync.Mutex
	trackers []*Tracker
	closed   bool
	io.WriteCloser
}

func (s *Socket) attach(tracker *Tracker) {
	s.trackers = append(s.trackers, tracker)
}

func (s *Socket) markClosed() {
	s.locker.Lock()
	s.closed = true
	s.locker.Unlock()
}

func (s *Socket) storeTrackers() []*Tracker {
	s.locker.Lock()
	defer s.locker.Unlock()
	return s.trackers
}

// Serve keeps the connection alive and detects dead peers. It sends ping
// frames periodically, processes ping/pong close control frames by reading,
// and removes the socket from every tracker it was registered on once the
// connection dies. It blocks until the connection is closed.
func (s *Socket) Serve() {
	s.conn.SetReadLimit(maxMessageSize)
	_ = s.conn.SetReadDeadline(time.Now().Add(pongWait))
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	go func() {
		for range pingTicker.C {
			if s.isClosed() {
				return
			}
			if err := s.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				s.teardown()
				return
			}
		}
	}()

	for {
		if _, _, err := s.conn.ReadMessage(); err != nil {
			s.teardown()
			return
		}
	}
}

func (s *Socket) isClosed() bool {
	s.locker.Lock()
	defer s.locker.Unlock()
	return s.closed
}

// teardown closes the connection and unregisters the socket from all
// trackers it was attached to.
func (s *Socket) teardown() {
	s.markClosed()
	_ = s.conn.Close()
	for _, tracker := range s.storeTrackers() {
		tracker.Remove(s)
	}
}

func (s *Socket) WriteMessage(msg Transmission) error {
	return s.WriteJSON(&msg)
}

func (s *Socket) Write(data []byte) (int, error) {
	s.locker.Lock()
	defer s.locker.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return len(data), s.conn.WriteMessage(websocket.TextMessage, data)
}

func (s *Socket) WriteJSON(data interface{}) error {
	d, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = s.Write(d)
	return err
}

func (s *Socket) Close() error {
	s.teardown()
	return nil
}