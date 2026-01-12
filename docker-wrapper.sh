#!/bin/bash

###############################################################################
# Script Wrapper para Docker con Detección Automática de Sudo
###############################################################################

# Detectar si necesitamos sudo
DOCKER_CMD="docker"
COMPOSE_CMD="docker-compose"

if ! docker info &> /dev/null 2>&1; then
    if sudo docker info &> /dev/null 2>&1; then
        DOCKER_CMD="sudo docker"
        if command -v docker-compose &> /dev/null; then
            COMPOSE_CMD="sudo docker-compose"
        else
            COMPOSE_CMD="sudo docker compose"
        fi
    fi
fi

# Exportar para que otros scripts puedan usarlo
export DOCKER_CMD
export COMPOSE_CMD

# Ejecutar el comando pasado como argumento
"$@"
