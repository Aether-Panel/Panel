#!/bin/bash

###############################################################################
# SkyPanel - Script de Instalación Automática
# Versión: 1.0.0
# Repositorio: https://github.com/SkyPanel/SkyPanel
###############################################################################

set -e

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Variables globales
INSTALL_DIR="/opt/skypanel"
DATA_DIR="/var/lib/skypanel"
CONFIG_DIR="/etc/skypanel"
LOG_DIR="/var/log/skypanel"
WEB_DIR="/var/www/skypanel"
SERVICE_USER="skypanel"
GITHUB_REPO="https://github.com/SkyPanel/SkyPanel.git"
INSTALL_TYPE=""
OS_TYPE=""
DOMAIN=""
PANEL_IP=""
PANEL_PORT="8080"
SFTP_PORT="5657"
GATUS_PORT="8081"
USE_DOMAIN=false
USE_SSL=false
SSL_EMAIL=""

# Función para imprimir mensajes
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
    echo -e "${RED}[✗]${NC} $1"
}

# Función para verificar si se ejecuta como root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "Este script debe ejecutarse como root (usa sudo)"
        exit 1
    fi
}

# Función para detectar el sistema operativo
detect_os() {
    print_info "Detectando sistema operativo..."
    
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS_TYPE=$ID
        OS_VERSION=$VERSION_ID
    elif [[ -f /etc/redhat-release ]]; then
        OS_TYPE="rhel"
        OS_VERSION=$(cat /etc/redhat-release | sed 's/.*release \([0-9.]*\).*/\1/')
    else
        print_error "No se pudo detectar el sistema operativo"
        exit 1
    fi
    
    print_success "Sistema operativo detectado: $OS_TYPE $OS_VERSION"
}

# Función para instalar dependencias según el SO
install_dependencies() {
    print_info "Instalando dependencias del sistema..."
    
    case $OS_TYPE in
        ubuntu|debian)
            apt update
            apt install -y curl wget git build-essential sqlite3 ufw nginx certbot python3-certbot-nginx
            ;;
        fedora)
            dnf install -y curl wget git gcc make sqlite firewalld nginx certbot python3-certbot-nginx
            ;;
        rhel|centos)
            if command -v dnf &> /dev/null; then
                dnf install -y curl wget git gcc make sqlite firewalld nginx certbot python3-certbot-nginx
            else
                yum install -y curl wget git gcc make sqlite firewalld nginx certbot python3-certbot-nginx
            fi
            ;;
        *)
            print_error "Sistema operativo no soportado: $OS_TYPE"
            exit 1
            ;;
    esac
    
    print_success "Dependencias del sistema instaladas"
}

# Función para instalar Docker y Docker Compose
install_docker() {
    print_info "Instalando Docker y Docker Compose..."
    
    # Verificar si Docker ya está instalado
    if command -v docker &> /dev/null; then
        print_warning "Docker ya está instalado"
        return
    fi
    
    case $OS_TYPE in
        ubuntu|debian)
            # Instalar Docker
            curl -fsSL https://get.docker.com -o get-docker.sh
            sh get-docker.sh
            rm get-docker.sh
            
            # Instalar Docker Compose
            curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
            chmod +x /usr/local/bin/docker-compose
            ;;
        fedora)
            dnf install -y docker docker-compose
            systemctl enable docker
            systemctl start docker
            ;;
        rhel|centos)
            if command -v dnf &> /dev/null; then
                dnf install -y docker docker-compose
            else
                yum install -y docker docker-compose
            fi
            systemctl enable docker
            systemctl start docker
            ;;
    esac
    
    # Agregar usuario actual al grupo docker (si no es root)
    if [[ $EUID -ne 0 ]] && [[ -n "$SUDO_USER" ]]; then
        usermod -aG docker $SUDO_USER
    fi
    
    print_success "Docker y Docker Compose instalados"
}

# Función para instalar Go
install_go() {
    print_info "Instalando Go..."
    
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        print_warning "Go ya está instalado (versión $GO_VERSION)"
        return
    fi
    
    GO_VERSION="1.24.4"
    ARCH=$(uname -m)
    
    case $ARCH in
        x86_64)
            GO_ARCH="amd64"
            ;;
        aarch64|arm64)
            GO_ARCH="arm64"
            ;;
        *)
            print_error "Arquitectura no soportada: $ARCH"
            exit 1
            ;;
    esac
    
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    
    # Agregar Go al PATH
    if ! grep -q "/usr/local/go/bin" /etc/profile; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    fi
    export PATH=$PATH:/usr/local/go/bin
    
    print_success "Go ${GO_VERSION} instalado"
}

