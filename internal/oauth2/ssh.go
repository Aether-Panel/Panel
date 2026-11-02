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

// sftpKeyCheckTimeout bounds one key check: the SSH client waits on it, and the default HTTP
// client has no timeout of its own
const sftpKeyCheckTimeout = 15 * time.Second

// errSFTPRefused is the one answer every refused login gets
var errSFTPRefused = errors.New("incorrect username or password")

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
	target, err := sftpKeyURL(config.AuthURL.Value())
	if err != nil {
		logging.Error.Printf("Cannot check SFTP keys with the panel: %s", err)
		return nil, errors.New("invalid response from authorization server")
	}

	body, err := json.Marshal(map[string]string{
		"username":  username,
		"publicKey": string(ssh.MarshalAuthorizedKey(key)),
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), sftpKeyCheckTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// the node signs in to the panel with its secret, as for the password check
	request.Header.Set("Authorization", "Bearer "+config.ClientSecret.Value())
	request.Header.Set("Content-Type", binding.MIMEJSON)

	response, err := skypanel.HTTP().Do(request)
	defer utils.CloseResponse(response)
	if err != nil {
		logging.Error.Printf("error talking to auth server: %s", err)
		return nil, errors.New("invalid response from authorization server")
	}

	switch response.StatusCode {
	case http.StatusOK:
		return sftpPermissionsFromAnswer(response.Body)
	case http.StatusBadRequest:
		// a key that is not the user's: clients offer every key they have in turn, so this
		// is routine and not worth an error in the log
		logging.Debug.Printf("The panel refused an SFTP key for %s", username)
		return nil, errSFTPRefused
	default:
		msg, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		logging.Error.Printf("Error checking an SFTP key with the auth server: [%d] [%s]", response.StatusCode, msg)
		return nil, errors.New("invalid response from authorization server")
	}
}

// sftpKeyURL is where the daemon asks the panel to check an SSH key: the panel daemon.auth.url
// points at, with the token route swapped for the key route and any path the panel is served
// under kept.
func sftpKeyURL(configured string) (string, error) {
	u, err := url.Parse(tokenURL(configured))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("daemon.auth.url %q is not an absolute http(s) URL", configured)
	}
	prefix := ""
	if p := strings.TrimRight(u.Path, "/"); strings.HasSuffix(p, oauth2EndpointPath) {
		prefix = strings.TrimSuffix(p, oauth2EndpointPath)
	}
	u.Path = prefix + sftpKeyCheckPath
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

	return sftpPermissionsFromAnswer(response.Body)
}

// sftpPermissionsFromAnswer reads the panel's answer to an SFTP login check (password or
// key): the scope it granted, "serverID:server.sftp", names the server the connection opens
func sftpPermissionsFromAnswer(body io.Reader) (*ssh.Permissions, error) {
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
			return sshPerms, nil
		}
	}
	return nil, errSFTPRefused
}
