package scraperdl

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/cavaliergopher/grab/v3"
)

type ScraperDl struct {
	URL            string
	Regex          string
	DownloadURL    string
	OutputVariable string
}

func (op ScraperDl) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	env.DisplayToConsole(true, "Scraping version from %s\n", op.URL)
	logging.Debug.Printf("Scraping url %s", op.URL)

	response, err := skypanel.HTTPGet(op.URL)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	defer utils.CloseResponse(response)

	if response.StatusCode != 200 {
		return skypanel.OperationResult{Error: fmt.Errorf("unexpected status code %d from scraper url", response.StatusCode)}
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	re, err := regexp.Compile(op.Regex)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	matches := re.FindStringSubmatch(string(bodyBytes))
	if len(matches) < 2 {
		return skypanel.OperationResult{Error: errors.New("could not find a match for the specified regex on the page")}
	}

	match := matches[1]
	env.DisplayToConsole(true, "Found version: %s\n", match)

	finalDownloadUrl := strings.ReplaceAll(op.DownloadURL, "${match}", match)

	env.DisplayToConsole(true, "Downloading %s\n", finalDownloadUrl)
	_, err = grab.Get(env.GetRootDirectory(), finalDownloadUrl)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}

	if op.OutputVariable != "" {
		return skypanel.OperationResult{
			VariableOverrides: map[string]interface{}{
				op.OutputVariable: match,
			},
			Error: nil,
		}
	}

	return skypanel.OperationResult{Error: nil}
}
