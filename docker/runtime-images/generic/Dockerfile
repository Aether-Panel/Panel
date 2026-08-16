FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl wget zip unzip xz-utils git bash && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives

WORKDIR /home/container