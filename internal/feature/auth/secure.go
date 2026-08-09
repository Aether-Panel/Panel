package auth

import (
	"net"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/shared/config"
	"github.com/gin-gonic/gin"
)

// isRequestSecure reports whether the request reached the panel over a secure
// connection. It returns true when the connection is directly TLS-encrypted or
// when the request comes from a trusted reverse proxy that declares HTTPS via
// the X-Forwarded-Proto header. Local development over plain HTTP (e.g.
// http://localhost) returns false, which keeps the auth cookie Secure flag off.
func isRequestSecure(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}

	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil || !isTrustedProxy(ip) {
		return false
	}

	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

// isTrustedProxy reports whether ip matches one of the configured trusted
// reverse proxy addresses or CIDRs (security.trustedProxies).
func isTrustedProxy(ip net.IP) bool {
	for _, entry := range config.SecurityTrustedProxies.Value() {
		if _, cidr, err := net.ParseCIDR(entry); err == nil {
			if cidr.Contains(ip) {
				return true
			}
		} else if trusted := net.ParseIP(entry); trusted != nil {
			if trusted.Equal(ip) {
				return true
			}
		}
	}
	return false
}
