#!/usr/bin/env bash

###############################################################################
# Aether Panel - Script de Instalación Automática (Docker)
# Versión: 4.2.1
# Repositorio: https://github.com/Aether-Panel/Panel.git
# Nota: Aether Panel es el nombre oficial. SkyPanel es el nombre en clave.
# Soporte: Solo instalación vía Docker. No existe modo nativo.
#
# Qué deja instalado (debe coincidir con tools/panelUpdate/panelUpdate.sh):
#   /opt/skypanel                      clon del repositorio (docker compose)
#   /opt/skypanel/.env                 contraseñas de la base de datos (solo root)
#   /opt/skypanel/docker-compose.override.yml
#                                      solo si cambias puertos o usas BD externa
#   /opt/skypanel/storage/             TODOS los datos (MariaDB, servidores,
#                                      backups, extensiones, logs, llave SFTP,
#                                      config.json del panel)
#   /etc/SkyPanel/installer.conf       opciones de esta instalación (sin secretos)
#   /etc/SkyPanel/credenciales.txt     contraseñas generadas (solo root)
#
# Volver a ejecutar el instalador es seguro: no borra datos, no regenera
# contraseñas de una base de datos existente y no recrea la cuenta admin.
###############################################################################

set -Eeuo pipefail
umask 022

# =============================================================================
# CONFIGURACIÓN Y VARIABLES GLOBALES
# =============================================================================

readonly SCRIPT_VERSION="4.2.1"
readonly SCRIPT_BUILD="20261026-hardening"
readonly PROJECT_NAME="Aether Panel"
readonly PROJECT_CODENAME="SkyPanel"
readonly INSTALL_URL="https://install.aetherpanel.es/install.sh"
# El repositorio se puede cambiar (forks, pruebas) con AETHER_REPO
GITHUB_REPO="${AETHER_REPO:-https://github.com/Aether-Panel/Panel.git}"
# Rama por defecto de las instalaciones. Se puede cambiar con --branch o AETHER_BRANCH.
# rama estable: una instalación nueva usa main salvo que se pida otra (-b / AETHER_BRANCH);
# al volver a ejecutar el instalador se mantiene la rama ya instalada
readonly DEFAULT_GIT_BRANCH="main"

# Directorios (usando nombre en clave SkyPanel con mayúsculas).
# INSTALL_DIR es fijo: la actualización desde el panel (internal/update) y
# tools/panelUpdate/panelUpdate.sh usan /opt/skypanel.
readonly DEFAULT_INSTALL_DIR="/opt/skypanel"
readonly DEFAULT_DATA_DIR="/var/lib/SkyPanel"   # legado: instalaciones antiguas fuera de Docker
readonly DEFAULT_CONFIG_DIR="/etc/SkyPanel"
readonly DEFAULT_LOG_DIR="/var/log/SkyPanel"    # legado
readonly DEFAULT_WEB_DIR="/var/www/SkyPanel"    # legado

readonly PANEL_CONTAINER="skypanel"
readonly MYSQL_CONTAINER="skypanel-mysql"
readonly SERVERS_NETWORK="skypanel-servers"
# Copias de seguridad que hace el instalador: fuera de $INSTALL_DIR para que una reinstalación
# o desinstalación no se las lleve
readonly BACKUP_ROOT="/var/backups/aether-panel"

# Puertos por defecto (docker-compose.yml: "8080:8080", "5657:5657", MariaDB 127.0.0.1:3307)
readonly DEFAULT_PANEL_PORT="8080"
readonly DEFAULT_SFTP_PORT="5657"
readonly DEFAULT_MYSQL_HOST_PORT="3307"
readonly PANEL_INTERNAL_PORT="8080"

# "ports: !override" en docker-compose.override.yml necesita Compose >= 2.24.4
readonly MIN_COMPOSE_FOR_OVERRIDE="2.24.4"
readonly OVERRIDE_MARKER="# Generado por el instalador de Aether Panel"

# Huella de la llave GPG oficial de los paquetes de Docker
readonly DOCKER_GPG_FINGERPRINT="9DC858229FC7DD38854AE2D88D81803C0EBFCD88"

# Contraseñas por defecto conocidas (repositorio público): nunca deben quedarse en producción
readonly KNOWN_DEFAULT_DB_PASSWORDS=" secreto_root_super_seguro secreto_base_de_datos_super_seguro skypanel_secret skypanel_password skypanel_root_password CHANGE_ME "

# Colores (solo si la salida es una terminal y no se pidió NO_COLOR)
if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
    RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'
    BLUE=$'\033[0;34m'; CYAN=$'\033[0;36m'; BOLD=$'\033[1m'; NC=$'\033[0m'
else
    RED=""; GREEN=""; YELLOW=""; BLUE=""; CYAN=""; BOLD=""; NC=""
fi
readonly RED GREEN YELLOW BLUE CYAN BOLD NC

INSTALL_DIR="$DEFAULT_INSTALL_DIR"
DATA_DIR="$DEFAULT_DATA_DIR"
CONFIG_DIR="$DEFAULT_CONFIG_DIR"
LOG_DIR="$DEFAULT_LOG_DIR"
WEB_DIR="$DEFAULT_WEB_DIR"
STATE_FILE="$CONFIG_DIR/installer.conf"
CREDENTIALS_FILE="$CONFIG_DIR/credenciales.txt"
LOCK_FILE="/run/aetherpanel-install.lock"

# Convierte "1/true/yes/si" en true
is_truthy() {
    case "${1:-}" in
        1|true|TRUE|True|yes|YES|si|SI|sí|y|Y) return 0 ;;
        *) return 1 ;;
    esac
}
truthy_env() { if is_truthy "${1:-}"; then echo true; else echo false; fi; }

# -----------------------------------------------------------------------------
# Opciones. Prioridad: argumentos > variables de entorno AETHER_* >
# instalación previa (installer.conf) > valores por defecto.
# -----------------------------------------------------------------------------
GIT_BRANCH="${AETHER_BRANCH:-}"
DOMAIN="${AETHER_DOMAIN:-}"
USE_SSL="${AETHER_SSL:-}"            # true | false | "" (sin decidir)
SSL_EMAIL="${AETHER_EMAIL:-}"
PANEL_PORT="${AETHER_PORT:-}"
SFTP_PORT="${AETHER_SFTP_PORT:-}"
PANEL_BIND="${AETHER_BIND:-}"        # IP del host donde se publica el puerto web
SERVER_IP="${AETHER_IP:-}"
MASTER_URL="${AETHER_MASTER_URL:-}"
NO_DOMAIN=false

DB_MODE="${AETHER_DB_MODE:-}"
DB_HOST="${AETHER_DB_HOST:-}"
DB_PORT="${AETHER_DB_PORT:-}"
DB_NAME="${AETHER_DB_NAME:-}"
DB_USER="${AETHER_DB_USER:-}"
DB_PASSWORD="${AETHER_DB_PASSWORD:-}"
DB_ROOT_PASSWORD="${AETHER_DB_ROOT_PASSWORD:-}"
MYSQL_HOST_PORT=""

# Cuenta administrador inicial
ADMIN_NAME="${AETHER_ADMIN_NAME:-admin}"
ADMIN_EMAIL="${AETHER_ADMIN_EMAIL:-}"
ADMIN_PASSWORD="${AETHER_ADMIN_PASSWORD:-}"
CREATE_ADMIN=""                      # true | false | "" (decidir según el caso)
CREATE_ADMIN_ONLY=false
REPLACE_ADMIN=false
ADMIN_CREATED=false
ADMIN_PASSWORD_GENERATED=false

# Funciones del panel que se guardan en su config.json (todas opcionales)
TURNSTILE_SITE_KEY="${AETHER_TURNSTILE_SITE_KEY:-}"
TURNSTILE_SECRET_KEY="${AETHER_TURNSTILE_SECRET_KEY:-}"
CF_DNS_TOKEN="${AETHER_CF_DNS_TOKEN:-}"
CF_ZONE_ID="${AETHER_CF_ZONE_ID:-}"
DNS_BASE_DOMAIN="${AETHER_DNS_BASE_DOMAIN:-}"
DNS_TARGET="${AETHER_DNS_TARGET:-}"
DNS_AUTO_ASSIGN="${AETHER_DNS_AUTO_ASSIGN:-}"
S3_ENDPOINT="${AETHER_S3_ENDPOINT:-}"
S3_REGION="${AETHER_S3_REGION:-}"
S3_BUCKET="${AETHER_S3_BUCKET:-}"
S3_ACCESS_KEY="${AETHER_S3_ACCESS_KEY:-}"
S3_SECRET_KEY="${AETHER_S3_SECRET_KEY:-}"
S3_PREFIX="${AETHER_S3_PREFIX:-}"
S3_INSECURE="${AETHER_S3_INSECURE:-}"
CURSEFORGE_KEY="${AETHER_CURSEFORGE_KEY:-}"
LOG_RETENTION_DAYS="${AETHER_LOG_RETENTION_DAYS:-}"

# Modos
FORCE=$(truthy_env "${AETHER_FORCE:-}")
AUTO_MODE=$(truthy_env "${AETHER_YES:-}")
NON_INTERACTIVE=$(truthy_env "${AETHER_NONINTERACTIVE:-}")
DRY_RUN=$(truthy_env "${AETHER_DRY_RUN:-}")
UPDATE_MODE=false
APPLY_PANEL_URL=false
REINSTALL_MODE=false
SKIP_BACKUP=false
DB_ADMIN_USER="${AETHER_DB_ADMIN_USER:-}"
DB_ADMIN_PASSWORD="${AETHER_DB_ADMIN_PASSWORD:-}"
INSTALL_STATE="absent"          # absent | stopped | running
FOREIGN_CONTAINER=false         # un contenedor "skypanel" que no es de esta instalación
ROLLBACK_ON_FAILURE=false       # en una actualización, un panel que no arranca vuelve atrás
PANEL_START_FAILED=false
PRE_UPDATE_COMMIT=""
LAST_DB_BACKUP=""
ROLLBACK_IMAGE=""
DBS_MODE=""; DBS_NAME=""; DBS_USER=""; DBS_HOST=""; DBS_PORT=""
ROTATE_DB_PASSWORDS=false
UNINSTALL_MODE=""
DEBUG=$(truthy_env "${AETHER_DEBUG:-}")

# Flags para omitir pasos
SKIP_DEPENDENCIES=false
SKIP_DOCKER=false
SKIP_FIREWALL=false
SKIP_NGINX=false

# Estado detectado
OS_TYPE=""; OS_LIKE=""; OS_VERSION=""; OS_CODENAME=""; OS_UBUNTU_CODENAME=""; OS_PRETTY=""
OS_FAMILY=""; DOCKER_DISTRO=""; DOCKER_CODENAME=""; ARCH=""
HAS_SYSTEMD=false
IN_CONTAINER=false
DC=()                       # comando docker compose (array)
COMPOSE_VERSION=""
DOCKER_READY=false
EXISTING_INSTALL=false
IS_NODE_ONLY=false
LOCAL_IP=""; PUBLIC_IP=""
SSH_PORTS=()
SSL_FAILED=false
DOMAIN_PROXIED=false
DOMAIN_SERVER_IP=""
PANEL_NOT_READY=false
NGINX_CONFIGURED=false
DEFAULT_DB_PASSWORDS_IN_USE=false
MASTER_URL_EXPLICIT=false
MASTER_URL_WRITTEN=false
CURRENT_STEP=""
STEP_NO=0
ENV_BACKUP=""
TMP_FILES=()

# Instalación previa (installer.conf o deducida)
PREV_DOMAIN=""; PREV_USE_SSL=""; PREV_SSL_EMAIL=""; PREV_PANEL_PORT=""; PREV_SFTP_PORT=""
PREV_PANEL_BIND=""; PREV_SERVER_IP=""; PREV_MASTER_URL=""; PREV_GIT_BRANCH=""; PREV_DB_MODE=""
PREV_MYSQL_HOST_PORT=""

# Cambios pendientes en el config.json del panel (clave, tipo, valor, secreto)
CFG_KEYS=(); CFG_TYPES=(); CFG_VALS=(); CFG_SECRET=()

# Detección de entorno interactivo
# - Piped (curl | bash): stdin no es TTY -> no se puede preguntar (modo automático)
# - -y / --yes:          modo automático pero con TTY -> se pregunta lo mínimo
# - --non-interactive:   nunca se pregunta nada (automatización)
CAN_PROMPT=false
if [[ -t 0 ]]; then
    CAN_PROMPT=true
else
    AUTO_MODE=true
fi

# =============================================================================
# FUNCIONES DE UTILIDAD
# =============================================================================

log_info()    { printf '%s[INFO]%s %s\n' "$BLUE" "$NC" "$*"; }
log_success() { printf '%s[✓]%s %s\n' "$GREEN" "$NC" "$*"; }
log_warning() { printf '%s[!]%s %s\n' "$YELLOW" "$NC" "$*" >&2; }
log_error()   { printf '%s[✗]%s %s\n' "$RED" "$NC" "$*" >&2; }
log_debug() {
    if [[ "$DEBUG" == "true" ]]; then
        printf '%s[DEBUG]%s %s\n' "$CYAN" "$NC" "$*" >&2
    fi
    return 0
}
die() { log_error "$*"; exit 1; }

print_banner() {
    # Solo se limpia la pantalla en una sesión interactiva real (clear falla sin TERM)
    if [[ "$CAN_PROMPT" == "true" && -t 1 && -n "${TERM:-}" && "$DEBUG" != "true" ]]; then
        clear 2>/dev/null || true
    fi
    printf '%s' "$BOLD"
    echo "============================================================"
    echo "  $PROJECT_NAME - Instalador Automático (Docker)"
    echo "  ($PROJECT_CODENAME - nombre en clave)"
    echo "  Versión: $SCRIPT_VERSION (build: $SCRIPT_BUILD)"
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  MODO SIMULACIÓN (--dry-run): no se cambiará nada"
    fi
    echo "============================================================"
    printf '%s\n' "$NC"
}

print_step() {
    STEP_NO=$((STEP_NO + 1))
    CURRENT_STEP="$1"
    echo ""
    printf '%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n' "$CYAN" "$NC"
    printf '%s► [%d] %s%s\n' "$BOLD" "$STEP_NO" "$1" "$NC"
    printf '%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n' "$CYAN" "$NC"
}

# Muestra un comando de forma segura para copiar/pegar
quote_cmd() {
    local out="" a
    for a in "$@"; do out+="$(printf '%q' "$a") "; done
    printf '%s' "${out% }"
}

# run CMD...: ejecuta un comando que CAMBIA el sistema (en --dry-run solo lo muestra).
# Nunca pases secretos como argumentos: usa variables de entorno o stdin.
run() {
    if [[ "$DRY_RUN" == "true" ]]; then
        printf '%s[dry-run]%s %s\n' "$CYAN" "$NC" "$(quote_cmd "$@")"
        return 0
    fi
    log_debug "+ $(quote_cmd "$@")"
    "$@"
}

# run_in DIR CMD...: igual que run, pero dentro de DIR (docker compose busca allí sus archivos)
run_in() {
    local dir="$1"; shift
    if [[ "$DRY_RUN" == "true" ]]; then
        printf '%s[dry-run]%s (cd %s) %s\n' "$CYAN" "$NC" "$dir" "$(quote_cmd "$@")"
        return 0
    fi
    log_debug "+ (cd $dir) $(quote_cmd "$@")"
    (cd "$dir" && "$@")
}

# docker compose en el directorio de instalación (igual que panelUpdate.sh)
dc() { run_in "$INSTALL_DIR" env COMPOSE_PROFILES=local-db "${DC[@]}" "$@"; }

# write_file RUTA MODO [show]: escribe stdin en RUTA de forma atómica.
# En --dry-run muestra el contenido solo si se pide "show" (nunca para secretos).
write_file() {
    local path="$1" mode="$2" show="${3:-}" tmp
    if [[ "$DRY_RUN" == "true" ]]; then
        printf '%s[dry-run]%s se escribiría %s (permisos %s)\n' "$CYAN" "$NC" "$path" "$mode"
        if [[ "$show" == "show" ]]; then
            sed 's/^/    | /'
        else
            cat >/dev/null
        fi
        return 0
    fi
    mkdir -p "$(dirname "$path")"
    tmp=$(mktemp "${path}.tmp.XXXXXX")
    TMP_FILES+=("$tmp")
    cat >"$tmp"
    chmod "$mode" "$tmp"
    mv -f "$tmp" "$path"
}

# Contraseña aleatoria de 32 caracteres hexadecimales (128 bits).
# Sin "tr | head" para evitar SIGPIPE con pipefail.
gen_secret() {
    local raw
    raw=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
    printf '%s' "${raw:0:32}"
}

# version_ge A B -> A >= B
version_ge() {
    local lowest
    lowest=$(printf '%s\n%s\n' "$2" "$1" | sort -V)
    lowest=${lowest%%$'\n'*}
    [[ "$lowest" == "$2" ]]
}

contains_word() { [[ " $1 " == *" $2 "* ]]; }

# Longitud en bytes (bcrypt, y por tanto el panel, limita la contraseña a 72 bytes)
byte_len() {
    local LC_ALL=C
    printf '%s' "${#1}"
}

# -----------------------------------------------------------------------------
# Preguntas (los valores se devuelven en una variable, nunca por stdout, para
# que los mensajes de error no acaben mezclados con la respuesta)
# -----------------------------------------------------------------------------

# prompt_read VAR "texto" [defecto] [secreto]
prompt_read() {
    local __var="$1" __text="$2" __def="${3:-}" __secret="${4:-false}" __ans="" __p
    __p="$__text"
    if [[ -n "$__def" && "$__secret" != "true" ]]; then __p+=" [$__def]"; fi
    if [[ "$__secret" == "true" ]]; then
        IFS= read -r -s -p "$__p: " __ans || __ans=""
        echo >&2
    else
        IFS= read -r -p "$__p: " __ans || __ans=""
    fi
    printf -v "$__var" '%s' "${__ans:-$__def}"
}

interactive_manual() { [[ "$CAN_PROMPT" == "true" && "$AUTO_MODE" != "true" ]]; }

# ask_yes_no VAR "pregunta" [yes|no]
ask_yes_no() {
    local __var="$1" __q="$2" __def="${3:-no}" __r="" __i
    if ! interactive_manual; then
        printf -v "$__var" '%s' "$__def"
        return 0
    fi
    for __i in 1 2 3 4 5; do
        prompt_read __r "$__q [y/n] (defecto: $__def)" ""
        __r=${__r:-$__def}
        case "$__r" in
            [YySs]|[Yy][Ee][Ss]|[Ss][IiÍí]) printf -v "$__var" yes; return 0 ;;
            [Nn]|[Nn][Oo]) printf -v "$__var" no; return 0 ;;
            *) log_warning "Responde 'y' (sí) o 'n' (no) [$__i/5]" ;;
        esac
    done
    printf -v "$__var" '%s' "$__def"
}

# ask_input VAR "pregunta" defecto [sensitive]
#   sensitive=true -> se pregunta también en modo -y (si hay terminal)
ask_input() {
    local __var="$1" __q="$2" __def="${3:-}" __sensitive="${4:-false}"
    # (4.0.0 comparaba con "true" pero se llamaba con "sensitive": en -y nunca preguntaba)
    if [[ "$__sensitive" == "sensitive" ]]; then __sensitive=true; fi
    if [[ "$CAN_PROMPT" != "true" ]] || { [[ "$AUTO_MODE" == "true" ]] && [[ "$__sensitive" != "true" ]]; }; then
        printf -v "$__var" '%s' "$__def"
        return 0
    fi
    prompt_read "$__var" "$__q" "$__def"
}

