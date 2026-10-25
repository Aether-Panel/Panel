package docker

import (
	"context"

	"github.com/moby/moby/client"
)

// ContainerAddresses lists where a server's container can be reached from this node: its IP
// on each network it joined, and the gateways of those networks (the host side, where its
// published ports listen). Services inside the container that are not published, such as a
// map's web server, are only reachable on the container IPs.
func (d *Docker) ContainerAddresses(ctx context.Context, containerName string) (ips, gateways []string, err error) {
	dockerClient, err := d.getClient()
	if err != nil {
		return nil, nil, err
	}
	info, err := dockerClient.ContainerInspect(ctx, containerName, client.ContainerInspectOptions{})
	if err != nil {
		return nil, nil, err
	}
	if info.Container.NetworkSettings == nil {
		return nil, nil, nil
	}
	for _, endpoint := range info.Container.NetworkSettings.Networks {
		if endpoint == nil {
			continue
		}
		if endpoint.IPAddress.IsValid() && !endpoint.IPAddress.IsUnspecified() {
			ips = append(ips, endpoint.IPAddress.String())
		}
		if endpoint.Gateway.IsValid() && !endpoint.Gateway.IsUnspecified() {
			gateways = append(gateways, endpoint.Gateway.String())
		}
	}
	return ips, gateways, nil
}
