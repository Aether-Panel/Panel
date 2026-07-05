###
# Builder container
###
ARG BUILDPLATFORM=linux/amd64
FROM --platform=${BUILDPLATFORM} node:22-alpine AS node

WORKDIR /build
# Optimización: Copiar archivos de dependencia (incluyendo workspaces) para cachear capas
COPY client/package.json client/yarn.lock* ./
COPY client/frontend/package.json ./frontend/
RUN yarn install --frozen-lockfile

# Copiar el resto del código
COPY client/ .
RUN rm -rf node_modules/.cache

RUN yarn build

ARG BUILDPLATFORM=linux/amd64
FROM --platform=${BUILDPLATFORM} tonistiigi/xx:1.9.0 AS xx

ARG BUILDPLATFORM=linux/amd64
FROM --platform=${BUILDPLATFORM} golang:1.25-alpine AS builder

RUN apk add --no-cache clang lld
COPY --from=xx / /

ARG tags
ARG version=devel
ARG sha=devel
ARG swagversion=1.16.4
ARG swagarch=x86_64

ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

RUN mkdir /SkyPanel

WORKDIR /build/SkyPanel

COPY go.mod go.sum ./

RUN go mod download && go mod verify

# Optimización: Instalar swag antes de copiar todo el código para cachear la descarga
RUN CGO_ENABLED=0 go install github.com/swaggo/swag/cmd/swag@v1.16.4

COPY . .

# Ejecutar swag init desde el GOPATH/bin
RUN /go/bin/swag init -o internal/web/swagger -g internal/web/loader.go

COPY --from=node /build/frontend/dist /build/SkyPanel/client/frontend/dist

ARG TARGETPLATFORM=linux/amd64
ARG curseforgeKey=''

RUN xx-apk add musl-dev gcc
RUN xx-go build -buildvcs=false -tags "$tags" -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3/internal/config.curseforgeKey=$curseforgeKey' -X 'github.com/SkyPanel/SkyPanel/v3.Hash=$sha' -X 'github.com/SkyPanel/SkyPanel/v3.Version=$version'" -o /SkyPanel/SkyPanel github.com/SkyPanel/SkyPanel/v3/cmd/panel
# RUN go test ./...
RUN xx-verify /SkyPanel/SkyPanel

###
# Generate final image
###

FROM alpine:3.21

EXPOSE 8080 5657
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 CMD nc -z localhost 8080 || exit 1
RUN apk add --no-cache netcat-openbsd
RUN mkdir -p /etc/SkyPanel && \
    mkdir -p /var/lib/SkyPanel /var/lib/SkyPanel/servers /var/lib/SkyPanel/binaries /var/lib/SkyPanel/cache && \
    mkdir -p /var/log/SkyPanel
RUN addgroup -S -g 1000 SkyPanel && \
    adduser -D -H -h /var/lib/SkyPanel -G SkyPanel -u 1000 SkyPanel && \
    chown -R SkyPanel:SkyPanel /etc/SkyPanel /var/lib/SkyPanel /var/log/SkyPanel

ENV GIN_MODE=release \
    PUFFER_PLATFORM="docker" \
    PUFFER_DOCKER_ROOT="" \
    PUFFER_DOCKER_DISALLOWHOST=true

#COPY --from=builder --chown=SkyPanel:SkyPanel /SkyPanel /SkyPanel/bin
#COPY --from=builder --chown=SkyPanel:SkyPanel /build/SkyPanel/entrypoint.sh /SkyPanel/bin/entrypoint.sh
#COPY --from=builder --chown=SkyPanel:SkyPanel /build/SkyPanel/config.docker.json /etc/SkyPanel/config.json
COPY --from=builder /SkyPanel/SkyPanel /SkyPanel/bin/SkyPanel
RUN chmod 755 /SkyPanel/bin/SkyPanel
RUN cat <<'EOF' > /SkyPanel/bin/entrypoint.sh
#!/usr/bin/env sh

echo "=== Iniciando Aether Panel ==="
echo "Fecha: $(date)"
echo "Variables de entorno:"
echo "  PUFFER_PLATFORM=${PUFFER_PLATFORM}"
echo "  PUFFER_WEB_HOST=${PUFFER_WEB_HOST}"
echo "  PUFFER_PANEL_DATABASE_DIALECT=${PUFFER_PANEL_DATABASE_DIALECT}"
echo "  PUFFER_PANEL_DATABASE_URL=${PUFFER_PANEL_DATABASE_URL}"

# Verificar que el binario existe
if [ ! -f /SkyPanel/bin/SkyPanel ]; then
    echo "ERROR: El binario /SkyPanel/bin/SkyPanel no existe"
    exit 1
fi

# Verificar permisos del binario
ls -la /SkyPanel/bin/SkyPanel

# Esperar a que MySQL esté disponible (máximo 60 segundos)
echo "Esperando a que MySQL esté disponible..."
DB_HOST=$(echo "$PUFFER_PANEL_DATABASE_URL" | sed -n 's/.*@tcp(\([^:]*\):.*/\1/p')
DB_PORT=$(echo "$PUFFER_PANEL_DATABASE_URL" | sed -n 's/.*@tcp([^:]*:\([0-9]*\)).*/\1/p')
if [ -z "$DB_HOST" ]; then
    DB_HOST="mysql"
fi
if [ -z "$DB_PORT" ]; then
    DB_PORT="3306"
fi

MAX_ATTEMPTS=60
ATTEMPT=0
while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    if nc -z "$DB_HOST" "$DB_PORT" 2>/dev/null; then
        echo "MySQL está disponible en $DB_HOST:$DB_PORT"
        break
    fi
    ATTEMPT=$((ATTEMPT + 1))
    echo "Intento $ATTEMPT/$MAX_ATTEMPTS: MySQL no disponible aún, esperando..."
    sleep 1
done

if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
    echo "ERROR: MySQL no está disponible después de $MAX_ATTEMPTS intentos"
    exit 1
fi

# Ejecutar migraciones de base de datos
echo "Ejecutando migraciones de base de datos..."
/SkyPanel/bin/SkyPanel db migrate
exitCode=$?
if [ $exitCode -eq 0 ] || [ $exitCode -eq 9 ]; then
    echo "Migraciones completadas (código: $exitCode)"
else
    echo "ERROR: Fallo en migraciones (código: $exitCode)"
    echo "Intentando continuar de todas formas..."
fi

# Iniciar el panel (debe quedarse corriendo)
echo "Iniciando panel..."
echo "Ejecutando: /SkyPanel/bin/SkyPanel run"
exec /SkyPanel/bin/SkyPanel run
EOF
RUN chmod 755 /SkyPanel/bin/entrypoint.sh && sed -i 's/\r$//' /SkyPanel/bin/entrypoint.sh
COPY --from=builder /build/SkyPanel/config.docker.json /etc/SkyPanel/config.json
RUN chmod 644 /etc/SkyPanel/config.json
COPY --from=builder /build/SkyPanel/client/frontend/dist /var/www/SkyPanel
RUN chown -R SkyPanel:SkyPanel /SkyPanel /var/www/SkyPanel /etc/SkyPanel


VOLUME /etc/SkyPanel
VOLUME /var/lib/SkyPanel
VOLUME /var/log/SkyPanel
VOLUME /var/www/SkyPanel

WORKDIR /var/lib/SkyPanel

USER SkyPanel

ENTRYPOINT ["sh", "/SkyPanel/bin/entrypoint.sh"]
