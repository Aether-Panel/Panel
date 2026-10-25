package servers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestDetectMapPlatform(t *testing.T) {
	tests := []struct {
		name     string
		modpack  string
		launcher *string
		steps    []string
		platform string
		reason   string
	}{
		{name: "modpack loader wins", modpack: "NeoForge", launcher: strPtr(""), steps: []string{"mojangdl"}, platform: "neoforge"},
		{name: "quilt modpack loads fabric mods", modpack: "quilt", platform: "fabric"},
		{name: "paper launcher", launcher: strPtr("paper"), platform: "paper"},
		{name: "spigot launcher", launcher: strPtr("spigot"), platform: "paper"},
		{name: "purpur launcher", launcher: strPtr("Purpur"), platform: "paper"},
		{name: "fabric launcher", launcher: strPtr("fabric"), platform: "fabric"},
		{name: "forge launcher", launcher: strPtr("forge"), platform: "forge"},
		{name: "hybrid launcher", launcher: strPtr("mohist"), steps: []string{"forgedl"}, reason: "unknown"},
		{name: "empty launcher is vanilla", launcher: strPtr(""), steps: []string{"javadl", "mojangdl"}, reason: "vanilla"},
		{name: "empty launcher without steps", launcher: strPtr(""), reason: "vanilla"},
		{name: "paperdl", steps: []string{"javadl", "paperdl:"}, platform: "paper"},
		{name: "paperdl folia", steps: []string{"paperdl:folia"}, platform: "paper"},
		{name: "paperdl velocity is a proxy", steps: []string{"paperdl:velocity"}, reason: "unknown"},
		{name: "fabricdl", steps: []string{"fabricdl", "command"}, platform: "fabric"},
		{name: "forgedl", steps: []string{"forgedl"}, platform: "forge"},
		{name: "neoforgedl", steps: []string{"neoforgedl"}, platform: "neoforge"},
		{name: "mojangdl", steps: []string{"mojangdl"}, reason: "vanilla"},
		{name: "nothing known", steps: []string{"download", "command"}, reason: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform, reason := detectMapPlatform(tt.modpack, tt.launcher, tt.steps)
			if platform != tt.platform || reason != tt.reason {
				t.Fatalf("got (%q, %q), want (%q, %q)", platform, reason, tt.platform, tt.reason)
			}
		})
	}
}

