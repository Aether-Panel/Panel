package curseforge

import (
	"context"
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/forgedl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/neoforgedl"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type CurseForge struct {
	ProjectID  uint
	FileID     uint
	JavaBinary string
}

// new plan
// download curseforge to cache if not present (curseforge/projectid/fileid)
// extract from cache to server directory
// determine modloader to use (do this via 3 ways)
// - is there a forge installer present
// - is there a variables.txt present
// - what does the client manifest.json indicate
// install desired mod loader
// - forge
// -   download installer to cache if not present
// -   run installer into cache dir
// -   copy directory to server
// - fabric
// -   check if either installer is in the cache
// -   download improved launcher to the cache
// -   download "old" installer to the cache
// -   run installer in cache
// -   copy directory to server
// - neoforgedl
// -   same as forge, but screw downloading
// - quilt
// -   will not deal with

var installerRegex = regexp.MustCompile("(neo)?forge-.*-installer.jar")
var errNoFile = errors.New("status code 404")

func (c CurseForge) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	var clientFile, serverFile File
	var err error
	if c.FileID == 0 {
		// we need to get the latest file id to do our calls
		files, err := getLatestFiles(c.ProjectID)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}

		for _, v := range files {
			if !IsAllowedFile(v.FileStatus) {
				continue
			}
			if v.ReleaseType != ReleaseFileType {
				continue
			}
			if serverFile.FileDate.Before(v.FileDate) {
				serverFile = v
				continue
			}
		}

		if serverFile.ID == 0 {
			err = errors.New("no files available on CurseForge")
			return skypanel.OperationResult{Error: err}
		}
	} else {
		serverFile, err = getFileByID(c.ProjectID, c.FileID)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
	}

	if !serverFile.IsServerPack && serverFile.ServerPackFileID != 0 {
		clientFile = serverFile
		serverFile, err = getFileByID(c.ProjectID, serverFile.ServerPackFileID)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
	}

	if clientFile.ID == 0 && serverFile.ParentProjectFileID != 0 {
		clientFile, err = getFileByID(c.ProjectID, serverFile.ParentProjectFileID)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
	}

	if !serverFile.IsServerPack {
		logging.Debug.Printf("File ID %d is not marked as a server pack, will not install\n", serverFile.ID)
		env.DisplayToConsole(true, "File ID %d is not marked as a server pack, will not install\n", serverFile.ID)
		return skypanel.OperationResult{Error: errors.New("not server pack")}
	}

	logging.Debug.Printf("Downloading modpack from %s\n", serverFile.DownloadURL)
	env.DisplayToConsole(true, "Downloading modpack from %s", serverFile.DownloadURL)
	err = downloadModpack(serverFile)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	/*if clientFile.ID != 0 {
		logging.Debug.Printf("Downloading modpack from %s\n", serverFile.DownloadURL)
		env.DisplayToConsole(true, "Downloading modpack from %s", serverFile.DownloadURL)
		err = downloadModpack(clientFile)
		if err != nil {
			return skypanel.OperationResult{Error: err}
		}
	}*/

	serverZipPath := getCacheFilePath(serverFile)
	logging.Debug.Printf("Extracting modpack from %s\n", serverZipPath)
	env.DisplayToConsole(true, "Extracting modpack from %s", serverZipPath)

	f, err := os.Open(serverZipPath)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	singleRoot, _ := files.DetermineIfSingleRoot(context.Background(), serverZipPath, f)
	_, _ = f.Seek(0, io.SeekStart)

	err = files.ExtractFromReader(f, serverZipPath, env.GetRootDirectory(), "*", singleRoot, nil)
	_ = f.Close()
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	// modpack now downloaded and extracted
	// worse case, this is all we could do...
	// best case, we can get the modpack set up how we need it

	// set 1: resolve the pack to a "modloader"
	var modLoader string
	var data = make(map[string]string)
	var jar string
	var vars map[string]string
	var manifest Manifest
	if jar, err = findInstallerJar(env); err == nil {
		logging.Debug.Printf("Found jar: %s\n", jar)
		if strings.HasPrefix(jar, "neoforgedl") {
			modLoader = "neoforgedl"
		} else {
			modLoader = "forge"
		}
		data["jar"] = jar
	} else if vars, err = readVariableFile(serverFile); err == nil {
		logging.Debug.Printf("Reading variables.txt\n")
		modLoader = strings.ToLower(vars["MODLOADER"])
		data["mcVersion"] = vars["MINECRAFT_VERSION"]
		data["version"] = vars["MODLOADER_VERSION"]
		data["installerVersion"] = vars["FABRIC_INSTALLER_VERSION"]
		logging.Debug.Printf("Resolved: %v\n", data)
	} else if manifest, err = getManifest(clientFile); err == nil {
		logging.Debug.Printf("Using manifest: %v\n", manifest.Minecraft)
		mcVersion := manifest.Minecraft.Version
		var loaderVersion string
		for _, v := range manifest.Minecraft.ModLoaders {
			if v.Primary {
				loaderVersion = v.ID
				break
			}
		}
		parts := strings.SplitN(loaderVersion, "-", 2)
		modLoader = parts[0]
		data["mcVersion"] = mcVersion
		data["version"] = parts[1]
		logging.Debug.Printf("Resolved: %v\n", data)
	} else {
		// give up
		env.DisplayToConsole(true, "Unknown server type. Could not prepare server for actual execution")
		return skypanel.OperationResult{Error: nil}
	}

	// we figured out the loader, now to run their "installer"
	switch modLoader {
	case "fabric":
		{
			err = installFabric(env, data, c.JavaBinary)
			if err != nil {
				return skypanel.OperationResult{Error: err}
			}
		}
	case "forge":
		fallthrough
	case "neoforge":
		{
			jarFile := data["jar"]
			if jarFile == "" {
				var downloadURL string
				var installerJar = "installer.jar"
				version := data["version"]

				if modLoader == "neoforge" {
					downloadURL = replaceTokens(neoforgedl.InstallerURL, map[string]string{"version": version})
				} else {
					// because forge has the version in the url, handle it
					mcVersion := data["mcVersion"]
					if !strings.HasPrefix(version, mcVersion) {
						version = mcVersion + "-" + version
					}
					downloadURL = replaceTokens(forgedl.InstallerURL, map[string]string{"version": version})
				}

				dl, err := skypanel.DownloadViaMaven(downloadURL, env)
				defer utils.Close(dl)
				if err != nil {
					return skypanel.OperationResult{Error: err}
				}
				// copy to server
				err = files.WriteFile(dl, filepath.Join(env.GetRootDirectory(), installerJar))
				if err != nil {
					return skypanel.OperationResult{Error: err}
				}
				jarFile = installerJar
			}

			err = installViaJar(args.Server, env, jarFile, c.JavaBinary)
			if err != nil {
				return skypanel.OperationResult{Error: err}
			}

			// grab the ServerStarter if there isn't a server.jar, just to help out
			// would prefer Forge's variant, but this will do
			runJarFile := filepath.Join(env.GetRootDirectory(), "server.jar")
			if _, err = os.Stat(runJarFile); os.IsNotExist(err) {
				env.DisplayToConsole(true, "Grabbing ServerStarter")
				var cachePath = filepath.Join(config.CacheFolder.Value(), "github.com", "neoforgedl", "serverstarter", NeoForgeServerStarterVersion, "server.jar")
				if _, err = os.Stat(cachePath); os.IsNotExist(err) {
					env.DisplayToConsole(true, "Downloading "+NeoForgeServerStarter)
					err = skypanel.DownloadFileToCache(NeoForgeServerStarter, cachePath)
					if err != nil {
						return skypanel.OperationResult{Error: err}
					}
				}
				err = files.CopyFile(cachePath, runJarFile)
				if err != nil {
					return skypanel.OperationResult{Error: err}
				}
			}
		}
	default:
		{
			env.DisplayToConsole(true, "Unsupported server type. Could not prepare server for actual execution")
			return skypanel.OperationResult{Error: nil}
		}
	}

	// loaders installed, at this stage, we're "done"
	env.DisplayToConsole(true, "Pack installed and should be good to go!")
	return skypanel.OperationResult{Error: nil}
}

