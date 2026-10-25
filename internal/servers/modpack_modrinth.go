package servers

import (
	"archive/zip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// Modrinth modpacks (.mrpack): a zip with modrinth.index.json, which lists the mods to download
// with their hashes, plus overrides/ and server-overrides/ folders copied over the server.
// Format: https://support.modrinth.com/en/articles/8802351-modrinth-modpack-format-mrpack

var (
	// modrinthAPI and the download checks are variables so tests can use a local server
	modrinthAPI = "https://api.modrinth.com/v2"
	// modpackDownloadAllowed are the hosts a .mrpack may download files from (the ones
	// Modrinth itself allows in published packs)
	modpackDownloadAllowed = func(u *url.URL) bool {
		if u.Scheme != "https" {
			return false
		}
		switch strings.ToLower(u.Hostname()) {
		case "cdn.modrinth.com", "github.com", "raw.githubusercontent.com", "gitlab.com":
			return true
		}
		return false
	}
)

const (
	modpackPageSize   = 20
	maxMrpackSize     = 1 << 30 // 1 GiB
	maxPackFileSize   = 512 << 20
	maxOverridesSize  = 4 << 30
	maxIndexSize      = 32 << 20
	maxAPIResponse    = 16 << 20
	packDownloaders   = 6
	fileDownloadLimit = 10 * time.Minute
)

// installableLoaders are the loaders the panel can set up, in order of preference
var installableLoaders = []string{"neoforge", "forge", "fabric"}

// ModpackHit is one search result
type ModpackHit struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Author       string    `json:"author"`
	IconURL      string    `json:"iconUrl"`
	Downloads    int64     `json:"downloads"`
	Loaders      []string  `json:"loaders"`
	GameVersions []string  `json:"gameVersions"`
	UpdatedAt    time.Time `json:"updatedAt"`
} // @name ModpackHit

// ModpackSearchResult is a page of search results
type ModpackSearchResult struct {
	Hits   []ModpackHit `json:"hits"`
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
} // @name ModpackSearchResult

// ModpackVersion is one installable version of a pack
type ModpackVersion struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	VersionNumber string    `json:"versionNumber"`
	GameVersions  []string  `json:"gameVersions"`
	Loaders       []string  `json:"loaders"`
	PublishedAt   time.Time `json:"publishedAt"`
	Downloads     int64     `json:"downloads"`
	// Type is release, beta or alpha
	Type string `json:"type"`
	// ServerPack is true for CurseForge files that are (or come with) a server pack
	ServerPack bool `json:"serverPack"`
} // @name ModpackVersion

// SearchModpacks searches a source; an empty query lists the most popular packs
func SearchModpacks(source, query string, offset int) (ModpackSearchResult, error) {
	if offset < 0 || offset > 10000 {
		offset = 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	switch source {
	case ModpackSourceModrinth:
		return searchModrinth(ctx, query, offset)
	case ModpackSourceCurseForge:
		if config.CurseForgeKey.Value() == "" {
			return ModpackSearchResult{}, ErrCurseForgeKeyMissing
		}
		return searchCurseForge(ctx, query, offset)
	}
	return ModpackSearchResult{}, fmt.Errorf("%w: source must be modrinth or curseforge", ErrModpackBadRequest)
}

// ModpackVersions lists the installable versions of a pack, newest first
func ModpackVersions(source, project string) ([]ModpackVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	switch source {
	case ModpackSourceModrinth:
		if !idPattern.MatchString(project) {
			return nil, fmt.Errorf("%w: unknown project", ErrModpackBadRequest)
		}
		return modrinthVersions(ctx, project)
	case ModpackSourceCurseForge:
		if !numericPattern.MatchString(project) {
			return nil, fmt.Errorf("%w: unknown project", ErrModpackBadRequest)
		}
		if config.CurseForgeKey.Value() == "" {
			return nil, ErrCurseForgeKeyMissing
		}
		return curseForgeVersions(ctx, project)
	}
	return nil, fmt.Errorf("%w: source must be modrinth or curseforge", ErrModpackBadRequest)
}

// getModpackJSON calls a JSON API; failures come back as UpstreamError
func getModpackJSON(ctx context.Context, site, address string, headers map[string]string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", skypanel.Display+" https://github.com/SkyPanel/SkyPanel")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := skypanel.HTTP().Do(req)
	defer utils.CloseResponse(res)
	if err != nil {
		return upstream(site, errors.New("could not be reached"))
	}
	switch {
	case res.StatusCode == http.StatusNotFound:
		return upstream(site, errors.New("not found"))
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return upstream(site, errors.New("refused the request (check the API key)"))
	case res.StatusCode == http.StatusTooManyRequests:
		return upstream(site, errors.New("too many requests, try again in a minute"))
	case res.StatusCode != http.StatusOK:
		return upstream(site, fmt.Errorf("answered %d", res.StatusCode))
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxAPIResponse)).Decode(target); err != nil {
		return upstream(site, fmt.Errorf("sent an unexpected answer: %w", err))
	}
	return nil
}

