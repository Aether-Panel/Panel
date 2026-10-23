package servers

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/curseforge"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// CurseForge modpacks. Searching and listing files talk to the CurseForge API with the node's
// key; the install itself reuses the "curseforge" operation the CurseForge template runs,
// which downloads the server pack and sets up its loader.

const (
	curseForgeMinecraft = 432
	curseForgeModpacks  = 4471
)

// curseForgeAPI is a variable so tests can use a local server
var curseForgeAPI = "https://api.curseforge.com"

// curseForgeLoaders maps the API's modLoader numbers to loader names
var curseForgeLoaders = map[int]string{1: "forge", 4: "fabric", 5: "quilt", 6: "neoforge"}

func curseForgeHeaders() map[string]string {
	return map[string]string{"x-api-key": config.CurseForgeKey.Value()}
}

func searchCurseForge(ctx context.Context, query string, offset int) (ModpackSearchResult, error) {
	q := url.Values{}
	q.Set("gameId", strconv.Itoa(curseForgeMinecraft))
	q.Set("classId", strconv.Itoa(curseForgeModpacks))
	q.Set("sortField", "2") // popularity
	q.Set("sortOrder", "desc")
	q.Set("pageSize", strconv.Itoa(modpackPageSize))
	q.Set("index", strconv.Itoa(offset))
	if query = strings.TrimSpace(query); query != "" {
		q.Set("searchFilter", query)
	}
	var res struct {
		Data []struct {
			ID            int     `json:"id"`
			Name          string  `json:"name"`
			Slug          string  `json:"slug"`
			Summary       string  `json:"summary"`
			DownloadCount float64 `json:"downloadCount"`
			Logo          *struct {
				ThumbnailURL string `json:"thumbnailUrl"`
			} `json:"logo"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
			LatestFilesIndexes []struct {
				GameVersion string `json:"gameVersion"`
				ModLoader   *int   `json:"modLoader"`
			} `json:"latestFilesIndexes"`
			DateModified         time.Time `json:"dateModified"`
			AllowModDistribution *bool     `json:"allowModDistribution"`
		} `json:"data"`
		Pagination struct {
			Index      int `json:"index"`
			PageSize   int `json:"pageSize"`
			TotalCount int `json:"totalCount"`
		} `json:"pagination"`
	}
	if err := getModpackJSON(ctx, "CurseForge", curseForgeAPI+"/v1/mods/search?"+q.Encode(), curseForgeHeaders(), &res); err != nil {
		return ModpackSearchResult{}, err
	}
	total := res.Pagination.TotalCount
	// CurseForge does not page past 10000 results
	if total > 10000 {
		total = 10000
	}
	result := ModpackSearchResult{Hits: make([]ModpackHit, 0, len(res.Data)), Total: total, Offset: res.Pagination.Index, Limit: res.Pagination.PageSize}
	for _, mod := range res.Data {
		// packs whose authors forbid third-party downloads cannot be installed
		if mod.AllowModDistribution != nil && !*mod.AllowModDistribution {
			continue
		}
		loaders := make([]string, 0, 2)
		versions := make([]string, 0, 4)
		for _, f := range mod.LatestFilesIndexes {
			if f.ModLoader != nil {
				loaders = append(loaders, curseForgeLoaders[*f.ModLoader])
			}
			versions = appendUnique(versions, f.GameVersion)
		}
		hit := ModpackHit{
			ID:           strconv.Itoa(mod.ID),
			Slug:         mod.Slug,
			Title:        mod.Name,
			Description:  mod.Summary,
			Downloads:    int64(mod.DownloadCount),
			Loaders:      supportedLoaders(loaders),
			GameVersions: releaseVersions(versions),
			UpdatedAt:    mod.DateModified,
		}
		if mod.Logo != nil {
			hit.IconURL = mod.Logo.ThumbnailURL
		}
		if len(mod.Authors) > 0 {
			hit.Author = mod.Authors[0].Name
		}
		result.Hits = append(result.Hits, hit)
	}
	return result, nil
}

func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

type curseForgeFile struct {
	ID                  int       `json:"id"`
	ModID               int       `json:"modId"`
	DisplayName         string    `json:"displayName"`
	FileName            string    `json:"fileName"`
	ReleaseType         int       `json:"releaseType"`
	FileStatus          int       `json:"fileStatus"`
	FileDate            time.Time `json:"fileDate"`
	DownloadCount       float64   `json:"downloadCount"`
	GameVersions        []string  `json:"gameVersions"`
	IsServerPack        bool      `json:"isServerPack"`
	ServerPackFileID    *int      `json:"serverPackFileId"`
	ParentProjectFileID *int      `json:"parentProjectFileId"`
}

func (f curseForgeFile) hasServerPack() bool {
	return f.IsServerPack || (f.ServerPackFileID != nil && *f.ServerPackFileID != 0)
}

// loaders reads the loaders from the file's version tags; quilt-only files are not offered
func (f curseForgeFile) loaders() (loaders []string, unsupported bool) {
	named := make([]string, 0, 2)
	for _, v := range f.GameVersions {
		switch strings.ToLower(v) {
		case "forge", "neoforge", "fabric":
			named = append(named, strings.ToLower(v))
		case "quilt":
			unsupported = true
		}
	}
	loaders = supportedLoaders(named)
	return loaders, unsupported && len(loaders) == 0
}

var curseForgeReleaseTypes = map[int]string{1: "release", 2: "beta", 3: "alpha"}

func curseForgeVersions(ctx context.Context, project string) ([]ModpackVersion, error) {
	var res struct {
		Data []curseForgeFile `json:"data"`
	}
	if err := getModpackJSON(ctx, "CurseForge", curseForgeAPI+"/v1/mods/"+project+"/files?pageSize=50", curseForgeHeaders(), &res); err != nil {
		return nil, err
	}
	listed := make(map[int]bool, len(res.Data))
	for _, f := range res.Data {
		listed[f.ID] = true
	}
	versions := make([]ModpackVersion, 0, len(res.Data))
	for _, f := range res.Data {
		if !f.hasServerPack() || !curseforge.IsAllowedFile(f.FileStatus) {
			continue
		}
		// a server pack whose client file is listed too is offered once, as that file
		if f.IsServerPack && f.ParentProjectFileID != nil && listed[*f.ParentProjectFileID] {
			continue
		}
		loaders, unsupported := f.loaders()
		if unsupported {
			continue
		}
		versions = append(versions, ModpackVersion{
			ID:            strconv.Itoa(f.ID),
			Name:          firstNonEmpty(f.DisplayName, f.FileName),
			VersionNumber: firstNonEmpty(f.DisplayName, f.FileName),
			GameVersions:  releaseVersions(f.GameVersions),
			Loaders:       loaders,
			PublishedAt:   f.FileDate,
			Downloads:     int64(f.DownloadCount),
			Type:          curseForgeReleaseTypes[f.ReleaseType],
			ServerPack:    true,
		})
	}
	sort.SliceStable(versions, func(i, j int) bool { return versions[i].PublishedAt.After(versions[j].PublishedAt) })
	return versions, nil
}

func resolveCurseForgePlan(ctx context.Context, req ModpackInstallRequest) (modpackPlan, error) {
	var mod struct {
		Data struct {
			ID      int    `json:"id"`
			GameID  int    `json:"gameId"`
			ClassID int    `json:"classId"`
			Name    string `json:"name"`
			Logo    *struct {
				ThumbnailURL string `json:"thumbnailUrl"`
			} `json:"logo"`
			AllowModDistribution *bool `json:"allowModDistribution"`
		} `json:"data"`
	}
	if err := getModpackJSON(ctx, "CurseForge", curseForgeAPI+"/v1/mods/"+req.ProjectID, curseForgeHeaders(), &mod); err != nil {
		return modpackPlan{}, err
	}
	if mod.Data.GameID != curseForgeMinecraft || mod.Data.ClassID != curseForgeModpacks {
		return modpackPlan{}, fmt.Errorf("%w: that project is not a Minecraft modpack", ErrModpackBadRequest)
	}
	if mod.Data.AllowModDistribution != nil && !*mod.Data.AllowModDistribution {
		return modpackPlan{}, fmt.Errorf("%w: the author of this pack does not allow installs outside the CurseForge app", ErrModpackBadRequest)
	}
	var file struct {
		Data curseForgeFile `json:"data"`
	}
	if err := getModpackJSON(ctx, "CurseForge", curseForgeAPI+"/v1/mods/"+req.ProjectID+"/files/"+req.VersionID, curseForgeHeaders(), &file); err != nil {
		return modpackPlan{}, err
	}
	if strconv.Itoa(file.Data.ModID) != req.ProjectID {
		return modpackPlan{}, fmt.Errorf("%w: that file belongs to another pack", ErrModpackBadRequest)
	}
	if !file.Data.hasServerPack() {
		return modpackPlan{}, fmt.Errorf("%w: that version has no server pack", ErrModpackBadRequest)
	}
	loaders, unsupported := file.Data.loaders()
	if unsupported {
		return modpackPlan{}, ErrModpackUnsupported
	}
	pack := InstalledModpack{
		Source:      ModpackSourceCurseForge,
		ProjectID:   req.ProjectID,
		VersionID:   req.VersionID,
		Title:       mod.Data.Name,
		VersionName: firstNonEmpty(file.Data.DisplayName, file.Data.FileName),
	}
	if mod.Data.Logo != nil {
		pack.IconURL = mod.Data.Logo.ThumbnailURL
	}
	if len(loaders) > 0 {
		pack.Loader = loaders[0]
	}
	if versions := releaseVersions(file.Data.GameVersions); len(versions) > 0 {
		pack.GameVersion = versions[0]
	}
	return modpackPlan{request: req, pack: pack, fileID: req.VersionID}, nil
}

// installCurseForgePack runs the "curseforge" operation, like the CurseForge template does
func (p *Server) installCurseForgePack(plan modpackPlan) error {
	env := p.RunningEnvironment
	args := map[string]interface{}{"projectId": plan.pack.ProjectID, "fileID": plan.fileID}
	ops := make([]skypanel.ConditionalMetadataType, 0, 2)
	if env.Type == "host" {
		javaBinary, err := p.javaSetup()
		if err != nil {
			return err
		}
		args["java"] = javaBinary
	}
	ops = append(ops, skypanel.ConditionalMetadataType{MetadataType: skypanel.MetadataType{Type: "curseforge", Metadata: args}})
	process, err := GenerateProcess(ops, env, p.DataToMap(), p.Execution.EnvironmentVariables)
	if err != nil {
		return err
	}
	if err := process.Run(p); err != nil {
		return upstream("CurseForge", err)
	}
	return nil
}
