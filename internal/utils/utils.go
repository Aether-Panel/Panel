package utils

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"
)

// ValidateExternalURL validates that a URL is safe for external requests (SSRF prevention).
// It ensures the URL uses http/https, has a valid host, and does not point to
// private/internal IP ranges, loopback, or localhost.
func ValidateExternalURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("invalid URL scheme: must be http or https")
	}

	host := u.Hostname()
	if host == "" {
		return errors.New("invalid URL: empty host")
	}

	// Check for localhost
	if strings.EqualFold(host, "localhost") {
		return errors.New("URL cannot point to localhost")
	}

	// Check if host is an IP address
	ip := net.ParseIP(host)
	if ip != nil {
		if isPrivateIP(ip) {
			return errors.New("URL cannot point to a private or loopback IP address")
		}
		return nil
	}

	// Resolve hostname and check all resolved IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		return errors.New("failed to resolve host: " + err.Error())
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return errors.New("URL resolves to a private or loopback IP address")
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check for loopback
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local unicast
	if ip.IsLinkLocalUnicast() {
		return true
	}

	// Check for private ranges
	if ip4 := ip.To4(); ip4 != nil {
		return isPrivateIPv4(ip4)
	}
	return ip.IsPrivate()
}

// isPrivateIPv4 reports whether ip4 is in a private, reserved or internal IPv4 range.
func isPrivateIPv4(ip4 net.IP) bool {
	// 10.0.0.0/8
	if ip4[0] == 10 {
		return true
	}
	// 172.16.0.0/12
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	// 192.168.0.0/16
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	// 169.254.0.0/16 (link-local)
	if ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	// 100.64.0.0/10 (CGNAT)
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}
	// 127.0.0.0/8
	if ip4[0] == 127 {
		return true
	}
	// 0.0.0.0/8
	if ip4[0] == 0 {
		return true
	}
	// 198.18.0.0/15 (benchmarking)
	return ip4[0] == 198 && ip4[1] >= 18 && ip4[1] <= 19
}

// NewRestrictedHTTPClient creates an *http.Client that blocks connections to
// private/internal IPs as an SSRF mitigation. The address is checked on the connection
// itself, after DNS resolution, so a host name that resolves to a public IP when checked
// and to an internal one when dialled (DNS rebinding) is refused as well.
func NewRestrictedHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || ip.IsUnspecified() || isPrivateIP(ip) {
				return errors.New("connection refused: target resolves to a private or loopback IP address")
			}
			return nil
		},
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          5,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

func GenerateRandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

func Union[T comparable](a, b []T) []T {
	result := make([]T, 0)

	if a == nil || b == nil || len(a) == 0 || len(b) == 0 {
		return result
	}

	for _, v := range a {
		for _, x := range b {
			if reflect.DeepEqual(v, x) {
				result = append(result, v)
				break
			}
		}
	}

	return result
}

func Remove[T comparable](a []T, b T) []T {
	if a == nil {
		return nil
	}

	replacement := make([]T, 0)
	for _, v := range a {
		if b == v {
			continue
		}
		replacement = append(replacement, v)
	}
	return replacement
}

func GetDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
