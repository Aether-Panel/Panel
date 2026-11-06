package servers

import (
	"context"
	"sync"

	"github.com/SkyPanel/SkyPanel/v3/internal/offsite"
)

// offsiteBucket is the S3 bucket as the backup code sees it, so the tests can use one in
// memory instead of a real bucket
type offsiteBucket interface {
	// Configured is whether off-site backups are on and fully set up
	Configured() bool
	// Reachable checks that the bucket answers and exists
	Reachable(ctx context.Context) error
	Upload(ctx context.Context, serverID, name, path string) error
	List(ctx context.Context, serverID string) ([]offsite.Object, error)
	// Delete removes a backup; a missing one is not an error
	Delete(ctx context.Context, serverID, name string) error
}

// bucket is where off-site copies go; the tests swap it for a fake
var bucket offsiteBucket = settingsBucket{}

// settingsBucket is the bucket in the panel's settings. They are read on every call, so a
// change made in the settings page applies to the next transfer without a restart.
type settingsBucket struct{}

func (settingsBucket) Configured() bool {
	_, ok := offsite.FromSettings()
	return ok
}

func (settingsBucket) Reachable(ctx context.Context) error {
	cfg, ok := offsite.FromSettings()
	if !ok {
		return offsite.ErrNotConfigured
	}
	return offsite.Reachable(ctx, cfg)
}

func (settingsBucket) Upload(ctx context.Context, serverID, name, path string) error {
	cfg, ok := offsite.FromSettings()
	if !ok {
		return offsite.ErrNotConfigured
	}
	return offsite.Upload(ctx, cfg, serverID, name, path)
}

func (settingsBucket) List(ctx context.Context, serverID string) ([]offsite.Object, error) {
	cfg, ok := offsite.FromSettings()
	if !ok {
		return nil, offsite.ErrNotConfigured
	}
	return offsite.List(ctx, cfg, serverID)
}

func (settingsBucket) Delete(ctx context.Context, serverID, name string) error {
	cfg, ok := offsite.FromSettings()
	if !ok {
		return offsite.ErrNotConfigured
	}
	return offsite.Delete(ctx, cfg, serverID, name)
}

// uploading holds the series with an upload in progress, per server. An upload runs after the
// backup has ended, so until it finishes the next run of the same series would not find it in
// the bucket and would upload again; it must wait for this one instead.
var uploading = struct {
	sync.Mutex
	series map[string]map[string]bool // server -> series
}{series: map[string]map[string]bool{}}

// startSeries claims the series (a task's id, or periodicSeries) of the server for one upload; it
// reports false while another upload of the same series is in progress
func startSeries(serverID, series string) bool {
	uploading.Lock()
	defer uploading.Unlock()
	if uploading.series[serverID][series] {
		return false
	}
	if uploading.series[serverID] == nil {
		uploading.series[serverID] = map[string]bool{}
	}
	uploading.series[serverID][series] = true
	return true
}

// endSeries frees the series once its upload is over, whatever the outcome
func endSeries(serverID, series string) {
	uploading.Lock()
	defer uploading.Unlock()
	delete(uploading.series[serverID], series)
	if len(uploading.series[serverID]) == 0 {
		delete(uploading.series, serverID)
	}
}
