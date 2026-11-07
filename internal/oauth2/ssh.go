package oauth2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin/binding"
	"golang.org/x/crypto/ssh"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// sftpKeyCheckPath is the panel route that checks an SSH key login for a node
// (internal/web/auth/sftp_key.go)
const sftpKeyCheckPath = "/auth/sftp-key"

// sftpAccessCheckPath is the panel route that says whether an open SFTP connection may stay
// open (internal/web/auth/sftp_check.go)
const sftpAccessCheckPath = "/auth/sftp-check"

// panelCheckTimeout bounds one key or access check: the SSH client waits on it, and the
// default HTTP client has no timeout of its own
const panelCheckTimeout = 15 * time.Second

// panelAnswerLimit bounds what is read of the panel's answer to a check
const panelAnswerLimit = 64 * 1024

// sftpExtUsername keeps the SFTP login in the connection's permissions, for Recheck
const sftpExtUsername = "username"

// errSFTPRefused is the one answer every refused login gets
var errSFTPRefused = errors.New("incorrect username or password")

// errNoAnswer is a check the panel did not answer: it says nothing about the user
var errNoAnswer = errors.New("invalid response from authorization server")

type WebSSHAuthorization struct {
}

func (ws *WebSSHAuthorization) Validate(username, password string) (*ssh.Permissions, error) {
	return validateSSH(username, password, true)
}

// ValidateKey asks the panel whether the key an SFTP client offered is one of the user's SSH
// keys and the user may use SFTP on the server, as Validate does for a password. A remote
// node does not have the users' keys, so without this only the panel's own node could take
// key logins. The SSH library calls this before the client proves it holds the private key
// and grants nothing until that proof checks out.
func (ws *WebSSHAuthorization) ValidateKey(username string, key ssh.PublicKey) (*ssh.Permissions, error) {
	status, answer, err := askPanel(sftpKeyCheckPath, map[string]string{
		"username":  username,
		"publicKey": string(ssh.MarshalAuthorizedKey(key)),
	})
	if err != nil {
		logging.Error.Printf("Cannot check an SFTP key with the panel: %s", err)
		return nil, errNoAnswer
	}

	switch status {
	case http.StatusOK:
		return sftpPermissionsFromAnswer(bytes.NewReader(answer), username)
	case http.StatusBadRequest:
		// a key that is not the user's: clients offer every key they have in turn, so this
		// is routine and not worth an error in the log
		logging.Debug.Printf("The panel refused an SFTP key for %s", username)
		return nil, errSFTPRefused
	default:
		logging.Error.Printf("Error checking an SFTP key with the auth server: [%d] [%s]", status, answer)
		return nil, errNoAnswer
	}
}

// Recheck asks the panel whether the user of an open SFTP connection may still use SFTP on
// its server. Only the panel's "no access" (400) is a no; anything else (panel unreachable,
// an older panel without the route, a node secret it does not know) is an error, so the
// connection stays open rather than ending on an outage.
func (ws *WebSSHAuthorization) Recheck(perms *ssh.Permissions) (bool, error) {
	if perms == nil || perms.Extensions[sftpExtUsername] == "" {
		return false, nil
	}
	username := perms.Extensions[sftpExtUsername]
	status, answer, err := askPanel(sftpAccessCheckPath, map[string]string{"username": username})
	if err != nil {
		return false, err
	}

	switch status {
	case http.StatusOK:
		granted, err := sftpPermissionsFromAnswer(bytes.NewReader(answer), username)
		if errors.Is(err, errSFTPRefused) {
			return false, nil
		}
		if err != nil {
			// not the panel's JSON (a proxy's page, say): no answer
			return false, fmt.Errorf("the panel's answer could not be read: %w", err)
		}
		// the answer must still be for the server this connection opened
		return granted.Extensions["server_id"] == perms.Extensions["server_id"], nil
	case http.StatusBadRequest:
		return false, nil
	default:
		return false, fmt.Errorf("the panel answered [%d] [%s]", status, answer[:min(len(answer), 200)])
	}
}

