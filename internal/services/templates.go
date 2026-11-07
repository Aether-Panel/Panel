package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"gorm.io/gorm"
)

var existingPaths = make(map[uint]repoCache)
var pathLock sync.Mutex

type Template struct {
	DB *gorm.DB
}

type repoCache struct {
	LastRefreshed time.Time
	Path          string
}

var localRepo = &models.TemplateRepo{
	ID:      0,
	Name:    "Local",
	IsLocal: true,
}

func (*Template) GetLocalRepoID() uint {
	return localRepo.ID
}

func (t *Template) GetRepos() ([]*models.TemplateRepo, error) {
	// Si hay templates.url configurado, ignoramos los repos de la BD y devolvemos uno sintético + Local
	if u := config.TemplatesURL.Value(); u != "" {
		vps := &models.TemplateRepo{
			ID:     1,
			Name:   "community",
			URL:    u,
			Branch: "vps",
		}
		return []*models.TemplateRepo{vps, localRepo}, nil
	}
	// Comportamiento original: repos desde DB + Local
	var repos []*models.TemplateRepo
	err := t.DB.Find(&repos).Error
	return append(repos, localRepo), err
}

func (t *Template) GetAllFromRepo(repoID uint) ([]*models.Template, error) {
	if repoID == localRepo.ID {
		return t.getAllLocal()
	}

	// Si hay URL de VPS configurada, resolvemos contra índice JSON en vez de Git
	if u := config.TemplatesURL.Value(); u != "" {
		return t.getAllFromVps(u)
	}
	repoDb := &models.TemplateRepo{
		ID: repoID,
	}

	err := t.DB.First(repoDb).Error
	if err != nil {
		return nil, err
	}

	path, err := validateRepoOnDisk(repoDb)
	if err != nil {
		return nil, err
	}

	return listTemplatesOnDisk(path)
}

func (t *Template) getAllLocal() ([]*models.Template, error) {
	var templates []*models.Template
	err := t.DB.Find(&templates).Error
	if err != nil {
		return nil, err
	}

	// because we don't want to return a ton of data, we'll only return a few select fields
	replacement := make([]*models.Template, len(templates))

	for k, v := range templates {
		replacement[k] = summarizeTemplate(v.Name, v)
	}

	return replacement, nil
}

// summarizeTemplate keeps only the fields needed to list templates
func summarizeTemplate(name string, v *models.Template) *models.Template {
	return &models.Template{
		Name: name,
		Server: skypanel.Server{
			Display:               v.Server.Display,
			Icon:                  v.Server.Icon,
			Type:                  v.Server.Type,
			Environment:           v.Server.Environment,
			SupportedEnvironments: v.Server.SupportedEnvironments,
			Requirements:          v.Server.Requirements,
		},
	}
}

// listTemplatesOnDisk reads every template json from the folders of a checked out repo
func listTemplatesOnDisk(path string) ([]*models.Template, error) {
	var templates []*models.Template

	folders, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, folder := range folders {
		if !folder.IsDir() || strings.HasPrefix(folder.Name(), ".") {
			continue
		}

		found, err := listTemplatesInFolder(filepath.Join(path, folder.Name()))
		if err != nil {
			return nil, err
		}
		templates = append(templates, found...)
	}

	return templates, nil
}

// listTemplatesInFolder skips data.json and any template that fails to parse
func listTemplatesInFolder(folderPath string) ([]*models.Template, error) {
	jsonFolder, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}

	var templates []*models.Template
	for _, v := range jsonFolder {
		if !strings.HasSuffix(v.Name(), ".json") {
			continue
		}

		if v.Name() == "data.json" {
			continue
		}

		name := strings.TrimSuffix(v.Name(), ".json")
		templatePath := filepath.Join(folderPath, v.Name())
		template, err := readTemplateFromDisk(name, templatePath)
		if err != nil {
			logging.Error.Printf("Error reading template from %s: %s", templatePath, err.Error())
			continue
		}

		templates = append(templates, summarizeTemplate(name, template))
	}
	return templates, nil
}

func (t *Template) Get(repoID uint, name string) (*models.Template, error) {
	template := &models.Template{
		Name: name,
	}
	if repoID == localRepo.ID {
		err := t.DB.First(template).Error
		if err != nil {
			return nil, err
		}
		return template, nil
	}

	// Si hay URL de VPS configurada, resolvemos contra índice JSON en vez de Git
	if u := config.TemplatesURL.Value(); u != "" {
		return t.getFromVps(u, name)
	}
	repoDb := &models.TemplateRepo{
		ID: repoID,
	}

	err := t.DB.First(repoDb).Error
	if err != nil {
		return nil, err
	}

	path, err := validateRepoOnDisk(repoDb)
	if err != nil {
		return nil, err
	}

	folderName, exists, err := findTemplateFolder(path, name)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, skypanel.ErrNoTemplate(name)
	}

	templatePath := filepath.Join(path, folderName, name+".json")
	template, err = readTemplateFromDisk(name, templatePath)
	if err != nil {
		return nil, err
	}

	readmePath := filepath.Join(path, folderName, "README.md")
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		logging.Error.Printf("Error reading readme %s: %s", readmePath, err.Error())
	} else {
		template.Readme = string(readme)
	}

	return template, nil
}

