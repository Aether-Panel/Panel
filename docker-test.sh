#!/bin/bash

###############################################################################
# Script de Prueba de SkyPanel en Docker
# 
# Este script facilita la construcción y prueba de SkyPanel en contenedores
###############################################################################

set -e

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo ""
}

# Verificar que Docker está instalado
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker no está instalado"
        print_info "Instala Docker desde: https://docs.docker.com/get-docker/"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        print_error "Docker no está corriendo o no tienes permisos"
        print_info "Ejecuta: sudo systemctl start docker"
        print_info "O agrega tu usuario al grupo docker: sudo usermod -aG docker $USER"
        exit 1
    fi
    
    print_success "Docker está instalado y corriendo"
}

# Verificar Docker Compose
check_docker_compose() {
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_error "Docker Compose no está instalado"
        exit 1
    fi
    print_success "Docker Compose está instalado"
}

# Construir imagen
build_image() {
    print_header "Construyendo Imagen de Docker"
    
    print_info "Esto puede tardar varios minutos..."
    
    if docker build -t skypanel:latest \
        --build-arg version=dev-$(date +%Y%m%d) \
        --build-arg sha=$(git rev-parse --short HEAD 2>/dev/null || echo "local") \
        . ; then
        print_success "Imagen construida exitosamente"
    else
        print_error "Error al construir la imagen"
        exit 1
    fi
}

# Iniciar contenedor
start_container() {
    print_header "Iniciando Contenedor"
    
    # Crear directorios para desarrollo
    mkdir -p dev-data/{config,data,logs}
    
    if docker-compose -f docker-compose.dev.yml up -d; then
        print_success "Contenedor iniciado"
        
        # Esperar a que el servicio esté listo
        print_info "Esperando a que el servicio esté listo..."
        sleep 5
        
        # Verificar estado
        if docker ps | grep -q skypanel-dev; then
            print_success "SkyPanel está corriendo"
            echo ""
            print_info "Accede al panel en: ${GREEN}http://localhost:8080${NC}"
            print_info "Gatus (monitoring) en: ${GREEN}http://localhost:8081${NC}"
            print_info "SFTP en puerto: ${GREEN}5657${NC}"
            echo ""
            print_info "Ver logs: ${YELLOW}docker-compose -f docker-compose.dev.yml logs -f${NC}"
            print_info "Detener: ${YELLOW}docker-compose -f docker-compose.dev.yml down${NC}"
        else
            print_error "El contenedor no está corriendo"
            print_info "Ver logs: docker-compose -f docker-compose.dev.yml logs"
        fi
    else
        print_error "Error al iniciar el contenedor"
        exit 1
    fi
}

# Ver logs
show_logs() {
    print_header "Logs del Contenedor"
    docker-compose -f docker-compose.dev.yml logs -f
}

# Detener contenedor
stop_container() {
    print_header "Deteniendo Contenedor"
    
    if docker-compose -f docker-compose.dev.yml down; then
        print_success "Contenedor detenido"
    else
        print_error "Error al detener el contenedor"
        exit 1
    fi
}

# Limpiar todo
clean_all() {
    print_header "Limpiando Todo"
    
    print_warning "Esto eliminará el contenedor, volúmenes y datos de desarrollo"
    read -p "¿Estás seguro? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker-compose -f docker-compose.dev.yml down -v
        rm -rf dev-data
        docker rmi skypanel:latest 2>/dev/null || true
        print_success "Limpieza completada"
    else
        print_info "Operación cancelada"
    fi
}

# Crear usuario admin
create_admin() {
    print_header "Crear Usuario Administrador"
    
    if ! docker ps | grep -q skypanel-dev; then
        print_error "El contenedor no está corriendo"
        print_info "Ejecuta primero: $0 start"
        exit 1
    fi
    
    read -p "Email del administrador: " admin_email
    read -s -p "Contraseña: " admin_password
    echo
    
    print_info "Creando usuario administrador..."
    
    docker exec -it skypanel-dev /SkyPanel/bin/SkyPanel user add \
        --email "$admin_email" \
        --password "$admin_password" \
        --admin
    
    print_success "Usuario administrador creado"
    print_info "Puedes iniciar sesión en: http://localhost:8080"
}

