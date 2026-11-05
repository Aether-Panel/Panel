package servers

import (
	"context"
	"errors"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/offsite"
	"github.com/stretchr/testify/assert"
)

// fakeBucket is an in-memory bucket. Set the switches before the code under test runs; read
// what happened through its methods, which lock, since uploads run in the background.
type fakeBucket struct {
	mu      sync.Mutex
	objects map[string]map[string]offsite.Object // server -> file name -> object
	uploads int                                  // Upload calls, failed ones included

	// disabled makes Configured report false, as when the S3 settings are off
	disabled bool
	// failUpload makes every Upload fail without storing anything
	failUpload bool
	// failReach makes Reachable fail, as when the bucket is down
	failReach bool
	// hold, when set, keeps every Upload waiting until it is closed (or its context ends),
	// so a test can look at an upload while it is in progress
	hold chan struct{}
}

// useFakeBucket swaps the real bucket for an empty, configured fake one for this test
func useFakeBucket(t *testing.T) *fakeBucket {
	fake := &fakeBucket{objects: map[string]map[string]offsite.Object{}}
	previous := bucket
	bucket = fake
	t.Cleanup(func() { bucket = previous })
	return fake
}

func (f *fakeBucket) Configured() bool {
	return !f.disabled
}

func (f *fakeBucket) Reachable(context.Context) error {
	if f.failReach {
		return errors.New("fake bucket unreachable")
	}
	return nil
}

func (f *fakeBucket) Upload(ctx context.Context, serverID, name, path string) error {
	f.mu.Lock()
	f.uploads++
	f.mu.Unlock()
	if f.hold != nil {
		select {
		case <-f.hold:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.failUpload {
		return errors.New("fake upload failed")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store(serverID, offsite.Object{FileName: name, Size: info.Size(), ModTime: time.Now()})
	return nil
}

func (f *fakeBucket) List(_ context.Context, serverID string) ([]offsite.Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]offsite.Object, 0, len(f.objects[serverID]))
	for _, o := range f.objects[serverID] {
		result = append(result, o)
	}
	return result, nil
}

func (f *fakeBucket) Delete(_ context.Context, serverID, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects[serverID], name)
	return nil
}

// put seeds an object, as if it had been uploaded at modTime
func (f *fakeBucket) put(serverID, name string, modTime time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store(serverID, offsite.Object{FileName: name, Size: 1, ModTime: modTime})
}

func (f *fakeBucket) store(serverID string, o offsite.Object) {
	if f.objects[serverID] == nil {
		f.objects[serverID] = map[string]offsite.Object{}
	}
	f.objects[serverID][o.FileName] = o
}

// names lists the server's objects, sorted
func (f *fakeBucket) names(serverID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, 0, len(f.objects[serverID]))
	for name := range f.objects[serverID] {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (f *fakeBucket) uploadCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.uploads
}

func TestSeries_OneUploadAtATimePerSeries(t *testing.T) {
	assert.True(t, startSeries("s1", "periodic"))
	assert.False(t, startSeries("s1", "periodic"), "the periodic series is already uploading")
	assert.True(t, startSeries("s1", "task-a"), "another series of the same server")
	assert.True(t, startSeries("s2", "periodic"), "the same series of another server")

	endSeries("s1", "periodic")
	assert.True(t, startSeries("s1", "periodic"), "free again once the upload ended")

	endSeries("s1", "periodic")
	endSeries("s1", "task-a")
	endSeries("s2", "periodic")
}
