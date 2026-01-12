#!/bin/bash

###############################################################################
# SkyPanel - Instalador Automático Profesional
# 
# Este script instala y configura SkyPanel de forma completamente automática:
# - Instala todas las dependencias necesarias
# - Clona el repositorio desde GitHub/GitLab
# - Compila frontend y backend
# - Configura nginx con SSL (Let's Encrypt)
# - Configura dos dominios: panel y status/gatus
# - Crea servicio systemd
# - Configura firewall
#
# Uso:
#   bash <(curl -s https://install.midominio.com/install.sh)
#
# Autor: SkyPanel Team
# Licencia: MIT
###############################################################################

set -euo pipefail  # Salir si hay errores, variables no definidas o pipes fallan

###############################################################################
# CONFIGURACIÓN Y VARIABLES
###############################################################################

# Versión del instalador
INSTALLER_VERSION="1.0.0"

# Repositorio por defecto (configurable con variable de entorno)
SKYPANEL_REPO_URL="${SKYPANEL_REPO_URL:-https://github.com/SkyPanel/SkyPanel.git}"
SKYPANEL_BRANCH="${SKYPANEL_BRANCH:-main}"

# Usuario y grupo del sistema
SKYPANEL_USER="skypanel"
SKYPANEL_GROUP="skypanel"
SKYPANEL_HOME="/opt/skypanel"
SKYPANEL_CONFIG="/etc/skypanel"
SKYPANEL_DATA="/var/lib/skypanel"
SKYPANEL_LOG="/var/log/skypanel"
SKYPANEL_SERVICE="/etc/systemd/system/skypanel.service"

# Puertos
PANEL_PORT="8080"
GATUS_PORT="8081"
SFTP_PORT="5657"

# Variables de dominio (se solicitarán al usuario)
PANEL_DOMAIN=""
STATUS_DOMAIN=""
PANEL_EMAIL=""

# Colores para output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly BOLD='\033[1m'
readonly NC='\033[0m' # No Color

# Directorio temporal para clonar el repositorio
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

###############################################################################
# FUNCIONES DE UTILIDAD
###############################################################################

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

print_header() {
    echo ""
    echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}${BOLD}  $1${NC}"
    echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════════════${NC}"
    echo ""
}

# Verificar que se ejecuta como root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "Este script debe ejecutarse como root o con sudo"
        exit 1
    fi
}

# Detectar distribución del sistema
detect_distro() {
    if [[ -f /etc/os-release ]]; then
        source /etc/os-release
        DISTRO=$ID
        VERSION=$VERSION_ID
    elif [[ -f /etc/debian_version ]]; then
        DISTRO="debian"
        VERSION=$(cat /etc/debian_version)
    elif [[ -f /etc/redhat-release ]]; then
        DISTRO="rhel"
        VERSION=$(cat /etc/redhat-release | sed 's/.*release \([0-9.]*\).*/\1/')
    else
        print_error "No se pudo detectar la distribución del sistema"
        exit 1
    fi
    
    print_success "Distribución detectada: ${BOLD}$DISTRO $VERSION${NC}"
    
    # Verificar que sea una versión soportada
    case $DISTRO in
        ubuntu)
            if [[ ! "$VERSION" =~ ^(20\.04|22\.04|24\.04)$ ]]; then
                print_warning "Ubuntu $VERSION no está oficialmente soportado, pero se intentará continuar"
            fi
            ;;
        debian)
            if [[ ! "$VERSION" =~ ^(10|11|12)$ ]]; then
                print_warning "Debian $VERSION no está oficialmente soportado, pero se intentará continuar"
            fi
            ;;
        fedora|rhel|centos)
            print_info "Distribución RHEL/Fedora detectada"
            ;;
        *)
            print_warning "Distribución $DISTRO puede no estar completamente soportada"
            ;;
    esac
}

###############################################################################
# INSTALACIÓN DE DEPENDENCIAS
###############################################################################

