package servers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func validateServerID(id string) (string, error) {
	if !filepath.IsLocal(id) {
		return "", fmt.Errorf("invalid server id: %s", id)
	}
	return id, nil
}

var (
	allServers     = make([]*Server, 0)
	allServersLock sync.RWMutex
)

func LoadFromFolder() {
	err := os.Mkdir(config.ServersFolder.Value(), 0755)
	if err != nil && !os.IsExist(err) {
		logging.Error.Fatalf("Error creating server data folder: %s", err)
	}
	programFiles, err := os.ReadDir(config.ServersFolder.Value())
	if err != nil {
		logging.Error.Fatalf("Error reading from server data folder: %s", err)
	}
	var program *Server
	for _, element := range programFiles {
		if element.IsDir() || !strings.HasSuffix(element.Name(), ".json") {
			continue
		}
		logging.Info.Printf("Attempting to load %s\n", element.Name())
		id := strings.TrimSuffix(element.Name(), filepath.Ext(element.Name()))
		program, err = Load(id)
		if err != nil {
			logging.Error.Printf("Error loading server details from json (%s): %s", element.Name(), err)
			continue
		}

		logging.Info.Printf("Loaded server %s", program.ID())
		addToCache(program)
	}
}

// GetAll returns a snapshot of the loaded servers, safe to range over while servers are
// created or deleted
func GetAll() []*Server {
	allServersLock.RLock()
	defer allServersLock.RUnlock()
	return append([]*Server(nil), allServers...)
}

func addToCache(server *Server) {
	allServersLock.Lock()
	defer allServersLock.Unlock()
	allServers = append(allServers, server)
}

// removeFromCache drops the server by id; looking it up again matters because the list
// may have changed while the server was being stopped
func removeFromCache(id string) {
	allServersLock.Lock()
	defer allServersLock.Unlock()
	for i, element := range allServers {
		if element.ID() == id {
			allServers = append(allServers[:i], allServers[i+1:]...)
			return
		}
	}
}

func Load(id string) (program *Server, err error) {
	safeID, err := validateServerID(id)
	if err != nil {
		return
	}
	var data []byte
	data, err = os.ReadFile(filepath.Join(config.ServersFolder.Value(), safeID+".json"))
	if err != nil {
		return
	}
	// an empty file used to return no server and no error, and the boot loop then crashed on it
	if len(data) == 0 {
		err = errors.New("server definition is empty")
		return
	}
	program, err = LoadFromData(id, data)
	return
}

func LoadFromData(id string, source []byte) (*Server, error) {
	data := CreateProgram()

	// HACK: Because golang thinks environment and Environment in the json are the same, we have to manually clean the
	// invalid record up....
	rawMap := make(map[string]interface{})
	err := json.Unmarshal(source, &rawMap)
	if err != nil {
		return nil, err
	}

	delete(rawMap, "Environment")
	source, err = json.Marshal(rawMap)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(source, &data)
	if err != nil {
		return nil, err
	}

	data.Identifier = id

	environmentType := data.Environment.Type

	if environmentType == "standard" || environmentType == "tty" {
		data.Environment.Type = "host"

		err = data.Save()
		if err != nil {
			return nil, err
		}
	}

	data.RunningEnvironment, err = CreateEnvironment(environmentType, config.ServersFolder.Value(), config.BackupsFolder.Value(), data.Server)
	if err != nil {
		return nil, err
	}

	data.Scheduler, err = LoadScheduler(data.ID())
	if err != nil {
		logging.Error.Printf("[%s] Could not read the scheduled tasks, starting with none: %s", data.ID(), err)
	}
	if data.Scheduler == nil {
		data.Scheduler = NewDefaultScheduler(data.ID())
		err = data.Scheduler.Init()
		if err != nil {
			return nil, err
		}
	}

	// the stats read the folder size the disk limit already measures
	data.RunningEnvironment.DiskUsage = data.DiskUsage

	fs, err := files.NewFileServer(data.RunningEnvironment.GetRootDirectory(), data.RunningEnvironment.GetUID(), data.RunningEnvironment.GetGid())
	if err != nil {
		return nil, err
	}
	data.SetFileServer(fs)

	data.Scheduler.Start()

	return data, nil
}

