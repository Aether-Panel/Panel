# Socket de Docker y permisos

El panel crea y controla los contenedores de los servidores de juego a través del socket de Docker del host. Este documento explica cómo se le da acceso y cómo resolver los errores de conexión y de permisos.

## Montaje del socket

Los dos archivos de Compose montan el socket del host en el contenedor del panel:

```yaml
# docker-compose.yml
volumes:
  - //var/run/docker.sock:/var/run/docker.sock

# docker-compose.dev.yml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:z
```

El panel usa el cliente de Docker con las variables estándar (`DOCKER_HOST`, etc.); sin ellas, se conecta a `unix:///var/run/docker.sock`.

## Usuario del contenedor

La imagen se ejecuta por defecto como el usuario `SkyPanel` (UID 1000), que no tiene permiso sobre el socket (`srw-rw----`, `root:docker`). Por eso:

- `docker-compose.yml` ejecuta el panel como root (`user: "0:0"`).
- `docker-compose.dev.yml` usa además `privileged: true`.

Que el panel sea root no implica que lo sean los juegos: con el panel como root, las instalaciones se ejecutan como root y el juego como `docker.serverUser` (`988:988` por defecto), sin las capacidades de Linux que no necesita, con `no-new-privileges` y con un límite de procesos (`docker.pidsLimit`). Ver [Usuario y aislamiento del contenedor](../backend/04-servers.md#usuario-y-aislamiento-del-contenedor).

## Solo contenedores (`docker.disallowHost`)

La imagen y `docker-compose.yml` fijan `SKYPANEL_DOCKER_DISALLOWHOST=true`. Con esta opción el nodo solo ejecuta servidores en contenedores: los entornos que ejecutan procesos directamente en el host (`host`, `tty`, `standard`, `bubblewrap`) no aparecen, se rechazan al crear o redefinir un servidor y no arrancan aunque su definición llegue por otro camino. No la desactive en la instalación con Docker: el panel corre dentro de un contenedor y los entornos de host no tendrían sentido.

## Solución de problemas

### «Cannot connect to the Docker daemon»

1. Compruebe que Docker funciona en el host:
   ```bash
   sudo systemctl status docker
   docker info
   ```
2. Compruebe que el socket existe en el host y que el contenedor lo ve:
   ```bash
   ls -la /var/run/docker.sock
   docker exec skypanel ls -la /var/run/docker.sock
   ```

Si el daemon no puede iniciarse dentro de Docker (`SKYPANEL_PLATFORM=docker`), el panel web sigue en marcha y lo registra con «Continuing without daemon in Docker environment», pero los servidores de este nodo no estarán disponibles hasta que se corrija y se reinicie el contenedor.

### «permission denied» al acceder al socket

El contenedor no se está ejecutando como root. Compruebe que el servicio `skypanel` tiene `user: "0:0"` (también en un `docker-compose.override.yml` propio) y recree el contenedor:

```bash
docker compose up -d --force-recreate skypanel
```

## Seguridad

- Quien controla el socket de Docker controla todo el Docker del host. Limite el acceso al servidor a personas de confianza.
- MariaDB y Adminer solo se publican en `127.0.0.1`; los servidores de juego van a su propia red (`skypanel-servers`) y no llegan al panel.
- Un proxy de autorización del socket (por ejemplo `docker-socket-proxy`) no está probado con el panel: el panel necesita crear, inspeccionar, arrancar y eliminar contenedores, descargar imágenes, inspeccionar redes y ejecutar comandos dentro de los contenedores.
