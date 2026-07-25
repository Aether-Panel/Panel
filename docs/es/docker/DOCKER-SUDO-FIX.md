# Docker Socket y Permisos

El panel necesita acceso al socket de Docker para orquestar contenedores de servidores de juego. Esto aplica tanto en desarrollo como en producción.

## Montaje del Socket

En ambos `docker-compose.yml` y `docker-compose.dev.yml` el socket se monta así:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

## Gestión de Servidores

El panel ejecuta cada servidor de juego en su propio contenedor Docker. Para esto requiere:

1. Acceso al socket Docker del host
2. Permisos para crear/eliminar contenedores
3. La configuración `PUFFER_DOCKER_DISALLOWHOST=true` para forzar el uso de Docker (deshabilitando entornos host/TTY)

## Solución de Problemas

### Error: "Cannot connect to the Docker daemon"

1. Verificar que Docker está corriendo en el host:
   ```bash
   sudo systemctl status docker
   ```

2. Verificar que el socket existe:
   ```bash
   ls -la /var/run/docker.sock
   ```

3. Verificar que el contenedor ve el socket:
   ```bash
   docker exec skypanel ls -la /var/run/docker.sock
   ```

### Error: "permission denied" al acceder al socket

El socket Docker tiene permisos `srw-rw----` (socket, owner root, group docker). El usuario `SkyPanel` (UID 1000) dentro del contenedor necesita acceso.

**Solución en producción (`docker-compose.yml`):**
El contenedor ejecuta como root (`user: "0:0"`), lo que evita problemas de permisos.

**Solución en desarrollo (`docker-compose.dev.yml`):**
Usa `privileged: true` y `user: "0:0"`, que también evita restricciones.

### El host no tiene Docker

Si el panel corre sin Docker en el host (ej: servidor sin Docker), deshabilitar el uso de Docker:

```yaml
environment:
  - PUFFER_DOCKER_DISALLOWHOST=false
```

Y quitar el montaje del socket. El panel usará entornos TTY en su lugar.

### Seguridad

- El socket Docker montado otorga control total sobre Docker del host
- En producción, asegurar que solo usuarios confiables tengan acceso al host
- Considerar usar `docker.sock` con proxies de autorización como `alexellis/faas-netes` o `docker-socket-proxy`
