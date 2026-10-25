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
	next      int
	creates   []services.CloudflareRecord
	deletes   []string
	failType  string // record type whose creation fails
	failText  string
	listCalls int
}

func (f *dnsFake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	reply := func(status int, payload map[string]interface{}) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
	}
	if r.Header.Get("Authorization") != "Bearer "+dnsTestToken {
		reply(http.StatusUnauthorized, map[string]interface{}{"success": false, "errors": []map[string]interface{}{{"code": 10000, "message": "Authentication error"}}})
		return
	}
	prefix := "/zones/" + dnsTestZone
	switch {
	case r.Method == http.MethodGet && r.URL.Path == prefix:
		reply(http.StatusOK, map[string]interface{}{"success": true, "result": map[string]string{"id": dnsTestZone, "name": "example.com"}})
	case r.Method == http.MethodGet && r.URL.Path == prefix+"/dns_records":
		f.listCalls++
		reply(http.StatusOK, map[string]interface{}{"success": true, "result": f.recordsNamed(r.URL.Query().Get("name"))})
	case r.Method == http.MethodPost && r.URL.Path == prefix+"/dns_records":
		var rec services.CloudflareRecord
		_ = json.NewDecoder(r.Body).Decode(&rec)
		if f.failType != "" && rec.Type == f.failType {
			reply(http.StatusBadRequest, map[string]interface{}{"success": false, "errors": []map[string]interface{}{{"code": 9000, "message": f.failText}}})
			return
		}
		f.next++
		rec.ID = fmt.Sprintf("%032x", f.next)
		f.records[rec.ID] = rec
		f.creates = append(f.creates, rec)
		reply(http.StatusOK, map[string]interface{}{"success": true, "result": rec})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, prefix+"/dns_records/"):
		id := strings.TrimPrefix(r.URL.Path, prefix+"/dns_records/")
		f.deletes = append(f.deletes, id)
		if _, ok := f.records[id]; !ok {
			reply(http.StatusNotFound, map[string]interface{}{"success": false, "errors": []map[string]interface{}{{"code": 81044, "message": "Record does not exist."}}})
			return
		}
		delete(f.records, id)
		reply(http.StatusOK, map[string]interface{}{"success": true, "result": map[string]string{"id": id}})
	default:
		reply(http.StatusNotFound, map[string]interface{}{"success": false, "errors": []map[string]interface{}{{"code": 7003, "message": "no route"}}})
	}
}

// recordsNamed lists the records with that name, never nil; the caller holds f.mu
func (f *dnsFake) recordsNamed(name string) []services.CloudflareRecord {
	found := []services.CloudflareRecord{}
	for _, rec := range f.records {
		if rec.Name == name {
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

	fake := &dnsFake{records: map[string]services.CloudflareRecord{}}
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
