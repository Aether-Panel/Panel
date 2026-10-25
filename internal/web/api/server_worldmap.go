package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/gin-gonic/gin"
)

// The world map is a web app (BlueMap or Dynmap) served by the game server itself, from files
// its owner controls. The panel shows it in an iframe under its own origin, so every answer
// goes out sandboxed (no access to the panel's origin, cookies or storage) and the iframe is
// authorised by a signed link in the path instead of the session.

const (
	// mapViewSandbox is the Content-Security-Policy of every viewer answer: scripts run, but in
	// an opaque origin that cannot call the panel's API with the viewer's session
	mapViewSandbox = "sandbox allow-scripts allow-popups allow-forms allow-pointer-lock allow-downloads"
	// a link stays valid between mapTokenLife and mapTokenLife+1h; it changes once an hour, so
	// the tab's polling keeps the same iframe address
	mapTokenLife     = 12 * time.Hour
	maxMapHTML       = 8 << 20
	mapViewParallel  = 32
	mapViewQueueWait = 15 * time.Second
	mapNodeCacheFor  = time.Minute
)

// headers the viewer reads or rewrites
const (
	headerContentType     = "Content-Type"
	headerCacheControl    = "Cache-Control"
	headerContentEncoding = "Content-Encoding"
	headerContentLength   = "Content-Length"
	headerAcceptEncoding  = "Accept-Encoding"
)