// findTemplateFolder scans every folder of the repo; the last folder holding the template wins
func findTemplateFolder(path, name string) (folderName string, exists bool, err error) {
	folders, err := os.ReadDir(path)
	if err != nil {
		return "", false, err
	}
	for _, folder := range folders {
		if !folder.IsDir() || strings.HasPrefix(folder.Name(), ".") {
			continue
		}

		jsonFolder, err := os.ReadDir(filepath.Join(path, folder.Name()))
		if err != nil {
			return "", false, err
		}

		for _, v := range jsonFolder {
			if v.Name() == name+".json" {
				folderName = folder.Name()
				exists = true
				break
			}
		}
	}
	return folderName, exists, nil
}

func (t *Template) Save(template *models.Template) error {
	return t.DB.Save(template).Error
}

// Delete removes a local template, unless a store product creates its servers with it and no
// other repo has a template of that name (ErrTemplateInUse)
func (t *Template) Delete(name string) error {
	products, err := t.productsLeftWithout(localRepo.ID, name)
	if err != nil {
		return err
	}
	if len(products) > 0 {
		return skypanel.ErrTemplateInUse(products)
	}

	model := &models.Template{
		Name: name,
	}

	res := t.DB.Delete(model)
	if res.Error != nil {
		return res.Error
	} else if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (t *Template) AddRepo(repo *models.TemplateRepo) error {
	existing, err := t.GetRepos()
	if err != nil {
		return err
	}
	for _, v := range existing {
		if v.Name == repo.Name {
			return skypanel.ErrRepoExists
		}
	}
	return t.DB.Save(repo).Error
}

// DeleteRepo removes the repo and its git checkout, which nothing else would ever reuse or
// remove, unless a store product finds its template only in this repo (ErrTemplateRepoInUse)
func (t *Template) DeleteRepo(id uint) error {
	// with templates.url set the panel reads none of the repos it stores: removing one changes
	// nothing a product uses
	if config.TemplatesURL.Value() == "" {
		products, err := t.productsLeftWithout(id, "")
		if err != nil {
			return err
		}
		if len(products) > 0 {
			return skypanel.ErrTemplateRepoInUse(products)
		}
	}

	if err := t.DB.Where(&models.TemplateRepo{ID: id}).Delete(&models.TemplateRepo{ID: id}).Error; err != nil {
		return err
	}
	// the repo is gone either way: a checkout that cannot be removed is only logged
	if err := forgetRepoCheckout(id); err != nil {
		logging.Error.Printf("template repo %d: its checkout could not be removed: %s", id, err)
	}
	return nil
}

// productsLeftWithout lists the ids of the store products whose template is in the repo and in
// no other the panel reads: provisioning looks a product's template up by name in every repo,
// so only those would fail once it is gone from this one. name limits it to the products of
// that template; "" takes them all. A repo that cannot be read leaves no product without its
// template, as provisioning cannot read it either.
func (t *Template) productsLeftWithout(repoID uint, name string) ([]string, error) {
	query := t.DB.Model(&models.ProvisionProduct{}).Where("template <> ''")
	if name != "" {
		query = query.Where("template = ?", name)
	}
	var products []models.ProvisionProduct
	if err := query.Order("product_id").Find(&products).Error; err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, nil
	}

	repos, err := t.GetRepos()
	if err != nil {
		return nil, err
	}
	onlyHere := make(map[string]bool)
	var left []string
	for _, product := range products {
		only, checked := onlyHere[product.Template]
		if !checked {
			only = t.hasTemplate(repoID, product.Template) && !t.anotherRepoHas(repos, repoID, product.Template)
			onlyHere[product.Template] = only
		}
		if only {
			left = append(left, product.ProductID)
		}
	}
	return left, nil
}

func (t *Template) hasTemplate(repoID uint, name string) bool {
	template, err := t.Get(repoID, name)
	return err == nil && template != nil
}

// anotherRepoHas is whether a repo other than skip has the template; one that cannot be read
// counts as not having it, so a delete is refused rather than let through
func (t *Template) anotherRepoHas(repos []*models.TemplateRepo, skip uint, name string) bool {
	for _, repo := range repos {
		if repo.ID != skip && t.hasTemplate(repo.ID, name) {
			return true
		}
	}
	return false
}

