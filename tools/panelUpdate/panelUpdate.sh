#!/bin/bash

# =============================================================================
# Aether Panel - Script de Actualización Automática
# Este script es ejecutado por el panel desde la interfaz web.
# =============================================================================

set -e

readonly PROJECT_NAME="Aether Panel"
readonly INSTALL_DIR="/opt/skypanel"
GIT_BRANCH="dev2.0"

# Detectar docker compose
if command -v docker-compose &>/dev/null; then
    DOCKER_COMPOSE="docker-compose"
elif docker compose version &>/dev/null; then
    DOCKER_COMPOSE="docker compose"
else
    echo "[✗] No se encontró docker compose."
    exit 1
fi

echo "[INFO] Iniciando actualización de $PROJECT_NAME..."

if ! docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^skypanel$"; then
    echo "[✗] No se detectó el contenedor de skypanel en ejecución."
    exit 1
fi

cd "$INSTALL_DIR"

# Preservar la configuración de nodo esclavo (Daemon only) si está activada
# para que los cambios personalizados sobrevivan al git pull que revierte el yml.
IS_SLAVE=false
SLAVE_TOKEN_PUBLIC=""
if grep -qE '^[[:space:]]*- SKYPANEL_PANEL_ENABLE=false' docker-compose.yml 2>/dev/null; then
    IS_SLAVE=true
    SLAVE_TOKEN_PUBLIC=$(grep -oE 'SKYPANEL_TOKEN_PUBLIC=[^ ]+' docker-compose.yml 2>/dev/null | head -1 | cut -d= -f2-)
    echo "[INFO] Nodo esclavo detectado: se preservará la configuración de daemon en el yml."
fi

echo "[INFO] Obteniendo últimos cambios de GitHub..."
git checkout -- . 2>/dev/null || true
git fetch origin
git checkout "$GIT_BRANCH" 2>/dev/null || git checkout main
git pull

# Re-aplicar configuración de esclavo sobre el yml recién actualizado
if [ "$IS_SLAVE" = "true" ]; then
    sed -i 's|^[[:space:]]*# - SKYPANEL_PANEL_ENABLE=false|      - SKYPANEL_PANEL_ENABLE=false|' docker-compose.yml
    if [ -n "$SLAVE_TOKEN_PUBLIC" ]; then
        sed -i "s|^[[:space:]]*# - SKYPANEL_TOKEN_PUBLIC=.*|      - SKYPANEL_TOKEN_PUBLIC=${SLAVE_TOKEN_PUBLIC}|" docker-compose.yml
    fi
    echo "[✓] Configuración de nodo esclavo re-aplicada en docker-compose.yml."
fi

# Reasignar puerto MySQL del host si 3306 está ocupado (ej. mysql-server local)
if command -v ss &>/dev/null; then
    if ss -ltn 2>/dev/null | grep -q ":3306\b"; then
        echo "[!] El puerto 3306 del host está en uso. Reasignando contenedor MySQL al puerto 3307."
        sed -i 's/- "3306:3306"/- "3307:3306"/' docker-compose.yml 2>/dev/null || true
    fi
fi

echo "[INFO] Reconstruyendo imagen Docker (esto puede tardar varios minutos)..."
$DOCKER_COMPOSE down 2>/dev/null || true
COMPOSE_PROFILES=local-db $DOCKER_COMPOSE up -d --build 2>&1

echo "[✓] $PROJECT_NAME actualizado correctamente."
