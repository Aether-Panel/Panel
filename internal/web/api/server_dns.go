package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// Automatic subdomains: instead of 203.0.113.5:25565 a server is reached at
// survival.play.example.com. The records live in the admin's Cloudflare zone; they are
// DNS-only (never proxied), because game traffic is not HTTP. Minecraft Java servers also get
// an SRV record, so players do not even type the port.

const (
	subdomainMinLength = 3
	subdomainMaxLength = 32

	// subdomainChangeInterval is how often one server may change its subdomain, which keeps a
	// busy owner from spending the zone's Cloudflare quota
	subdomainChangeInterval = 10 * time.Second

	// subdomainRequestTimeout bounds the Cloudflare calls of one change
	subdomainRequestTimeout = 45 * time.Second

	subdomainReasonInvalid  = "invalid"
	subdomainReasonReserved = "reserved"
	subdomainReasonTaken    = "taken"
	subdomainReasonCurrent  = "current"
)

// subdomainPattern is one DNS label: lowercase letters, digits and inner hyphens
var subdomainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// hostnamePattern accepts a dotted DNS name to point a CNAME at
var hostnamePattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// reservedSubdomains are names an admin is likely to use for something else in the same zone
var reservedSubdomains = map[string]bool{
	"www": true, "mail": true, "panel": true, "api": true, "admin": true, "ftp": true,
	"ns1": true, "ns2": true, "ns3": true, "ns4": true, "smtp": true, "imap": true, "pop": true,
	"pop3": true, "webmail": true, "mx": true, "dns": true, "cdn": true, "status": true,
	"dashboard": true, "billing": true, "support": true, "help": true, "docs": true,
	"autodiscover": true, "autoconfig": true, "localhost": true, "root": true, "node": true,
	"sftp": true, "ssh": true, "vpn": true, "cpanel": true, "whm": true, "app": true,
	"auth": true, "login": true, "static": true, "assets": true, "dev": true, "test": true,
}

// accentFold turns accented letters of a server name into their plain form for the slug
var accentFold = map[rune]string{
	'á': "a", 'à': "a", 'ä': "a", 'â': "a", 'ã': "a", 'å': "a", 'ā': "a",
	'é': "e", 'è': "e", 'ë': "e", 'ê': "e", 'ē': "e",
	'í': "i", 'ì': "i", 'ï': "i", 'î': "i", 'ī': "i",
	'ó': "o", 'ò': "o", 'ö': "o", 'ô': "o", 'õ': "o", 'ø': "o", 'ō': "o",
	'ú': "u", 'ù': "u", 'ü': "u", 'û': "u", 'ū': "u",
	'ñ': "n", 'ç': "c", 'ý': "y", 'ÿ': "y", 'ß': "ss", 'æ': "ae", 'œ': "oe",
}

var (
	errSubdomainInvalid  = skypanel.CreateError("el subdominio solo puede tener letras minúsculas, números y guiones (3 a 32 caracteres, sin guion al principio ni al final)", "ErrSubdomainInvalid")
	errSubdomainReserved = skypanel.CreateError("ese subdominio está reservado", "ErrSubdomainReserved")
	errSubdomainTaken    = skypanel.CreateError("ese subdominio ya está en uso", "ErrSubdomainTaken")
	errSubdomainDisabled = skypanel.CreateError("los subdominios automáticos no están configurados en este panel", "ErrSubdomainDisabled")
	errSubdomainWait     = skypanel.CreateError("espera unos segundos antes de volver a cambiar el subdominio", "ErrSubdomainRateLimited")
	errSubdomainNoTarget = skypanel.CreateError("el nodo de este servidor no tiene una dirección pública; configura un destino DNS en los ajustes del panel", "ErrSubdomainNoTarget")
)

// dnsProviderError marks a failure reported by Cloudflare, which the admin or owner can act
// on, as opposed to a database failure
type dnsProviderError struct{ err error }

