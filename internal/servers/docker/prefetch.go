package docker

import (
	"context"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/moby/moby/client"
)

const (
	// the daemon has just started: let it settle (and its servers start) first
	prefetchDelay = 2 * time.Minute
	// new runtime images are published with panel updates; once a day picks them up
	prefetchEvery = 24 * time.Hour
	prefetchLimit = 30 * time.Minute
)

// PrefetchImages keeps the panel's runtime images (docker.prefetchImages) on the node and up to
// date, in the background. A server starting with an image the node lacks waits for a download
// of hundreds of MB first; and an image the node already has was never downloaded again, so a
// template needing a newer one (a library added to steamcmd) failed until someone pulled it by
// hand. It runs a while after the daemon starts and then once a day.
func PrefetchImages() {
	images := config.DockerPrefetchImages.Value()
	if len(images) == 0 {
		return
	}
	go func() {
		defer skypanel.Recover()
		time.Sleep(prefetchDelay)
		for {
			prefetchOnce(images)
			time.Sleep(prefetchEvery)
		}
	}()
}

func prefetchOnce(images []string) {
	dockerClient, err := client.New(client.FromEnv)
	if err != nil {
		return
	}
	defer utils.Close(dockerClient)

	ctx, cancel := context.WithTimeout(context.Background(), prefetchLimit)
	defer cancel()
	if _, err := dockerClient.Ping(ctx, client.PingOptions{}); err != nil {
		return // no Docker on this node
	}
	for _, image := range images {
		if err := refreshImage(ctx, dockerClient, image); err != nil {
			logging.Error.Printf("Could not download the runtime image %s: %s", image, err)
		}
	}
}

// refreshImage pulls the image and, when that replaced an older copy, removes the old one if no
// container uses it, so a daily refresh does not pile up unused layers.
func refreshImage(ctx context.Context, dockerClient *client.Client, image string) error {
	before := imageID(ctx, dockerClient, image)
	pull, err := dockerClient.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	if err := pull.Wait(ctx); err != nil {
		return err
	}
	after := imageID(ctx, dockerClient, image)
	if before != "" && after != "" && before != after {
		logging.Info.Printf("Updated the runtime image %s", image)
		// fails, and keeps it, while a container still runs on the old one
		_, _ = dockerClient.ImageRemove(ctx, before, client.ImageRemoveOptions{})
	} else if before == "" && after != "" {
		logging.Info.Printf("Downloaded the runtime image %s", image)
	}
	return nil
}

func imageID(ctx context.Context, dockerClient *client.Client, image string) string {
	info, err := dockerClient.ImageInspect(ctx, image)
	if err != nil {
		return ""
	}
	return info.ID
}