# ask_password VAR "pregunta" min_len allow_empty
#   allow_empty=true -> una respuesta vacía deja VAR vacía (el llamador genera una segura)
ask_password() {
    local __var="$1" __q="$2" __min="${3:-1}" __allow_empty="${4:-false}" __a="" __b="" __i
    if [[ "$CAN_PROMPT" != "true" ]]; then
        printf -v "$__var" '%s' ""
        return 0
    fi
    for __i in 1 2 3 4 5; do
        prompt_read __a "$__q" "" true
        if [[ -z "$__a" ]]; then
            if [[ "$__allow_empty" == "true" ]]; then
                printf -v "$__var" '%s' ""
                return 0
            fi
            log_error "La contraseña no puede estar vacía (intento $__i/5)"
            continue
        fi
        if (( ${#__a} < __min )); then
            log_error "La contraseña debe tener al menos $__min caracteres (intento $__i/5)"
            continue
        fi
        prompt_read __b "Confirma la contraseña" "" true
        if [[ "$__a" == "$__b" ]]; then
            printf -v "$__var" '%s' "$__a"
            return 0
        fi
        log_error "Las contraseñas no coinciden (intento $__i/5)"
    done
    die "Demasiados intentos fallidos"
}

# -----------------------------------------------------------------------------
# Validaciones
# -----------------------------------------------------------------------------

validate_port() {
    local port="$1"
    [[ "$port" =~ ^[0-9]+$ ]] && (( 10#$port >= 1 && 10#$port <= 65535 ))
}

validate_ip() {
    local ip="$1" octet
    [[ "$ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || return 1
    local IFS='.'
    for octet in $ip; do
        (( 10#$octet <= 255 )) || return 1
    done
    return 0
}

validate_domain() {
    [[ "$1" =~ ^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$ ]]
}

validate_email() {
    [[ "$1" =~ ^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$ ]]
}

# Contraseñas de la BD local: se usan en la URL de conexión, en la interpolación de
# docker compose y en el healthcheck de MariaDB (sh), así que solo caracteres seguros.
validate_local_db_password() {
    [[ "$1" =~ ^[A-Za-z0-9._+=,-]{8,}$ ]]
}
# BD externa: se permite casi todo menos lo que rompe .env / docker compose
validate_external_db_password() {
    [[ -n "$1" && ! "$1" =~ [[:space:]\$\"\'\\\`#] ]]
}

is_private_ip() {
    local ip="$1"
    [[ "$ip" =~ ^10\. ]] && return 0
    [[ "$ip" =~ ^127\. ]] && return 0
    [[ "$ip" =~ ^192\.168\. ]] && return 0
    [[ "$ip" =~ ^169\.254\. ]] && return 0
    [[ "$ip" =~ ^172\.(1[6-9]|2[0-9]|3[0-1])\. ]] && return 0
    [[ "$ip" =~ ^100\.(6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\. ]] && return 0   # CGNAT
    return 1
}

# =============================================================================
# LIMPIEZA Y ERRORES
# =============================================================================

on_error() {
    local rc=$? line="$1" cmd="$2"
    # Solo informa el proceso principal (evita mensajes duplicados desde subshells)
    if (( BASH_SUBSHELL > 0 )); then return 0; fi
    echo "" >&2
    log_error "Error (código $rc) en la línea $line: $cmd"
    if [[ -n "$CURRENT_STEP" ]]; then
        log_error "Paso: $CURRENT_STEP"
    fi
    log_error "No se ha borrado ningún dato. Corrige el problema y vuelve a ejecutar el"
    log_error "instalador: es seguro repetirlo (usa --debug para ver más detalle)."
}

on_exit() {
    local f
    # Si se interrumpió una sincronización de git, se devuelve el .env del servidor
    if [[ -n "$ENV_BACKUP" && -f "$ENV_BACKUP" ]]; then
        cp -p "$ENV_BACKUP" "$INSTALL_DIR/.env" 2>/dev/null || true
        log_warning ".env del servidor restaurado desde la copia de seguridad"
        rm -f "$ENV_BACKUP"
    fi
    for f in "${TMP_FILES[@]+"${TMP_FILES[@]}"}"; do
        rm -f "$f" 2>/dev/null || true
    done
}

trap 'on_error "$LINENO" "$BASH_COMMAND"' ERR
trap on_exit EXIT
trap 'echo; log_error "Interrumpido por el usuario"; exit 130' INT TERM

acquire_lock() {
    if [[ "$DRY_RUN" == "true" ]] || ! command -v flock >/dev/null 2>&1; then
        return 0
    fi
    exec 9>"$LOCK_FILE"
    if ! flock -n 9; then
        die "Ya hay otra ejecución del instalador en curso (bloqueo $LOCK_FILE)"
    fi
}

# =============================================================================
# DETECCIÓN DEL SISTEMA
# =============================================================================

require_root() {
    if [[ $EUID -eq 0 ]]; then return 0; fi
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "No eres root: la simulación puede no detectar todo"
        return 0
    fi
    log_error "Este script debe ejecutarse como root"
    echo ""
    echo "Usa uno de los siguientes métodos:"
    echo ""
    echo "  Método 1 (wizard manual interactivo - recomendado):"
    echo "    sudo su"
    echo "    bash -c \"\$(curl -fsSL $INSTALL_URL)\""
    echo ""
    echo "  Método 2 (modo automático - sin preguntas):"
    echo "    curl -fsSL $INSTALL_URL | sudo bash -s -- --non-interactive"
    echo ""
    echo "  Método 3 (descargar, revisar y ejecutar):"
    echo "    curl -fsSL $INSTALL_URL -o /tmp/install.sh"
    echo "    sudo bash /tmp/install.sh --help"
    echo ""
    echo "  Nota: Evita 'bash <(curl -s ...)' ya que el process substitution"
    echo "  (/dev/fd/N) no está disponible en todos los entornos."
    exit 1
}

detect_os() {
    print_step "Detectando sistema operativo"

    local info
    if [[ -r /etc/os-release ]]; then
        # Se lee en un subshell para no pisar variables del script (ID, VERSION...)
        # shellcheck disable=SC1091
        info=$(. /etc/os-release && printf '%s\n%s\n%s\n%s\n%s\n%s\n' \
            "${ID:-}" "${ID_LIKE:-}" "${VERSION_ID:-}" "${VERSION_CODENAME:-}" \
            "${UBUNTU_CODENAME:-}" "${PRETTY_NAME:-}")
        { read -r OS_TYPE; read -r OS_LIKE; read -r OS_VERSION; read -r OS_CODENAME
          read -r OS_UBUNTU_CODENAME; read -r OS_PRETTY; } <<<"$info"
    elif [[ -f /etc/redhat-release ]]; then
        OS_TYPE="rhel"
        OS_VERSION=$(sed -n 's/[^0-9]*\([0-9][0-9]*\(\.[0-9]*\)\?\).*/\1/p' /etc/redhat-release)
        OS_PRETTY=$(cat /etc/redhat-release)
    elif [[ -f /etc/debian_version ]]; then
        OS_TYPE="debian"
        OS_VERSION=$(cat /etc/debian_version)
    else
        die "No se pudo detectar el sistema operativo"
    fi

    case "$OS_TYPE" in
        ubuntu|linuxmint|pop|elementary|zorin|neon)
            OS_FAMILY="debian"; DOCKER_DISTRO="ubuntu"
            DOCKER_CODENAME="${OS_UBUNTU_CODENAME:-$OS_CODENAME}" ;;
        debian|raspbian)
            OS_FAMILY="debian"; DOCKER_DISTRO="debian"; DOCKER_CODENAME="$OS_CODENAME" ;;
        fedora)
            OS_FAMILY="fedora"; DOCKER_DISTRO="fedora" ;;
        rhel)
            OS_FAMILY="rhel"; DOCKER_DISTRO="rhel" ;;
        centos|rocky|alma|almalinux|ol)
            OS_FAMILY="rhel"; DOCKER_DISTRO="centos" ;;
        arch|manjaro|endeavouros|garuda|artix)
            # Docker y Compose vienen en los repositorios de la distribución
            OS_FAMILY="arch"; DOCKER_DISTRO="arch" ;;
        opensuse-leap|opensuse-tumbleweed|opensuse-slowroll|sles)
            OS_FAMILY="suse"; DOCKER_DISTRO="suse" ;;
        *)
            if contains_word "$OS_LIKE" ubuntu; then
                OS_FAMILY="debian"; DOCKER_DISTRO="ubuntu"; DOCKER_CODENAME="${OS_UBUNTU_CODENAME:-$OS_CODENAME}"
            elif contains_word "$OS_LIKE" debian; then
                OS_FAMILY="debian"; DOCKER_DISTRO="debian"; DOCKER_CODENAME="$OS_CODENAME"
            elif contains_word "$OS_LIKE" rhel || contains_word "$OS_LIKE" centos || contains_word "$OS_LIKE" fedora; then
                OS_FAMILY="rhel"; DOCKER_DISTRO="centos"
            elif contains_word "$OS_LIKE" arch; then
                OS_FAMILY="arch"; DOCKER_DISTRO="arch"
            elif contains_word "$OS_LIKE" suse; then
                OS_FAMILY="suse"; DOCKER_DISTRO="suse"
            else
                die "Sistema operativo no soportado: ${OS_PRETTY:-$OS_TYPE}. Funciona en Ubuntu, Debian, Fedora, RHEL/Rocky/Alma, Arch y openSUSE (y derivadas): hace falta un gestor de paquetes conocido (apt, dnf, pacman o zypper) para instalar Docker, Nginx y certbot."
            fi
            log_warning "Distribución no probada ($OS_TYPE); se trata como derivada de $DOCKER_DISTRO"
            ;;
    esac

    if [[ "$OS_FAMILY" == "debian" && -z "$DOCKER_CODENAME" ]] && command -v lsb_release >/dev/null 2>&1; then
        DOCKER_CODENAME=$(lsb_release -cs 2>/dev/null || true)
    fi

    # Versiones mínimas documentadas (docs/*/setup/01-installation.md)
    local major="${OS_VERSION%%.*}"
    if [[ "$major" =~ ^[0-9]+$ ]]; then
        case "$OS_TYPE" in
            # 20.04 y Debian 11 ya no reciben parches de seguridad, y Docker dejó de publicarse para ellas
            ubuntu) (( major >= 22 )) || log_warning "Ubuntu $OS_VERSION ya no tiene soporte; usa 22.04 o posterior" ;;
            debian) (( major >= 12 )) || log_warning "Debian $OS_VERSION ya no tiene soporte; usa Debian 12 o posterior" ;;
            rhel|centos|rocky|alma|almalinux) (( major >= 8 )) || log_warning "$OS_TYPE $OS_VERSION es anterior a 8 (no soportado)" ;;
        esac
    fi

    # Arquitectura: la imagen se construye para amd64 o arm64
    case "$(uname -m)" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) die "Arquitectura no soportada: $(uname -m) (se necesita amd64 o arm64)" ;;
    esac

    if [[ -d /run/systemd/system ]]; then
        HAS_SYSTEMD=true
    fi
    if [[ -f /.dockerenv || -f /run/.containerenv ]]; then
        IN_CONTAINER=true
    fi

    log_success "Detectado: ${OS_PRETTY:-$OS_TYPE $OS_VERSION} ($OS_FAMILY, $ARCH)"
    if [[ "$HAS_SYSTEMD" != "true" ]]; then
        log_warning "systemd no está activo: Docker y Nginx no se podrán iniciar/habilitar automáticamente"
    fi
    if [[ "$IN_CONTAINER" == "true" ]]; then
        log_warning "Parece que estás dentro de un contenedor: el panel necesita Docker del host"
    fi
}

svc() { # svc ACCIÓN SERVICIO (solo con systemd)
    if [[ "$HAS_SYSTEMD" == "true" ]]; then
        run systemctl "$@"
    else
        log_warning "Sin systemd: ejecuta tú mismo el equivalente a 'systemctl $*'"
    fi
}

check_resources() {
    print_step "Verificando recursos del sistema"

    local mem_mb=0 swap_mb=0 key value _unit
    if [[ -r /proc/meminfo ]]; then
        while read -r key value _unit; do
            case "$key" in
                MemTotal:) mem_mb=$((value / 1024)) ;;
                SwapTotal:) swap_mb=$((value / 1024)) ;;
            esac
        done </proc/meminfo
    fi

    if (( mem_mb < 1000 )); then
        log_warning "RAM: ${mem_mb}MB. Se necesita al menos 1GB (2GB recomendados)"
        confirm_or_exit "¿Deseas continuar igualmente?"
    elif (( mem_mb < 1900 )); then
        log_warning "RAM: ${mem_mb}MB. Se recomiendan 2GB (la construcción de la imagen usa mucha memoria)"
    else
        log_success "RAM: ${mem_mb}MB"
    fi
    if (( mem_mb + swap_mb < 3000 )); then
        log_warning "RAM + swap = $((mem_mb + swap_mb))MB: la compilación puede fallar por falta de memoria."
        log_info "Si ocurre, crea swap: fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile"
    fi

    # Espacio: el código y los datos van a /opt; las imágenes a /var/lib/docker
    local path avail checked=" "
    for path in "$(dirname "$INSTALL_DIR")" /var/lib/docker; do
        while [[ ! -d "$path" && "$path" != "/" ]]; do path=$(dirname "$path"); done
        contains_word "$checked" "$path" && continue
        checked+="$path "
        avail=$(df -Pm "$path" 2>/dev/null | awk 'NR==2 {print int($4/1024)}' || echo 0)
        avail=${avail:-0}
        if (( avail < 10 )); then
            log_warning "Espacio libre en $path: ${avail}GB. Se recomiendan al menos 10GB"
            confirm_or_exit "¿Deseas continuar igualmente?"
        else
            log_success "Espacio libre en $path: ${avail}GB"
        fi
    done
}

# En modo manual pregunta; en automático solo avisa y continúa
confirm_or_exit() {
    local ans
    if interactive_manual; then
        ask_yes_no ans "$1" "no"
        if [[ "$ans" != "yes" ]]; then exit 1; fi
    fi
    return 0
}

# Detecta docker / docker compose sin instalar nada
detect_docker() {
    DOCKER_READY=false
    DC=()
    COMPOSE_VERSION=""
    command -v docker >/dev/null 2>&1 || return 0
    docker info >/dev/null 2>&1 && DOCKER_READY=true

    # Se prefiere el plugin "docker compose" (v2); docker-compose v1 no es compatible
    if docker compose version >/dev/null 2>&1; then
        DC=(docker compose)
        COMPOSE_VERSION=$(docker compose version --short 2>/dev/null || true)
    elif command -v docker-compose >/dev/null 2>&1; then
        local v
        v=$(docker-compose version --short 2>/dev/null || true)
        v=${v#v}
        if [[ "${v%%.*}" =~ ^[0-9]+$ ]] && (( ${v%%.*} >= 2 )); then
            DC=(docker-compose)
            COMPOSE_VERSION="$v"
        else
            log_warning "docker-compose $v (v1) no es compatible; se instalará el plugin de Compose v2"
        fi
    fi
    COMPOSE_VERSION=${COMPOSE_VERSION#v}
}

ensure_docker_running() {
    detect_docker
    if ! command -v docker >/dev/null 2>&1; then
        if [[ "$DRY_RUN" == "true" ]]; then
            log_info "[dry-run] Docker no está instalado; se instalaría en una ejecución real"
            DC=(docker compose)
            return 0
        fi
        die "Docker no está instalado (quita --skip-docker / --skip-deps o instálalo a mano)"
    fi
    if [[ "$DOCKER_READY" != "true" ]]; then
        log_warning "Docker está instalado pero el daemon no responde; intentando iniciarlo..."
        svc enable --now docker || true
        local i
        if [[ "$DRY_RUN" == "true" ]]; then
            log_info "[dry-run] Docker no responde; en una ejecución real se esperaría a que arranque"
            if (( ${#DC[@]} == 0 )); then DC=(docker compose); fi
            return 0
        fi
        for i in 1 2 3 4 5 6 7 8 9 10; do
            if docker info >/dev/null 2>&1; then DOCKER_READY=true; break; fi
            sleep 2
        done
        if [[ "$DOCKER_READY" != "true" && "$DRY_RUN" != "true" ]]; then
            die "El daemon de Docker no está corriendo. Inícialo con: systemctl start docker"
        fi
    fi
    if (( ${#DC[@]} == 0 )); then
        if [[ "$SKIP_DEPENDENCIES" != "true" && "$SKIP_DOCKER" != "true" ]]; then
            install_compose_plugin
            detect_docker
        fi
        if (( ${#DC[@]} == 0 )); then
            if [[ "$DRY_RUN" == "true" ]]; then DC=(docker compose); return 0; fi
            die "Docker Compose v2 no está instalado (paquete docker-compose-plugin o docker-compose-v2)"
        fi
    fi
    log_success "Docker: $(docker --version 2>/dev/null | head -n1)"
    log_success "Docker Compose: ${DC[*]} ${COMPOSE_VERSION:-?}"
}

# ¿Qué ocupa un puerto TCP? -> free | ours | docker:NOMBRES | proc:NOMBRE | unknown
port_owner() {
    local port="$1" out names proc
    if ! command -v ss >/dev/null 2>&1; then echo unknown; return 0; fi
    out=$(ss -Hltnp "sport = :$port" 2>/dev/null || true)
    if [[ -z "$out" ]]; then echo free; return 0; fi
    if [[ "$DOCKER_READY" == "true" ]]; then
        names=$(docker ps --filter "publish=$port" --format '{{.Names}}' 2>/dev/null || true)
        names=${names//$'\n'/ }
        if [[ -n "$names" ]]; then
            if [[ " $names " == *" $PANEL_CONTAINER "* || " $names " == *" $MYSQL_CONTAINER "* ]]; then
                echo ours
            else
                echo "docker:$names"
            fi
            return 0
        fi
    fi
    proc=$(sed -n 's/.*users:(("\([^"]*\)".*/\1/p' <<<"$out")
    proc=${proc%%$'\n'*}
    echo "proc:${proc:-desconocido}"
}

# ask_port VARIABLE "Pregunta" DEFECTO OTRO_PUERTO: pide un puerto hasta que sea válido, distinto
# de OTRO_PUERTO, no reservado por el propio panel y libre (o ya usado por esta instalación)
ask_port() {
    local __var="$1" label="$2" def="$3" other="$4" p owner reason i
    for i in 1 2 3 4 5; do
        ask_input p "$label" "$def"
        reason=""
        if ! validate_port "$p"; then
            reason="debe ser un número entre 1 y 65535"
        elif [[ -n "$other" && "$p" == "$other" ]]; then
            reason="ya lo usa otro servicio del panel"
        elif [[ "$p" == "22" ]] || contains_word "$(detect_ssh_ports_list)" "$p"; then
            reason="es el puerto de SSH"
        elif [[ "$p" == "80" || "$p" == "443" ]] && [[ "$SKIP_NGINX" != "true" ]]; then
            reason="lo usa Nginx delante del panel"
        elif [[ "$p" == "$DEFAULT_MYSQL_HOST_PORT" || "$p" == "8081" ]]; then
            reason="lo usa la base de datos o Adminer del panel"
        else
            owner=$(port_owner "$p")
            case "$owner" in
                free|ours|unknown) ;;
                docker:*) reason="lo usa el contenedor ${owner#docker:}" ;;
                proc:*) reason="lo usa el proceso ${owner#proc:}" ;;
            esac
        fi
        if [[ -z "$reason" ]]; then
            printf -v "$__var" '%s' "$p"
            return 0
        fi
        log_error "Puerto $p: $reason"
    done
    die "No se indicó un puerto válido para: $label"
}

# Puertos SSH como lista (detect_ssh_ports llena SSH_PORTS)
detect_ssh_ports_list() {
    detect_ssh_ports >/dev/null 2>&1 || true
    echo "${SSH_PORTS[*]:-22}"
}

check_ports() {
    print_step "Comprobando puertos"
    # MariaDB se publica en 127.0.0.1:3307 (docker-compose.yml). Si está ocupado se usa otro.
    MYSQL_HOST_PORT="${MYSQL_HOST_PORT:-${PREV_MYSQL_HOST_PORT:-$DEFAULT_MYSQL_HOST_PORT}}"
    if ! command -v ss >/dev/null 2>&1; then
        log_warning "No está disponible 'ss' (iproute2): no se pueden comprobar los puertos"
        return 0
    fi

    local conflicts=0 port owner label entry
    local -a checks=("$PANEL_PORT:Panel web" "$SFTP_PORT:SFTP")
    if [[ "$SKIP_NGINX" != "true" ]]; then
        checks+=("80:HTTP (Nginx)")
        if [[ "$USE_SSL" == "true" ]]; then checks+=("443:HTTPS (Nginx)"); fi
    fi
    for entry in "${checks[@]}"; do
        port=${entry%%:*}; label=${entry#*:}
        owner=$(port_owner "$port")
        case "$owner" in
            free) log_success "Puerto $port libre ($label)" ;;
            ours) log_success "Puerto $port en uso por $PROJECT_NAME ($label)" ;;
            proc:nginx) if [[ "$port" == 80 || "$port" == 443 ]]; then log_success "Puerto $port en uso por Nginx ($label)"; else log_error "Puerto $port ocupado por nginx ($label)"; conflicts=$((conflicts + 1)); fi ;;
            unknown) log_warning "No se pudo comprobar el puerto $port" ;;
            *) log_error "Puerto $port ($label) ocupado por ${owner#*:}"; conflicts=$((conflicts + 1)) ;;
        esac
    done

    if [[ "$DB_MODE" == "local" ]]; then
        owner=$(port_owner "$MYSQL_HOST_PORT")
        if [[ "$owner" != free && "$owner" != ours && "$owner" != unknown ]]; then
            log_warning "El puerto $MYSQL_HOST_PORT del host (MariaDB del panel) está en uso por ${owner#*:}"
            for port in $(seq 3308 3399); do
                if [[ "$(port_owner "$port")" == free ]]; then
                    MYSQL_HOST_PORT="$port"
                    log_info "MariaDB del panel se publicará en 127.0.0.1:$MYSQL_HOST_PORT"
                    break
                fi
            done
        fi
    fi

    if (( conflicts > 0 )); then
        if [[ "$FORCE" == "true" ]]; then
            log_warning "Hay puertos ocupados; se continúa por --force"
        elif [[ "$DRY_RUN" == "true" ]]; then
            log_warning "[dry-run] Una instalación real se detendría aquí por los puertos ocupados"
        else
            die "Libera los puertos indicados o elige otros con --port / --sftp-port"
        fi
    fi
}

check_network() {
    if ! command -v curl >/dev/null 2>&1; then return 0; fi
    if ! curl -fsS --max-time 10 -o /dev/null https://github.com 2>/dev/null; then
        log_warning "No se pudo conectar con github.com: la descarga del código puede fallar"
    fi
}

detect_ssh_ports() {
    local ports=" " p out key value
    if [[ -n "${SSH_CONNECTION:-}" ]]; then
        p=${SSH_CONNECTION##* }
        if [[ "$p" =~ ^[0-9]+$ ]]; then ports+="$p "; fi
    fi
    if command -v sshd >/dev/null 2>&1; then
        out=$(sshd -T 2>/dev/null || true)
        while read -r key value; do
            if [[ "$key" == "port" && "$value" =~ ^[0-9]+$ ]] && ! contains_word "$ports" "$value"; then
                ports+="$value "
            fi
        done <<<"$out"
    fi
    if [[ "$ports" == " " ]]; then ports=" 22 "; fi
    read -r -a SSH_PORTS <<<"$ports"
}

# =============================================================================
# DETECCIÓN DE IP DEL SERVIDOR
# =============================================================================

fetch_public_ip() {
    local s ip
    command -v curl >/dev/null 2>&1 || return 1
    for s in https://ifconfig.me https://ipinfo.io/ip https://icanhazip.com https://checkip.amazonaws.com; do
        ip=$(curl --proto '=https' -4 -fsS --max-time 5 "$s" 2>/dev/null | tr -d '[:space:]') || continue
        if validate_ip "$ip" && ! is_private_ip "$ip"; then
            printf '%s' "$ip"
            return 0
        fi
    done
    return 1
}

detect_local_ip() {
    local ip=""
    if command -v ip >/dev/null 2>&1; then
        ip=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") {print $(i + 1); exit}}' || true)
    fi
    if ! validate_ip "$ip" && command -v hostname >/dev/null 2>&1; then
        ip=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
    fi
    if validate_ip "$ip"; then printf '%s' "$ip"; fi
}

detect_server_ip() {
    print_step "Detectando IP del servidor"

    LOCAL_IP=$(detect_local_ip)
    PUBLIC_IP=$(fetch_public_ip || true)

    if [[ -n "$SERVER_IP" ]]; then
        log_success "IP indicada: $SERVER_IP"
    elif [[ -z "$DOMAIN" && -n "$LOCAL_IP" ]]; then
        # Sin dominio se prioriza la IP local (instalaciones en red local)
        SERVER_IP="$LOCAL_IP"
        log_success "IP detectada (local): $SERVER_IP"
    elif [[ -n "$PUBLIC_IP" ]]; then
        SERVER_IP="$PUBLIC_IP"
        log_success "IP detectada (pública): $SERVER_IP"
    elif [[ -n "$LOCAL_IP" ]]; then
        SERVER_IP="$LOCAL_IP"
        log_success "IP detectada (local): $SERVER_IP"
    else
        SERVER_IP="127.0.0.1"
        log_warning "No se pudo detectar la IP, usando localhost (indícala con --ip)"
    fi

    if [[ -n "$PUBLIC_IP" && "$PUBLIC_IP" != "$SERVER_IP" ]]; then
        log_info "IP pública: $PUBLIC_IP (usa --ip $PUBLIC_IP si el panel debe ser accesible desde Internet)"
    fi
}

# =============================================================================
# INSTALACIÓN PREVIA: ESTADO Y DETECCIÓN
# =============================================================================

load_state() {
    if [[ ! -r "$STATE_FILE" ]]; then
        infer_previous_state
        return 0
    fi
    local key value
    while IFS='=' read -r key value || [[ -n "$key" ]]; do
        case "$key" in
            DOMAIN|USE_SSL|SSL_EMAIL|PANEL_PORT|SFTP_PORT|PANEL_BIND|SERVER_IP|MASTER_URL|GIT_BRANCH|DB_MODE|MYSQL_HOST_PORT)
                printf -v "PREV_$key" '%s' "$value" ;;
        esac
    done <"$STATE_FILE"
    log_debug "Estado previo cargado de $STATE_FILE"
}

# Instalaciones hechas con el instalador 4.0.0 no tienen installer.conf: se deduce
# el dominio/SSL de la configuración de Nginx que dejó aquel instalador.
infer_previous_state() {
    local conf name
    for conf in /etc/nginx/sites-available/skypanel /etc/nginx/conf.d/skypanel.conf; do
        [[ -r "$conf" ]] || continue
        name=$(awk '$1 == "server_name" {gsub(";", "", $2); print $2; exit}' "$conf")
        if [[ -n "$name" ]] && ! validate_ip "$name"; then
            PREV_DOMAIN="$name"
            if grep -q 'listen 443' "$conf"; then PREV_USE_SSL=true; else PREV_USE_SSL=false; fi
        fi
        break
    done
}

save_state() {
    {
        echo "# Opciones de la instalación de $PROJECT_NAME (install.sh $SCRIPT_VERSION)."
        echo "# Se usan como valores por defecto al volver a ejecutar el instalador."
        echo "# No contiene contraseñas. Fecha: $(date '+%Y-%m-%d %H:%M:%S')"
        echo "DOMAIN=$DOMAIN"
        echo "USE_SSL=$USE_SSL"
        echo "SSL_EMAIL=$SSL_EMAIL"
        echo "PANEL_PORT=$PANEL_PORT"
        echo "SFTP_PORT=$SFTP_PORT"
        echo "PANEL_BIND=$PANEL_BIND"
        echo "SERVER_IP=$SERVER_IP"
        echo "MASTER_URL=$MASTER_URL"
        echo "GIT_BRANCH=$GIT_BRANCH"
        echo "DB_MODE=$DB_MODE"
        echo "MYSQL_HOST_PORT=$MYSQL_HOST_PORT"
    } | write_file "$STATE_FILE" 600 show
    run chmod 700 "$CONFIG_DIR"
}

container_exists() { docker inspect "$1" >/dev/null 2>&1; }
container_running() {
    local s
    s=$(docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null || true)
    [[ "$s" == "true" ]]
}

# container_is_ours NOMBRE: el contenedor lo creó docker compose desde $INSTALL_DIR. Otro
# proyecto puede tener un contenedor con el mismo nombre, y ese no se toca.
container_is_ours() {
    local dir
    dir=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$1" 2>/dev/null || true)
    [[ -n "$dir" ]] || return 1
    [[ "$(readlink -f "$dir" 2>/dev/null || echo "$dir")" == "$(readlink -f "$INSTALL_DIR" 2>/dev/null || echo "$INSTALL_DIR")" ]]
}

# detect_existing_installation: EXISTING_INSTALL e INSTALL_STATE (absent, stopped, running)
detect_existing_installation() {
    EXISTING_INSTALL=false
    INSTALL_STATE=absent
    FOREIGN_CONTAINER=false
    if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1 && container_exists "$PANEL_CONTAINER"; then
        if container_is_ours "$PANEL_CONTAINER"; then
            EXISTING_INSTALL=true
            if container_running "$PANEL_CONTAINER"; then INSTALL_STATE=running; else INSTALL_STATE=stopped; fi
        else
            FOREIGN_CONTAINER=true
        fi
    fi
    if [[ -d "$INSTALL_DIR/.git" ]]; then
        EXISTING_INSTALL=true
        if [[ "$INSTALL_STATE" == "absent" ]]; then INSTALL_STATE=stopped; fi
    fi
    if [[ -f "$INSTALL_DIR/docker-compose.yml" ]] && grep -qE '^[[:space:]]*- SKYPANEL_PANEL_ENABLE=false' "$INSTALL_DIR/docker-compose.yml"; then
        IS_NODE_ONLY=true
    fi
}

# show_install_state: qué hay instalado, en qué rama y versión, y si funciona
show_install_state() {
    case "$INSTALL_STATE" in
        absent) log_info "Estado: $PROJECT_NAME no está instalado en este servidor" ;;
        stopped) log_warning "Estado: instalado en $INSTALL_DIR, pero el panel está detenido" ;;
        running) log_success "Estado: instalado en $INSTALL_DIR y funcionando" ;;
    esac
    if [[ -d "$INSTALL_DIR/.git" ]]; then
        log_info "Rama $(git_in rev-parse --abbrev-ref HEAD 2>/dev/null || echo '?'), versión $(git_in log -1 --format='%h del %cs' 2>/dev/null || echo '?')"
    fi
    if [[ "$FOREIGN_CONTAINER" == "true" ]]; then
        log_warning "Hay un contenedor llamado '$PANEL_CONTAINER' que no pertenece a $INSTALL_DIR: no se tocará"
    fi
}

# Menú cuando ya hay una instalación (solo en modo manual)
existing_installation_menu() {
    print_step "Instalación existente detectada"
    log_info "Directorio: $INSTALL_DIR"
    if [[ "$IS_NODE_ONLY" == "true" ]]; then
        log_info "Este servidor está configurado como nodo (solo daemon)"
    fi
    if [[ "$FORCE" == "true" ]]; then
        log_info "Modo forzado: se recrearán los contenedores (los datos se conservan)"
        return 0
    fi
    if ! interactive_manual; then
        log_info "Se reconfigurará manteniendo los datos, contraseñas y cuentas existentes"
        return 0
    fi

    local choice
    echo ""
    echo "  Ya hay una instalación. ¿Qué deseas hacer?"
    echo "  1) Actualizar (conserva los datos)"
    echo "  2) Reconfigurar: dominio, puertos… (conserva los datos)"
    echo "  3) Reinstalar desde cero (borra los datos)"
    echo "  4) Desinstalar"
    echo "  5) Cancelar"
    echo ""
    prompt_read choice "Selecciona [1-5]" "1"
    case "$choice" in
        1) log_info "Iniciando actualización..."; UPDATE_MODE=true ;;
        2) log_info "Iniciando reconfiguración..." ;;
        3) REINSTALL_MODE=true; reinstall_prepare ;;
        4) UNINSTALL_MODE=""; uninstall; exit 0 ;;
        *) echo "Cancelado."; exit 0 ;;
    esac
}

