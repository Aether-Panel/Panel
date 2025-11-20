#!/bin/bash

set -e

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Variables
PANEL_NAME="SkyPanel"
PANEL_USER="SkyPanel"
PANEL_GROUP="SkyPanel"
INSTALL_DIR="/opt/${PANEL_NAME}"
DATA_DIR="/var/lib/${PANEL_NAME}"
CONFIG_DIR="/etc/${PANEL_NAME}"
LOG_DIR="/var/log/${PANEL_NAME}"
WEB_ROOT="/var/www/${PANEL_NAME}"
SERVICE_NAME="${PANEL_NAME,,}"
REPO_URL="${SkyPanel_REPO_URL:-https://github.com/SkyPanel/SkyPanel.git}"
BRANCH="${SkyPanel_BRANCH:-master}"
PORT="${SkyPanel_PORT:-8080}"

# Detectar IP de la VPS
detect_ip() {
    local ip=""
    
    # Intentar obtener IP pública primero
    ip=$(curl -s -4 --connect-timeout 3 https://checkip.amazonaws.com 2>/dev/null || echo "")
    
    # Si falla, intentar obtener IP local
    if [ -z "$ip" ]; then
        ip=$(hostname -I | awk '{print $1}' 2>/dev/null || echo "")
    fi
    
    # Si aún no hay IP, usar localhost
    if [ -z "$ip" ]; then
        ip="127.0.0.1"
    fi
    
    echo "$ip"
}

print_header() {
    echo -e "${BLUE}"
    echo "╔══════════════════════════════════════════════════════════╗"
    echo "║          Instalador Automático de ${PANEL_NAME}           ║"
    echo "╚══════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_step() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1" >&2
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

print_info() {
    echo -e "${BLUE}[i]${NC} $1"
}

# Verificar si se ejecuta como root
check_root() {
    if [ "$EUID" -ne 0 ]; then 
        print_error "Por favor ejecuta este script como root: sudo bash install.sh"
        exit 1
    fi
}

# Detectar distribución
detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO=$ID
        VERSION=$VERSION_ID
    else
        print_error "No se pudo detectar la distribución"
        exit 1
    fi
    
    print_info "Distribución detectada: ${DISTRO} ${VERSION}"
}

# Instalar dependencias
install_dependencies() {
    print_info "Instalando dependencias del sistema..."
    
    if [ "$DISTRO" = "ubuntu" ] || [ "$DISTRO" = "debian" ]; then
        apt-get update
        apt-get install -y curl wget git build-essential libsqlite3-dev
        
    elif [ "$DISTRO" = "centos" ] || [ "$DISTRO" = "rhel" ] || [ "$DISTRO" = "fedora" ]; then
        if command -v dnf &> /dev/null; then
            dnf install -y curl wget git gcc gcc-c++ make sqlite-devel
        else
            yum install -y curl wget git gcc gcc-c++ make sqlite-devel
        fi
    else
        print_error "Distribución no soportada: ${DISTRO}"
        exit 1
    fi
    
    print_step "Dependencias del sistema instaladas"
}

# Instalar Go
install_go() {
    if command -v go &> /dev/null; then
        INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        REQUIRED_VERSION="1.21"
        
        # Verificar si la versión instalada cumple con el requisito mínimo
        if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$INSTALLED_VERSION" | sort -V | head -n1)" = "$REQUIRED_VERSION" ]; then
            print_step "Go ya está instalado (versión ${INSTALLED_VERSION})"
            export PATH=$PATH:/usr/local/go/bin
            return
        else
            print_warning "Go ${INSTALLED_VERSION} es muy antigua, se instalará una versión más reciente"
        fi
    fi
    
    print_info "Instalando Go..."
    
    # Detectar última versión de Go disponible
    GO_VERSION=$(curl -s https://go.dev/VERSION?m=text | head -1)
    if [ -z "$GO_VERSION" ]; then
        # Fallback a versión específica si no se puede detectar
        GO_VERSION="go1.23.3"
    fi
    
    cd /tmp
    wget -q "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz"
    tar -C /usr/local -xzf "${GO_VERSION}.linux-amd64.tar.gz"
    rm "${GO_VERSION}.linux-amd64.tar.gz"
    
    # Agregar Go al PATH
    if ! grep -q "/usr/local/go/bin" /etc/profile; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    fi
    # Asegurar que Go esté en PATH para esta sesión
    export PATH=$PATH:/usr/local/go/bin
    
    print_step "Go instalado (${GO_VERSION})"
}

# Instalar Node.js y Yarn
install_node() {
    if command -v node &> /dev/null; then
        NODE_VERSION=$(node -v | sed 's/v//' | cut -d. -f1)
        if [ "$NODE_VERSION" -ge 22 ]; then
            print_step "Node.js ya está instalado (versión $(node -v))"
            
            if ! command -v yarn &> /dev/null; then
                print_info "Instalando Yarn..."
                npm install -g yarn
            else
                print_step "Yarn ya está instalado"
            fi
            return
        fi
    fi
    
    print_info "Instalando Node.js 22..."
    
    if [ "$DISTRO" = "ubuntu" ] || [ "$DISTRO" = "debian" ]; then
        curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
        apt-get install -y nodejs
    elif [ "$DISTRO" = "centos" ] || [ "$DISTRO" = "rhel" ] || [ "$DISTRO" = "fedora" ]; then
        if command -v dnf &> /dev/null; then
            dnf install -y nodejs npm
        else
            curl -fsSL https://rpm.nodesource.com/setup_22.x | bash -
            yum install -y nodejs
        fi
    fi
    
    # Instalar Yarn
    print_info "Instalando Yarn..."
    npm install -g yarn
    
    print_step "Node.js 22 y Yarn instalados"
}

# Crear usuario y grupos
create_user() {
    if id "$PANEL_USER" &>/dev/null; then
        print_step "Usuario ${PANEL_USER} ya existe"
    else
        print_info "Creando usuario ${PANEL_USER}..."
        useradd -r -s /bin/false -d "$DATA_DIR" -c "${PANEL_NAME} daemon" "$PANEL_USER"
        print_step "Usuario ${PANEL_USER} creado"
    fi
}

# Clonar y compilar
build_panel() {
    print_info "Descargando código fuente..."
    
    if [ -d "$INSTALL_DIR" ]; then
        print_warning "El directorio ${INSTALL_DIR} ya existe. ¿Eliminarlo? (s/N)"
        read -r response
        if [[ "$response" =~ ^([sS][iI][mM]|[sS])$ ]]; then
            rm -rf "$INSTALL_DIR"
        else
            print_info "Usando instalación existente..."
            return
        fi
    fi
    
    # Clonar repositorio
    cd /tmp
    if [ -d "SkyPanel-temp" ]; then
        rm -rf SkyPanel-temp
    fi
    
    git clone --branch "$BRANCH" --depth 1 "$REPO_URL" SkyPanel-temp
    mv SkyPanel-temp "$INSTALL_DIR"
    
    cd "$INSTALL_DIR"
    
    # Compilar frontend
    print_info "Compilando frontend..."
    cd client/frontend
    yarn install
    yarn build
    cd "$INSTALL_DIR"
    
    # Compilar backend
    print_info "Compilando backend..."
    export PATH=$PATH:/usr/local/go/bin
    export CGO_ENABLED=1
    export CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
    
    # Instalar swag para documentación
    export PATH=$PATH:/usr/local/go/bin
    if ! command -v swag &> /dev/null; then
        print_info "Instalando swag..."
        export GOPATH=$HOME/go
        export PATH=$PATH:$GOPATH/bin
        go install github.com/swaggo/swag/cmd/swag@latest
    fi
    
    # Generar documentación Swagger
    export PATH=$PATH:/usr/local/go/bin
    if command -v swag &> /dev/null; then
        swag init --md . -o web/swagger -g web/loader.go 2>/dev/null || true
    else
        ~/go/bin/swag init --md . -o web/swagger -g web/loader.go 2>/dev/null || true
    fi
    
    # Descargar dependencias de Go
    go mod download
    go mod verify
    
    # Compilar
    go build -tags "" -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3.Hash=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')' -X 'github.com/SkyPanel/SkyPanel/v3.Version=$(git describe --tags 2>/dev/null || echo 'dev')'" -o "${PANEL_NAME,,}" ./cmd
    
    print_step "Panel compilado exitosamente"
}

# Crear estructura de directorios
create_directories() {
    print_info "Creando estructura de directorios..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR/servers"
    mkdir -p "$DATA_DIR/binaries"
    mkdir -p "$DATA_DIR/cache"
    mkdir -p "$DATA_DIR/backups"
    mkdir -p "$LOG_DIR"
    mkdir -p "$WEB_ROOT"
    
    # Copiar archivos web compilados
    if [ -d "$INSTALL_DIR/client/frontend/dist" ]; then
        cp -r "$INSTALL_DIR/client/frontend/dist"/* "$WEB_ROOT/"
    fi
    
    # Mover binario
    mv "$INSTALL_DIR/${PANEL_NAME,,}" "/usr/local/bin/${PANEL_NAME,,}"
    chmod +x "/usr/local/bin/${PANEL_NAME,,}"
    
    # Establecer permisos
    chown -R "$PANEL_USER:$PANEL_GROUP" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR" "$WEB_ROOT"
    chmod o-rx "$CONFIG_DIR" "$DATA_DIR"
    
    print_step "Estructura de directorios creada"
}

# Generar configuración
generate_config() {
    local ip=$(detect_ip)
    local master_url="http://${ip}:${PORT}"
    
    print_info "Generando configuración..."
    print_info "IP detectada: ${ip}"
    print_info "Puerto: ${PORT}"
    print_info "URL Master: ${master_url}"
    
    # Generar token aleatorio para el panel
    local token=$(openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64)
    local session_key=$(openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64)
    
    cat > "$CONFIG_DIR/config.json" <<EOF
{
  "logs": "${LOG_DIR}",
  "web": {
    "host": "0.0.0.0:${PORT}"
  },
  "panel": {
    "enable": true,
    "database": {
      "dialect": "sqlite3",
      "url": "file:${DATA_DIR}/database.db?cache=shared"
    },
    "web": {
      "files": "${WEB_ROOT}"
    },
    "settings": {
      "companyName": "${PANEL_NAME}",
      "defaultTheme": "Default",
      "themeSettings": "{}",
      "masterUrl": "${master_url}"
    },
    "token": "${token}",
    "sessionKey": "${session_key}",
    "registrationEnabled": true
  },
  "daemon": {
    "enable": true,
    "data": {
      "root": "${DATA_DIR}",
      "servers": "${DATA_DIR}/servers",
      "binaries": "${DATA_DIR}/binaries",
      "cache": "${DATA_DIR}/cache",
      "backups": {
        "folder": "${DATA_DIR}/backups"
      }
    },
    "sftp": {
      "host": "0.0.0.0:5657"
    },
    "auth": {
      "url": "${master_url}"
    },
    "console": {
      "buffer": 50,
      "forward": false
    },
    "data": {
      "crashLimit": 3
    }
  },
  "security": {
    "forceOpenat": false,
    "trustedProxies": [],
    "disableUnshare": false
  }
}
EOF
    
    chown "$PANEL_USER:$PANEL_GROUP" "$CONFIG_DIR/config.json"
    chmod 600 "$CONFIG_DIR/config.json"
    
    print_step "Configuración generada"
}

# Configurar systemd
setup_systemd() {
    print_info "Configurando servicio systemd..."
    
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=${PANEL_NAME} Game Server Management Panel
After=network.target

[Service]
Type=simple
User=${PANEL_USER}
Group=${PANEL_GROUP}
WorkingDirectory=${DATA_DIR}
ExecStart=/usr/local/bin/${PANEL_NAME,,} run --config=${CONFIG_DIR}/config.json
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${LOG_DIR} ${CONFIG_DIR}
ReadOnlyPaths=${WEB_ROOT}

[Install]
WantedBy=multi-user.target
EOF
    
    # Actualizar base de datos
    print_info "Actualizando base de datos..."
    sudo -u "$PANEL_USER" /usr/local/bin/${PANEL_NAME,,} dbmigrate --config="${CONFIG_DIR}/config.json" || true
    
    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}.service"
    
    print_step "Servicio systemd configurado"
}

# Iniciar servicio
start_service() {
    print_info "Iniciando servicio..."
    
    systemctl start "${SERVICE_NAME}.service"
    
    sleep 3
    
    if systemctl is-active --quiet "${SERVICE_NAME}.service"; then
        print_step "Servicio iniciado exitosamente"
        print_info "Estado del servicio:"
        systemctl status "${SERVICE_NAME}.service" --no-pager -l
    else
        print_error "Error al iniciar el servicio"
        print_info "Revisa los logs con: journalctl -u ${SERVICE_NAME} -n 50"
        exit 1
    fi
}

# Mostrar información final
show_final_info() {
    local ip=$(detect_ip)
    
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║          ${PANEL_NAME} instalado exitosamente              ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BLUE}Información de acceso:${NC}"
    echo -e "  URL: ${GREEN}http://${ip}:${PORT}${NC}"
    echo -e "  Configuración: ${CONFIG_DIR}/config.json"
    echo -e "  Datos: ${DATA_DIR}"
    echo -e "  Logs: ${LOG_DIR}"
    echo ""
    echo -e "${BLUE}Comandos útiles:${NC}"
    echo -e "  Iniciar: ${GREEN}systemctl start ${SERVICE_NAME}${NC}"
    echo -e "  Detener: ${GREEN}systemctl stop ${SERVICE_NAME}${NC}"
    echo -e "  Reiniciar: ${GREEN}systemctl restart ${SERVICE_NAME}${NC}"
    echo -e "  Estado: ${GREEN}systemctl status ${SERVICE_NAME}${NC}"
    echo -e "  Logs: ${GREEN}journalctl -u ${SERVICE_NAME} -f${NC}"
    echo ""
    echo -e "${YELLOW}Importante:${NC}"
    echo -e "  1. Accede al panel y crea el primer usuario administrador"
    echo -e "  2. Configura el firewall si es necesario:"
    echo -e "     ${GREEN}ufw allow ${PORT}/tcp${NC}"
    echo -e "     ${GREEN}ufw allow 5657/tcp${NC}  # SFTP"
    echo ""
}

# Función principal
main() {
    print_header
    check_root
    detect_distro
    
    print_info "Iniciando instalación de ${PANEL_NAME}..."
    echo ""
    
    install_dependencies
    install_go
    install_node
    create_user
    build_panel
    create_directories
    generate_config
    setup_systemd
    start_service
    show_final_info
}

# Ejecutar instalación
main

