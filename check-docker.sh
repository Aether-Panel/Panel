#!/bin/bash

###############################################################################
# Script de Verificación Pre-Docker
# Verifica que todo esté listo antes de construir
###############################################################################

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Verificación Pre-Docker para SkyPanel${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo ""

ERRORS=0
WARNINGS=0

# Verificar Docker
echo -n "Verificando Docker... "
if command -v docker &> /dev/null; then
    DOCKER_VERSION=$(docker --version | awk '{print $3}' | sed 's/,//')
    echo -e "${GREEN}✓${NC} Instalado (${DOCKER_VERSION})"
    
    # Verificar que Docker esté corriendo
    echo -n "Verificando Docker daemon... "
    if docker info &> /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Corriendo"
    else
        echo -e "${RED}✗${NC} No está corriendo"
        echo "  Ejecuta: sudo systemctl start docker"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo -e "${RED}✗${NC} No instalado"
    echo "  Instala desde: https://docs.docker.com/get-docker/"
    ERRORS=$((ERRORS + 1))
fi

# Verificar Docker Compose
echo -n "Verificando Docker Compose... "
if command -v docker-compose &> /dev/null; then
    COMPOSE_VERSION=$(docker-compose --version | awk '{print $3}' | sed 's/,//')
    echo -e "${GREEN}✓${NC} Instalado (${COMPOSE_VERSION})"
elif docker compose version &> /dev/null 2>&1; then
    COMPOSE_VERSION=$(docker compose version --short)
    echo -e "${GREEN}✓${NC} Instalado (${COMPOSE_VERSION})"
else
    echo -e "${RED}✗${NC} No instalado"
    ERRORS=$((ERRORS + 1))
fi

# Verificar puertos
echo ""
echo "Verificando puertos disponibles..."
for port in 8080 5657 8081; do
    echo -n "  Puerto $port... "
    if ! sudo netstat -tulpn 2>/dev/null | grep -q ":$port " && ! ss -tulpn 2>/dev/null | grep -q ":$port "; then
        echo -e "${GREEN}✓${NC} Disponible"
    else
        echo -e "${YELLOW}⚠${NC}  En uso"
        WARNINGS=$((WARNINGS + 1))
    fi
done

# Verificar espacio en disco
echo ""
echo -n "Verificando espacio en disco... "
AVAILABLE=$(df -BG . | tail -1 | awk '{print $4}' | sed 's/G//')
if [ "$AVAILABLE" -ge 5 ]; then
    echo -e "${GREEN}✓${NC} ${AVAILABLE}GB disponibles"
else
    echo -e "${YELLOW}⚠${NC}  Solo ${AVAILABLE}GB disponibles (se recomiendan 5GB+)"
    WARNINGS=$((WARNINGS + 1))
fi

# Verificar RAM
echo -n "Verificando RAM... "
TOTAL_RAM=$(free -g | awk '/^Mem:/{print $2}')
if [ "$TOTAL_RAM" -ge 2 ]; then
    echo -e "${GREEN}✓${NC} ${TOTAL_RAM}GB total"
else
    echo -e "${YELLOW}⚠${NC}  Solo ${TOTAL_RAM}GB total (se recomiendan 2GB+)"
    WARNINGS=$((WARNINGS + 1))
fi

# Verificar archivos necesarios
echo ""
echo "Verificando archivos de configuración..."
for file in Dockerfile docker-compose.yml docker-compose.dev.yml docker-test.sh quickstart-docker.sh; do
    echo -n "  $file... "
    if [ -f "$file" ]; then
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${RED}✗${NC} No encontrado"
        ERRORS=$((ERRORS + 1))
    fi
done

# Verificar permisos de scripts
echo ""
echo "Verificando permisos de scripts..."
for script in docker-test.sh quickstart-docker.sh; do
    echo -n "  $script... "
    if [ -x "$script" ]; then
        echo -e "${GREEN}✓${NC} Ejecutable"
    else
        echo -e "${YELLOW}⚠${NC}  No ejecutable"
        chmod +x "$script" 2>/dev/null && echo -e "    ${GREEN}✓${NC} Permisos corregidos" || echo -e "    ${RED}✗${NC} No se pudieron corregir"
    fi
done

# Resumen
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ Todo listo para ejecutar Docker!${NC}"
    echo ""
    echo "Siguiente paso:"
    echo -e "  ${YELLOW}./quickstart-docker.sh${NC}"
    echo ""
    echo "O paso a paso:"
    echo -e "  ${YELLOW}./docker-test.sh build${NC}"
    echo -e "  ${YELLOW}./docker-test.sh start${NC}"
    echo -e "  ${YELLOW}./docker-test.sh admin${NC}"
else
    echo -e "${RED}✗ Se encontraron $ERRORS errores${NC}"
    echo "Por favor, corrige los errores antes de continuar"
fi

if [ $WARNINGS -gt 0 ]; then
    echo -e "${YELLOW}⚠ $WARNINGS advertencias (puedes continuar)${NC}"
fi
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"

exit $ERRORS