// repoCheckoutPath is the folder a repo is cloned into
func repoCheckoutPath(id uint) string {
	return filepath.Join(config.CacheFolder.Value(), "template-repos", fmt.Sprintf("%d", id))
}

// forgetRepoCheckout removes the repo's checkout from disk and from the in-memory cache
func forgetRepoCheckout(id uint) error {
	pathLock.Lock()
	defer pathLock.Unlock()
	delete(existingPaths, id)

	// only ever a folder inside template-repos, never the cache folder or anything above it
	base := filepath.Clean(filepath.Join(config.CacheFolder.Value(), "template-repos"))
	path := filepath.Clean(repoCheckoutPath(id))
	if filepath.Dir(path) != base {
		return fmt.Errorf("invalid template repo path: %s", path)
	}
	return os.RemoveAll(path)
}

func readTemplateFromDisk(name, path string) (*models.Template, error) {
	cleanPath := filepath.Clean(path)
	cacheDir := filepath.Clean(config.CacheFolder.Value())
	if !strings.HasPrefix(cleanPath, cacheDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid template path: %s", path)
	}
	file, err := os.Open(cleanPath)

	if err != nil {
		return nil, err
	}

	defer utils.Close(file)

	template := &models.Template{
		Name: name,
	}
	err = json.NewDecoder(file).Decode(template)
	return template, err
}

func validateRepoOnDisk(repo *models.TemplateRepo) (string, error) {
	// temp locations!!!
	pathLock.Lock()
	defer pathLock.Unlock()

	if cache, exists := existingPaths[repo.ID]; exists {
		if cache.LastRefreshed.Add(time.Minute * 5).After(time.Now()) {
			return cache.Path, nil
		}

		if err := pullRepo(repo, cache); err != nil {
			return "", err
		}
	} else if err := cloneRepo(repo); err != nil {
		return "", err
	}

	return existingPaths[repo.ID].Path, nil
}

