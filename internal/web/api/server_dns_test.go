package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	dnsTestToken = "cf-test-token-do-not-leak-42"
	dnsTestZone  = "0123456789abcdef0123456789abcdef"
)

// dnsFake is a Cloudflare zone in memory
type dnsFake struct {
	mu        sync.Mutex
	records   map[string]services.CloudflareRecord
	zoneOf    map[string]string // record id → zone id
	zones     map[string]string // zone id → zone name; the test zone when empty
	next      int
	creates   []services.CloudflareRecord
	deletes   []string
	failType  string // record type whose creation fails
	failText  string
	listCalls int
}

// dnsReply writes a Cloudflare API answer
func dnsReply(w http.ResponseWriter, status int, payload map[string]interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// dnsFailure is a Cloudflare error answer
func dnsFailure(w http.ResponseWriter, status, code int, message string) {
	dnsReply(w, status, map[string]interface{}{"success": false, "errors": []map[string]interface{}{{"code": code, "message": message}}})
}

func dnsSuccess(w http.ResponseWriter, result interface{}) {
	dnsReply(w, http.StatusOK, map[string]interface{}{"success": true, "result": result})
}

// zoneName is the name of a zone the fake knows, the test zone when no zones were set
func (f *dnsFake) zoneName(zone string) (string, bool) {
	if len(f.zones) == 0 && zone == dnsTestZone {
		return "example.com", true
	}
	name, ok := f.zones[zone]
	return name, ok
}

func (f *dnsFake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer "+dnsTestToken {
		dnsFailure(w, http.StatusUnauthorized, 10000, "Authentication error")
		return
	}
	zone := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/zones/"), "/", 2)[0]
	zoneName, known := f.zoneName(zone)
	if !known {
		dnsFailure(w, http.StatusNotFound, 7003, "Could not route to /zones/"+zone)
		return
	}
	prefix := "/zones/" + zone
	records := prefix + "/dns_records"
	id, onRecord := strings.CutPrefix(r.URL.Path, records+"/")
	if onRecord && f.zoneOf[id] != "" && f.zoneOf[id] != zone {
		dnsFailure(w, http.StatusNotFound, 81044, "Record does not exist.")
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == prefix:
		dnsSuccess(w, map[string]string{"id": zone, "name": zoneName})
	case r.Method == http.MethodGet && r.URL.Path == records:
		f.listCalls++
		dnsSuccess(w, f.recordsNamed(zone, r.URL.Query().Get("name")))
	case r.Method == http.MethodPost && r.URL.Path == records:
		f.createRecord(w, r, zone)
	case r.Method == http.MethodPut && onRecord:
		f.updateRecord(w, r, id)
	case r.Method == http.MethodDelete && onRecord:
		f.deleteRecord(w, id)
	default:
		dnsFailure(w, http.StatusNotFound, 7003, "no route")
	}
}

func (f *dnsFake) createRecord(w http.ResponseWriter, r *http.Request, zone string) {
	var rec services.CloudflareRecord
	_ = json.NewDecoder(r.Body).Decode(&rec)
	if f.failType != "" && rec.Type == f.failType {
		dnsFailure(w, http.StatusBadRequest, 9000, f.failText)
		return
	}
	f.next++
	rec.ID = fmt.Sprintf("%032x", f.next)
	f.records[rec.ID] = rec
	f.zoneOf[rec.ID] = zone
	f.creates = append(f.creates, rec)
	dnsSuccess(w, rec)
}

func (f *dnsFake) updateRecord(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := f.records[id]; !ok {
		dnsFailure(w, http.StatusNotFound, 81044, "Record does not exist.")
		return
	}
	var rec services.CloudflareRecord
	_ = json.NewDecoder(r.Body).Decode(&rec)
	rec.ID = id
	f.records[id] = rec
	dnsSuccess(w, rec)
}

func (f *dnsFake) deleteRecord(w http.ResponseWriter, id string) {
	f.deletes = append(f.deletes, id)
	if _, ok := f.records[id]; !ok {
		dnsFailure(w, http.StatusNotFound, 81044, "Record does not exist.")
		return
	}
	delete(f.records, id)
	dnsSuccess(w, map[string]string{"id": id})
}

// recordsNamed lists the zone's records with that name, never nil; the caller holds f.mu
func (f *dnsFake) recordsNamed(zone, name string) []services.CloudflareRecord {
	found := []services.CloudflareRecord{}
	for _, rec := range f.records {
		if rec.Name == name && f.zoneOf[rec.ID] == zone {
			found = append(found, rec)
		}
	}
	return found
}

