# Guía de Instalación

Esta guía detalla los pasos necesarios para instalar Aether Panel en su servidor. Soportamos múltiples métodos de instalación para adaptarse a diferentes entornos y necesidades.

> **Nota:** La instalación es **solo vía Docker**. No existe un modo nativo (binarios sueltos). El script de instalación y el `docker-compose.yml` del repositorio construyen la imagen desde el `Dockerfile`.

## Requisitos del Sistema

Antes de comenzar, asegúrese de que su servidor cumpla con los siguientes requisitos mínimos:

*   **Sistema Operativo:** Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+)
*   **Arquitectura:** amd64 o arm64
*   **Recursos (Mínimos):** 2 vCPU, 2GB RAM
*   **Docker:** Docker Engine + Docker Compose v2 (el instalador lo instala si falta)
*   **Puertos:** 8080 (Web), 5657 (SFTP) abiertos en el firewall

---

## Método 1: Instalación Automática (Recomendado)

Nuestro script de instalación automático maneja la instalación de Docker, la descarga del repositorio, la construcción de la imagen y la configuración del servicio.

Ejecute el siguiente comando como usuario root o con privilegios sudo:

```bash
bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"
```

Alternativas (evitan el process substitution):

```bash
# Modo automático (sin preguntas)
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash

# Descargar y ejecutar
curl -fsSL https://install.aetherpanel.es/install.sh -o /tmp/install.sh
sudo bash /tmp/install.sh
```

El instalador le guiará a través del proceso:
1.  Verificación del sistema: arquitectura, RAM (2 GB o swap), espacio, puertos libres.
2.  Instalación de Docker y Docker Compose (si no están presentes), con la clave GPG de Docker verificada.
3.  Clonado del repositorio desde `https://github.com/Aether-Panel/Panel.git` (rama `dev2.0`, o la ya instalada).
4.  Contraseñas aleatorias para la base de datos (en `.env`, solo legible por root).
5.  Construcción de la imagen y configuración de puertos, dominio, SSL y cortafuegos (opcional).
6.  URL del panel (para los enlaces de cambio de contraseña y de acceso directo) y cuenta de administrador.
7.  Inicio de Aether Panel y un resumen con la dirección y dónde quedan las credenciales (`/etc/SkyPanel/credenciales.txt`).

Volver a ejecutarlo sobre una instalación existente es seguro: conserva los datos, las contraseñas y la cuenta de administrador.

### Opciones útiles

```bash
# Ver todas las opciones
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --help

# Instalación desatendida con dominio y SSL
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --non-interactive \
  --domain panel.ejemplo.com --ssl --email admin@ejemplo.com --admin-email admin@ejemplo.com

# Ver qué haría, sin cambiar nada
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --dry-run

# Actualizar o desinstalar (conservando los datos)
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --update
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --uninstall
```

También puede dejar configurados Turnstile (`--turnstile-site-key`, `--turnstile-secret-key`), los subdominios automáticos de Cloudflare (`--cf-dns-token`, `--cf-zone-id`, `--dns-base-domain`), las copias en S3 (`--s3-*`) y la clave de CurseForge; todo se puede cambiar después en Ajustes. Cambiar los puertos (`--port`, `--sftp-port`) necesita Docker Compose 2.24.4 o superior. El código del instalador está en `tools/install/install.sh`.

---

## Método 2: Instalación Manual vía Docker

Si prefiere tener control total sobre el proceso, clone el repositorio y use el `docker-compose.yml` incluido.

### 1. Clonar el Repositorio

```bash
git clone -b dev2.0 https://github.com/Aether-Panel/Panel.git /opt/skypanel
cd /opt/skypanel
```

### 2. Configurar el Entorno

El `docker-compose.yml` usa variables con defaults (opcional definir `.env`):

| Variable | Default | Uso |
|---|---|---|
| `DB_ROOT_PASSWORD` | `skypanel_secret` | Password root de MariaDB |
| `DB_DATABASE` | `skypanel` | Nombre de la base de datos |
| `DB_USER` | `skypanel` | Usuario de la base de datos |
| `DB_PASSWORD` | `skypanel_secret` | Password del usuario |

### 3. Construir e Iniciar

```bash
docker compose up -d --build
```

### 4. Crear Usuario Administrador

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --password 'admin123' --admin
```

> **Importante:** Recuerde cambiar `admin@example.com` y `'admin123'` por sus datos reales.

---

## Método 3: Instalación como Nodo Secundario (Esclavo)

Si desea configurar un servidor adicional que funcione únicamente como daemon (nodo esclavo) para alojar servidores de juegos, conectándose a un Panel Maestro existente:

### Usando Docker Compose

Añada las siguientes variables de entorno a la configuración del servicio `skypanel` en su `docker-compose.yml`:

```yaml
    environment:
      # Apagar la interfaz del panel web y la base de datos
      - SKYPANEL_PANEL_ENABLE=false
      # Configurar la llave pública del Panel Maestro para validación de tokens
      # Asegúrese de cambiar <IP-DEL-PANEL-MAESTRO> por la IP o dominio real
      - SKYPANEL_TOKEN_PUBLIC=http://<IP-DEL-PANEL-MAESTRO>:8080/auth/publickey
```

### Sin Docker (Instalación Manual)

> **No existe modo nativo.** Si instaló el nodo con el instalador automático, la configuración equivalente se aplica vía variables de entorno. En un despliegue manual sin compose, edite el archivo `/etc/SkyPanel/config.json` dentro del contenedor y modifique la sección `panel` y `token`:

```json
  "panel": {
    "enable": false
  },
  "token": {
    "public": "http://<IP-DEL-PANEL-MAESTRO>:8080/auth/publickey"
  }
```

Reinicie el contenedor con `docker restart skypanel`.

> **Nota Aclaratoria:** Aunque desactive el Panel, el proceso seguirá escuchando en el puerto HTTP (por defecto `8080`) ya que el Daemon necesita este puerto para su API REST (comunicación con el Maestro). Si intenta acceder por el navegador, verá un error 404, esto es completamente normal e indica que el Nodo está funcionando correctamente.

---

## Post-Instalación

Una vez instalado el panel principal, estará accesible en:
`http://<IP-DE-SU-SERVIDOR>:8080`

Inicie sesión con las credenciales de administrador creadas durante el proceso de instalación.
