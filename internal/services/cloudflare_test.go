package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCFToken = "cf-secret-token-0123456789"
	testCFZone  = "0123456789abcdef0123456789abcdef"
)

type cfCall struct {
	Method string
	Path   string
	Query  string
	Body   map[string]interface{}
}

// fakeCloudflare answers like the API for one zone named example.com
type fakeCloudflare struct {
	mu      sync.Mutex
	calls   []cfCall
	records map[string]CloudflareRecord
	next    int
	fail    string // message of an error to answer every write with
}

func (f *fakeCloudflare) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		call := cfCall{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery}
		if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodDelete {
			_ = json.NewDecoder(r.Body).Decode(&call.Body)
		}
		f.calls = append(f.calls, call)

		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+testCFToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`))
			return
		}
		prefix := "/zones/" + testCFZone
		switch {
		case r.Method == http.MethodGet && r.URL.Path == prefix:
			writeCFResult(w, CloudflareZone{ID: testCFZone, Name: "example.com", Status: "active"})
		case r.Method == http.MethodGet && r.URL.Path == prefix+"/dns_records":
			writeCFResult(w, f.findRecords(r.URL.Query().Get("name")))
		case r.Method == http.MethodPost && r.URL.Path == prefix+"/dns_records":
			f.createRecord(w, call.Body)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, prefix+"/dns_records/"):
			f.deleteRecord(w, strings.TrimPrefix(r.URL.Path, prefix+"/dns_records/"))
		default:
			t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":7003,"message":"No route"}]}`))
		}
	})
}

// writeCFResult answers a successful API call
func writeCFResult(w http.ResponseWriter, result interface{}) {
	data, _ := json.Marshal(map[string]interface{}{"success": true, "errors": []interface{}{}, "result": result})
	_, _ = w.Write(data)
}

func (f *fakeCloudflare) findRecords(name string) []CloudflareRecord {
	var found []CloudflareRecord
	for _, rec := range f.records {
		if rec.Name == name {
			found = append(found, rec)
		}
	}
	return found
}

func (f *fakeCloudflare) createRecord(w http.ResponseWriter, body map[string]interface{}) {
	if f.fail != "" {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(map[string]interface{}{"success": false, "errors": []map[string]interface{}{{"code": 9109, "message": f.fail}}})
		_, _ = w.Write(data)
		return
	}
	var rec CloudflareRecord
	data, _ := json.Marshal(body)
	_ = json.Unmarshal(data, &rec)
	f.next++
	rec.ID = fmt.Sprintf("%032x", f.next)
	if f.records == nil {
		f.records = map[string]CloudflareRecord{}
	}
	f.records[rec.ID] = rec
	writeCFResult(w, rec)
}

func (f *fakeCloudflare) deleteRecord(w http.ResponseWriter, id string) {
	if _, exists := f.records[id]; !exists {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":81044,"message":"Record does not exist."}]}`))
		return
	}
	delete(f.records, id)
	writeCFResult(w, map[string]string{"id": id})
}

func newTestCloudflare(t *testing.T) (*Cloudflare, *fakeCloudflare) {
	fake := &fakeCloudflare{}
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)
	cf := NewCloudflare(testCFToken, testCFZone)
	cf.BaseURL = server.URL
	return cf, fake
}

func TestCloudflare_CreateListDelete(t *testing.T) {
	cf, fake := newTestCloudflare(t)
	ctx := context.Background()
	proxied := false

	rec, err := cf.CreateRecord(ctx, CloudflareRecord{Type: "A", Name: "survival.play.example.com", Content: "203.0.113.5", TTL: 1, Proxied: &proxied})
	require.NoError(t, err)
	assert.Len(t, rec.ID, 32)
	assert.Equal(t, false, fake.calls[0].Body["proxied"], "game records are never proxied")

	found, err := cf.ListRecords(ctx, "survival.play.example.com")
	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Contains(t, fake.calls[1].Query, "name=survival.play.example.com")

	require.NoError(t, cf.DeleteRecord(ctx, rec.ID))
	assert.Empty(t, fake.records)
	// already gone counts as removed
	require.NoError(t, cf.DeleteRecord(ctx, rec.ID))
}

func TestCloudflare_SRVRecordBody(t *testing.T) {
	cf, fake := newTestCloudflare(t)
	_, err := cf.CreateRecord(context.Background(), CloudflareRecord{
		Type: "SRV",
		Name: "_minecraft._tcp.survival.play.example.com",
		TTL:  1,
		Data: &CloudflareSRV{Priority: 0, Weight: 5, Port: 25570, Target: "survival.play.example.com"},
	})
	require.NoError(t, err)
	body := fake.calls[0].Body
	assert.NotContains(t, body, "proxied", "SRV records cannot be proxied")
	data := body["data"].(map[string]interface{})
	assert.EqualValues(t, 25570, data["port"])
	assert.Equal(t, "survival.play.example.com", data["target"])
}

func TestCloudflare_TokenNeverInErrors(t *testing.T) {
	cf, fake := newTestCloudflare(t)
	fake.fail = "Invalid request for token " + testCFToken

	_, err := cf.CreateRecord(context.Background(), CloudflareRecord{Type: "A", Name: "x.example.com", Content: "203.0.113.5"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testCFToken)
	assert.Contains(t, err.Error(), "9109")

	// a wrong token reads as Cloudflare's own refusal
	cf.Token = "wrong-token-abcdef"
	_, err = cf.Zone(context.Background())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "wrong-token-abcdef")
	assert.Contains(t, err.Error(), "Authentication error")

	// a network failure does not carry it either
	cf = NewCloudflare(testCFToken, testCFZone)
	cf.BaseURL = "http://127.0.0.1:1/" + testCFToken
	_, err = cf.Zone(context.Background())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testCFToken)
}

func TestCloudflare_RefusesBadIDs(t *testing.T) {
	cf, fake := newTestCloudflare(t)
	cf.ZoneID = "../../user"
	_, err := cf.Zone(context.Background())
	assert.Error(t, err)

	cf.ZoneID = testCFZone
	assert.Error(t, cf.DeleteRecord(context.Background(), "../tokens"))
	assert.Empty(t, fake.calls)

	assert.ErrorIs(t, NewCloudflare("", testCFZone).DeleteRecord(context.Background(), strings.Repeat("a", 32)), ErrCloudflareNotConfigured)
}

func TestCloudflare_Verify(t *testing.T) {
	cf, fake := newTestCloudflare(t)

	zone, err := cf.Verify(context.Background(), "play.example.com")
	require.NoError(t, err)
	assert.Equal(t, "example.com", zone)
	assert.Empty(t, fake.records, "the test record is removed again")
	assert.Equal(t, "_aether-panel-test.play.example.com", fake.calls[1].Body["name"])

	_, err = cf.Verify(context.Background(), "play.other.org")
	assert.ErrorContains(t, err, "no pertenece")
}
