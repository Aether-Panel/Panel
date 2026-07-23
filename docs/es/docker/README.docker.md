# Documentación Docker

## Visión General

El proyecto incluye 4 Dockerfiles y 2 configuraciones de Docker Compose para diferentes propósitos.

| Archivo | Propósito |
|---|---|
| `Dockerfile` | Imagen principal de producción (multi-stage) |
| `Dockerfile-curseforge` | Tester de integración CurseForge |
| `Dockerfile-templatetester` | Tester de plantillas (Ubuntu + Mono + Node) |
| `Dockerfile-formatter` | Formateador de código fuente |
| `docker-compose.yml` | Producción: Panel + MariaDB |
| `docker-compose.dev.yml` | Desarrollo: Panel standalone con SQLite |

## Imagen Principal (`Dockerfile`)

Multi-stage build:

```
Stage 1 (node):26-alpine
  → Build frontend (Yarn workspaces, Astro)

Stage 2 (golang:1.26-alpine + tonistiigi/xx)
  → Compila binario Go con CGO, swag, ldflags
  → Frontend build copiado desde stage 1

Stage 3 (alpine:3.24)
  → Imagen final mínima (3.24 MB base + binario + frontend)
```

### Puertos Expuestos

| Puerto | Servicio |
|---|---|
| `8080` | Panel Web (API + frontend) |
| `5657` | SFTP |

### Entrypoint

`/SkyPanel/bin/entrypoint.sh`:
1. Espera a que MySQL esté disponible (hasta 60s, via `nc`)
2. Ejecuta `SkyPanel db migrate`
3. Inicia `SkyPanel run`

### Variables de Entorno

| Variable | Descripción | Default en Docker |
|---|---|---|
| `GIN_MODE` | Modo de Gin | `release` |
| `PUFFER_PLATFORM` | Plataforma | `docker` |
| `PUFFER_DOCKER_ROOT` | Root de Docker | `""` |
| `PUFFER_DOCKER_DISALLOWHOST` | Forzar Docker | `true` |
| `PUFFER_WEB_HOST` | Bind address | `0.0.0.0:8080` |
| `PUFFER_PANEL_DATABASE_DIALECT` | Dialecto BD | `mysql` |
| `PUFFER_PANEL_DATABASE_URL` | Connection string | — |
| `PUFFER_PANEL_SETTINGS_COMPANYNAME` | Marca | `Aether Panel` |
| `PUFFER_PANEL_REGISTRATIONENABLED` | Registro abierto | `true` |
| `PUFFER_PANEL_SETTINGS_DEFAULTTHEME` | Tema default | `SkyPanel` |

### Usuario

Ejecuta como usuario `SkyPanel` (UID 1000) no-root. Los directorios de datos se crean con `mkdir -p` en el build y se asignan a `SkyPanel:SkyPanel`.

### Volúmenes

| Ruta en contenedor | Propósito |
|---|---|
| `/etc/SkyPanel` | Configuración (config.json) |
| `/var/lib/SkyPanel` | Datos runtime (servidores, backups, cache, logs) |
| `/var/log/SkyPanel` | Logs |
| `/var/www/SkyPanel` | Frontend estático |

## Producción (`docker-compose.yml`)

```yaml
services:
  mysql:
    image: mariadb:10.11
    environment:
      - MYSQL_ROOT_PASSWORD=${DB_ROOT_PASSWORD:-skypanel_secret}
      - MYSQL_DATABASE=${DB_DATABASE:-skypanel}
      - MYSQL_USER=${DB_USER:-skypanel}
      - MYSQL_PASSWORD=${DB_PASSWORD:-skypanel_secret}
    volumes:
      - ./storage/mysql-data:/var/lib/mysql
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h localhost -u root -p$${MYSQL_ROOT_PASSWORD}"]

  skypanel:
    build:
      context: .
      dockerfile: Dockerfile
      args:
        - version=dev-docker
        - sha=local-build
    ports:
      - "8080:8080"
      - "5657:5657"
    volumes:
      - ./storage/skypanel-config:/etc/SkyPanel
      - ./storage/skypanel-data:/var/lib/SkyPanel
      - ./storage/skypanel-logs:/var/log/SkyPanel
      - //var/run/docker.sock:/var/run/docker.sock
    environment:
      - PUFFER_PANEL_DATABASE_DIALECT=mysql
      - PUFFER_PANEL_DATABASE_URL=${DB_USER:-skypanel}:${DB_PASSWORD:-skypanel_secret}@tcp(mysql:3306)/${DB_DATABASE:-skypanel}?charset=utf8&parseTime=true
    depends_on:
      mysql:
        condition: service_healthy
```

