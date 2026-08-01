package skypanel

import (
	"crypto"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
)

func DownloadFile(url, fileName string, env *Environment) error {
	root, err := env.validatedRoot()
	if err != nil {
		return err
	}
	cleanRoot := filepath.Clean(root)
	rootPrefix := cleanRoot + string(filepath.Separator)
	targetPath := filepath.Join(cleanRoot, fileName)
	cleanTarget := filepath.Clean(targetPath)
	if !strings.HasPrefix(cleanTarget, rootPrefix) {
		return fmt.Errorf("invalid file path: %s", fileName)
	}
	target, err := os.Create(cleanTarget)
	defer utils.Close(target)
	if err != nil {
		return err
	}

	env.DisplayToConsole(true, "Downloading: "+url+"\n")

	response, err := HTTPGet(url)
	defer utils.CloseResponse(response)
	if err != nil {
		return err
	}

	_, err = io.Copy(target, response.Body)
	return err
}

func DownloadFileToCache(url, fileName string) error {
	parent := filepath.Dir(fileName)
	err := os.MkdirAll(parent, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	target, err := os.Create(fileName)
	defer utils.Close(target)
	if err != nil {
		return err
	}

	logging.Info.Printf("Downloading: %s\n", url)

	response, err := HTTPGet(url)
	defer utils.CloseResponse(response)
	if err != nil {
		return err
	}

	_, err = io.Copy(target, response.Body)
	return err
}

func downloadFile(url string) (io.ReadCloser, error) {
	logging.Info.Printf("Downloading: %s", url)
	response, err := HTTPGet(url)
	if err != nil {
		return nil, err
	}
	return response.Body, err
}

func cacheFile(downloadURL, localPath string) (io.ReadCloser, error) {
	dl, err := downloadFile(downloadURL)
	if err != nil {
		utils.Close(dl)
		return nil, err
	}
	parent := filepath.Dir(localPath)
	err = os.MkdirAll(parent, 0755)
	if err != nil && !os.IsExist(err) {
		logging.Info.Printf("Failed directories in cache: %s", err)
		return dl, nil
	}
	f, err := os.Create(localPath)
	if err != nil {
		utils.Close(f)
		logging.Info.Printf("Failed creating file in cache: %s", err)
		return dl, nil
	}
	_, err = io.Copy(f, dl)
	utils.Close(dl)
	if err != nil {
		// failed actually writing to the successfully created file
		utils.Close(f)
		return nil, err
	}
	err = f.Sync()
	if err != nil {
		return nil, err
	}
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func Download(downloadURL, hash string, algorithm crypto.Hash, cache bool, env *Environment) (io.ReadCloser, error) {
	if env != nil {
		env.DisplayToConsole(true, "Downloading: %s\n", downloadURL)
	}

	if !cache {
		// don't interact with cache, directly return download response
		return downloadFile(downloadURL)
	}
	// caching allowed
	localPath := filepath.Join(config.CacheFolder.Value(), strings.TrimPrefix(strings.TrimPrefix(downloadURL, "http:// "), "https:// "))

	if os.PathSeparator != '/' {
		localPath = strings.ReplaceAll(localPath, "/", string(os.PathSeparator))
	}

	// try to open existing cached file
	f, err := os.Open(localPath)
	if os.IsNotExist(err) {
		// cache miss, need to download
		return cacheFile(downloadURL, localPath)
	} else if err != nil {
		logging.Info.Printf("Failed opening cached file despite it existing: %s", err)
		return downloadFile(downloadURL)
	}

	h := algorithm.New()
	if _, err := io.Copy(h, f); err != nil {
		utils.Close(f)
		logging.Info.Printf("Cached file is not readable, will download (%s)", localPath)
		return downloadFile(downloadURL)
	}
	actualHash := fmt.Sprintf("%x", h.Sum(nil))
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	if hash == actualHash {
		logging.Info.Printf("Using cached copy of file: %s\n", downloadURL)
		return f, nil
	}

	logging.Info.Printf("Cache expected %s but was actually %s, downloading new version and caching to %s", hash, actualHash, localPath)
	utils.Close(f)
	return cacheFile(downloadURL, localPath)
}

func DownloadHash(hashURL string, algorithm crypto.Hash) (string, error) {
	logging.Info.Printf("Downloading hash from %s", hashURL)
	response, err := HTTPGet(hashURL)
	defer utils.CloseResponse(response)
	if err != nil {
		return "", err
	}

	data := make([]byte, algorithm.Size()*2)
	_, err = response.Body.Read(data)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func DownloadViaMaven(downloadURL string, env *Environment) (io.ReadCloser, error) {
	hashURL := downloadURL + ".sha1"
	expectedHash, err := DownloadHash(hashURL, crypto.SHA1)
	if err != nil {
		logging.Info.Printf("Failed downloading hash, not using cache")
		return Download(downloadURL, "", crypto.SHA1, false, env)
	}

	return Download(downloadURL, expectedHash, crypto.SHA1, true, env)
}
