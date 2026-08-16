# Referencia Rápida Docker

## Imágenes

```bash
# Construir imagen principal
docker build -t skypanel:latest .

# Build multi-plataforma
docker build --platform linux/amd64 -t skypanel:latest .
docker build --platform linux/arm64 -t skypanel:latest .

# Dockerfiles auxiliares
docker build -f Dockerfile-curseforge -t curseforge-tester .
docker build -f Dockerfile-templatetester -t template-tester .
docker build -f Dockerfile-formatter -t formatter .
```

## Docker Compose

```bash
# Producción (Panel + MariaDB)
docker-compose up -d

# Desarrollo (Panel standalone, SQLite)
docker-compose -f docker-compose.dev.yml up -d

# Logs
docker-compose logs -f
docker-compose -f docker-compose.dev.yml logs -f

# Detener
docker-compose down
docker-compose -f docker-compose.dev.yml down -v
```

## Comandos del Panel

```bash
# Crear usuario admin
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin

# Shell
docker exec -it skypanel sh

# Versión
docker exec skypanel /SkyPanel/bin/SkyPanel version

# Migraciones de esquema
docker exec skypanel /SkyPanel/bin/SkyPanel db upgrade
```

## Puertos

| Puerto | Servicio | Prod | Dev |
|---|---|---|---|
| `8080` | Panel Web | sí | sí |
| `5657` | SFTP | sí | sí |
| `3306` | MariaDB | sí | no |

## Volúmenes

| Ruta | Propósito | Prod | Dev |
|---|---|---|---|
| `./storage/skypanel-config/` | Config | sí | no |
| `./storage/skypanel-data/` | Datos | sí | no |
| `./storage/skypanel-logs/` | Logs | sí | no |
| `./storage/mysql-data/` | BD MySQL | sí | no |
| `./dev-data/data/` | Datos | no | sí |
| `./dev-data/logs/` | Logs | no | sí |

## Variables Comunes

```bash
# Cambiar a SQLite (dev)
SKYPANEL_PANEL_DATABASE_DIALECT=sqlite3

# Personalizar marca
SKYPANEL_PANEL_SETTINGS_COMPANYNAME="Mi Panel"

# Template URL
SKYPANEL_TEMPLATES_URL=https://templates.aetherpanel.es/templates.json

# Gemini API Key
SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY=tu-api-key

# Discord Webhooks
SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOK=https://discord.com/api/webhooks/...
```

## Solución Rápida

```bash
# 1. Iniciar
docker-compose up -d

# 2. Ver logs hasta que esté listo
docker-compose logs -f

# 3. Crear admin
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin

# 4. Abrir http://localhost:8080
```
