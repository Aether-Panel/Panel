package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// dataJSONFile is the test definition each template folder holds
const dataJSONFile = "data.json"

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

func readDataTxtFile(fileName string) (map[string]interface{}, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer utils.Close(file)

	result := make(map[string]interface{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "=")
		key := parts[0]
		value := parts[1]
		result[key] = value
	}
	return result, nil
}

func readDataJSONFile(fileName string) ([]*TestData, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer utils.Close(file)

	result := make([]*TestData, 0)
	err = json.NewDecoder(file).Decode(&result)
	return result, err
}

func buildTests() []*TestScenario {
	templateFolder := resolveTemplateFolder()

	templateFolders, err := os.ReadDir(templateFolder)
	panicIf(err)

	testScenarios := make([]*TestScenario, 0)

	for _, folder := range templateFolders {
		if !folder.IsDir() || strings.HasPrefix(folder.Name(), ".") {
			continue
		}
		testScenarios = append(testScenarios, scanTemplateFolder(templateFolder, folder.Name())...)
	}

	return filterScenarios(testScenarios)
}

// resolveTemplateFolder uses the flag, then TEMPLATE_PATH, and otherwise clones the repo into the working dir
func resolveTemplateFolder() string {
	var templateFolder = CmdFlags.TemplateFolder
	if templateFolder != "" {
		return templateFolder
	}
	if templateFolder = os.Getenv("TEMPLATE_PATH"); templateFolder != "" {
		return templateFolder
	}

	templateFolder = filepath.Join(CmdFlags.WorkingDir, "templates")
	err := os.MkdirAll(templateFolder, 0755)
	panicIf(err)

	log.Printf("Cloning template repo")
	_, err = git.PlainClone(templateFolder, false, &git.CloneOptions{
		URL:           "https://templates.aetherpanel.es/templates.json",
		ReferenceName: plumbing.ReferenceName(CmdFlags.GitRef),
		SingleBranch:  true,
		Depth:         1,
	})
	panicIf(err)
	return templateFolder
}

// scanTemplateFolder collects the scenarios of every template file in one template folder
func scanTemplateFolder(templateFolder, folderName string) []*TestScenario {
	log.Printf("Scanning %s", folderName)

	if _, err := os.Stat(filepath.Join(templateFolder, folderName, ".skip")); err == nil {
		log.Printf("  Skipping as skip file present")
		return nil
	}

	files, err := os.ReadDir(filepath.Join(templateFolder, folderName))
	panicIf(err)

	testScenarios := make([]*TestScenario, 0)
	for _, file := range files {
		if file.Name() == dataJSONFile {
			continue
		}

		log.Printf("  Scanning %s", file.Name())

		if !isFileSelected(file.Name()) {
			log.Printf("  Skipping %s as not in files", file.Name())
			continue
		}

		if strings.HasSuffix(file.Name(), ".json") {
			testScenarios = append(testScenarios, scenariosForTemplate(templateFolder, folderName, file.Name())...)
		}
	}
	return testScenarios
}

// isFileSelected reports whether the file passes the -files filter, which allows everything when empty
func isFileSelected(fileName string) bool {
	if len(CmdFlags.Files) == 0 {
		return true
	}
	for _, t := range CmdFlags.Files {
		if t == fileName {
			return true
		}
	}
	return false
}

// scenariosForTemplate reads one template file and expands it into its test scenarios
func scenariosForTemplate(templateFolder, folderName, fileName string) []*TestScenario {
	filePath := filepath.Join(templateFolder, folderName, fileName)
	tmp := &TestTemplate{}
	tmp.Name = strings.TrimSuffix(fileName, ".json")

	var err error
	tmp.Template, err = os.ReadFile(filePath)
	panicIf(err)

	_, err = os.Stat(filepath.Join(templateFolder, folderName, "data.txt"))
	if err == nil {
		tmp.Variables, err = readDataTxtFile(filepath.Join(templateFolder, folderName, "data.txt"))
		panicIf(err)
	} else if !os.IsNotExist(err) {
		panicIf(err)
	}

	template := skypanel.Server{}
	err = json.NewDecoder(bytes.NewReader(tmp.Template)).Decode(&template)
	panicIf(err)

	_, err = os.Stat(filepath.Join(templateFolder, folderName, dataJSONFile))
	switch {
	case err == nil:
		return scenariosFromDataJSON(filepath.Join(templateFolder, folderName, dataJSONFile), tmp)
	case os.IsNotExist(err):
		// no data json, which means it's a single test
		// but, each template could support envs, so auto-process each
		return scenariosFromEnvironments(tmp, template)
	default:
		panicIf(err)
	}
	return nil
}