install_dependencies() {
    print_header "Instalando Dependencias del Sistema"
    
    case $DISTRO in
        ubuntu|debian)
            export DEBIAN_FRONTEND=noninteractive
            print_info "Actualizando lista de paquetes..."
            apt-get update -qq
            
            print_info "Instalando paquetes base..."
            apt-get install -y \
                curl \
                wget \
                git \
                build-essential \
                sqlite3 \
                libsqlite3-dev \
                nginx \
                certbot \
                python3-certbot-nginx \
                ufw \
                ca-certificates \
                gnupg \
                lsb-release \
                openssl \
                > /dev/null 2>&1
            ;;
        fedora|rhel|centos)
            if [[ "$DISTRO" == "centos" ]] || [[ "$DISTRO" == "rhel" ]]; then
                print_info "Instalando EPEL repository..."
                dnf install -y epel-release > /dev/null 2>&1
            fi
            
            print_info "Instalando paquetes base..."
            dnf install -y \
                curl \
                wget \
                git \
                gcc \
                gcc-c++ \
                make \
                sqlite \
                sqlite-devel \
                nginx \
                certbot \
                python3-certbot-nginx \
                firewalld \
                ca-certificates \
                openssl \
                > /dev/null 2>&1
            ;;
        *)
            print_error "Distribución no soportada: $DISTRO"
            exit 1
            ;;
    esac
    
    print_success "Dependencias del sistema instaladas"
}

# Instalar Go
install_go() {
    print_header "Instalando Go"
    
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        print_info "Go ya está instalado: $GO_VERSION"
        
        # Verificar versión mínima (1.21+)
        MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
        MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
        if [[ "$MAJOR" -lt 1 ]] || ([[ "$MAJOR" -eq 1 ]] && [[ "$MINOR" -lt 21 ]]); then
            print_warning "Go $GO_VERSION es muy antiguo, se instalará una versión más reciente"
        else
            print_success "Go $GO_VERSION es compatible"
            export PATH=$PATH:/usr/local/go/bin
            return 0
        fi
    fi
    
    print_info "Instalando Go 1.23.4..."
    
    GO_VERSION="1.23.4"
    GO_ARCH="amd64"
    
    if [[ "$(uname -m)" == "aarch64" ]] || [[ "$(uname -m)" == "arm64" ]]; then
        GO_ARCH="arm64"
    fi
    
    cd /tmp
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -O go.tar.gz
    tar -C /usr/local -xzf go.tar.gz > /dev/null 2>&1
    rm -f go.tar.gz
    
    # Agregar Go al PATH
    if ! grep -q "/usr/local/go/bin" /etc/profile; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    fi
    export PATH=$PATH:/usr/local/go/bin
    
    print_success "Go ${GO_VERSION} instalado correctamente"
}

# Instalar Node.js y Yarn
install_nodejs() {
    print_header "Instalando Node.js y Yarn"
    
    if command -v node &> /dev/null; then
        NODE_VERSION=$(node -v | sed 's/v//')
        print_info "Node.js ya está instalado: $NODE_VERSION"
        
        # Verificar versión mínima (18+)
        MAJOR=$(echo "$NODE_VERSION" | cut -d. -f1)
        if [[ "$MAJOR" -ge 18 ]]; then
            if command -v yarn &> /dev/null; then
                print_success "Node.js $NODE_VERSION y Yarn ya están instalados"
                return 0
            fi
        else
            print_warning "Node.js $NODE_VERSION es muy antiguo, se instalará una versión más reciente"
        fi
    fi
    
    print_info "Instalando Node.js 22 LTS..."
    
    case $DISTRO in
        ubuntu|debian)
            curl -fsSL https://deb.nodesource.com/setup_22.x | bash - > /dev/null 2>&1
            apt-get install -y nodejs > /dev/null 2>&1
            ;;
        fedora|rhel|centos)
            curl -fsSL https://rpm.nodesource.com/setup_22.x | bash - > /dev/null 2>&1
            dnf install -y nodejs > /dev/null 2>&1
            ;;
    esac
    
    # Instalar Yarn
    print_info "Instalando Yarn..."
    npm install -g yarn > /dev/null 2>&1
    
    print_success "Node.js $(node -v) y Yarn $(yarn -v) instalados"
}