func TestMapPort(t *testing.T) {
	tests := []struct {
		name string
		vars map[string]interface{}
		kind string
		want int
	}{
		{name: "bluemap default", vars: map[string]interface{}{"port": 25565}, kind: MapBlueMap, want: 8100},
		{name: "dynmap default", vars: map[string]interface{}{"port": 25565}, kind: MapDynmap, want: 8123},
		{name: "first extra port", vars: map[string]interface{}{"port": 25565, "port2": "25570", "port3": 25571}, kind: MapBlueMap, want: 25570},
		{name: "skips empty extra port", vars: map[string]interface{}{"port": 25565, "port2": "", "port3": 25571}, kind: MapDynmap, want: 25571},
		{name: "ignores the game port", vars: map[string]interface{}{"port": 25565, "port2": 25565}, kind: MapBlueMap, want: 8100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapPort(tt.vars, tt.kind); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// fakeMapVersions is what the fake Modrinth API answers for a project's versions
func fakeMapVersions(path, loaders, games string) []map[string]interface{} {
	var versions []map[string]interface{}
	switch {
	case path == "/project/bluemap/version" && loaders == `["paper"]` && games == `["1.21.4"]`:
		versions = []map[string]interface{}{
			{"version_number": "5.6-beta", "version_type": "beta", "files": []map[string]interface{}{{"url": "https://cdn.modrinth.com/b.jar", "filename": "bluemap-5.6-beta-paper.jar", "primary": true}}},
			{"version_number": "5.5", "version_type": "release", "files": []map[string]interface{}{
				{"url": "https://cdn.modrinth.com/sources.jar", "filename": "bluemap-5.5-sources.jar", "primary": false},
				{"url": "https://cdn.modrinth.com/main.jar", "filename": "bluemap-5.5-paper.jar", "primary": true},
			}},
		}
	case path == "/project/dynmap/version" && loaders == `["spigot","bukkit"]`:
		versions = []map[string]interface{}{
			{"version_number": "3.7-beta-8", "version_type": "beta", "files": []map[string]interface{}{{"url": "https://cdn.modrinth.com/d.jar", "filename": "Dynmap-3.7-beta-8-spigot.jar", "primary": true}}},
		}
	case path == "/project/bluemap/version" && loaders == `["fabric"]`:
		versions = []map[string]interface{}{
			{"version_number": "evil", "version_type": "release", "files": []map[string]interface{}{{"url": "https://cdn.modrinth.com/x", "filename": "../../evil.jar", "primary": true}}},
		}
	}
	if versions == nil {
		versions = []map[string]interface{}{}
	}
	return versions
}

func TestFindMapRelease(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/project/") {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing User-Agent")
		}
		loaders := r.URL.Query().Get("loaders")
		games := r.URL.Query().Get("game_versions")
		queries = append(queries, r.URL.Path+" "+loaders+" "+games)
		versions := fakeMapVersions(r.URL.Path, loaders, games)
		_ = json.NewEncoder(w).Encode(versions)
	}))
	defer server.Close()
	previous := mapModrinthAPI
	mapModrinthAPI = server.URL
	defer func() { mapModrinthAPI = previous }()

	file, version, err := findMapRelease(MapBlueMap, "paper", "1.21.4")
	if err != nil || version != "5.5" || file.Filename != "bluemap-5.5-paper.jar" {
		t.Fatalf("bluemap: got %+v %q %v", file, version, err)
	}

	// the paper loader has nothing, so the spigot/bukkit build is used; a beta is fine
	// when there is no release
	queries = nil
	file, version, err = findMapRelease(MapDynmap, "paper", "1.21.4")
	if err != nil || version != "3.7-beta-8" || file.Filename != "Dynmap-3.7-beta-8-spigot.jar" {
		t.Fatalf("dynmap: got %+v %q %v", file, version, err)
	}
	if len(queries) != 2 {
		t.Fatalf("expected the paper query and then the spigot/bukkit fallback, got %v", queries)
	}

	// a file name that would leave the mods folder is never picked
	if _, _, err := findMapRelease(MapBlueMap, "fabric", "1.21.4"); err == nil {
		t.Fatal("expected no usable build")
	}
	if _, _, err := findMapRelease(MapBlueMap, "vanilla", "1.21.4"); err != ErrMapUnsupported {
		t.Fatalf("expected ErrMapUnsupported, got %v", err)
	}
}

func TestCheckMapDownloadURL(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://cdn.modrinth.com/data/x/versions/y/bluemap.jar": true,
		"http://cdn.modrinth.com/data/x.jar":                     false,
		"https://evil.example.com/bluemap.jar":                   false,
		"file:///etc/passwd":                                     false,
	} {
		if err := checkMapDownloadURL(raw); (err == nil) != ok {
			t.Errorf("%s: got %v", raw, err)
		}
	}
}

