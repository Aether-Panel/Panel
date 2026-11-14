package utils

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

// DownloadFile downloads url with NewDownloadHTTPClient into a folder of the panel's own, then
// hands the file to save under the name the download got (from the URL or the server's
// Content-Disposition). It is never written straight into a server's folder: grab writes by
// path, and the game could have left a symlink there under that name.
func DownloadFile(url string, onPrivate func(net.IP), save func(name string, content io.Reader) error) error {
	return DownloadWith(NewDownloadHTTPClient(onPrivate), url, save)
}

// DownloadWith is DownloadFile with the given client; nil is grab's default client, which
// has no address guard (tests use it to reach their local server)
func DownloadWith(httpClient *http.Client, url string, save func(name string, content io.Reader) error) error {
	// the cache folder is on the data disk, where the server's files go anyway; the system's
	// temporary folder only when it cannot be made
	parent := config.CacheFolder.Value()
	if os.MkdirAll(parent, 0755) != nil {
		parent = ""
	}
	dir, err := os.MkdirTemp(parent, "download-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	client := grab.NewClient()
	if httpClient != nil {
		client.HTTPClient = httpClient
	}
	req, err := grab.NewRequest(dir, url)
	if err != nil {
		return err
	}
	resp := client.Do(req)
	if err = resp.Err(); err != nil {
		return err
	}
	file, err := os.Open(resp.Filename)
	if err != nil {
		return err
	}
	defer Close(file)
	return save(filepath.Base(resp.Filename), file)
}
