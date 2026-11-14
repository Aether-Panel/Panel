package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// CloudflareAPIBase is Cloudflare's REST API v4
const CloudflareAPIBase = "https://api.cloudflare.com/client/v4"

// cloudflareTimeout bounds every call, so a slow API never holds a request (or a lock) for long
const cloudflareTimeout = 15 * time.Second

// cloudflareZonesPath prefixes every zone route of the API
const cloudflareZonesPath = "/zones/"

// ErrCloudflareNotConfigured is returned when the token or the zone is missing
var ErrCloudflareNotConfigured = skypanel.CreateError("the Cloudflare API token or zone ID is missing", "ErrCloudflareNotConfigured")

// errCloudflareZoneID is returned when the zone ID does not look like one
var errCloudflareZoneID = skypanel.CreateError("the Cloudflare zone ID must be 32 hexadecimal characters", "ErrCloudflareZoneID")

// cloudflareID is the shape of zone and record IDs; anything else is refused before it reaches a URL
var cloudflareID = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)

// Cloudflare manages DNS records of one zone through the official API, with a scoped token
// (Zone → DNS → Edit). The token is sent as a Bearer header only, never in a URL, and is
// scrubbed from every error this client returns.
type Cloudflare struct {
	Token   string
	ZoneID  string
	BaseURL string
	Client  *http.Client
}

// NewCloudflare builds a client for the given zone against the real API
func NewCloudflare(token, zoneID string) *Cloudflare {
	return &Cloudflare{
		Token:   strings.TrimSpace(token),
		ZoneID:  strings.TrimSpace(zoneID),
		BaseURL: CloudflareAPIBase,
		Client:  &http.Client{Timeout: cloudflareTimeout},
	}
}

// CloudflareRecord is a DNS record as the API takes and returns it
type CloudflareRecord struct {
	ID      string         `json:"id,omitempty"`
	Type    string         `json:"type"`
	Name    string         `json:"name"`
	Content string         `json:"content,omitempty"`
	TTL     int            `json:"ttl,omitempty"`
	Proxied *bool          `json:"proxied,omitempty"`
	Comment string         `json:"comment,omitempty"`
	Data    *CloudflareSRV `json:"data,omitempty"`
}

// CloudflareSRV is the data of an SRV record
type CloudflareSRV struct {
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	Port     int    `json:"port"`
	Target   string `json:"target"`
}

// CloudflareZone is the part of a zone the panel needs
type CloudflareZone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// CloudflareError is a refusal from the API, with Cloudflare's own message
type CloudflareError struct {
	Status  int
	Code    int
	Message string
}

func (e *CloudflareError) Error() string {
	return e.panelError().Error()
}

// As lets the API answer the refusal as a coded error, which the page shows in the viewer's
// language with Cloudflare's own message inside
func (e *CloudflareError) As(target interface{}) bool {
	panelErr, ok := target.(**skypanel.Error)
	if ok {
		*panelErr = e.panelError()
	}
	return ok
}

func (e *CloudflareError) panelError() *skypanel.Error {
	if e.Code != 0 {
		return skypanel.ErrCloudflareRefusedCode(e.Message, e.Code)
	}
	return skypanel.ErrCloudflareRefused(e.Message)
}

// IsCloudflareNotFound reports whether the API said the zone or record does not exist
func IsCloudflareNotFound(err error) bool {
	var cfErr *CloudflareError
	return errors.As(err, &cfErr) && cfErr.Status == http.StatusNotFound
}

type cloudflareEnvelope struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result json.RawMessage `json:"result"`
}

// Zone returns the configured zone; it fails when the token cannot read it
func (cf *Cloudflare) Zone(ctx context.Context) (CloudflareZone, error) {
	var zone CloudflareZone
	if err := cf.check(); err != nil {
		return zone, err
	}
	err := cf.do(ctx, http.MethodGet, cloudflareZonesPath+cf.ZoneID, nil, &zone)
	return zone, err
}

// ListRecords returns the records of the zone with exactly this name, of any type
func (cf *Cloudflare) ListRecords(ctx context.Context, name string) ([]CloudflareRecord, error) {
	if err := cf.check(); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("name", name)
	query.Set("per_page", "100")
	var records []CloudflareRecord
	err := cf.do(ctx, http.MethodGet, cloudflareZonesPath+cf.ZoneID+"/dns_records?"+query.Encode(), nil, &records)
	return records, err
}

// CreateRecord adds a record to the zone and returns it with its ID
func (cf *Cloudflare) CreateRecord(ctx context.Context, record CloudflareRecord) (CloudflareRecord, error) {
	var created CloudflareRecord
	if err := cf.check(); err != nil {
		return created, err
	}
	record.ID = ""
	err := cf.do(ctx, http.MethodPost, cloudflareZonesPath+cf.ZoneID+"/dns_records", record, &created)
	return created, err
}

