package oauth2

import (
	"bytes"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/gin-gonic/gin/binding"
	"net/http"
	"net/url"
	"strings"
)

// tokenPath is where the panel answers token requests (internal/web/oauth2)
const tokenPath = "/oauth2/token"

func createRequest(data url.Values) (request *http.Request) {
	authURL := tokenURL(config.AuthURL.Value())
	request, _ = http.NewRequest("POST", authURL, bytes.NewBufferString(data.Encode()))

	request.Header.Add("Authorization", "Bearer "+config.ClientSecret.Value())
	request.Header.Add("Content-Type", binding.MIMEPOSTForm)
	return
}

// tokenURL returns the URL the daemon sends its token requests to. daemon.auth.url is meant
// to be the panel's full token URL, but its default (http://localhost:8080) is only the
// panel's address, and a remote node configured the same way got the web page instead of a
// token. A URL with no path gets the token path added; full URLs, and anything that does
// not parse as an absolute URL, are used as they are.
func tokenURL(configured string) string {
	u, err := url.Parse(configured)
	if err != nil || u.Host == "" || strings.Trim(u.Path, "/") != "" {
		return configured
	}
	u.Path = tokenPath
	return u.String()
}

type TokenInfoResponse struct {
	Active bool   `json:"active"`
	Scope  string `json:"scope,omitempty"`
	ErrorResponse
} // @name OAuth2TokenInfoResponse

type TokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
	Scope       string `json:"scope"`
	ErrorResponse
} // @name OAuth2TokenResponse

type ErrorResponse struct {
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
} // @name OAuthErrorResponse
