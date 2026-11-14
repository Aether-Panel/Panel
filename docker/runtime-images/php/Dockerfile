# PHP for the PHP and Laravel templates: the official image plus what PHP apps and frameworks
# usually need and the official image leaves out, so `composer install` and a database work
# without building anything on the node. RUNTIME_VERSION is the PHP minor version (8.3).
ARG RUNTIME_VERSION=8.3
FROM composer:2 AS composer
FROM php:${RUNTIME_VERSION}-cli

ENV DEBIAN_FRONTEND=noninteractive

# The extensions are built against the -dev packages, which stay: they bring the shared libraries
# the extensions link against, whatever those are called in this Debian release. Only the
# compilers go afterwards: the official image's PHPIZE_DEPS, written out so no unquoted variable
# has to split into words.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        autoconf bash ca-certificates curl dpkg-dev file g++ gcc git libc-dev \
        libfreetype-dev libicu-dev libjpeg62-turbo-dev libonig-dev libpng-dev libpq-dev \
        libzip-dev make pkg-config re2c unzip zip; \
    docker-php-ext-configure gd --with-freetype --with-jpeg; \
    docker-php-ext-install -j"$(nproc)" \
        bcmath exif gd intl mysqli opcache pcntl \
        pdo_mysql pdo_pgsql pgsql sockets zip; \
    pecl install redis; \
    docker-php-ext-enable redis; \
    apt-get purge -y --auto-remove \
        autoconf dpkg-dev file g++ gcc libc-dev make pkg-config re2c; \
    rm -rf /var/lib/apt/lists/* /tmp/pear ~/.pearrc; \
    cp "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"; \
    for ext in bcmath gd intl mysqli pdo_mysql pdo_pgsql redis zip; do \
        php -m | grep -qix "$ext"; \
    done; \
    groupadd -r container; \
    useradd -r -g container -m -d /home/container container

COPY --from=composer /usr/bin/composer /usr/bin/composer

WORKDIR /home/container

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 CMD command -v php || exit 1

USER container
