# Docker Socket and Permissions

The panel creates and controls the game server containers through the host's Docker socket. This document explains how it gets access and how to fix connection and permission errors.

## Socket mount

Both Compose files mount the host's socket into the panel container:

```yaml
# docker-compose.yml
volumes:
  - //var/run/docker.sock:/var/run/docker.sock

# docker-compose.dev.yml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:z
```

The panel uses the Docker client with the standard variables (`DOCKER_HOST`, etc.); without them it connects to `unix:///var/run/docker.sock`.

## Container user

By default the image runs as the `SkyPanel` user (UID 1000), which has no permission on the socket (`srw-rw----`, `root:docker`). That is why:

- `docker-compose.yml` runs the panel as root (`user: "0:0"`).
- `docker-compose.dev.yml` also uses `privileged: true`.

The panel being root does not make the games root: with the panel as root, installs run as root and the game as `docker.serverUser` (`988:988` by default), without the Linux capabilities it does not need, with `no-new-privileges` and with a process limit (`docker.pidsLimit`). See [Container user and isolation](../backend/04-servers.md#container-user-and-isolation).

## Containers only (`docker.disallowHost`)

The image and `docker-compose.yml` set `SKYPANEL_DOCKER_DISALLOWHOST=true`. With this option the node only runs servers in containers: the environments that run processes directly on the host (`host`, `tty`, `standard`, `bubblewrap`) are hidden, refused when a server is created or redefined, and do not start even if their definition arrives another way. Do not turn it off in the Docker install: the panel runs inside a container and host environments would make no sense.

## Troubleshooting

### "Cannot connect to the Docker daemon"

1. Check that Docker runs on the host:
   ```bash
   sudo systemctl status docker
   docker info
   ```
2. Check that the socket exists on the host and that the container sees it:
   ```bash
   ls -la /var/run/docker.sock
   docker exec skypanel ls -la /var/run/docker.sock
   ```

If the daemon cannot start inside Docker (`SKYPANEL_PLATFORM=docker`), the web panel keeps running and logs "Continuing without daemon in Docker environment", but this node's servers will not be available until it is fixed and the container restarted.

### "permission denied" on the socket

The container is not running as root. Check that the `skypanel` service has `user: "0:0"` (also in your own `docker-compose.override.yml`) and recreate the container:

```bash
docker compose up -d --force-recreate skypanel
```

## Security

- Whoever controls the Docker socket controls the host's whole Docker. Limit access to the server to trusted people.
- MariaDB and Adminer are only published on `127.0.0.1`; game servers use their own network (`skypanel-servers`) and cannot reach the panel.
- A socket authorisation proxy (such as `docker-socket-proxy`) has not been tested with the panel: the panel needs to create, inspect, start and remove containers, pull images, inspect networks and run commands inside containers.