func (f *dnsFake) addExternal(rec services.CloudflareRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	rec.ID = fmt.Sprintf("%032x", f.next)
	f.records[rec.ID] = rec
	f.zoneOf[rec.ID] = dnsTestZone
}

func (f *dnsFake) byName(name string) []services.CloudflareRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	var found []services.CloudflareRecord
	for _, rec := range f.records {
		if rec.Name == name {
			found = append(found, rec)
		}
	}
	return found
}

type dnsTestEnv struct {
	db       *gorm.DB
	fake     *dnsFake
	settings dnsSettings
	node     *models.Node
}

func setupDNSTest(t *testing.T) *dnsTestEnv {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "dns_test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}, &models.Server{}))

	fake := &dnsFake{records: map[string]services.CloudflareRecord{}, zoneOf: map[string]string{}}
	api := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(api.Close)

	node := &models.Node{Name: "eu-1", PublicHost: "203.0.113.5", PrivateHost: "10.0.0.5", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "secret"}
	require.NoError(t, db.Create(node).Error)

	env := &dnsTestEnv{
		db:   db,
		fake: fake,
		node: node,
		settings: dnsSettings{
			Token:      dnsTestToken,
			ZoneID:     dnsTestZone,
			Domain:     "play.example.com",
			AutoAssign: true,
			APIBase:    api.URL,
		},
	}

	previous := currentDNSSettings
	currentDNSSettings = func() dnsSettings { return env.settings }
	previousLimiter := subdomainChanges
	subdomainChanges = &changeLimiter{interval: subdomainChangeInterval}
	t.Cleanup(func() {
		currentDNSSettings = previous
		subdomainChanges = previousLimiter
	})
	return env
}

// addServer stores a server on the test node (or on the local node when onNode is false)
func (env *dnsTestEnv) addServer(t *testing.T, id, name, kind string, port uint16, onNode bool) *models.Server {
	server := &models.Server{Identifier: id, Name: name, Type: kind, Port: port}
	if onNode {
		server.NodeID = env.node.ID
		server.Node = *env.node
	}
	require.NoError(t, env.db.Omit(clause.Associations).Create(server).Error)
	return env.reload(t, id)
}

func (env *dnsTestEnv) reload(t *testing.T, id string) *models.Server {
	server, err := (&services.Server{DB: env.db}).Get(id)
	require.NoError(t, err)
	return server
}

func (env *dnsTestEnv) router() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", env.db) })
	r.GET("/servers/:serverId/subdomain", middleware.ResolveServerPanel, getServerSubdomain)
	r.PUT("/servers/:serverId/subdomain", middleware.ResolveServerPanel, putServerSubdomain)
	r.DELETE("/servers/:serverId/subdomain", middleware.ResolveServerPanel, deleteServerSubdomain)
	r.GET("/servers/:serverId/subdomain/check", middleware.ResolveServerPanel, checkServerSubdomain)
	r.POST("/settings/test/cloudflare", sendTestCloudflare)
	return r
}

