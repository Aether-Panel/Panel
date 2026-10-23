package query

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"time"
)

// Source answers Valve's A2S queries, which most Steam dedicated servers speak (Source and
// GoldSrc games, Rust, ARK, Valheim, CS2, Garry's Mod, Unturned...).
// https://developer.valvesoftware.com/wiki/Server_queries

type SourceResponse struct {
	Name       string   `json:"name"`
	Map        string   `json:"map"`
	Game       string   `json:"game"`
	NumPlayers int      `json:"numPlayers"`
	MaxPlayers int      `json:"maxPlayers"`
	Bots       int      `json:"bots"`
	Players    []string `json:"players"`
}

const sourceTimeout = 1500 * time.Millisecond

var (
	errSourceShort    = errors.New("a2s: short response")
	errSourceResponse = errors.New("a2s: unexpected response")
)

var a2sInfoRequest = append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 'T'}, append([]byte("Source Engine Query"), 0)...)

// Source asks a Steam server for its name, map and players. Player names are best effort:
// some games hide them, and then only the counts are filled in.
func Source(ip string, port int) (SourceResponse, error) {
	if port <= 0 {
		return SourceResponse{}, errors.New("port is required")
	}
	conn, err := net.DialTimeout("udp", net.JoinHostPort(hostOrLocal(ip), strconv.Itoa(port)), sourceTimeout)
	if err != nil {
		return SourceResponse{}, err
	}
	defer func() { _ = conn.Close() }()

	info, err := sourceRequest(conn, a2sInfoRequest, 'I')
	if err != nil {
		return SourceResponse{}, err
	}
	res, err := parseSourceInfo(info)
	if err != nil {
		return SourceResponse{}, err
	}

	if players, err := sourceRequest(conn, []byte{0xFF, 0xFF, 0xFF, 0xFF, 'U', 0xFF, 0xFF, 0xFF, 0xFF}, 'D'); err == nil {
		res.Players = parseSourcePlayers(players)
	}
	if res.Players == nil {
		res.Players = []string{}
	}
	return res, nil
}

// sourceRequest sends a request and answers one challenge if the server asks for it
func sourceRequest(conn net.Conn, request []byte, want byte) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if err := conn.SetDeadline(time.Now().Add(sourceTimeout)); err != nil {
			return nil, err
		}
		if _, err := conn.Write(request); err != nil {
			return nil, err
		}
		buf := make([]byte, 1400)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		packet := buf[:n]
		// only single packets are handled; split responses are rare for info and players
		if len(packet) < 5 || !bytes.Equal(packet[:4], []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
			return nil, errSourceResponse
		}
		switch packet[4] {
		case want:
			return packet[5:], nil
		case 'A':
			if len(packet) < 9 {
				return nil, errSourceShort
			}
			challenge := packet[5:9]
			if request[4] == 'U' {
				request = append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 'U'}, challenge...)
			} else {
				request = append(append([]byte{}, a2sInfoRequest...), challenge...)
			}
		default:
			return nil, errSourceResponse
		}
	}
	return nil, errSourceResponse
}

type sourceReader struct {
	data []byte
	err  error
}

func (r *sourceReader) byte() byte {
	if r.err != nil || len(r.data) < 1 {
		r.err = errSourceShort
		return 0
	}
	b := r.data[0]
	r.data = r.data[1:]
	return b
}

func (r *sourceReader) skip(n int) {
	if r.err != nil || len(r.data) < n {
		r.err = errSourceShort
		return
	}
	r.data = r.data[n:]
}

func (r *sourceReader) string() string {
	if r.err != nil {
		return ""
	}
	end := bytes.IndexByte(r.data, 0)
	if end < 0 {
		r.err = errSourceShort
		return ""
	}
	s := string(r.data[:end])
	r.data = r.data[end+1:]
	return s
}

func parseSourceInfo(data []byte) (SourceResponse, error) {
	r := &sourceReader{data: data}
	r.byte() // protocol
	res := SourceResponse{Name: r.string(), Map: r.string()}
	r.string() // folder
	res.Game = r.string()
	r.skip(2) // steam app id
	res.NumPlayers = int(r.byte())
	res.MaxPlayers = int(r.byte())
	res.Bots = int(r.byte())
	return res, r.err
}

func parseSourcePlayers(data []byte) []string {
	r := &sourceReader{data: data}
	count := int(r.byte())
	players := make([]string, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		r.byte() // index
		name := r.string()
		r.skip(8) // score (int32) and duration (float32)
		if r.err == nil && name != "" {
			players = append(players, name)
		}
	}
	return players
}
