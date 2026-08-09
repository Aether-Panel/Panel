package servers

import (
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/internal/runtime/docker"
	"github.com/SkyPanel/SkyPanel/v3/internal/runtime/tty"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"path/filepath"
	"strings"
	"sync"
)

var envMapping = make(map[string]skypanel.EnvironmentFactory)

func init() {
	envMapping["host"] = tty.EnvironmentFactory{}
	envMapping["tty"] = tty.EnvironmentFactory{}
	envMapping["standard"] = tty.EnvironmentFactory{}
	envMapping["docker"] = docker.EnvironmentFactory{}
}

func CreateEnvironment(environmentType, folder string, backupFolder string, server skypanel.Server) (*skypanel.Environment, error) {
	factory := envMapping[environmentType]

	if factory == nil {
		return nil, fmt.Errorf("undefined environment: %s", environmentType)
	}

	item := &skypanel.Environment{
		Type:            factory.Key(),
		ServerID:        server.Identifier,
		ConsoleTracker:  skypanel.CreateTracker(),
		StatusTracker:   skypanel.CreateTracker(),
		StatsTracker:    skypanel.CreateTracker(),
		ConsoleBuffer:   skypanel.CreateCache(),
		BackupDirectory: filepath.Join(backupFolder, server.Identifier),
		Wait:            &sync.WaitGroup{},
		Server:          server,
	}
	item.Implementation = factory.Create()
	err := utils.UnmarshalTo(server.Environment.Metadata, item)
	if err != nil {
		return nil, err
	}

	err = utils.UnmarshalTo(server.Environment.Metadata, item.Implementation)
	if err != nil {
		return nil, err
	}

	if item.RootDirectory == "" {
		item.RootDirectory = filepath.Join(folder, server.Identifier)
	} else {
		absRoot, errRoot := filepath.Abs(item.RootDirectory)
		absFolder, errFolder := filepath.Abs(folder)
		if errRoot != nil || errFolder != nil {
			return nil, fmt.Errorf("invalid root directory: %s", item.RootDirectory)
		}
		rel, errRel := filepath.Rel(absFolder, absRoot)
		if errRel != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("root directory %s must be within %s", item.RootDirectory, folder)
		}
		item.RootDirectory = absRoot
	}

	item.CreateWrapper()

	return item, nil
}

func GetSupportedEnvironments() []string {
	deduper := make(map[string]bool)

	for k := range envMapping {
		deduper[k] = true
	}

	result := make([]string, len(deduper))
	i := 0
	for k := range deduper {
		result[i] = k
		i++
	}

	return result
}
