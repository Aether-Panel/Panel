package update

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// legacyCommand is how the Docker API showed an update container's command, quotes included
const legacyCommand = "chroot /host bash -c 'cd /opt/skypanel && chmod +x tools/panelUpdate/panelUpdate.sh && ./tools/panelUpdate/panelUpdate.sh'"

type fakeDocker struct {
	mu       sync.Mutex
	items    []container.Summary
	listErr  error
	removed  []string
	waited   chan string
	finished chan container.WaitResponse
}

func (f *fakeDocker) ContainerList(_ context.Context, options client.ContainerListOptions) (client.ContainerListResult, error) {
	if !options.All {
		return client.ContainerListResult{}, errors.New("a stopped container is only listed with All")
	}
	return client.ContainerListResult{Items: f.items}, f.listErr
}

func (f *fakeDocker) ContainerRemove(_ context.Context, id string, _ client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
	return client.ContainerRemoveResult{}, nil
}

func (f *fakeDocker) ContainerWait(_ context.Context, id string, _ client.ContainerWaitOptions) client.ContainerWaitResult {
	f.waited <- id
	return client.ContainerWaitResult{Result: f.finished, Error: make(chan error)}
}

func (f *fakeDocker) removedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.removed)
}

func TestRemoveLeftovers_RemovesOnlyFinishedUpdateContainers(t *testing.T) {
	f := &fakeDocker{items: []container.Summary{
		{ID: "labelled", Image: Image, Labels: map[string]string{Label: "true"}, State: container.StateExited},
		{ID: "legacy", Image: Image, Command: legacyCommand, State: container.StateExited},
		{ID: "never-started", Image: Image, Labels: map[string]string{Label: "true"}, State: container.StateCreated},
		{ID: "someone-elses-alpine", Image: Image, Command: "sh -c echo hi", State: container.StateExited},
		{ID: "script-in-another-image", Image: "ubuntu:latest", Command: legacyCommand, State: container.StateExited},
		{ID: "a-server", Image: "node:24", Labels: map[string]string{"skypanel.server": "abc"}, State: container.StateExited},
	}}

	removeLeftovers(context.Background(), f)

	if got, want := f.removedIDs(), []string{"labelled", "legacy", "never-started"}; !slices.Equal(got, want) {
		t.Fatalf("removed %v, want %v", got, want)
	}
}

func TestRemoveLeftovers_ARunningUpdateIsRemovedWhenItEnds(t *testing.T) {
	f := &fakeDocker{
		items: []container.Summary{
			{ID: "still-updating", Image: Image, Labels: map[string]string{Label: "true"}, State: container.StateRunning},
			{ID: "old", Image: Image, Command: legacyCommand, State: container.StateExited},
		},
		waited:   make(chan string, 1),
		finished: make(chan container.WaitResponse, 1),
	}
	done := make(chan struct{})
	go func() {
		removeLeftovers(context.Background(), f)
		close(done)
	}()

	select {
	case id := <-f.waited:
		if id != "still-updating" {
			t.Fatalf("waited for %q", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the running update container was not waited for")
	}
	if got := f.removedIDs(); !slices.Equal(got, []string{"old"}) {
		t.Fatalf("removed %v before the update ended, want only the finished one", got)
	}

	f.finished <- container.WaitResponse{StatusCode: 0}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("did not return after the update ended")
	}
	if got := f.removedIDs(); !slices.Equal(got, []string{"old", "still-updating"}) {
		t.Fatalf("removed %v, want the running one too once it ended", got)
	}
}

func TestRemoveLeftovers_ADockerErrorRemovesNothing(t *testing.T) {
	f := &fakeDocker{
		items:   []container.Summary{{ID: "labelled", Image: Image, Labels: map[string]string{Label: "true"}, State: container.StateExited}},
		listErr: errors.New("no docker here"),
	}

	removeLeftovers(context.Background(), f)

	if got := f.removedIDs(); len(got) != 0 {
		t.Fatalf("removed %v after the list failed", got)
	}
}

func TestIsUpdateContainer_TheLegacyCommandMatchesWhatTriggerRuns(t *testing.T) {
	// Docker shows a container's command as its words joined by spaces, quoting the ones with spaces
	c := container.Summary{Image: Image, Command: "chroot /host bash -c '" + Command[len(Command)-1] + "'"}
	if !isUpdateContainer(c) {
		t.Fatalf("the command Trigger runs, %q, is not recognised", c.Command)
	}
}
