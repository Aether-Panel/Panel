# 📦 Instalación de SkyPanel

## Tabla de Contenidos

- [Requisitos del Sistema](#requisitos-del-sistema)
- [Instalación Automática](#instalación-automática)
- [Instalación Manual](#instalación-manual)
- [Instalación con Docker](#instalación-con-docker)
- [Verificación de la Instalación](#verificación-de-la-instalación)
- [Configuración Inicial](#configuración-inicial)
- [Solución de Problemas](#solución-de-problemas)

---

## Requisitos del Sistema

### Requisitos Mínimos

| Componente | Requisito |
|------------|-----------|
| **Sistema Operativo** | Ubuntu 20.04+, Debian 10+, CentOS 8+, RHEL 8+, Fedora 35+ |
| **RAM** | 2 GB mínimo (4 GB recomendado) |
| **Almacenamiento** | 10 GB mínimo (20 GB recomendado) |
| **CPU** | 2 núcleos mínimo |
| **Acceso** | Root o sudo |
| **Red** | Conexión a Internet estable |

### Requisitos de Software

SkyPanel requiere las siguientes dependencias (instaladas automáticamente por el script):

- **Go** 1.21 o superior
- **Node.js** 18 o superior
- **Yarn** (gestor de paquetes)
- **Git** para clonar el repositorio
- **SQLite3** (o MySQL/PostgreSQL si se prefiere)
- **Build tools** (gcc, make, etc.)

### Puertos Requeridos

| Puerto | Protocolo | Uso | Obligatorio |
|--------|-----------|-----|-------------|
| `8080` | TCP | Panel Web (HTTP/HTTPS) | ✅ Sí |
| `5657` | TCP | SFTP (transferencia de archivos) | ✅ Sí |
| `8081` | TCP | Gatus (monitoreo) | ⚠️ Opcional |

> **⚠️ Importante**: Asegúrate de que estos puertos estén abiertos en tu firewall.

---

## Instalación Automática

La forma más rápida y recomendada de instalar SkyPanel es usando el script de instalación automática.

### Método 1: Instalación desde URL

```bash
bash <(curl -s https://tu-servidor.com/install.sh)
```

O usando una IP específica:

```bash
bash <(curl -s http://192.168.0.5:8080/install.sh)
```

### Método 2: Instalación Local

Si ya tienes el script descargado:

```bash
# Descargar el script
wget https://raw.githubusercontent.com/SkyPanel/SkyPanel/master/install.sh

# Dar permisos de ejecución
chmod +x install.sh

# Ejecutar como root
sudo bash install.sh
```

### Personalización con Variables de Entorno

Puedes personalizar la instalación usando variables de entorno:

```bash
# Cambiar el repositorio Git
export SKYPANEL_REPO_URL="https://github.com/tu-usuario/SkyPanel.git"

# Cambiar la rama
export SKYPANEL_BRANCH="develop"

# Cambiar el puerto del panel
export SKYPANEL_PORT="9090"

# Cambiar el directorio de instalación
export SKYPANEL_INSTALL_DIR="/opt/skypanel"

# Ejecutar instalación
sudo bash install.sh
```

### Proceso de Instalación Automática

El script realizará los siguientes pasos:

1. ✅ **Detección del Sistema**: Identifica tu distribución Linux
2. ✅ **Instalación de Dependencias**: Instala Git, build-essential, SQLite, etc.
3. ✅ **Instalación de Go**: Descarga e instala Go 1.24+
4. ✅ **Instalación de Node.js y Yarn**: Configura Node.js 22+ y Yarn
5. ✅ **Creación de Usuario**: Crea el usuario del sistema `skypanel`
6. ✅ **Clonación del Repositorio**: Descarga el código fuente
7. ✅ **Compilación del Frontend**: Construye la aplicación Vue.js
8. ✅ **Compilación del Backend**: Construye el binario de Go
9. ✅ **Configuración de Directorios**: Crea la estructura de carpetas
10. ✅ **Generación de Configuración**: Crea `config.json` automáticamente
11. ✅ **Configuración de Systemd**: Registra el servicio del sistema
12. ✅ **Inicio del Servicio**: Inicia SkyPanel automáticamente

### Estructura de Directorios Creada

```
/etc/skypanel/          # Archivos de configuración
├── config.json         # Configuración principal
└── templates/          # Plantillas de servidores

/var/lib/skypanel/      # Datos del panel
├── database.db         # Base de datos SQLite
├── servers/            # Servidores de juegos
├── backups/            # Respaldos
└── cache/              # Archivos temporales

/var/log/skypanel/      # Archivos de registro
├── skypanel.log        # Log principal
└── error.log           # Log de errores

/var/www/skypanel/      # Archivos web del frontend
├── index.html
├── assets/
└── ...

/opt/skypanel/          # Código fuente y binario
├── skypanel            # Binario ejecutable
├── client/             # Frontend Vue.js
└── ...
```

---

## Instalación Manual

Si prefieres tener control total sobre el proceso, puedes instalar SkyPanel manualmente.

### Paso 1: Instalar Dependencias

#### Ubuntu/Debian

```bash
# Actualizar repositorios
sudo apt update && sudo apt upgrade -y

# Instalar dependencias básicas
sudo apt install -y git build-essential curl wget sqlite3

# Instalar Go 1.24
wget https://go.dev/dl/go1.24.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verificar instalación de Go
go version

# Instalar Node.js 22 y Yarn
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs
sudo npm install -g yarn

# Verificar instalaciones
node --version
yarn --version
```

#### CentOS/RHEL/Fedora

```bash
# Actualizar sistema
sudo dnf update -y

# Instalar dependencias básicas
sudo dnf install -y git gcc make curl wget sqlite

# Instalar Go 1.24
wget https://go.dev/dl/go1.24.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Instalar Node.js 22 y Yarn
curl -fsSL https://rpm.nodesource.com/setup_22.x | sudo bash -
sudo dnf install -y nodejs
sudo npm install -g yarn
```

### Paso 2: Crear Usuario del Sistema

```bash
# Crear usuario skypanel
sudo useradd -r -m -d /var/lib/skypanel -s /bin/bash skypanel

# Crear directorios necesarios
sudo mkdir -p /etc/skypanel
sudo mkdir -p /var/lib/skypanel/{servers,backups,cache,binaries}
sudo mkdir -p /var/log/skypanel
sudo mkdir -p /var/www/skypanel

# Asignar permisos
sudo chown -R skypanel:skypanel /var/lib/skypanel
sudo chown -R skypanel:skypanel /var/log/skypanel
sudo chown -R skypanel:skypanel /var/www/skypanel
```

### Paso 3: Clonar el Repositorio

```bash
# Clonar en /opt/skypanel
sudo git clone https://github.com/SkyPanel/SkyPanel.git /opt/skypanel
cd /opt/skypanel

# Cambiar propietario
sudo chown -R skypanel:skypanel /opt/skypanel
```

### Paso 4: Compilar el Frontend

```bash
# Cambiar al directorio del frontend
cd /opt/skypanel/client/frontend

# Instalar dependencias
yarn install

# Compilar para producción
yarn build

# Copiar archivos compilados al directorio web
sudo cp -r dist/* /var/www/skypanel/
```

### Paso 5: Compilar el Backend

```bash
# Volver al directorio raíz
cd /opt/skypanel

# Descargar dependencias de Go
go mod download
go mod verify

# Compilar el binario
go build -tags "" \
  -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3.Hash=manual' -X 'github.com/SkyPanel/SkyPanel/v3.Version=3.0.0'" \
  -o skypanel ./cmd

# Copiar binario a /usr/local/bin
sudo cp skypanel /usr/local/bin/skypanel
sudo chmod +x /usr/local/bin/skypanel
```

### Paso 6: Crear Archivo de Configuración

```bash
# Crear config.json
sudo tee /etc/skypanel/config.json > /dev/null <<EOF
{
  "logs": "/var/log/skypanel",
  "panel": {
    "database": {
      "dialect": "sqlite3",
      "url": "file:/var/lib/skypanel/database.db?cache=shared"
    },
    "web": {
      "files": "/var/www/skypanel"
    },
    "gatus": {
      "enable": true,
      "port": 8081
    },
    "settings": {
      "companyname": "SkyPanel",
      "defaulttheme": "SkyPanel",
      "masterurl": "http://$(curl -s ifconfig.me):8080"
    }
  },
  "daemon": {
    "data": {
      "root": "/var/lib/skypanel"
    },
    "sftp": {
      "host": "0.0.0.0:5657"
    }
  },
  "web": {
    "host": "0.0.0.0:8080"
  }
}
EOF

# Asignar permisos
sudo chown skypanel:skypanel /etc/skypanel/config.json
sudo chmod 600 /etc/skypanel/config.json
```

### Paso 7: Crear Servicio Systemd

```bash
# Crear archivo de servicio
sudo tee /etc/systemd/system/skypanel.service > /dev/null <<EOF
[Unit]
Description=SkyPanel Game Server Management Panel
After=network.target

[Service]
Type=simple
User=skypanel
Group=skypanel
WorkingDirectory=/var/lib/skypanel
ExecStart=/usr/local/bin/skypanel run --config /etc/skypanel/config.json
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=skypanel

# Límites de recursos
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

# Recargar systemd
sudo systemctl daemon-reload

# Habilitar inicio automático
sudo systemctl enable skypanel

# Iniciar servicio
sudo systemctl start skypanel
```

---

## Instalación con Docker

SkyPanel también puede ejecutarse en un contenedor Docker.

### Método 1: Docker Compose (Recomendado)

Crea un archivo `docker-compose.yml`:

```yaml
version: '3.8'

services:
  skypanel:
    image: skypanel/skypanel:latest
    container_name: skypanel
    restart: unless-stopped
    ports:
      - "8080:8080"   # Panel Web
      - "5657:5657"   # SFTP
      - "8081:8081"   # Gatus (opcional)
    volumes:
      - ./config:/etc/skypanel
      - ./data:/var/lib/skypanel
      - ./logs:/var/log/skypanel
      - /var/run/docker.sock:/var/run/docker.sock  # Para gestión de contenedores
    environment:
      - SKYPANEL_WEB_HOST=0.0.0.0:8080
      - SKYPANEL_DATABASE_DIALECT=sqlite3
      - SKYPANEL_DATABASE_URL=file:/var/lib/skypanel/database.db?cache=shared
    networks:
      - skypanel-network

networks:
  skypanel-network:
    driver: bridge
```

Ejecutar:

```bash
# Crear directorios
mkdir -p config data logs

# Iniciar contenedor
docker-compose up -d

# Ver logs
docker-compose logs -f
```

### Método 2: Docker Run

```bash
# Crear volúmenes
docker volume create skypanel-config
docker volume create skypanel-data
docker volume create skypanel-logs

# Ejecutar contenedor
docker run -d \
  --name skypanel \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 5657:5657 \
  -p 8081:8081 \
  -v skypanel-config:/etc/skypanel \
  -v skypanel-data:/var/lib/skypanel \
  -v skypanel-logs:/var/log/skypanel \
  -v /var/run/docker.sock:/var/run/docker.sock \
  skypanel/skypanel:latest
```

### Construir Imagen Personalizada

```bash
# Clonar repositorio
git clone https://github.com/SkyPanel/SkyPanel.git
cd SkyPanel

# Construir imagen
docker build -t skypanel:custom .

# Ejecutar
docker run -d \
  --name skypanel \
  -p 8080:8080 \
  -p 5657:5657 \
  skypanel:custom
```

---

## Verificación de la Instalación

### Verificar Estado del Servicio

```bash
# Ver estado del servicio
sudo systemctl status skypanel

# Debería mostrar: Active: active (running)
```

### Verificar Logs

```bash
# Ver logs en tiempo real
sudo journalctl -u skypanel -f

# Ver últimas 50 líneas
sudo journalctl -u skypanel -n 50

# Ver logs del día
sudo journalctl -u skypanel --since today
```

### Verificar Puertos

```bash
# Verificar que los puertos estén escuchando
sudo netstat -tulpn | grep -E '8080|5657|8081'

# O usando ss
sudo ss -tulpn | grep -E '8080|5657|8081'

# Salida esperada:
# tcp   LISTEN   0.0.0.0:8080   (skypanel)
# tcp   LISTEN   0.0.0.0:5657   (skypanel)
# tcp   LISTEN   0.0.0.0:8081   (skypanel)
```

### Verificar Acceso Web

```bash
# Probar acceso local
curl -I http://localhost:8080

# Debería retornar: HTTP/1.1 200 OK
```

### Verificar Base de Datos

```bash
# Verificar que la base de datos se creó
sudo -u skypanel sqlite3 /var/lib/skypanel/database.db ".tables"

# Debería mostrar las tablas: users, servers, nodes, permissions, etc.
```

---

## Configuración Inicial

### Acceder al Panel

1. Abre tu navegador web
2. Navega a: `http://TU_IP:8080`
3. Deberías ver la página de inicio de SkyPanel

### Crear Primer Usuario Administrador

#### Método 1: Interfaz Web

Si la registración está habilitada, simplemente regístrate desde la web.

#### Método 2: Línea de Comandos (Recomendado)

```bash
# Crear usuario administrador
sudo -u skypanel skypanel user add

# El comando te pedirá:
# - Username: admin
# - Email: admin@example.com
# - Password: (tu contraseña segura)
```

O en una sola línea:

```bash
# Crear usuario admin directamente
sudo -u skypanel skypanel user add \
  --username admin \
  --email admin@example.com \
  --password "TuContraseñaSegura123!" \
  --admin
```

### Configurar Firewall

#### UFW (Ubuntu/Debian)

```bash
# Permitir puertos de SkyPanel
sudo ufw allow 8080/tcp comment 'SkyPanel Web'
sudo ufw allow 5657/tcp comment 'SkyPanel SFTP'
sudo ufw allow 8081/tcp comment 'SkyPanel Gatus'

# Si usas SSH, asegúrate de permitirlo
sudo ufw allow 22/tcp

# Habilitar firewall
sudo ufw enable

# Verificar reglas
sudo ufw status
```

#### Firewalld (CentOS/RHEL/Fedora)

```bash
# Permitir puertos
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=5657/tcp
sudo firewall-cmd --permanent --add-port=8081/tcp

# Recargar firewall
sudo firewall-cmd --reload

# Verificar
sudo firewall-cmd --list-ports
```

### Configurar Proxy Reverso (Opcional)

Si deseas usar un dominio y HTTPS, configura Nginx o Apache.

#### Nginx

```bash
# Instalar Nginx
sudo apt install nginx -y  # Ubuntu/Debian
# sudo dnf install nginx -y  # CentOS/RHEL/Fedora

# Crear configuración
sudo tee /etc/nginx/sites-available/skypanel > /dev/null <<'EOF'
server {
    listen 80;
    server_name panel.tudominio.com;

    # Redirigir a HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name panel.tudominio.com;

    # Certificados SSL (usar Let's Encrypt)
    ssl_certificate /etc/letsencrypt/live/panel.tudominio.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/panel.tudominio.com/privkey.pem;

    # Configuración SSL
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Logs
    access_log /var/log/nginx/skypanel-access.log;
    error_log /var/log/nginx/skypanel-error.log;

    # Proxy a SkyPanel
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Gatus (monitoreo)
    location /status {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
EOF

# Habilitar sitio
sudo ln -s /etc/nginx/sites-available/skypanel /etc/nginx/sites-enabled/

# Probar configuración
sudo nginx -t

# Recargar Nginx
sudo systemctl reload nginx
```

#### Obtener Certificado SSL con Let's Encrypt

```bash
# Instalar Certbot
sudo apt install certbot python3-certbot-nginx -y  # Ubuntu/Debian
# sudo dnf install certbot python3-certbot-nginx -y  # CentOS/RHEL/Fedora

# Obtener certificado
sudo certbot --nginx -d panel.tudominio.com

# Renovación automática (ya configurada por defecto)
sudo certbot renew --dry-run
```

---

## Solución de Problemas

### Problema: El servicio no inicia

**Síntomas**: `systemctl status skypanel` muestra "failed"

**Solución**:

```bash
# Ver logs detallados
sudo journalctl -u skypanel -n 100 --no-pager

# Verificar permisos
sudo chown -R skypanel:skypanel /var/lib/skypanel
sudo chown -R skypanel:skypanel /var/log/skypanel

# Verificar configuración
sudo -u skypanel skypanel run --config /etc/skypanel/config.json

# Si hay errores de sintaxis en config.json
sudo nano /etc/skypanel/config.json
```

### Problema: Puerto 8080 ya en uso

**Síntomas**: Error "address already in use"

**Solución**:

```bash
# Verificar qué proceso usa el puerto
sudo lsof -i :8080

# Cambiar puerto en configuración
sudo nano /etc/skypanel/config.json
# Cambiar "host": "0.0.0.0:8080" a "host": "0.0.0.0:9090"

# Reiniciar servicio
sudo systemctl restart skypanel
```

### Problema: Error de base de datos

**Síntomas**: "database is locked" o "unable to open database"

**Solución**:

```bash
# Verificar permisos
sudo chown skypanel:skypanel /var/lib/skypanel/database.db
sudo chmod 644 /var/lib/skypanel/database.db

# Si la base de datos está corrupta, restaurar desde backup
sudo -u skypanel cp /var/lib/skypanel/backups/database.db.backup /var/lib/skypanel/database.db

# O crear nueva base de datos
sudo -u skypanel rm /var/lib/skypanel/database.db
sudo systemctl restart skypanel
```

### Problema: No se puede acceder desde Internet

**Síntomas**: Funciona en localhost pero no desde IP externa

**Solución**:

```bash
# Verificar firewall
sudo ufw status  # Ubuntu/Debian
sudo firewall-cmd --list-all  # CentOS/RHEL/Fedora

# Verificar que escucha en 0.0.0.0 y no en 127.0.0.1
sudo netstat -tulpn | grep 8080

# Si escucha en 127.0.0.1, cambiar en config.json
sudo nano /etc/skypanel/config.json
# "host": "0.0.0.0:8080"  # No "127.0.0.1:8080"
```

### Problema: Frontend no carga (página en blanco)

**Síntomas**: Panel muestra página en blanco o error 404

**Solución**:

```bash
# Verificar que los archivos del frontend existen
ls -la /var/www/skypanel/

# Si no existen, recompilar frontend
cd /opt/skypanel/client/frontend
yarn build
sudo cp -r dist/* /var/www/skypanel/

# Verificar configuración en config.json
sudo nano /etc/skypanel/config.json
# "web": { "files": "/var/www/skypanel" }

# Reiniciar servicio
sudo systemctl restart skypanel
```

### Problema: SFTP no funciona

**Síntomas**: No se puede conectar por SFTP

**Solución**:

```bash
# Verificar que el puerto SFTP está escuchando
sudo netstat -tulpn | grep 5657

# Verificar configuración
sudo nano /etc/skypanel/config.json
# "sftp": { "host": "0.0.0.0:5657" }

# Verificar firewall
sudo ufw allow 5657/tcp

# Probar conexión SFTP
sftp -P 5657 usuario@localhost
```

### Problema: Gatus no inicia

**Síntomas**: Monitoreo no funciona

**Solución**:

```bash
# Verificar que Gatus está habilitado
sudo nano /etc/skypanel/config.json
# "gatus": { "enable": true, "port": 8081 }

# Verificar logs
sudo journalctl -u skypanel | grep -i gatus

# Verificar puerto
sudo netstat -tulpn | grep 8081

# Reiniciar servicio
sudo systemctl restart skypanel
```

---

## Comandos Útiles de Gestión

### Gestión del Servicio

```bash
# Ver estado
sudo systemctl status skypanel

# Iniciar
sudo systemctl start skypanel

# Detener
sudo systemctl stop skypanel

# Reiniciar
sudo systemctl restart skypanel

# Recargar configuración (sin reiniciar)
sudo systemctl reload skypanel

# Habilitar inicio automático
sudo systemctl enable skypanel

# Deshabilitar inicio automático
sudo systemctl disable skypanel
```

### Gestión de Logs

```bash
# Ver logs en tiempo real
sudo journalctl -u skypanel -f

# Ver logs desde hace 1 hora
sudo journalctl -u skypanel --since "1 hour ago"

# Ver logs de hoy
sudo journalctl -u skypanel --since today

# Ver logs con prioridad de error
sudo journalctl -u skypanel -p err

# Limpiar logs antiguos (mantener últimos 7 días)
sudo journalctl --vacuum-time=7d
```

### Gestión de Usuarios

```bash
# Listar usuarios
sudo -u skypanel skypanel user list

# Crear usuario
sudo -u skypanel skypanel user add

# Eliminar usuario
sudo -u skypanel skypanel user delete --username usuario

# Cambiar contraseña
sudo -u skypanel skypanel user password --username usuario
```

### Gestión de Base de Datos

```bash
# Hacer backup de la base de datos
sudo -u skypanel cp /var/lib/skypanel/database.db /var/lib/skypanel/backups/database-$(date +%Y%m%d).db

# Migrar base de datos
sudo -u skypanel skypanel db migrate

# Actualizar base de datos
sudo -u skypanel skypanel db upgrade
```

---

## Próximos Pasos

Una vez completada la instalación:

1. 📖 Lee la [Guía de Configuración](./02-configuration.md) para personalizar SkyPanel
2. 🎮 Consulta la [Guía de Creación de Servidores](./03-creating-servers.md) para crear tu primer servidor
3. 👥 Revisa la [Gestión de Usuarios](./04-user-management.md) para administrar permisos
4. 🔧 Explora la [Referencia de API](./05-api-reference.md) para automatización

---

## Recursos Adicionales

- 📚 [Documentación Completa](https://docs.skypanel.com)
- 💬 [Discord de la Comunidad](https://discord.gg/skypanel)
- 🐛 [Reportar Problemas](https://github.com/SkyPanel/SkyPanel/issues)
- 📝 [Changelog](https://github.com/SkyPanel/SkyPanel/releases)

---

**¿Necesitas ayuda?** Únete a nuestro [Discord](https://discord.gg/skypanel) o abre un [issue en GitHub](https://github.com/SkyPanel/SkyPanel/issues).
