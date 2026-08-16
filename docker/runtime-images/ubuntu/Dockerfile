FROM ubuntu:24.04

ENV DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1 \
    DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        zip unzip curl wget xz-utils git ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives