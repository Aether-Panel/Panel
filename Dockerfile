###
# Builder container
###
FROM --platform=$BUILDPLATFORM node:22-alpine AS node

WORKDIR /build
COPY client .

RUN rm -rf /build/*/node_modules/ && \
    rm -rf /build/*/dist/

RUN yarn install && \
    yarn build

FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

RUN apk add clang lld
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
COPY gatus ./gatus
RUN go mod download && go mod verify

COPY . .

# Instalar swag via go install para asegurar compatibilidad
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.4

# Ejecutar swag init desde el GOPATH/bin
RUN /go/bin/swag init -o web/swagger -g web/loader.go

COPY --from=node /build/frontend/dist /build/SkyPanel/client/frontend/dist

ARG TARGETPLATFORM
ARG curseforgeKey=''

RUN xx-apk add musl-dev gcc
RUN xx-go build -buildvcs=false -tags "$tags" -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3/config.curseforgeKey=$curseforgeKey' -X 'github.com/SkyPanel/SkyPanel/v3.Hash=$sha' -X 'github.com/SkyPanel/SkyPanel/v3.Version=$version'" -o /SkyPanel/SkyPanel github.com/SkyPanel/SkyPanel/v3/cmd
# RUN go test ./...
RUN xx-verify /SkyPanel/SkyPanel

###
# Generate final image
###

FROM alpine

EXPOSE 8080 5657
RUN mkdir -p /etc/SkyPanel && \
    mkdir -p /var/lib/SkyPanel /var/lib/SkyPanel/servers /var/lib/SkyPanel/binaries /var/lib/SkyPanel/cache && \
    mkdir -p /var/log/SkyPanel
#addgroup --system -g 1000 SkyPanel && \
#adduser -D -H --home /var/lib/SkyPanel --ingroup SkyPanel -u 1000 SkyPanel && \
#chown -R SkyPanel:SkyPanel /etc/SkyPanel /var/lib/SkyPanel /var/log/SkyPanel

ENV GIN_MODE=release \
    PUFFER_PLATFORM="docker" \
    PUFFER_DOCKER_ROOT="" \
    PUFFER_DOCKER_DISALLOWHOST=true

#COPY --from=builder --chown=SkyPanel:SkyPanel --chmod=755 /SkyPanel /SkyPanel/bin
#COPY --from=builder --chown=SkyPanel:SkyPanel --chmod=755 /build/SkyPanel/entrypoint.sh /SkyPanel/bin/entrypoint.sh
#COPY --from=builder --chown=SkyPanel:SkyPanel --chmod=755 /build/SkyPanel/config.docker.json /etc/SkyPanel/config.json
COPY --from=builder --chmod=755 /SkyPanel/SkyPanel /SkyPanel/bin/SkyPanel
COPY --from=builder --chmod=755 /build/SkyPanel/entrypoint.sh /SkyPanel/bin/entrypoint.sh
COPY --from=builder --chmod=755 /build/SkyPanel/config.docker.json /etc/SkyPanel/config.json
COPY --from=builder --chmod=755 /build/SkyPanel/client/frontend/dist /var/www/SkyPanel

VOLUME /etc/SkyPanel
VOLUME /var/lib/SkyPanel
VOLUME /var/log/SkyPanel
VOLUME /var/www/SkyPanel

WORKDIR /var/lib/SkyPanel

#USER SkyPanel

ENTRYPOINT ["sh", "/SkyPanel/bin/entrypoint.sh"]
