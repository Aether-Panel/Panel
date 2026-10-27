#!/usr/bin/env bash

###############################################################################
# Aether Panel - Script de Instalación Automática (Docker)
# Versión: 4.1.0
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

readonly SCRIPT_VERSION="4.1.0"
readonly SCRIPT_BUILD="20261026-hardening"
readonly PROJECT_NAME="Aether Panel"
readonly PROJECT_CODENAME="SkyPanel"
readonly INSTALL_URL="https://install.aetherpanel.es/install.sh"
# El repositorio se puede cambiar (forks, pruebas) con AETHER_REPO
GITHUB_REPO="${AETHER_REPO:-https://github.com/Aether-Panel/Panel.git}"
# Rama por defecto de las instalaciones. Se puede cambiar con --branch o AETHER_BRANCH.
readonly DEFAULT_GIT_BRANCH="dev2.0"

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
        *)
            if contains_word "$OS_LIKE" ubuntu; then
                OS_FAMILY="debian"; DOCKER_DISTRO="ubuntu"; DOCKER_CODENAME="${OS_UBUNTU_CODENAME:-$OS_CODENAME}"
            elif contains_word "$OS_LIKE" debian; then
                OS_FAMILY="debian"; DOCKER_DISTRO="debian"; DOCKER_CODENAME="$OS_CODENAME"
            elif contains_word "$OS_LIKE" rhel || contains_word "$OS_LIKE" centos || contains_word "$OS_LIKE" fedora; then
                OS_FAMILY="rhel"; DOCKER_DISTRO="centos"
            else
                die "Sistema operativo no soportado: ${OS_PRETTY:-$OS_TYPE} (soportados: Ubuntu, Debian, Fedora, RHEL/Rocky/Alma)"
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
            ubuntu) (( major >= 20 )) || log_warning "Ubuntu $OS_VERSION es anterior a 20.04 (no soportado)" ;;
            debian) (( major >= 11 )) || log_warning "Debian $OS_VERSION es anterior a 11 (no soportado)" ;;
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

detect_existing_installation() {
    EXISTING_INSTALL=false
    if [[ -d "$INSTALL_DIR/.git" ]] || { command -v docker >/dev/null 2>&1 && container_exists "$PANEL_CONTAINER"; }; then
        EXISTING_INSTALL=true
    fi
    if [[ -f "$INSTALL_DIR/docker-compose.yml" ]] && grep -qE '^[[:space:]]*- SKYPANEL_PANEL_ENABLE=false' "$INSTALL_DIR/docker-compose.yml"; then
        IS_NODE_ONLY=true
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
    echo "  ¿Qué deseas hacer?"
    echo "  1) Actualizar (mantener datos)"
    echo "  2) Reinstalar / reconfigurar (mantener datos)"
    echo "  3) Desinstalar"
    echo "  4) Cancelar"
    echo ""
    prompt_read choice "Selecciona [1-4]" "1"
    case "$choice" in
        1) log_info "Iniciando actualización..."; UPDATE_MODE=true ;;
        2) log_info "Iniciando reconfiguración..." ;;
        3) UNINSTALL_MODE="basic"; uninstall; exit 0 ;;
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
        local p
        ask_input p "Puerto del panel" "$PANEL_PORT"
        if validate_port "$p"; then PANEL_PORT="$p"; else log_warning "Puerto inválido; se usa $PANEL_PORT"; fi
        ask_input p "Puerto SFTP" "$SFTP_PORT"
        if validate_port "$p"; then SFTP_PORT="$p"; else log_warning "Puerto inválido; se usa $SFTP_PORT"; fi
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

start_stack() {
    print_step "Iniciando $PROJECT_NAME"

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
    # Con dominio se usa Nginx (puertos 80/443)
    if [[ -n "$DOMAIN" && "$SKIP_NGINX" != "true" ]]; then tcp_ports+=(80 443); fi

    local p
    case "$OS_FAMILY" in
        debian)
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
        fedora|rhel)
            if ! command -v firewall-cmd >/dev/null 2>&1; then
                log_warning "firewalld no está instalado; no se configura el firewall"
                return 0
            fi
            if [[ "$HAS_SYSTEMD" == "true" ]] && ! systemctl is-active --quiet firewalld; then
                # Primero las reglas permanentes de SSH, luego se arranca
                for p in "${SSH_PORTS[@]}"; do run firewall-offline-cmd --add-port="$p/tcp" || true; done
                svc enable --now firewalld
            fi
            run firewall-cmd --permanent --add-service=ssh
            for p in "${SSH_PORTS[@]}"; do run firewall-cmd --permanent --add-port="$p/tcp"; done
            for p in "${tcp_ports[@]}"; do run firewall-cmd --permanent --add-port="$p/tcp"; done
            run firewall-cmd --reload
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
    fi
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

