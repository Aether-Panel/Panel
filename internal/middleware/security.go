package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets the headers every answer of the panel carries: no other site may frame
// it (clickjacking), browsers do not guess content types, full URLs (map links carry a token)
// do not leak to other sites, and once reached over HTTPS the browser keeps to HTTPS.
// Handlers that serve untrusted pages (extensions, world maps) replace the policy with their
// own sandbox.
func SecurityHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Security-Policy", "frame-ancestors 'self'; object-src 'none'; base-uri 'self'")
	h.Set("X-Frame-Options", "SAMEORIGIN")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		h.Set("Strict-Transport-Security", "max-age=31536000")
	}
	c.Next()
}

// SameOriginGuard refuses a request the browser sent with the panel's cookies but that another
// site started: a form posted from a game server's web page on the same IP, or a websocket
// opened from it to read a console. SameSite=Lax does not stop those, since a site is the host
// without the port (and for some browsers without the scheme), so a tenant's server on
// http://IP:25566 is the same site as the panel on http://IP:8080. Requests without cookies
// (bearer tokens, the daemon, OAuth2 clients) are not affected.
func SameOriginGuard(c *gin.Context) {
	if !carriesPanelCookie(c.Request) || !changesState(c.Request) || sameOrigin(c.Request) {
		c.Next()
		return
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-origin request refused"})
}

func carriesPanelCookie(r *http.Request) bool {
	for _, name := range []string{"skypanel_auth", "session"} {
		if cookie, err := r.Cookie(name); err == nil && cookie.Value != "" {
			return true
		}
	}
	return false
}

// changesState is true for anything but a plain read; a websocket handshake is a GET, but it
// opens a channel that reads consoles and sends commands.
func changesState(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
	}
	return true
}

// sameOrigin trusts Sec-Fetch-Site, which every current browser sends and pages cannot forge,
// then falls back to Origin. A request with neither did not come from a browser page.
func sameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) || strings.EqualFold(u.Host, r.Header.Get("X-Forwarded-Host")) {
		return true
	}
	master, err := url.Parse(config.MasterURL.Value())
	return err == nil && master.Host != "" && strings.EqualFold(u.Host, master.Host)
}

// signedPathToken is the token in a world map link, /api/map-view/<server>/<token>/...
var signedPathToken = regexp.MustCompile(`(/api/map-view/[^/]+/)[^/?]+`)

// secretQuery are query values that sign someone in or open something
var secretQuery = regexp.MustCompile(`(?i)([?&](?:token|code|access_token|refresh_token)=)[^&]*`)

// RedactedLogFormatter is gin's access log line with the tokens some links carry left out:
// a map link opens a server's map for hours and the log is read by more people than the panel
func RedactedLogFormatter(param gin.LogFormatterParams) string {
	path := secretQuery.ReplaceAllString(signedPathToken.ReplaceAllString(param.Path, "${1}***"), "${1}***")
	if param.Latency > time.Minute {
		param.Latency = param.Latency.Truncate(time.Second)
	}
	return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		param.StatusCode,
		param.Latency,
		param.ClientIP,
		param.Method,
		path,
		param.ErrorMessage,
	)
}