## Desarrollo (`docker-compose.dev.yml`)

```yaml
services:
  skypanel:
    container_name: skypanel-dev
    build:
      context: .
      dockerfile: Dockerfile
      args:
        - version=dev-local
        - sha=dev
    ports:
      - "8080:8080"
      - "5657:5657"
    volumes:
      - ./dev-data/data:/var/lib/SkyPanel:z
      - ./dev-data/logs:/var/log/SkyPanel:z
      - /var/run/docker.sock:/var/run/docker.sock:z
    privileged: true
    user: "0:0"
    environment:
      - GIN_MODE=debug
      - PUFFER_LOGS_LEVEL=DEBUG
      # SQLite por defecto (no requiere servicio de BD)
```

## Comandos Útiles

```bash
# Iniciar producción
docker-compose up -d

# Iniciar desarrollo
docker-compose -f docker-compose.dev.yml up -d

# Ver logs
docker-compose logs -f

# Detener
docker-compose down

# Crear usuario admin
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin

# Shell interactivo
docker exec -it skypanel sh

# Ver versión
docker exec skypanel /SkyPanel/bin/SkyPanel version

# Ejecutar comando CLI
docker exec skypanel /SkyPanel/bin/SkyPanel version

# Reconstruir imagen
docker-compose build

# Actualizar imagen
docker-compose pull
```

## Build Multi-Plataforma

El Dockerfile soporta build multi-arquitectura via `tonistiigi/xx`:

```bash
# Build para linux/amd64
docker build --platform linux/amd64 -t skypanel:latest .

# Build para linux/arm64
docker build --platform linux/arm64 -t skypanel:latest .
```

## Configuración Personalizada

La configuración por defecto está en `config.docker.json` con MySQL. Para usar SQLite en producción, cambia las variables de entorno:

```yaml
environment:
  - PUFFER_PANEL_DATABASE_DIALECT=sqlite3
  # No requiere PUFFER_PANEL_DATABASE_URL (default: file:skypanel.db)
  # Elimina depends_on: mysql
```

## Dockerfiles Auxiliares

### `Dockerfile-curseforge`
Imagen para el tester de CurseForge. Build con CGO, runtime Eclipse Temurin 25 (Java). Usado para verificar modpacks de CurseForge.

### `Dockerfile-templatetester`
Imagen pesada para testear plantillas. Incluye Mono (para SteamCMD), Node.js 20, GCC, zip/unzip, y dependencias i386. Construye la herramienta `templatetester`.

### `Dockerfile-formatter`
Imagen mínima para formatear código Go. Construye la herramienta `formatter` y la ejecuta sobre el código fuente.

## Solución de Problemas

### Error: "Bind for 0.0.0.0:8080 failed: port is already allocated"
Cambiar mapeo de puertos:
```yaml
ports:
  - "9000:8080"
```

### Error: "permission denied" al conectar Docker socket
Asegurar que el usuario del contenedor tiene permisos. En producción:
```yaml
user: "0:0"  # Temporal
```
En desarrollo ya está configurado con `privileged: true` y `user: "0:0"`.

### Error: MySQL connection refused
El entrypoint espera hasta 60s. Verificar:
```bash
docker logs skypanel
docker logs skypanel-mysql
```

### Los cambios no persisten
Verificar volúmenes:
```bash
docker volume ls
docker inspect skypanel | grep Mounts
```