###############################################################################
# CONFIGURACIÓN DEL SISTEMA
###############################################################################

# Crear usuario y directorios
create_user_and_directories() {
    print_header "Creando Usuario y Directorios"
    
    # Crear grupo
    if ! getent group "$SKYPANEL_GROUP" > /dev/null 2>&1; then
        groupadd -r "$SKYPANEL_GROUP"
        print_success "Grupo $SKYPANEL_GROUP creado"
    else
        print_info "Grupo $SKYPANEL_GROUP ya existe"
    fi
    
    # Crear usuario
    if ! id "$SKYPANEL_USER" &>/dev/null; then
        useradd -r -g "$SKYPANEL_GROUP" -d "$SKYPANEL_HOME" -s /bin/false "$SKYPANEL_USER"
        print_success "Usuario $SKYPANEL_USER creado"
    else
        print_info "Usuario $SKYPANEL_USER ya existe"
    fi
    
    # Crear directorios
    print_info "Creando estructura de directorios..."
    mkdir -p "$SKYPANEL_HOME"
    mkdir -p "$SKYPANEL_CONFIG"
    mkdir -p "$SKYPANEL_DATA"
    mkdir -p "$SKYPANEL_LOG"
    mkdir -p "$SKYPANEL_DATA/servers"
    mkdir -p "$SKYPANEL_DATA/binaries"
    mkdir -p "$SKYPANEL_DATA/cache"
    mkdir -p "$SKYPANEL_DATA/gatus"
    mkdir -p "$SKYPANEL_DATA/web"
    
    # Establecer permisos
    chown -R "$SKYPANEL_USER:$SKYPANEL_GROUP" "$SKYPANEL_HOME"
    chown -R "$SKYPANEL_USER:$SKYPANEL_GROUP" "$SKYPANEL_CONFIG"
    chown -R "$SKYPANEL_USER:$SKYPANEL_GROUP" "$SKYPANEL_DATA"
    chown -R "$SKYPANEL_USER:$SKYPANEL_GROUP" "$SKYPANEL_LOG"
    
    print_success "Usuario y directorios creados"
}

# Clonar y compilar el panel
clone_and_build() {
    print_header "Clonando y Compilando SkyPanel"
    
    print_info "Clonando repositorio desde: $SKYPANEL_REPO_URL"
    print_info "Rama: $SKYPANEL_BRANCH"
    
    cd "$TEMP_DIR"
    
    if ! git clone -b "$SKYPANEL_BRANCH" "$SKYPANEL_REPO_URL" skypanel-source > /dev/null 2>&1; then
        print_error "No se pudo clonar el repositorio"
        print_error "Verifica que la URL y la rama sean correctas"
        exit 1
    fi
    
    cd skypanel-source
    
    print_success "Repositorio clonado"
    
    # Compilar frontend
    print_info "Compilando frontend (esto puede tardar varios minutos)..."
    cd client/frontend
    
    if ! yarn install --frozen-lockfile > /dev/null 2>&1; then
        print_error "Error al instalar dependencias del frontend"
        exit 1
    fi
    
    if ! yarn build > /dev/null 2>&1; then
        print_error "Error al compilar el frontend"
        exit 1
    fi
    
    print_success "Frontend compilado"
    
    # Volver a la raíz y compilar backend
    cd "$TEMP_DIR/skypanel-source"
    
    print_info "Compilando backend (esto puede tardar varios minutos)..."
    export PATH=$PATH:/usr/local/go/bin
    
    if ! go mod download > /dev/null 2>&1; then
        print_error "Error al descargar dependencias de Go"
        exit 1
    fi
    
    if ! go build -o skypanel ./cmd > /dev/null 2>&1; then
        print_error "Error al compilar el backend"
        exit 1
    fi
    
    print_success "Backend compilado"
}