// askPanel posts a JSON body to one of the panel's routes for nodes, signed in with the
// node's secret, and returns the status and the start of the answer. err is set only when
// there is no answer at all. It logs nothing: callers decide how often that is worth it.
func askPanel(route string, body interface{}) (int, []byte, error) {
	target, err := panelRouteURL(config.AuthURL.Value(), route)
	if err != nil {
		return 0, nil, err
	}

	data, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), panelCheckTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	// the node signs in to the panel with its secret, as for the password check
	request.Header.Set("Authorization", "Bearer "+config.ClientSecret.Value())
	request.Header.Set("Content-Type", binding.MIMEJSON)

	response, err := skypanel.HTTP().Do(request)
	defer utils.CloseResponse(response)
	if err != nil {
		return 0, nil, fmt.Errorf("error talking to the panel: %w", err)
	}
	answer, err := io.ReadAll(io.LimitReader(response.Body, panelAnswerLimit))
	if err != nil {
		return 0, nil, fmt.Errorf("error reading the panel's answer: %w", err)
	}
	return response.StatusCode, answer, nil
}

// sftpKeyURL is where the daemon asks the panel to check an SSH key
func sftpKeyURL(configured string) (string, error) {
	return panelRouteURL(configured, sftpKeyCheckPath)
}

// panelRouteURL is one of the panel's routes for nodes: the panel daemon.auth.url points at,
// with the token route swapped for the given one and any path the panel is served under kept.
func panelRouteURL(configured, route string) (string, error) {
	u, err := url.Parse(tokenURL(configured))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("daemon.auth.url %q is not an absolute http(s) URL", configured)
	}
	prefix := ""
	if p := strings.TrimRight(u.Path, "/"); strings.HasSuffix(p, oauth2EndpointPath) {
		prefix = strings.TrimSuffix(p, oauth2EndpointPath)
	}
	u.Path = prefix + route
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func validateSSH(username string, password string, _ bool) (*ssh.Permissions, error) {
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", username)
	data.Set("password", password)
	data.Set("scope", "sftp")

	request := createRequest(data)

	response, err := skypanel.HTTP().Do(request)
	defer utils.CloseResponse(response)
	if err != nil {
		logging.Error.Printf("error talking to auth server: %s", err)
		return nil, errors.New("invalid response from authorization server")
	}

	// we should only get a 200, if we get any others, we have a problem
	if response.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(response.Body)

		logging.Error.Printf("Error talking to auth server: [%d] [%s]", response.StatusCode, msg)
		return nil, errors.New("invalid response from authorization server")
	}

	return sftpPermissionsFromAnswer(response.Body, username)
}

// sftpPermissionsFromAnswer reads the panel's answer to an SFTP login check (password or
// key): the scope it granted, "serverID:server.sftp", names the server the connection opens.
// The login is kept for Recheck.
func sftpPermissionsFromAnswer(body io.Reader, username string) (*ssh.Permissions, error) {
	var respArr map[string]interface{}
	err := json.NewDecoder(body).Decode(&respArr)
	if err != nil {
		return nil, err
	}
	if respArr["error"] != nil {
		return nil, errSFTPRefused
	}
	// an answer without a scope is a refusal, not a reason to panic
	granted, _ := respArr["scope"].(string)
	for _, v := range strings.Split(granted, " ") {
		t := strings.Split(v, ":")
		if len(t) != 2 {
			continue
		}
		serverID := t[0]
		scope := t[1]

		if scopes.ScopeServerSftp.Is(scope) {
			sshPerms := &ssh.Permissions{}
			sshPerms.Extensions = make(map[string]string)
			sshPerms.Extensions["server_id"] = serverID
			sshPerms.Extensions[sftpExtUsername] = username
			return sshPerms, nil
		}
	}
	return nil, errSFTPRefused
}
