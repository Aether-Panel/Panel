package query

import (
	"bufio"
	"encoding/hex"
	"net"
	"os"
	"strings"
	"sync"
)

// hostOrLocal is where to reach a game server that listens on all addresses. On a plain
// install that is this machine. When the panel itself runs in a Docker container (the
// docker-compose setup), game servers are other containers on a separate network, and the
// way to them is the port they publish on the host, reached through the container's
// default gateway.
func hostOrLocal(ip string) string {
	if ip != "" && ip != "0.0.0.0" && ip != "::" {
		return ip
	}
	localHostOnce.Do(func() { localHost = detectLocalHost() })
	return localHost
}

var (
	localHostOnce sync.Once
	localHost     string
)

func detectLocalHost() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		if gw := defaultGateway("/proc/net/route"); gw != "" {
			return gw
		}
	}
	return "127.0.0.1"
}

// defaultGateway reads the IPv4 default route from a Linux route table
func defaultGateway(routeFile string) string {
	file, err := os.Open(routeFile)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// Iface Destination Gateway ...; the default route has destination 00000000
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		raw, err := hex.DecodeString(fields[2])
		if err != nil || len(raw) != 4 {
			continue
		}
		// the table is little endian
		return net.IPv4(raw[3], raw[2], raw[1], raw[0]).String()
	}
	return ""
}