func doJSON(r http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestValidateSubdomain(t *testing.T) {
	cases := map[string]string{
		"survival":              "",
		"mc-1":                  "",
		"abc":                   "",
		"ab":                    subdomainReasonInvalid,
		"-abc":                  subdomainReasonInvalid,
		"abc-":                  subdomainReasonInvalid,
		"Survival":              subdomainReasonInvalid,
		"su_rv":                 subdomainReasonInvalid,
		"sur.vival":             subdomainReasonInvalid,
		"ñandu":                 subdomainReasonInvalid,
		"":                      subdomainReasonInvalid,
		strings.Repeat("a", 32): "",
		strings.Repeat("a", 33): subdomainReasonInvalid,
		"www":                   subdomainReasonReserved,
		"mail":                  subdomainReasonReserved,
		"panel":                 subdomainReasonReserved,
		"api":                   subdomainReasonReserved,
		"admin":                 subdomainReasonReserved,
		"ftp":                   subdomainReasonReserved,
		"ns1":                   subdomainReasonReserved,
		"ns2":                   subdomainReasonReserved,
	}
	for name, want := range cases {
		assert.Equal(t, want, validateSubdomain(name), "name %q", name)
	}
}

func TestSlugifySubdomain(t *testing.T) {
	cases := map[string]string{
		"Survival":                   "survival",
		"Mi Servidor Épico!":         "mi-servidor-epico",
		"  --Año 2024 ñoño--  ":      "ano-2024-nono",
		"Straße & Co.":               "strasse-co",
		"★":                          "",
		"a__b...c":                   "a-b-c",
		strings.Repeat("ab ", 20):    "ab-ab-ab-ab-ab-ab-ab-ab-ab-ab-ab",
		"Vanilla 1.21 (Java) — test": "vanilla-1-21-java-test",
	}
	for in, want := range cases {
		got := slugifySubdomain(in)
		assert.Equal(t, want, got, "name %q", in)
		assert.LessOrEqual(t, len(got), subdomainMaxLength)
	}
}

func TestSubdomainCandidate(t *testing.T) {
	assert.Equal(t, "survival", subdomainCandidate("survival", 1))
	assert.Equal(t, "survival-2", subdomainCandidate("survival", 2))
	long := strings.Repeat("a", 31) + "b"
	got := subdomainCandidate(long, 12)
	assert.Len(t, got, subdomainMaxLength)
	assert.True(t, strings.HasSuffix(got, "-12"))
	assert.Equal(t, "", validateSubdomain(got))
}

func TestSubdomainBase_FallsBackToID(t *testing.T) {
	assert.Equal(t, "srv-abc-123", subdomainBase(&models.Server{Name: "★", Identifier: "abc_123"}))
	assert.Equal(t, "lobby", subdomainBase(&models.Server{Name: "Lobby", Identifier: "abc"}))
}

func TestWantsSRVRecord(t *testing.T) {
	assert.True(t, wantsSRVRecord(&models.Server{Type: "minecraft", Port: 25565}))
	assert.True(t, wantsSRVRecord(&models.Server{Type: "Minecraft-Java", Port: 25565}))
	assert.False(t, wantsSRVRecord(&models.Server{Type: "minecraft", Port: 0}))
	assert.False(t, wantsSRVRecord(&models.Server{Type: "minecraft-bedrock", Port: 19132}))
	assert.False(t, wantsSRVRecord(&models.Server{Type: "srcds", Port: 27015}))
}

func TestAssignSubdomain_MinecraftGetsAAndSRV(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "mc01", "Survival", "minecraft", 25570, true)

	updated, err := assignSubdomain(context.Background(), env.db, server, "survival", env.settings)
	require.NoError(t, err)
	assert.Equal(t, "survival", updated.Subdomain)
	assert.Len(t, updated.DNSRecordIDs, 2)

	require.Len(t, env.fake.creates, 2)
	a := env.fake.creates[0]
	assert.Equal(t, "A", a.Type)
	assert.Equal(t, "survival.play.example.com", a.Name)
	assert.Equal(t, "203.0.113.5", a.Content)
	require.NotNil(t, a.Proxied)
	assert.False(t, *a.Proxied)
	srv := env.fake.creates[1]
	assert.Equal(t, "SRV", srv.Type)
	assert.Equal(t, "_minecraft._tcp.survival.play.example.com", srv.Name)
	require.NotNil(t, srv.Data)
	assert.Equal(t, 25570, srv.Data.Port)
	assert.Equal(t, "survival.play.example.com", srv.Data.Target)

	stored := env.reload(t, "mc01")
	assert.Equal(t, "survival", stored.Subdomain)
	assert.Equal(t, updated.DNSRecordIDs, stored.DNSRecordIDs)
	assert.Equal(t, "Survival", stored.Name, "only the subdomain columns change")
}

func TestAssignSubdomain_HostnameNodeGetsCNAME(t *testing.T) {
	env := setupDNSTest(t)
	env.node.PublicHost = "node1.hosting.example.net"
	require.NoError(t, env.db.Save(env.node).Error)
	server := env.addServer(t, "rust01", "Rust", "rust", 28015, true)

	_, err := assignSubdomain(context.Background(), env.db, server, "rust", env.settings)
	require.NoError(t, err)
	require.Len(t, env.fake.creates, 1, "no SRV record outside Minecraft")
	assert.Equal(t, "CNAME", env.fake.creates[0].Type)
	assert.Equal(t, "node1.hosting.example.net", env.fake.creates[0].Content)
}

