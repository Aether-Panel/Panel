package curseforge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/SkyPanel/SkyPanel/v3/internal/shared/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

func getAddonData(projectID uint) (AddonResponse, error) {
	u := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d", projectID)

	response, err := callCurseForge(u)
	if err != nil {
		return AddonResponse{}, err
	}
	defer utils.CloseResponse(response)

	if response.StatusCode == http.StatusNotFound {
		return AddonResponse{}, nil
	}

	if response.StatusCode != http.StatusOK {
		return AddonResponse{}, skypanel.ErrCurseForgeStatus(response.Status)
	}

	d, err := io.ReadAll(response.Body)
	if err != nil {
		return AddonResponse{}, err
	}

	var addon AddonResponse
	err = json.Unmarshal(d, &addon)
	if err != nil {
		return AddonResponse{}, err
	}
	return addon, nil
}

func getAddonFileData(projectID uint, fileID uint) (FileResponse, error) {
	u := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files/%d", projectID, fileID)

	response, err := callCurseForge(u)
	if err != nil {
		return FileResponse{}, err
	}
	defer utils.CloseResponse(response)

	if response.StatusCode == http.StatusNotFound {
		return FileResponse{}, skypanel.ErrCurseForgeFile(projectID, fileID)
	}

	if response.StatusCode != http.StatusOK {
		return FileResponse{}, skypanel.ErrCurseForgeStatus(response.Status)
	}

	var res FileResponse
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil {
		return FileResponse{}, err
	}
	return res, nil
}

func getLatestFiles(projectID uint) ([]File, error) {
	addon, err := getAddonData(projectID)
	if err != nil {
		return nil, err
	}

	if !addon.Data.AllowModDistribution {
		return nil, skypanel.ErrCurseForgeDistribution(projectID)
	}

	return addon.Data.LatestFiles, err
}

func getFileByID(projectID uint, fileID uint) (File, error) {
	addon, addonErr := getAddonData(projectID)

	if addonErr != nil {
		return File{}, addonErr
	}

	if !addon.Data.AllowModDistribution {
		return File{}, skypanel.ErrCurseForgeDistribution(projectID)
	}

	file, fileErr := getAddonFileData(projectID, fileID)

	if fileErr != nil {
		return File{}, fileErr
	}

	return file.Data, nil
}

func callCurseForge(u string) (*http.Response, error) {
	path, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	request := &http.Request{
		Method: "GET",
		URL:    path,
		Header: http.Header{},
	}
	request.Header.Add("x-api-key", config.CurseForgeKey.Value())

	logging.Debug.Printf("Calling %s\n", request.URL.String())
	response, err := skypanel.HTTP().Do(request)
	return response, err
}