func Create(program *Server) (server *Server, err error) {
	if GetFromCache(program.ID()) != nil {
		return nil, skypanel.ErrServerAlreadyExists
	}

	serverID, err := validateServerID(program.ID())
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			// revert since we have an error
			_ = os.Remove(filepath.Join(config.ServersFolder.Value(), serverID))
			_ = os.Remove(filepath.Join(config.ServersFolder.Value(), serverID+".json"))
			if program.RunningEnvironment != nil {
				_ = program.RunningEnvironment.Delete()
			}
			server = nil
		}
	}()

	err = os.Mkdir(filepath.Join(config.ServersFolder.Value(), serverID), 0755)
	if err != nil {
		logging.Error.Printf("Error writing server: %s", err)
		return
	}

	err = program.Save()
	if err != nil {
		logging.Error.Printf("Error writing server: %s", err)
		return
	}

	server, err = Load(program.ID())
	if err != nil {
		return
	}

	// the folder belongs to the user the game runs as; otherwise it stays root's until the
	// server first starts, and a copy or a restored backup lands in a folder the game can't write
	if env := server.RunningEnvironment; env.GetUID() > 0 && os.Getuid() == 0 {
		if chownErr := os.Chown(env.GetRootDirectory(), env.GetUID(), env.GetGid()); chownErr != nil && !os.IsNotExist(chownErr) {
			logging.Error.Printf("[%s] Could not give the server folder to its user: %s", serverID, chownErr)
		}
	}

	err = server.GetEnvironment().Create()
	if err != nil {
		return
	}

	addToCache(server)
	return
}

func Delete(id string) (err error) {
	program := GetFromCache(id)
	if program == nil {
		return
	}
	safeID, err := validateServerID(id)
	if err != nil {
		return
	}
	running, err := program.IsRunning()

	if err != nil {
		return
	}

	if running {
		err = program.Kill()
		if err != nil {
			return
		}

		err = program.RunningEnvironment.WaitForMainProcess()
		if err != nil {
			return
		}
	}

	program.Scheduler.Stop()
	_ = program.GetFileServer().Close()

	err = program.Destroy()
	if err != nil {
		return
	}
	err = os.Remove(filepath.Join(config.ServersFolder.Value(), safeID+".json"))
	if err != nil {
		logging.Error.Printf("Error removing server: %s", err)
	}
	// and so does the folder a Docker server used as /tmp
	if tmp := program.RunningEnvironment.GetTmpDirectory(); tmp != "" {
		if err := os.RemoveAll(tmp); err != nil {
			logging.Error.Printf("Error removing the temporary folder of server %s: %s", safeID, err)
		}
	}
	// the server's tasks, crash reports and notification settings go with it
	forgetMetrics(safeID)
	for _, ext := range []string{".cron", ".crashes", ".notify", ".metrics", ".worldmap"} {
		if err := os.Remove(filepath.Join(config.ServersFolder.Value(), safeID+ext)); err != nil && !os.IsNotExist(err) {
			logging.Error.Printf("Error removing %s for server %s: %s", ext, safeID, err)
		}
	}
	ForgetModpack(safeID)
	// the saved versions of its files are a folder, not a single file
	if err := DeleteFileHistory(safeID); err != nil {
		logging.Error.Printf("Error removing the file history of server %s: %s", safeID, err)
	}
	removeFromCache(id)
	return
}

func GetFromCache(id string) *Server {
	allServersLock.RLock()
	defer allServersLock.RUnlock()
	for _, element := range allServers {
		if element != nil && element.ID() == id {
			return element
		}
	}
	return nil
}

func Save(id string) (err error) {
	program := GetFromCache(id)
	if program == nil {
		err = errors.New("no server with given id")
		return
	}
	err = program.Save()
	return
}

func Reload(id string) (err error) {
	program := GetFromCache(id)
	if program == nil {
		err = errors.New("server does not exist")
		return
	}

	// the new environment is not attached to a running process: reloading a running server
	// would leave it without console, stop or crash handling until the daemon restarts
	if err = program.IsIdle(); err != nil {
		return
	}

	logging.Info.Printf("Reloading server %s", program.ID())
	newVersion, err := Load(id)
	if err != nil {
		logging.Error.Printf("Error reloading server: %s", err)
		return
	}

	program.RunningEnvironment = newVersion.RunningEnvironment
	program.RunningEnvironment.DiskUsage = program.DiskUsage
	program.Server = newVersion.Server

	// Load opened the server folder again and started a scheduler of its own. The server
	// takes that scheduler (stopping its old one) and keeps its folder handle, so neither is
	// left running behind: every leaked scheduler fired each task once more
	program.Scheduler.Stop()
	program.Scheduler = newVersion.Scheduler
	if newVersion.fileServer != nil {
		_ = newVersion.fileServer.Close()
	}

	return
}