func (e *dnsProviderError) Error() string { return e.err.Error() }
func (e *dnsProviderError) Unwrap() error { return e.err }

func providerError(err error) error {
	if err == nil {
		return nil
	}
	return &dnsProviderError{err: err}
}

// dnsSettings is a snapshot of the admin's Cloudflare configuration
type dnsSettings struct {
	Token      string
	ZoneID     string
	Domain     string
	Target     string
	AutoAssign bool

	// APIBase replaces Cloudflare's API address; only tests set it
	APIBase string
}

func (s dnsSettings) configured() bool {
	return s.Token != "" && s.ZoneID != "" && s.Domain != ""
}

func (s dnsSettings) client() *services.Cloudflare {
	cf := services.NewCloudflare(s.Token, s.ZoneID)
	if s.APIBase != "" {
		cf.BaseURL = s.APIBase
	}
	return cf
}

// currentDNSSettings reads the configuration; tests replace it
var currentDNSSettings = func() dnsSettings {
	return dnsSettings{
		Token:      strings.TrimSpace(config.CloudflareAPIToken.Value()),
		ZoneID:     strings.TrimSpace(config.CloudflareZoneID.Value()),
		Domain:     normalizeDNSName(config.DNSBaseDomain.Value()),
		Target:     normalizeDNSName(config.DNSTarget.Value()),
		AutoAssign: config.DNSAutoAssign.Value(),
	}
}

func normalizeDNSName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimPrefix(strings.TrimPrefix(name, "https://"), "http://")
	return strings.Trim(name, ". /")
}

// registerServerDNS adds the subdomain routes
func registerServerDNS(g *gin.RouterGroup) {
	// owners hold flags.edit; data.edit lets a subuser who manages the server settings in too
	perm := middleware.RequiresAnyPermission(scopes.ScopeServerEditFlags, scopes.ScopeServerEditData)

	g.GET("/:serverId/subdomain", perm, middleware.ResolveServerPanel, getServerSubdomain)
	g.PUT("/:serverId/subdomain", perm, middleware.ResolveServerPanel, putServerSubdomain)
	g.DELETE("/:serverId/subdomain", perm, middleware.ResolveServerPanel, deleteServerSubdomain)
	g.OPTIONS("/:serverId/subdomain", response.CreateOptions("GET", "PUT", "DELETE"))

	g.GET("/:serverId/subdomain/check", perm, middleware.ResolveServerPanel, checkServerSubdomain)
	g.OPTIONS("/:serverId/subdomain/check", response.CreateOptions("GET"))
}

type subdomainRecordView struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type subdomainView struct {
	Enabled   bool                  `json:"enabled"`
	Domain    string                `json:"domain"`
	Subdomain string                `json:"subdomain"`
	Address   string                `json:"address"`
	SRV       bool                  `json:"srv"`
	Port      uint16                `json:"port"`
	Records   []subdomainRecordView `json:"records"`
}

func viewSubdomain(server *models.Server, s dnsSettings) subdomainView {
	view := subdomainView{
		Enabled:   s.configured(),
		Domain:    s.Domain,
		Subdomain: server.Subdomain,
		SRV:       wantsSRVRecord(server),
		Port:      server.Port,
		Records:   []subdomainRecordView{},
	}
	if server.Subdomain == "" || s.Domain == "" {
		return view
	}
	view.Address = server.Subdomain + "." + s.Domain
	if kind, content, err := resolveDNSTarget(server, s); err == nil {
		for _, r := range planSubdomainRecords(server, view.Address, kind, content) {
			content := r.Content
			if r.Data != nil {
				content = strconv.Itoa(r.Data.Priority) + " " + strconv.Itoa(r.Data.Weight) + " " + strconv.Itoa(r.Data.Port) + " " + r.Data.Target
			}
			view.Records = append(view.Records, subdomainRecordView{Type: r.Type, Name: r.Name, Content: content})
		}
	}
	return view
}