func searchModrinth(ctx context.Context, query string, offset int) (ModpackSearchResult, error) {
	facets := `[["project_type:modpack"],["categories:fabric","categories:forge","categories:neoforge"],["server_side:required","server_side:optional"]]`
	q := url.Values{}
	q.Set("facets", facets)
	q.Set("limit", strconv.Itoa(modpackPageSize))
	q.Set("offset", strconv.Itoa(offset))
	if query = strings.TrimSpace(query); query != "" {
		q.Set("query", query)
		q.Set("index", "relevance")
	} else {
		q.Set("index", "downloads")
	}
	var res struct {
		Hits []struct {
			ProjectID    string    `json:"project_id"`
			Slug         string    `json:"slug"`
			Title        string    `json:"title"`
			Description  string    `json:"description"`
			Author       string    `json:"author"`
			IconURL      string    `json:"icon_url"`
			Downloads    int64     `json:"downloads"`
			Categories   []string  `json:"categories"`
			Versions     []string  `json:"versions"`
			DateModified time.Time `json:"date_modified"`
		} `json:"hits"`
		Offset    int `json:"offset"`
		Limit     int `json:"limit"`
		TotalHits int `json:"total_hits"`
	}
	if err := getModpackJSON(ctx, "Modrinth", modrinthAPI+"/search?"+q.Encode(), nil, &res); err != nil {
		return ModpackSearchResult{}, err
	}
	result := ModpackSearchResult{Hits: make([]ModpackHit, 0, len(res.Hits)), Total: res.TotalHits, Offset: res.Offset, Limit: res.Limit}
	for _, hit := range res.Hits {
		result.Hits = append(result.Hits, ModpackHit{
			ID:           hit.ProjectID,
			Slug:         hit.Slug,
			Title:        hit.Title,
			Description:  hit.Description,
			Author:       hit.Author,
			IconURL:      hit.IconURL,
			Downloads:    hit.Downloads,
			Loaders:      supportedLoaders(hit.Categories),
			GameVersions: releaseVersions(hit.Versions),
			UpdatedAt:    hit.DateModified,
		})
	}
	return result, nil
}

// supportedLoaders keeps the loaders the panel installs, in a stable order
func supportedLoaders(values []string) []string {
	loaders := make([]string, 0, 2)
	for _, loader := range installableLoaders {
		for _, v := range values {
			if strings.EqualFold(v, loader) {
				loaders = append(loaders, loader)
				break
			}
		}
	}
	return loaders
}

// releaseVersions keeps release game versions, newest first
func releaseVersions(values []string) []string {
	versions := make([]string, 0, len(values))
	for _, v := range values {
		if stableVersion.MatchString(v) {
			versions = append(versions, v)
		}
	}
	return sortVersionsDesc(versions)
}

type modrinthVersion struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Name          string    `json:"name"`
	VersionNumber string    `json:"version_number"`
	VersionType   string    `json:"version_type"`
	GameVersions  []string  `json:"game_versions"`
	Loaders       []string  `json:"loaders"`
	DatePublished time.Time `json:"date_published"`
	Downloads     int64     `json:"downloads"`
	Files         []struct {
		URL      string            `json:"url"`
		Filename string            `json:"filename"`
		Primary  bool              `json:"primary"`
		Size     int64             `json:"size"`
		Hashes   map[string]string `json:"hashes"`
	} `json:"files"`
}

// mrpackFile is the .mrpack of a version, if it has one
func (v modrinthVersion) mrpackFile() (address string, size int64, hashes map[string]string, ok bool) {
	for _, f := range v.Files {
		if !strings.HasSuffix(strings.ToLower(f.Filename), ".mrpack") {
			continue
		}
		if !ok || f.Primary {
			address, size, hashes, ok = f.URL, f.Size, f.Hashes, true
		}
	}
	return
}