# Función para instalar Node.js y Yarn
install_nodejs() {
    print_info "Instalando Node.js y Yarn..."
    
    if command -v node &> /dev/null; then
        NODE_VERSION=$(node --version)
        print_warning "Node.js ya está instalado (versión $NODE_VERSION)"
    else
        case $OS_TYPE in
            ubuntu|debian)
                curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
                apt install -y nodejs
                ;;
            fedora)
                curl -fsSL https://rpm.nodesource.com/setup_22.x | bash -
                dnf install -y nodejs
                ;;
            rhel|centos)
                curl -fsSL https://rpm.nodesource.com/setup_22.x | bash -
                if command -v dnf &> /dev/null; then
                    dnf install -y nodejs
                else
                    yum install -y nodejs
                fi
                ;;
        esac
        print_success "Node.js instalado"
    fi
    
    if ! command -v yarn &> /dev/null; then
        npm install -g yarn
        print_success "Yarn instalado"
    else
        print_warning "Yarn ya está instalado"
    fi
}

# Función para configurar firewall
configure_firewall() {
    print_info "Configurando firewall..."
    
    case $OS_TYPE in
        ubuntu|debian)
            # UFW
            ufw --force enable
            ufw allow 22/tcp  # SSH
            ufw allow ${PANEL_PORT}/tcp
            ufw allow ${SFTP_PORT}/tcp
            ufw allow ${GATUS_PORT}/tcp
            if [[ "$USE_SSL" == true ]]; then
                ufw allow 80/tcp   # HTTP para Let's Encrypt
                ufw allow 443/tcp # HTTPS
            fi
            print_success "Firewall (UFW) configurado"
            ;;
        fedora|rhel|centos)
            # Firewalld
            if ! systemctl is-active --quiet firewalld; then
                systemctl enable firewalld
                systemctl start firewalld
            fi
            firewall-cmd --permanent --add-service=ssh
            firewall-cmd --permanent --add-port=${PANEL_PORT}/tcp
            firewall-cmd --permanent --add-port=${SFTP_PORT}/tcp
            firewall-cmd --permanent --add-port=${GATUS_PORT}/tcp
            if [[ "$USE_SSL" == true ]]; then
                firewall-cmd --permanent --add-service=http
                firewall-cmd --permanent --add-service=https
            fi
            firewall-cmd --reload
            print_success "Firewall (firewalld) configurado"
            ;;
    esac
}

# Función para obtener la IP del servidor
get_server_ip() {
    # Intentar obtener IP pública
    PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || curl -s ipinfo.io/ip 2>/dev/null || echo "")
    
    # Intentar obtener IP local
    if [[ -n "$PUBLIC_IP" ]]; then
        PANEL_IP=$PUBLIC_IP
    else
        # Fallback a IP local
        PANEL_IP=$(hostname -I | awk '{print $1}')
    fi
    
    print_info "IP del servidor detectada: $PANEL_IP"
}

# Función para preguntar configuración
ask_configuration() {
    print_info "Configuración de instalación"
    echo ""
    
    # Tipo de instalación
    echo "¿Qué tipo de instalación deseas?"
    echo "1) Con Docker (recomendado)"
    echo "2) Sin Docker (instalación nativa)"
    read -p "Selecciona una opción [1-2]: " install_choice
    
    case $install_choice in
        1)
            INSTALL_TYPE="docker"
            ;;
        2)
            INSTALL_TYPE="native"
            ;;
        *)
            print_error "Opción inválida"
            exit 1
            ;;
    esac
    
    # Dominio o IP
    echo ""
    read -p "¿Deseas usar un dominio? (s/n): " use_domain_choice
    if [[ "$use_domain_choice" == "s" || "$use_domain_choice" == "S" ]]; then
        USE_DOMAIN=true
        read -p "Ingresa el dominio (ej: panel.tudominio.com): " DOMAIN
        
        # SSL
        read -p "¿Deseas configurar SSL con Let's Encrypt? (s/n): " use_ssl_choice
        if [[ "$use_ssl_choice" == "s" || "$use_ssl_choice" == "S" ]]; then
            USE_SSL=true
            read -p "Ingresa tu email para Let's Encrypt: " SSL_EMAIL
        fi
    else
        USE_DOMAIN=false
        get_server_ip
    fi
    
    # Puertos (opcional)
    echo ""
    read -p "Puerto del panel web [8080]: " custom_port
    if [[ -n "$custom_port" ]]; then
        PANEL_PORT=$custom_port
    fi
    
    read -p "Puerto SFTP [5657]: " custom_sftp
    if [[ -n "$custom_sftp" ]]; then
        SFTP_PORT=$custom_sftp
    fi
    
    read -p "Puerto Gatus [8081]: " custom_gatus
    if [[ -n "$custom_gatus" ]]; then
        GATUS_PORT=$custom_gatus
    fi
}