# Instalar binarios y archivos
install_files() {
    print_header "Instalando Archivos del Panel"
    
    cd "$TEMP_DIR/skypanel-source"
    
    # Copiar binario
    print_info "Instalando binario..."
    cp skypanel /usr/local/bin/skypanel
    chmod +x /usr/local/bin/skypanel
    chown "$SKYPANEL_USER:$SKYPANEL_GROUP" /usr/local/bin/skypanel
    
    # Copiar frontend compilado
    print_info "Instalando archivos del frontend..."
    if [[ -d "client/frontend/dist" ]]; then
        cp -r client/frontend/dist/* "$SKYPANEL_DATA/web/"
        chown -R "$SKYPANEL_USER:$SKYPANEL_GROUP" "$SKYPANEL_DATA/web"
        print_success "Frontend instalado"
    else
        print_error "No se encontró el directorio dist del frontend"
        exit 1
    fi
    
    print_success "Archivos instalados correctamente"
}

# Generar configuración
generate_config() {
    print_header "Generando Configuración"
    
    # Detectar IP pública
    print_info "Detectando IP pública..."
    PUBLIC_IP=$(curl -s --max-time 5 https://checkip.amazonaws.com 2>/dev/null || echo "")
    if [[ -z "$PUBLIC_IP" ]]; then
        PUBLIC_IP=$(hostname -I | awk '{print $1}' || echo "127.0.0.1")
    fi
    if [[ -z "$PUBLIC_IP" ]]; then
        PUBLIC_IP="127.0.0.1"
    fi
    
    print_info "IP detectada: $PUBLIC_IP"
    
    # Generar tokens aleatorios
    TOKEN=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
    SESSION_KEY=$(openssl rand -hex 32)
    
    # Crear config.json
    print_info "Creando archivo de configuración..."
    cat > "$SKYPANEL_CONFIG/config.json" <<EOF
{
  "web": {
    "host": "127.0.0.1:${PANEL_PORT}"
  },
  "panel": {
    "email": {
      "provider": "none"
    },
    "gatus": {
      "enable": true,
      "port": ${GATUS_PORT}
    },
    "registrationenabled": false,
    "sessionkey": "${SESSION_KEY}",
    "settings": {
      "companyname": "SkyPanel",
      "defaulttheme": "SkyPanel",
      "masterurl": "https://${PANEL_DOMAIN}",
      "themesettings": "{\"primary\":\"#3b82f6\",\"background\":\"#0f172a\",\"foreground\":\"#e2e8f0\"}"
    },
    "token": "${TOKEN}"
  },
  "daemon": {
    "data": {
      "root": "${SKYPANEL_DATA}"
    },
    "sftp": {
      "host": "0.0.0.0:${SFTP_PORT}"
    },
    "auth": {
      "url": "https://${PANEL_DOMAIN}"
    }
  },
  "database": {
    "type": "sqlite",
    "host": "${SKYPANEL_DATA}/SkyPanel.db"
  },
  "servers": {
    "folder": "${SKYPANEL_DATA}/servers"
  },
  "binaries": {
    "folder": "${SKYPANEL_DATA}/binaries"
  },
  "cache": {
    "folder": "${SKYPANEL_DATA}/cache"
  }
}
EOF
    
    chown "$SKYPANEL_USER:$SKYPANEL_GROUP" "$SKYPANEL_CONFIG/config.json"
    chmod 600 "$SKYPANEL_CONFIG/config.json"
    
    print_success "Configuración generada"
}

# Crear servicio systemd
create_systemd_service() {
    print_header "Creando Servicio Systemd"
    
    cat > "$SKYPANEL_SERVICE" <<EOF
[Unit]
Description=SkyPanel - Game Server Management Panel
Documentation=https://github.com/SkyPanel/SkyPanel
After=network.target

[Service]
Type=notify
WorkingDirectory=${SKYPANEL_DATA}
ExecStart=/usr/local/bin/skypanel runService --config ${SKYPANEL_CONFIG}/config.json
User=${SKYPANEL_USER}
Group=${SKYPANEL_GROUP}
Restart=always
RestartSec=5
TimeoutStopSec=5m
OOMPolicy=continue
Environment="GIN_MODE=release"
StandardOutput=journal
StandardError=journal
SyslogIdentifier=skypanel

# Seguridad
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${SKYPANEL_DATA} ${SKYPANEL_LOG} ${SKYPANEL_CONFIG}

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    systemctl enable skypanel > /dev/null 2>&1
    
    print_success "Servicio systemd creado y habilitado"
}

###############################################################################
# CONFIGURACIÓN DE NGINX
###############################################################################

# Configurar nginx para el panel
configure_nginx_panel() {
    print_header "Configurando Nginx para el Panel"
    
    cat > /etc/nginx/sites-available/skypanel <<EOF
# Redirección HTTP a HTTPS
server {
    listen 80;
    listen [::]:80;
    server_name ${PANEL_DOMAIN};

    # Permitir certificados Let's Encrypt
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    # Redirigir todo lo demás a HTTPS
    location / {
        return 301 https://\$server_name\$request_uri;
    }
}

# Configuración HTTPS
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${PANEL_DOMAIN};

    # Certificados SSL (se actualizarán después de obtenerlos)
    ssl_certificate /etc/letsencrypt/live/${PANEL_DOMAIN}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${PANEL_DOMAIN}/privkey.pem;
    
    # Configuración SSL moderna
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Headers de seguridad
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Logs
    access_log /var/log/nginx/skypanel-access.log;
    error_log /var/log/nginx/skypanel-error.log;

    # Tamaño máximo de archivo
    client_max_body_size 100M;
    client_body_timeout 60s;

    # Proxy al panel
    location / {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_cache_bypass \$http_upgrade;
        proxy_read_timeout 300s;
        proxy_connect_timeout 75s;
    }

    # WebSocket support
    location /socket.io/ {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF
    
    # Habilitar sitio
    if [[ -d /etc/nginx/sites-enabled ]]; then
        ln -sf /etc/nginx/sites-available/skypanel /etc/nginx/sites-enabled/
    else
        # Para sistemas que usan conf.d
        cp /etc/nginx/sites-available/skypanel /etc/nginx/conf.d/skypanel.conf
    fi
    
    print_success "Nginx configurado para el panel"
}

# Configurar nginx para status/gatus
configure_nginx_status() {
    print_header "Configurando Nginx para Status/Gatus"
    
    cat > /etc/nginx/sites-available/skypanel-status <<EOF
# Redirección HTTP a HTTPS
server {
    listen 80;
    listen [::]:80;
    server_name ${STATUS_DOMAIN};

    # Permitir certificados Let's Encrypt
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    # Redirigir todo lo demás a HTTPS
    location / {
        return 301 https://\$server_name\$request_uri;
    }
}

# Configuración HTTPS
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${STATUS_DOMAIN};

    # Certificados SSL (se actualizarán después de obtenerlos)
    ssl_certificate /etc/letsencrypt/live/${STATUS_DOMAIN}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${STATUS_DOMAIN}/privkey.pem;
    
    # Configuración SSL moderna
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Headers de seguridad
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;

    # Logs
    access_log /var/log/nginx/skypanel-status-access.log;
    error_log /var/log/nginx/skypanel-status-error.log;

    # Proxy a Gatus
    location / {
        proxy_pass http://127.0.0.1:${GATUS_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_cache_bypass \$http_upgrade;
        proxy_read_timeout 300s;
        proxy_connect_timeout 75s;
    }
}
EOF
    
    # Habilitar sitio
    if [[ -d /etc/nginx/sites-enabled ]]; then
        ln -sf /etc/nginx/sites-available/skypanel-status /etc/nginx/sites-enabled/
    else
        # Para sistemas que usan conf.d
        cp /etc/nginx/sites-available/skypanel-status /etc/nginx/conf.d/skypanel-status.conf
    fi
    
    print_success "Nginx configurado para status/gatus"
}

# Obtener certificados SSL
obtain_ssl_certificates() {
    print_header "Obteniendo Certificados SSL"
    
    # Verificar que nginx esté corriendo
    if ! systemctl is-active --quiet nginx; then
        print_info "Iniciando nginx..."
        systemctl start nginx || {
            print_error "No se pudo iniciar nginx"
            exit 1
        }
    fi
    
    # Verificar configuración de nginx
    if ! nginx -t > /dev/null 2>&1; then
        print_error "Error en la configuración de nginx"
        print_info "Ejecuta 'nginx -t' para ver los errores"
        exit 1
    fi
    
    # Recargar nginx con la nueva configuración
    systemctl reload nginx || {
        print_error "No se pudo recargar nginx"
        exit 1
    }
    
    # Obtener certificado para el panel
    print_info "Obteniendo certificado SSL para ${BOLD}$PANEL_DOMAIN${NC}..."
    print_warning "Asegúrate de que el dominio apunta a este servidor (DNS A record)"
    
    if certbot certonly --nginx \
        --non-interactive \
        --agree-tos \
        --email "$PANEL_EMAIL" \
        -d "$PANEL_DOMAIN" \
        --keep-until-expiring > /dev/null 2>&1; then
        print_success "Certificado SSL obtenido para $PANEL_DOMAIN"
    else
        print_warning "No se pudo obtener el certificado SSL para $PANEL_DOMAIN"
        print_warning "Verifica que:"
        print_warning "  1. El dominio $PANEL_DOMAIN apunta a este servidor"
        print_warning "  2. Los puertos 80 y 443 están abiertos"
        print_warning "  3. Puedes ejecutar manualmente: certbot --nginx -d $PANEL_DOMAIN"
    fi
    
    # Obtener certificado para status
    print_info "Obteniendo certificado SSL para ${BOLD}$STATUS_DOMAIN${NC}..."
    
    if certbot certonly --nginx \
        --non-interactive \
        --agree-tos \
        --email "$PANEL_EMAIL" \
        -d "$STATUS_DOMAIN" \
        --keep-until-expiring > /dev/null 2>&1; then
        print_success "Certificado SSL obtenido para $STATUS_DOMAIN"
    else
        print_warning "No se pudo obtener el certificado SSL para $STATUS_DOMAIN"
        print_warning "Verifica que:"
        print_warning "  1. El dominio $STATUS_DOMAIN apunta a este servidor"
        print_warning "  2. Los puertos 80 y 443 están abiertos"
        print_warning "  3. Puedes ejecutar manualmente: certbot --nginx -d $STATUS_DOMAIN"
    fi
    
    # Configurar renovación automática
    if systemctl list-units --type=timer 2>/dev/null | grep -q certbot.timer; then
        systemctl enable certbot.timer > /dev/null 2>&1 || true
        systemctl start certbot.timer > /dev/null 2>&1 || true
    fi
    
    # Recargar nginx con los certificados
    systemctl reload nginx
    
    print_success "Proceso de certificados SSL completado"
}

###############################################################################
# CONFIGURACIÓN DE FIREWALL
###############################################################################

configure_firewall() {
    print_header "Configurando Firewall"
    
    case $DISTRO in
        ubuntu|debian)
            if command -v ufw &> /dev/null; then
                print_info "Configurando UFW..."
                ufw --force enable > /dev/null 2>&1
                ufw allow 22/tcp > /dev/null 2>&1
                ufw allow 80/tcp > /dev/null 2>&1
                ufw allow 443/tcp > /dev/null 2>&1
                ufw allow ${SFTP_PORT}/tcp > /dev/null 2>&1
                print_success "UFW configurado"
            fi
            ;;
        fedora|rhel|centos)
            if command -v firewall-cmd &> /dev/null; then
                print_info "Configurando firewalld..."
                systemctl enable firewalld > /dev/null 2>&1
                systemctl start firewalld > /dev/null 2>&1
                firewall-cmd --permanent --add-service=ssh > /dev/null 2>&1
                firewall-cmd --permanent --add-service=http > /dev/null 2>&1
                firewall-cmd --permanent --add-service=https > /dev/null 2>&1
                firewall-cmd --permanent --add-port=${SFTP_PORT}/tcp > /dev/null 2>&1
                firewall-cmd --reload > /dev/null 2>&1
                print_success "Firewalld configurado"
            fi
            ;;
    esac
}

###############################################################################
# ENTRADA DE USUARIO
###############################################################################


get_user_input() {
    print_header "SkyPanel - Instalación Automática"
    
    echo -e "${CYAN}Este script instalará SkyPanel en tu servidor.${NC}"
    echo -e "${CYAN}Necesitarás proporcionar la siguiente información:${NC}"
    echo ""
    
    # Solicitar dominio del panel
    while [[ -z "$PANEL_DOMAIN" ]]; do
        read -p "$(echo -e "${BLUE}Ingresa el dominio para el panel ${BOLD}(ej: panel.tudominio.com):${NC}") " PANEL_DOMAIN
        if [[ -z "$PANEL_DOMAIN" ]]; then
            print_error "El dominio no puede estar vacío"
        fi
    done
    
    # Solicitar dominio de status
    while [[ -z "$STATUS_DOMAIN" ]]; do
        read -p "$(echo -e "${BLUE}Ingresa el dominio para status/gatus ${BOLD}(ej: status.tudominio.com):${NC}") " STATUS_DOMAIN
        if [[ -z "$STATUS_DOMAIN" ]]; then
            print_error "El dominio no puede estar vacío"
        fi
    done
    
    # Solicitar datos del administrador
    echo ""
    echo -e "${CYAN}Configuración del Administrador:${NC}"
    
    while [[ -z "$ADMIN_USER" ]]; do
        read -p "$(echo -e "${BLUE}Nombre de usuario administrador:${NC} ") " ADMIN_USER
        if [[ -z "$ADMIN_USER" ]]; then
            print_error "El usuario no puede estar vacío"
        fi
    done

    while [[ -z "$ADMIN_EMAIL" ]]; do
        read -p "$(echo -e "${BLUE}Email del administrador:${NC} ") " ADMIN_EMAIL
        if [[ -z "$ADMIN_EMAIL" ]]; then
            print_error "El email no puede estar vacío"
        elif [[ ! "$ADMIN_EMAIL" =~ ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ ]]; then
            print_error "El email no tiene un formato válido"
            ADMIN_EMAIL=""
        fi
    done
    
    # Usar el mismo email para SSL por defecto (si el usuario quiere)
    PANEL_EMAIL="$ADMIN_EMAIL"
    
    while [[ -z "$ADMIN_PASSWORD" ]]; do
        read -s -p "$(echo -e "${BLUE}Contraseña del administrador:${NC} ") " ADMIN_PASSWORD
        echo ""
        if [[ -z "$ADMIN_PASSWORD" ]]; then
            print_error "La contraseña no puede estar vacía"
        elif [[ ${#ADMIN_PASSWORD} -lt 8 ]]; then
            print_error "La contraseña debe tener al menos 8 caracteres"
            ADMIN_PASSWORD=""
        fi
    done
    
    echo ""
    echo -e "${YELLOW}${BOLD}Resumen de la instalación:${NC}"
    echo -e "  Panel:     ${GREEN}https://${PANEL_DOMAIN}${NC}"
    echo -e "  Status:    ${GREEN}https://${STATUS_DOMAIN}${NC}"
    echo -e "  Admin:     ${GREEN}${ADMIN_USER} (${ADMIN_EMAIL})${NC}"
    echo ""
    read -p "$(echo -e "${YELLOW}¿Continuar con la instalación? ${BOLD}(s/N):${NC}") " confirm
    if [[ ! "$confirm" =~ ^[Ss]$ ]]; then
        print_info "Instalación cancelada por el usuario"
        exit 0
    fi
    echo ""
}

create_admin_user() {
    print_header "Creando Usuario Administrador"
    
    # Asegurar permisos de la carpeta data antes de escribir la DB
    chown -R "$SKYPANEL_USER:$SKYPANEL_GROUP" "$SKYPANEL_DATA"
    
    print_info "Registrando usuario ${ADMIN_USER}..."
    
    # Ejecutar comando como el usuario skypanel para que la DB tenga los permisos correctos
    if runuser -u "$SKYPANEL_USER" -- /usr/local/bin/skypanel user add --config "$SKYPANEL_CONFIG/config.json" --name "$ADMIN_USER" --email "$ADMIN_EMAIL" --password "$ADMIN_PASSWORD" --admin; then
        print_success "Usuario administrador creado exitosamente"
    else
        print_warning "No se pudo crear el usuario automáticamente."
        print_warning "Deberás crearlo manualmente usando: skypanel user add"
    fi
}

###############################################################################
# FUNCIÓN PRINCIPAL
###############################################################################

main() {
    # Banner
    clear
    print_header "SkyPanel - Instalador Automático v${INSTALLER_VERSION}"
    
    # Verificaciones iniciales
    check_root
    detect_distro
    
    # Solicitar información al usuario
    get_user_input
    
    # Proceso de instalación
    print_info "Iniciando instalación de SkyPanel..."
    echo ""
    
    install_dependencies
    install_go
    install_nodejs
    create_user_and_directories
    clone_and_build
    install_files
    generate_config
    create_admin_user
    create_systemd_service
    configure_nginx_panel
    configure_nginx_status
    obtain_ssl_certificates
    configure_firewall
    
    # Iniciar el servicio
    print_header "Iniciando Servicio"
    print_info "Iniciando SkyPanel..."
    systemctl start skypanel
    
    # Esperar un momento para que el servicio inicie
    sleep 3
    
    if systemctl is-active --quiet skypanel; then
        print_success "SkyPanel está corriendo"
    else
        print_warning "SkyPanel no se inició correctamente"
        print_warning "Revisa los logs con: ${BOLD}journalctl -u skypanel -f${NC}"
    fi
    
    # Mensaje final
    echo ""
    print_header "¡Instalación Completada!"
    
    echo -e "${GREEN}${BOLD}SkyPanel ha sido instalado exitosamente${NC}"
    echo ""
    echo -e "  Panel:  ${CYAN}https://${PANEL_DOMAIN}${NC}"
    echo -e "  Status: ${CYAN}https://${STATUS_DOMAIN}${NC}"
    echo ""
    echo -e "${YELLOW}${BOLD}Tus Credenciales:${NC}"
    echo -e "  Usuario: ${BOLD}${ADMIN_USER}${NC}"
    echo -e "  Email:   ${BOLD}${ADMIN_EMAIL}${NC}"
    echo ""
    echo -e "${YELLOW}${BOLD}Comandos útiles:${NC}"
    echo -e "  Ver estado:       ${BOLD}systemctl status skypanel${NC}"
    echo -e "  Reiniciar:        ${BOLD}systemctl restart skypanel${NC}"
    echo -e "  Cambiar password: ${BOLD}skypanel user password --email \"${ADMIN_EMAIL}\" --password \"NuevaPassword\"${NC}"
    echo ""
    echo -e "${GREEN}¡Gracias por usar SkyPanel!${NC}"
    echo ""
}

# Ejecutar función principal
main "$@"