func TestAssignSubdomain_TargetSettingForLocalNode(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "loc01", "Local", "generic", 7777, false)

	_, err := assignSubdomain(context.Background(), env.db, server, "local", env.settings)
	assert.ErrorIs(t, err, errSubdomainNoTarget, "0.0.0.0 cannot be published")
	assert.Empty(t, env.fake.creates)

	env.settings.Target = "2001:db8::10"
	_, err = assignSubdomain(context.Background(), env.db, server, "local", env.settings)
	require.NoError(t, err)
	assert.Equal(t, "AAAA", env.fake.creates[0].Type)
	assert.Equal(t, "2001:db8::10", env.fake.creates[0].Content)
}

func TestAssignSubdomain_UniqueAcrossServers(t *testing.T) {
	env := setupDNSTest(t)
	first := env.addServer(t, "one", "One", "generic", 1000, true)
	second := env.addServer(t, "two", "Two", "generic", 1001, true)

	_, err := assignSubdomain(context.Background(), env.db, first, "shared", env.settings)
	require.NoError(t, err)
	_, err = assignSubdomain(context.Background(), env.db, second, "shared", env.settings)
	assert.ErrorIs(t, err, errSubdomainTaken)
	assert.Equal(t, http.StatusConflict, subdomainErrorStatus(err))
}

func TestAssignSubdomain_ExternalRecordIsNotOverwritten(t *testing.T) {
	env := setupDNSTest(t)
	env.fake.addExternal(services.CloudflareRecord{Type: "A", Name: "lobby.play.example.com", Content: "198.51.100.1"})
	server := env.addServer(t, "lob01", "Lobby", "generic", 1000, true)

	_, err := assignSubdomain(context.Background(), env.db, server, "lobby", env.settings)
	assert.ErrorIs(t, err, errSubdomainTaken)
	assert.Empty(t, env.fake.creates)
}

func TestAssignSubdomain_ChangeRemovesOldRecords(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "mc02", "Creative", "minecraft", 25565, true)

	first, err := assignSubdomain(context.Background(), env.db, server, "creative", env.settings)
	require.NoError(t, err)
	second, err := assignSubdomain(context.Background(), env.db, first, "build", env.settings)
	require.NoError(t, err)

	assert.ElementsMatch(t, first.DNSRecordIDs, env.fake.deletes)
	assert.Empty(t, env.fake.byName("creative.play.example.com"))
	assert.Len(t, env.fake.byName("build.play.example.com"), 1)
	assert.Equal(t, second.DNSRecordIDs, env.reload(t, "mc02").DNSRecordIDs)
}

func TestAssignSubdomain_RollsBackWhenACreateFails(t *testing.T) {
	env := setupDNSTest(t)
	env.fake.failType = "SRV"
	env.fake.failText = "SRV refused"
	server := env.addServer(t, "mc03", "Skyblock", "minecraft", 25565, true)

	_, err := assignSubdomain(context.Background(), env.db, server, "skyblock", env.settings)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, subdomainErrorStatus(err))
	assert.Empty(t, env.fake.byName("skyblock.play.example.com"), "the A record is removed again")
	assert.Equal(t, "", env.reload(t, "mc03").Subdomain)
}

func TestAutoAssignSubdomain_Dedupes(t *testing.T) {
	env := setupDNSTest(t)
	taken := env.addServer(t, "old01", "Survival", "minecraft", 25565, true)
	_, err := assignSubdomain(context.Background(), env.db, taken, "survival", env.settings)
	require.NoError(t, err)
	// survival-2 is used in the zone by something outside the panel
	env.fake.addExternal(services.CloudflareRecord{Type: "CNAME", Name: "survival-2.play.example.com", Content: "elsewhere.example.net"})

	env.addServer(t, "new01", "Survival", "minecraft", 25566, true)
	name, err := autoAssignSubdomain(context.Background(), env.db, "new01", env.settings)
	require.NoError(t, err)
	assert.Equal(t, "survival-3", name)
	assert.Equal(t, "survival-3", env.reload(t, "new01").Subdomain)

	// a reserved name moves on to the next one too
	env.addServer(t, "new02", "Admin", "generic", 1000, true)
	name, err = autoAssignSubdomain(context.Background(), env.db, "new02", env.settings)
	require.NoError(t, err)
	assert.Equal(t, "admin-2", name)
}

