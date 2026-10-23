package servers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/curseforge"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/hashicorp/go-version"
)

// Loader setup for Modrinth packs. Whatever the loader, the result must start with the
// Minecraft templates ("minecraft" and "minecraft-curseforge"), whose run command uses
// fabric-server-launch.jar if it exists, else server.jar, else Forge's unix_args.txt only when
// modlauncher is 'forge'. So every loader ends up as a server.jar:
//   - Fabric: the Fabric server launcher (meta.fabricmc.net) saved as server.jar; it fetches
//     the vanilla server on first start.
//   - Forge/NeoForge 1.17+: the official installer runs with --installServer (it writes
//     libraries/ and run.sh), then NeoForged's ServerStarterJar is placed as server.jar. It is
//     a same-process wrapper that reads run.sh and launches Forge 1.17+ and every NeoForge
//     version, so "java -Xmx.. -jar server.jar nogui" works without touching modlauncher or
//     resolvedForgeVersion (resolveneoforgeversion would not find NeoForge's folder anyway).
//   - Forge before 1.17: the installer produces forge-<version>.jar, renamed to server.jar.

var (
	fabricMetaAPI = "https://meta.fabricmc.net/v2"
	forgeMaven    = "https://maven.minecraftforge.net"
	neoforgeMaven = "https://maven.neoforged.net/releases"
	serverStarter = curseforge.NeoForgeServerStarter

	loaderVersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$`)
)

const maxLauncherSize = 64 << 20

// installerFor is the installer URL and the name it gets in the server folder
func installerFor(loader, mc, loaderVersion string) (address, jarName string, err error) {
	if !loaderVersionPattern.MatchString(loaderVersion) || !loaderVersionPattern.MatchString(mc) {
		return "", "", fmt.Errorf("the pack asks for an invalid %s version", loader)
	}
	switch loader {
	case "forge":
		full := loaderVersion
		if !strings.HasPrefix(full, mc+"-") {
			full = mc + "-" + loaderVersion
		}
		return fmt.Sprintf("%s/net/minecraftforge/forge/%s/forge-%s-installer.jar", forgeMaven, full, full), "forge-" + full + "-installer.jar", nil
	case "neoforge":
		// NeoForge for 1.20.1 was still published as "forge" under the neoforged group
		if mc == "1.20.1" {
			full := strings.TrimPrefix(loaderVersion, "1.20.1-")
			full = "1.20.1-" + full
			return fmt.Sprintf("%s/net/neoforged/forge/%s/forge-%s-installer.jar", neoforgeMaven, full, full), "forge-" + full + "-installer.jar", nil
		}
		return fmt.Sprintf("%s/net/neoforged/neoforge/%s/neoforge-%s-installer.jar", neoforgeMaven, loaderVersion, loaderVersion), "neoforge-" + loaderVersion + "-installer.jar", nil
	}
	return "", "", ErrModpackUnsupported
}

func (p *Server) installLoader(ctx context.Context, loader, mc, loaderVersion string) error {
	env := p.RunningEnvironment
	switch loader {
	case "fabric":
		env.DisplayToConsole(true, "Installing Fabric %s for Minecraft %s\n", loaderVersion, mc)
		return installFabricLauncher(ctx, p.GetFileServer(), mc, loaderVersion)
	case "forge", "neoforge":
		return p.installForgeLike(ctx, loader, mc, loaderVersion)
	}
	return ErrModpackUnsupported
}

// installFabricLauncher saves the Fabric server launcher for this game and loader as server.jar
func installFabricLauncher(ctx context.Context, fsys files.FileServer, mc, loaderVersion string) error {
	if !loaderVersionPattern.MatchString(loaderVersion) || !loaderVersionPattern.MatchString(mc) {
		return errors.New("the pack asks for an invalid Fabric version")
	}
	var installers []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	}
	if err := getModpackJSON(ctx, "Fabric", fabricMetaAPI+"/versions/installer", nil, &installers); err != nil {
		return err
	}
	installer := ""
	for _, i := range installers {
		if i.Stable && loaderVersionPattern.MatchString(i.Version) {
			installer = i.Version
			break
		}
	}
	if installer == "" {
		return upstream("Fabric", errors.New("no stable installer is listed"))
	}
	address := fmt.Sprintf("%s/versions/loader/%s/%s/%s/server/jar", fabricMetaAPI, mc, loaderVersion, installer)
	return downloadTrusted(ctx, "Fabric", address, func(r io.Reader) error {
		_, err := writeFileFS(fsys, "server.jar", r, maxLauncherSize)
		return err
	})
}

// downloadTrusted downloads from a fixed, known address (loader sites)
func downloadTrusted(ctx context.Context, site, address string, write func(io.Reader) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", skypanel.Display+" https://github.com/SkyPanel/SkyPanel")
	res, err := skypanel.HTTP().Do(req)
	defer utils.CloseResponse(res)
	if err != nil {
		return upstream(site, errors.New("could not be reached"))
	}
	if res.StatusCode != http.StatusOK {
		return upstream(site, fmt.Errorf("answered %d for %s", res.StatusCode, req.URL.Path))
	}
	return write(res.Body)
}

// javaSetup installs the right Java on host servers and says which binary to call
func (p *Server) javaSetup() (string, error) {
	env := p.RunningEnvironment
	if env.Type != "host" {
		return "java", nil
	}
	jv := p.variableString("javaversion")
	if jv == "" {
		return "java", nil
	}
	ops := []skypanel.ConditionalMetadataType{{MetadataType: skypanel.MetadataType{Type: "javadl", Metadata: map[string]interface{}{"version": jv}}}}
	process, err := GenerateProcess(ops, env, p.DataToMap(), p.Execution.EnvironmentVariables)
	if err != nil {
		return "", err
	}
	if err := process.Run(p); err != nil {
		return "", fmt.Errorf("could not install Java %s: %w", jv, err)
	}
	return "java" + jv, nil
}

func (p *Server) installForgeLike(ctx context.Context, loader, mc, loaderVersion string) error {
	env := p.RunningEnvironment
	fsys := p.GetFileServer()
	root := env.GetRootDirectory()
	name := map[string]string{"forge": "Forge", "neoforge": "NeoForge"}[loader]

	address, jarName, err := installerFor(loader, mc, loaderVersion)
	if err != nil {
		return err
	}
	env.DisplayToConsole(true, "Downloading the %s %s installer\n", name, loaderVersion)
	dl, err := skypanel.DownloadViaMaven(address, env)
	if err != nil {
		utils.Close(dl)
		return upstream(name, fmt.Errorf("the installer could not be downloaded: %w", err))
	}
	_, err = writeFileFS(fsys, jarName, dl, 256<<20)
	utils.Close(dl)
	if err != nil {
		return err
	}

	javaBinary, err := p.javaSetup()
	if err != nil {
		return err
	}
	env.DisplayToConsole(true, "Running the %s installer, this can take a few minutes\n", name)
	if err := curseforge.InstallViaJar(p, env, jarName, javaBinary); err != nil {
		return fmt.Errorf("the %s installer failed: %w", name, err)
	}

	if fileExists(root, "server.jar") {
		return nil
	}
	// very old Forge leaves a "-universal" jar
	if matches, _ := filepath.Glob(filepath.Join(root, "forge-*-universal.jar")); len(matches) > 0 {
		return fsys.Rename(filepath.Base(matches[0]), "server.jar")
	}
	if !fileExists(root, "run.sh") {
		return fmt.Errorf("the %s installer did not produce a server to start", name)
	}
	if mcVersion, err := version.NewVersion(mc); err == nil && mcVersion.LessThan(version.Must(version.NewVersion("1.17"))) {
		return fmt.Errorf("the %s installer did not produce a server jar", name)
	}
	env.DisplayToConsole(true, "Adding the server starter (server.jar)\n")
	return installServerStarter(ctx, fsys)
}

// installServerStarter copies NeoForged's ServerStarterJar, cached once per node, as server.jar
func installServerStarter(ctx context.Context, fsys files.FileServer) error {
	cachePath := filepath.Join(config.CacheFolder.Value(), "github.com", "neoforgedl", "serverstarter", curseforge.NeoForgeServerStarterVersion, "server.jar")
	if fi, err := os.Stat(cachePath); err != nil || fi.Size() == 0 {
		if err := cacheDownload(ctx, "GitHub", serverStarter, cachePath); err != nil {
			return err
		}
	}
	f, err := os.Open(cachePath)
	if err != nil {
		return err
	}
	defer utils.Close(f)
	_, err = writeFileFS(fsys, "server.jar", f, maxLauncherSize)
	return err
}

// cacheDownload saves a download in the node's cache, only once it arrived complete
func cacheDownload(ctx context.Context, site, address, cachePath string) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(cachePath), "download-*")
	if err != nil {
		return err
	}
	err = downloadTrusted(ctx, site, address, func(r io.Reader) error {
		n, err := io.Copy(tmp, io.LimitReader(r, maxLauncherSize+1))
		if err == nil && n > maxLauncherSize {
			err = errors.New("the download is larger than allowed")
		}
		return err
	})
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), cachePath)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}

func fileExists(root, name string) bool {
	fi, err := os.Lstat(filepath.Join(root, name))
	return err == nil && fi.Mode().IsRegular()
}
