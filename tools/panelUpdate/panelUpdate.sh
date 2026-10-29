#!/bin/bash

# =============================================================================
# Aether Panel - Script de Actualización Automática
# Este script es ejecutado por el panel desde la interfaz web.
# =============================================================================

set -e

readonly PROJECT_NAME="Aether Panel"
readonly INSTALL_DIR="/opt/skypanel"
# rama estable: la que se usa cuando la instalación no indica otra
readonly DEFAULT_GIT_BRANCH="main"
# Opciones guardadas por el instalador (install.sh 4.1+): de ahí sale la rama instalada
readonly INSTALLER_STATE="/etc/SkyPanel/installer.conf"

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

# La rama que usa este servidor: AETHER_BRANCH si se indica, la que guardó el instalador o la
# que tiene el repositorio. Una actualización nunca cambia de rama: antes se forzaba dev2.0
# y una instalación en otra rama se pasaba a dev2.0 (o a main) sin avisar.
GIT_BRANCH="${AETHER_BRANCH:-}"
if [[ -z "$GIT_BRANCH" && -r "$INSTALLER_STATE" ]]; then
    GIT_BRANCH=$(sed -n 's/^GIT_BRANCH=//p' "$INSTALLER_STATE" | tail -n 1)
fi
if [[ -z "$GIT_BRANCH" ]]; then
    GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)
fi
if [[ -z "$GIT_BRANCH" || "$GIT_BRANCH" == "HEAD" ]]; then
    GIT_BRANCH="$DEFAULT_GIT_BRANCH"
fi
if [[ ! "$GIT_BRANCH" =~ ^[A-Za-z0-9._/-]+$ ]]; then
    echo "[✗] Nombre de rama inválido: $GIT_BRANCH"
    exit 1
fi
echo "[INFO] Rama: $GIT_BRANCH"

# Preservar la configuración de nodo esclavo (Daemon only) si está activada
# para que los cambios personalizados sobrevivan al git pull que revierte el yml.
IS_SLAVE=false
SLAVE_TOKEN_PUBLIC=""
if grep -qE '^[[:space:]]*- SKYPANEL_PANEL_ENABLE=false' docker-compose.yml 2>/dev/null; then
    IS_SLAVE=true
    SLAVE_TOKEN_PUBLIC=$(grep -oE 'SKYPANEL_TOKEN_PUBLIC=[^ ]+' docker-compose.yml 2>/dev/null | head -1 | cut -d= -f2-)
    echo "[INFO] Nodo esclavo detectado: se preservará la configuración de daemon en el yml."
fi

# Conservar los archivos propios de este servidor (contraseñas en .env) para que
# ni el git checkout ni el git pull los reviertan o borren.
readonly LOCAL_FILES=(.env)
LOCAL_BACKUP_DIR=$(mktemp -d)
for f in "${LOCAL_FILES[@]}"; do
    if [[ -f "$f" ]]; then
        cp -p "$f" "$LOCAL_BACKUP_DIR/"
    fi
done
restore_local_files() {
    for f in "${LOCAL_FILES[@]}"; do
        if [[ -f "$LOCAL_BACKUP_DIR/$f" ]]; then
            cp -p "$LOCAL_BACKUP_DIR/$f" "$f"
        fi
    done
    rm -rf "$LOCAL_BACKUP_DIR"
}
trap restore_local_files EXIT

echo "[INFO] Obteniendo últimos cambios de GitHub..."
git checkout -- . 2>/dev/null || true
git fetch origin
if ! git rev-parse --verify --quiet "origin/$GIT_BRANCH" >/dev/null; then
    echo "[✗] La rama $GIT_BRANCH no existe en GitHub. No se actualiza nada."
    exit 1
fi
git checkout "$GIT_BRANCH"
git pull --ff-only origin "$GIT_BRANCH"

restore_local_files
trap - EXIT
echo "[✓] Archivos locales del servidor conservados: ${LOCAL_FILES[*]}"

# Re-aplicar configuración de esclavo sobre el yml recién actualizado
if [[ "$IS_SLAVE" = "true" ]]; then
    sed -i 's|^[[:space:]]*# - SKYPANEL_PANEL_ENABLE=false|      - SKYPANEL_PANEL_ENABLE=false|' docker-compose.yml
    if [[ -n "$SLAVE_TOKEN_PUBLIC" ]]; then
        sed -i "s|^[[:space:]]*# - SKYPANEL_TOKEN_PUBLIC=.*|      - SKYPANEL_TOKEN_PUBLIC=${SLAVE_TOKEN_PUBLIC}|" docker-compose.yml
    fi
    echo "[✓] Configuración de nodo esclavo re-aplicada en docker-compose.yml."
fi

# (MariaDB ya se publica en 127.0.0.1:3307 en docker-compose.yml: no choca con un MySQL
# del host, así que no hay que reasignar nada.)

# Primero se construye, con el panel aún en marcha: la compilación tarda varios minutos y,
# si falla, el panel sigue funcionando con la versión anterior.
echo "[INFO] Construyendo la nueva imagen (esto puede tardar varios minutos)..."
COMPOSE_PROFILES=local-db $DOCKER_COMPOSE build 2>&1

echo "[INFO] Reiniciando con la versión nueva..."
COMPOSE_PROFILES=local-db $DOCKER_COMPOSE up -d 2>&1

# Cada compilación deja su caché y nada la borra: tras unas cuantas actualizaciones ocupa
# decenas de GB y el disco se llena. Se dejan 3 GB para que la próxima compile rápido.
echo "[INFO] Liberando la caché de compilaciones anteriores..."
docker builder prune -f --max-used-space 3GB >/dev/null 2>&1 \
    || docker builder prune -f --keep-storage 3GB >/dev/null 2>&1 \
    || echo "[AVISO] No se pudo liberar la caché de compilación (docker builder prune)."

echo "[✓] $PROJECT_NAME actualizado correctamente."