// @Summary Get a server's subdomain
// @Description Returns whether automatic subdomains are configured and the server's own subdomain
// @Success 200 {object} subdomainView
// @Param id path string true "Server ID"
// @Tags Servers
// @Router /api/servers/{id}/subdomain [get]
// @Security OAuth2Application[server.flags.edit]
func getServerSubdomain(c *gin.Context) {
	c.JSON(http.StatusOK, viewSubdomain(getServerFromGin(c), currentDNSSettings()))
}

// @Summary Check a subdomain
// @Description Tells whether a name can be used as this server's subdomain, and why not
// @Success 200 {object} nil
// @Param id path string true "Server ID"
// @Param name query string true "Wanted subdomain"
// @Tags Servers
// @Router /api/servers/{id}/subdomain/check [get]
// @Security OAuth2Application[server.flags.edit]
func checkServerSubdomain(c *gin.Context) {
	server := getServerFromGin(c)
	name := strings.ToLower(strings.TrimSpace(c.Query("name")))

	if reason := validateSubdomain(name); reason != "" {
		c.JSON(http.StatusOK, gin.H{"available": false, "reason": reason})
		return
	}
	if name == server.Subdomain {
		c.JSON(http.StatusOK, gin.H{"available": true, "reason": subdomainReasonCurrent})
		return
	}
	taken, err := subdomainTaken(middleware.GetDatabase(c), name, server.Identifier)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if taken {
		c.JSON(http.StatusOK, gin.H{"available": false, "reason": subdomainReasonTaken})
		return
	}
	c.JSON(http.StatusOK, gin.H{"available": true, "reason": ""})
}

// @Summary Set a server's subdomain
// @Description Creates the DNS records for the subdomain, and removes the old ones when it changes
// @Success 200 {object} subdomainView
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Param id path string true "Server ID"
// @Tags Servers
// @Router /api/servers/{id}/subdomain [put]
// @Security OAuth2Application[server.flags.edit]
func putServerSubdomain(c *gin.Context) {
	server := getServerFromGin(c)
	s := currentDNSSettings()

	var body struct {
		Subdomain string `json:"subdomain"`
	}
	if err := c.ShouldBindJSON(&body); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if !s.configured() {
		response.HandleError(c, errSubdomainDisabled, http.StatusBadRequest)
		return
	}

	name := strings.ToLower(strings.TrimSpace(body.Subdomain))
	if err := subdomainReasonError(validateSubdomain(name)); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if name == server.Subdomain && len(server.DNSRecordIDs) > 0 {
		c.JSON(http.StatusOK, viewSubdomain(server, s))
		return
	}
	if !subdomainChanges.allow(server.Identifier, time.Now()) {
		response.HandleError(c, errSubdomainWait, http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), subdomainRequestTimeout)
	defer cancel()
	updated, err := assignSubdomain(ctx, middleware.GetDatabase(c), server, name, s)
	if err != nil {
		response.HandleError(c, err, subdomainErrorStatus(err))
		return
	}
	c.JSON(http.StatusOK, viewSubdomain(updated, s))
}