// scenariosFromDataJSON makes one scenario per entry in the folder's data.json
func scenariosFromDataJSON(dataFile string, tmp *TestTemplate) []*TestScenario {
	testScenarios := make([]*TestScenario, 0)
	tests, err := readDataJSONFile(dataFile)
	for _, v := range tests {
		log.Printf("  Considering %s", v.Name)
		testScenarios = append(testScenarios, &TestScenario{
			Name: v.Name,
			Test: &TestTemplate{
				Template:           tmp.Template,
				Name:               tmp.Name,
				Variables:          v.Variables,
				Environment:        v.Environment,
				IgnoreExitCode:     v.IgnoreExitCode,
				RuntimeRequirement: v.RuntimeRequirement,
			},
		})
	}
	panicIf(err)
	return testScenarios
}

// scenariosFromEnvironments makes one scenario per supported environment, or a single one when none are listed
func scenariosFromEnvironments(tmp *TestTemplate, template skypanel.Server) []*TestScenario {
	if len(template.SupportedEnvironments) == 0 {
		log.Printf("  Considering %s", tmp.Name)
		return []*TestScenario{{
			Name: tmp.Name,
			Test: tmp,
		}}
	}

	testScenarios := make([]*TestScenario, 0)
	for _, v := range template.SupportedEnvironments {
		z := &TestTemplate{
			Template:           tmp.Template,
			Name:               tmp.Name,
			Environment:        make(map[string]interface{}),
			Variables:          make(map[string]interface{}),
			RuntimeRequirement: tmp.RuntimeRequirement,
		}

		scenario := &TestScenario{
			Name: z.Name,
			Test: z,
		}
		if v.Type != "host" {
			scenario.Name = scenario.Name + "-" + v.Type
		}

		for r, p := range v.Metadata {
			scenario.Test.Environment[r] = p
		}

		scenario.Test.Environment["type"] = v.Type
		log.Printf("  Considering %s", scenario.Name)
		testScenarios = append(testScenarios, scenario)
	}
	return testScenarios
}

// filterScenarios drops scenarios matching a skip rule unless a required rule also matches
func filterScenarios(testScenarios []*TestScenario) []*TestScenario {
	finalScenarioList := make([]*TestScenario, 0)
	for _, scenario := range testScenarios {
		if isScenarioSkipped(scenario.Name) {
			log.Printf("Skipping %s", scenario.Name)
			continue
		}
		log.Printf("Will run test for: %s", scenario.Name)
		finalScenarioList = append(finalScenarioList, scenario)
	}

	return finalScenarioList
}

func isScenarioSkipped(name string) bool {
	if !matchesAnyWildcard(name, CmdFlags.Skip) {
		return false
	}
	return !matchesAnyWildcard(name, CmdFlags.Required)
}

func matchesAnyWildcard(name string, patterns []string) bool {
	for _, v := range patterns {
		if utils.CompareWildcard(name, v) {
			return true
		}
	}
	return false
}

func ioCopy(dest io.Writer, src io.Reader) {
	var buf = make([]byte, 1024)
	var readErr error
	var n int

	for readErr != io.EOF {
		n, readErr = src.Read(buf)
		_, _ = dest.Write(buf[:n])
	}
}

func createCreateBody(scenario *TestScenario) io.ReadCloser {
	model := &models.ServerCreation{
		Server: skypanel.Server{},
		NodeID: 0,
		Name:   scenario.Name,
	}

	_ = json.NewDecoder(bytes.NewReader(scenario.Test.Template)).Decode(&model.Server)

	for k, v := range scenario.Test.Variables {
		val, ok := model.Variables[k]
		if ok {
			val.Value = v
			model.Variables[k] = val
		}
	}

	envType := scenario.Test.Environment["type"].(string)

	for _, v := range model.SupportedEnvironments {
		if v.Type == envType {
			model.Environment = v
		}
	}

	for k, v := range scenario.Test.Environment {
		model.Environment.Metadata[k] = v
	}

	buf := &bytes.Buffer{}
	_ = json.NewEncoder(buf).Encode(model)
	return io.NopCloser(buf)
}
