#!/bin/bash

###############################################################################
# Script de Inicio Rápido para SkyPanel en Docker
# Ejecuta este script para probar SkyPanel en minutos
###############################################################################

set -e

# Colores
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}"
cat << "EOF"
   _____ _          ____                  _ 
  / ____| |        |  _ \                | |
 | (___ | | ___   _| |_) | __ _ _ __   ___| |
  \___ \| |/ / | | |  _ < / _` | '_ \ / _ \ |
  ____) |   <| |_| | |_) | (_| | | | |  __/ |
 |_____/|_|\_\\__, |____/ \__,_|_| |_|\___|_|
               __/ |                          
              |___/                           

EOF
echo -e "${NC}"

echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Inicio Rápido de SkyPanel en Docker${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo ""

# Verificar Docker
echo -e "${YELLOW}[1/5]${NC} Verificando Docker..."
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}⚠${NC}  Docker no está instalado"
    echo "   Instala Docker desde: https://docs.docker.com/get-docker/"
    exit 1
fi

# Verificar si Docker funciona sin sudo
DOCKER_CMD="docker"
if ! docker info &> /dev/null 2>&1; then
    # Intentar con sudo
    if sudo docker info &> /dev/null 2>&1; then
        echo -e "${YELLOW}⚠${NC}  Docker requiere sudo"
        echo "   Para usar sin sudo, ejecuta:"
        echo "   ${BLUE}sudo usermod -aG docker $USER${NC}"
        echo "   ${BLUE}newgrp docker${NC}"
        echo ""
        read -p "¿Continuar con sudo? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
        DOCKER_CMD="sudo docker"
        COMPOSE_CMD="sudo docker-compose"
    else
        echo -e "${YELLOW}⚠${NC}  Docker no está corriendo"
        echo "   Ejecuta: sudo systemctl start docker"
        exit 1
    fi
else
    COMPOSE_CMD="docker-compose"
fi

echo -e "${GREEN}✓${NC} Docker está listo"

# Verificar Docker Compose
echo -e "${YELLOW}[2/5]${NC} Verificando Docker Compose..."
if command -v docker-compose &> /dev/null; then
    if [[ "$DOCKER_CMD" == "sudo docker" ]]; then
        COMPOSE_CMD="sudo docker-compose"
    else
        COMPOSE_CMD="docker-compose"
    fi
    echo -e "${GREEN}✓${NC} Docker Compose está listo"
elif $DOCKER_CMD compose version &> /dev/null 2>&1; then
    COMPOSE_CMD="$DOCKER_CMD compose"
    echo -e "${GREEN}✓${NC} Docker Compose está listo (plugin)"
else
    echo -e "${YELLOW}⚠${NC}  Docker Compose no está instalado"
    exit 1
fi

# Construir imagen
echo ""
echo -e "${YELLOW}[3/5]${NC} Construyendo imagen de Docker..."
echo "   ${BLUE}Esto puede tardar 10-15 minutos la primera vez${NC}"
echo ""

# Usar un archivo temporal para logs para no perder el código de salida
BUILD_LOG=$(mktemp)
if $DOCKER_CMD build -t skypanel:latest \
    --build-arg version=quickstart-$(date +%Y%m%d) \
    --build-arg sha=local \
    . > "$BUILD_LOG" 2>&1; then
    grep -E "Step|Successfully|ERROR" "$BUILD_LOG" || cat "$BUILD_LOG"
    rm "$BUILD_LOG"
    echo ""
    echo -e "${GREEN}✓${NC} Imagen construida exitosamente"
else
    echo -e "${RED}✗ Error al construir la imagen. Detalles completos:${NC}"
    echo "---------------------------------------------------"
    cat "$BUILD_LOG"
    echo "---------------------------------------------------"
    rm "$BUILD_LOG"
    exit 1
fi

# Crear directorios
echo ""
echo -e "${YELLOW}[4/5]${NC} Preparando directorios..."
mkdir -p dev-data/{config,data,logs}
cp config.docker.json dev-data/config/config.json
chmod -R 777 dev-data
echo -e "${GREEN}✓${NC} Directorios creados"

# Iniciar contenedor
echo ""
echo -e "${YELLOW}[5/5]${NC} Iniciando contenedor..."
if $COMPOSE_CMD -f docker-compose.dev.yml up -d; then
    echo -e "${GREEN}✓${NC} Contenedor iniciado"
else
    echo -e "${YELLOW}⚠${NC}  Error al iniciar el contenedor"
    exit 1
fi

# Esperar a que el servicio esté listo
echo ""
echo -e "${BLUE}Esperando a que el servicio esté listo...${NC}"
sleep 8

# Verificar estado
if $DOCKER_CMD ps | grep -q skypanel-dev; then
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  ✓ SkyPanel está corriendo exitosamente!${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${BLUE}📱 Acceso al Panel:${NC}"
    echo -e "   Panel Web:  ${GREEN}http://localhost:8080${NC}"
    echo -e "   Gatus:      ${GREEN}http://localhost:8081${NC}"
    echo -e "   SFTP:       ${GREEN}localhost:5657${NC}"
    echo ""
    echo -e "${BLUE}👤 Crear Usuario Administrador:${NC}"
    echo -e "   ${YELLOW}./docker-test.sh admin${NC}"
    echo ""
    echo -e "${BLUE}📊 Comandos Útiles:${NC}"
    echo -e "   Ver logs:    ${YELLOW}./docker-test.sh logs${NC}"
    echo -e "   Ver estado:  ${YELLOW}./docker-test.sh status${NC}"
    echo -e "   Detener:     ${YELLOW}./docker-test.sh stop${NC}"
    echo -e "   Ayuda:       ${YELLOW}./docker-test.sh help${NC}"
    echo ""
    echo -e "${BLUE}📚 Documentación:${NC}"
    echo -e "   ${YELLOW}cat README.docker.md${NC}"
    echo ""
else
    echo -e "${YELLOW}⚠${NC}  El contenedor no está corriendo correctamente"
    echo "   Ver logs: docker-compose -f docker-compose.dev.yml logs"
    exit 1
fi
