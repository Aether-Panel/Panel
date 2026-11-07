package skypanel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// hostServer is a template that runs on the host, so its binaries are looked up on the PATH.
func hostServer() Server {
	return Server{Environment: MetadataType{Type: "host"}}
}

func TestRequirements_AnyListedAlternativeSatisfiesABinaryRow(t *testing.T) {
	// "sh" is on the PATH of every system the tests run on; the other name is on none
	req := Requirements{Binaries: []string{"skypanel-missing-binary-xyz || sh"}}
	assert.NoError(t, req.Test(hostServer()))

	req = Requirements{Binaries: []string{"sh || skypanel-missing-binary-xyz"}}
	assert.NoError(t, req.Test(hostServer()))
}

func TestRequirements_ARowFailsWhenNoAlternativeIsFound(t *testing.T) {
	req := Requirements{Binaries: []string{"skypanel-missing-a || skypanel-missing-b"}}
	err := req.Test(hostServer())
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "skypanel-missing-a OR skypanel-missing-b")
	}
}

func TestRequirements_EveryRowIsNeeded(t *testing.T) {
	req := Requirements{Binaries: []string{"sh", "skypanel-missing-binary-xyz"}}
	assert.Error(t, req.Test(hostServer()))
}

func TestRequirements_AnEmptyRowAsksForNothing(t *testing.T) {
	req := Requirements{Binaries: []string{"", "sh"}}
	assert.NoError(t, req.Test(hostServer()))
}

func TestRequirements_VariablesAreReplacedInBinaryNames(t *testing.T) {
	server := hostServer()
	server.Variables = map[string]Variable{"shell": {Value: "sh"}}
	req := Requirements{Binaries: []string{"skypanel-missing-binary-xyz || ${shell}"}}
	assert.NoError(t, req.Test(server))
}