func TestAutoAssignSubdomain_MissingServer(t *testing.T) {
	env := setupDNSTest(t)
	_, err := autoAssignSubdomain(context.Background(), env.db, "nope", env.settings)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestReleaseSubdomain_DeletesRecords(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "mc04", "Hub", "minecraft", 25565, true)
	updated, err := assignSubdomain(context.Background(), env.db, server, "hub", env.settings)
	require.NoError(t, err)

	releaseSubdomain(env.db, updated)
	assert.ElementsMatch(t, updated.DNSRecordIDs, env.fake.deletes)
	assert.Empty(t, env.fake.byName("hub.play.example.com"))
}

func TestSubdomainRoutes(t *testing.T) {
	env := setupDNSTest(t)
	env.addServer(t, "mc05", "Factions", "minecraft", 25565, true)
	env.addServer(t, "other", "Other", "generic", 1000, true)
	r := env.router()

	w := doJSON(r, http.MethodGet, "/servers/mc05/subdomain", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var view subdomainView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &view))
	assert.True(t, view.Enabled)
	assert.Equal(t, "play.example.com", view.Domain)
	assert.True(t, view.SRV)
	assert.Empty(t, view.Subdomain)

	w = doJSON(r, http.MethodGet, "/servers/mc05/subdomain/check?name=www", nil)
	assert.JSONEq(t, `{"available":false,"reason":"reserved"}`, w.Body.String())
	w = doJSON(r, http.MethodGet, "/servers/mc05/subdomain/check?name=-x", nil)
	assert.JSONEq(t, `{"available":false,"reason":"invalid"}`, w.Body.String())
	w = doJSON(r, http.MethodGet, "/servers/mc05/subdomain/check?name=factions", nil)
	assert.JSONEq(t, `{"available":true,"reason":""}`, w.Body.String())

	w = doJSON(r, http.MethodPut, "/servers/mc05/subdomain", map[string]string{"subdomain": "Factions"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &view))
	assert.Equal(t, "factions.play.example.com", view.Address)
	require.Len(t, view.Records, 2)
	assert.Equal(t, "0 5 25565 factions.play.example.com", view.Records[1].Content)

	w = doJSON(r, http.MethodGet, "/servers/mc05/subdomain/check?name=factions", nil)
	assert.JSONEq(t, `{"available":true,"reason":"current"}`, w.Body.String())
	w = doJSON(r, http.MethodGet, "/servers/other/subdomain/check?name=factions", nil)
	assert.JSONEq(t, `{"available":false,"reason":"taken"}`, w.Body.String())

	w = doJSON(r, http.MethodPut, "/servers/other/subdomain", map[string]string{"subdomain": "factions"})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "ErrSubdomainTaken")
	w = doJSON(r, http.MethodPut, "/servers/other/subdomain", map[string]string{"subdomain": "admin"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ErrSubdomainReserved")

	// a second change within the interval is refused
	w = doJSON(r, http.MethodPut, "/servers/mc05/subdomain", map[string]string{"subdomain": "pvp"})
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	subdomainChanges = &changeLimiter{interval: subdomainChangeInterval}
	w = doJSON(r, http.MethodDelete, "/servers/mc05/subdomain", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	stored := env.reload(t, "mc05")
	assert.Empty(t, stored.Subdomain)
	assert.Empty(t, stored.DNSRecordIDs)
	assert.Empty(t, env.fake.byName("factions.play.example.com"))
}

func TestSubdomainRoutes_DisabledWithoutSettings(t *testing.T) {
	env := setupDNSTest(t)
	env.settings.Token = ""
	env.addServer(t, "mc06", "Mini", "minecraft", 25565, true)
	r := env.router()

	w := doJSON(r, http.MethodGet, "/servers/mc06/subdomain", nil)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
	w = doJSON(r, http.MethodPut, "/servers/mc06/subdomain", map[string]string{"subdomain": "mini"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ErrSubdomainDisabled")
}

func TestSubdomainRoutes_TokenNeverLeaks(t *testing.T) {
	env := setupDNSTest(t)
	env.fake.failType = "A"
	env.fake.failText = "bad request from token " + dnsTestToken
	env.addServer(t, "mc07", "Leak", "minecraft", 25565, true)

	var logs bytes.Buffer
	previous := logging.Error.Writer()
	logging.Error.SetOutput(&logs)
	defer logging.Error.SetOutput(previous)

	w := doJSON(env.router(), http.MethodPut, "/servers/mc07/subdomain", map[string]string{"subdomain": "leak"})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "Cloudflare")
	assert.NotContains(t, w.Body.String(), dnsTestToken)
	assert.NotEmpty(t, logs.String())
	assert.NotContains(t, logs.String(), dnsTestToken)
}

func TestSendTestCloudflare(t *testing.T) {
	env := setupDNSTest(t)
	r := env.router()

	w := doJSON(r, http.MethodPost, "/settings/test/cloudflare", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"zone":"example.com"}`, w.Body.String())

	// the values on screen win over the saved ones
	w = doJSON(r, http.MethodPost, "/settings/test/cloudflare", map[string]string{"panel.dns.cloudflare.token": "wrong-token-123"})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "Authentication error")
	assert.NotContains(t, w.Body.String(), "wrong-token-123")

	w = doJSON(r, http.MethodPost, "/settings/test/cloudflare", map[string]string{"panel.dns.baseDomain": "play.other.org"})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	w = doJSON(r, http.MethodPost, "/settings/test/cloudflare", map[string]string{"panel.dns.cloudflare.zoneId": ""})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChangeLimiter(t *testing.T) {
	l := &changeLimiter{interval: 10 * time.Second}
	now := time.Now()
	assert.True(t, l.allow("a", now))
	assert.False(t, l.allow("a", now.Add(5*time.Second)))
	assert.True(t, l.allow("b", now.Add(5*time.Second)))
	assert.True(t, l.allow("a", now.Add(11*time.Second)))
}

func TestRefreshSubdomainRecords_FollowsTheNewPort(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "mc01", "Survival", "minecraft", 25570, true)
	updated, err := assignSubdomain(context.Background(), env.db, server, "survival", env.settings)
	require.NoError(t, err)

	require.NoError(t, env.db.Model(&models.Server{}).Where("identifier = ?", "mc01").UpdateColumn("port", 25600).Error)
	moved := env.reload(t, "mc01")
	require.NoError(t, refreshSubdomainRecords(context.Background(), env.db, moved, env.settings))

	srv := env.fake.byName("_minecraft._tcp.survival.play.example.com")
	require.Len(t, srv, 1)
	require.NotNil(t, srv[0].Data)
	assert.Equal(t, 25600, srv[0].Data.Port, "the SRV record points at the new port")
	assert.Len(t, env.fake.creates, 2, "the records are edited in place, not created again")
	assert.ElementsMatch(t, updated.DNSRecordIDs, env.reload(t, "mc01").DNSRecordIDs)
}

func TestRefreshSubdomainRecords_FollowsTheServerToAnotherNode(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "mc01", "Survival", "minecraft", 25570, true)
	_, err := assignSubdomain(context.Background(), env.db, server, "survival", env.settings)
	require.NoError(t, err)

	other := &models.Node{Name: "us-1", PublicHost: "198.51.100.7", PrivateHost: "10.0.0.7", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "other"}
	require.NoError(t, env.db.Create(other).Error)
	require.NoError(t, env.db.Exec("UPDATE servers SET node_id = ? WHERE identifier = ?", other.ID, "mc01").Error)
	require.NoError(t, refreshSubdomainRecords(context.Background(), env.db, env.reload(t, "mc01"), env.settings))

	a := env.fake.byName("survival.play.example.com")
	require.Len(t, a, 1)
	assert.Equal(t, "198.51.100.7", a[0].Content, "the name points at the new node")
}

func TestRefreshSubdomainRecords_DropsTheSRVRecordWhenItNoLongerApplies(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "mc01", "Survival", "minecraft", 25570, true)
	_, err := assignSubdomain(context.Background(), env.db, server, "survival", env.settings)
	require.NoError(t, err)

	require.NoError(t, env.db.Model(&models.Server{}).Where("identifier = ?", "mc01").UpdateColumn("type", "generic").Error)
	require.NoError(t, refreshSubdomainRecords(context.Background(), env.db, env.reload(t, "mc01"), env.settings))

	assert.Empty(t, env.fake.byName("_minecraft._tcp.survival.play.example.com"))
	assert.Len(t, env.fake.byName("survival.play.example.com"), 1)
	assert.Len(t, env.reload(t, "mc01").DNSRecordIDs, 1)
}

func TestRefreshSubdomainRecords_NothingToDoWithoutASubdomain(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "mc01", "Survival", "minecraft", 25570, true)

	require.NoError(t, refreshSubdomainRecords(context.Background(), env.db, server, env.settings))
	assert.Zero(t, env.fake.listCalls, "no Cloudflare call for a server without a subdomain")
}

const dnsOtherZone = "fedcba9876543210fedcba9876543210"

// useTwoZones lets the fake answer for the test zone and a second one, as when the admin moves
// the subdomains to another domain
func (env *dnsTestEnv) useTwoZones() {
	env.fake.mu.Lock()
	defer env.fake.mu.Unlock()
	env.fake.zones = map[string]string{dnsTestZone: "example.com", dnsOtherZone: "example.es"}
}

func (f *dnsFake) zoneOfName(name string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var zones []string
	for id, rec := range f.records {
		if rec.Name == name {
			zones = append(zones, f.zoneOf[id])
		}
	}
	return zones
}

func TestAssignSubdomain_NodeHostnameBeatsTheDNSTarget(t *testing.T) {
	env := setupDNSTest(t)
	env.node.PublicHost = "usa.example.com"
	require.NoError(t, env.db.Save(env.node).Error)
	env.settings.Domain = "example.es"
	env.settings.Target = "198.51.100.99"
	server := env.addServer(t, "mc01", "Pablo", "minecraft", 25565, true)

	_, err := assignSubdomain(context.Background(), env.db, server, "pablo", env.settings)
	require.NoError(t, err)
	require.Len(t, env.fake.creates, 2)
	name := env.fake.creates[0]
	assert.Equal(t, "CNAME", name.Type)
	assert.Equal(t, "pablo.example.es", name.Name, "the subdomain may be under another domain than the node")
	assert.Equal(t, "usa.example.com", name.Content, "a node's own name wins over the panel-wide target")

	srv := env.fake.creates[1]
	require.NotNil(t, srv.Data)
	assert.Equal(t, "usa.example.com", srv.Data.Target, "an SRV record never names an alias")
}

func TestAssignSubdomain_TargetStillCoversANodeWithoutAPublicAddress(t *testing.T) {
	env := setupDNSTest(t)
	env.node.PublicHost = "10.0.0.5"
	require.NoError(t, env.db.Save(env.node).Error)
	env.settings.Target = "198.51.100.99"
	server := env.addServer(t, "rust01", "Rust", "rust", 28015, true)

	_, err := assignSubdomain(context.Background(), env.db, server, "rust", env.settings)
	require.NoError(t, err)
	assert.Equal(t, "A", env.fake.creates[0].Type)
	assert.Equal(t, "198.51.100.99", env.fake.creates[0].Content)
}

func TestAssignSubdomain_RemembersWhereTheRecordsLive(t *testing.T) {
	env := setupDNSTest(t)
	server := env.addServer(t, "mc01", "Survival", "minecraft", 25570, true)

	_, err := assignSubdomain(context.Background(), env.db, server, "survival", env.settings)
	require.NoError(t, err)
	stored := env.reload(t, "mc01")
	assert.Equal(t, "play.example.com", stored.DNSDomain)
	assert.Equal(t, dnsTestZone, stored.DNSZoneID)

	view := models.FromServer(stored)
	assert.Equal(t, "survival.play.example.com", view.Subdomain, "the server list shows the name")
	assert.True(t, view.SubdomainSRV)
}

func TestMoveSubdomain_FollowsANewBaseDomain(t *testing.T) {
	env := setupDNSTest(t)
	env.useTwoZones()
	server := env.addServer(t, "mc01", "Survival", "minecraft", 25570, true)
	_, err := assignSubdomain(context.Background(), env.db, server, "survival", env.settings)
	require.NoError(t, err)

	env.settings.Domain = "example.es"
	env.settings.ZoneID = dnsOtherZone
	before := env.reload(t, "mc01")
	assert.True(t, subdomainElsewhere(before, env.settings))
	assert.Equal(t, "survival.play.example.com", viewSubdomain(before, env.settings).Address, "until it moves, the address that works is shown")

	require.NoError(t, moveSubdomain(context.Background(), env.db, before, env.settings))

	assert.Empty(t, env.fake.byName("survival.play.example.com"), "the old records are removed from the old zone")
	assert.Empty(t, env.fake.byName("_minecraft._tcp.survival.play.example.com"))
	assert.Equal(t, []string{dnsOtherZone}, env.fake.zoneOfName("survival.example.es"))
	assert.Equal(t, []string{dnsOtherZone}, env.fake.zoneOfName("_minecraft._tcp.survival.example.es"))

	after := env.reload(t, "mc01")
	assert.Equal(t, "survival", after.Subdomain)
	assert.Equal(t, "example.es", after.DNSDomain)
	assert.Equal(t, dnsOtherZone, after.DNSZoneID)
	assert.False(t, subdomainElsewhere(after, env.settings))
	assert.Equal(t, "survival.example.es", viewSubdomain(after, env.settings).Address)

	creates := len(env.fake.creates)
	require.NoError(t, moveSubdomain(context.Background(), env.db, after, env.settings))
	assert.Len(t, env.fake.creates, creates, "a server already there is left alone")
}

func TestMoveSubdomain_KeepsTheOldNameWhenTheNewOneIsUsed(t *testing.T) {
	env := setupDNSTest(t)
	env.useTwoZones()
	server := env.addServer(t, "rust01", "Rust", "rust", 28015, true)
	_, err := assignSubdomain(context.Background(), env.db, server, "rust", env.settings)
	require.NoError(t, err)

	env.fake.mu.Lock()
	env.fake.next++
	external := fmt.Sprintf("%032x", env.fake.next)
	env.fake.records[external] = services.CloudflareRecord{ID: external, Type: "A", Name: "rust.example.es", Content: "192.0.2.1"}
	env.fake.zoneOf[external] = dnsOtherZone
	env.fake.mu.Unlock()

	env.settings.Domain = "example.es"
	env.settings.ZoneID = dnsOtherZone
	err = moveSubdomain(context.Background(), env.db, env.reload(t, "rust01"), env.settings)
	assert.ErrorIs(t, err, errSubdomainTaken)

	stored := env.reload(t, "rust01")
	assert.Equal(t, "play.example.com", stored.DNSDomain, "the server keeps the address that still works")
	assert.Len(t, env.fake.byName("rust.play.example.com"), 1)
}

func TestRememberSubdomainHome_FillsOlderSubdomains(t *testing.T) {
	env := setupDNSTest(t)
	env.addServer(t, "old01", "Old", "rust", 28015, true)
	env.addServer(t, "none01", "None", "rust", 28016, true)
	require.NoError(t, env.db.Exec("UPDATE servers SET subdomain = 'old', dns_record_ids = '[\"abc\"]' WHERE identifier = 'old01'").Error)

	require.NoError(t, rememberSubdomainHome(env.db, env.settings))
	old := env.reload(t, "old01")
	assert.Equal(t, "play.example.com", old.DNSDomain)
	assert.Equal(t, dnsTestZone, old.DNSZoneID)
	assert.Empty(t, env.reload(t, "none01").DNSDomain, "a server without a subdomain stays as it is")
}

func TestSweepSubdomains_OnlyTheNodesServers(t *testing.T) {
	env := setupDNSTest(t)
	other := &models.Node{Name: "us-1", PublicHost: "198.51.100.7", PrivateHost: "10.0.0.7", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "other"}
	require.NoError(t, env.db.Create(other).Error)

	first := env.addServer(t, "rust01", "Rust", "rust", 28015, true)
	_, err := assignSubdomain(context.Background(), env.db, first, "rust", env.settings)
	require.NoError(t, err)
	env.addServer(t, "rust02", "Rust 2", "rust", 28016, true)
	require.NoError(t, env.db.Exec("UPDATE servers SET node_id = ? WHERE identifier = ?", other.ID, "rust02").Error)
	_, err = assignSubdomain(context.Background(), env.db, env.reload(t, "rust02"), "rust-two", env.settings)
	require.NoError(t, err)

	env.node.PublicHost = "usa.example.com"
	require.NoError(t, env.db.Save(env.node).Error)
	sweepSubdomains(env.db, "test", &env.node.ID, refreshSubdomainRecords)

	moved := env.fake.byName("rust.play.example.com")
	require.Len(t, moved, 1)
	assert.Equal(t, "CNAME", moved[0].Type)
	assert.Equal(t, "usa.example.com", moved[0].Content, "the node's servers follow its new address")
	untouched := env.fake.byName("rust-two.play.example.com")
	require.Len(t, untouched, 1)
	assert.Equal(t, "198.51.100.7", untouched[0].Content, "the other node's servers are not touched")
}

func TestTouchesDNSSettings(t *testing.T) {
	assert.True(t, touchesDNSSettings("panel.settings.companyName", "panel.dns.baseDomain"))
	assert.True(t, touchesDNSSettings("panel.dns.target"))
	assert.False(t, touchesDNSSettings("panel.settings.companyName", "panel.dns.autoAssign"))
}