func modrinthVersions(ctx context.Context, project string) ([]ModpackVersion, error) {
	var res []modrinthVersion
	if err := getModpackJSON(ctx, "Modrinth", modrinthAPI+"/project/"+url.PathEscape(project)+"/version", nil, &res); err != nil {
		return nil, err
	}
	versions := make([]ModpackVersion, 0, len(res))
	for _, v := range res {
		loaders := supportedLoaders(v.Loaders)
		if len(loaders) == 0 {
			continue
		}
		if _, _, _, ok := v.mrpackFile(); !ok {
			continue
		}
		versions = append(versions, ModpackVersion{
			ID:            v.ID,
			Name:          v.Name,
			VersionNumber: v.VersionNumber,
			GameVersions:  releaseVersions(v.GameVersions),
			Loaders:       loaders,
			PublishedAt:   v.DatePublished,
			Downloads:     v.Downloads,
			Type:          v.VersionType,
		})
	}
	sort.SliceStable(versions, func(i, j int) bool { return versions[i].PublishedAt.After(versions[j].PublishedAt) })
	if len(versions) > 100 {
		versions = versions[:100]
	}
	return versions, nil
}

func resolveModrinthPlan(ctx context.Context, req ModpackInstallRequest) (modpackPlan, error) {
	var v modrinthVersion
	if err := getModpackJSON(ctx, "Modrinth", modrinthAPI+"/version/"+url.PathEscape(req.VersionID), nil, &v); err != nil {
		return modpackPlan{}, err
	}
	var project struct {
		ID      string `json:"id"`
		Slug    string `json:"slug"`
		Title   string `json:"title"`
		IconURL string `json:"icon_url"`
	}
	if err := getModpackJSON(ctx, "Modrinth", modrinthAPI+"/project/"+url.PathEscape(req.ProjectID), nil, &project); err != nil {
		return modpackPlan{}, err
	}
	if v.ProjectID != project.ID {
		return modpackPlan{}, fmt.Errorf("%w: that version belongs to another pack", ErrModpackBadRequest)
	}
	loaders := supportedLoaders(v.Loaders)
	if len(loaders) == 0 {
		return modpackPlan{}, ErrModpackUnsupported
	}
	address, size, hashes, ok := v.mrpackFile()
	if !ok {
		return modpackPlan{}, fmt.Errorf("%w: that version has no .mrpack file", ErrModpackBadRequest)
	}
	gameVersion := ""
	if versions := releaseVersions(v.GameVersions); len(versions) > 0 {
		gameVersion = versions[0]
	} else if len(v.GameVersions) > 0 {
		gameVersion = v.GameVersions[0]
	}
	return modpackPlan{
		request:  req,
		packURL:  address,
		packSize: size,
		hashes:   hashes,
		pack: InstalledModpack{
			Source:      ModpackSourceModrinth,
			ProjectID:   project.ID,
			VersionID:   v.ID,
			Title:       project.Title,
			VersionName: firstNonEmpty(v.VersionNumber, v.Name),
			IconURL:     project.IconURL,
			Loader:      loaders[0],
			GameVersion: gameVersion,
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// mrpackIndex is modrinth.index.json
type mrpackIndex struct {
	FormatVersion int    `json:"formatVersion"`
	Game          string `json:"game"`
	Name          string `json:"name"`
	VersionID     string `json:"versionId"`
	Files         []struct {
		Path      string            `json:"path"`
		Hashes    map[string]string `json:"hashes"`
		Env       map[string]string `json:"env"`
		Downloads []string          `json:"downloads"`
		FileSize  int64             `json:"fileSize"`
	} `json:"files"`
	Dependencies map[string]string `json:"dependencies"`
}

// packLoader reads the loader and its version from the index dependencies
func (idx mrpackIndex) packLoader() (loader, loaderVersion string, err error) {
	switch {
	case idx.Dependencies["neoforge"] != "":
		return "neoforge", idx.Dependencies["neoforge"], nil
	case idx.Dependencies["forge"] != "":
		return "forge", idx.Dependencies["forge"], nil
	case idx.Dependencies["fabric-loader"] != "":
		return "fabric", idx.Dependencies["fabric-loader"], nil
	}
	return "", "", ErrModpackUnsupported
}

func readMrpackIndex(zr *zip.Reader) (mrpackIndex, error) {
	var idx mrpackIndex
	f, err := zr.Open("modrinth.index.json")
	if err != nil {
		return idx, errors.New("the pack has no modrinth.index.json")
	}
	defer utils.Close(f)
	if err := json.NewDecoder(io.LimitReader(f, maxIndexSize)).Decode(&idx); err != nil {
		return idx, fmt.Errorf("the pack index cannot be read: %w", err)
	}
	if idx.FormatVersion != 1 || idx.Game != "minecraft" {
		return idx, errors.New("the pack is not a Minecraft pack this panel understands")
	}
	if idx.Dependencies["minecraft"] == "" {
		return idx, errors.New("the pack does not say which Minecraft version it needs")
	}
	return idx, nil
}

// verifier checks a download against its SHA-512 (Modrinth lists one for every file; the
// SHA-1 it also lists is too weak to rely on)
type verifier struct {
	name     string
	expected string
	h        hash.Hash
}

func newVerifier(hashes map[string]string) (*verifier, error) {
	if v := strings.ToLower(strings.TrimSpace(hashes["sha512"])); v != "" {
		return &verifier{name: "sha512", expected: v, h: sha512.New()}, nil
	}
	return nil, errors.New("no SHA-512 hash to check the file against")
}

func (v *verifier) check(name string) error {
	if hex.EncodeToString(v.h.Sum(nil)) != v.expected {
		return fmt.Errorf("%s did not match its %s hash; the download is damaged or was changed", name, v.name)
	}
	return nil
}

// openDownload starts a download of an allowed URL
func openDownload(ctx context.Context, address string) (*http.Response, error) {
	u, err := url.Parse(address)
	if err != nil || !modpackDownloadAllowed(u) {
		return nil, fmt.Errorf("the pack wants a download from a host that is not allowed: %s", address)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", skypanel.Display+" https://github.com/SkyPanel/SkyPanel")
	res, err := skypanel.HTTP().Do(req)
	if err != nil {
		return nil, upstream(u.Hostname(), errors.New("could not be reached"))
	}
	if res.StatusCode != http.StatusOK {
		utils.CloseResponse(res)
		return nil, upstream(u.Hostname(), fmt.Errorf("answered %d for %s", res.StatusCode, u.Path))
	}
	return res, nil
}

// downloadMrpack saves the .mrpack to a temporary file and checks its hash
func downloadMrpack(ctx context.Context, address string, size int64, hashes map[string]string) (string, error) {
	check, err := newVerifier(hashes)
	if err != nil {
		return "", err
	}
	if size > maxMrpackSize {
		return "", errors.New("the pack is larger than 1 GB")
	}
	res, err := openDownload(ctx, address)
	if err != nil {
		return "", err
	}
	defer utils.CloseResponse(res)

	dir := config.CacheFolder.Value()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "modpack-*.mrpack")
	if err != nil {
		return "", err
	}
	n, err := io.Copy(io.MultiWriter(tmp, check.h), io.LimitReader(res.Body, maxMrpackSize+1))
	closeErr := tmp.Close()
	if err == nil && n > maxMrpackSize {
		err = errors.New("the pack is larger than 1 GB")
	}
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = check.check("the pack")
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// downloadPackFile downloads one mod into the server folder and checks its hash
func downloadPackFile(ctx context.Context, fsys files.FileServer, target string, downloads []string, hashes map[string]string, size int64) error {
	limit := int64(maxPackFileSize)
	if size > 0 {
		if size > maxPackFileSize {
			return fmt.Errorf("%s is larger than allowed", target)
		}
		limit = size
	}
	var lastErr error
	for _, address := range downloads {
		check, err := newVerifier(hashes)
		if err != nil {
			return fmt.Errorf("%s: %w", target, err)
		}
		fileCtx, cancel := context.WithTimeout(ctx, fileDownloadLimit)
		res, err := openDownload(fileCtx, address)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		_, err = writeFileFS(fsys, target, io.TeeReader(res.Body, check.h), limit)
		utils.CloseResponse(res)
		cancel()
		if err == nil {
			if err = check.check(target); err != nil {
				_ = fsys.Remove(target)
			}
		}
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%s has no download", target)
	}
	return lastErr
}

// downloadMrpackFiles downloads the server's files of the index, a few at a time
func downloadMrpackFiles(ctx context.Context, fsys files.FileServer, idx mrpackIndex, progress func(done, total int)) error {
	type job struct {
		target    string
		downloads []string
		hashes    map[string]string
		size      int64
	}
	jobs := make([]job, 0, len(idx.Files))
	for _, f := range idx.Files {
		if strings.EqualFold(f.Env["server"], "unsupported") {
			continue
		}
		target, err := cleanPackPath(f.Path)
		if err != nil {
			return err
		}
		jobs = append(jobs, job{target: target, downloads: f.Downloads, hashes: f.Hashes, size: f.FileSize})
	}
	total := len(jobs)
	progress(0, total)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		done     atomic.Int64
		firstErr error
		errOnce  sync.Once
		wg       sync.WaitGroup
	)
	queue := make(chan job)
	for i := 0; i < packDownloaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range queue {
				if ctx.Err() != nil {
					continue
				}
				if err := downloadPackFile(ctx, fsys, j.target, j.downloads, j.hashes, j.size); err != nil {
					errOnce.Do(func() { firstErr = err; cancel() })
					continue
				}
				progress(int(done.Add(1)), total)
			}
		}()
	}
	for _, j := range jobs {
		queue <- j
	}
	close(queue)
	wg.Wait()
	return firstErr
}

// extractMrpackOverrides copies overrides/ and then server-overrides/ over the server folder
func extractMrpackOverrides(fsys files.FileServer, zr *zip.Reader) error {
	var written int64
	for _, prefix := range []string{"overrides/", "server-overrides/"} {
		for _, entry := range zr.File {
			if !strings.HasPrefix(entry.Name, prefix) {
				continue
			}
			rel := strings.TrimPrefix(entry.Name, prefix)
			if rel == "" {
				continue
			}
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}
			n, err := extractMrpackOverride(fsys, entry, rel, int64(maxOverridesSize)-written)
			if err != nil {
				return err
			}
			written += n
		}
	}
	return nil
}

