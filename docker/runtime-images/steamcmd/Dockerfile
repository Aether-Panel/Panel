FROM steamcmd/steamcmd:ubuntu-20

ENV DEBIAN_FRONTEND=noninteractive

RUN dpkg --add-architecture i386 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        libtinfo5:i386 libncurses5:i386 libcurl3-gnutls:i386 \
        zip unzip curl wget ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives

RUN useradd -m -d /home/container container

WORKDIR /home/container

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 CMD command -v sh || exit 1

USER container