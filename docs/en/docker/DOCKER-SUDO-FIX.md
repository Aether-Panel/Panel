# Docker Socket and Permissions

The panel needs access to the Docker socket to orchestrate game server containers. This applies in both development and production.

## Mounting the Socket

In both `docker-compose.yml` and `docker-compose.dev.yml` the socket is mounted as follows:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

## Server Management

The panel runs each game server in its own Docker container. For this it requires:

1. Access to the host's Docker socket
2. Permissions to create/delete containers
3. The `SKYPANEL_DOCKER_DISALLOWHOST=true` configuration to force Docker usage (disabling host/TTY environments)

## Troubleshooting

### Error: "Cannot connect to the Docker daemon"

1. Verify that Docker is running on the host:
   ```bash
   sudo systemctl status docker
   ```

2. Verify that the socket exists:
   ```bash
   ls -la /var/run/docker.sock
   ```

3. Verify that the container sees the socket:
   ```bash
   docker exec skypanel ls -la /var/run/docker.sock
   ```

### Error: "permission denied" when accessing the socket

The Docker socket has permissions `srw-rw----` (socket, owner root, group docker). The `SkyPanel` user (UID 1000) inside the container needs access.

**Solution in production (`docker-compose.yml`):**
The container runs as root (`user: "0:0"`), which avoids permission issues.

**Solution in development (`docker-compose.dev.yml`):**
Uses `privileged: true` and `user: "0:0"`, which also avoids restrictions.

### Host does not have Docker

If the panel runs without Docker on the host (e.g., server without Docker), disable Docker usage:

```yaml
environment:
  - SKYPANEL_DOCKER_DISALLOWHOST=false
```

And remove the socket mount. The panel will use TTY environments instead.

### Security

- The mounted Docker socket grants full control over the host's Docker
- In production, ensure that only trusted users have access to the host
- Consider using `docker.sock` with authorization proxies like `alexellis/faas-netes` or `docker-socket-proxy`