package query

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func cstring(s string) []byte { return append([]byte(s), 0) }

// fakeSteamServer answers A2S_INFO and A2S_PLAYER, asking for a challenge first like real
// servers since 2020 do
func fakeSteamServer(t *testing.T) int {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Cleanup(func() { _ = conn.Close() })
	challenge := []byte{1, 2, 3, 4}
	header := []byte{0xFF, 0xFF, 0xFF, 0xFF}

	go func() {
		buf := make([]byte, 1400)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			req := buf[:n]
			switch {
			case req[4] == 'T' && !bytes.HasSuffix(req, challenge):
				_, _ = conn.WriteTo(append(append([]byte{}, header...), append([]byte{'A'}, challenge...)...), addr)
			case req[4] == 'T':
				info := append(append([]byte{}, header...), 'I', 17)
				info = append(info, cstring("Mi servidor")...)
				info = append(info, cstring("de_dust2")...)
				info = append(info, cstring("csgo")...)
				info = append(info, cstring("Counter-Strike")...)
				info = append(info, 0xDA, 0x02, 3, 16, 1)
				_, _ = conn.WriteTo(info, addr)
			case req[4] == 'U' && !bytes.Equal(req[5:9], challenge):
				_, _ = conn.WriteTo(append(append([]byte{}, header...), append([]byte{'A'}, challenge...)...), addr)
			case req[4] == 'U':
				players := append(append([]byte{}, header...), 'D', 3)
				for i, name := range []string{"ana", "", "luis"} {
					players = append(players, byte(i))
					players = append(players, cstring(name)...)
					players = append(players, make([]byte, 8)...)
				}
				_, _ = conn.WriteTo(players, addr)
			}
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func TestSource_InfoAndPlayersWithChallenges(t *testing.T) {
	port := fakeSteamServer(t)

	res, err := Source("127.0.0.1", port)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "Mi servidor", res.Name)
	assert.Equal(t, "de_dust2", res.Map)
	assert.Equal(t, "Counter-Strike", res.Game)
	assert.Equal(t, 3, res.NumPlayers)
	assert.Equal(t, 16, res.MaxPlayers)
	assert.Equal(t, 1, res.Bots)
	// nameless entries (bots, hidden players) are left out
	assert.Equal(t, []string{"ana", "luis"}, res.Players)
}

func TestSource_SilentServerTimesOut(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		return
	}
	defer func() { _ = conn.Close() }()

	_, err = Source("127.0.0.1", conn.LocalAddr().(*net.UDPAddr).Port)
	assert.Error(t, err)
}

func TestParseSourceInfo_ShortPacket(t *testing.T) {
	_, err := parseSourceInfo([]byte{17, 'a'})
	assert.Error(t, err)
}

func TestDefaultGateway(t *testing.T) {
	table := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"eth0\t0011A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0\n" +
		"eth0\t00000000\t0100A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0\n"
	file := filepath.Join(t.TempDir(), "route")
	assert.NoError(t, os.WriteFile(file, []byte(table), 0644))

	assert.Equal(t, "192.168.0.1", defaultGateway(file))
	assert.Equal(t, "", defaultGateway(filepath.Join(t.TempDir(), "missing")))
}

func TestHostOrLocal_KeepsASpecificAddress(t *testing.T) {
	assert.Equal(t, "10.0.0.5", hostOrLocal("10.0.0.5"))
	assert.NotEqual(t, "0.0.0.0", hostOrLocal("0.0.0.0"))
}