# nginx_site_config SERVER_NAME http|ssl
nginx_site_config() {
    local name="$1" mode="$2" acme_root="/var/www/html"
    cat <<EOF
# Aether Panel - generado por install.sh $SCRIPT_VERSION.
# Se regenera al volver a ejecutar el instalador.
map \$http_upgrade \$skypanel_connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
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
    server_name $name;

    ssl_certificate /etc/letsencrypt/live/$name/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$name/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SkyPanelSSL:10m;
    ssl_session_timeout 1d;

EOF
        nginx_proxy_locations
        echo "}"
    else
        echo ""
        nginx_proxy_locations
        echo "}"
    fi
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
        # IP sin dominio (proxy en el puerto 80 como en versiones anteriores)
        install_nginx_site "$(nginx_site_config "$SERVER_IP" http)"
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
    echo "   • Adminer (BD): 127.0.0.1:8081 (solo local; usa un túnel SSH: ssh -L 8081:127.0.0.1:8081 ...)"
    if [[ "$SKIP_FIREWALL" != "true" ]]; then
        echo "   Nota: Docker publica sus puertos por delante de ufw/firewalld."
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
    echo "🐳 Comandos útiles:"
    echo "   • Estado:     cd $INSTALL_DIR && ${DC[*]:-docker compose} ps"
    echo "   • Logs:       cd $INSTALL_DIR && ${DC[*]:-docker compose} logs -f"
    echo "   • Reiniciar:  cd $INSTALL_DIR && ${DC[*]:-docker compose} restart"
    echo "   • Actualizar: desde el panel (usa tools/panelUpdate/panelUpdate.sh) o"
    echo "                 curl -fsSL $INSTALL_URL | sudo bash -s -- --update"
    echo "   • Desinstalar (conserva datos): curl -fsSL $INSTALL_URL | sudo bash -s -- --uninstall"
    echo ""
    echo "   Opciones de esta instalación: $STATE_FILE"
    echo "   Datos (BD, servidores, backups, extensiones, logs): $INSTALL_DIR/storage"
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

    sync_repository
    configure_env_file
    configure_compose_override
    build_images
    rotate_db_passwords
    compute_master_url
    queue_panel_config
    start_stack
    create_admin
    save_state
    print_summary "Actualización completada"
    log_info "Los datos y la configuración se mantuvieron intactos"
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

    if ! command -v docker >/dev/null 2>&1; then
        log_warning "Docker no está instalado"
    else
        detect_docker
    fi
    if (( ${#DC[@]} == 0 )); then DC=(docker compose); fi
    load_state

    local has_container=false
    if command -v docker >/dev/null 2>&1 && container_exists "$PANEL_CONTAINER"; then has_container=true; fi

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
        case "$UNINSTALL_MODE" in data) def=2 ;; full) def=3 ;; esac
        echo "¿Estás seguro de que deseas desinstalar $PROJECT_NAME?"
        echo ""
        echo "Opciones:"
        echo "  1) Desinstalar solo (mantener datos y configuración)"
        echo "  2) Desinstalar + Eliminar datos (mantener configuración)"
        echo "  3) Desinstalar + Eliminar TODO (completo)"
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
            local confirm
            prompt_read confirm "Esto BORRA la base de datos y los servidores. Escribe BORRAR para confirmar" ""
            [[ "$confirm" == "BORRAR" ]] || { echo "Cancelado."; exit 0; }
        fi
    elif [[ "$UNINSTALL_MODE" != "basic" && "$AUTO_CONFIRMED" != "true" ]]; then
        die "Borrar datos sin terminal requiere confirmación explícita: añade --yes"
    fi

    print_step "Desinstalando ($UNINSTALL_MODE)"

    # Contenedores del panel (los datos están en $INSTALL_DIR/storage y NO se tocan aquí)
    if [[ -f "$INSTALL_DIR/docker-compose.yml" ]]; then
        local -a down_args=(down --remove-orphans)
        if [[ "$UNINSTALL_MODE" == "full" ]]; then down_args+=(--rmi local --volumes); fi
        dc "${down_args[@]}" || log_warning "docker compose down terminó con avisos"
    fi
    local c
    for c in "$PANEL_CONTAINER" "$MYSQL_CONTAINER" skypanel-adminer; do
        if command -v docker >/dev/null 2>&1 && container_exists "$c"; then
            run docker rm -f "$c" || true
        fi
    done

    # Nginx
    local conf
    for conf in /etc/nginx/sites-enabled/skypanel /etc/nginx/sites-available/skypanel /etc/nginx/conf.d/skypanel.conf; do
        if [[ -e "$conf" || -L "$conf" ]]; then
            run rm -f "$conf"
            NGINX_CONFIGURED=true
        fi
    done
    if [[ "$NGINX_CONFIGURED" == "true" ]] && command -v nginx >/dev/null 2>&1; then
        log_info "Configuración de Nginx eliminada"
        if nginx -t >/dev/null 2>&1; then svc reload nginx || true; fi
    fi

    if [[ "$UNINSTALL_MODE" == "data" || "$UNINSTALL_MODE" == "full" ]]; then
        # Servidores de juego creados por el panel (en su red propia)
        if command -v docker >/dev/null 2>&1; then
            local ids
            ids=$(docker ps -aq --filter "network=$SERVERS_NETWORK" 2>/dev/null || true)
            if [[ -n "$ids" ]]; then
                log_info "Eliminando contenedores de servidores de juego..."
                # shellcheck disable=SC2086 # lista de IDs separada por espacios
                run docker rm -f $ids || true
            fi
            if docker network inspect "$SERVERS_NETWORK" >/dev/null 2>&1; then
                run docker network rm "$SERVERS_NETWORK" || true
            fi
        fi
        log_info "Eliminando datos ($INSTALL_DIR/storage)..."
        run rm -rf "$INSTALL_DIR/storage"
        if [[ -d "$DATA_DIR" ]]; then run rm -rf "$DATA_DIR"; fi
    fi

    if [[ "$UNINSTALL_MODE" == "full" ]]; then
        log_info "Eliminando instalación y configuración..."
        local cert_domain="${PREV_DOMAIN:-$DOMAIN}"
        run rm -rf "$INSTALL_DIR" "$CONFIG_DIR"
        local d
        for d in "$LOG_DIR" "$WEB_DIR"; do
            if [[ -d "$d" ]]; then run rm -rf "$d"; fi
        done
        # Solo el certificado del dominio del panel (nunca los de otros sitios)
        if [[ -n "$cert_domain" && -d "/etc/letsencrypt/live/$cert_domain" ]] && command -v certbot >/dev/null 2>&1; then
            log_info "Eliminando el certificado SSL de $cert_domain..."
            run certbot delete --cert-name "$cert_domain" --non-interactive || true
        fi
    fi

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
            echo "   • Contenedores, imagen y servidores de juego"
            echo "   • Directorio de instalación y datos ($INSTALL_DIR)"
            echo "   • Opciones y credenciales ($CONFIG_DIR)"
            echo "   • Configuración de Nginx y certificado del panel"
            echo ""
            echo "   Docker, Nginx y las reglas del firewall siguen instalados."
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
      --update               Actualizar sin borrar datos

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
        local main_choice
        echo "============================================================"
        echo "  Menú Principal"
        echo "============================================================"
        echo ""
        echo "  1) Instalar $PROJECT_NAME (Docker)"
        echo "  2) Actualizar $PROJECT_NAME (mantener datos)"
        echo "  3) Crear cuenta administrador"
        echo "  4) Desinstalar $PROJECT_NAME"
        echo "  5) Salir"
        echo ""
        prompt_read main_choice "Selecciona [1-5]" "1"
        case "$main_choice" in
            1) : ;;
            2) UPDATE_MODE=true ;;
            3) CREATE_ADMIN=true; CREATE_ADMIN_ONLY=true ;;
            4) UNINSTALL_MODE="basic"; uninstall; exit 0 ;;
            5) echo "¡Hasta luego!"; exit 0 ;;
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

    # Modo actualización (sin borrar datos)
    if [[ "$UPDATE_MODE" == "true" ]]; then
        update_panel
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
    print_summary
}

# Ejecutar
main "$@"