# Función para instalar con Docker
install_docker_version() {
    print_info "Instalando SkyPanel con Docker..."
    
    # Crear directorios
    mkdir -p $INSTALL_DIR
    cd $INSTALL_DIR
    
    # Clonar repositorio
    print_info "Clonando repositorio desde GitHub..."
    if [[ -d ".git" ]]; then
        print_warning "El directorio ya contiene un repositorio Git, actualizando..."
        git pull
    else
        git clone $GITHUB_REPO .
    fi
    
    # Configurar docker-compose.yml
    print_info "Configurando docker-compose.yml..."
    
    # Actualizar puertos en docker-compose.yml si es necesario
    if [[ -f "docker-compose.yml" ]]; then
        sed -i "s/8080:8080/${PANEL_PORT}:8080/g" docker-compose.yml
        sed -i "s/5657:5657/${SFTP_PORT}:5657/g" docker-compose.yml
        sed -i "s/8081:8081/${GATUS_PORT}:8081/g" docker-compose.yml
    fi
    
    # Construir y levantar contenedores
    print_info "Construyendo imagen Docker..."
    docker-compose build
    
    print_info "Iniciando contenedores..."
    docker-compose up -d
    
    print_success "SkyPanel instalado con Docker"
}

# Función para instalar sin Docker
install_native_version() {
    print_info "Instalando SkyPanel sin Docker..."
    
    # Crear usuario del sistema
    if ! id "$SERVICE_USER" &>/dev/null; then
        useradd -r -m -d $DATA_DIR -s /bin/bash $SERVICE_USER
        print_success "Usuario $SERVICE_USER creado"
    fi
    
    # Crear directorios
    mkdir -p $INSTALL_DIR $DATA_DIR $CONFIG_DIR $LOG_DIR $WEB_DIR
    chown -R $SERVICE_USER:$SERVICE_USER $DATA_DIR $LOG_DIR
    
    # Clonar repositorio
    print_info "Clonando repositorio desde GitHub..."
    cd $INSTALL_DIR
    if [[ -d ".git" ]]; then
        print_warning "El directorio ya contiene un repositorio Git, actualizando..."
        git pull
    else
        git clone $GITHUB_REPO .
    fi
    
    # Compilar frontend
    print_info "Compilando frontend..."
    cd client/frontend
    yarn install
    yarn build
    cp -r dist/* $WEB_DIR/
    chown -R www-data:www-data $WEB_DIR
    
    # Compilar backend
    print_info "Compilando backend..."
    cd $INSTALL_DIR
    export PATH=$PATH:/usr/local/go/bin
    go mod download
    go build -o /usr/local/bin/skypanel ./cmd
    chmod +x /usr/local/bin/skypanel
    
    # Crear archivo de configuración
    print_info "Creando archivo de configuración..."
    cat > $CONFIG_DIR/config.json <<EOF
{
  "logs": "$LOG_DIR",
  "panel": {
    "database": {
      "dialect": "sqlite3",
      "url": "file:$DATA_DIR/database.db?cache=shared"
    },
    "web": {
      "files": "$WEB_DIR"
    },
    "gatus": {
      "enable": true
    }
  },
  "daemon": {
    "data": {
      "root": "$DATA_DIR"
    }
  }
}
EOF
    chown $SERVICE_USER:$SERVICE_USER $CONFIG_DIR/config.json
    
    # Crear servicio systemd
    print_info "Creando servicio systemd..."
    cat > /etc/systemd/system/skypanel.service <<EOF
[Unit]
Description=SkyPanel - Panel de Gestión de Servidores de Juegos
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
WorkingDirectory=$DATA_DIR
ExecStart=/usr/local/bin/skypanel run
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    systemctl enable skypanel
    systemctl start skypanel
    
    print_success "SkyPanel instalado sin Docker"
}

# Función para configurar Nginx
configure_nginx() {
    print_info "Configurando Nginx..."
    
    if [[ "$USE_DOMAIN" == true ]]; then
        # Configuración con dominio
        if [[ "$USE_SSL" == true ]]; then
            # HTTPS con SSL
            cat > /etc/nginx/sites-available/skypanel <<EOF
server {
    listen 80;
    server_name $DOMAIN;
    
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }
    
    location / {
        return 301 https://\$server_name\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name $DOMAIN;
    
    ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;
    
    # Configuración SSL
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    # Proxy para el panel
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
    }
    
    # WebSocket para consola
    location /ws {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    }
}
EOF
        else
            # HTTP sin SSL
            cat > /etc/nginx/sites-available/skypanel <<EOF
server {
    listen 80;
    server_name $DOMAIN;
    
    location / {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_cache_bypass \$http_upgrade;
    }
    
    location /ws {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    }
}
EOF
        fi
    else
        # Configuración con IP
        cat > /etc/nginx/sites-available/skypanel <<EOF
server {
    listen 80;
    server_name $PANEL_IP;
    
    location / {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_cache_bypass \$http_upgrade;
    }
    
    location /ws {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    }
}
EOF
    fi
    
    # Habilitar sitio
    if [[ -d "/etc/nginx/sites-enabled" ]]; then
        # Ubuntu/Debian
        ln -sf /etc/nginx/sites-available/skypanel /etc/nginx/sites-enabled/
        rm -f /etc/nginx/sites-enabled/default
    else
        # Fedora/RHEL/CentOS usan conf.d directamente
        if [[ ! -d "/etc/nginx/sites-available" ]]; then
            mkdir -p /etc/nginx/sites-available
        fi
        cp /etc/nginx/sites-available/skypanel /etc/nginx/conf.d/skypanel.conf
    fi
    
    # Probar configuración
    nginx -t
    
    # Reiniciar Nginx
    systemctl restart nginx
    systemctl enable nginx
    
    # Configurar SSL si se solicitó
    if [[ "$USE_SSL" == true && "$USE_DOMAIN" == true ]]; then
        print_info "Configurando SSL con Let's Encrypt..."
        certbot --nginx -d $DOMAIN --non-interactive --agree-tos --email $SSL_EMAIL
        systemctl restart nginx
    fi
    
    print_success "Nginx configurado"
}

# Función principal
main() {
    clear
    echo "=========================================="
    echo "  SkyPanel - Instalador Automático"
    echo "=========================================="
    echo ""
    
    check_root
    detect_os
    ask_configuration
    
    # Instalar dependencias base
    install_dependencies
    
    # Instalar según el tipo
    if [[ "$INSTALL_TYPE" == "docker" ]]; then
        install_docker
        install_docker_version
    else
        install_go
        install_nodejs
        install_native_version
    fi
    
    # Configurar firewall
    configure_firewall
    
    # Configurar Nginx
    configure_nginx
    
    # Resumen final
    echo ""
    echo "=========================================="
    print_success "¡Instalación completada!"
    echo "=========================================="
    echo ""
    
    if [[ "$USE_DOMAIN" == true ]]; then
        if [[ "$USE_SSL" == true ]]; then
            echo "Accede al panel en: https://$DOMAIN"
        else
            echo "Accede al panel en: http://$DOMAIN"
        fi
    else
        echo "Accede al panel en: http://$PANEL_IP:$PANEL_PORT"
    fi
    
    echo ""
    echo "Puertos configurados:"
    echo "  - Panel Web: $PANEL_PORT"
    echo "  - SFTP: $SFTP_PORT"
    echo "  - Gatus: $GATUS_PORT"
    echo ""
    
    if [[ "$INSTALL_TYPE" == "docker" ]]; then
        echo "Comandos útiles:"
        echo "  - Ver logs: docker-compose logs -f"
        echo "  - Detener: docker-compose stop"
        echo "  - Iniciar: docker-compose start"
        echo "  - Reiniciar: docker-compose restart"
    else
        echo "Comandos útiles:"
        echo "  - Ver estado: systemctl status skypanel"
        echo "  - Ver logs: journalctl -u skypanel -f"
        echo "  - Detener: systemctl stop skypanel"
        echo "  - Iniciar: systemctl start skypanel"
        echo "  - Reiniciar: systemctl restart skypanel"
    fi
    echo ""
}

# Ejecutar función principal
main
