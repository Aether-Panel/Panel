# PHP for the PHP and Laravel templates: the official image plus what PHP apps and frameworks
# usually need and the official image leaves out, so `composer install` and a database work
# without building anything on the node. RUNTIME_VERSION is the PHP minor version (8.3).
ARG RUNTIME_VERSION=8.3
FROM composer:2 AS composer
FROM php:${RUNTIME_VERSION}-cli

ENV DEBIAN_FRONTEND=noninteractive

# The extensions are built against the -dev packages, which stay: they bring the shared libraries
# the extensions link against, whatever those are called in this Debian release. Only the
# compilers go afterwards.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        bash ca-certificates curl git unzip zip \
        libfreetype-dev libjpeg62-turbo-dev libpng-dev libzip-dev libpq-dev libicu-dev libonig-dev \
        $PHPIZE_DEPS; \
    docker-php-ext-configure gd --with-freetype --with-jpeg; \
    docker-php-ext-install -j"$(nproc)" bcmath exif gd intl mysqli opcache pcntl pdo_mysql pdo_pgsql pgsql sockets zip; \
    pecl install redis; \
    docker-php-ext-enable redis; \
    apt-get purge -y --auto-remove $PHPIZE_DEPS; \
    rm -rf /var/lib/apt/lists/* /tmp/pear ~/.pearrc; \
    cp "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"; \
    for ext in bcmath gd intl mysqli pdo_mysql pdo_pgsql redis zip; do php -m | grep -qix "$ext"; done; \
    groupadd -r container; \
    useradd -r -g container -m -d /home/container container

COPY --from=composer /usr/bin/composer /usr/bin/composer

WORKDIR /home/container

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 CMD command -v php || exit 1

USER container
