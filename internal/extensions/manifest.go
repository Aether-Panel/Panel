// Package extensions installs administrator add-ons and delivers the panel's events to them.
//
// An extension is a zip with an extension.json at its root and the files of its pages.
// Pages are shown in a sandboxed frame (an opaque origin with no access to the panel's
// session or API), and events go to the extension's webhook, signed with its secret.
package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/services"
)

// ManifestFile is the file at the root of every extension package
const ManifestFile = "extension.json"

// Where a page is shown
const (
	// PlacementServer is a tab on every server's page
	PlacementServer = "server"
	// PlacementAdmin is an entry in the administration menu
	PlacementAdmin = "admin"
)

// Events an extension can subscribe to
const (
	EventServerCreated   = "server.created"
	EventServerDeleted   = "server.deleted"
	EventServerStarted   = "server.started"
	EventServerStopped   = "server.stopped"
	EventServerCrashed   = "server.crashed"
	EventServerBackup    = "server.backup"
	EventServerResources = "server.resources"
)

// Events lists every event, in the order the documentation gives them
var Events = []string{
	EventServerCreated, EventServerDeleted, EventServerStarted, EventServerStopped,
	EventServerCrashed, EventServerBackup, EventServerResources,
}

const (
	maxPages = 10
	maxField = 300
)

var (
	idPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)
	pageIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	versionPattern = regexp.MustCompile(`^[0-9A-Za-z.+-]{1,20}$`)
)

// Manifest is extension.json
type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author,omitempty"`
	Description string `json:"description,omitempty"`
	Pages       []Page `json:"pages,omitempty"`
	// Webhook receives the Events the extension subscribed to, as signed JSON posts
	Webhook string   `json:"webhook,omitempty"`
	Events  []string `json:"events,omitempty"`
}

// Page is an HTML file of the package the panel shows in a frame
type Page struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Placement string `json:"placement"`
	// Path is the page's file inside the package, such as "index.html"
	Path string `json:"path"`
}

// ParseManifest reads and checks extension.json; has says whether a file is in the package.
func ParseManifest(data []byte, has func(name string) bool) (*Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s is not valid: %w", ManifestFile, err)
	}
	return &m, m.validate(has)
}

func (m *Manifest) validate(has func(name string) bool) error {
	m.Name = strings.TrimSpace(m.Name)
	m.Author = strings.TrimSpace(m.Author)
	m.Description = strings.TrimSpace(m.Description)
	switch {
	case !idPattern.MatchString(m.ID):
		return errors.New("id must be 2 to 40 lowercase letters, digits or dashes")
	case m.Name == "" || len(m.Name) > 60:
		return errors.New("name must have 1 to 60 characters")
	case !versionPattern.MatchString(m.Version):
		return errors.New("version must be up to 20 characters such as 1.0.0")
	case len(m.Author) > 60 || len(m.Description) > maxField:
		return errors.New("author or description is too long")
	case len(m.Pages) > maxPages:
		return fmt.Errorf("an extension has at most %d pages", maxPages)
	case len(m.Pages) == 0 && m.Webhook == "":
		return errors.New("an extension needs at least one page or a webhook")
	}
	if err := services.ValidateWebhook(m.Webhook); err != nil {
		return errors.New("webhook must be an https:// address")
	}

	seen := map[string]bool{}
	for i := range m.Pages {
		p := &m.Pages[i]
		p.Title = strings.TrimSpace(p.Title)
		p.Path = strings.TrimPrefix(p.Path, "./")
		switch {
		case !pageIDPattern.MatchString(p.ID) || seen[p.ID]:
			return fmt.Errorf("page %q: id must be unique, lowercase letters, digits or dashes", p.ID)
		case p.Title == "" || len(p.Title) > 40:
			return fmt.Errorf("page %q: title must have 1 to 40 characters", p.ID)
		case p.Placement != PlacementServer && p.Placement != PlacementAdmin:
			return fmt.Errorf("page %q: placement must be %q or %q", p.ID, PlacementServer, PlacementAdmin)
		case !isPackagePath(p.Path) || !strings.HasSuffix(strings.ToLower(p.Path), ".html"):
			return fmt.Errorf("page %q: path must be an .html file inside the package", p.ID)
		case !has(p.Path):
			return fmt.Errorf("page %q: %s is not in the package", p.ID, p.Path)
		}
		seen[p.ID] = true
	}

	known := map[string]bool{}
	for _, e := range Events {
		known[e] = true
	}
	for _, e := range m.Events {
		if !known[e] {
			return fmt.Errorf("unknown event %q", e)
		}
	}
	if len(m.Events) > 0 && m.Webhook == "" {
		return errors.New("events need a webhook to be sent to")
	}
	return nil
}

// Subscribed says whether the extension asked for the event
func (m *Manifest) Subscribed(event string) bool {
	for _, e := range m.Events {
		if e == event {
			return true
		}
	}
	return false
}

// isPackagePath accepts a clean relative path that stays inside the package
func isPackagePath(name string) bool {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return false
	}
	clean := path.Clean(name)
	return clean == name && clean != "." && !strings.HasPrefix(clean, "../") && clean != ".."
}