func TestSetConfigValue(t *testing.T) {
	core := "# BlueMap core config\n# set this to true\naccept-download: false\n\nrender-thread-count: 1\n"
	updated, changed := setConfigValue(core, "accept-download", "true")
	if !changed || updated != "# BlueMap core config\n# set this to true\naccept-download: true\n\nrender-thread-count: 1\n" {
		t.Fatalf("got %q (changed %v)", updated, changed)
	}
	again, changed := setConfigValue(updated, "accept-download", "true")
	if changed || again != updated {
		t.Fatal("patching twice must not change anything")
	}

	// indented keys belong to other blocks and are left alone
	web := "enabled: true\nport: 8100\nlog: {\n  port: 1\n}\n"
	updated, changed = setConfigValue(web, "port", "25570")
	if !changed || updated != "enabled: true\nport: 25570\nlog: {\n  port: 1\n}\n" {
		t.Fatalf("got %q", updated)
	}

	// quoted values compare by their content
	if _, changed := setConfigValue("port: \"8100\"\n", "port", "8100"); changed {
		t.Fatal("a quoted equal value is already right")
	}

	// a commented-out key gets the line right below its comment
	updated, _ = setConfigValue("#port: 8100\nother: 1\n", "port", "25570")
	if updated != "#port: 8100\nport: 25570\nother: 1\n" {
		t.Fatalf("got %q", updated)
	}

	// a missing key is added at the end; Windows line endings stay
	updated, _ = setConfigValue("a: 1\r\nb: 2", "webserver-port", "8123")
	if updated != "a: 1\r\nb: 2\r\nwebserver-port: 8123\r\n" {
		t.Fatalf("got %q", updated)
	}

	// Dynmap's YAML
	dynmap := "deftemplatesuffix: hires\nwebserver-bindaddress: 0.0.0.0\nwebserver-port: 8123\n"
	updated, changed = setConfigValue(dynmap, "webserver-port", "25570")
	if !changed || !strings.Contains(updated, "\nwebserver-port: 25570\n") || !strings.Contains(updated, "webserver-bindaddress: 0.0.0.0") {
		t.Fatalf("got %q", updated)
	}
	if _, changed := setConfigValue(updated, "webserver-port", "25570"); changed {
		t.Fatal("patching twice must not change anything")
	}
}

func TestMapConfigEdits(t *testing.T) {
	edits := mapConfigEdits(&worldMapSidecar{Kind: MapBlueMap, Platform: "paper", AcceptDownload: true}, 8100)
	if len(edits) != 2 || edits[0].file != "plugins/BlueMap/core.conf" || edits[0].key != "accept-download" || edits[1].file != "plugins/BlueMap/webserver.conf" || edits[1].value != "8100" {
		t.Fatalf("got %+v", edits)
	}
	// without the owner's consent the download stays off
	edits = mapConfigEdits(&worldMapSidecar{Kind: MapBlueMap, Platform: "fabric"}, 25570)
	if len(edits) != 1 || edits[0].file != "config/bluemap/webserver.conf" || edits[0].value != "25570" {
		t.Fatalf("got %+v", edits)
	}
	edits = mapConfigEdits(&worldMapSidecar{Kind: MapDynmap, Platform: "forge"}, 8123)
	if len(edits) != 1 || edits[0].file != "dynmap/configuration.txt" || edits[0].key != "webserver-port" {
		t.Fatalf("got %+v", edits)
	}
	edits = mapConfigEdits(&worldMapSidecar{Kind: MapDynmap, Platform: "paper"}, 8123)
	if edits[0].file != "plugins/dynmap/configuration.txt" {
		t.Fatalf("got %+v", edits)
	}
}

func TestSafeRelativePath(t *testing.T) {
	for name, ok := range map[string]bool{
		"plugins/BlueMap-5.5-paper.jar": true,
		"mods/dynmap.jar":               true,
		"../escape.jar":                 false,
		"/etc/passwd":                   false,
		"plugins/../../x.jar":           false,
		"plugins\\x.jar":                false,
		"":                              false,
	} {
		if safeRelativePath(name) != ok {
			t.Errorf("%q: want %v", name, ok)
		}
	}
}

func TestStableGameVersion(t *testing.T) {
	for in, want := range map[string]string{"1.21.4": "1.21.4", "1.20": "1.20", "latest": "", "": "", "24w14a": ""} {
		if got := stableGameVersion(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
	if got := gameVersionPattern.FindString("Paper 1.21.4"); got != "1.21.4" {
		t.Errorf("status version: got %q", got)
	}
}
