package servers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// operationsManifest is the list of operations the template editor describes, one per step
// type, which the frontend keeps next to its registry (operations.ts) and checks against it
const operationsManifest = "client/frontend/src/features/templates/schema/operations.json"

// TestOperationsManifest_MatchesTheNode fails when an operation registered here has no entry in
// the template editor (so it would have no form there), when the editor describes an operation
// the node does not run, and when the editor would offer a server operation in templates: those
// are for scheduled tasks, since a server cannot start or back up while it installs.
func TestOperationsManifest_MatchesTheNode(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(templateRepoRoot(t), filepath.FromSlash(operationsManifest)))
	if err != nil {
		t.Fatalf("reading %s: %v", operationsManifest, err)
	}
	var manifest struct {
		Operations []struct {
			Key       string `json:"key"`
			Templates bool   `json:"templates"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parsing %s: %v", operationsManifest, err)
	}

	inTemplates := make(map[string]bool, len(manifest.Operations))
	for _, op := range manifest.Operations {
		if _, dup := inTemplates[op.Key]; dup {
			t.Errorf("%s lists %q twice", operationsManifest, op.Key)
		}
		inTemplates[op.Key] = op.Templates
	}

	serverOnly := make(map[string]bool, len(serverOperationFactories))
	for _, f := range serverOperationFactories {
		serverOnly[f.Key()] = true
	}

	var missing, unknown []string
	for key := range commandMapping {
		if _, ok := inTemplates[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key, templates := range inTemplates {
		if _, ok := commandMapping[key]; !ok {
			unknown = append(unknown, key)
			continue
		}
		if templates == serverOnly[key] {
			t.Errorf("%s: %q should have \"templates\": %v", operationsManifest, key, !serverOnly[key])
		}
	}
	sort.Strings(missing)
	sort.Strings(unknown)
	if len(missing) > 0 {
		t.Errorf("operations the node registers without an entry in the template editor (add them to operations.ts, then operations.json): %v", missing)
	}
	if len(unknown) > 0 {
		t.Errorf("%s lists operations the node does not register: %v", operationsManifest, unknown)
	}
}