# =============================================================================
# PREGUNTAS DE CONFIGURACIÓN
#   - Manual: pregunta todo paso a paso
#   - Auto (-y con terminal): pregunta solo datos sensibles (cuenta admin)
#   - --non-interactive / sin terminal: nada; usa argumentos, entorno y defaults
# =============================================================================

db_is_initialized() { [[ -d "$INSTALL_DIR/storage/mysql-data/mysql" ]]; }

ask_configuration() {
    print_step "Configuración de instalación"

    local ans

    # --- Dominio y SSL ---------------------------------------------------------
    if [[ "$NO_DOMAIN" == "true" ]]; then
        DOMAIN=""
        USE_SSL=false
    fi
    if [[ -z "$DOMAIN" && "$NO_DOMAIN" != "true" ]]; then DOMAIN="$PREV_DOMAIN"; fi
    if [[ -z "$USE_SSL" ]]; then USE_SSL="$PREV_USE_SSL"; fi
    if [[ -z "$SSL_EMAIL" ]]; then SSL_EMAIL="$PREV_SSL_EMAIL"; fi

    if interactive_manual; then
        ask_yes_no ans "¿Usar un dominio para el panel?" "$([[ -n "$DOMAIN" ]] && echo yes || echo no)"
        if [[ "$ans" == "yes" ]]; then
            local i
            for i in 1 2 3; do
                ask_input DOMAIN "Dominio (ej. panel.midominio.com)" "$DOMAIN"
                if validate_domain "$DOMAIN"; then break; fi
                log_error "Dominio inválido: '$DOMAIN'"
                DOMAIN=""
            done
            [[ -n "$DOMAIN" ]] || die "No se indicó un dominio válido"
            ask_yes_no ans "¿Configurar HTTPS con Let's Encrypt?" "$([[ "$USE_SSL" == "false" ]] && echo no || echo yes)"
            if [[ "$ans" == "yes" ]]; then
                print_ssl_requirements
                USE_SSL=true
                ask_input SSL_EMAIL "Email para Let's Encrypt (avisos de caducidad; vacío = sin email)" "${SSL_EMAIL:-$ADMIN_EMAIL}" sensitive
            else
                USE_SSL=false
            fi
        else
            DOMAIN=""
            USE_SSL=false
        fi
        MASTER_URL_EXPLICIT=true
    fi

    # --- Puertos -----------------------------------------------------------------
    if [[ -z "$PANEL_PORT" ]]; then PANEL_PORT="${PREV_PANEL_PORT:-$DEFAULT_PANEL_PORT}"; fi
    if [[ -z "$SFTP_PORT" ]]; then SFTP_PORT="${PREV_SFTP_PORT:-$DEFAULT_SFTP_PORT}"; fi
    if [[ -z "$PANEL_BIND" ]]; then PANEL_BIND="${PREV_PANEL_BIND:-0.0.0.0}"; fi
    if interactive_manual; then
        echo ""
        ask_yes_no ans "¿Usar los puertos predeterminados (panel $DEFAULT_PANEL_PORT, SFTP $DEFAULT_SFTP_PORT)?" \
            "$([[ "$PANEL_PORT" == "$DEFAULT_PANEL_PORT" && "$SFTP_PORT" == "$DEFAULT_SFTP_PORT" ]] && echo yes || echo no)"
        if [[ "$ans" == "yes" ]]; then
            PANEL_PORT="$DEFAULT_PANEL_PORT"
            SFTP_PORT="$DEFAULT_SFTP_PORT"
        else
            ask_port PANEL_PORT "Puerto del panel" "$PANEL_PORT" ""
            ask_port SFTP_PORT "Puerto SFTP" "$SFTP_PORT" "$PANEL_PORT"
        fi
    fi

    # --- Base de datos -------------------------------------------------------------
    if [[ -z "$DB_MODE" ]]; then DB_MODE="${PREV_DB_MODE:-local}"; fi
    if ! db_is_initialized && interactive_manual && [[ "$EXISTING_INSTALL" != "true" ]]; then
        echo ""
        echo "Base de datos:"
        echo "  1) Local (MariaDB en Docker) - recomendado"
        echo "  2) Externa (servidor MySQL/MariaDB existente)"
        local db_choice
        prompt_read db_choice "Selecciona [1-2]" "1"
        case "$db_choice" in
            2)
                DB_MODE="external"
                echo ""
                ask_input DB_HOST "Host MySQL" "${DB_HOST:-localhost}"
                ask_input DB_PORT "Puerto MySQL" "${DB_PORT:-3306}"
                ask_input DB_NAME "Nombre BD" "${DB_NAME:-skypanel}"
                ask_input DB_USER "Usuario BD" "${DB_USER:-skypanel}"
                if [[ -z "$DB_PASSWORD" ]]; then
                    ask_password DB_PASSWORD "Contraseña del usuario BD" 1 false
                fi
                echo ""
                log_info "Si la base de datos o el usuario todavía no existen, el instalador los crea con una"
                log_info "cuenta de administrador (root u otra con permiso CREATE USER). No se guarda en ningún sitio."
                ask_yes_no ans "¿La base de datos y el usuario ya existen?" "yes"
                if [[ "$ans" != "yes" ]]; then
                    ask_input DB_ADMIN_USER "Usuario administrador de MySQL" "${DB_ADMIN_USER:-root}"
                    if [[ -z "$DB_ADMIN_PASSWORD" ]]; then
                        ask_password DB_ADMIN_PASSWORD "Contraseña del administrador de MySQL" 1 false
                    fi
                fi
                ;;
            *)
                DB_MODE="local"
                echo ""
                log_info "Las contraseñas de MariaDB se pueden generar automáticamente (recomendado)."
                if [[ -z "$DB_PASSWORD" ]]; then
                    ask_password DB_PASSWORD "Contraseña usuario BD (vacío = generar una segura)" 8 true
                fi
                if [[ -z "$DB_ROOT_PASSWORD" ]]; then
                    ask_password DB_ROOT_PASSWORD "Contraseña root MySQL (vacío = generar una segura)" 8 true
                fi
                ;;
        esac
    fi

    # --- Cuenta administrador -------------------------------------------------------
    if [[ "$IS_NODE_ONLY" == "true" ]]; then
        CREATE_ADMIN=false
    elif [[ -z "$CREATE_ADMIN" ]]; then
        if [[ -n "$ADMIN_EMAIL" ]]; then
            CREATE_ADMIN=true
        elif [[ "$EXISTING_INSTALL" == "true" ]] && db_is_initialized; then
            # Reinstalación: las cuentas existentes NO se tocan salvo que se pida
            CREATE_ADMIN=false
            if interactive_manual; then
                ask_yes_no ans "¿Añadir una cuenta administrador nueva?" "no"
                if [[ "$ans" == "yes" ]]; then CREATE_ADMIN=true; fi
            fi
        elif [[ "$CAN_PROMPT" == "true" ]]; then
            CREATE_ADMIN=true
        elif [[ -n "$DOMAIN" ]]; then
            CREATE_ADMIN=true
        else
            CREATE_ADMIN=false   # sin terminal y sin email: se explica cómo crearla al final
        fi
    fi

    if [[ "$CREATE_ADMIN" == "true" ]]; then
        if [[ "$CAN_PROMPT" == "true" ]]; then
            echo ""
            echo "============================================================"
            echo "  Cuenta administrador inicial"
            echo "============================================================"
            if [[ -z "$ADMIN_EMAIL" ]]; then
                local i def_email="${SSL_EMAIL:-}"
                if [[ -z "$def_email" && -n "$DOMAIN" ]]; then def_email="admin@$DOMAIN"; fi
                for i in 1 2 3; do
                    ask_input ADMIN_EMAIL "Email del administrador" "$def_email" sensitive
                    if validate_email "$ADMIN_EMAIL"; then break; fi
                    log_error "Email inválido: '$ADMIN_EMAIL'"
                    ADMIN_EMAIL=""
                done
            fi
            if [[ -z "$ADMIN_PASSWORD" ]]; then
                ask_password ADMIN_PASSWORD "Contraseña del administrador (mín. 8; vacío = generar una segura)" 8 true
            fi
        fi
        if [[ -z "$ADMIN_EMAIL" && -n "$DOMAIN" ]]; then ADMIN_EMAIL="admin@$DOMAIN"; fi
        if [[ -z "$ADMIN_EMAIL" ]]; then
            log_warning "No hay email de administrador: la cuenta se creará después (ver resumen)"
            CREATE_ADMIN=false
        fi
    fi
}

# Comprueba la coherencia de todas las opciones antes de cambiar nada
validate_configuration() {
    local errors=0
    if [[ -n "$DOMAIN" ]] && ! validate_domain "$DOMAIN"; then
        log_error "Dominio inválido: $DOMAIN"; errors=$((errors + 1))
    fi
    if [[ -n "$DOMAIN" && "$DOMAIN" != *.* ]]; then
        log_warning "El dominio '$DOMAIN' no tiene punto; Let's Encrypt no podrá emitir un certificado"
    fi
    if [[ -z "$USE_SSL" ]]; then USE_SSL=false; fi
    if [[ "$USE_SSL" == "true" && -z "$DOMAIN" ]]; then
        log_error "--ssl necesita un dominio (--domain)"; errors=$((errors + 1))
    fi
    validate_port "$PANEL_PORT" || { log_error "Puerto del panel inválido: $PANEL_PORT"; errors=$((errors + 1)); }
    validate_port "$SFTP_PORT" || { log_error "Puerto SFTP inválido: $SFTP_PORT"; errors=$((errors + 1)); }
    if [[ "$PANEL_PORT" == "$SFTP_PORT" ]]; then
        log_error "El puerto del panel y el de SFTP no pueden ser el mismo"; errors=$((errors + 1))
    fi
    if [[ "$PANEL_BIND" != "0.0.0.0" ]] && ! validate_ip "$PANEL_BIND"; then
        log_error "--bind debe ser una IPv4 (ej. 127.0.0.1): $PANEL_BIND"; errors=$((errors + 1))
    fi
    if [[ -n "$SERVER_IP" ]] && ! validate_ip "$SERVER_IP"; then
        log_error "--ip debe ser una IPv4: $SERVER_IP"; errors=$((errors + 1))
    fi
    if [[ -n "$MASTER_URL" && ! "$MASTER_URL" =~ ^https?://[^[:space:]/]+(/[^[:space:]]*)?$ ]]; then
        log_error "--master-url debe empezar por http:// o https://: $MASTER_URL"; errors=$((errors + 1))
    fi
    if [[ -n "$SSL_EMAIL" ]] && ! validate_email "$SSL_EMAIL"; then
        log_error "Email inválido: $SSL_EMAIL"; errors=$((errors + 1))
    fi
    if [[ -n "$ADMIN_EMAIL" ]] && ! validate_email "$ADMIN_EMAIL"; then
        log_error "Email del administrador inválido: $ADMIN_EMAIL"; errors=$((errors + 1))
    fi
    if [[ -n "$ADMIN_PASSWORD" ]] && (( $(byte_len "$ADMIN_PASSWORD") < 8 || $(byte_len "$ADMIN_PASSWORD") > 72 )); then
        log_error "La contraseña del administrador debe tener entre 8 y 72 bytes"; errors=$((errors + 1))
    fi
    if [[ ! "$ADMIN_NAME" =~ ^[[:print:]]{1,100}$ ]]; then
        log_error "Nombre de usuario admin inválido"; errors=$((errors + 1))
    fi

    case "$DB_MODE" in
        local)
            local v
            for v in DB_PASSWORD DB_ROOT_PASSWORD; do
                if [[ -n "${!v}" ]] && ! validate_local_db_password "${!v}"; then
                    log_error "$v: usa al menos 8 caracteres de [A-Za-z0-9._+=,-] (o déjala vacía para generarla)"
                    errors=$((errors + 1))
                fi
            done
            ;;
        external)
            DB_PORT="${DB_PORT:-3306}"; DB_NAME="${DB_NAME:-skypanel}"; DB_USER="${DB_USER:-skypanel}"
            if [[ -z "$DB_HOST" ]]; then log_error "--db-mode external necesita --db-host"; errors=$((errors + 1)); fi
            validate_port "$DB_PORT" || { log_error "Puerto de BD inválido: $DB_PORT"; errors=$((errors + 1)); }
            if ! db_is_initialized && [[ -z "$(env_get DB_HOST)" ]] && ! validate_external_db_password "$DB_PASSWORD"; then
                log_error "--db-pass es obligatoria con BD externa y no puede contener espacios, comillas, \$, \\, \` ni #"
                errors=$((errors + 1))
            fi
            ;;
        *) log_error "--db-mode debe ser local o external"; errors=$((errors + 1)) ;;
    esac
    if [[ -n "$DB_USER" && ! "$DB_USER" =~ ^[A-Za-z0-9_]+$ ]]; then
        log_error "Usuario de BD inválido (solo letras, números y _): $DB_USER"; errors=$((errors + 1))
    fi
    if [[ -n "$DB_NAME" && ! "$DB_NAME" =~ ^[A-Za-z0-9_]+$ ]]; then
        log_error "Nombre de BD inválido (solo letras, números y _): $DB_NAME"; errors=$((errors + 1))
    fi
    if [[ -n "$LOG_RETENTION_DAYS" && ! "$LOG_RETENTION_DAYS" =~ ^[0-9]+$ ]]; then
        log_error "--log-retention-days debe ser un número (0 = guardar siempre)"; errors=$((errors + 1))
    fi
    if [[ -n "$TURNSTILE_SITE_KEY" && -z "$TURNSTILE_SECRET_KEY" ]] || [[ -z "$TURNSTILE_SITE_KEY" && -n "$TURNSTILE_SECRET_KEY" ]]; then
        log_error "Turnstile necesita las dos claves: --turnstile-site-key y --turnstile-secret-key"; errors=$((errors + 1))
    fi
    if [[ -n "$S3_BUCKET$S3_ACCESS_KEY$S3_SECRET_KEY$S3_ENDPOINT" ]] && [[ -z "$S3_BUCKET" || -z "$S3_ACCESS_KEY" || -z "$S3_SECRET_KEY" ]]; then
        log_error "Las copias en S3 necesitan al menos --s3-bucket, --s3-access-key y --s3-secret-key"; errors=$((errors + 1))
    fi
    if [[ -n "$DNS_BASE_DOMAIN" ]] && ! validate_domain "$DNS_BASE_DOMAIN"; then
        log_error "--dns-base-domain inválido: $DNS_BASE_DOMAIN"; errors=$((errors + 1))
    fi
    if [[ "$ROTATE_DB_PASSWORDS" == "true" && "$DB_MODE" != "local" ]]; then
        log_error "--rotate-db-passwords solo se aplica a la base de datos local"; errors=$((errors + 1))
    fi

    if (( errors > 0 )); then
        die "Hay $errors opción(es) inválida(s). Revisa --help"
    fi
}

# =============================================================================
# INSTALACIÓN DE DEPENDENCIAS
# =============================================================================

apt_update() {
    run env DEBIAN_FRONTEND=noninteractive apt-get -q -o DPkg::Lock::Timeout=600 update --allow-releaseinfo-change \
        || log_warning "apt-get update terminó con errores (algún repositorio de terceros); se continúa"
}

# Espera el bloqueo de dpkg (unattended-upgrades en VPS recién creados) en lugar de fallar
apt_install() {
    run env DEBIAN_FRONTEND=noninteractive apt-get -y -q -o DPkg::Lock::Timeout=600 \
        -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold \
        install --no-install-recommends "$@"
}

pacman_install() {
    run pacman -Sy --needed --noconfirm "$@"
}

zypper_install() {
    run zypper --non-interactive --quiet install --no-recommends "$@"
}

dnf_install() {
    if command -v dnf >/dev/null 2>&1; then
        run dnf install -y -q "$@"
    else
        run yum install -y -q "$@"
    fi
}

install_dependencies() {
    if [[ "$SKIP_DEPENDENCIES" == "true" ]]; then
        log_info "Dependencias omitidas (--skip-deps)"
        return 0
    fi

    print_step "Instalando dependencias del sistema"

    # Solo lo necesario: la compilación del panel ocurre dentro de Docker. curl no se pide si
    # ya está: en Rocky/Alma 9 viene como curl-minimal, que choca con el paquete curl y hacía
    # fallar toda la instalación de dependencias.
    local -a pkgs=(ca-certificates git jq)
    if ! command -v curl >/dev/null 2>&1; then pkgs+=(curl); fi
    local need_nginx=false
    if [[ "$SKIP_NGINX" != "true" ]]; then need_nginx=true; fi

    case "$OS_FAMILY" in
        debian)
            pkgs+=(gnupg iproute2 util-linux)
            if [[ "$need_nginx" == "true" ]]; then pkgs+=(nginx); fi
            if [[ "$SKIP_FIREWALL" != "true" ]]; then pkgs+=(ufw); fi
            if [[ "$USE_SSL" == "true" && "$need_nginx" == "true" ]]; then pkgs+=(certbot); fi
            apt_update
            apt_install "${pkgs[@]}"
            ;;
        fedora|rhel)
            pkgs+=(iproute util-linux)
            if [[ "$need_nginx" == "true" ]]; then pkgs+=(nginx); fi
            if [[ "$SKIP_FIREWALL" != "true" ]]; then pkgs+=(firewalld); fi
            if [[ "$USE_SSL" == "true" && "$need_nginx" == "true" ]]; then
                # certbot está en EPEL en RHEL/Rocky/Alma
                if [[ "$OS_FAMILY" == "rhel" ]]; then
                    dnf_install epel-release || log_warning "No se pudo instalar EPEL (necesario para certbot)"
                fi
                pkgs+=(certbot)
            fi
            dnf_install "${pkgs[@]}"
            ;;
        arch)
            # Docker, Compose y buildx son paquetes de Arch: no hay repositorio aparte
            pkgs+=(iproute2 util-linux)
            if ! command -v docker >/dev/null 2>&1; then pkgs+=(docker docker-compose docker-buildx); fi
            if [[ "$need_nginx" == "true" ]]; then pkgs+=(nginx); fi
            if [[ "$SKIP_FIREWALL" != "true" ]]; then pkgs+=(ufw); fi
            if [[ "$USE_SSL" == "true" && "$need_nginx" == "true" ]]; then pkgs+=(certbot); fi
            pacman_install "${pkgs[@]}"
            ;;
        suse)
            pkgs=(ca-certificates git-core jq iproute2 util-linux)
            if ! command -v curl >/dev/null 2>&1; then pkgs+=(curl); fi
            # iptables con el motor nftables: los núcleos nuevos ya no traen los módulos del
            # iptables clásico, y sin ellos Docker no arranca ("Module ip_tables not found")
            if ! command -v docker >/dev/null 2>&1; then pkgs+=(docker docker-compose docker-buildx iptables-backend-nft nftables); fi
            if [[ "$need_nginx" == "true" ]]; then pkgs+=(nginx); fi
            if [[ "$SKIP_FIREWALL" != "true" ]]; then pkgs+=(firewalld); fi
            zypper_install "${pkgs[@]}"
            if [[ "$USE_SSL" == "true" && "$need_nginx" == "true" ]]; then
                # según la versión el paquete se llama certbot o python3-certbot
                zypper_install certbot 2>/dev/null || zypper_install python3-certbot \
                    || log_warning "No se pudo instalar certbot: el panel funcionará por HTTP hasta instalarlo"
            fi
            ;;
        *)
            die "No sé instalar paquetes en esta distribución ($OS_FAMILY)"
            ;;
    esac

    # Instalar Docker si no está
    if ! command -v docker >/dev/null 2>&1; then
        install_docker
    fi

    log_success "Dependencias instaladas"
}

# Descarga la llave GPG de Docker y comprueba su huella antes de confiar en ella
fetch_docker_gpg_key() {
    local dest="$1" tmp fpr
    tmp=$(mktemp)
    TMP_FILES+=("$tmp")
    curl --proto '=https' --tlsv1.2 -fsSL "https://download.docker.com/linux/$DOCKER_DISTRO/gpg" -o "$tmp"
    if command -v gpg >/dev/null 2>&1; then
        fpr=$(gpg --show-keys --with-colons "$tmp" 2>/dev/null | awk -F: '$1 == "fpr" {print $10}')
        if [[ "$fpr" != *"$DOCKER_GPG_FINGERPRINT"* ]]; then
            die "La llave GPG descargada de Docker no tiene la huella esperada ($DOCKER_GPG_FINGERPRINT)"
        fi
        log_success "Llave GPG de Docker verificada"
    else
        log_warning "gpg no disponible: no se pudo verificar la huella de la llave de Docker"
    fi
    install -m 0644 "$tmp" "$dest"
}

install_docker() {
    if [[ "$SKIP_DOCKER" == "true" ]]; then
        log_info "Instalación de Docker omitida (--skip-docker)"
        return 0
    fi

    print_step "Instalando Docker"

    case "$OS_FAMILY" in
        debian)
            [[ -n "$DOCKER_CODENAME" ]] || die "No se pudo determinar el nombre en clave de la distribución para el repositorio de Docker"
            local keyring="/etc/apt/keyrings/docker.asc"
            # Instalaciones de 4.0.0 usaban docker.gpg: se respeta si ya existe
            if [[ -f /etc/apt/keyrings/docker.gpg ]]; then keyring="/etc/apt/keyrings/docker.gpg"; fi
            if [[ "$DRY_RUN" == "true" ]]; then
                run curl --proto '=https' --tlsv1.2 -fsSL "https://download.docker.com/linux/$DOCKER_DISTRO/gpg" -o "$keyring"
            elif [[ ! -f "$keyring" ]]; then
                install -m 0755 -d /etc/apt/keyrings
                fetch_docker_gpg_key "$keyring"
            fi
            local arch_deb
            arch_deb=$(dpkg --print-architecture 2>/dev/null || echo "$ARCH")
            echo "deb [arch=$arch_deb signed-by=$keyring] https://download.docker.com/linux/$DOCKER_DISTRO $DOCKER_CODENAME stable" \
                | write_file /etc/apt/sources.list.d/docker.list 644 show
            apt_update
            apt_install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
        fedora|rhel)
            curl_to_file "https://download.docker.com/linux/$DOCKER_DISTRO/docker-ce.repo" /etc/yum.repos.d/docker-ce.repo
            # --allowerasing: en RHEL 8 podman/runc entran en conflicto con docker-ce
            if command -v dnf >/dev/null 2>&1; then
                run dnf install -y -q --allowerasing docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            else
                run yum install -y -q docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            fi
            ;;
        arch)
            pacman_install docker docker-compose docker-buildx
            ;;
        suse)
            zypper_install docker docker-compose docker-buildx iptables-backend-nft nftables
            ;;
    esac

    svc enable --now docker
    log_success "Docker instalado"
}

curl_to_file() {
    if [[ "$DRY_RUN" == "true" ]]; then
        run curl --proto '=https' --tlsv1.2 -fsSL "$1" -o "$2"
        return 0
    fi
    local tmp
    tmp=$(mktemp)
    TMP_FILES+=("$tmp")
    curl --proto '=https' --tlsv1.2 -fsSL "$1" -o "$tmp"
    install -m 0644 "$tmp" "$2"
}

install_compose_plugin() {
    log_info "Instalando el plugin Docker Compose v2..."
    case "$OS_FAMILY" in
        debian)
            apt_install docker-compose-plugin 2>/dev/null || apt_install docker-compose-v2 \
                || log_warning "No se pudo instalar Docker Compose v2 automáticamente"
            ;;
        fedora|rhel)
            dnf_install docker-compose-plugin || log_warning "No se pudo instalar Docker Compose v2 automáticamente"
            ;;
        arch)
            pacman_install docker-compose || log_warning "No se pudo instalar Docker Compose v2 automáticamente"
            ;;
        suse)
            zypper_install docker-compose || log_warning "No se pudo instalar Docker Compose v2 automáticamente"
            ;;
    esac
}

# =============================================================================
# CÓDIGO DEL PANEL (git) - mismo comportamiento que tools/panelUpdate/panelUpdate.sh
# =============================================================================

git_in() { git -C "$INSTALL_DIR" -c safe.directory="$INSTALL_DIR" "$@"; }
GIT=(git -C "$INSTALL_DIR" -c "safe.directory=$INSTALL_DIR")

resolve_branch() {
    if [[ -n "$GIT_BRANCH" ]]; then return 0; fi
    if [[ -n "$PREV_GIT_BRANCH" ]]; then
        GIT_BRANCH="$PREV_GIT_BRANCH"
    elif [[ -d "$INSTALL_DIR/.git" ]]; then
        # Sin indicación: se sigue en la rama que ya tiene la instalación
        GIT_BRANCH=$(git_in rev-parse --abbrev-ref HEAD 2>/dev/null || true)
        if [[ -z "$GIT_BRANCH" || "$GIT_BRANCH" == "HEAD" ]]; then GIT_BRANCH="$DEFAULT_GIT_BRANCH"; fi
    else
        GIT_BRANCH="$DEFAULT_GIT_BRANCH"
    fi
    if [[ ! "$GIT_BRANCH" =~ ^[A-Za-z0-9._/-]+$ ]]; then
        die "Nombre de rama inválido: $GIT_BRANCH"
    fi
}