# Ejecutar comando en el contenedor
exec_command() {
    if ! docker ps | grep -q skypanel-dev; then
        print_error "El contenedor no está corriendo"
        exit 1
    fi
    
    docker exec -it skypanel-dev /SkyPanel/bin/SkyPanel "$@"
}

# Shell interactivo
shell() {
    print_header "Shell Interactivo"
    
    if ! docker ps | grep -q skypanel-dev; then
        print_error "El contenedor no está corriendo"
        exit 1
    fi
    
    docker exec -it skypanel-dev /bin/sh
}

# Mostrar estado
status() {
    print_header "Estado de SkyPanel"
    
    if docker ps | grep -q skypanel-dev; then
        print_success "Contenedor corriendo"
        echo ""
        docker ps --filter "name=skypanel-dev" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        echo ""
        
        # Verificar salud
        print_info "Verificando salud del servicio..."
        if curl -s http://localhost:8080/ > /dev/null 2>&1; then
            print_success "Panel web respondiendo en http://localhost:8080"
        else
            print_warning "Panel web no está respondiendo"
        fi
        
        if curl -s http://localhost:8081/ > /dev/null 2>&1; then
            print_success "Gatus respondiendo en http://localhost:8081"
        else
            print_warning "Gatus no está respondiendo"
        fi
    else
        print_warning "Contenedor no está corriendo"
        print_info "Ejecuta: $0 start"
    fi
}

# Menú de ayuda
show_help() {
    cat << EOF
${BLUE}SkyPanel Docker Test Script${NC}

${YELLOW}Uso:${NC}
    $0 [comando]

${YELLOW}Comandos:${NC}
    ${GREEN}build${NC}          Construir imagen de Docker
    ${GREEN}start${NC}          Iniciar contenedor
    ${GREEN}stop${NC}           Detener contenedor
    ${GREEN}restart${NC}        Reiniciar contenedor
    ${GREEN}logs${NC}           Ver logs en tiempo real
    ${GREEN}status${NC}         Ver estado del contenedor
    ${GREEN}shell${NC}          Abrir shell interactivo
    ${GREEN}admin${NC}          Crear usuario administrador
    ${GREEN}clean${NC}          Limpiar todo (contenedor, volúmenes, datos)
    ${GREEN}rebuild${NC}        Reconstruir e iniciar
    ${GREEN}help${NC}           Mostrar esta ayuda

${YELLOW}Ejemplos:${NC}
    # Construcción y prueba rápida
    $0 build && $0 start
    
    # Ver logs
    $0 logs
    
    # Crear admin
    $0 admin
    
    # Limpiar todo y empezar de nuevo
    $0 clean && $0 build && $0 start

${YELLOW}URLs:${NC}
    Panel Web:  ${GREEN}http://localhost:8080${NC}
    Gatus:      ${GREEN}http://localhost:8081${NC}
    SFTP:       ${GREEN}localhost:5657${NC}

EOF
}

# Comando principal
main() {
    case "${1:-help}" in
        build)
            check_docker
            build_image
            ;;
        start)
            check_docker
            check_docker_compose
            start_container
            ;;
        stop)
            check_docker_compose
            stop_container
            ;;
        restart)
            check_docker_compose
            stop_container
            sleep 2
            start_container
            ;;
        logs)
            check_docker_compose
            show_logs
            ;;
        status)
            check_docker
            status
            ;;
        shell)
            check_docker
            shell
            ;;
        admin)
            check_docker
            create_admin
            ;;
        clean)
            check_docker
            check_docker_compose
            clean_all
            ;;
        rebuild)
            check_docker
            check_docker_compose
            stop_container 2>/dev/null || true
            build_image
            start_container
            ;;
        exec)
            shift
            exec_command "$@"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "Comando desconocido: $1"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

main "$@"
