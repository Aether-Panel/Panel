package curseforge

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

func TestFactoryReadsTheTemplatesFileId(t *testing.T) {
	if err := config.CurseForgeKey.Set("test-key", false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.CurseForgeKey.Set("", false) })

	create := func(args map[string]interface{}) *CurseForge {
		t.Helper()
		op, err := Factory.Create(skypanel.CreateOperation{OperationArgs: args})
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return op.(*CurseForge)
	}

	// the template writes fileId; reading only fileID silently installed the newest file instead
	if got := create(map[string]interface{}{"projectId": "238222", "fileId": "4712345"}).FileID; got != 4712345 {
		t.Fatalf("fileId: FileID = %d, want 4712345", got)
	}
	if got := create(map[string]interface{}{"projectId": "238222", "fileID": "4712345"}).FileID; got != 4712345 {
		t.Fatalf("fileID: FileID = %d, want 4712345", got)
	}
	// an empty file id (the template's default) means the newest file
	if got := create(map[string]interface{}{"projectId": "238222", "fileId": ""}).FileID; got != 0 {
		t.Fatalf("empty fileId: FileID = %d, want 0", got)
	}
	if _, err := Factory.Create(skypanel.CreateOperation{OperationArgs: map[string]interface{}{"projectId": "238222", "fileId": "abc"}}); err == nil {
		t.Fatal("a non-numeric file id must be refused")
	}
}
