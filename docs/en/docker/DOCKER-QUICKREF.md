# Docker Quick Reference

## Images

```bash
# Build main image
docker build -t skypanel:latest .

# Multi-platform build
docker build --platform linux/amd64 -t skypanel:latest .
docker build --platform linux/arm64 -t skypanel:latest .

# Auxiliary Dockerfiles
docker build -f Dockerfile-curseforge -t curseforge-tester .
docker build -f Dockerfile-templatetester -t template-tester .
docker build -f Dockerfile-formatter -t formatter .
```

## Docker Compose

```bash
# Production (Panel + MariaDB)
docker-compose up -d

# Development (Standalone panel, SQLite)
docker-compose -f docker-compose.dev.yml up -d

# Logs
docker-compose logs -f
docker-compose -f docker-compose.dev.yml logs -f

# Stop
docker-compose down
docker-compose -f docker-compose.dev.yml down -v
```

## Panel Commands

```bash
# Create admin user
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin

# Shell
docker exec -it skypanel sh

# Version
docker exec skypanel /SkyPanel/bin/SkyPanel version

# Schema migrations
docker exec skypanel /SkyPanel/bin/SkyPanel db upgrade
```

## Ports

| Port | Service | Prod | Dev |
|---|---|---|---|
| `8080` | Web Panel | yes | yes |
| `5657` | SFTP | yes | yes |
| `3306` | MariaDB | yes | no |

## Volumes

| Path | Purpose | Prod | Dev |
|---|---|---|---|
| `./storage/skypanel-config/` | Config | yes | no |
| `./storage/skypanel-data/` | Data | yes | no |
| `./storage/skypanel-logs/` | Logs | yes | no |
| `./storage/mysql-data/` | MySQL DB | yes | no |
| `./dev-data/data/` | Data | no | yes |
| `./dev-data/logs/` | Logs | no | yes |

## Common Variables

```bash
# Change to SQLite (dev)
SKYPANEL_PANEL_DATABASE_DIALECT=sqlite3

# Customize brand
SKYPANEL_PANEL_SETTINGS_COMPANYNAME="My Panel"

# Template URL
SKYPANEL_TEMPLATES_URL=https://templates.aetherpanel.es/templates.json

# Gemini API Key
SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY=your-api-key

# Discord Webhooks
SKYPANEL_PANEL_NOTIFICATIONS_DISCORDWEBHOOK=https://discord.com/api/webhooks/...
```

## Quick Solution

```bash
# 1. Start
docker-compose up -d

# 2. View logs until ready
docker-compose logs -f

# 3. Create admin
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin

# 4. Open http://localhost:8080
```