FROM cm2network/steamcmd:ubuntu-20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN dpkg --add-architecture i386 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        libtinfo5:i386 libncurses5:i386 libcurl3-gnutls:i386 \
        zip unzip curl wget ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives