package utils

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/cavaliergopher/grab/v3"
)

// checkDownloadAddress decides whether a template download may connect to ip. Loopback,
// link-local (cloud metadata lives at 169.254.169.254) and unspecified addresses are never
// valid sources: reaching them from the node could expose its credentials or local
// services. Private network addresses are allowed unless security.blockPrivateDownloads is
// set, because some installs keep mirrors on their LAN; onPrivate is told about them.
func checkDownloadAddress(ip net.IP, onPrivate func(net.IP)) error {
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return fmt.Errorf("refusing to download from internal address %v", ip)
	}
	if isPrivateIP(ip) {
		if config.SecurityBlockPrivateDownloads.Value() {
			return fmt.Errorf("refusing to download from private address %v (security.blockPrivateDownloads)", ip)
		}
		if onPrivate != nil {
			onPrivate(ip)
		}
	}
	return nil
}

// NewDownloadHTTPClient returns a client for URLs that come from templates. The address is
// checked when the connection is made, so a host name that changes what it resolves to
// cannot get around it. There is no overall timeout because game files can be large.
func NewDownloadHTTPClient(onPrivate func(net.IP)) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			return checkDownloadAddress(net.ParseIP(host), onPrivate)
		},
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	return &http.Client{Transport: transport}
}

// DownloadFile saves url into directory using NewDownloadHTTPClient
func DownloadFile(directory, url string, onPrivate func(net.IP)) error {
	client := grab.NewClient()
	client.HTTPClient = NewDownloadHTTPClient(onPrivate)

	req, err := grab.NewRequest(directory, url)
	if err != nil {
		return err
	}
	return client.Do(req).Err()
}