// UpdateRecord replaces a record of the zone
func (cf *Cloudflare) UpdateRecord(ctx context.Context, id string, record CloudflareRecord) (CloudflareRecord, error) {
	var updated CloudflareRecord
	if err := cf.check(); err != nil {
		return updated, err
	}
	if !cloudflareID.MatchString(id) {
		return updated, skypanel.ErrCloudflareRecordID(id)
	}
	record.ID = ""
	err := cf.do(ctx, http.MethodPut, cloudflareZonesPath+cf.ZoneID+"/dns_records/"+id, record, &updated)
	return updated, err
}

// DeleteRecord removes a record; one that is already gone counts as removed
func (cf *Cloudflare) DeleteRecord(ctx context.Context, id string) error {
	if err := cf.check(); err != nil {
		return err
	}
	if !cloudflareID.MatchString(id) {
		return skypanel.ErrCloudflareRecordID(id)
	}
	err := cf.do(ctx, http.MethodDelete, cloudflareZonesPath+cf.ZoneID+"/dns_records/"+id, nil, nil)
	if IsCloudflareNotFound(err) {
		return nil
	}
	return err
}

// Verify checks that the token can read the zone, that domain belongs to it and that records
// can be written there (a TXT record is created and removed again). It returns the zone name.
func (cf *Cloudflare) Verify(ctx context.Context, domain string) (string, error) {
	zone, err := cf.Zone(ctx)
	if err != nil {
		return "", err
	}
	zoneName := strings.ToLower(strings.TrimSuffix(zone.Name, "."))
	domain = strings.ToLower(strings.Trim(strings.TrimSpace(domain), "."))
	if domain != "" && domain != zoneName && !strings.HasSuffix(domain, "."+zoneName) {
		return zoneName, skypanel.ErrCloudflareDomainNotInZone(domain, zoneName)
	}
	if domain == "" {
		domain = zoneName
	}

	test, err := cf.CreateRecord(ctx, CloudflareRecord{
		Type:    "TXT",
		Name:    "_aether-panel-test." + domain,
		Content: "\"aether panel connection test\"",
		TTL:     60,
		Comment: "Aether Panel: connection test",
	})
	if err != nil {
		return zoneName, err
	}
	if err := cf.DeleteRecord(ctx, test.ID); err != nil {
		return zoneName, err
	}
	return zoneName, nil
}

func (cf *Cloudflare) check() error {
	if cf.Token == "" || cf.ZoneID == "" {
		return ErrCloudflareNotConfigured
	}
	if !cloudflareID.MatchString(cf.ZoneID) {
		return errCloudflareZoneID
	}
	return nil
}

func (cf *Cloudflare) do(ctx context.Context, method, path string, body, out interface{}) error {
	base := strings.TrimSuffix(cf.BaseURL, "/")
	if base == "" {
		base = CloudflareAPIBase
	}
	client := cf.Client
	if client == nil {
		client = &http.Client{Timeout: cloudflareTimeout}
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	ctx, cancel := context.WithTimeout(ctx, cloudflareTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return cf.scrub(err)
	}
	req.Header.Set("Authorization", "Bearer "+cf.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return skypanel.ErrCloudflareUnreachable(cf.scrubText(err.Error()))
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return skypanel.ErrCloudflareBadAnswer(cf.scrubText(err.Error()))
	}

	var envelope cloudflareEnvelope
	decodeErr := json.Unmarshal(raw, &envelope)
	if resp.StatusCode >= http.StatusBadRequest || decodeErr != nil || !envelope.Success {
		cfErr := &CloudflareError{Status: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
		if len(envelope.Errors) > 0 {
			cfErr.Code = envelope.Errors[0].Code
			cfErr.Message = envelope.Errors[0].Message
		}
		if cfErr.Message == "" {
			cfErr.Message = "unexpected answer"
		}
		cfErr.Message = cf.scrubText(cfErr.Message)
		return cfErr
	}

	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return skypanel.ErrCloudflareBadAnswer(err.Error())
		}
	}
	return nil
}

// scrub keeps the token out of an error, whatever produced it
func (cf *Cloudflare) scrub(err error) error {
	if err == nil || cf.Token == "" || !strings.Contains(err.Error(), cf.Token) {
		return err
	}
	return errors.New(cf.scrubText(err.Error()))
}

func (cf *Cloudflare) scrubText(text string) string {
	if cf.Token == "" {
		return text
	}
	return strings.ReplaceAll(text, cf.Token, "[token]")
}
