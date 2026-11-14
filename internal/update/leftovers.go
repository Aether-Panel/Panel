package update

import (
	"context"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// Label marks the container of an update. The update restarts the panel, and with it goes
// the monitor that was to remove the container, so the next start finds it by this label.
const Label = "skypanel.update"

// legacyScript is in the command of the update containers made before they had the label.
const legacyScript = "tools/panelUpdate/panelUpdate.sh"

// dockerAPI is the part of the Docker client the clean-up of old update containers uses.
type dockerAPI interface {
	ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	ContainerWait(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult
}

// RemoveLeftovers removes the containers of earlier updates of this host. One that is
// still running, because the update restarted the panel half way, is removed when it ends.
// It blocks until then, so it is meant to run in its own goroutine.
func RemoveLeftovers() {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		logging.Debug.Printf("Old update containers not checked: %v", err)
		return
	}
	defer cli.Close()
	removeLeftovers(context.Background(), cli)
}

func removeLeftovers(ctx context.Context, cli dockerAPI) {
	list, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		logging.Debug.Printf("Old update containers not checked: %v", err)
		return
	}
	var running []string
	for _, c := range list.Items {
		if !isUpdateContainer(c) {
			continue
		}
		if c.State == container.StateRunning || c.State == container.StateRestarting {
			running = append(running, c.ID)
			continue
		}
		removeUpdateContainer(ctx, cli, c.ID)
	}
	for _, id := range running {
		wait := cli.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
		select {
		case <-wait.Result:
		case err := <-wait.Error:
			logging.Debug.Printf("Could not wait for update container %s: %v", id, err)
			continue
		}
		removeUpdateContainer(ctx, cli, id)
	}
}

func removeUpdateContainer(ctx context.Context, cli dockerAPI, id string) {
	if _, err := cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true}); err != nil {
		logging.Error.Printf("Failed to remove old update container %s: %v", id, err)
		return
	}
	logging.Info.Printf("Removed old update container %s", id)
}

// isUpdateContainer tells an update container: one with the label, or one made before the
// label existed, which ran the update script from the update image chrooted into the host.
func isUpdateContainer(c container.Summary) bool {
	if _, ok := c.Labels[Label]; ok {
		return true
	}
	return c.Image == Image && strings.HasPrefix(c.Command, "chroot /host ") && strings.Contains(c.Command, legacyScript)
}