backup_env_file() {
    ENV_BACKUP=""
    if [[ -f "$INSTALL_DIR/.env" && "$DRY_RUN" != "true" ]]; then
        ENV_BACKUP=$(mktemp)
        cp -p "$INSTALL_DIR/.env" "$ENV_BACKUP"
    fi
}

restore_env_file() {
    if [[ -n "$ENV_BACKUP" && -f "$ENV_BACKUP" ]]; then
        cp -p "$ENV_BACKUP" "$INSTALL_DIR/.env"
        rm -f "$ENV_BACKUP"
        ENV_BACKUP=""
        log_success "Archivo .env del servidor conservado"
    fi
}

sync_repository() {
    print_step "Descargando el código del panel (rama $GIT_BRANCH)"

    if [[ ! -d "$INSTALL_DIR/.git" ]]; then
        if [[ -d "$INSTALL_DIR" ]] && [[ -n "$(ls -A "$INSTALL_DIR" 2>/dev/null)" ]]; then
            # Carpeta con datos pero sin git (p. ej. tras una desinstalación parcial):
            # se inicializa git en el sitio sin tocar storage/ ni .env
            log_warning "$INSTALL_DIR existe pero no es un repositorio git; se inicializa sin borrar nada"
            backup_env_file
            run git -C "$INSTALL_DIR" init -q
            run "${GIT[@]}" remote add origin "$GITHUB_REPO" 2>/dev/null || run "${GIT[@]}" remote set-url origin "$GITHUB_REPO"
            run "${GIT[@]}" fetch origin "$GIT_BRANCH"
            run "${GIT[@]}" checkout -f -B "$GIT_BRANCH" --track "origin/$GIT_BRANCH"
            restore_env_file
        else
            run mkdir -p "$(dirname "$INSTALL_DIR")"
            run git clone --branch "$GIT_BRANCH" "$GITHUB_REPO" "$INSTALL_DIR"
        fi
        log_success "Repositorio descargado"
        return 0
    fi

    # Repositorio existente
    if [[ "$DRY_RUN" != "true" ]] && ! git_in ls-remote --exit-code --heads origin "$GIT_BRANCH" >/dev/null 2>&1; then
        die "La rama '$GIT_BRANCH' no existe en $(git_in remote get-url origin 2>/dev/null || echo origin)"
    fi

    # Nodo esclavo: panelUpdate.sh re-aplica esta configuración tras el pull; aquí igual
    local slave_token=""
    if [[ "$IS_NODE_ONLY" == "true" ]]; then
        slave_token=$(sed -n 's/.*SKYPANEL_TOKEN_PUBLIC=\([^ ]*\).*/\1/p' "$INSTALL_DIR/docker-compose.yml" | head -n1)
    fi

    # Cambios locales en archivos del repositorio (salvo .env): se guardan antes de descartarlos
    local changes
    changes=$(git_in diff -- . ':(exclude).env' 2>/dev/null || true)
    if [[ -n "$changes" ]]; then
        local patch
        patch="$CONFIG_DIR/backups/cambios-locales-$(date +%Y%m%d-%H%M%S).patch"
        printf '%s\n' "$changes" | write_file "$patch" 600
        log_warning "Había cambios locales en el repositorio; copia guardada en $patch"
    fi

    backup_env_file
    run "${GIT[@]}" checkout -- .
    run "${GIT[@]}" fetch origin
    local current
    current=$(git_in rev-parse --abbrev-ref HEAD 2>/dev/null || true)
    if [[ "$current" != "$GIT_BRANCH" ]]; then
        log_info "Cambiando de rama: $current -> $GIT_BRANCH"
        run "${GIT[@]}" checkout "$GIT_BRANCH"
    fi
    if ! run "${GIT[@]}" pull --ff-only origin "$GIT_BRANCH"; then
        restore_env_file
        die "No se pudo actualizar la rama $GIT_BRANCH (historia divergente). Revisa $INSTALL_DIR con git status"
    fi
    restore_env_file

    if [[ "$IS_NODE_ONLY" == "true" ]]; then
        run sed -i 's|^[[:space:]]*# - SKYPANEL_PANEL_ENABLE=false|      - SKYPANEL_PANEL_ENABLE=false|' "$INSTALL_DIR/docker-compose.yml"
        if [[ -n "$slave_token" ]]; then
            run sed -i "s|^[[:space:]]*# - SKYPANEL_TOKEN_PUBLIC=.*|      - SKYPANEL_TOKEN_PUBLIC=${slave_token}|" "$INSTALL_DIR/docker-compose.yml"
        fi
        log_success "Configuración de nodo (solo daemon) re-aplicada en docker-compose.yml"
    fi

    log_success "Código actualizado ($(git_in rev-parse --short HEAD 2>/dev/null || echo '?'))"
}

# =============================================================================
# .env (contraseñas de la base de datos)
#   El .env está versionado en git con valores de ejemplo; cada servidor guarda
#   el suyo y panelUpdate.sh lo conserva en cada actualización.
# =============================================================================

