# Guía de Instalación

Esta guía detalla los pasos necesarios para instalar Aether Panel en su servidor. Soportamos múltiples métodos de instalación para adaptarse a diferentes entornos y necesidades.

## Requisitos del Sistema

Antes de comenzar, asegúrese de que su servidor cumpla con los siguientes requisitos mínimos:

*   **Sistema Operativo:** Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+)
*   **Arquitectura:** amd64 o arm64
*   **Recursos (Mínimos):** 2 vCPU, 2GB RAM
*   **Puertos:** 8080 (Web), 5657 (SFTP) abiertos en el firewall

---

## Método 1: Instalación Automática (Recomendado)

Nuestro script de instalación automático maneja la configuración de usuarios, la instalación de dependencias y la configuración del servicio systemd.

Ejecute el siguiente comando como usuario root o con privilegios sudo:

```bash
bash <(curl -s https://install.aetherpanel.com/install.sh)
```

El instalador le guiará a través del proceso:
1.  Verificación de dependencias.
2.  Instalación de Docker (si no está presente).
3.  Descarga del binario de SkyPanel.
4.  Creación del usuario administrador inicial.
5.  Inicio del servicio SkyPanel.

---

## Método 2: Instalación Manual

Si prefiere tener control total sobre el proceso de instalación o su distribución no es compatible con el script automático, siga estos pasos.

### 1. Preparar el Entorno

Cree el usuario del sistema y los directorios necesarios:

```bash
# Crear usuario sin privilegios
useradd -r -m -d /var/lib/SkyPanel -s /bin/false skypanel

# Crear estructura de directorios
mkdir -p /var/lib/SkyPanel
mkdir -p /etc/SkyPanel
mkdir -p /var/log/SkyPanel
```

### 2. Descargar e Instalar

Descargue la última versión estable desde nuestra página de lanzamientos:

```bash
# Ejemplo para Linux AMD64
wget https://github.com/aetherpanel/aetherpanel/releases/latest/download/SkyPanel_linux_amd64 -O /usr/local/bin/SkyPanel
chmod +x /usr/local/bin/SkyPanel
```

### 3. Configuración Inicial

Habilite el servicio y cree el primer usuario administrador:

```bash
# Añadir usuario administrador
/usr/local/bin/SkyPanel user add --admin

# Habilitar servicio (si usa systemd)
/usr/local/bin/SkyPanel runservice
```

---

## Método 3: Instalación vía Docker

Para ejecutar Aether Panel en un contenedor aislado, utilice Docker.

### Ejecución Directa

```bash
docker run -d \
  --name skypanel \
  -p 8080:8080 \
  -p 5657:5657 \
  -v skypanel-config:/etc/SkyPanel \
  -v skypanel-data:/var/lib/SkyPanel \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --restart=always \
  aetherpanel/skypanel:latest
```

### Docker Compose

Guarde el siguiente contenido en un archivo `docker-compose.yml`:

```yaml
version: '3'
services:
  skypanel:
    image: aetherpanel/skypanel:latest
    ports:
      - "8080:8080"
      - "5657:5657"
    volumes:
      - ./config:/etc/SkyPanel
      - ./data:/var/lib/SkyPanel
      - /var/run/docker.sock:/var/run/docker.sock
    restart: always
```

Inicie el servicio con:
```bash
docker-compose up -d
```

### Crear Usuario Administrador (Docker)

Una vez que el contenedor esté corriendo, ejecute este comando para crear su cuenta de administrador:

```bash
docker exec -it skypanel /usr/local/bin/SkyPanel user add --name admin --email admin@example.com --password 'admin123' --admin
```

> **Importante:** Recuerde cambiar `admin@example.com` y `'admin123'` por sus datos reales.

---

## Método 4: Instalación como Nodo Secundario (Esclavo)

Si desea configurar un servidor adicional que funcione únicamente como daemon (nodo esclavo) para alojar servidores de juegos, conectándose a un Panel Maestro existente:

### Usando Docker Compose

Añada las siguientes variables de entorno a la configuración del servicio `skypanel` en su `docker-compose.yml`:

```yaml
    environment:
      # Apagar la interfaz del panel web y la base de datos
      - PUFFER_PANEL_ENABLE=false
      # Configurar la llave pública del Panel Maestro para validación de tokens
      # Asegúrese de cambiar <IP-DEL-PANEL-MAESTRO> por la IP o dominio real
      - PUFFER_TOKEN_PUBLIC=http://<IP-DEL-PANEL-MAESTRO>:8080/auth/publickey
```

### Sin Docker (Instalación Manual)

Si instaló el nodo manualmente, edite el archivo `/etc/SkyPanel/config.json` y modifique la sección `panel` y agregue `token`:

```json
  "panel": {
    "enable": false
  },
  "token": {
    "public": "http://<IP-DEL-PANEL-MAESTRO>:8080/auth/publickey"
  }
```

Reinicie el servicio con `systemctl restart skypanel`.

---

## Post-Instalación

Una vez instalado el panel principal, estará accesible en:
`http://<IP-DE-SU-SERVIDOR>:8080`

Inicie sesión con las credenciales de administrador creadas durante el proceso de instalación.
