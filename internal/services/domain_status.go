package services

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
)

// domainStatusFile is written by the server's aether-domain service (tools/install/install.sh
// --apply-panel-url) after it sets up Nginx, the certificate and the firewall for the Base URL
const domainStatusFile = "domain-status.json"

// DomainStatus is what the server did with the Base URL the last time it changed. State is
// "working", "ok", "error", or "none" when the service has never run (an install made
// without the installer, or one older than 4.1.3).
type DomainStatus struct {
	State  string `json:"state"`
	URL    string `json:"url,omitempty"`
	Domain string `json:"domain,omitempty"`
	HTTPS  bool   `json:"https"`
	// Proxied is a domain that does not point at this server, as with Cloudflare's proxy: the
	// website reaches it, game ports and SFTP do not, so those use ServerIP instead
	Proxied   bool   `json:"proxied"`
	ServerIP  string `json:"serverIp,omitempty"`
	Message   string `json:"message,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
} // @name DomainStatus

// ReadDomainStatus reads the domain service's last result, kept next to the panel's config.json
func ReadDomainStatus() (DomainStatus, error) {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(config.FileUsed()), domainStatusFile))
	if errors.Is(err, os.ErrNotExist) {
		return DomainStatus{State: "none"}, nil
	}
	if err != nil {
		return DomainStatus{}, err
	}
	var status DomainStatus
	err = json.Unmarshal(data, &status)
	return status, err
}

// proxiedDomainServerIP is the server's IP when the Base URL's domain sits behind a proxy that
// only passes the website, empty otherwise
func proxiedDomainServerIP() string {
	status, err := ReadDomainStatus()
	if err != nil || !status.Proxied || net.ParseIP(status.ServerIP) == nil {
		return ""
	}
	return status.ServerIP
}
