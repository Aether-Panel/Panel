package servers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStartFailureReason_KeepsTheMissingProgram(t *testing.T) {
	err := errors.New(`Error response from daemon: failed to create task for container: failed to create shim task: OCI runtime create failed: runc create failed: unable to start container process: error during container init: exec: "docker-entrypoint.sh": executable file not found in $PATH: unknown`)

	assert.Equal(t, `"docker-entrypoint.sh": executable file not found in $PATH`, startFailureReason(err))
}

func TestStartFailureReason_LeavesOtherErrorsAlone(t *testing.T) {
	err := errors.New("Error response from daemon: driver failed programming external connectivity: port is already allocated")

	assert.Equal(t, "driver failed programming external connectivity: port is already allocated", startFailureReason(err))
}
