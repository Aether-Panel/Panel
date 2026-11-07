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
    # Solo una línea activa: la comentada del repositorio es un ejemplo (http://<MASTER-IP>:8080/...),
    # y activarla dejaría al nodo sin la clave del panel (la variable gana a config.json)
    SLAVE_TOKEN_PUBLIC=$(grep -E '^[[:space:]]*- SKYPANEL_TOKEN_PUBLIC=' docker-compose.yml 2>/dev/null | head -1 | sed 's/.*SKYPANEL_TOKEN_PUBLIC=//; s/[[:space:]].*//')
    case "$SLAVE_TOKEN_PUBLIC" in *"<"*) SLAVE_TOKEN_PUBLIC="" ;; esac
    echo "[INFO] Nodo esclavo detectado: se preservará la configuración de daemon en el yml."
fi

# token.public del config.json del nodo (el mismo archivo que usa install.sh), o nada.
# Con jq si está; si no, un sed que tolera espacios y saltos de línea.
config_token_public() {
    local file="storage/skypanel-data/config.json"
    if [[ -f "storage/skypanel-config/config.json" ]]; then
        file="storage/skypanel-config/config.json"
    fi
    [[ -r "$file" ]] || return 0
    if command -v jq &>/dev/null; then
        jq -r '.token.public // empty | strings' "$file" 2>/dev/null || true
    else
        tr -d '\n\r' <"$file" \
            | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*{[^}]*"public"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
            | head -1
    fi
}

# token_url_answers URL: 0 si la dirección devuelve la clave del panel (JSON con un array
# "keys"), 1 si no responde o responde otra cosa, 2 si no hay curl para comprobarlo.
token_url_answers() {
    local body
    command -v curl &>/dev/null || return 2
    body=$(curl -fsS --max-time 10 "$1" 2>/dev/null) || return 1
    if command -v jq &>/dev/null; then
        jq -e '.keys | type == "array"' <<<"$body" &>/dev/null || return 1
    else
        grep -qE '"keys"[[:space:]]*:[[:space:]]*\[' <<<"$body" || return 1
    fi
}

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
    # La variable gana a config.json: si config.json ya tiene la dirección de la clave del panel
    # (nodo unido con install.sh --join), una línea vieja dejaría al nodo rechazando al panel.
    # Igual que enable_node_compose en install.sh, la línea se queda comentada, pero solo si es
    # la misma dirección o si la de config.json responde: en un nodo montado a mano la buena
    # puede ser la de la línea y la de config.json una vieja.
    KEEP_TOKEN_OVERRIDE=true
    if [[ -n "$SLAVE_TOKEN_PUBLIC" ]]; then
        CONFIG_TOKEN_PUBLIC=$(config_token_public)
        if [[ -n "$CONFIG_TOKEN_PUBLIC" ]]; then
            if [[ "$CONFIG_TOKEN_PUBLIC" == "$SLAVE_TOKEN_PUBLIC" ]]; then
                KEEP_TOKEN_OVERRIDE=false
                echo "[INFO] No se re-aplica SKYPANEL_TOKEN_PUBLIC: config.json (token.public) ya tiene la misma dirección."
            else
                TOKEN_CHECK=0
                token_url_answers "$CONFIG_TOKEN_PUBLIC" || TOKEN_CHECK=$?
                case "$TOKEN_CHECK" in
                    0) KEEP_TOKEN_OVERRIDE=false
                       echo "[INFO] No se re-aplica SKYPANEL_TOKEN_PUBLIC: la dirección de la clave del panel sale de config.json (token.public), que responde." ;;
                    2) echo "[AVISO] Se re-aplica SKYPANEL_TOKEN_PUBLIC: no hay curl para comprobar la dirección de config.json (token.public)." ;;
                    *) echo "[AVISO] Se re-aplica SKYPANEL_TOKEN_PUBLIC: la dirección de config.json (token.public) no devuelve la clave del panel." ;;
                esac
            fi
        fi
    fi
    if [[ -n "$SLAVE_TOKEN_PUBLIC" && "$KEEP_TOKEN_OVERRIDE" == "false" ]]; then
        sed -i 's|^\([[:space:]]*\)- SKYPANEL_TOKEN_PUBLIC=|\1# - SKYPANEL_TOKEN_PUBLIC=|' docker-compose.yml
    elif [[ -n "$SLAVE_TOKEN_PUBLIC" ]]; then
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

# Las imágenes nuestras que se quedan sin etiqueta y sin uso: compilaciones anteriores del panel
# (unos 400 MB cada una) y versiones anteriores de ghcr.io/aether-panel/*. Se reconocen por el
# entrypoint del panel o por la etiqueta source del repositorio; las demás imágenes del equipo
# no son nuestras y no se tocan. docker image rm se niega si algún contenedor aún la usa.
echo "[INFO] Borrando imágenes antiguas del panel..."
for img in $(docker images -f dangling=true -q 2>/dev/null || true); do
    entrypoint=$(docker image inspect -f '{{json .Config.Entrypoint}}' "$img" 2>/dev/null || true)
    source=$(docker image inspect -f '{{index .Config.Labels "org.opencontainers.image.source"}}' "$img" 2>/dev/null || true)
    case "$entrypoint|$source" in
        *'/SkyPanel/bin/entrypoint.sh'*|*'|https://github.com/Aether-Panel/'*)
            docker image rm "$img" >/dev/null 2>&1 || true ;;
    esac
done

echo "[✓] $PROJECT_NAME actualizado correctamente."
