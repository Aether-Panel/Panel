package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	_ = config.ConsoleForward.Set(true, false)
	_ = config.SecurityDisableUnshare.Set(true, false)

	root, err := os.MkdirTemp("", "reproinstall")
	if err != nil {
		return fmt.Errorf("mkdir temp: %w", err)
	}
	defer os.RemoveAll(root)

	workDir := filepath.Join(root, "srv")
	_ = os.MkdirAll(workDir, 0755)

	_ = config.ServersFolder.Set(filepath.Join(root, "servers"), false)
	_ = config.BinariesFolder.Set(filepath.Join(root, "binaries"), false)
	_ = config.CacheFolder.Set(filepath.Join(root, "cache"), false)

	logging.OriginalStdOut = os.Stdout

	serverID := "mc-repro"
	server := servers.CreateProgram()
	server.Identifier = serverID
	server.Type = skypanel.Type{Type: "minecraft-java"}

	serverFolder := filepath.Join(workDir, serverID)
	if err := os.MkdirAll(serverFolder, 0755); err != nil {
		return fmt.Errorf("mkdir error: %w", err)
	}

	// Paper 1.20.1 install (javaversion blank -> javadl skipped in host env)
	server.Variables = map[string]skypanel.Variable{
		"eula":        {Value: false},
		"modlauncher": {Value: "paper"},
		"version":     {Value: "1.20.1"},
		"paperbuild":  {Value: "latest"},
		"javaversion": {Value: ""},
		"ip":          {Value: "0.0.0.0"},
		"port":        {Value: 25565},
		"memory":      {Value: 512},
		"cpu":         {Value: 100},
		"disk":        {Value: 2048},
		"motd":        {Value: "test"},
		"jvmArgs":     {Value: ""},
	}

	inst := []skypanel.ConditionalMetadataType{
		{
			If:           "javaversion != '' && env == 'host'",
			MetadataType: skypanel.MetadataType{Type: "javadl", Metadata: map[string]interface{}{"version": "${javaversion}"}},
		},
		{
			If: "modlauncher == 'paper'",
			MetadataType: skypanel.MetadataType{Type: "paperdl", Metadata: map[string]interface{}{
				"build": "${paperbuild}", "minecraftVersion": "${version}", "target": "server.jar",
			}},
		},
		{
			If: "!file_exists(\"server.properties\")",
			MetadataType: skypanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{
				"text": "server-ip=${ip}\nserver-port=${port}\nmotd=${motd}\n", "target": "server.properties",
			}},
		},
		{
			MetadataType: skypanel.MetadataType{Type: "writefile", Metadata: map[string]interface{}{"text": "eula=${eula}", "target": "eula.txt"}},
		},
	}
	server.Installation = inst
	server.Environment = skypanel.MetadataType{Type: "host"}
	server.Execution.Command = []skypanel.Command{{Command: "java -Xmx${memory}M -jar server.jar nogui"}}

	env, err := servers.CreateEnvironment("host", workDir, "", server.Server)
	if err != nil {
		return fmt.Errorf("CreateEnvironment error: %w", err)
	}
	server.RunningEnvironment = env

	fs, err := files.NewFileServer(serverFolder, 0, 0)
	if err != nil {
		return fmt.Errorf("NewFileServer error: %w", err)
	}
	server.SetFileServer(fs)

	fmt.Println("Installing Paper 1.20.1 ...")
	if err := server.Install(); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	rootDir := env.GetRootDirectory()
	_, statErr := os.Stat(filepath.Join(rootDir, "server.jar"))
	if statErr != nil {
		return fmt.Errorf("server.jar missing after install: %w", statErr)
	}
	fmt.Println("INSTALL OK: server.jar present in", rootDir)
	return nil
}