// pullRepo refreshes a cached checkout; callers must hold pathLock
func pullRepo(repo *models.TemplateRepo, cache repoCache) error {
	logging.Debug.Printf("Updating local git repo for %s: %s", repo.Name, cache)

	r, err := git.PlainOpen(cache.Path)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	auth, err := repoAuth(repo)
	if err != nil {
		return err
	}

	err = w.Pull(&git.PullOptions{
		SingleBranch:  true,
		ReferenceName: plumbing.ReferenceName("refs/heads/" + repo.Branch),
		RemoteName:    "origin",
		Auth:          auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	existingPaths[repo.ID] = repoCache{
		LastRefreshed: time.Now(),
		Path:          cache.Path,
	}
	return nil
}

// cloneRepo makes a fresh checkout of the repo; callers must hold pathLock
func cloneRepo(repo *models.TemplateRepo) error {
	path := repoCheckoutPath(repo.ID)

	// if the directory already exists, we may need to nuke it
	fi, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if fi != nil {
		_ = os.RemoveAll(path)
	}

	err = os.MkdirAll(path, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	auth, err := repoAuth(repo)
	if err != nil {
		return err
	}

	logging.Debug.Printf("Checking out repo %s: %s", repo.Name, path)
	_, err = git.PlainClone(path, false, &git.CloneOptions{
		URL:           repo.URL,
		SingleBranch:  true,
		ReferenceName: plumbing.ReferenceName("refs/heads/" + repo.Branch),
		Auth:          auth,
	})
	if err != nil {
		return err
	}

	existingPaths[repo.ID] = repoCache{
		LastRefreshed: time.Now(),
		Path:          path,
	}
	return nil
}

// repoAuth is how to sign in to a private repo, nil for a public one. An SSH address uses
// the SSH key (Password unlocks it when it is encrypted; the host must be in known_hosts);
// an HTTP(S) address uses the personal access token, else the username and password. The
// credentials are never logged: errors only say which kind failed.
func repoAuth(repo *models.TemplateRepo) (transport.AuthMethod, error) {
	endpoint, err := transport.NewEndpoint(repo.URL)
	if err != nil {
		// not err itself: it quotes the address, which can hold a user:password
		return nil, fmt.Errorf("the address of template repo %s is not valid", repo.Name)
	}

	switch endpoint.Protocol {
	case "ssh":
		return sshRepoAuth(repo, endpoint.User)
	case "http", "https":
		return httpRepoAuth(repo), nil
	}
	return nil, nil
}

// sshRepoAuth signs in with the repo's SSH key: the user is Username, else the one in the
// address, else "git"
func sshRepoAuth(repo *models.TemplateRepo, addressUser string) (transport.AuthMethod, error) {
	if repo.SSHKey == "" {
		return nil, nil
	}
	user := repo.Username
	if user == "" {
		user = addressUser
	}
	if user == "" {
		user = gitssh.DefaultUsername
	}
	auth, err := gitssh.NewPublicKeys(user, []byte(repo.SSHKey), repo.Password)
	if err != nil {
		return nil, fmt.Errorf("the SSH key of template repo %s cannot be read", repo.Name)
	}
	return auth, nil
}

// httpRepoAuth signs in with the personal access token, else the username and password
func httpRepoAuth(repo *models.TemplateRepo) transport.AuthMethod {
	if repo.PAT != "" {
		// GitHub and GitLab take a token with any username; Bitbucket needs the real one
		user := repo.Username
		if user == "" {
			user = "git"
		}
		return &githttp.BasicAuth{Username: user, Password: repo.PAT}
	}
	if repo.Username != "" || repo.Password != "" {
		return &githttp.BasicAuth{Username: repo.Username, Password: repo.Password}
	}
	return nil
}

// --- Origen alternativo: índice JSON remoto (VPS) ---
type vpsIndexEntry struct {
	URL string `json:"url"`
}

func httpGetJSON(url string, target interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}

	// Crear la petición con headers apropiados para evitar bloqueos de Cloudflare
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Headers para parecer un navegador legítimo
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,es;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bad status %d from %s: %s", resp.StatusCode, url, string(b))
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(target)
}

// vpsCacheTTL is how long what templates.url served is reused. Every list request used to
// download the index and every template in it again.
const vpsCacheTTL = 5 * time.Minute

// vpsCacheEntry is the index downloaded from url and, once a list was asked for, the
// templates it lists
type vpsCacheEntry struct {
	url       string
	fetched   time.Time
	index     map[string]vpsIndexEntry
	templates []*models.Template
}

var (
	vpsCache     vpsCacheEntry
	vpsCacheLock sync.Mutex
)

// fresh reports whether the cache holds the index of indexURL and is recent enough; a
// changed templates.url is never served from the old one
func (c *vpsCacheEntry) fresh(indexURL string) bool {
	return c.index != nil && c.url == indexURL && time.Since(c.fetched) < vpsCacheTTL
}

// vpsIndex is the index at indexURL, downloaded only when the cached one is not fresh
func vpsIndex(indexURL string) (map[string]vpsIndexEntry, error) {
	vpsCacheLock.Lock()
	defer vpsCacheLock.Unlock()
	if vpsCache.fresh(indexURL) {
		return vpsCache.index, nil
	}

	var idx map[string]vpsIndexEntry
	if err := httpGetJSON(indexURL, &idx); err != nil {
		return nil, err
	}
	if idx == nil {
		idx = map[string]vpsIndexEntry{}
	}
	vpsCache = vpsCacheEntry{url: indexURL, fetched: time.Now(), index: idx}
	return idx, nil
}

func (t *Template) getAllFromVps(indexURL string) ([]*models.Template, error) {
	vpsCacheLock.Lock()
	if vpsCache.fresh(indexURL) && vpsCache.templates != nil {
		result := slices.Clone(vpsCache.templates)
		vpsCacheLock.Unlock()
		return result, nil
	}
	vpsCacheLock.Unlock()

	idx, err := vpsIndex(indexURL)
	if err != nil {
		return nil, err
	}
	result := make([]*models.Template, 0, len(idx))
	complete := true
	for name, entry := range idx {
		target := resolveURL(indexURL, entry.URL)
		tmp, err := t.getTemplateFromURL(name, target)
		if err != nil {
			logging.Error.Printf("Error reading template from %s: %s", target, err.Error())
			complete = false
			continue
		}
		result = append(result, tmp)
	}

	// a list missing a template that failed is not kept: the next request tries again
	if complete {
		vpsCacheLock.Lock()
		if vpsCache.fresh(indexURL) {
			vpsCache.templates = slices.Clone(result)
		}
		vpsCacheLock.Unlock()
	}
	return result, nil
}

func (t *Template) getFromVps(indexURL, name string) (*models.Template, error) {
	idx, err := vpsIndex(indexURL)
	if err != nil {
		return nil, err
	}
	entry, ok := idx[name]
	if !ok {
		return nil, skypanel.ErrNoTemplate(name)
	}
	return t.getTemplateFromURL(name, resolveURL(indexURL, entry.URL))
}

func (t *Template) getTemplateFromURL(name, url string) (*models.Template, error) {
	tmp := &models.Template{Name: name}
	err := httpGetJSON(url, tmp)
	if err != nil {
		return nil, err
	}
	return tmp, nil
}

func resolveURL(baseStr, refStr string) string {
	// Construye URL absoluta a partir de base (índice) y ref (posible relativa)
	base, err := url.Parse(baseStr)
	if err != nil {
		return refStr
	}
	ref, err := url.Parse(refStr)
	if err != nil {
		return refStr
	}
	return base.ResolveReference(ref).String()
}