// extractMrpackOverride writes one overrides entry, keeping files within the remaining size
// budget; it returns the bytes written
func extractMrpackOverride(fsys files.FileServer, entry *zip.File, rel string, remaining int64) (int64, error) {
	target, err := cleanPackPath(strings.TrimSuffix(rel, "/"))
	if err != nil {
		return 0, err
	}
	if entry.FileInfo().IsDir() {
		return 0, fsys.MkdirAll(target, 0755)
	}
	if remaining <= 0 {
		return 0, errors.New("the pack's files are larger than allowed")
	}
	rc, err := entry.Open()
	if err != nil {
		return 0, err
	}
	n, err := writeFileFS(fsys, target, rc, remaining)
	utils.Close(rc)
	return n, err
}

// applyMrpack installs the files of a downloaded .mrpack and returns its index
func applyMrpack(ctx context.Context, fsys files.FileServer, packPath string, step func(step string, current, total int)) (mrpackIndex, error) {
	zr, err := zip.OpenReader(packPath)
	if err != nil {
		return mrpackIndex{}, fmt.Errorf("the pack is not a valid .mrpack: %w", err)
	}
	defer utils.Close(zr)
	idx, err := readMrpackIndex(&zr.Reader)
	if err != nil {
		return idx, err
	}
	if _, _, err := idx.packLoader(); err != nil {
		return idx, err
	}
	step(ModpackStepDownloadFiles, 0, 0)
	if err := downloadMrpackFiles(ctx, fsys, idx, func(done, total int) { step(ModpackStepDownloadFiles, done, total) }); err != nil {
		return idx, err
	}
	step(ModpackStepExtracting, 0, 0)
	if err := extractMrpackOverrides(fsys, &zr.Reader); err != nil {
		return idx, err
	}
	return idx, nil
}

