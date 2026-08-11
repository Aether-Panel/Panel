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

echo "[INFO] Obteniendo últimos cambios de GitHub..."
git checkout -- docker-compose.yml 2>/dev/null || true
git fetch origin
git checkout "$GIT_BRANCH" 2>/dev/null || git checkout main
git pull

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