var (
	mapTokenSecret = newMapTokenSecret()
	mapServerID    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	mapHeadTag     = regexp.MustCompile(`(?i)<head(?:\s[^>]*)?>`)
	mapHTMLTag     = regexp.MustCompile(`(?i)<html(?:\s[^>]*)?>`)
	mapDoctype     = regexp.MustCompile(`(?i)^\s*<!doctype[^>]*>`)
	headerToken    = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+.^_|~-]+$`)
)

// mapStorageShim runs first in the map's page. In the sandbox's opaque origin localStorage,
// sessionStorage and document.cookie throw, and workers cannot load from the panel's URL;
// the maps use all of these, so each is replaced by an in-memory (or blob) stand-in only when
// the real one fails.
const mapStorageShim = `<script>(function(){` +
	`function m(){var d=Object.create(null);return{get length(){return Object.keys(d).length},key:function(i){var k=Object.keys(d);return i<k.length?k[i]:null},getItem:function(k){k=String(k);return k in d?d[k]:null},setItem:function(k,v){d[String(k)]=String(v)},removeItem:function(k){delete d[String(k)]},clear:function(){d=Object.create(null)}}}` +
	`["localStorage","sessionStorage"].forEach(function(n){try{window[n].getItem("_")}catch(e){try{Object.defineProperty(window,n,{value:m(),configurable:true,enumerable:true})}catch(_){}}});` +
	`try{void document.cookie}catch(e){var c=Object.create(null);try{Object.defineProperty(document,"cookie",{configurable:true,get:function(){return Object.keys(c).map(function(k){return k+"="+c[k]}).join("; ")},set:function(v){var p=String(v).split(";")[0],i=p.indexOf("=");if(i>0)c[p.slice(0,i).trim()]=p.slice(i+1).trim()}})}catch(_){}}` +
	`if(window.Worker){var W=window.Worker,P=function(u,o){try{return new W(u,o)}catch(e){var a=JSON.stringify(new URL(u,location.href).href),s=o&&o.type==="module"?"import "+a+";":"importScripts("+a+");";return new W(URL.createObjectURL(new Blob([s],{type:"text/javascript"})),o)}};P.prototype=W.prototype;window.Worker=P}` +
	`})();</script>`

func newMapTokenSecret() []byte {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic("no randomness for the world map links: " + err.Error())
	}
	return secret
}

// registerServerWorldMap adds the world map routes
func registerServerWorldMap(g *gin.RouterGroup) {
	g.GET("/:serverId/map", middleware.RequiresPermission(scopes.ScopeServerView), middleware.ResolveServerPanel, getWorldMap)
	g.DELETE("/:serverId/map", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/map", response.CreateOptions("GET", "DELETE"))
	g.POST("/:serverId/map/install", middleware.RequiresPermission(scopes.ScopeServerFileEdit), middleware.ResolveServerPanel, proxyServerRequest)
	g.OPTIONS("/:serverId/map/install", response.CreateOptions("POST"))
}

// registerWorldMapViewer serves the map to the panel's iframe through a signed link
func registerWorldMapViewer(g *gin.RouterGroup) {
	g.GET("/:serverId/:token/*path", viewWorldMap)
	g.HEAD("/:serverId/:token/*path", viewWorldMap)
	g.OPTIONS("/:serverId/:token/*path", viewWorldMap)
}

// signMapToken signs a server id and an expiry: "<expiry base 36>.<HMAC-SHA256>"
func signMapToken(serverID string, expiry time.Time) string {
	exp := strconv.FormatInt(expiry.Unix(), 36)
	return exp + "." + base64.RawURLEncoding.EncodeToString(mapTokenMAC(serverID, exp))
}

func mapTokenMAC(serverID, exp string) []byte {
	mac := hmac.New(sha256.New, mapTokenSecret)
	mac.Write([]byte(serverID))
	mac.Write([]byte{0})
	mac.Write([]byte(exp))
	return mac.Sum(nil)
}

// verifyMapToken checks the signature and that the link has not expired
func verifyMapToken(serverID, token string, now time.Time) bool {
	exp, sig, ok := strings.Cut(token, ".")
	if !ok || len(token) > 128 {
		return false
	}
	unix, err := strconv.ParseInt(exp, 36, 64)
	if err != nil {
		return false
	}
	expiry := time.Unix(unix, 0)
	if !now.Before(expiry) || expiry.After(now.Add(mapTokenLife+time.Hour)) {
		return false
	}
	given, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return false
	}
	return hmac.Equal(given, mapTokenMAC(serverID, exp))
}

// mapViewURL is the iframe address for a server, the same for a whole hour
func mapViewURL(serverID string, now time.Time) string {
	expiry := now.Truncate(time.Hour).Add(mapTokenLife + time.Hour)
	return "/api/map-view/" + url.PathEscape(serverID) + "/" + signMapToken(serverID, expiry) + "/"
}

// @Summary Get world map
// @Description Whether the server can run BlueMap or Dynmap, which one is installed, whether its web map answers, and the signed link that shows it
// @Success 200 {object} servers.WorldMapStatus
// @Param id path string true "Server ID"
// @Tags Server
// @Router /api/servers/{id}/map [get]
// @Security OAuth2Application[server.view]
func getWorldMap(c *gin.Context) {
	server := c.MustGet("server").(*models.Server)
	ns := &services.Node{DB: middleware.GetDatabase(c)}
	res, err := ns.CallNodeContext(c.Request.Context(), &server.Node, http.MethodGet, "/daemon/server/"+url.PathEscape(server.Identifier)+"/map", nil, http.Header{"Accept": {"application/json"}})
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	defer utils.CloseResponse(res)
	if res.StatusCode != http.StatusOK {
		c.DataFromReader(res.StatusCode, -1, res.Header.Get(headerContentType), io.LimitReader(res.Body, 64*1024), nil)
		return
	}
	var status servers.WorldMapStatus
	if response.HandleError(c, json.NewDecoder(io.LimitReader(res.Body, 64*1024)).Decode(&status), http.StatusBadGateway) {
		return
	}
	status.ViewURL = ""
	if status.Installed != "" {
		status.ViewURL = mapViewURL(server.Identifier, time.Now())
	}
	c.JSON(http.StatusOK, status)
}

// cachedMapNode spares the database a lookup for every map tile
type cachedMapNode struct {
	node models.Node
	at   time.Time
}

var (
	mapNodes     = make(map[string]cachedMapNode)
	mapNodesLock sync.Mutex
	mapSlots     = make(map[string]chan struct{})
	mapSlotsLock sync.Mutex
)

func mapNodeFor(c *gin.Context, serverID string) (*models.Node, error) {
	mapNodesLock.Lock()
	cached, ok := mapNodes[serverID]
	mapNodesLock.Unlock()
	if ok && time.Since(cached.at) < mapNodeCacheFor {
		node := cached.node
		return &node, nil
	}
	db := middleware.GetDatabase(c)
	if db == nil {
		return nil, errors.New("no database")
	}
	server, err := (&services.Server{DB: db}).Get(serverID)
	if err != nil {
		return nil, err
	}
	mapNodesLock.Lock()
	mapNodes[serverID] = cachedMapNode{node: server.Node, at: time.Now()}
	mapNodesLock.Unlock()
	node := server.Node
	return &node, nil
}

// acquireMapSlot limits how many viewer requests of one server run at once
func acquireMapSlot(ctx context.Context, serverID string) (release func(), ok bool) {
	mapSlotsLock.Lock()
	slots, found := mapSlots[serverID]
	if !found {
		slots = make(chan struct{}, mapViewParallel)
		mapSlots[serverID] = slots
	}
	mapSlotsLock.Unlock()
	timer := time.NewTimer(mapViewQueueWait)
	defer timer.Stop()
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, true
	case <-ctx.Done():
		return nil, false
	case <-timer.C:
		return nil, false
	}
}

// setMapViewHeaders puts the sandbox on an answer; every viewer answer carries them
func setMapViewHeaders(h http.Header) {
	h.Set("Content-Security-Policy", mapViewSandbox)
	h.Set("X-Content-Type-Options", "nosniff")
	// the page runs in an opaque origin, so its own requests back here are cross-origin; the
	// link in the path is the only credential, so any origin may read the answer
	h.Set("Access-Control-Allow-Origin", "*")
	// the signed link must not leak to sites the map links to
	h.Set("Referrer-Policy", "no-referrer")
	h.Del("Set-Cookie")
}

func mapViewError(c *gin.Context, status int, message string) {
	setMapViewHeaders(c.Writer.Header())
	c.Header(headerCacheControl, "no-store")
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}

// mapResponseHeaders are the map's headers that reach the browser
var mapResponseHeaders = []string{headerContentType, headerContentEncoding, headerContentLength, headerCacheControl, "ETag", "Last-Modified", "Expires", "Content-Range", "Accept-Ranges", "Vary"}

// viewWorldMap serves the map's web app to the iframe; no session, the signed link decides
func viewWorldMap(c *gin.Context) {
	serverID := c.Param("serverId")
	token := c.Param("token")
	if !mapServerID.MatchString(serverID) || !verifyMapToken(serverID, token, time.Now()) {
		mapViewError(c, http.StatusForbidden, "this map link is not valid any more; open the map again from the panel")
		return
	}

	if c.Request.Method == http.MethodOptions {
		answerMapPreflight(c)
		return
	}

	// keep the path escaped as the browser sent it
	escaped := c.Request.URL.EscapedPath()
	marker := "/" + serverID + "/" + token
	index := strings.Index(escaped, marker)
	if index < 0 {
		mapViewError(c, http.StatusBadRequest, "invalid path")
		return
	}
	rest := escaped[index+len(marker):]
	decoded, err := url.PathUnescape(rest)
	if err != nil || !strings.HasPrefix(rest, "/") || len(rest) > 2048 || strings.Contains("/"+decoded+"/", "/../") {
		mapViewError(c, http.StatusBadRequest, "invalid path")
		return
	}

	release, ok := acquireMapSlot(c.Request.Context(), serverID)
	if !ok {
		mapViewError(c, http.StatusServiceUnavailable, "too many map requests at once")
		return
	}
	defer release()

	node, err := mapNodeFor(c, serverID)
	if err != nil {
		mapViewError(c, http.StatusNotFound, "server not found")
		return
	}

	target := "/daemon/server/" + serverID + "/map/view" + rest
	if c.Request.URL.RawQuery != "" && len(c.Request.URL.RawQuery) < 2048 {
		target += "?" + c.Request.URL.RawQuery
	}
	headers := mapNodeHeaders(c)

	ns := &services.Node{DB: middleware.GetDatabase(c)}
	res, err := ns.CallNodeContext(c.Request.Context(), node, c.Request.Method, target, nil, headers)
	if err != nil {
		mapViewError(c, http.StatusBadGateway, "the node could not be reached")
		return
	}
	defer utils.CloseResponse(res)
	writeMapResponse(c, res, path.Ext(decoded))
}

// answerMapPreflight answers the browser's CORS preflight for a viewer request
func answerMapPreflight(c *gin.Context) {
	setMapViewHeaders(c.Writer.Header())
	c.Header("Access-Control-Allow-Methods", "GET, HEAD")
	if requested := c.GetHeader("Access-Control-Request-Headers"); requested != "" {
		allowed := make([]string, 0)
		for _, name := range strings.Split(requested, ",") {
			if name = strings.TrimSpace(name); headerToken.MatchString(name) && len(allowed) < 16 {
				allowed = append(allowed, name)
			}
		}
		c.Header("Access-Control-Allow-Headers", strings.Join(allowed, ", "))
	}
	c.Header("Access-Control-Max-Age", "600")
	c.AbortWithStatus(http.StatusNoContent)
}

// mapNodeHeaders are the browser's request headers passed on to the node
func mapNodeHeaders(c *gin.Context) http.Header {
	headers := http.Header{}
	for _, name := range []string{"Accept", "Accept-Language", "Range", "If-None-Match", "If-Modified-Since"} {
		if value := c.GetHeader(name); value != "" {
			headers.Set(name, value)
		}
	}
	// only gzip: the panel has to be able to unpack a page to add the storage shim
	if strings.Contains(strings.ToLower(c.GetHeader(headerAcceptEncoding)), "gzip") {
		headers.Set(headerAcceptEncoding, "gzip")
	} else {
		headers.Set(headerAcceptEncoding, "identity")
	}
	return headers
}

// writeMapResponse copies the map's answer to the browser with the sandbox headers, adding the
// storage shim to pages
func writeMapResponse(c *gin.Context, res *http.Response, ext string) {
	out := c.Writer.Header()
	for _, name := range mapResponseHeaders {
		if value := res.Header.Get(name); value != "" {
			out.Set(name, value)
		}
	}
	if out.Get(headerContentType) == "" && res.StatusCode == http.StatusOK {
		if guess := mime.TypeByExtension(ext); guess != "" {
			out.Set(headerContentType, guess)
		} else {
			out.Set(headerContentType, "application/octet-stream")
		}
	}
	if out.Get(headerCacheControl) == "" {
		out.Set(headerCacheControl, "private, no-cache")
	}
	setMapViewHeaders(out)

	isHTML := strings.HasPrefix(strings.ToLower(out.Get(headerContentType)), "text/html")
	if !isHTML || c.Request.Method != http.MethodGet || res.StatusCode != http.StatusOK {
		c.Status(res.StatusCode)
		if c.Request.Method != http.MethodHead {
			_, _ = io.Copy(c.Writer, res.Body)
		}
		return
	}

	page, err := prepareMapHTML(res.Body, res.Header.Get(headerContentEncoding))
	if err != nil {
		out.Del(headerContentLength)
		out.Del(headerContentEncoding)
		mapViewError(c, http.StatusBadGateway, err.Error())
		return
	}
	out.Del(headerContentEncoding)
	out.Del("Content-Range")
	out.Del("ETag")
	out.Set(headerContentLength, strconv.Itoa(len(page)))
	out.Set(headerCacheControl, "no-cache")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(page)
}

var errMapPageTooLarge = errors.New("the map's page is too large")

// prepareMapHTML unpacks a page when needed and adds the storage shim
func prepareMapHTML(body io.Reader, encoding string) ([]byte, error) {
	reader := io.LimitReader(body, maxMapHTML+1)
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "identity":
	case "gzip", "x-gzip":
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, errors.New("the map's page could not be unpacked")
		}
		defer gz.Close()
		reader = io.LimitReader(gz, maxMapHTML+1)
	default:
		return nil, errors.New("the map's page uses an unsupported encoding")
	}
	page, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.New("the map's page could not be read")
	}
	if len(page) > maxMapHTML {
		return nil, errMapPageTooLarge
	}
	return injectMapShim(page), nil
}

// injectMapShim puts the shim right after <head>, so it runs before the map's own scripts
func injectMapShim(page []byte) []byte {
	at := 0
	if loc := mapHeadTag.FindIndex(page); loc != nil {
		at = loc[1]
	} else if loc := mapHTMLTag.FindIndex(page); loc != nil {
		at = loc[1]
	} else if loc := mapDoctype.FindIndex(page); loc != nil {
		at = loc[1]
	}
	var out bytes.Buffer
	out.Grow(len(page) + len(mapStorageShim))
	out.Write(page[:at])
	out.WriteString(mapStorageShim)
	out.Write(page[at:])
	return out.Bytes()
}
