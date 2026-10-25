package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/curseforge"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/resolveforgeversion"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"os"
	"path/filepath"
)

var tests = []UnitTest{
	{
		CanFail: false,
		Data: curseforge.CurseForge{
			// All the Mods 9 https://www.curseforge.com/minecraft/modpacks/all-the-mods-9/files/7097957
			ProjectID: 715572,
			FileID:    7097957,
		},
	},
	{
		CanFail: false,
		Data: curseforge.CurseForge{
			// Pixelmon https://www.curseforge.com/minecraft/modpacks/the-pixelmon-modpack/files/4966924
			ProjectID: 389615,
			FileID:    4966924,
		},
	},
	{
		CanFail: true,
		Data: curseforge.CurseForge{
			// RLCraft https://www.curseforge.com/minecraft/modpacks/rlcraft/files/4612990
			ProjectID: 285109,
			FileID:    4612990,
		},
	},
	{
		CanFail: false,
		Data: curseforge.CurseForge{
			// Better MC [FABRIC] https://www.curseforge.com/minecraft/modpacks/better-mc-fabric-bmc1/files/4883129
			ProjectID: 452013,
			FileID:    4883129,
		},
	},
	{
		CanFail: false,
		Data: curseforge.CurseForge{
			ProjectID: 876781,
			FileID:    0,
		},
	},
	{
		CanFail: false,
		Data: curseforge.CurseForge{
			// MeatballCraft https://www.curseforge.com/minecraft/modpacks/meatballcraft/files/5842863
			ProjectID: 411966,
			FileID:    5842863,
		},
	},
	{
		CanFail: false,
		Data: curseforge.CurseForge{
			// Farmopolis https://www.curseforge.com/minecraft/modpacks/farmopolis/files/7112573
			ProjectID: 1270262,
			FileID:    7112573,
		},
	},
}

func main() {
	if config.CurseForgeKey.Value() == "" {
		_ = config.CurseForgeKey.Set(os.Getenv("CURSEFORGE_KEY"), false)
	}
	if config.CacheFolder.Value() == "" {
		_ = config.CacheFolder.Set(".", false)
	}

	_ = config.ConsoleForward.Set(true, false)
	_ = config.SecurityDisableUnshare.Set(true, false)

	logging.OriginalStdOut = os.Stdout

	var specificID uint
	flag.UintVar(&specificID, "projectId", 0, "Specific project id")
	flag.Parse()

	results := make(map[UnitTest]error)

	for _, unitTest := range tests {
		if specificID != 0 && unitTest.Data.ProjectID != specificID {
			continue
		}
		results[unitTest] = runUnitTest(unitTest)
	}

	if reportResults(results) {
		os.Exit(1)
	}
}

// runUnitTest installs one modpack into a scratch server and returns why it failed, if it did
func runUnitTest(unitTest UnitTest) error {
	test := unitTest.Data

	fmt.Printf("Testing %d\n", test.ProjectID)
	if test.JavaBinary == "" {
		test.JavaBinary = "java"
	}
	serverID := fmt.Sprintf("%d-%d", test.ProjectID, test.FileID)

	_ = os.RemoveAll(serverID)
	_ = os.Mkdir(serverID, 0755)

	server := servers.CreateProgram()
	server.Identifier = serverID

	env, err := servers.CreateEnvironment("host", serverID, "", server.Server)
	if err != nil {
		return err
	}
	server.RunningEnvironment = env

	fs, err := files.NewFileServer(serverID, os.Getuid(), os.Getgid())
	if err != nil {
		return err
	}

	server.SetFileServer(fs)

	arg := skypanel.RunOperatorArgs{
		Environment: env,
		Server:      server,
	}

	result := test.Run(arg)
	if result.Error != nil {
		return result.Error
	}
	var fi os.FileInfo
	if fi, err = os.Lstat(filepath.Join(server.RunningEnvironment.GetRootDirectory(), "server.jar")); err == nil && !fi.IsDir() {
		return nil
	}

	op := resolveforgeversion.ResolveForgeVersion{OutputVariable: "result"}
	result = op.Run(arg)
	if result.Error != nil && !os.IsNotExist(err) {
		return result.Error
	}
	if result.VariableOverrides == nil || result.VariableOverrides["result"] == "" {
		return errors.New("failed to resolve to specific version based on unix_args.txt")
	}
	return nil
}

// reportResults prints each project's outcome and reports whether a must-pass project failed
func reportResults(results map[UnitTest]error) bool {
	failed := false
	for k, v := range results {
		fmt.Printf("Project: %d\n", k.Data.ProjectID)
		if v == nil {
			fmt.Println("  Passes")
		} else {
			fmt.Printf("  Fail: %s\n", v)
			if !k.CanFail {
				failed = true
			}
		}
	}
	return failed
}

type UnitTest struct {
	Data    curseforge.CurseForge
	CanFail bool
}
