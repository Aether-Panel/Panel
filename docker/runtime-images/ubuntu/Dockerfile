FROM ubuntu:24.04

ENV DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1 \
    DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl git unzip wget xz-utils zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives && \
    groupadd -r container && \
    useradd -r -g container -m -d /home/container container

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 CMD command -v sh || exit 1

USER container