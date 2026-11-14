package models

import (
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
)

// DNSName cleans a domain typed in the settings: lowercase, no scheme, no dots, slashes or
// spaces around it
func DNSName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimPrefix(strings.TrimPrefix(name, "https://"), "http://")
	return strings.Trim(name, ". /")
}

// WantsSRVRecord is the Minecraft rule: a server whose type is "minecraft" or "minecraft-java"
// (the same test the server settings page uses) and that has a main port gets an SRV record,
// so Java clients find the port by themselves. Bedrock does not read SRV records.
func (s *Server) WantsSRVRecord() bool {
	kind := strings.ToLower(strings.TrimSpace(s.Type))
	return (kind == "minecraft" || kind == "minecraft-java") && s.Port > 0
}

// SubdomainDomain is the base domain the server's subdomain records live under: the one they
// were made with, or the current setting for a subdomain made before the panel kept it
func (s *Server) SubdomainDomain() string {
	if s.DNSDomain != "" {
		return s.DNSDomain
	}
	return DNSName(config.DNSBaseDomain.Value())
}

// SubdomainAddress is the name players use instead of the IP (survival.play.example.com), or ""
// when the server has no subdomain records
func (s *Server) SubdomainAddress() string {
	domain := s.SubdomainDomain()
	if s.Subdomain == "" || len(s.DNSRecordIDs) == 0 || domain == "" {
		return ""
	}
	return s.Subdomain + "." + domain
}