func findInstallerJar(env *skypanel.Environment) (string, error) {
	entries, err := os.ReadDir(env.GetRootDirectory())
	if err != nil {
		return "", err
	}

	for _, v := range entries {
		if v.IsDir() {
			continue
		}
		if installerRegex.MatchString(v.Name()) {
			return v.Name(), nil
		}
	}
	return "", os.ErrNotExist
}

func installViaJar(server skypanel.DaemonServer, env *skypanel.Environment, jarFile string, javaBinary string) error {
	cleanRoot := filepath.Clean(env.GetRootDirectory())
	rootPrefix := cleanRoot + string(filepath.Separator)

	// installer found, we will run this one
	result := make(chan int, 1)
	err := env.Execute(skypanel.ExecutionData{
		Command: fmt.Sprintf("%s -jar %s --installServer", javaBinary, jarFile),
		Callback: func(exitCode int) {
			result <- exitCode
			env.DisplayToConsole(true, "Installer exit code: %d", exitCode)
		},
		Variables: server.DataToMap(),
	})
	if err != nil {
		return err
	}
	if <-result != 0 {
		return errors.New("failed to run installer")
	}

	// delete installer now
	cleanJar := filepath.Base(jarFile)
	jarPath := filepath.Join(cleanRoot, cleanJar)
	cleanJarPath := filepath.Clean(jarPath)
	if strings.HasPrefix(cleanJarPath, rootPrefix) {
		err = os.Remove(cleanJarPath)
		if err != nil {
			env.DisplayToConsole(true, "Failed to delete installer")
		}
	}
	logPath := filepath.Join(cleanRoot, cleanJar+".log")
	cleanLogPath := filepath.Clean(logPath)
	if strings.HasPrefix(cleanLogPath, rootPrefix) {
		err = os.Remove(cleanLogPath)
		if err != nil {
			env.DisplayToConsole(true, "Failed to delete installer")
		}
	}

	// if this is before 1.16, we have a root jar
	// or if there's a shim
	possibleRenames := []string{
		strings.Replace(cleanJar, "-installer", "", 1),      // pre 1.17 forge
		strings.Replace(cleanJar, "-installer", "-shim", 1), // forge shim
	}

	var fi os.FileInfo
	for _, f := range possibleRenames {
		fPath := filepath.Join(cleanRoot, f)
		cleanFPath := filepath.Clean(fPath)
		serverPath := filepath.Join(cleanRoot, "server.jar")
		cleanServerPath := filepath.Clean(serverPath)
		if strings.HasPrefix(cleanFPath, rootPrefix) && strings.HasPrefix(cleanServerPath, rootPrefix) {
			if fi, err = os.Lstat(cleanFPath); err == nil && !fi.IsDir() {
				err = os.Rename(cleanFPath, cleanServerPath)
				if err != nil {
					return err
				}
			} else if fi, err = os.Lstat(cleanFPath); err == nil && !fi.IsDir() {
				err = os.Rename(cleanFPath, cleanServerPath)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func installFabric(env *skypanel.Environment, data map[string]string, javaBinary string) error {
	// this is a mess
	// there's 2 options that exist for fabric
	// there is an "improved" launcher, which is just a jar that we need
	// or we have to pull the installer and run it

	cleanRoot := filepath.Clean(env.GetRootDirectory())
	rootPrefix := cleanRoot + string(filepath.Separator)

	// see if improved is available
	fabricURL := replaceTokens(ImprovedFabricInstallerURL, data)
	targetFile := filepath.Join(cleanRoot, "server.jar")
	cleanTarget := filepath.Clean(targetFile)

	env.DisplayToConsole(true, "Downloading %s to %s", fabricURL, cleanTarget)
	err := downloadFile(fabricURL, cleanTarget)
	if err == nil {
		// this was a good file, we got what we need
		return nil
	}

	if !errors.Is(err, errNoFile) {
		// we got a 404, so we can't use the improved version at all
		fabricURL = replaceTokens(FabricInstallerURL, data)
		targetFile = filepath.Join(cleanRoot, "fabric-installer.jar")
		cleanTarget = filepath.Clean(targetFile)

		env.DisplayToConsole(true, "Downloading %s to %s", fabricURL, cleanTarget)
		err = downloadFile(fabricURL, cleanTarget)
		if err != nil {
			return err
		}

		// forge installer found, we will run this one
		result := make(chan int, 1)
		err = env.Execute(skypanel.ExecutionData{
			Command: fmt.Sprintf("%s -jar fabric-installer server -mcversion %s -loader %s -downloadMinecraft", javaBinary, data["mcVersion"], data["version"]),
			Callback: func(exitCode int) {
				result <- exitCode
				env.DisplayToConsole(true, "Installer exit code: %d", exitCode)
			},
		})
		if err != nil {
			return err
		}
		if <-result != 0 {
			return errors.New("failed to run fabric installer")
		}

		// delete installer now
		installerPath := filepath.Join(cleanRoot, "fabric-installer.jar")
		cleanInstaller := filepath.Clean(installerPath)
		if strings.HasPrefix(cleanInstaller, rootPrefix) {
			err = os.Remove(cleanInstaller)
			if err != nil {
				env.DisplayToConsole(true, "Failed to delete installer")
			}
		}

		removePath := filepath.Join(cleanRoot, "server.jar")
		removeClean := filepath.Clean(removePath)
		renameFrom := filepath.Join(cleanRoot, "fabric-server-launch.jar")
		renameFromClean := filepath.Clean(renameFrom)
		renameToClean := removeClean
		if strings.HasPrefix(removeClean, rootPrefix) && strings.HasPrefix(renameFromClean, rootPrefix) && strings.HasPrefix(renameToClean, rootPrefix) {
			_ = os.Remove(removeClean)
			err = os.Rename(renameFromClean, renameToClean)
		}
		return err
	}

	return err
}

func downloadFile(url, target string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer utils.Close(file)
	response, err := skypanel.HTTP().Get(url)
	defer utils.CloseResponse(response)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusNotFound {
		return errNoFile
	}
	_, err = io.Copy(file, response.Body)
	return err
}

func replaceTokens(msg string, data map[string]string) string {
	result := msg
	for k, v := range data {
		result = strings.ReplaceAll(result, "${"+k+"}", v)
	}
	return result
}
