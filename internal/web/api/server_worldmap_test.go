package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMapTokenSignAndVerify(t *testing.T) {
	now := time.Now()
	token := signMapToken("abc123", now.Add(mapTokenLife))
	if !verifyMapToken("abc123", token, now) {
		t.Fatal("a fresh token must verify")
	}
	if verifyMapToken("abc124", token, now) {
		t.Fatal("a token is only valid for its own server")
	}
	if verifyMapToken("abc123", token, now.Add(mapTokenLife+time.Second)) {
		t.Fatal("an expired token must fail")
	}
	// a valid signature over an expiry further away than the panel ever signs is refused too
	far := signMapToken("abc123", now.Add(30*24*time.Hour))
	if verifyMapToken("abc123", far, now) {
		t.Fatal("a token valid for a month must fail")
	}
	exp, sig, _ := strings.Cut(token, ".")
	tampered := exp + "." + strings.Repeat("A", len(sig))
	for _, bad := range []string{"", "nodot", "zz.zz", tampered, exp + ".", "." + sig, token + "x", strings.Repeat("a", 200)} {
		if verifyMapToken("abc123", bad, now) {
			t.Errorf("token %q must fail", bad)
		}
	}

	// the iframe link stays the same within the hour and is valid for at least 12 hours
	at := time.Date(2026, 10, 23, 10, 5, 0, 0, time.UTC)
	first := mapViewURL("abc123", at)
	if first != mapViewURL("abc123", at.Add(50*time.Minute)) {
		t.Fatal("the link must not change on every poll")
	}
	if !strings.HasPrefix(first, "/api/map-view/abc123/") || !strings.HasSuffix(first, "/") {
		t.Fatalf("unexpected link %q", first)
	}
	linkToken := strings.TrimSuffix(strings.TrimPrefix(first, "/api/map-view/abc123/"), "/")
	if !verifyMapToken("abc123", linkToken, at.Add(mapTokenLife-time.Minute)) {
		t.Fatal("the link must last 12 hours")
	}
}

func TestInjectMapShim(t *testing.T) {
	tests := []struct {
		name  string
		page  string
		after string
	}{
		{name: "head", page: "<!DOCTYPE html><html><head><title>BlueMap</title></head></html>", after: "<!DOCTYPE html><html><head>"},
		{name: "head with attributes and case", page: "<html lang=en><HEAD data-x=\"1\">\n<script src=x></script>", after: "<html lang=en><HEAD data-x=\"1\">"},
		{name: "header is not head", page: "<html><body><header>x</header>", after: "<html>"},
		{name: "no html tag", page: "<!doctype html>\n<script>map()</script>", after: "<!doctype html>"},
		{name: "fragment", page: "<div>map</div>", after: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(injectMapShim([]byte(tt.page)))
			if !strings.HasPrefix(got, tt.after+mapStorageShim) {
				t.Fatalf("shim not right after %q:\n%s", tt.after, got)
			}
			if strings.Replace(got, mapStorageShim, "", 1) != tt.page {
				t.Fatal("the rest of the page must stay as it was")
			}
		})
	}
}

func TestPrepareMapHTML(t *testing.T) {
	page := "<html><head></head><body>map</body></html>"
	plain, err := prepareMapHTML(strings.NewReader(page), "")
	if err != nil || !bytes.Contains(plain, []byte("<head>"+mapStorageShim+"</head>")) {
		t.Fatalf("plain: %v %s", err, plain)
	}

	var packed bytes.Buffer
	gz := gzip.NewWriter(&packed)
	_, _ = gz.Write([]byte(page))
	_ = gz.Close()
	unpacked, err := prepareMapHTML(&packed, "gzip")
	if err != nil || string(unpacked) != string(plain) {
		t.Fatalf("gzip: %v %s", err, unpacked)
	}

	if _, err := prepareMapHTML(strings.NewReader("not gzip"), "gzip"); err == nil {
		t.Fatal("a broken gzip body must fail")
	}
	if _, err := prepareMapHTML(strings.NewReader(page), "br"); err == nil {
		t.Fatal("an encoding the panel cannot unpack must fail")
	}
	if _, err := prepareMapHTML(io.LimitReader(zeroReader{}, maxMapHTML+10), ""); err != errMapPageTooLarge {
		t.Fatalf("a huge page must fail, got %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

// checkMapSandboxHeaders fails the test unless the answer carries the sandbox and dropped the
// map's other headers
func checkMapSandboxHeaders(t *testing.T, h http.Header) {
	t.Helper()
	if h.Get("Content-Security-Policy") != mapViewSandbox || strings.Contains(mapViewSandbox, "allow-same-origin") {
		t.Fatalf("missing sandbox: %q", h.Get("Content-Security-Policy"))
	}
	if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Set-Cookie") != "" || h.Get("X-Other") != "" {
		t.Fatalf("unexpected headers %v", h)
	}
	if h.Get("Access-Control-Allow-Origin") != "*" || h.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("unexpected headers %v", h)
	}
}

func TestWriteMapResponseHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name        string
		contentType string
		body        string
		wantShim    bool
	}{
		{name: "page", contentType: "text/html; charset=utf-8", body: "<html><head></head></html>", wantShim: true},
		{name: "tile", contentType: "application/octet-stream", body: "tile-bytes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/map-view/a/b/", nil)
			upstream := &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type":   {tt.contentType},
					"Set-Cookie":     {"session=evil"},
					"Content-Length": {"10"},
					"X-Other":        {"1"},
				},
				Body: io.NopCloser(strings.NewReader(tt.body)),
			}
			writeMapResponse(c, upstream, "")
			h := rec.Header()
			checkMapSandboxHeaders(t, h)
			if got := strings.Contains(rec.Body.String(), mapStorageShim); got != tt.wantShim {
				t.Fatalf("shim present %v, want %v", got, tt.wantShim)
			}
			if tt.wantShim && h.Get("Content-Length") != strconv.Itoa(rec.Body.Len()) {
				t.Fatal("the page's length must be recomputed")
			}
		})
	}
}

func TestViewWorldMapRejectsBadLinks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerWorldMapViewer(engine.Group("/api/map-view"))

	for _, target := range []string{
		"/api/map-view/abc123/nope/",
		"/api/map-view/abc123/" + signMapToken("other", time.Now().Add(time.Hour)) + "/",
		"/api/map-view/abc123/" + signMapToken("abc123", time.Now().Add(-time.Minute)) + "/",
	} {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: got %d", target, rec.Code)
		}
		if rec.Header().Get("Content-Security-Policy") != mapViewSandbox {
			t.Errorf("%s: error answers are sandboxed too", target)
		}
	}

	// a preflight with a valid link answers without reaching the node
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/map-view/abc123/"+signMapToken("abc123", time.Now().Add(time.Hour))+"/settings.json", nil)
	req.Header.Set("Access-Control-Request-Headers", "x-requested-with, bad header")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Headers") != "x-requested-with" {
		t.Fatalf("preflight: %d %v", rec.Code, rec.Header())
	}
}
