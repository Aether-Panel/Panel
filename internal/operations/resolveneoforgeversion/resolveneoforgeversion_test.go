package resolveneoforgeversion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

func TestRunFindsTheInstalledNeoForge(t *testing.T) {
	// the NeoForge installer leaves its launch arguments under libraries/net/neoforged/neoforge
	root := t.TempDir()
	dir := filepath.Join(root, "libraries", "net", "neoforged", "neoforge", "21.4.150")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unix_args.txt"), []byte("-cp x"), 0644); err != nil {
		t.Fatal(err)
	}

	env := &skypanel.Environment{RootDirectory: root, ConsoleBuffer: skypanel.CreateCache(), ConsoleTracker: skypanel.CreateTracker()}
	op := ResolveNeoForgeVersion{MinecraftVersion: "1.21.4", OutputVariable: "resolvedNeoForgeVersion"}
	result := op.Run(skypanel.RunOperatorArgs{Environment: env})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if got := result.VariableOverrides["resolvedNeoForgeVersion"]; got != "21.4.150" {
		t.Fatalf("resolved %v, want 21.4.150", got)
	}
}