env_get() {
    local key="$1" file="${2:-$INSTALL_DIR/.env}" line val=""
    [[ -r "$file" ]] || return 0
    while IFS= read -r line || [[ -n "$line" ]]; do
        if [[ "$line" == "$key="* ]]; then val=${line#*=}; fi
    done <"$file"
    val=${val%$'\r'}
    val=${val#\"}; val=${val%\"}; val=${val#\'}; val=${val%\'}
    printf '%s' "$val"
}

# env_set CLAVE VALOR: cambia o añade una línea sin tocar el resto (el valor no
# aparece en la lista de procesos: solo se usan builtins de bash)
env_set() {
    local key="$1" value="$2" file="$INSTALL_DIR/.env" tmp line found=false
    if [[ "$DRY_RUN" == "true" ]]; then
        if [[ "$key" == *PASSWORD* ]]; then value="(oculta)"; fi
        printf '%s[dry-run]%s .env: %s=%s\n' "$CYAN" "$NC" "$key" "$value"
        return 0
    fi
    tmp=$(mktemp "$file.XXXXXX")
    TMP_FILES+=("$tmp")
    if [[ -f "$file" ]]; then
        while IFS= read -r line || [[ -n "$line" ]]; do
            if [[ "$line" == "$key="* ]]; then
                printf '%s=%s\n' "$key" "$value"
                found=true
            else
                printf '%s\n' "$line"
            fi
        done <"$file" >"$tmp"
    fi
    if [[ "$found" != "true" ]]; then printf '%s=%s\n' "$key" "$value" >>"$tmp"; fi
    chmod 600 "$tmp"
    mv -f "$tmp" "$file"
}

configure_env_file() {
    print_step "Configurando credenciales de la base de datos (.env)"
    local env_file="$INSTALL_DIR/.env"
    local cur_pass cur_root
    cur_pass=$(env_get DB_PASSWORD)
    cur_root=$(env_get DB_ROOT_PASSWORD)

    if db_is_initialized; then
        # MariaDB ya creada: sus contraseñas solo se fijan al crearla. Cambiar el .env
        # ahora dejaría al panel sin acceso, así que no se toca (salvo --rotate-db-passwords).
        if [[ -n "$DB_PASSWORD$DB_ROOT_PASSWORD" && "$ROTATE_DB_PASSWORDS" != "true" ]]; then
            log_warning "La base de datos ya existe: se ignoran --db-pass/--db-root-pass (usa --rotate-db-passwords para cambiarlas)"
        fi
        if contains_word "$KNOWN_DEFAULT_DB_PASSWORDS" "${cur_pass:-skypanel_secret}" \
            || contains_word "$KNOWN_DEFAULT_DB_PASSWORDS" "${cur_root:-skypanel_secret}"; then
            DEFAULT_DB_PASSWORDS_IN_USE=true
            if [[ "$ROTATE_DB_PASSWORDS" != "true" ]]; then
                log_warning "La base de datos usa contraseñas por defecto públicas (están en el repositorio)."
                log_warning "Los servidores de juego comparten red con MariaDB: cámbialas con --rotate-db-passwords"
            fi
        fi
        log_success "Se conservan las credenciales existentes de la base de datos"
    else
        # Base de datos nueva: contraseñas propias de este servidor
        if [[ -z "$DB_PASSWORD" ]]; then
            if [[ -n "$cur_pass" ]] && ! contains_word "$KNOWN_DEFAULT_DB_PASSWORDS" "$cur_pass"; then
                DB_PASSWORD="$cur_pass"      # reintento tras un fallo: se reutiliza
            else
                DB_PASSWORD=$(gen_secret)
            fi
        fi
        if [[ -z "$DB_ROOT_PASSWORD" ]]; then
            if [[ -n "$cur_root" ]] && ! contains_word "$KNOWN_DEFAULT_DB_PASSWORDS" "$cur_root"; then
                DB_ROOT_PASSWORD="$cur_root"
            else
                DB_ROOT_PASSWORD=$(gen_secret)
            fi
        fi
        env_set DB_ROOT_PASSWORD "$DB_ROOT_PASSWORD"
        env_set DB_DATABASE "${DB_NAME:-skypanel}"
        env_set DB_USER "${DB_USER:-skypanel}"
        env_set DB_PASSWORD "$DB_PASSWORD"
        log_success "Contraseñas propias de la base de datos guardadas en $env_file (solo root)"
    fi

    if [[ "$DB_MODE" == "external" ]]; then
        local host="$DB_HOST"
        if [[ -n "$host" ]]; then
            if [[ "$host" == "localhost" || "$host" == "127.0.0.1" ]]; then
                log_warning "Dentro del contenedor 'localhost' es el propio contenedor: se usa host.docker.internal"
                log_warning "Asegúrate de que MySQL del host escucha en la IP del puente Docker (bind-address)"
                host="host.docker.internal"
            fi
            env_set DB_HOST "$host"
            env_set DB_PORT "${DB_PORT:-3306}"
            env_set DB_DATABASE "${DB_NAME:-skypanel}"
            env_set DB_USER "${DB_USER:-skypanel}"
            if [[ -n "$DB_PASSWORD" ]]; then env_set DB_PASSWORD "$DB_PASSWORD"; fi
        fi
    fi

    if [[ -f "$env_file" ]]; then run chmod 600 "$env_file"; fi
}

# Cambia las contraseñas de MariaDB de una instalación existente (opt-in)
rotate_db_passwords() {
    [[ "$ROTATE_DB_PASSWORDS" == "true" ]] || return 0
    print_step "Cambiando las contraseñas de la base de datos"

    if ! db_is_initialized; then
        log_info "La base de datos aún no existe: ya se crea con contraseñas nuevas"
        return 0
    fi

    local old_root new_root new_user user
    old_root=$(env_get DB_ROOT_PASSWORD)
    old_root=${old_root:-skypanel_secret}
    user=$(env_get DB_USER); user=${user:-skypanel}
    new_root="${DB_ROOT_PASSWORD:-$(gen_secret)}"
    new_user="${DB_PASSWORD:-$(gen_secret)}"

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[dry-run] Se cambiarían las contraseñas de root y $user en MariaDB y en .env"
        return 0
    fi

    dc up -d mysql
    local i
    for i in $(seq 1 30); do
        if [[ "$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$MYSQL_CONTAINER" 2>/dev/null)" == "healthy" ]]; then break; fi
        sleep 2
    done

    # Las nuevas contraseñas se guardan antes de aplicarlas, por si algo falla a mitad
    save_credentials "Contraseñas nuevas de MariaDB (rotación)" \
        "DB_ROOT_PASSWORD=$new_root" "DB_PASSWORD=$new_user"

    local sql
    sql="ALTER USER IF EXISTS 'root'@'%' IDENTIFIED BY '$new_root';
ALTER USER IF EXISTS 'root'@'localhost' IDENTIFIED BY '$new_root';
ALTER USER IF EXISTS '$user'@'%' IDENTIFIED BY '$new_user';
FLUSH PRIVILEGES;"
    # La contraseña va por variable de entorno y el SQL por stdin: nada en la línea de comandos
    if ! MYSQL_PWD="$old_root" docker exec -i -e MYSQL_PWD "$MYSQL_CONTAINER" mariadb -uroot <<<"$sql"; then
        die "No se pudieron cambiar las contraseñas en MariaDB (¿la contraseña root del .env es correcta?)"
    fi
    env_set DB_ROOT_PASSWORD "$new_root"
    env_set DB_PASSWORD "$new_user"
    DEFAULT_DB_PASSWORDS_IN_USE=false
    log_success "Contraseñas de MariaDB cambiadas (también en $INSTALL_DIR/.env)"
}

# =============================================================================
# docker-compose.override.yml (puertos personalizados / BD externa)
#   Docker Compose lo combina automáticamente con docker-compose.yml. No está en
#   git, así que sobrevive a panelUpdate.sh (git checkout -- . / git pull).
# =============================================================================

configure_compose_override() {
    local file="$INSTALL_DIR/docker-compose.override.yml"
    local need_ports=false need_mysql=false need_external=false

    if [[ "$PANEL_PORT" != "$DEFAULT_PANEL_PORT" || "$SFTP_PORT" != "$DEFAULT_SFTP_PORT" || "$PANEL_BIND" != "0.0.0.0" ]]; then
        need_ports=true
    fi
    MYSQL_HOST_PORT="${MYSQL_HOST_PORT:-$DEFAULT_MYSQL_HOST_PORT}"
    if [[ "$DB_MODE" == "local" && "$MYSQL_HOST_PORT" != "$DEFAULT_MYSQL_HOST_PORT" ]]; then
        need_mysql=true
    fi
    if [[ "$DB_MODE" == "external" ]]; then need_external=true; fi

    local ours=false
    if [[ -f "$file" ]] && grep -qF "$OVERRIDE_MARKER" "$file"; then ours=true; fi

    if [[ "$need_ports$need_mysql$need_external" == "falsefalsefalse" ]]; then
        if [[ "$ours" == "true" ]]; then
            run rm -f "$file"
            log_info "Ya no hace falta docker-compose.override.yml (valores por defecto); eliminado"
        elif [[ -f "$file" ]]; then
            log_warning "Existe un docker-compose.override.yml propio; Docker Compose lo seguirá aplicando"
        fi
        return 0
    fi

    print_step "Configurando puertos y conexión (docker-compose.override.yml)"

    if [[ "$need_ports" == "true" || "$need_mysql" == "true" ]] && [[ "$DRY_RUN" != "true" ]]; then
        if [[ -z "$COMPOSE_VERSION" ]] || ! version_ge "$COMPOSE_VERSION" "$MIN_COMPOSE_FOR_OVERRIDE"; then
            die "Cambiar puertos necesita Docker Compose >= $MIN_COMPOSE_FOR_OVERRIDE (tienes ${COMPOSE_VERSION:-?}). Actualiza Docker o usa los puertos por defecto"
        fi
    fi

    if [[ -f "$file" && "$ours" != "true" ]]; then
        local bak
        bak="$file.bak-$(date +%Y%m%d-%H%M%S)"
        run cp -p "$file" "$bak"
        log_warning "docker-compose.override.yml existente guardado como $bak"
    fi

    local web_mapping="$PANEL_PORT:$PANEL_INTERNAL_PORT"
    if [[ "$PANEL_BIND" != "0.0.0.0" ]]; then web_mapping="$PANEL_BIND:$web_mapping"; fi

    {
        echo "$OVERRIDE_MARKER (install.sh $SCRIPT_VERSION)."
        echo "# Docker Compose lo combina con docker-compose.yml. Se regenera al volver a"
        echo "# ejecutar el instalador (usa --port / --sftp-port / --bind para cambiarlo)."
        echo "services:"
        if [[ "$need_ports" == "true" || "$need_external" == "true" ]]; then
            echo "  skypanel:"
        fi
        if [[ "$need_ports" == "true" ]]; then
            echo "    ports: !override"
            echo "      - \"$web_mapping\" # Panel web"
            echo "      - \"$SFTP_PORT:$SFTP_PORT\" # SFTP (el panel escucha en este puerto: daemon.sftp.host)"
        fi
        if [[ "$need_external" == "true" ]]; then
            echo "    environment:"
            # Las variables se interpolan desde .env: la contraseña no queda en este archivo
            # shellcheck disable=SC2016
            echo '      SKYPANEL_PANEL_DATABASE_URL: "${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:${DB_PORT})/${DB_DATABASE}?charset=utf8&parseTime=true"'
            echo "    extra_hosts:"
            echo "      - \"host.docker.internal:host-gateway\""
        fi
        if [[ "$need_mysql" == "true" ]]; then
            echo "  mysql:"
            echo "    ports: !override"
            # shellcheck disable=SC2016
            echo '      - "${DB_BIND_ADDRESS:-127.0.0.1}:'"$MYSQL_HOST_PORT"':3306"'
        fi
    } | write_file "$file" 644 show

    if [[ "$DRY_RUN" != "true" && -d "$INSTALL_DIR" ]]; then
        if ! (cd "$INSTALL_DIR" && "${DC[@]}" config -q); then
            die "docker compose rechaza la configuración generada en $file"
        fi
    fi
    log_success "docker-compose.override.yml listo"
}

# =============================================================================
# CONFIGURACIÓN DEL PANEL (config.json)
#   En Docker el panel lee /var/lib/SkyPanel/config.json (storage/skypanel-data)
#   salvo que exista /etc/SkyPanel/config.json (storage/skypanel-config). El
#   panel también escribe en él desde Ajustes, así que solo se edita con el
#   contenedor parado. Las variables SKYPANEL_* del entorno tendrían prioridad
#   y los cambios desde Ajustes se perderían al reiniciar: por eso se usa el
#   archivo y no el entorno.
# =============================================================================

panel_config_file() {
    if [[ -f "$INSTALL_DIR/storage/skypanel-config/config.json" ]]; then
        echo "$INSTALL_DIR/storage/skypanel-config/config.json"
    else
        echo "$INSTALL_DIR/storage/skypanel-data/config.json"
    fi
}

# cfg_get CLAVE (sin distinguir mayúsculas, como viper)
cfg_get() {
    local file
    file=$(panel_config_file)
    [[ -r "$file" ]] && command -v jq >/dev/null 2>&1 || return 0
    jq -r --arg k "$1" '
        def g($p): reduce $p[] as $s (.;
            if type == "object" then (to_entries | map(select(.key | ascii_downcase == $s)) | (.[0].value // null)) else null end);
        g($k | ascii_downcase | split(".")) // empty | if type == "array" then join(",") else tostring end' "$file" 2>/dev/null || true
}

# cfg_set CLAVE TIPO(string|bool|int|array) VALOR [secret]
cfg_set() {
    CFG_KEYS+=("$1"); CFG_TYPES+=("$2"); CFG_VALS+=("$3"); CFG_SECRET+=("${4:-}")
}

compute_master_url() {
    if [[ -n "$MASTER_URL" ]]; then
        MASTER_URL="${MASTER_URL%/}"
        return 0
    fi
    # Sin cambios de dominio/IP/puerto en esta ejecución: la URL de la instalación anterior
    if [[ "$MASTER_URL_EXPLICIT" != "true" && -n "$PREV_MASTER_URL" ]]; then
        MASTER_URL="$PREV_MASTER_URL"
        return 0
    fi
    if [[ -n "$DOMAIN" ]]; then
        if [[ "$SKIP_NGINX" == "true" ]]; then
            # Sin Nginx del instalador: o hay un proxy propio (SSL) o se entra por el puerto
            if [[ "$USE_SSL" == "true" ]]; then MASTER_URL="https://$DOMAIN"; else MASTER_URL="http://$DOMAIN:$PANEL_PORT"; fi
        elif [[ "$USE_SSL" == "true" ]]; then
            MASTER_URL="https://$DOMAIN"
        else
            MASTER_URL="http://$DOMAIN"
        fi
    else
        MASTER_URL="http://$SERVER_IP:$PANEL_PORT"
    fi
}

# Decide qué ajustes del panel hay que escribir
queue_panel_config() {
    CFG_KEYS=(); CFG_TYPES=(); CFG_VALS=(); CFG_SECRET=()

    if [[ "$IS_NODE_ONLY" != "true" ]]; then
        # URL maestra: los enlaces de recuperación de contraseña y el SSO de facturación
        # se construyen SOLO con ella. Se respeta la que el admin haya cambiado en Ajustes
        # salvo que en esta ejecución se indique dominio/IP/puerto/URL.
        local current
        current=$(cfg_get panel.settings.masterurl)
        if [[ -z "$current" || "$MASTER_URL_EXPLICIT" == "true" ]]; then
            if [[ "$current" != "$MASTER_URL" ]]; then
                cfg_set panel.settings.masterurl string "$MASTER_URL"
                MASTER_URL_WRITTEN=true
            fi
        elif [[ "$current" != "$MASTER_URL" ]]; then
            log_info "Se mantiene la URL maestra configurada en el panel: $current"
            MASTER_URL="$current"
        fi
    fi

    # SFTP: el panel escucha en daemon.sftp.host y anuncia ese puerto a los usuarios
    local sftp_cur
    sftp_cur=$(cfg_get daemon.sftp.host)
    if [[ "$SFTP_PORT" != "$DEFAULT_SFTP_PORT" || ( -n "$sftp_cur" && "$sftp_cur" != "0.0.0.0:$SFTP_PORT" ) ]]; then
        cfg_set daemon.sftp.host string "0.0.0.0:$SFTP_PORT"
    fi

    # Cloudflare Turnstile (sustituye al captcha integrado, que viene activado por defecto)
    if [[ -n "$TURNSTILE_SITE_KEY" ]]; then
        cfg_set panel.turnstile.enabled bool true
        cfg_set panel.turnstile.sitekey string "$TURNSTILE_SITE_KEY"
        cfg_set panel.turnstile.secretkey string "$TURNSTILE_SECRET_KEY" secret
    fi

    # Subdominios automáticos con Cloudflare DNS
    if [[ -n "$CF_DNS_TOKEN" ]]; then cfg_set panel.dns.cloudflare.token string "$CF_DNS_TOKEN" secret; fi
    if [[ -n "$CF_ZONE_ID" ]]; then cfg_set panel.dns.cloudflare.zoneid string "$CF_ZONE_ID"; fi
    if [[ -n "$DNS_BASE_DOMAIN" ]]; then cfg_set panel.dns.basedomain string "$DNS_BASE_DOMAIN"; fi
    if [[ -n "$DNS_TARGET" ]]; then cfg_set panel.dns.target string "$DNS_TARGET"; fi
    if [[ -n "$DNS_AUTO_ASSIGN" ]]; then
        cfg_set panel.dns.autoassign bool "$(truthy_env "$DNS_AUTO_ASSIGN")"
    fi

    # Copias de seguridad en S3
    if [[ -n "$S3_BUCKET" ]]; then
        cfg_set daemon.backups.s3.enabled bool true
        cfg_set daemon.backups.s3.bucket string "$S3_BUCKET"
        cfg_set daemon.backups.s3.accesskey string "$S3_ACCESS_KEY" secret
        cfg_set daemon.backups.s3.secretkey string "$S3_SECRET_KEY" secret
        if [[ -n "$S3_ENDPOINT" ]]; then cfg_set daemon.backups.s3.endpoint string "$S3_ENDPOINT"; fi
        if [[ -n "$S3_REGION" ]]; then cfg_set daemon.backups.s3.region string "$S3_REGION"; fi
        if [[ -n "$S3_PREFIX" ]]; then cfg_set daemon.backups.s3.prefix string "$S3_PREFIX"; fi
        if [[ -n "$S3_INSECURE" ]]; then cfg_set daemon.backups.s3.insecure bool "$(truthy_env "$S3_INSECURE")"; fi
    fi

    if [[ -n "$CURSEFORGE_KEY" ]]; then cfg_set daemon.curseforge.key string "$CURSEFORGE_KEY" secret; fi
    if [[ -n "$LOG_RETENTION_DAYS" ]]; then cfg_set logretentiondays int "$LOG_RETENTION_DAYS"; fi
}

# Detrás de Nginx el panel ve todas las peticiones desde la puerta de enlace de su red
# Docker: se confía en ella para leer la IP real (límites por IP, captcha, registros).
queue_trusted_proxies() {
    [[ "$NGINX_CONFIGURED" == "true" ]] || return 0
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[dry-run] Se confiaría en la puerta de enlace de la red Docker del panel (security.trustedproxies)"
        return 0
    fi
    [[ -z "$(cfg_get security.trustedproxies)" ]] || return 0
    local nets net gw=""
    nets=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' "$PANEL_CONTAINER" 2>/dev/null || true)
    for net in $nets; do
        gw=$(docker network inspect -f '{{range .IPAM.Config}}{{.Gateway}} {{end}}' "$net" 2>/dev/null || true)
        gw=${gw%% *}
        [[ -n "$gw" ]] && break
    done
    if validate_ip "$gw"; then
        cfg_set security.trustedproxies array "$gw"
    fi
}

apply_panel_config() {
    local n=${#CFG_KEYS[@]} i file
    (( n > 0 )) || return 0
    file=$(panel_config_file)

    if [[ "$DRY_RUN" == "true" ]]; then
        printf '%s[dry-run]%s se guardarían en %s:\n' "$CYAN" "$NC" "$file"
        for ((i = 0; i < n; i++)); do
            if [[ -n "${CFG_SECRET[$i]}" ]]; then
                printf '    | %s = (oculto)\n' "${CFG_KEYS[$i]}"
            else
                printf '    | %s = %s\n' "${CFG_KEYS[$i]}" "${CFG_VALS[$i]}"
            fi
        done
        return 0
    fi

    if ! command -v jq >/dev/null 2>&1; then
        log_warning "jq no está instalado: configura a mano en el panel (Ajustes):"
        for ((i = 0; i < n; i++)); do log_warning "  ${CFG_KEYS[$i]}"; done
        return 0
    fi

    # Programa jq: las claves son constantes de este script; los valores llegan por
    # variables de entorno (no aparecen en la lista de procesos)
    local filter expr
    # shellcheck disable=SC2016 # $p/$v/$par son variables de jq, no de bash
    filter='def setci($p; $v): ($p[:-1]) as $par | ($p[-1]) as $leaf
        | setpath($par; ((getpath($par) // {}) | with_entries(select((.key | ascii_downcase) != $leaf))) + {($leaf): $v});
        .'
    for ((i = 0; i < n; i++)); do
        case "${CFG_TYPES[$i]}" in
            bool) expr="(env.AETHER_CFG_$i == \"true\")" ;;
            int) expr="(env.AETHER_CFG_$i | tonumber)" ;;
            array) expr="(env.AETHER_CFG_$i | split(\",\"))" ;;
            *) expr="env.AETHER_CFG_$i" ;;
        esac
        filter+=" | setci(\"${CFG_KEYS[$i]}\" | split(\".\"); $expr)"
    done

    mkdir -p "$(dirname "$file")"
    local tmp src
    tmp=$(mktemp "$file.tmp.XXXXXX")
    TMP_FILES+=("$tmp")
    src="$file"
    if [[ ! -s "$file" ]]; then
        src=$(mktemp); TMP_FILES+=("$src"); echo '{}' >"$src"
    else
        cp -p "$file" "$file.bak-installer"
        chmod 600 "$file.bak-installer"
    fi
    if ! (
        for ((i = 0; i < n; i++)); do export "AETHER_CFG_$i=${CFG_VALS[$i]}"; done
        jq "$filter" "$src" >"$tmp"
    ); then
        log_warning "No se pudo actualizar $file (¿JSON inválido?). Configura los ajustes desde el panel."
        return 0
    fi
    chmod 600 "$tmp"
    mv -f "$tmp" "$file"
    for ((i = 0; i < n; i++)); do
        log_success "Ajuste del panel: ${CFG_KEYS[$i]}"
    done
}

# =============================================================================
# CONSTRUCCIÓN E INICIO DE CONTENEDORES
# =============================================================================

# El Dockerfile fija "ARG TARGETPLATFORM=linux/amd64" en la etapa de compilación: en un
# host arm64 genera un binario amd64 dentro de una imagen arm64 y el panel no arranca
# ("ld-musl-x86_64.so.1" no encontrado). Se avisa antes de una compilación de 15 minutos.
check_build_platform() {
    local df="$INSTALL_DIR/Dockerfile"
    [[ "$ARCH" == "arm64" && -r "$df" ]] || return 0
    if grep -qE '^[[:space:]]*ARG[[:space:]]+TARGETPLATFORM=linux/amd64' "$df"; then
        log_warning "Este servidor es arm64, pero el Dockerfile de la rama $GIT_BRANCH compila el panel para amd64"
        log_warning "(ARG TARGETPLATFORM=linux/amd64): el contenedor no podrá arrancar en esta máquina."
        log_warning "Usa un servidor amd64 o una rama donde el Dockerfile no fije TARGETPLATFORM."
        if [[ "$FORCE" != "true" && "$DRY_RUN" != "true" ]]; then
            confirm_or_exit "¿Construir igualmente?"
            if ! interactive_manual; then
                die "Compilación detenida (usa --force para intentarlo de todos modos)"
            fi
        fi
    fi
}

build_images() {
    check_build_platform
    print_step "Construyendo la imagen Docker"
    log_info "Esto puede tardar 5-15 minutos la primera vez. El panel actual (si existe) sigue funcionando mientras tanto."
    if ! dc build; then
        log_error "La construcción de la imagen falló."
        log_error "Causas habituales: poca memoria (añade swap), poco disco (docker system prune) o sin red."
        die "Revisa los mensajes anteriores y vuelve a ejecutar el instalador"
    fi
    log_success "Imagen construida"
}

panel_local_host() {
    if [[ "$PANEL_BIND" == "0.0.0.0" ]]; then echo 127.0.0.1; else echo "$PANEL_BIND"; fi
}

# pull_images: descarga las imágenes que no se construyen aquí (MariaDB, Adminer), con
# reintentos. Docker Hub limita las descargas sin cuenta por IP (100 cada 6 horas), y en un
# servidor con IP compartida se agotan: entonces se explica en vez de dar un error de Docker.
pull_images() {
    if [[ "$DRY_RUN" == "true" ]]; then
        dc pull --ignore-buildable
        return 0
    fi
    local attempt out
    out=$(mktemp)
    TMP_FILES+=("$out")
    for attempt in 1 2 3; do
        if dc pull --ignore-buildable 2>&1 | tee "$out"; then
            return 0
        fi
        if (( attempt < 3 )); then
            log_warning "No se pudieron descargar las imágenes; reintento en $((attempt * 30)) s..."
            sleep $((attempt * 30))
        fi
    done
    if grep -qiE '429|too many requests|toomanyrequests' "$out"; then
        die "Docker Hub ha limitado las descargas desde esta IP. Espera unas horas o inicia sesión con 'docker login' y vuelve a ejecutar el instalador (es seguro repetirlo)."
    fi
    die "No se pudieron descargar las imágenes de Docker. Revisa la conexión a Internet y vuelve a ejecutar el instalador."
}

start_stack() {
    print_step "Iniciando $PROJECT_NAME"

    pull_images

    local -a recreate=()
    if [[ "$FORCE" == "true" ]]; then recreate=(--force-recreate); fi

    # Se crean los contenedores sin arrancarlos: así se puede escribir la configuración
    # del panel antes de su primer arranque (o con él parado)
    dc up -d --no-start "${recreate[@]+"${recreate[@]}"}"
    queue_trusted_proxies

    if (( ${#CFG_KEYS[@]} > 0 )); then
        if [[ "$DRY_RUN" != "true" ]] && container_running "$PANEL_CONTAINER"; then
            log_info "Deteniendo el panel para guardar su configuración (los servidores se detienen con él)..."
            dc stop "$PANEL_CONTAINER"
        fi
        apply_panel_config
    fi

    dc up -d
    if ! wait_for_panel; then
        PANEL_NOT_READY=true
    fi
    return 0
}

wait_for_panel() {
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[dry-run] Se esperaría a que el panel responda en http://$(panel_local_host):$PANEL_PORT/" # NOSONAR: comprobación local, el panel solo sirve http dentro del servidor
        return 0
    fi
    log_info "Esperando a que el panel responda (migraciones de base de datos incluidas)..."
    PANEL_START_FAILED=false
    local url code status restarts start=$SECONDS next=15
    # el panel escucha en http dentro del servidor (Nginx pone el https delante): esta
    # comprobación va a la propia máquina, nunca sale por la red
    url="http://$(panel_local_host):$PANEL_PORT/" # NOSONAR
    while (( SECONDS - start < 300 )); do
        code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 "$url" 2>/dev/null || true) # NOSONAR: misma comprobación local
        if [[ "$code" =~ ^[1-5][0-9][0-9]$ ]]; then
            log_success "Panel respondiendo en $url (HTTP $code)" # NOSONAR: dirección local, ver arriba
            return 0
        fi
        status=$(docker inspect -f '{{.State.Status}}' "$PANEL_CONTAINER" 2>/dev/null || echo "missing")
        restarts=$(docker inspect -f '{{.RestartCount}}' "$PANEL_CONTAINER" 2>/dev/null || echo 0)
        if [[ "$status" == "exited" || "$status" == "dead" || "$status" == "missing" ]] || (( restarts >= 3 )); then
            log_error "El contenedor $PANEL_CONTAINER no arranca (estado: $status, reinicios: $restarts). Últimas líneas:"
            docker logs --tail 40 "$PANEL_CONTAINER" 2>&1 | sed 's/^/    /' >&2 || true
            if [[ "$ROLLBACK_ON_FAILURE" == "true" ]]; then
                PANEL_START_FAILED=true
                return 1
            fi
            die "Revisa los logs con: docker logs $PANEL_CONTAINER"
        fi
        if (( SECONDS - start >= next )); then
            log_info "Esperando... ($((SECONDS - start))s, estado: $status)"
            next=$((next + 15))
        fi
        sleep 3
    done
    log_warning "El panel aún no responde tras 5 minutos; puede seguir arrancando."
    log_warning "Comprueba: docker logs -f $PANEL_CONTAINER"
    return 1
}

cleanup_existing() {
    print_step "Recreando contenedores existentes (los datos se conservan)"
    # Solo se paran y eliminan los contenedores: storage/ (datos) no se toca
    if [[ -f "$INSTALL_DIR/docker-compose.yml" ]]; then
        dc down --remove-orphans || log_warning "docker compose down terminó con avisos"
    fi
}

# =============================================================================
# CONFIGURACIÓN DEL FIREWALL
#   Nota: Docker publica sus puertos por delante de ufw/firewalld; estas reglas
#   cubren el resto del sistema. Para limitar el panel usa --bind 127.0.0.1.
# =============================================================================

configure_firewall() {
    if [[ "$SKIP_FIREWALL" == "true" ]]; then
        log_info "Firewall omitido (--skip-firewall)"
        return 0
    fi

    print_step "Configurando firewall"
    detect_ssh_ports

    local -a tcp_ports=()
    if [[ "$PANEL_BIND" == "0.0.0.0" ]]; then tcp_ports+=("$PANEL_PORT"); fi
    tcp_ports+=("$SFTP_PORT")
    # Nginx atiende el puerto 80 también sin dominio (proxy por IP), y con dominio el 443.
    # Sin abrir el 80, un dominio con el proxy de Cloudflare da 522: no llega al servidor.
    if [[ "$SKIP_NGINX" != "true" ]]; then
        tcp_ports+=(80)
        if [[ -n "$DOMAIN" ]]; then tcp_ports+=(443); fi
    fi

    local p
    case "$OS_FAMILY" in
        debian|arch)
            if ! command -v ufw >/dev/null 2>&1; then
                log_warning "ufw no está instalado; no se configura el firewall"
                return 0
            fi
            # SSH primero, para no perder la conexión al activar ufw
            for p in "${SSH_PORTS[@]}"; do run ufw allow "$p/tcp" comment 'SSH'; done
            for p in "${tcp_ports[@]}"; do run ufw allow "$p/tcp" comment 'Aether Panel'; done
            local ufw_status
            ufw_status=$(ufw status 2>/dev/null || true)
            if [[ "$ufw_status" != *"Status: active"* ]]; then
                log_info "Activando ufw (SSH permitido en: ${SSH_PORTS[*]})"
                run ufw --force enable
            fi
            ;;
        fedora|rhel|suse)
            if ! command -v firewall-cmd >/dev/null 2>&1; then
                log_warning "firewalld no está instalado; no se configura el firewall"
                return 0
            fi
            local started_firewalld=false
            if [[ "$HAS_SYSTEMD" == "true" ]] && ! systemctl is-active --quiet firewalld; then
                # Primero las reglas permanentes de SSH, luego se arranca
                for p in "${SSH_PORTS[@]}"; do run firewall-offline-cmd --add-port="$p/tcp" || true; done
                svc enable --now firewalld
                started_firewalld=true
            fi
            run firewall-cmd --permanent --add-service=ssh
            for p in "${SSH_PORTS[@]}"; do run firewall-cmd --permanent --add-port="$p/tcp"; done
            for p in "${tcp_ports[@]}"; do run firewall-cmd --permanent --add-port="$p/tcp"; done
            run firewall-cmd --reload
            # firewalld borra al arrancar las reglas que Docker creó para sus redes, y Docker
            # solo las repone cuando ya encontró firewalld en marcha al iniciarse. Sin esto los
            # contenedores no se ven entre sí y el panel no llega a MySQL.
            if [[ "$started_firewalld" == "true" ]] && systemctl is-active --quiet docker; then
                log_info "Reiniciando Docker para que recree sus reglas en firewalld"
                svc restart docker
            fi
            ;;
    esac

    log_success "Firewall: SSH (${SSH_PORTS[*]}) y ${tcp_ports[*]} permitidos"
}

# =============================================================================
# CONFIGURACIÓN DE NGINX (+ Let's Encrypt)
# =============================================================================

nginx_paths() {
    if [[ -d /etc/nginx/sites-available || "$OS_FAMILY" == "debian" ]]; then
        NGINX_CONF="/etc/nginx/sites-available/skypanel"
        NGINX_LINK="/etc/nginx/sites-enabled/skypanel"
    else
        NGINX_CONF="/etc/nginx/conf.d/skypanel.conf"
        NGINX_LINK=""
        nginx_include_conf_d
    fi
}

# El nginx.conf de Arch no lee conf.d: se añade la línea dentro del bloque http (una vez)
nginx_include_conf_d() {
    local main=/etc/nginx/nginx.conf
    [[ -f "$main" ]] || return 0
    if grep -Eq '^[^#]*include[[:space:]]+(/etc/nginx/)?conf\.d/\*\.conf' "$main"; then return 0; fi
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[dry-run] Se añadiría 'include /etc/nginx/conf.d/*.conf;' a $main"
        return 0
    fi
    run mkdir -p /etc/nginx/conf.d
    cp -p "$main" "$main.antes-aether"
    if ! sed -i '0,/^[[:space:]]*http[[:space:]]*{/s//&\n    include \/etc\/nginx\/conf.d\/*.conf;/' "$main" || ! grep -q 'include /etc/nginx/conf.d/\*.conf' "$main"; then
        cp -p "$main.antes-aether" "$main"
        log_warning "No se pudo añadir conf.d a $main: añade 'include /etc/nginx/conf.d/*.conf;' dentro de http { }"
        return 0
    fi
    log_info "Nginx: $main ahora lee /etc/nginx/conf.d (copia en $main.antes-aether)"
}

nginx_proxy_locations() {
    local upstream
    upstream="http://$(panel_local_host):$PANEL_PORT"
    cat <<EOF
    # Subidas de archivos y backups grandes
    client_max_body_size 0;

    location / {
        proxy_pass $upstream;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$skypanel_connection_upgrade;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        # Consola y websockets: conexiones largas
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_request_buffering off;
    }
EOF
}

# nginx_site_config SERVER_NAME http|ssl [default]
# "default" hace de este sitio el predeterminado del puerto 80: sin dominio, el panel contesta
# por la IP y por cualquier dominio que apunte aquí, en vez de la página de bienvenida de Nginx
nginx_site_config() {
    local name="$1" mode="$2" default="${3:-}" acme_root="/var/www/html" listen_opts=""
    if [[ "$default" == "default" ]]; then listen_opts=" default_server"; name="_"; fi
    cat <<EOF
# Aether Panel - generado por install.sh $SCRIPT_VERSION.
# Se regenera al volver a ejecutar el instalador.
map \$http_upgrade \$skypanel_connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80$listen_opts;
    listen [::]:80$listen_opts;
    server_name $name;

    location /.well-known/acme-challenge/ {
        root $acme_root;
    }
EOF
    if [[ "$mode" == "ssl" ]]; then
        cat <<EOF

    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name $name;

    ssl_certificate /etc/letsencrypt/live/$name/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$name/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SkyPanelSSL:10m;
    ssl_session_timeout 1d;

    # Con Cloudflare "Always Use HTTPS" el reto de una renovación llega por aquí
    location /.well-known/acme-challenge/ {
        root $acme_root;
    }

EOF
        nginx_proxy_locations
        echo "}"
    else
        echo ""
        nginx_proxy_locations
        echo "}"
    fi
}

# nginx_take_default_server: deja libre el papel de sitio predeterminado del puerto 80. Quita
# solo el enlace al sitio de bienvenida de la distribución (el archivo se queda en
# sites-available y se recupera con ln -s); si otro sitio ya es el predeterminado, no lo toca
# y devuelve 1.
nginx_take_default_server() {
    local distro_default="/etc/nginx/sites-enabled/default"
    if [[ -L "$distro_default" && "$(readlink -f "$distro_default")" == "/etc/nginx/sites-available/default" ]]; then
        log_info "Desactivando el sitio de bienvenida de Nginx (queda en sites-available/default)"
        run rm -f "$distro_default"
    fi
    local f
    for f in /etc/nginx/nginx.conf /etc/nginx/conf.d/*.conf /etc/nginx/sites-enabled/*; do
        [[ -f "$f" ]] || continue
        [[ "$(readlink -f "$f")" == "$(readlink -f "$NGINX_CONF" 2>/dev/null)" ]] && continue
        if grep -Eqs '^[[:space:]]*listen[^;#]*[[:space:]:]80[[:space:]][^;#]*default_server' "$f"; then
            log_warning "Otro sitio de Nginx ($f) es el predeterminado del puerto 80: el panel contestará solo por la IP"
            return 1
        fi
    done
    return 0
}

# Escribe la configuración y la prueba; si nginx -t falla se restaura la anterior
install_nginx_site() {
    local content="$1" backup=""
    if [[ -f "$NGINX_CONF" && "$DRY_RUN" != "true" ]]; then
        backup=$(mktemp); TMP_FILES+=("$backup")
        cp -p "$NGINX_CONF" "$backup"
    fi
    printf '%s\n' "$content" | write_file "$NGINX_CONF" 644 show
    if [[ -n "$NGINX_LINK" ]]; then run ln -sf "$NGINX_CONF" "$NGINX_LINK"; fi
    if [[ "$DRY_RUN" == "true" ]]; then return 0; fi
    local out
    if ! out=$(nginx -t 2>&1); then
        printf '%s\n' "$out" | sed 's/^/    /' >&2
        if [[ -n "$backup" ]]; then
            cp -p "$backup" "$NGINX_CONF"
        else
            rm -f "$NGINX_CONF"
            if [[ -n "$NGINX_LINK" ]]; then rm -f "$NGINX_LINK"; fi
        fi
        die "La configuración de Nginx no es válida (se ha restaurado la anterior)"
    fi
    if [[ "$HAS_SYSTEMD" == "true" ]]; then
        if systemctl is-active --quiet nginx; then
            run systemctl reload nginx
        else
            run systemctl enable --now nginx
        fi
    else
        nginx -s reload 2>/dev/null || nginx
    fi
}

# Lo que hace falta para el certificado, sobre todo con Cloudflare delante
print_ssl_requirements() {
    echo ""
    log_info "Certificado de Let's Encrypt: se valida por HTTP en el puerto 80 de este servidor."
    echo "   • Sin Cloudflare, o con la nube gris: el dominio debe apuntar a esta IP y el puerto 80"
    echo "     estar abierto (también en el firewall del proveedor)."
    echo "   • Con el proxy de Cloudflare (nube naranja) también funciona, sin quitarlo, si el 80 llega:"
    echo "       - Modo SSL \"Flexible\" o \"Full\"; \"Full (strict)\" cuando el certificado ya exista."
    echo "       - La PRIMERA emisión falla si \"Always Use HTTPS\" está activado y el servidor aún no"
    echo "         tiene certificado: desactívalo unos minutos, o pon la nube gris solo para emitirlo."
    echo "       - Las renovaciones automáticas funcionan con la nube naranja y \"Always Use HTTPS\":"
    echo "         Nginx responde al reto también por HTTPS."
    echo "   • Otra opción sin validar por este servidor: un certificado de origen de Cloudflare"
    echo "     (modo Full strict), que se instala a mano en Nginx."
    echo "   • Los juegos y el SFTP no pasan por el proxy de Cloudflare: el panel les dará la IP."
    echo ""
}

check_domain_dns() {
    local resolved
    resolved=$(getent ahostsv4 "$DOMAIN" 2>/dev/null | awk '{print $1}' | sort -u | tr '\n' ' ' || true)
    if [[ -z "$resolved" ]]; then
        log_warning "El dominio $DOMAIN no resuelve todavía: Let's Encrypt fallará hasta que el DNS apunte aquí"
        return 0
    fi
    if [[ -n "$PUBLIC_IP" ]] && ! contains_word "$resolved" "$PUBLIC_IP"; then
        log_warning "$DOMAIN apunta a ${resolved% } y la IP pública de este servidor es $PUBLIC_IP"
        log_warning "(normal si usas el proxy de Cloudflare; si no, el certificado fallará)"
    else
        log_success "DNS de $DOMAIN: ${resolved% }"
    fi
}

request_certificate() {
    local acme_root="/var/www/html" attempt
    local -a email_args=(--register-unsafely-without-email)
    if [[ -n "$SSL_EMAIL" ]]; then email_args=(--email "$SSL_EMAIL" --no-eff-email); fi

    run mkdir -p "$acme_root/.well-known/acme-challenge"
    log_info "Emitiendo certificado SSL para $DOMAIN ..."
    for attempt in 1 2; do
        # --keep-until-expiring: al reinstalar no se pide otro certificado (límites de Let's Encrypt)
        # --deploy-hook: tras cada renovación automática Nginx carga el certificado nuevo
        if run certbot certonly --webroot -w "$acme_root" -d "$DOMAIN" \
            --non-interactive --agree-tos --keep-until-expiring \
            --deploy-hook "systemctl reload nginx" "${email_args[@]}"; then
            return 0
        fi
        if (( attempt == 1 )); then
            log_warning "Primer intento de certbot falló; reintentando..."
            svc restart nginx || true
            sleep 3
        fi
    done

    log_error "Certbot falló tras 2 intentos."
    log_error ""
    log_error "Posibles causas:"
    log_error "  1. El puerto 80 no está abierto desde internet."
    log_error "     Abre el puerto 80 y 443 en el firewall de tu proveedor de hosting"
    log_error "     (Vultr, Hetzner, etc.) además de UFW del servidor."
    log_error "  2. El DNS de '$DOMAIN' no apunta a la IP pública de este servidor."
    log_error "     Verifica: dig +short $DOMAIN"
    log_error ""
    log_error "Una vez resuelto, vuelve a ejecutar el instalador con: --domain $DOMAIN --ssl"
    log_error ""
    log_error "Se usará HTTP mientras tanto."
    return 1
}

configure_nginx() {
    if [[ "$SKIP_NGINX" == "true" ]]; then
        log_info "Nginx omitido (--skip-nginx)"
        return 0
    fi

    print_step "Configurando Nginx"

    if ! command -v nginx >/dev/null 2>&1; then
        case "$OS_FAMILY" in
            debian) apt_install nginx ;;
            fedora|rhel) dnf_install nginx ;;
        esac
    fi
    if ! command -v nginx >/dev/null 2>&1 && [[ "$DRY_RUN" != "true" ]]; then
        log_warning "Nginx no está disponible; se omite el proxy"
        return 0
    fi
    nginx_paths

    # SELinux (RHEL/Fedora): Nginx necesita permiso para conectar con el panel
    if command -v getenforce >/dev/null 2>&1 && [[ "$(getenforce 2>/dev/null)" == "Enforcing" ]]; then
        run setsebool -P httpd_can_network_connect 1 || log_warning "No se pudo activar httpd_can_network_connect"
    fi

    if [[ -n "$DOMAIN" && "$USE_SSL" == "true" ]]; then
        # === HTTPS con dominio ===
        check_domain_dns
        if [[ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]]; then
            log_success "Ya existe un certificado para $DOMAIN"
        else
            # FASE 1: config HTTP para el reto ACME (sin certificados nginx -t fallaría)
            install_nginx_site "$(nginx_site_config "$DOMAIN" http)"
            if ! command -v certbot >/dev/null 2>&1; then
                case "$OS_FAMILY" in
                    debian) apt_install certbot || true ;;
                    *) dnf_install certbot || true ;;
                esac
            fi
            if ! request_certificate; then
                USE_SSL=false
                SSL_FAILED=true
            fi
        fi
        # FASE 2: config SSL definitivo (los certificados ya existen)
        if [[ "$USE_SSL" == "true" ]]; then
            install_nginx_site "$(nginx_site_config "$DOMAIN" ssl)"
            log_success "Nginx + SSL configurado"
        else
            log_success "Nginx configurado (sin SSL)"
        fi
    elif [[ -n "$DOMAIN" ]]; then
        # --- HTTP con dominio ---
        install_nginx_site "$(nginx_site_config "$DOMAIN" http)"
        log_success "Nginx configurado"
    else
        # IP sin dominio (proxy en el puerto 80 como en versiones anteriores). Si se puede, es el
        # sitio predeterminado, así un dominio que ya apunte aquí también llega al panel.
        if nginx_take_default_server; then
            install_nginx_site "$(nginx_site_config "$SERVER_IP" http default)"
        else
            install_nginx_site "$(nginx_site_config "$SERVER_IP" http)"
        fi
        log_success "Nginx configurado"
    fi
    NGINX_CONFIGURED=true
}

# =============================================================================
# CUENTA ADMINISTRADOR Y CREDENCIALES
# =============================================================================

# save_credentials "título" "LÍNEA" ... -> se añade a un archivo solo legible por root
save_credentials() {
    local title="$1"; shift
    if [[ "$DRY_RUN" == "true" ]]; then
        printf '%s[dry-run]%s se guardaría "%s" en %s (permisos 600)\n' "$CYAN" "$NC" "$title" "$CREDENTIALS_FILE"
        return 0
    fi
    mkdir -p "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"
    (
        umask 077
        {
            echo ""
            echo "# $(date '+%Y-%m-%d %H:%M:%S') - $title"
            printf '%s\n' "$@"
        } >>"$CREDENTIALS_FILE"
    )
    chmod 600 "$CREDENTIALS_FILE"
}

create_admin() {
    if [[ "$CREATE_ADMIN" != "true" ]]; then return 0; fi
    if [[ "$IS_NODE_ONLY" == "true" ]]; then
        log_info "Nodo solo daemon: las cuentas se crean en el panel maestro"
        return 0
    fi

    print_step "Creando cuenta administrador"

    if [[ "$PANEL_NOT_READY" == "true" ]]; then
        log_warning "El panel aún no responde: crea la cuenta más tarde con --create-admin"
        return 0
    fi
    if [[ -z "$ADMIN_EMAIL" ]] && [[ "$CAN_PROMPT" == "true" ]]; then
        ask_input ADMIN_EMAIL "Email del administrador" "" sensitive
    fi
    if ! validate_email "$ADMIN_EMAIL"; then
        log_error "Email del administrador inválido: '$ADMIN_EMAIL'"
        ADMIN_CREATED=false
        return 0
    fi
    if [[ -z "$ADMIN_PASSWORD" ]]; then
        ADMIN_PASSWORD=$(gen_secret)
        ADMIN_PASSWORD_GENERATED=true
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        printf '%s[dry-run]%s docker exec %s /SkyPanel/bin/SkyPanel user add --name %q --email %q --password (oculta) --admin%s\n' \
            "$CYAN" "$NC" "$PANEL_CONTAINER" "$ADMIN_NAME" "$ADMIN_EMAIL" "$([[ "$REPLACE_ADMIN" == "true" ]] && echo ' --force')"
        ADMIN_CREATED=true
        return 0
    fi

    local -a args=(user add --name "$ADMIN_NAME" --email "$ADMIN_EMAIL" --admin)
    # --force BORRA y recrea un usuario con el mismo email/nombre: solo si se pide
    if [[ "$REPLACE_ADMIN" == "true" ]]; then args+=(--force); fi

    local output
    # La contraseña viaja por variable de entorno, no en la línea de comandos de docker
    output=$(AETHER_ADMIN_PW="$ADMIN_PASSWORD" docker exec -e AETHER_ADMIN_PW "$PANEL_CONTAINER" \
        sh -c 'exec /SkyPanel/bin/SkyPanel "$@" --password "$AETHER_ADMIN_PW"' sh "${args[@]}" 2>&1) || true

    if [[ "$output" == *"User added"* ]]; then
        ADMIN_CREATED=true
        log_success "Cuenta administrador creada: $ADMIN_EMAIL"
        if [[ "$ADMIN_PASSWORD_GENERATED" == "true" ]]; then
            save_credentials "Cuenta administrador del panel" "Usuario: $ADMIN_NAME" "Email: $ADMIN_EMAIL" "Contraseña: $ADMIN_PASSWORD"
        fi
    else
        ADMIN_CREATED=false
        log_warning "No se pudo crear la cuenta automáticamente:"
        printf '%s\n' "$output" | sed 's/^/    /' >&2
        if [[ "$output" == *[Dd]uplicate* || "$output" == *UNIQUE* || "$output" == *exist* ]]; then
            log_warning "Ya existe un usuario con ese email o nombre. Para reemplazarlo usa --replace-admin"
        fi
    fi
}

# =============================================================================
# RESUMEN FINAL
# =============================================================================

print_summary() {
    local title="${1:-Instalación completada}"
    print_step "$title"

    echo ""
    echo "============================================================"
    printf '  %s✓ %s%s\n' "$GREEN" "$title" "$NC"
    echo "============================================================"
    echo ""

    if [[ "$IS_NODE_ONLY" == "true" ]]; then
        echo "📍 Nodo (solo daemon) escuchando en el puerto $PANEL_PORT"
    else
        printf '📍 Accede al panel: %s%s%s\n' "$GREEN" "$MASTER_URL" "$NC"
        if [[ "$NGINX_CONFIGURED" == "true" && -n "$DOMAIN" && "$PANEL_BIND" == "0.0.0.0" ]]; then
            echo "   (también directo en http://$SERVER_IP:$PANEL_PORT)" # NOSONAR: la dirección directa del panel es http
        fi
        if [[ "$MASTER_URL_WRITTEN" == "true" ]]; then
            echo "   URL maestra guardada en el panel (enlaces de recuperación de contraseña y SSO)."
        fi
    fi
    if [[ "$SSL_FAILED" == "true" ]]; then
        printf '   %s⚠  No se pudo obtener el certificado SSL: el panel funciona por HTTP.%s\n' "$YELLOW" "$NC"
        echo "      Cuando el DNS y el puerto 80 estén listos: install.sh --domain $DOMAIN --ssl"
    fi

    echo ""
    echo "📋 Puertos:"
    echo "   • Panel web: $PANEL_PORT$([[ "$PANEL_BIND" != "0.0.0.0" ]] && echo " (solo en $PANEL_BIND)")"
    echo "   • SFTP: $SFTP_PORT (contraseña o llave SSH)"
    if [[ "$NGINX_CONFIGURED" == "true" ]]; then
        echo "   • Nginx: 80$([[ -n "$DOMAIN" ]] && echo " y 443")"
    fi
    echo "   • Adminer (BD): 127.0.0.1:8081 (solo local; usa un túnel SSH: ssh -L 8081:127.0.0.1:8081 ...)"
    local fw_ports="${SSH_PORTS[*]:-22}"
    if [[ "$PANEL_BIND" == "0.0.0.0" ]]; then fw_ports+=" $PANEL_PORT"; fi
    fw_ports+=" $SFTP_PORT"
    if [[ "$NGINX_CONFIGURED" == "true" ]]; then fw_ports+=" 80"; [[ -n "$DOMAIN" ]] && fw_ports+=" 443"; fi
    echo "   Abre en el firewall de tu proveedor (Hetzner, OVH, AWS…) los TCP: ${fw_ports// /, }"
    echo "   y el puerto de cada servidor de juego que crees (p. ej. 25565 para Minecraft)."
    if [[ "$SKIP_FIREWALL" != "true" ]]; then
        echo "   En este servidor ya están abiertos (Docker publica sus puertos por delante de ufw/firewalld)."
    fi

    if [[ "$DRY_RUN" != "true" ]] && command -v docker >/dev/null 2>&1; then
        echo ""
        echo "🐳 Contenedores:"
        docker ps -a --filter "label=com.docker.compose.project.working_dir=$INSTALL_DIR" --format '   • {{.Names}}: {{.Status}}' 2>/dev/null || true
    fi

    if [[ "$IS_NODE_ONLY" != "true" ]]; then
        db_settings
        echo ""
        if [[ "$DBS_MODE" == "external" ]]; then
            echo "🗄  Base de datos externa: $DBS_HOST:$DBS_PORT, base de datos $DBS_NAME, usuario $DBS_USER"
        else
            echo "🗄  Base de datos: MariaDB en Docker ($MYSQL_CONTAINER), base de datos $DBS_NAME, usuario $DBS_USER"
        fi
        echo "   Contraseñas en $INSTALL_DIR/.env (solo root)."
    fi

    if [[ "$IS_NODE_ONLY" != "true" ]]; then
        echo ""
        echo "🔐 Seguridad:"
        if [[ -n "$TURNSTILE_SITE_KEY" || "$(cfg_get panel.turnstile.enabled)" == "true" ]]; then
            echo "   • Anti-bots: Cloudflare Turnstile activado."
        else
            echo "   • Anti-bots: captcha integrado activado (sin claves; se resuelve en el navegador)."
            echo "     Para usar Cloudflare Turnstile: Ajustes del panel, o --turnstile-site-key/--turnstile-secret-key."
        fi
        echo "   • Registro público de usuarios: activado por defecto (docker-compose.yml)."
        if [[ "$DEFAULT_DB_PASSWORDS_IN_USE" == "true" ]]; then
            printf '   %s• ⚠  La base de datos usa contraseñas por defecto PÚBLICAS.%s\n' "$YELLOW" "$NC"
            echo "     Cámbialas con: install.sh --update --rotate-db-passwords"
        else
            echo "   • Contraseñas de la base de datos: $INSTALL_DIR/.env (solo root)."
        fi
    fi

    echo ""
    if [[ "$ADMIN_CREATED" == "true" ]]; then
        echo "============================================================"
        printf '  %s✓ Cuenta administrador creada%s\n' "$GREEN" "$NC"
        echo "============================================================"
        printf '  Usuario:    %s%s%s\n' "$GREEN" "$ADMIN_NAME" "$NC"
        printf '  Email:      %s%s%s\n' "$GREEN" "$ADMIN_EMAIL" "$NC"
        if [[ "$ADMIN_PASSWORD_GENERATED" == "true" ]]; then
            echo "  Contraseña: generada; está en $CREDENTIALS_FILE (solo root):"
            echo "              sudo cat $CREDENTIALS_FILE"
        else
            echo "  Contraseña: la que indicaste"
        fi
        echo ""
        printf '  %s⚠  Cambia la contraseña tras el primer inicio de sesión.%s\n' "$YELLOW" "$NC"
        if [[ "$ADMIN_PASSWORD_GENERATED" == "true" ]]; then
            printf '  %s   Borra %s cuando la tengas a salvo.%s\n' "$YELLOW" "$CREDENTIALS_FILE" "$NC"
        fi
    elif [[ "$IS_NODE_ONLY" != "true" ]] && { [[ "$EXISTING_INSTALL" != "true" ]] || [[ "$CREATE_ADMIN" == "true" ]]; }; then
        echo "============================================================"
        printf '  %s⚠  Crea tu cuenta administrador%s\n' "$YELLOW" "$NC"
        echo "============================================================"
        echo "  Ejecuta (te pedirá la contraseña, que así no queda en el historial):"
        echo "   docker exec -it $PANEL_CONTAINER /SkyPanel/bin/SkyPanel user add --name admin --email tu@email.com --admin"
        echo "  O con este instalador:"
        echo "   sudo bash install.sh --create-admin --admin-email tu@email.com"
    fi

    echo ""
    echo "============================================================"
    echo "  ⚡ Próximos pasos"
    echo "============================================================"
    echo "1. Entra en $MASTER_URL e inicia sesión."
    echo "2. Configura el correo (Ajustes) para que funcione la recuperación de contraseñas."
    echo "3. Opcional en Ajustes: Turnstile, subdominios con Cloudflare, copias en S3, CurseForge."
    echo ""
    local dcmd="${DC[*]:-docker compose}"
    echo "🛠  Administración (desde $INSTALL_DIR: cd $INSTALL_DIR):"
    echo "   • Estado:              $dcmd ps"
    echo "   • Logs en directo:     $dcmd logs -f skypanel     (todos: $dcmd logs -f)"
    echo "   • Reiniciar:           $dcmd restart"
    echo "   • Detener / arrancar:  $dcmd stop   /   $dcmd start"
    if [[ "$IS_NODE_ONLY" != "true" ]]; then
        echo "   • Crear administrador: docker exec -it $PANEL_CONTAINER /SkyPanel/bin/SkyPanel user add --name NOMBRE --email EMAIL --admin"
        echo "                          (pide la contraseña; también: sudo bash install.sh --create-admin --admin-email EMAIL)"
        echo "   • Editar un usuario:   docker exec -it $PANEL_CONTAINER /SkyPanel/bin/SkyPanel user edit"
        echo "                          (nombre, email, contraseña, administrador, quitar 2FA)"
    fi
    echo "   • Versión:             docker exec $PANEL_CONTAINER /SkyPanel/bin/SkyPanel version"
    echo "   • Actualizar:          sudo bash install.sh --update   (o desde Ajustes del panel)"
    echo "   • Reinstalar / quitar: sudo bash install.sh --reinstall   /   --uninstall"
    echo "     (el instalador: curl -fsSL $INSTALL_URL -o install.sh)"
    echo ""
    echo "📁 Archivos:"
    echo "   • Panel y docker-compose:  $INSTALL_DIR"
    echo "   • Datos persistentes:      $INSTALL_DIR/storage (BD, servidores, copias, extensiones, logs)"
    echo "   • Opciones del instalador: $STATE_FILE"
    echo "   • Copias del instalador:   $BACKUP_ROOT"
    echo ""
}

# =============================================================================
# ACTUALIZACIÓN SIN BORRAR DATOS
#   Actualiza el código y reconstruye la imagen Docker manteniendo
#   la base de datos y configuración existentes.
# =============================================================================

update_panel() {
    print_step "Actualizando $PROJECT_NAME (sin borrar datos)"

    if [[ ! -d "$INSTALL_DIR/.git" ]]; then
        log_error "No se detectó una instalación de $PROJECT_NAME en $INSTALL_DIR"
        log_info "Ejecuta el instalador completo primero: sudo bash install.sh"
        exit 1
    fi

    ensure_docker_running
    resolve_branch
    if [[ -z "$DB_MODE" ]]; then DB_MODE="${PREV_DB_MODE:-local}"; fi
    if [[ -z "$PANEL_PORT" ]]; then PANEL_PORT="${PREV_PANEL_PORT:-$DEFAULT_PANEL_PORT}"; fi
    if [[ -z "$SFTP_PORT" ]]; then SFTP_PORT="${PREV_SFTP_PORT:-$DEFAULT_SFTP_PORT}"; fi
    if [[ -z "$PANEL_BIND" ]]; then PANEL_BIND="${PREV_PANEL_BIND:-0.0.0.0}"; fi
    if [[ -z "$DOMAIN" && "$NO_DOMAIN" != "true" ]]; then DOMAIN="$PREV_DOMAIN"; fi
    if [[ -z "$USE_SSL" ]]; then USE_SSL="${PREV_USE_SSL:-false}"; fi
    if [[ -z "$SERVER_IP" ]]; then SERVER_IP="$PREV_SERVER_IP"; fi
    if [[ -z "$SERVER_IP" ]]; then
        LOCAL_IP=$(detect_local_ip)
        SERVER_IP="${LOCAL_IP:-127.0.0.1}"
    fi
    MYSQL_HOST_PORT="${PREV_MYSQL_HOST_PORT:-$DEFAULT_MYSQL_HOST_PORT}"
    if [[ -f /etc/nginx/sites-available/skypanel || -f /etc/nginx/conf.d/skypanel.conf ]]; then
        NGINX_CONFIGURED=true
    fi
    if [[ -z "$CREATE_ADMIN" ]]; then CREATE_ADMIN=false; fi
    if [[ "$DOMAIN" != "$PREV_DOMAIN" || "$USE_SSL" != "${PREV_USE_SSL:-false}" ]] && [[ -n "$PREV_DOMAIN$DOMAIN" ]]; then
        log_warning "--update no reconfigura Nginx ni SSL: para cambiar el dominio ejecuta el instalador sin --update"
    fi
    validate_configuration

    # Versión de antes, para volver a ella si la nueva no arranca
    PRE_UPDATE_COMMIT=$(git_in rev-parse HEAD 2>/dev/null || true)
    log_info "Versión instalada: $(git_in log -1 --format='%h del %cs' 2>/dev/null || echo '?') (rama $(git_in rev-parse --abbrev-ref HEAD 2>/dev/null || echo '?'))"
    backup_before_update

    sync_repository
    if [[ "$(git_in rev-parse HEAD 2>/dev/null || true)" == "$PRE_UPDATE_COMMIT" ]]; then
        log_info "No hay cambios nuevos en la rama $GIT_BRANCH; se reconstruye y reinicia igualmente"
    fi
    configure_env_file
    configure_compose_override
    check_external_db
    tag_rollback_image
    build_images
    rotate_db_passwords
    compute_master_url
    queue_panel_config
    ROLLBACK_ON_FAILURE=true
    start_stack
    ROLLBACK_ON_FAILURE=false
    if [[ "$PANEL_START_FAILED" == "true" ]]; then
        rollback_update
    fi
    create_admin
    save_state
    install_domain_service
    record_domain_status
    print_summary "Actualización completada"
    log_info "Los datos y la configuración se mantuvieron intactos"
}

# =============================================================================
# COPIAS DE SEGURIDAD
#   La base de datos se copia con mariadb-dump (una transacción, sin parar el
#   panel) y se comprueba antes de seguir: gzip íntegro y con tablas dentro.
#   Las contraseñas pasan por MYSQL_PWD en el entorno, nunca en la línea de
#   comandos ni en los logs.
# =============================================================================

# db_settings: modo, host, puerto, nombre y usuario de la base de datos del panel (sin contraseñas)
db_settings() {
    DBS_MODE="${DB_MODE:-${PREV_DB_MODE:-local}}"
    if [[ -n "$(env_get DB_HOST)" ]]; then DBS_MODE="external"; fi
    DBS_NAME="$(env_get DB_DATABASE)"; DBS_NAME="${DBS_NAME:-${DB_NAME:-skypanel}}"
    DBS_USER="$(env_get DB_USER)"; DBS_USER="${DBS_USER:-${DB_USER:-skypanel}}"
    DBS_HOST="$(env_get DB_HOST)"; DBS_HOST="${DBS_HOST:-$DB_HOST}"
    DBS_PORT="$(env_get DB_PORT)"; DBS_PORT="${DBS_PORT:-${DB_PORT:-3306}}"
}

# backup_database CARPETA: copia comprobada de la base de datos; deja la ruta en LAST_DB_BACKUP
backup_database() {
    local dest="$1" file stamp ok=false
    db_settings
    stamp=$(date +%Y%m%d-%H%M%S)
    file="$dest/basedatos-$stamp.sql.gz"
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[dry-run] Se copiaría la base de datos ($DBS_MODE) en $file"
        LAST_DB_BACKUP="$file"
        return 0
    fi
    run mkdir -p "$dest"
    chmod 700 "$BACKUP_ROOT" "$dest" 2>/dev/null || true
    log_info "Copiando la base de datos ($DBS_MODE, $DBS_NAME)..."
    (
        umask 077
        if [[ "$DBS_MODE" == "local" ]]; then
            if ! container_running "$MYSQL_CONTAINER"; then
                dc up -d mysql >/dev/null 2>&1 || true
                local i
                for i in $(seq 1 30); do
                    [[ "$(docker inspect -f '{{.State.Health.Status}}' "$MYSQL_CONTAINER" 2>/dev/null)" == "healthy" ]] && break
                    sleep 2
                done
            fi
            MYSQL_PWD="$(env_get DB_ROOT_PASSWORD)" docker exec -i -e MYSQL_PWD "$MYSQL_CONTAINER" \
                mariadb-dump -uroot --single-transaction --routines --triggers --databases "$DBS_NAME" | gzip >"$file"
        else
            MYSQL_PWD="$(env_get DB_PASSWORD)" docker run --rm -i --network host -e MYSQL_PWD mariadb:10.11 \
                mariadb-dump -h "$DBS_HOST" -P "$DBS_PORT" -u "$DBS_USER" --single-transaction --routines --triggers --databases "$DBS_NAME" | gzip >"$file"
        fi
    ) && ok=true
    if [[ "$ok" == "true" ]] && gzip -t "$file" 2>/dev/null && gzip -dc "$file" | grep -q -m1 'CREATE TABLE'; then
        LAST_DB_BACKUP="$file"
        log_success "Copia de la base de datos comprobada: $file ($(du -h "$file" | cut -f1))"
        # se guardan las 10 más recientes de esta carpeta
        find "$dest" -maxdepth 1 -name 'basedatos-*.sql.gz' -printf '%T@ %p\n' 2>/dev/null | sort -rn | tail -n +11 | cut -d' ' -f2- | xargs -r rm -f
        return 0
    fi
    rm -f "$file"
    log_error "La copia de la base de datos falló o salió vacía"
    return 1
}

# restore_hint ARCHIVO: cómo devolver la base de datos a una copia
restore_hint() {
    db_settings
    if [[ "$DBS_MODE" == "local" ]]; then
        echo "   gzip -dc $1 | sudo docker exec -i -e MYSQL_PWD=\"\$(grep ^DB_ROOT_PASSWORD= $INSTALL_DIR/.env | cut -d= -f2-)\" $MYSQL_CONTAINER mariadb -uroot"
    else
        echo "   gzip -dc $1 | mariadb -h $DBS_HOST -P $DBS_PORT -u $DBS_USER -p"
    fi
}

backup_before_update() {
    if [[ "$SKIP_BACKUP" == "true" ]]; then
        log_warning "Actualización sin copia de la base de datos (--skip-backup)"
        return 0
    fi
    print_step "Copia de seguridad antes de actualizar"
    if backup_database "$BACKUP_ROOT/actualizaciones"; then return 0; fi
    if interactive_manual; then
        local ans
        ask_yes_no ans "No se pudo copiar la base de datos. ¿Actualizar igualmente, sin copia?" "no"
        [[ "$ans" == "yes" ]] && return 0
    fi
    die "Actualización cancelada: sin una copia de la base de datos no se sigue. Comprueba que la base de datos funciona, o usa --skip-backup bajo tu responsabilidad."
}

# tag_rollback_image: guarda la imagen actual del panel antes de reconstruir
tag_rollback_image() {
    ROLLBACK_IMAGE=""
    [[ "$DRY_RUN" == "true" ]] && return 0
    local image
    image=$(docker inspect -f '{{.Config.Image}}' "$PANEL_CONTAINER" 2>/dev/null || true)
    [[ -n "$image" ]] || return 0
    image="${image%:latest}"
    if docker image tag "$image:latest" "$image:anterior" 2>/dev/null; then
        ROLLBACK_IMAGE="$image"
        log_info "Imagen actual guardada como $image:anterior (para volver atrás si la nueva falla)"
    fi
}

# rollback_update: la versión nueva no arranca; se vuelve al código y la imagen de antes
rollback_update() {
    print_step "Volviendo a la versión anterior"
    log_error "La versión nueva del panel no arranca. Los datos no se han borrado."
    if [[ -n "$PRE_UPDATE_COMMIT" ]]; then
        backup_env_file
        run "${GIT[@]}" reset -q --hard "$PRE_UPDATE_COMMIT"
        restore_env_file
        log_info "Código devuelto a $(git_in log -1 --format='%h del %cs')"
    fi
    if [[ -n "$ROLLBACK_IMAGE" ]]; then
        run docker image tag "$ROLLBACK_IMAGE:anterior" "$ROLLBACK_IMAGE:latest"
    else
        build_images
    fi
    dc up -d
    if wait_for_panel; then
        log_success "El panel vuelve a funcionar con la versión anterior"
    fi
    echo ""
    if [[ -n "$LAST_DB_BACKUP" ]]; then
        echo "La copia de la base de datos de antes de actualizar está en:"
        echo "   $LAST_DB_BACKUP"
        echo "Si la versión nueva llegó a cambiar la base de datos y algo no va bien, se restaura con:"
        restore_hint "$LAST_DB_BACKUP"
    fi
    die "Actualización revertida. Revisa los mensajes anteriores (docker logs $PANEL_CONTAINER) antes de reintentar."
}

# =============================================================================
# BASE DE DATOS EXTERNA: se comprueba antes de construir nada
# =============================================================================

# mysql_external SQL [usuario]: ejecuta SQL contra la BD externa (contraseña en MYSQL_PWD)
mysql_external() {
    local sql="$1" user="${2:-$DBS_USER}" db="${3:-}"
    local -a args=(-h "$DBS_HOST" -P "$DBS_PORT" -u "$user" --connect-timeout=10)
    if [[ -n "$db" ]]; then args+=("$db"); fi
    printf '%s\n' "$sql" | docker run --rm -i --network host -e MYSQL_PWD mariadb:10.11 mariadb "${args[@]}"
}

check_external_db() {
    [[ "$DB_MODE" == "external" ]] || return 0
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[dry-run] Se comprobaría la conexión a la base de datos externa"
        return 0
    fi
    print_step "Comprobando la base de datos externa"
    db_settings
    DBS_HOST="${DB_HOST:-$DBS_HOST}"; DBS_PORT="${DB_PORT:-$DBS_PORT}"; DBS_NAME="${DB_NAME:-$DBS_NAME}"; DBS_USER="${DB_USER:-$DBS_USER}"
    local pass="${DB_PASSWORD:-$(env_get DB_PASSWORD)}"
    case "$DBS_HOST" in
        localhost|127.*|::1)
            die "La base de datos externa no puede ser '$DBS_HOST': dentro del contenedor del panel eso es el propio contenedor. Usa la IP privada de este servidor o el hostname del servidor de base de datos." ;;
    esac

    if MYSQL_PWD="$pass" mysql_external "SELECT 1;" "$DBS_USER" "$DBS_NAME" >/dev/null 2>&1; then
        log_success "Conexión correcta: $DBS_USER@$DBS_HOST:$DBS_PORT/$DBS_NAME"
        return 0
    fi

    if [[ -n "$DB_ADMIN_USER" && -n "$DB_ADMIN_PASSWORD" ]]; then
        log_info "Creando la base de datos '$DBS_NAME' y el usuario '$DBS_USER' con $DB_ADMIN_USER..."
        # solo los permisos sobre esa base de datos; la contraseña va por la entrada, no por argumentos
        local sql
        sql="CREATE DATABASE IF NOT EXISTS \`$DBS_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '$DBS_USER'@'%' IDENTIFIED BY '$pass';
GRANT ALL PRIVILEGES ON \`$DBS_NAME\`.* TO '$DBS_USER'@'%';
FLUSH PRIVILEGES;"
        if ! MYSQL_PWD="$DB_ADMIN_PASSWORD" mysql_external "$sql" "$DB_ADMIN_USER" >/dev/null 2>&1; then
            die "No se pudo crear la base de datos o el usuario con la cuenta '$DB_ADMIN_USER' (¿contraseña correcta, permiso CREATE USER, acceso desde esta IP?)"
        fi
        if MYSQL_PWD="$pass" mysql_external "SELECT 1;" "$DBS_USER" "$DBS_NAME" >/dev/null 2>&1; then
            log_success "Base de datos y usuario listos: $DBS_USER@$DBS_HOST:$DBS_PORT/$DBS_NAME"
            return 0
        fi
    fi

    log_error "No se puede conectar a $DBS_USER@$DBS_HOST:$DBS_PORT/$DBS_NAME. Comprueba:"
    log_error "  • que el servidor de base de datos acepta conexiones desde esta IP (bind-address, firewall);"
    log_error "  • el usuario y la contraseña, y que la base de datos existe"
    log_error "    (o indica una cuenta de administrador para crearlas: --db-admin-user / --db-admin-pass)."
    die "Base de datos externa no accesible: no se ha cambiado nada"
}

# =============================================================================
# DOMINIO DESDE AJUSTES (servicio aether-domain)
#   Al guardar un dominio en Ajustes → General → URL principal, el servidor tiene
#   que configurar Nginx, el certificado y el firewall, y eso el panel (dentro de
#   Docker) no lo puede hacer. Una unidad .path de systemd vigila el config.json
#   del panel y ejecuta "install.sh --apply-panel-url", que usa las mismas
#   funciones que la instalación. El resultado queda en domain-status.json, junto
#   al config.json, y el panel lo muestra bajo el campo.
# =============================================================================

DOMAIN_SERVICE="aether-domain"
# Tras un certificado fallido no se vuelve a pedir el mismo hasta pasada una hora:
# Let's Encrypt limita los intentos fallidos por dominio
DOMAIN_RETRY_SECONDS=3600

domain_status_file() { echo "$(dirname "$(panel_config_file)")/domain-status.json"; }

# domain_proxy_check: DOMAIN_PROXIED=true cuando el dominio no apunta a este servidor, que es lo
# que pasa con el proxy de Cloudflare (nube naranja). La web llega igual, pero los juegos
# (Minecraft en el 25565…) y el SFTP no pasan por ese proxy: el panel les da entonces la IP
# del servidor (DOMAIN_SERVER_IP) en lugar del dominio.
domain_proxy_check() {
    DOMAIN_PROXIED=false
    DOMAIN_SERVER_IP=""
    [[ -n "$DOMAIN" ]] || return 0
    local resolved ip
    resolved=$(getent ahostsv4 "$DOMAIN" 2>/dev/null | awk '{print $1}' | sort -u | tr '\n' ' ' || true)
    [[ -n "$resolved" ]] || return 0
    ip="${PUBLIC_IP:-}"
    if [[ -z "$ip" ]]; then ip=$(fetch_public_ip || true); fi
    if [[ -z "$ip" ]]; then ip="$SERVER_IP"; fi
    if [[ -n "$ip" ]] && ! contains_word "$resolved" "$ip"; then
        DOMAIN_PROXIED=true
        DOMAIN_SERVER_IP="$ip"
        log_info "$DOMAIN pasa por un proxy (${resolved% }): jugadores y SFTP usarán la IP $ip"
    fi
}

# write_domain_status ESTADO(working|ok|error) URL MENSAJE
write_domain_status() {
    local file tmp https=false proxied=false
    file=$(domain_status_file)
    [[ -d "$(dirname "$file")" ]] || return 0
    if [[ "$DRY_RUN" == "true" ]]; then
        printf '%s[dry-run]%s estado del dominio: %s (%s)\n' "$CYAN" "$NC" "$1" "$3"
        return 0
    fi
    if [[ "$USE_SSL" == "true" ]]; then https=true; fi
    if [[ "$DOMAIN_PROXIED" == "true" ]]; then proxied=true; fi
    tmp=$(mktemp "$file.XXXXXX")
    jq -n --arg state "$1" --arg url "$2" --arg message "$3" \
        --arg domain "$DOMAIN" --argjson https "$https" \
        --argjson proxied "$proxied" --arg serverIp "${DOMAIN_SERVER_IP:-}" \
        --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        '{state: $state, url: $url, domain: $domain, https: $https, proxied: $proxied, serverIp: $serverIp, message: $message, updatedAt: $at}' >"$tmp"
    # el panel lo lee con su propio usuario
    chmod 644 "$tmp"
    mv -f "$tmp" "$file"
}

apply_panel_url() {
    if [[ ! -d "$INSTALL_DIR/.git" ]]; then
        log_error "No hay una instalación de $PROJECT_NAME en $INSTALL_DIR"
        exit 1
    fi
    local url host scheme port want_domain="" want_ssl=false
    url=$(cfg_get panel.settings.masterUrl)
    url="${url%/}"
    if [[ -z "$url" ]]; then
        log_info "Ajustes no tiene URL principal: no hay nada que aplicar"
        return 0
    fi
    if [[ ! "$url" =~ ^(https?)://([^/:]+)(:([0-9]+))?(/.*)?$ ]]; then
        log_warning "La URL principal no es válida: $url"
        return 0
    fi
    scheme="${BASH_REMATCH[1]}"; host="${BASH_REMATCH[2],,}"; port="${BASH_REMATCH[4]}"

    if validate_ip "$host" || [[ "$host" == "localhost" ]]; then
        want_domain=""
    elif [[ -n "$port" ]]; then
        # http://dominio:8080 entra directo al panel: Nginx no interviene
        log_info "La URL principal lleva puerto ($url): no se configura Nginx"
        return 0
    elif validate_domain "$host"; then
        want_domain="$host"
        if [[ "$scheme" == "https" ]]; then want_ssl=true; fi
    else
        log_warning "La URL principal no tiene un dominio válido: $url"
        return 0
    fi

    if [[ "$want_domain" == "${PREV_DOMAIN:-}" && "$want_ssl" == "${PREV_USE_SSL:-false}" ]]; then
        log_info "El servidor ya está configurado para $url"
        # instalaciones de antes del servicio, o una URL que vuelve a la ya aplicada: Ajustes lo
        # muestra como activo
        DOMAIN="$want_domain"; USE_SSL="$want_ssl"
        if [[ -z "$SERVER_IP" ]]; then SERVER_IP="${PREV_SERVER_IP:-}"; fi
        domain_proxy_check
        if [[ "$(jq -r '.state + " " + .url + " " + (.proxied|tostring)' "$(domain_status_file)" 2>/dev/null || true)" != "ok $url $DOMAIN_PROXIED" ]]; then
            write_domain_status ok "$url" "El servidor ya está configurado para $url"
        fi
        return 0
    fi

    # Un certificado que acaba de fallar no se pide otra vez en cada cambio de Ajustes
    local status_file last_state last_url last_at last_epoch now
    status_file=$(domain_status_file)
    if [[ -r "$status_file" ]]; then
        last_state=$(jq -r '.state // empty' "$status_file" 2>/dev/null || true)
        last_url=$(jq -r '.url // empty' "$status_file" 2>/dev/null || true)
        last_at=$(jq -r '.updatedAt // empty' "$status_file" 2>/dev/null || true)
        last_epoch=$(date -u -d "$last_at" +%s 2>/dev/null || echo 0)
        now=$(date -u +%s)
        if [[ "$last_state" == "error" && "$last_url" == "$url" ]] && (( now - last_epoch < DOMAIN_RETRY_SECONDS )); then
            log_info "El último intento para $url falló hace menos de una hora; se reintentará después"
            return 0
        fi
    fi

    print_step "Aplicando la URL principal de Ajustes: $url"
    DOMAIN="$want_domain"
    USE_SSL="$want_ssl"
    if [[ -z "$DOMAIN" ]]; then NO_DOMAIN=true; fi
    SSL_EMAIL="${SSL_EMAIL:-${PREV_SSL_EMAIL:-}}"
    if [[ -z "$PANEL_PORT" ]]; then PANEL_PORT="${PREV_PANEL_PORT:-$DEFAULT_PANEL_PORT}"; fi
    if [[ -z "$SFTP_PORT" ]]; then SFTP_PORT="${PREV_SFTP_PORT:-$DEFAULT_SFTP_PORT}"; fi
    if [[ -z "$PANEL_BIND" ]]; then PANEL_BIND="${PREV_PANEL_BIND:-0.0.0.0}"; fi
    if [[ -z "$SERVER_IP" ]]; then SERVER_IP="${PREV_SERVER_IP:-}"; fi
    if [[ -z "$SERVER_IP" ]]; then
        LOCAL_IP=$(detect_local_ip)
        SERVER_IP="${LOCAL_IP:-127.0.0.1}"
    fi
    if [[ -z "$GIT_BRANCH" ]]; then GIT_BRANCH="${PREV_GIT_BRANCH:-$DEFAULT_GIT_BRANCH}"; fi
    if [[ -z "$DB_MODE" ]]; then DB_MODE="${PREV_DB_MODE:-local}"; fi
    MYSQL_HOST_PORT="${PREV_MYSQL_HOST_PORT:-$DEFAULT_MYSQL_HOST_PORT}"
    # la URL ya está en Ajustes: no se vuelve a escribir en el config.json
    MASTER_URL="$url"
    write_domain_status working "$url" "Configurando Nginx${DOMAIN:+ para $DOMAIN}"

    configure_firewall
    configure_nginx
    save_state
    domain_proxy_check

    if [[ "$SSL_FAILED" == "true" ]]; then
        write_domain_status error "$url" "No se pudo obtener el certificado para $DOMAIN: comprueba que el dominio apunta a este servidor y que el puerto 80 está abierto. El panel funciona por HTTP mientras tanto."
    elif [[ -n "$DOMAIN" && "$USE_SSL" == "true" ]]; then
        write_domain_status ok "$url" "Nginx y el certificado HTTPS de $DOMAIN están configurados"
    elif [[ -n "$DOMAIN" ]]; then
        write_domain_status ok "$url" "Nginx está configurado para $DOMAIN (sin HTTPS)"
    else
        write_domain_status ok "$url" "Nginx contesta por la IP del servidor"
    fi
}

install_domain_service() {
    if [[ "$IS_NODE_ONLY" == "true" || "$HAS_SYSTEMD" != "true" || "$SKIP_NGINX" == "true" ]]; then return 0; fi
    local config_file script="$INSTALL_DIR/tools/install/install.sh"
    config_file=$(panel_config_file)
    if [[ ! -f "$script" ]]; then
        log_warning "No está $script: el dominio de Ajustes no se aplicará solo"
        return 0
    fi
    printf '%s\n' \
        "[Unit]" \
        "Description=Aether Panel: aplicar la URL principal de Ajustes (Nginx, certificado, firewall)" \
        "After=network-online.target docker.service" \
        "" \
        "[Service]" \
        "Type=oneshot" \
        "ExecStart=/bin/bash $script --apply-panel-url" |
        write_file "/etc/systemd/system/$DOMAIN_SERVICE.service" 644
    printf '%s\n' \
        "[Unit]" \
        "Description=Aether Panel: vigilar la URL principal de Ajustes" \
        "" \
        "[Path]" \
        "PathChanged=$config_file" \
        "" \
        "[Install]" \
        "WantedBy=multi-user.target" |
        write_file "/etc/systemd/system/$DOMAIN_SERVICE.path" 644
    run systemctl daemon-reload
    run systemctl enable --now "$DOMAIN_SERVICE.path"
    log_success "Dominio desde Ajustes: el servidor aplicará la URL principal al guardarla"
}

# record_domain_status: lo que la instalación dejó configurado, para que Ajustes lo muestre
record_domain_status() {
    if [[ "$IS_NODE_ONLY" == "true" || "$SKIP_NGINX" == "true" ]]; then return 0; fi
    local url
    url=$(cfg_get panel.settings.masterUrl)
    url="${url%/}"
    [[ -n "$url" ]] || return 0
    domain_proxy_check
    if [[ "$SSL_FAILED" == "true" ]]; then
        write_domain_status error "$url" "No se pudo obtener el certificado para $DOMAIN"
    else
        write_domain_status ok "$url" "El servidor está configurado para $url"
    fi
}

remove_domain_service() {
    if [[ ! -f "/etc/systemd/system/$DOMAIN_SERVICE.path" ]]; then return 0; fi
    svc disable --now "$DOMAIN_SERVICE.path" || true
    run rm -f "/etc/systemd/system/$DOMAIN_SERVICE.path" "/etc/systemd/system/$DOMAIN_SERVICE.service"
    run systemctl daemon-reload || true
}

# =============================================================================
# DESINSTALACIÓN
#   basic: para y elimina los contenedores y el sitio de Nginx. Conserva el
#          código, .env y TODOS los datos (storage/).
#   data:  además borra los datos (BD, servidores, backups). Conserva .env,
#          override y opciones.
#   full:  borra todo lo instalado (incluido $INSTALL_DIR y el certificado del
#          dominio del panel).
# =============================================================================

uninstall() {
    print_banner
    require_root
    acquire_lock
    # sin detect_os: solo hace falta saber si hay systemd (servicio del dominio, Nginx)
    if [[ -d /run/systemd/system ]]; then HAS_SYSTEMD=true; fi

    if ! command -v docker >/dev/null 2>&1; then
        log_warning "Docker no está instalado"
    else
        detect_docker
    fi
    if (( ${#DC[@]} == 0 )); then DC=(docker compose); fi
    load_state

    local has_container=false
    if command -v docker >/dev/null 2>&1 && container_exists "$PANEL_CONTAINER" && container_is_ours "$PANEL_CONTAINER"; then has_container=true; fi

    printf '%s════════════════════════════════════════════════════════════%s\n' "$RED" "$NC"
    printf '%s  ⚠️  ADVERTENCIA: DESINSTALACIÓN DE %s%s\n' "$RED" "$PROJECT_NAME" "$NC"
    printf '%s════════════════════════════════════════════════════════════%s\n' "$RED" "$NC"
    echo ""

    if [[ "$has_container" != "true" && ! -d "$INSTALL_DIR" ]]; then
        printf '%sNo se detectó ninguna instalación de %s%s\n' "$YELLOW" "$PROJECT_NAME" "$NC"
        echo ""
        echo "Posibles ubicaciones verificadas:"
        echo "  • Contenedor Docker: $PANEL_CONTAINER"
        echo "  • Directorio de instalación: $INSTALL_DIR"
        echo ""
        exit 0
    fi

    if interactive_manual; then
        local choice def=1
        case "$UNINSTALL_MODE" in data) def=2 ;; full) def=3 ;; *) def=1 ;; esac
        echo "¿Estás seguro de que deseas desinstalar $PROJECT_NAME?"
        echo ""
        echo "Opciones:"
        echo "  1) Quitar el panel pero conservar datos y configuración (se vuelve a levantar con --update)"
        echo "  2) Quitar el panel y BORRAR los datos (conserva código y configuración)"
        echo "  3) Desinstalación completa: BORRA todo lo del panel (datos, código, configuración)"
        echo "  4) Cancelar"
        echo ""
        prompt_read choice "Selecciona [1-4]" "$def"
        case "$choice" in
            1) UNINSTALL_MODE="basic" ;;
            2) UNINSTALL_MODE="data" ;;
            3) UNINSTALL_MODE="full" ;;
            *) echo "Cancelado."; exit 0 ;;
        esac
        if [[ "$UNINSTALL_MODE" != "basic" ]]; then
            db_settings
            echo ""
            printf '%s⚠  Esto es irreversible: se borran la base de datos%s y los servidores de juego.%s
' "$RED" \
                "$([[ "$DBS_MODE" == "external" ]] && echo " local (la externa $DBS_HOST no se toca)")" "$NC"
            local confirm
            prompt_read confirm "Escribe BORRAR para confirmar" ""
            [[ "$confirm" == "BORRAR" ]] || { echo "Cancelado."; exit 0; }
        fi
    else
        if [[ -z "$UNINSTALL_MODE" ]]; then UNINSTALL_MODE="basic"; fi
        if [[ "$UNINSTALL_MODE" != "basic" && "$AUTO_CONFIRMED" != "true" ]]; then
            die "Borrar datos sin terminal requiere confirmación explícita: añade --yes"
        fi
    fi

    if [[ "$UNINSTALL_MODE" != "basic" ]] && interactive_manual && [[ -d "$INSTALL_DIR/storage" ]]; then
        local ans
        ask_yes_no ans "¿Hacer antes una copia de seguridad (base de datos y datos)?" "yes"
        if [[ "$ans" == "yes" ]]; then backup_installation "desinstalacion"; fi
    fi

    print_step "Desinstalando ($UNINSTALL_MODE)"
    remove_installation "$UNINSTALL_MODE" false
    verify_removal "$UNINSTALL_MODE"

    # ============================================
    # RESUMEN FINAL
    # ============================================
    print_step "Desinstalación completada"
    echo ""
    echo "============================================================"
    printf '  %s✓ %s desinstalado%s\n' "$GREEN" "$PROJECT_NAME" "$NC"
    echo "============================================================"
    echo ""
    case "$UNINSTALL_MODE" in
        full)
            echo "✅ Todo eliminado:"
            echo "   • Contenedores, imagen, redes y servidores de juego del panel"
            echo "   • Directorio de instalación y datos ($INSTALL_DIR)"
            echo "   • Opciones y credenciales ($CONFIG_DIR)"
            echo "   • Sitio de Nginx, certificado del panel, servicio $DOMAIN_SERVICE"
            echo "   • Reglas del firewall del panel y del SFTP"
            echo ""
            echo "   Siguen instalados Docker, Nginx y certbot, y las reglas de SSH, 80 y 443."
            if [[ "${DBS_MODE:-}" == "external" ]]; then
                echo "   La base de datos externa ${DBS_HOST:-} no se ha tocado."
            fi
            ;;
        data)
            echo "✅ Eliminado:"
            echo "   • Contenedores y servidores de juego"
            echo "   • Datos ($INSTALL_DIR/storage)"
            echo ""
            echo "✅ Conservado:"
            echo "   • Código, .env y opciones ($INSTALL_DIR, $CONFIG_DIR)"
            ;;
        *)
            echo "✅ Eliminado:"
            echo "   • Contenedores Docker"
            echo "   • Configuración Nginx"
            echo ""
            echo "✅ Conservado:"
            echo "   • Datos ($INSTALL_DIR/storage: base de datos, servidores, backups)"
            echo "   • Código y contraseñas ($INSTALL_DIR, .env)"
            echo ""
            echo "   Para volver a ponerlo en marcha: sudo bash install.sh --update"
            ;;
    esac
    echo ""
    echo "¡Gracias por haber usado $PROJECT_NAME!"
    echo ""
}

# =============================================================================
# REINSTALACIÓN Y LIMPIEZA
#   Solo se borra lo que es de esta instalación: los contenedores creados por
#   docker compose desde $INSTALL_DIR, los servidores de juego del panel
#   (etiqueta skypanel.server), sus redes e imagen, $INSTALL_DIR y $CONFIG_DIR.
#   Una base de datos externa nunca se toca.
# =============================================================================

# ids de los contenedores de esta instalación (compose) y, con "servers", los servidores de juego
project_containers() {
    command -v docker >/dev/null 2>&1 || return 0
    docker ps -aq --filter "label=com.docker.compose.project.working_dir=$INSTALL_DIR" 2>/dev/null || true
    if [[ "${1:-}" == "servers" ]]; then
        docker ps -aq --filter "label=skypanel.server" 2>/dev/null || true
    fi
}

# backup_installation MOTIVO: base de datos + storage (sin los archivos de MariaDB, ya copiados)
# + .env y opciones, en $BACKUP_ROOT/MOTIVO-FECHA
backup_installation() {
    local dest
    dest="$BACKUP_ROOT/$1-$(date +%Y%m%d-%H%M%S)"
    print_step "Copia de seguridad en $dest"
    if [[ -d "$INSTALL_DIR/storage" ]]; then
        log_info "Tamaño de los datos: $(du -sh --exclude=mysql-data "$INSTALL_DIR/storage" 2>/dev/null | cut -f1 || echo '?')"
    fi
    backup_database "$dest" || log_warning "Sin copia de la base de datos (¿estaba el contenedor de MariaDB parado o borrado?)"
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[dry-run] Se copiarían $INSTALL_DIR/storage, .env y $CONFIG_DIR"
        return 0
    fi
    run mkdir -p "$dest"
    chmod 700 "$dest"
    if ! tar -C "$INSTALL_DIR" --exclude='storage/mysql-data' -czf "$dest/datos.tar.gz" storage .env 2>/dev/null; then
        die "La copia de los datos falló; no se ha borrado nada"
    fi
    if [[ -d "$CONFIG_DIR" ]]; then tar -C / -czf "$dest/configuracion.tar.gz" "${CONFIG_DIR#/}" 2>/dev/null || true; fi
    tar -tzf "$dest/datos.tar.gz" >/dev/null 2>&1 || die "La copia de los datos no se puede leer; no se ha borrado nada"
    log_success "Copia de seguridad en $dest ($(du -sh "$dest" | cut -f1))"
}

# remove_installation MODO(basic|data|full) CONSERVAR_CERTIFICADO(true|false)
remove_installation() {
    local mode="$1" keep_cert="${2:-false}"

    # Contenedores del panel (los datos están en $INSTALL_DIR/storage y NO se tocan en basic)
    if [[ -f "$INSTALL_DIR/docker-compose.yml" ]] && command -v docker >/dev/null 2>&1; then
        local -a down_args=(down --remove-orphans)
        if [[ "$mode" == "full" ]]; then down_args+=(--rmi local --volumes); fi
        dc "${down_args[@]}" || log_warning "docker compose down terminó con avisos"
    fi
    local id
    for id in $(project_containers); do
        run docker rm -f "$id" >/dev/null || true
    done

    remove_domain_service

    # Nginx (solo el sitio del panel)
    local conf
    for conf in /etc/nginx/sites-enabled/skypanel /etc/nginx/sites-available/skypanel /etc/nginx/conf.d/skypanel.conf; do
        if [[ -e "$conf" || -L "$conf" ]]; then
            run rm -f "$conf"
            NGINX_CONFIGURED=true
        fi
    done
    if [[ "$NGINX_CONFIGURED" == "true" ]] && command -v nginx >/dev/null 2>&1; then
        log_info "Configuración de Nginx del panel eliminada"
        if nginx -t >/dev/null 2>&1; then svc reload nginx || true; fi
    fi

    if [[ "$mode" == "data" || "$mode" == "full" ]]; then
        # Servidores de juego creados por el panel: llevan su etiqueta
        if command -v docker >/dev/null 2>&1; then
            local ids
            ids=$(docker ps -aq --filter "label=skypanel.server" 2>/dev/null || true)
            if [[ -n "$ids" ]]; then
                log_info "Eliminando contenedores de servidores de juego..."
                # shellcheck disable=SC2086 # lista de IDs separada por espacios
                run docker rm -f $ids >/dev/null || true
            fi
            if docker network inspect "$SERVERS_NETWORK" >/dev/null 2>&1 && [[ -z "$(docker network inspect -f '{{range .Containers}}x{{end}}' "$SERVERS_NETWORK" 2>/dev/null)" ]]; then
                run docker network rm "$SERVERS_NETWORK" >/dev/null || true
            fi
        fi
        log_info "Eliminando datos ($INSTALL_DIR/storage)..."
        run rm -rf "$INSTALL_DIR/storage"
        if [[ -d "$DATA_DIR" ]]; then run rm -rf "$DATA_DIR"; fi
    fi

    if [[ "$mode" == "full" ]]; then
        log_info "Eliminando instalación y configuración..."
        local cert_domain="${PREV_DOMAIN:-$DOMAIN}"
        if command -v docker >/dev/null 2>&1; then
            local image
            for image in $(docker images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -E '^skypanel-skypanel:' || true); do
                run docker image rm -f "$image" >/dev/null || true
            done
        fi
        remove_firewall_rules
        run rm -rf "$INSTALL_DIR" "$CONFIG_DIR"
        local d
        for d in "$LOG_DIR" "$WEB_DIR"; do
            if [[ -d "$d" ]]; then run rm -rf "$d"; fi
        done
        # Solo el certificado del dominio del panel (nunca los de otros sitios)
        if [[ "$keep_cert" != "true" && -n "$cert_domain" && -d "/etc/letsencrypt/live/$cert_domain" ]] && command -v certbot >/dev/null 2>&1; then
            log_info "Eliminando el certificado SSL de $cert_domain..."
            run certbot delete --cert-name "$cert_domain" --non-interactive || true
        fi
    fi
}

# Reglas que añadió el instalador para el panel y el SFTP (las de SSH, 80 y 443 se quedan:
# otros sitios del servidor pueden usarlas)
remove_firewall_rules() {
    local panel="${PREV_PANEL_PORT:-$DEFAULT_PANEL_PORT}" sftp="${PREV_SFTP_PORT:-$DEFAULT_SFTP_PORT}" p
    if command -v ufw >/dev/null 2>&1; then
        for p in "$panel" "$sftp"; do
            if ufw status 2>/dev/null | grep -Eq "^$p/tcp .*# Aether Panel"; then
                run ufw delete allow "$p/tcp" >/dev/null || true
            fi
        done
    elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
        for p in "$panel" "$sftp"; do
            run firewall-cmd --permanent --remove-port="$p/tcp" >/dev/null 2>&1 || true
        done
        run firewall-cmd --reload >/dev/null 2>&1 || true
    fi
}

# verify_removal MODO: comprueba que no queda nada de lo que se ha borrado
verify_removal() {
    [[ "$DRY_RUN" == "true" ]] && return 0
    local mode="$1" left=0
    print_step "Comprobando la limpieza"
    if [[ -n "$(project_containers)" ]]; then log_warning "Quedan contenedores del panel"; left=$((left + 1)); else log_success "Sin contenedores del panel"; fi
    if [[ "$mode" != "basic" ]]; then
        if command -v docker >/dev/null 2>&1 && [[ -n "$(docker ps -aq --filter label=skypanel.server 2>/dev/null)" ]]; then
            log_warning "Quedan servidores de juego"; left=$((left + 1))
        else
            log_success "Sin servidores de juego"
        fi
        if [[ -d "$INSTALL_DIR/storage" ]]; then log_warning "Quedan datos en $INSTALL_DIR/storage"; left=$((left + 1)); else log_success "Datos eliminados"; fi
    fi
    if [[ "$mode" == "full" ]]; then
        if [[ -d "$INSTALL_DIR" || -d "$CONFIG_DIR" ]]; then log_warning "Quedan $INSTALL_DIR o $CONFIG_DIR"; left=$((left + 1)); else log_success "Archivos y configuración eliminados"; fi
        if command -v docker >/dev/null 2>&1 && docker images --format '{{.Repository}}' 2>/dev/null | grep -qx 'skypanel-skypanel'; then
            log_warning "Queda la imagen skypanel-skypanel"; left=$((left + 1))
        else
            log_success "Imagen del panel eliminada"
        fi
    fi
    if [[ -f "/etc/systemd/system/$DOMAIN_SERVICE.path" ]]; then log_warning "Queda el servicio $DOMAIN_SERVICE"; left=$((left + 1)); fi
    if (( left > 0 )); then
        log_warning "Quedaron $left elemento(s): revisa los avisos anteriores"
    fi
}

# reinstall_prepare: avisa, ofrece una copia, pide confirmación, borra la instalación y deja
# todo listo para una instalación nueva (que vuelve a preguntar dominio, puertos y base de datos)
reinstall_prepare() {
    print_step "Reinstalación desde cero"
    if [[ "$EXISTING_INSTALL" != "true" ]]; then
        log_info "No hay ninguna instalación: se hace una instalación normal"
        REINSTALL_MODE=false
        return 0
    fi
    if command -v docker >/dev/null 2>&1; then detect_docker; fi
    if (( ${#DC[@]} == 0 )); then DC=(docker compose); fi
    db_settings
    printf '%s════════════════════════════════════════════════════════════%s\n' "$RED" "$NC"
    printf '%s  ⚠️  REINSTALAR BORRA LOS DATOS DE ESTA INSTALACIÓN%s\n' "$RED" "$NC"
    printf '%s════════════════════════════════════════════════════════════%s\n' "$RED" "$NC"
    echo "Se borrará:"
    echo "   • Contenedores, imagen y redes del panel, y todos los servidores de juego"
    echo "   • $INSTALL_DIR: código, .env y datos (servidores, copias, extensiones, logs)"
    if [[ "$DBS_MODE" == "local" ]]; then
        echo "   • La base de datos local (usuarios, servidores, ajustes)"
    fi
    echo "   • $CONFIG_DIR (opciones y credenciales guardadas), el sitio de Nginx y el servicio $DOMAIN_SERVICE"
    echo "No se tocará:"
    if [[ "$DBS_MODE" == "external" ]]; then
        echo "   • La base de datos externa $DBS_HOST:$DBS_PORT/$DBS_NAME: sus datos siguen ahí; si la"
        echo "     vuelves a indicar, el panel nuevo los usará. Bórrala tú si quieres empezar de cero."
    fi
    echo "   • Docker, Nginx, certbot, el certificado del dominio y otros contenedores del servidor"
    echo ""

    local backup=true
    if interactive_manual; then
        local ans
        ask_yes_no ans "¿Hacer antes una copia de seguridad (base de datos y datos)?" "yes"
        [[ "$ans" == "yes" ]] || backup=false
        local confirm
        prompt_read confirm "Escribe REINSTALAR para borrar y reinstalar" ""
        [[ "$confirm" == "REINSTALAR" ]] || { echo "Cancelado."; exit 0; }
    else
        [[ "$AUTO_CONFIRMED" == "true" ]] || die "Reinstalar sin terminal borra datos: confirma con --yes"
        [[ "$SKIP_BACKUP" == "true" ]] && backup=false
    fi
    if [[ "$backup" == "true" ]]; then backup_installation "reinstalacion"; fi

    remove_installation full true
    verify_removal full

    # instalación nueva: nada de la anterior se usa como valor por defecto
    EXISTING_INSTALL=false
    INSTALL_STATE=absent
    PREV_DOMAIN=""; PREV_USE_SSL=""; PREV_SSL_EMAIL=""; PREV_PANEL_PORT=""; PREV_SFTP_PORT=""
    PREV_PANEL_BIND=""; PREV_SERVER_IP=""; PREV_MASTER_URL=""; PREV_DB_MODE=""; PREV_MYSQL_HOST_PORT=""
    log_success "Instalación anterior eliminada: empieza la instalación nueva"
}

# =============================================================================
# AYUDA Y ARGUMENTOS
# =============================================================================

show_help() {
    cat <<EOF
Aether Panel - Script de Instalación (Docker) v$SCRIPT_VERSION

Uso:
  sudo bash install.sh [OPCIONES]
  curl -fsSL $INSTALL_URL | sudo bash -s -- [OPCIONES]

Nota: Este script requiere permisos de root (usa sudo)

Modos de instalación:
  Manual:   sudo bash install.sh                    (pregunta todo paso a paso)
  Auto:     sudo bash install.sh -y                 (pregunta solo la cuenta admin)
  Sin preguntas: sudo bash install.sh --non-interactive [opciones]
  Simulación:    sudo bash install.sh --dry-run [opciones]  (no cambia nada)

Opciones de Instalación:
  -b, --branch RAMA          Rama git (defecto: $DEFAULT_GIT_BRANCH, o la ya instalada)
  -d, --domain DOMINIO       Dominio para el panel (Nginx en 80/443)
      --no-domain            Quitar el dominio de una instalación anterior
  -s, --ssl                  Habilitar SSL con Let's Encrypt (requiere dominio)
      --no-ssl               Desactivar SSL
  -e, --email EMAIL          Email para Let's Encrypt (y admin por defecto)
  -p, --port PUERTO          Puerto del panel (defecto: $DEFAULT_PANEL_PORT)
      --sftp-port PUERTO     Puerto SFTP (defecto: $DEFAULT_SFTP_PORT)
      --bind IP              Publicar el panel solo en esa IP (ej. 127.0.0.1 tras Nginx)
      --ip IP                IP del servidor para la URL (defecto: autodetectada)
      --master-url URL       URL pública del panel (defecto: según dominio/SSL o IP:puerto)
      --db-mode MODO         Base de datos: local|external (defecto: local)
      --db-host HOST         Host de base de datos (modo external)
      --db-port PUERTO       Puerto de base de datos (defecto: 3306)
      --db-name NOMBRE       Nombre de base de datos (defecto: skypanel)
      --db-user USUARIO      Usuario de base de datos (defecto: skypanel)
      --db-pass PASS         Contraseña de base de datos (defecto: generada)
      --db-root-pass PASS    Contraseña root MySQL (modo local; defecto: generada)
      --rotate-db-passwords  Cambiar las contraseñas de una BD ya creada
      --admin-email EMAIL    Email del administrador inicial
      --admin-pass PASS      Contraseña del administrador (defecto: generada)
      --admin-name NOMBRE    Nombre de usuario admin (defecto: admin)
      --no-admin             No crear cuenta administrador
      --replace-admin        Reemplazar (borrar y recrear) un usuario con ese email/nombre
  -f, --force                Recrear contenedores aunque ya exista (los datos se conservan)
  -y, --yes                  Modo automático (solo datos sensibles); confirma borrados
      --non-interactive      No preguntar nada nunca (automatización)
      --dry-run              Mostrar lo que se haría sin cambiar el sistema
      --skip-deps            Omitir instalación de dependencias
      --skip-docker          Omitir instalación de Docker
      --skip-firewall        Omitir configuración de firewall
      --skip-nginx           Omitir configuración de Nginx

Funciones del panel (opcionales; también configurables luego en Ajustes):
      --turnstile-site-key K     Cloudflare Turnstile en lugar del captcha integrado
      --turnstile-secret-key K
      --cf-dns-token TOKEN       Subdominios automáticos (Cloudflare, permiso Zone DNS Edit)
      --cf-zone-id ID
      --dns-base-domain DOMINIO
      --dns-target DESTINO       Destino de los registros (defecto: IP pública de cada nodo)
      --dns-auto-assign          Asignar subdominio automáticamente a servidores nuevos
      --s3-endpoint URL          Copias de seguridad en S3 (o compatible: MinIO, R2...)
      --s3-region REGIÓN
      --s3-bucket BUCKET
      --s3-access-key KEY
      --s3-secret-key KEY
      --s3-prefix PREFIJO        (defecto del panel: aether-backups)
      --s3-insecure              HTTP sin TLS (MinIO en red local)
      --curseforge-key KEY       Clave API de CurseForge
      --log-retention-days N     Días de logs rotados a conservar (defecto del panel: 14)

Opciones de Desinstalación:
  -u, --uninstall            Desinstalar solo (mantener datos)
      --uninstall-data       Desinstalar + eliminar datos (requiere -y sin terminal)
      --uninstall-all        Desinstalar + eliminar todo (requiere -y sin terminal)

Opciones de Cuenta:
      --create-admin         Crear cuenta administrador (solo)

Opciones de Actualización:
      --update               Actualizar sin borrar datos (copia la base de datos antes y vuelve a la
                             versión anterior si la nueva no arranca)
      --reinstall            Borrar la instalación y volver a instalar (pide confirmación; --yes sin terminal)
      --skip-backup          No hacer copia de seguridad antes de actualizar o reinstalar
      --db-admin-user U      Con BD externa: cuenta que crea la base de datos y el usuario si no existen
      --db-admin-pass P      Su contraseña (mejor por entorno: AETHER_DB_ADMIN_PASSWORD); no se guarda
      --apply-panel-url      Configurar Nginx/SSL para la URL principal de Ajustes (lo usa el servicio aether-domain)

Opciones:
      --debug                Modo debug
  -V, --version              Mostrar la versión
  -h, --help                 Mostrar esta ayuda

Variables de entorno (alternativa a las opciones; mejor para contraseñas, que así
no quedan en el historial ni en la lista de procesos):
  AETHER_BRANCH AETHER_DOMAIN AETHER_SSL AETHER_EMAIL AETHER_PORT AETHER_SFTP_PORT
  AETHER_BIND AETHER_IP AETHER_MASTER_URL AETHER_DB_MODE AETHER_DB_HOST AETHER_DB_PORT
  AETHER_DB_NAME AETHER_DB_USER AETHER_DB_PASSWORD AETHER_DB_ROOT_PASSWORD
  AETHER_ADMIN_NAME AETHER_ADMIN_EMAIL AETHER_ADMIN_PASSWORD
  AETHER_TURNSTILE_SITE_KEY AETHER_TURNSTILE_SECRET_KEY AETHER_CF_DNS_TOKEN
  AETHER_CF_ZONE_ID AETHER_DNS_BASE_DOMAIN AETHER_DNS_TARGET AETHER_DNS_AUTO_ASSIGN
  AETHER_S3_ENDPOINT AETHER_S3_REGION AETHER_S3_BUCKET AETHER_S3_ACCESS_KEY
  AETHER_S3_SECRET_KEY AETHER_S3_PREFIX AETHER_S3_INSECURE AETHER_CURSEFORGE_KEY
  AETHER_LOG_RETENTION_DAYS AETHER_YES AETHER_NONINTERACTIVE AETHER_DRY_RUN
  AETHER_FORCE AETHER_DEBUG AETHER_REPO
  Valores que empiezan por '-': usa --opcion=valor.

Ejemplos:
  # Instalar (wizard manual)
  sudo bash install.sh

  # Instalar automático
  sudo bash install.sh -y

  # Instalar con dominio + SSL sin preguntas
  sudo AETHER_ADMIN_PASSWORD='...' bash install.sh -d panel.ejemplo.com -s \\
       -e admin@ejemplo.com --admin-email admin@ejemplo.com --non-interactive

  # Ver qué haría sin tocar nada
  sudo bash install.sh -d panel.ejemplo.com -s --non-interactive --dry-run

  # Reinstalar forzadamente (recrea contenedores, conserva datos)
  sudo bash install.sh -f -y

  # Crear cuenta administrador en una instalación existente
  sudo bash install.sh --create-admin --admin-email admin@ejemplo.com

  # Actualizar sin borrar datos
  sudo bash install.sh --update

  # Desinstalar manteniendo datos
  sudo bash install.sh --uninstall

  # Desinstalar todo completamente
  sudo bash install.sh --uninstall-all -y
EOF
}

REPLY=""
SHIFT_N=1
PASSWORD_ARGS_USED=false
AUTO_CONFIRMED=$AUTO_MODE   # -y explícito (no por falta de terminal)
if [[ ! -t 0 ]] && ! is_truthy "${AETHER_YES:-}"; then AUTO_CONFIRMED=false; fi
HAS_INSTALL_ARGS=false

# need_value OPCIÓN HAS_EQ VALOR_EQ SIGUIENTE -> REPLY, SHIFT_N
need_value() {
    if [[ "$2" == "true" ]]; then
        REPLY="$3"; SHIFT_N=1
    else
        if [[ -z "${4:-}" || "${4:-}" == -* ]]; then
            log_error "La opción $1 necesita un valor (ejemplo: $1 VALOR o $1=VALOR)"
            exit 1
        fi
        REPLY="$4"; SHIFT_N=2
    fi
    if [[ -z "$REPLY" ]]; then
        log_error "La opción $1 necesita un valor"
        exit 1
    fi
}

parse_args() {
    local opt eqval has_eq
    while (( $# > 0 )); do
        opt="$1"; eqval=""; has_eq=false; SHIFT_N=1
        if [[ "$opt" == --*=* ]]; then
            eqval="${opt#*=}"; opt="${opt%%=*}"; has_eq=true
        fi
        case "$opt" in
            -d|--domain) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DOMAIN="$REPLY"; MASTER_URL_EXPLICIT=true; HAS_INSTALL_ARGS=true ;;
            --no-domain) NO_DOMAIN=true; DOMAIN=""; MASTER_URL_EXPLICIT=true; HAS_INSTALL_ARGS=true ;;
            -s|--ssl) USE_SSL=true; MASTER_URL_EXPLICIT=true; HAS_INSTALL_ARGS=true ;;
            --no-ssl) USE_SSL=false; MASTER_URL_EXPLICIT=true; HAS_INSTALL_ARGS=true ;;
            -e|--email) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; SSL_EMAIL="$REPLY"; HAS_INSTALL_ARGS=true ;;
            -p|--port) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; PANEL_PORT="$REPLY"; MASTER_URL_EXPLICIT=true; HAS_INSTALL_ARGS=true ;;
            --sftp-port) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; SFTP_PORT="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --bind) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; PANEL_BIND="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --ip) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; SERVER_IP="$REPLY"; MASTER_URL_EXPLICIT=true; HAS_INSTALL_ARGS=true ;;
            --master-url) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; MASTER_URL="$REPLY"; MASTER_URL_EXPLICIT=true; HAS_INSTALL_ARGS=true ;;
            --db-mode) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_MODE="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --db-host) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_HOST="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --db-port) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_PORT="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --db-name) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_NAME="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --db-user) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_USER="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --db-pass) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_PASSWORD="$REPLY"; PASSWORD_ARGS_USED=true; HAS_INSTALL_ARGS=true ;;
            --db-root-pass) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_ROOT_PASSWORD="$REPLY"; PASSWORD_ARGS_USED=true; HAS_INSTALL_ARGS=true ;;
            --rotate-db-passwords) ROTATE_DB_PASSWORDS=true; HAS_INSTALL_ARGS=true ;;
            --admin-email) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; ADMIN_EMAIL="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --admin-pass) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; ADMIN_PASSWORD="$REPLY"; PASSWORD_ARGS_USED=true; HAS_INSTALL_ARGS=true ;;
            --admin-name) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; ADMIN_NAME="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --no-admin) CREATE_ADMIN=false; HAS_INSTALL_ARGS=true ;;
            --replace-admin) REPLACE_ADMIN=true; HAS_INSTALL_ARGS=true ;;
            --create-admin) CREATE_ADMIN=true; CREATE_ADMIN_ONLY=true; HAS_INSTALL_ARGS=true ;;
            --turnstile-site-key) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; TURNSTILE_SITE_KEY="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --turnstile-secret-key) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; TURNSTILE_SECRET_KEY="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --cf-dns-token) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; CF_DNS_TOKEN="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --cf-zone-id) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; CF_ZONE_ID="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --dns-base-domain) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DNS_BASE_DOMAIN="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --dns-target) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DNS_TARGET="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --dns-auto-assign) DNS_AUTO_ASSIGN=true; HAS_INSTALL_ARGS=true ;;
            --s3-endpoint) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; S3_ENDPOINT="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --s3-region) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; S3_REGION="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --s3-bucket) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; S3_BUCKET="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --s3-access-key) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; S3_ACCESS_KEY="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --s3-secret-key) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; S3_SECRET_KEY="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --s3-prefix) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; S3_PREFIX="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --s3-insecure) S3_INSECURE=true; HAS_INSTALL_ARGS=true ;;
            --curseforge-key) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; CURSEFORGE_KEY="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --log-retention-days) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; LOG_RETENTION_DAYS="$REPLY"; HAS_INSTALL_ARGS=true ;;
            -f|--force) FORCE=true; HAS_INSTALL_ARGS=true ;;
            -y|--yes) AUTO_MODE=true; AUTO_CONFIRMED=true; HAS_INSTALL_ARGS=true ;;
            --non-interactive) NON_INTERACTIVE=true; HAS_INSTALL_ARGS=true ;;
            --dry-run) DRY_RUN=true ;;
            --skip-deps) SKIP_DEPENDENCIES=true ;;
            --skip-docker) SKIP_DOCKER=true ;;
            --skip-firewall) SKIP_FIREWALL=true ;;
            --skip-nginx) SKIP_NGINX=true ;;
            -u|--uninstall) UNINSTALL_MODE="basic"; HAS_INSTALL_ARGS=true ;;
            --uninstall-data) UNINSTALL_MODE="data"; HAS_INSTALL_ARGS=true ;;
            --uninstall-all) UNINSTALL_MODE="full"; HAS_INSTALL_ARGS=true ;;
            --update) UPDATE_MODE=true; HAS_INSTALL_ARGS=true ;;
            --reinstall) REINSTALL_MODE=true; HAS_INSTALL_ARGS=true ;;
            --skip-backup) SKIP_BACKUP=true ;;
            --db-admin-user) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_ADMIN_USER="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --db-admin-pass) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; DB_ADMIN_PASSWORD="$REPLY"; HAS_INSTALL_ARGS=true ;;
            --apply-panel-url) APPLY_PANEL_URL=true; HAS_INSTALL_ARGS=true; NON_INTERACTIVE=true ;;
            -b|--branch) need_value "$opt" "$has_eq" "$eqval" "${2:-}"; GIT_BRANCH="$REPLY" ;;
            --debug) DEBUG=true ;;
            -V|--version) echo "$SCRIPT_VERSION ($SCRIPT_BUILD)"; exit 0 ;;
            -h|--help) show_help; exit 0 ;;
            *) log_error "Opción desconocida: $1"; echo "Usa --help para ver las opciones." >&2; exit 1 ;;
        esac
        if [[ "$has_eq" == "true" && "$SHIFT_N" == "1" && "$REPLY" != "$eqval" ]]; then
            log_error "La opción $opt no admite valor"
            exit 1
        fi
        shift "$SHIFT_N"
        REPLY=""
    done

    if [[ "$NON_INTERACTIVE" == "true" ]]; then
        AUTO_MODE=true
        CAN_PROMPT=false
    fi
    if [[ "$DEBUG" == "true" ]]; then
        log_debug "Modo debug activado"
    fi
    if [[ "$PASSWORD_ARGS_USED" == "true" ]]; then
        log_warning "Has pasado contraseñas como argumentos: quedan en el historial y en 'ps'."
        log_warning "Mejor usa AETHER_ADMIN_PASSWORD / AETHER_DB_PASSWORD / AETHER_DB_ROOT_PASSWORD."
    fi
}

# =============================================================================
# PUNTO DE ENTRADA PRINCIPAL
# =============================================================================

create_admin_only() {
    require_root
    acquire_lock
    detect_os
    detect_existing_installation
    ensure_docker_running
    load_state
    if [[ "$DRY_RUN" != "true" ]] && ! container_running "$PANEL_CONTAINER"; then
        die "El contenedor $PANEL_CONTAINER no está corriendo"
    fi
    if [[ -z "$ADMIN_EMAIL" ]]; then
        if [[ "$CAN_PROMPT" == "true" ]]; then
            ask_input ADMIN_EMAIL "Email del administrador" "" sensitive
        else
            die "Indica el email con --admin-email (o AETHER_ADMIN_EMAIL)"
        fi
    fi
    if [[ -z "$ADMIN_PASSWORD" && "$CAN_PROMPT" == "true" ]]; then
        ask_password ADMIN_PASSWORD "Contraseña del administrador (mín. 8; vacío = generar una segura)" 8 true
    fi
    validate_email "$ADMIN_EMAIL" || die "Email del administrador inválido: $ADMIN_EMAIL"
    CREATE_ADMIN=true
    create_admin
    if [[ "$ADMIN_CREATED" == "true" ]]; then
        echo ""
        printf '  %s⚠  IMPORTANTE: cambia la contraseña tras el primer inicio de sesión.%s\n' "$YELLOW" "$NC"
        echo ""
        printf '  Usuario: %s%s%s\n' "$GREEN" "$ADMIN_NAME" "$NC"
        printf '  Email:   %s%s%s\n' "$GREEN" "$ADMIN_EMAIL" "$NC"
        if [[ "$ADMIN_PASSWORD_GENERATED" == "true" ]]; then
            echo "  Contraseña: generada; está en $CREDENTIALS_FILE (sudo cat $CREDENTIALS_FILE)"
        fi
    else
        exit 1
    fi
}

main() {
    parse_args "$@"

    # Verificar si es modo desinstalación
    if [[ -n "$UNINSTALL_MODE" ]]; then
        uninstall
        exit 0
    fi

    # Banner inicial
    print_banner

    # Menú principal interactivo (solo si no se pasó ningún argumento)
    if [[ "$HAS_INSTALL_ARGS" == "false" ]] && interactive_manual; then
        require_root
        detect_existing_installation
        show_install_state
        local main_choice def=1
        if [[ "$EXISTING_INSTALL" == "true" ]]; then def=2; fi
        echo ""
        echo "============================================================"
        echo "  Menú Principal"
        echo "============================================================"
        echo ""
        echo "  1) Instalar"
        echo "  2) Actualizar (conserva los datos)"
        echo "  3) Reinstalar desde cero (borra los datos)"
        echo "  4) Desinstalar"
        echo "  5) Crear cuenta administrador"
        echo "  6) Salir"
        echo ""
        while true; do
            prompt_read main_choice "Selecciona [1-6]" "$def"
            if [[ "$main_choice" =~ ^[2-5]$ && "$EXISTING_INSTALL" != "true" ]]; then
                log_warning "No hay ninguna instalación de $PROJECT_NAME: elige 1 para instalar"
                continue
            fi
            break
        done
        case "$main_choice" in
            1) : ;;
            2) UPDATE_MODE=true ;;
            3) REINSTALL_MODE=true ;;
            4) UNINSTALL_MODE=""; uninstall; exit 0 ;;
            5) CREATE_ADMIN=true; CREATE_ADMIN_ONLY=true ;;
            6) echo "¡Hasta luego!"; exit 0 ;;
            *) echo "Opción inválida"; exit 1 ;;
        esac
    fi

    # Modo solo crear cuenta administrador
    if [[ "$CREATE_ADMIN_ONLY" == "true" ]]; then
        create_admin_only
        exit 0
    fi

    # Verificaciones iniciales (no cambian nada)
    require_root
    acquire_lock
    detect_os
    load_state
    detect_existing_installation

    if [[ "$FOREIGN_CONTAINER" == "true" && "$UPDATE_MODE" != "true" && "$APPLY_PANEL_URL" != "true" ]]; then
        die "Ya existe un contenedor llamado '$PANEL_CONTAINER' que no pertenece a $INSTALL_DIR. Renómbralo o elimínalo (es de otro proyecto: el instalador no lo toca)."
    fi

    # Modo actualización (sin borrar datos)
    if [[ "$UPDATE_MODE" == "true" ]]; then
        if [[ "$EXISTING_INSTALL" != "true" ]]; then
            die "No hay ninguna instalación de $PROJECT_NAME que actualizar: ejecuta el instalador sin --update"
        fi
        update_panel
        exit 0
    fi

    # Reinstalación: se borra lo anterior y sigue como instalación nueva
    if [[ "$REINSTALL_MODE" == "true" ]]; then
        reinstall_prepare
    fi

    # El servicio aether-domain: aplicar la URL principal guardada en Ajustes
    if [[ "$APPLY_PANEL_URL" == "true" ]]; then
        apply_panel_url
        exit 0
    fi

    check_resources
    if [[ "$EXISTING_INSTALL" == "true" ]]; then
        existing_installation_menu
        if [[ "$UPDATE_MODE" == "true" ]]; then
            update_panel
            exit 0
        fi
    else
        log_success "No se detectó instalación previa"
    fi

    # Configuración interactiva (manual o auto según modo)
    ask_configuration
    detect_server_ip
    validate_configuration
    resolve_branch

    # Proceso de instalación
    install_dependencies
    ensure_docker_running
    check_ports
    check_network
    sync_repository
    configure_env_file
    configure_compose_override
    check_external_db
    build_images
    rotate_db_passwords
    if [[ "$FORCE" == "true" && "$EXISTING_INSTALL" == "true" ]]; then
        cleanup_existing
    fi
    configure_firewall
    configure_nginx
    compute_master_url
    queue_panel_config
    start_stack
    create_admin
    save_state
    install_domain_service
    record_domain_status
    print_summary
}

# Ejecutar
main "$@"