// @Summary Remove a server's subdomain
// @Description Deletes the server's DNS records
// @Success 204 {object} nil
// @Failure 422 {object} ErrorResponse
// @Param id path string true "Server ID"
// @Tags Servers
// @Router /api/servers/{id}/subdomain [delete]
// @Security OAuth2Application[server.flags.edit]
func deleteServerSubdomain(c *gin.Context) {
	server := getServerFromGin(c)
	s := currentDNSSettings()
	db := middleware.GetDatabase(c)

	if server.Subdomain == "" && len(server.DNSRecordIDs) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	if len(server.DNSRecordIDs) > 0 {
		if s.Token == "" || s.ZoneID == "" {
			response.HandleError(c, errSubdomainDisabled, http.StatusBadRequest)
			return
		}
		if !subdomainChanges.allow(server.Identifier, time.Now()) {
			response.HandleError(c, errSubdomainWait, http.StatusTooManyRequests)
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), subdomainRequestTimeout)
	defer cancel()

	subdomainLock.Lock()
	defer subdomainLock.Unlock()

	remaining, err := deleteDNSRecords(ctx, s.client(), server.DNSRecordIDs)
	subdomain := ""
	if len(remaining) > 0 {
		// keep the name while some of its records are still there, so removing can be retried
		subdomain = server.Subdomain
	}
	if saveErr := saveSubdomain(db, server.Identifier, subdomain, remaining); response.HandleError(c, saveErr, http.StatusInternalServerError) {
		return
	}
	if err != nil {
		response.HandleError(c, providerError(err), http.StatusUnprocessableEntity)
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Cloudflare test
// @Description Checks the Cloudflare token, zone and base domain by creating and removing a TXT record. The values sent override the saved ones, so they can be tried before saving.
// @Success 200 {object} nil
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Tags Panel Settings
// @Router /api/settings/test/cloudflare [post]
// @Security OAuth2Application[settings.edit]
func sendTestCloudflare(c *gin.Context) {
	s := currentDNSSettings()
	var overrides map[string]interface{}
	_ = c.ShouldBindJSON(&overrides)
	if v, ok := overrides[config.CloudflareAPIToken.Key()]; ok {
		s.Token = strings.TrimSpace(cast.ToString(v))
	}
	if v, ok := overrides[config.CloudflareZoneID.Key()]; ok {
		s.ZoneID = strings.TrimSpace(cast.ToString(v))
	}
	if v, ok := overrides[config.DNSBaseDomain.Key()]; ok {
		s.Domain = normalizeDNSName(cast.ToString(v))
	}

	if s.Token == "" || s.ZoneID == "" {
		response.HandleError(c, services.ErrCloudflareNotConfigured, http.StatusBadRequest)
		return
	}
	if s.Domain != "" && !hostnamePattern.MatchString(s.Domain) {
		response.HandleError(c, errors.New("el dominio base no es un nombre de dominio válido"), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), subdomainRequestTimeout)
	defer cancel()
	zone, err := s.client().Verify(ctx, s.Domain)
	if err != nil {
		// 422 rather than 502: the panel hides the text of 5xx errors, and the admin needs to
		// read what Cloudflare answered (bad token, wrong zone...)
		response.HandleError(c, err, http.StatusUnprocessableEntity)
		return
	}
	c.JSON(http.StatusOK, gin.H{"zone": zone})
}

func subdomainErrorStatus(err error) int {
	var provider *dnsProviderError
	switch {
	case errors.Is(err, errSubdomainTaken):
		return http.StatusConflict
	case errors.Is(err, errSubdomainInvalid), errors.Is(err, errSubdomainReserved):
		return http.StatusBadRequest
	case errors.Is(err, errSubdomainNoTarget), errors.As(err, &provider):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// validateSubdomain returns why name cannot be a subdomain, or "" when it can
func validateSubdomain(name string) string {
	if len(name) < subdomainMinLength || len(name) > subdomainMaxLength || !subdomainPattern.MatchString(name) {
		return subdomainReasonInvalid
	}
	if reservedSubdomains[name] {
		return subdomainReasonReserved
	}
	return ""
}

func subdomainReasonError(reason string) error {
	switch reason {
	case "":
		return nil
	case subdomainReasonReserved:
		return errSubdomainReserved
	case subdomainReasonTaken:
		return errSubdomainTaken
	default:
		return errSubdomainInvalid
	}
}

// subdomainTaken reports whether another server already uses name
func subdomainTaken(db *gorm.DB, name, serverID string) (bool, error) {
	var count int64
	err := db.Model(&models.Server{}).Where("subdomain = ? AND identifier <> ?", name, serverID).Count(&count).Error
	return count > 0, err
}

// slugifySubdomain turns a server name into a subdomain: lowercase, accents stripped, anything
// else that is not a letter or digit becomes a hyphen, at most 32 characters
func slugifySubdomain(name string) string {
	var b strings.Builder
	hyphen := true // no leading hyphen
	for _, r := range strings.ToLower(name) {
		if plain, ok := accentFold[r]; ok {
			b.WriteString(plain)
			hyphen = false
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			hyphen = false
			continue
		}
		if !hyphen {
			b.WriteByte('-')
			hyphen = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > subdomainMaxLength {
		slug = strings.TrimRight(slug[:subdomainMaxLength], "-")
	}
	return slug
}

// subdomainCandidate is the n-th name tried for base: base, base-2, base-3…, cut so the
// suffix always fits
func subdomainCandidate(base string, n int) string {
	if n <= 1 {
		return base
	}
	suffix := "-" + strconv.Itoa(n)
	if len(base)+len(suffix) > subdomainMaxLength {
		base = strings.TrimRight(base[:subdomainMaxLength-len(suffix)], "-")
	}
	return base + suffix
}

// subdomainBase is the slug a new server starts from: its name, or its ID when the name
// leaves too little (a name like "A" or "★")
func subdomainBase(server *models.Server) string {
	slug := slugifySubdomain(server.Name)
	if len(slug) < subdomainMinLength {
		slug = slugifySubdomain("srv-" + server.Identifier)
	}
	return slug
}

// wantsSRVRecord is the Minecraft rule: a server whose type is "minecraft" or "minecraft-java"
// (the same test the server settings page uses) and that has a main port gets an SRV record,
// so Java clients find the port by themselves. Bedrock does not read SRV records.
func wantsSRVRecord(server *models.Server) bool {
	kind := strings.ToLower(strings.TrimSpace(server.Type))
	return (kind == "minecraft" || kind == "minecraft-java") && server.Port > 0
}

// publicIP parses host as an address players on the internet can reach
func publicIP(host string) net.IP {
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
		return nil
	}
	return ip
}

func addressRecord(ip net.IP) (string, string) {
	if ip.To4() != nil {
		return "A", ip.String()
	}
	return "AAAA", ip.String()
}

func usableHostname(host string) bool {
	host = normalizeDNSName(host)
	return host != "localhost" && net.ParseIP(host) == nil && hostnamePattern.MatchString(host) &&
		!strings.HasSuffix(host, ".local") && !strings.HasSuffix(host, ".internal") && !strings.HasSuffix(host, ".localhost")
}

// resolveDNSTarget decides where the server's name points: the node's public IP when it has
// one; otherwise the admin's DNS target; otherwise the local node's configured IP; otherwise
// the node's hostname (as a CNAME). It returns the record type and its content.
func resolveDNSTarget(server *models.Server, s dnsSettings) (string, string, error) {
	nodeHost := strings.TrimSpace(server.Node.PublicHost)
	if ip := publicIP(nodeHost); ip != nil {
		kind, content := addressRecord(ip)
		return kind, content, nil
	}

	if target := normalizeDNSName(s.Target); target != "" {
		if ip := net.ParseIP(target); ip != nil {
			if ip.IsUnspecified() || ip.IsLoopback() {
				return "", "", errSubdomainNoTarget
			}
			kind, content := addressRecord(ip)
			return kind, content, nil
		}
		if hostnamePattern.MatchString(target) {
			return "CNAME", target, nil
		}
		return "", "", errSubdomainNoTarget
	}

	if server.Node.IsLocal() {
		if ip := publicIP(config.NodeIP.Value()); ip != nil {
			kind, content := addressRecord(ip)
			return kind, content, nil
		}
	}

	if usableHostname(nodeHost) {
		return "CNAME", normalizeDNSName(nodeHost), nil
	}
	return "", "", errSubdomainNoTarget
}

// planSubdomainRecords lists the records a subdomain needs: the name itself, plus the SRV
// record of a Minecraft Java server
func planSubdomainRecords(server *models.Server, fqdn, kind, content string) []services.CloudflareRecord {
	proxied := false
	comment := "Aether Panel: server " + server.Identifier
	records := []services.CloudflareRecord{{
		Type:    kind,
		Name:    fqdn,
		Content: content,
		TTL:     1, // automatic
		Proxied: &proxied,
		Comment: comment,
	}}
	if wantsSRVRecord(server) {
		records = append(records, services.CloudflareRecord{
			Type:    "SRV",
			Name:    "_minecraft._tcp." + fqdn,
			TTL:     1,
			Comment: comment,
			Data:    &services.CloudflareSRV{Priority: 0, Weight: 5, Port: int(server.Port), Target: fqdn},
		})
	}
	return records
}

// subdomainLock serialises subdomain changes, so two servers cannot both claim a free name
// between the check and the save
var subdomainLock sync.Mutex

// assignSubdomain gives server the subdomain name: it creates the new records, saves them and
// then removes the old ones. When anything fails before the save, the new records are removed
// again and the server keeps its old subdomain.
func assignSubdomain(ctx context.Context, db *gorm.DB, server *models.Server, name string, s dnsSettings) (*models.Server, error) {
	if !s.configured() {
		return nil, errSubdomainDisabled
	}
	if err := subdomainReasonError(validateSubdomain(name)); err != nil {
		return nil, err
	}

	subdomainLock.Lock()
	defer subdomainLock.Unlock()

	taken, err := subdomainTaken(db, name, server.Identifier)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, errSubdomainTaken
	}

	kind, content, err := resolveDNSTarget(server, s)
	if err != nil {
		return nil, err
	}

	fqdn := name + "." + s.Domain
	records := planSubdomainRecords(server, fqdn, kind, content)
	cf := s.client()

	// a record with one of these names that this server does not own means the name is used
	// in the zone outside the panel; it is never overwritten
	own := make(map[string]bool, len(server.DNSRecordIDs))
	for _, id := range server.DNSRecordIDs {
		own[id] = true
	}
	for _, record := range records {
		existing, err := cf.ListRecords(ctx, record.Name)
		if err != nil {
			return nil, providerError(err)
		}
		for _, e := range existing {
			if !own[e.ID] {
				return nil, errSubdomainTaken
			}
		}
	}

	created := make([]string, 0, len(records))
	for _, record := range records {
		rec, err := cf.CreateRecord(ctx, record)
		if err != nil {
			rollbackDNSRecords(cf, created)
			return nil, providerError(err)
		}
		created = append(created, rec.ID)
	}

	if err := saveSubdomain(db, server.Identifier, name, created); err != nil {
		rollbackDNSRecords(cf, created)
		return nil, err
	}

	var stale []string
	for _, id := range server.DNSRecordIDs {
		if !containsString(created, id) {
			stale = append(stale, id)
		}
	}
	if remaining, err := deleteDNSRecords(ctx, cf, stale); err != nil {
		logging.Error.Printf("subdomain of server %s: %d old DNS records could not be removed: %v", server.Identifier, len(remaining), err)
	}

	updated := *server
	updated.Subdomain = name
	updated.DNSRecordIDs = created
	return &updated, nil
}

func containsString(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// saveSubdomain writes only the two subdomain columns, without the model hooks, so it never
// overwrites what another request changed on the server meanwhile
func saveSubdomain(db *gorm.DB, serverID, name string, recordIDs []string) error {
	return db.Model(&models.Server{Identifier: serverID}).
		Where("identifier = ?", serverID).
		Select("subdomain", "dns_record_ids").
		UpdateColumns(&models.Server{Subdomain: name, DNSRecordIDs: recordIDs}).Error
}

// deleteDNSRecords removes records and returns the ones that could not be removed
func deleteDNSRecords(ctx context.Context, cf *services.Cloudflare, ids []string) ([]string, error) {
	var remaining []string
	var firstErr error
	for _, id := range ids {
		if err := cf.DeleteRecord(ctx, id); err != nil {
			remaining = append(remaining, id)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return remaining, firstErr
}

// rollbackDNSRecords removes records created by a change that did not go through; it gets its
// own deadline, because the change may have failed by running out of time
func rollbackDNSRecords(cf *services.Cloudflare, ids []string) {
	if len(ids) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if remaining, err := deleteDNSRecords(ctx, cf, ids); err != nil {
		logging.Error.Printf("%d DNS records of a failed subdomain change could not be removed: %v", len(remaining), err)
	}
}

// changeLimiter lets each key through once per interval
type changeLimiter struct {
	mu       sync.Mutex
	last     map[string]time.Time
	interval time.Duration
}

func (l *changeLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.last == nil {
		l.last = make(map[string]time.Time)
	}
	if last, ok := l.last[key]; ok && now.Sub(last) < l.interval {
		return false
	}
	if len(l.last) > 1000 {
		for k, t := range l.last {
			if now.Sub(t) >= l.interval {
				delete(l.last, k)
			}
		}
	}
	l.last[key] = now
	return true
}

var subdomainChanges = &changeLimiter{interval: subdomainChangeInterval}

// autoSubdomainDelays are the waits before each look for the new server: the create request
// commits its transaction only after the handler returns
var autoSubdomainDelays = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}

// autoSubdomainAttempts bounds the -2, -3… names tried for a new server
const autoSubdomainAttempts = 20

// scheduleAutoSubdomain gives a new server a subdomain when the admin turned that on
func scheduleAutoSubdomain(serverID string) {
	s := currentDNSSettings()
	if !s.configured() || !s.AutoAssign {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logging.Error.Printf("automatic subdomain for server %s failed: %v", serverID, r)
			}
		}()
		db, err := database.GetConnection()
		if err != nil {
			logging.Error.Printf("automatic subdomain for server %s: %v", serverID, err)
			return
		}
		for _, delay := range autoSubdomainDelays {
			time.Sleep(delay)
			ctx, cancel := context.WithTimeout(context.Background(), subdomainRequestTimeout)
			name, err := autoAssignSubdomain(ctx, db, serverID, s)
			cancel()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				logging.Error.Printf("automatic subdomain for server %s: %v", serverID, err)
			} else {
				logging.Info.Printf("server %s got the subdomain %s.%s", serverID, name, s.Domain)
			}
			return
		}
		logging.Error.Printf("automatic subdomain for server %s: the server was not found", serverID)
	}()
}

// autoAssignSubdomain gives the server a subdomain derived from its name, trying name,
// name-2, name-3… until one is free
func autoAssignSubdomain(ctx context.Context, db *gorm.DB, serverID string, s dnsSettings) (string, error) {
	server, err := (&services.Server{DB: db}).Get(serverID)
	if err != nil {
		return "", err
	}
	if server.Subdomain != "" {
		return server.Subdomain, nil
	}

	base := subdomainBase(server)
	for n := 1; n <= autoSubdomainAttempts; n++ {
		candidate := subdomainCandidate(base, n)
		if validateSubdomain(candidate) != "" {
			continue
		}
		taken, err := subdomainTaken(db, candidate, serverID)
		if err != nil {
			return "", err
		}
		if taken {
			continue
		}
		_, err = assignSubdomain(ctx, db, server, candidate, s)
		if errors.Is(err, errSubdomainTaken) {
			continue
		}
		return candidate, err
	}
	return "", errSubdomainTaken
}

// releaseSubdomain removes a server's DNS records when it is deleted; it is best effort and
// never stops the deletion
func releaseSubdomain(_ *gorm.DB, server *models.Server) {
	if server == nil || len(server.DNSRecordIDs) == 0 {
		return
	}
	s := currentDNSSettings()
	if s.Token == "" || s.ZoneID == "" {
		logging.Error.Printf("server %s had a subdomain, but Cloudflare is no longer configured: its DNS records stay in the zone", server.Identifier)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if remaining, err := deleteDNSRecords(ctx, s.client(), server.DNSRecordIDs); err != nil {
		logging.Error.Printf("server %s: %d DNS records could not be removed: %v", server.Identifier, len(remaining), err)
	}
}