func (p *Server) installModrinthPack(plan modpackPlan, pack *InstalledModpack, state *modpackJob) error {
	env := p.RunningEnvironment
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	state.setStep(ModpackStepDownloadPack, 0, 0)
	env.DisplayToConsole(true, "Downloading the pack\n")
	packPath, err := downloadMrpack(ctx, plan.packURL, plan.packSize, plan.hashes)
	if err != nil {
		return err
	}
	defer os.Remove(packPath)

	// downloads report from several workers at once
	var progressLock sync.Mutex
	lastShown, highest := -1, 0
	idx, err := applyMrpack(ctx, p.GetFileServer(), packPath, func(step string, current, total int) {
		progressLock.Lock()
		defer progressLock.Unlock()
		switch step {
		case ModpackStepDownloadFiles:
			if current < highest {
				return
			}
			highest = current
			state.setStep(step, current, total)
			// one line every 10% keeps the console readable
			if total > 0 && current*10/total != lastShown {
				lastShown = current * 10 / total
				env.DisplayToConsole(true, "Downloading mods %d/%d\n", current, total)
			}
		case ModpackStepExtracting:
			state.setStep(step, current, total)
			env.DisplayToConsole(true, "Copying the pack's configuration\n")
		}
	})
	if err != nil {
		return err
	}

	mc := idx.Dependencies["minecraft"]
	loader, loaderVersion, _ := idx.packLoader()
	if mc != pack.GameVersion {
		pack.GameVersion = mc
		p.setVariableIfPresent("version", mc)
		p.setVariableIfPresent("javaversion", javaForMinecraft(mc))
		if err := p.Save(); err != nil {
			return err
		}
	}
	pack.Loader = loader

	state.setStep(ModpackStepLoader, 0, 0)
	return p.installLoader(ctx, loader, mc, loaderVersion)
}
