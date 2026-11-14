package command

import (
	"sync"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exitingEnvironment runs every command as a process that exits at once with exitCode, and
// handles the exit the way the Docker environment does: the wait is released only after
// the callback returns
type exitingEnvironment struct{ exitCode int }

func (f exitingEnvironment) ExecuteAsyncImpl(env *skypanel.Environment, steps skypanel.ExecutionData) error {
	env.Wait.Add(1)
	go func() {
		defer env.Wait.Done()
		if steps.Callback != nil {
			steps.Callback(f.exitCode)
		}
	}()
	return nil
}
func (exitingEnvironment) KillImpl(*skypanel.Environment) error { return nil }
func (exitingEnvironment) GetStatsImpl(*skypanel.Environment) (*skypanel.ServerStats, error) {
	return &skypanel.ServerStats{}, nil
}
func (exitingEnvironment) SendCodeImpl(*skypanel.Environment, int) error     { return nil }
func (exitingEnvironment) GetUIDImpl(*skypanel.Environment) int              { return 0 }
func (exitingEnvironment) GetGidImpl(*skypanel.Environment) int              { return 0 }
func (exitingEnvironment) IsRunningImpl(*skypanel.Environment) (bool, error) { return false, nil }

func runCommand(t *testing.T, exitCode int) skypanel.OperationResult {
	env := &skypanel.Environment{
		Wait:           &sync.WaitGroup{},
		ConsoleBuffer:  skypanel.CreateCache(),
		ConsoleTracker: skypanel.CreateTracker(),
		Implementation: exitingEnvironment{exitCode: exitCode},
	}
	done := make(chan skypanel.OperationResult, 1)
	go func() {
		done <- Command{Commands: []string{"steamcmd +quit"}}.Run(skypanel.RunOperatorArgs{Environment: env})
	}()
	select {
	case result := <-done:
		return result
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the command never finished: its exit handling is blocked, and the server stays installing")
		return skypanel.OperationResult{}
	}
}

func TestCommand_AFailedCommandEndsTheStep(t *testing.T) {
	result := runCommand(t, 1)
	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "exit code 1")
}

func TestCommand_ASuccessfulCommandGoesOn(t *testing.T) {
	assert.NoError(t, runCommand(t, 0).Error)
}
