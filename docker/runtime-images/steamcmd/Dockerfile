FROM steamcmd/steamcmd:ubuntu-22

ENV DEBIAN_FRONTEND=noninteractive

RUN dpkg --add-architecture i386 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl libcurl3-gnutls libcurl3-gnutls:i386 libncurses5:i386 \
        libtinfo5:i386 libxi6 unzip wget xvfb zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives && \
    useradd -m -d /home/container container

WORKDIR /home/container

ENV HOME=/home/container \
    USER=container

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 CMD command -v sh || exit 1

USER container