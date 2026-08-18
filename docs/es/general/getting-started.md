# Primeros Pasos - Guía Completa

## Bienvenido a Aether Panel

Esta guía te llevará paso a paso desde la instalación hasta tener tu primer servidor funcionando. Sigue cada sección en orden para una configuración exitosa.

## Requisitos del Sistema

Antes de comenzar, asegúrate de que tu sistema cumpla con los siguientes requisitos:

Es importante tener en cuenta que Aether Panel requiere al menos 2 vcores de CPU, 2GB de RAM y 5GB de almacenamiento solo para funcionar correctamente. Estos requisitos no cubren el uso de los servicios que gestionará en el panel.

- **cardTitle**: Hardware y Software
- **cardTitle2**: Mínimo
- Sistema Operativo: Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+).
- Arquitectura: amd64 o arm64.
- Virtualización: KVM, OpenVZ, LXC o servidor dedicado (para los nodos).
- Software: Docker y Docker Compose (v20.10.0+, opcional).
- Hardware: 2 vcores de CPU, 2GB de RAM, 5GB de almacenamiento (mínimo).

- **cardTitle3**: Recomendado
- Sistema Operativo: Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+).
- Arquitectura: amd64 o arm64.
- Virtualización: KVM, OpenVZ, LXC o servidor dedicado (para los nodos).
- Software: Docker y Docker Compose (v20.10.0+, opcional).
- Hardware: 2 vcores de CPU, 4GB de RAM, 20GB de almacenamiento.

### Puertos Necesarios

Asegúrate de que los siguientes puertos estén disponibles:

- 8080/TCP: Panel web (HTTP/HTTPS) - Obligatorio
- 5657/TCP: SFTP para transferencia de archivos - Obligatorio
- 22/TCP: SSH para administración - Recomendado cambiar a puerto no estándar

### Verificar Requisitos

Antes de instalar, verifica que tienes acceso root o sudo:

Verifica que los puertos estén libres:

Si algún puerto está en uso, detén el servicio o cambia el puerto en la configuración.

- **code1**: sudo whoami
- **code2**: # Verificar puerto 8080
sudo netstat -tuln | grep 8080

# Verificar puerto 5657
sudo netstat -tuln | grep 5657
## Instalación Paso a Paso

### Métodos de Instalación

Aether Panel puede instalarse de tres formas diferentes. Elige la que mejor se adapte a tus necesidades:

- Instalación Automática con Script (Recomendado): El método más fácil y rápido
- Instalación con Docker: Ideal para entornos containerizados
- Instalación Manual: Para usuarios avanzados que quieren control total

- **linuxTitle**: Método 1: Instalación Automática con Script
- **linuxP1**: Este es el método más recomendado. El script instala Docker (si falta), clona el repositorio, construye la imagen y configura el servicio.
- **linuxP2**: Ejecuta el siguiente comando como root o con sudo para iniciar la instalación:
- **linuxCode**: bash -c "$(curl -s https://install.aetherpanel.es/install.sh)"
- **linuxP3**: O descarga el script primero y luego ejecútalo:
- **linuxCode2**: # Descargar el script
wget https://install.aetherpanel.es/install.sh

# Dar permisos de ejecución
chmod +x install.sh

# Ejecutar como root
sudo bash install.sh
### Proceso Interactivo

Durante la instalación, el script te preguntará:

- ¿Usar un dominio o solo IP? (si usas dominio, configura Nginx como proxy reverso)
- SSL: ¿Quieres configurar SSL con Let's Encrypt? (solo si usas dominio)
- Puertos: ¿Quieres cambiar los puertos por defecto? (opcional)

### El script de instalación realizará los siguientes pasos:

- Detectará automáticamente tu sistema operativo (Ubuntu, Debian, Fedora, CentOS, RHEL)
- Instalará Docker y Docker Compose v2 (si no están presentes)
- Clonará el repositorio desde https://github.com/Aether-Panel/Panel.git
- Construirá la imagen con `docker compose build` (rama dev2.0)
- Configurará Nginx como proxy reverso (solo si usas dominio)
- Configurará SSL con Let's Encrypt (opcional)
- Creará el usuario administrador inicial
- Iniciará los contenedores con `docker compose up -d`

- **dockerTitle**: Método 2: Instalación con Docker
- **dockerP1**: Si prefieres usar Docker, puedes desplegar Aether Panel con Docker Compose. Este método es ideal si ya estás familiarizado con Docker.
### Requisitos Previos

Asegúrate de tener Docker y Docker Compose instalados:

- **checkDocker**: # Verificar Docker
docker --version

# Verificar Docker Compose
docker compose version
#### Si no tienes Docker instalado:

- **ubuntu**: # Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
- **fedora**: # Fedora/RHEL/CentOS
sudo dnf install -y docker docker-compose-plugin
sudo systemctl enable docker
sudo systemctl start docker
### Pasos para instalar con Docker:

#### 1. Clonar el repositorio

Esto descargará todo el código fuente del panel.

```text
git clone https://github.com/Aether-Panel/Panel.git
cd Panel
```

#### 2. Configurar base de datos (opcional)

Si quieres usar MySQL externo, edita config.docker.json:

Si usas el MySQL del docker-compose.yml, no necesitas cambiar nada.

```text
{
  "panel": {
    "database": {
      "dialect": "mysql",
      "url": "usuario:contraseña@tcp(IP:3306)/basedatos?charset=utf8mb4&parseTime=true"
    }
  }
}
```

#### 3. Construir la imagen Docker

Este proceso puede tardar varios minutos la primera vez, ya que descarga dependencias y compila el panel.

```text
docker compose build
```

#### 4. Iniciar los contenedores

El flag -d ejecuta los contenedores en segundo plano (detached mode).

```text
docker compose up -d
```

#### 5. Verificar que los contenedores estén corriendo

Deberías ver los contenedores 'skypanel' y 'skypanel-mysql' con estado 'Up'.

```text
docker compose ps
```

#### 6. Ver los logs (opcional)

Presiona Ctrl+C para salir de los logs.

```text
docker compose logs -f
```

> **Nota:** No existe una imagen pre-construida publicada. La imagen se construye siempre desde el `Dockerfile` del repositorio (`docker compose build` o `docker compose up -d --build`).

### Crear Usuario Administrador (Docker)

Una vez que el contenedor esté corriendo, ejecuta este comando para crear tu cuenta de administrador:

Recuerda cambiar admin@example.com y admin123 por tus datos reales.

```text
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --password 'admin123' --admin
```

### Configuración de Docker

El docker-compose.yml incluye:

- Servicio MySQL/MariaDB para la base de datos
- Servicio Aether Panel con todos los puertos necesarios
- Volúmenes persistentes para datos y configuración
- Health checks para verificar el estado de los servicios

### Post-Instalación

Una vez instalado, el panel estará accesible en:

Inicia sesión con las credenciales de administrador creadas durante el proceso de instalación.

```text
http://<IP-DE-SU-SERVIDOR>:8080
```

- **nativeTitle**: Método 3: Instalación Manual (Sin Docker)
- **nativeP1**: **No existe un modo nativo soportado.** La instalación oficial es solo vía Docker. Si aún así quieres compilar el panel desde el código fuente (por ejemplo, para desarrollo), puedes hacerlo siguiendo estos pasos, pero no es el método de instalación recomendado ni el que usa el instalador automático.
### Instalar Dependencias Manualmente

#### Ubuntu/Debian

```text
# Actualizar sistema
sudo apt update && sudo apt upgrade -y

# Instalar dependencias básicas
sudo apt install -y git build-essential curl wget sqlite3

# Instalar Go 1.25.0
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verificar Go
go version

# Instalar Node.js 22
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs

# Instalar Yarn
sudo npm install -g yarn

# Verificar instalaciones
node --version
yarn --version
```

#### Fedora/RHEL/CentOS

```text
# Actualizar sistema
sudo dnf update -y

# Instalar dependencias básicas
sudo dnf install -y git gcc make curl wget sqlite

# Instalar Go 1.25.0
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Instalar Node.js 22
curl -fsSL https://rpm.nodesource.com/setup_22.x | sudo bash -
sudo dnf install -y nodejs

# Instalar Yarn
sudo npm install -g yarn
```

### Proceso de Compilación Manual:

#### 1. Clonar el repositorio

```text
git clone https://github.com/Aether-Panel/Panel.git
cd Panel
```

#### 2. Compilar el frontend

Esto compilará la interfaz web (Astro + React).

```text
cd client/frontend
yarn install
yarn build
```

#### 3. Compilar el backend

Esto creará el binario ejecutable 'SkyPanel'.

```text
cd ../..
go mod download
go build -o SkyPanel ./cmd/panel
```

#### 4. Crear estructura de directorios

```text
sudo mkdir -p /etc/SkyPanel
sudo mkdir -p /var/lib/SkyPanel
sudo mkdir -p /var/log/SkyPanel
sudo mkdir -p /var/www/aetherpanel
```

#### 5. Copiar archivos compilados

```text
sudo cp -r client/frontend/dist/* /var/www/aetherpanel/
sudo cp SkyPanel /usr/sbin/
sudo chmod +x /usr/sbin/SkyPanel
```

#### 6. Crear archivo de configuración

```text
sudo tee /etc/SkyPanel/config.json > /dev/null <<EOF
{
  "logs": "/var/log/SkyPanel",
  "panel": {
    "database": {
      "dialect": "sqlite3",
      "url": "file:/var/lib/SkyPanel/database.db?cache=shared"
    },
    "web": {
      "files": "/var/www/aetherpanel"
    }
  },
  "daemon": {
    "data": {
      "root": "/var/lib/SkyPanel"
    }
  }
}
EOF
```

#### 7. Crear usuario del sistema

```text
sudo useradd -r -m -d /var/lib/SkyPanel -s /bin/bash skypanel
sudo chown -R skypanel:skypanel /var/lib/SkyPanel /var/log/SkyPanel
```

#### 8. Crear servicio systemd

```text
sudo tee /etc/systemd/system/skypanel.service > /dev/null <<EOF
[Unit]
Description=Aether Panel - Panel de Gestión de Servidores
After=network.target

[Service]
Type=simple
User=skypanel
WorkingDirectory=/var/lib/SkyPanel
ExecStart=/usr/sbin/SkyPanel runService --config /etc/SkyPanel/config.json
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable skypanel
```

## Verificación de la Instalación

Después de instalar, verifica que todo esté funcionando correctamente:

### Verificar Servicio (Instalación Nativa)

```text
# Ver estado del servicio
sudo systemctl status skypanel

# Ver logs en tiempo real
sudo journalctl -u skypanel -f
```

### Verificar Contenedores (Instalación Docker)

```text
# Ver estado de contenedores
docker compose ps

# Ver logs
docker compose logs -f skypanel
```

### Verificar Puertos

```text
# Verificar que los puertos estén escuchando
sudo netstat -tuln | grep -E '8080|5657'

# O usar ss
sudo ss -tuln | grep -E '8080|5657'
```

### Verificar Panel Web

```text
# Desde el servidor
curl http://localhost:8080

# O desde tu navegador
# http://TU_IP:8080
```

## Primer Acceso y Configuración Inicial

Una vez completada la instalación y verificación, es hora de acceder al panel y configurarlo por primera vez.

### Acceder al Panel

Abre tu navegador y accede a:

Serás recibido por la pantalla de inicio de sesión.

- **ip**: http://TU_IP:8080
- **domain**: https://tu-dominio.com (si configuraste dominio y SSL)
### Paso 1: Crear Usuario Administrador

Antes de poder iniciar sesión, necesitas crear un usuario administrador usando el CLI.

Si instalaste con Docker, ejecuta los comandos dentro del contenedor:

Si instalaste nativamente, ejecuta directamente:

- **dockerCommand**: # Entrar al contenedor
docker exec -it skypanel sh

# Crear usuario admin
/SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin
- **nativeCommand**: sudo -u skypanel /usr/sbin/SkyPanel user add --name admin --email admin@example.com --admin
- **p4**: O desde el directorio de instalación:
- **localCommand**: SkyPanel user add --name admin --email admin@example.com --admin
- **p5**: El sistema te pedirá una contraseña. Elige una contraseña segura.
#### Parámetros del Comando

- --username: Nombre de usuario para iniciar sesión
- --email: Dirección de correo electrónico
- --admin: Otorga permisos de administrador completo

### Paso 2: Iniciar Sesión

Ahora puedes iniciar sesión en el panel:

- Abre http://TU_IP:8080 en tu navegador
- Ingresa el nombre de usuario que creaste
- Ingresa la contraseña
- Haz clic en 'Iniciar Sesión'

### Comandos CLI Disponibles

Para ver todos los comandos disponibles del CLI de Aether Panel:

O desde el contenedor Docker:

- **helpCommand**: SkyPanel --help
- **dockerHelp**: docker exec skypanel /SkyPanel/bin/SkyPanel --help
#### Comandos Principales

##### Gestión de Usuarios

- SkyPanel user add --name USER --email EMAIL --admin
- SkyPanel user add --name USER --email EMAIL --password PASS (no interactivo)
- SkyPanel user edit (menú interactivo para cambiar username/email/password/admin/2FA)

##### Gestión de Base de Datos

- SkyPanel db upgrade (actualizar esquema de BD a la nueva versión)

##### Ejecutar Panel

- SkyPanel runService (iniciar como servicio con systemd notify)
- SkyPanel version (ver versión)

### Paso 3: Configurar Firewall

Si no tienes habilitados los puertos en el firewall, el panel no será accesible desde fuera del servidor.

#### Ubuntu/Debian (UFW)

```text
# Habilitar UFW si no está activo
sudo ufw enable

# Permitir puertos necesarios
sudo ufw allow 8080/tcp
sudo ufw allow 5657/tcp
sudo ufw allow 8081/tcp

# Verificar reglas
sudo ufw status
```

#### Fedora/RHEL/CentOS (firewalld)

```text
# Iniciar firewalld si no está activo
sudo systemctl start firewalld
sudo systemctl enable firewalld

# Permitir puertos necesarios
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=5657/tcp
sudo firewall-cmd --permanent --add-port=8081/tcp

# Aplicar cambios
sudo firewall-cmd --reload

# Verificar reglas
sudo firewall-cmd --list-ports
```

### Paso 4: Configuración Inicial en el Panel

Una vez que hayas iniciado sesión, configura los ajustes básicos:

#### Configuración Básica

- Ve a Settings (Configuración) en el menú
- Configura la URL del panel (ej: http://TU_IP:8080 o https://tu-dominio.com)
- Configura el email del administrador
- Configura el nombre de la empresa/organización
- Guarda los cambios

#### Configurar Nodos

Si instalaste en el mismo servidor donde correrán los servidores (nodo local), no necesitas configurar nada más. El nodo local se configura automáticamente.

Si quieres usar nodos remotos:

- Ve a Admin  Nodos
- Haz clic en 'Crear Nodo'
- Ingresa la IP del nodo remoto
- Ingresa el puerto del daemon (por defecto 8080)
- Ingresa el token de autenticación del daemon
- Verifica la conexión
- Guarda el nodo

#### Configurar Discord (Opcional)

Para recibir notificaciones en Discord:

Para crear un webhook de Discord:

- Ve a Settings  Notifications
- Ingresa la URL del webhook de Discord
- Configura los tipos de notificaciones que quieres recibir
- Guarda los cambios

- Ve a tu servidor de Discord
- Configuración del Canal  Integraciones  Webhooks
- Crea un nuevo webhook
- Copia la URL del webhook

### Paso 5: Crear tu Primer Servidor

Ahora estás listo para crear tu primer servidor:

¡Felicidades! Ya tienes tu primer servidor funcionando en Aether Panel.

- Ve a la sección Servidores
- Haz clic en 'Crear Servidor'
- Selecciona el nodo donde se ejecutará (o usa el nodo local)
- Elige una plantilla (ej: Minecraft Java Edition)
- Configura el nombre del servidor
- Configura el puerto del servidor
- Ajusta los recursos (RAM, CPU) si es necesario
- Haz clic en 'Crear'
- Espera a que el servidor se instale
- Haz clic en 'Iniciar' para arrancar el servidor

## Resumen de la Instalación

Si has seguido todos los pasos, deberías tener:

-  Aether Panel instalado y funcionando
-  Usuario administrador creado
-  Firewall configurado
-  Panel accesible en http://TU_IP:8080
-  Configuración básica completada
-  Primer servidor creado (opcional)

### Próximos Pasos

Ahora que tienes el panel funcionando, puedes:

- Explorar todas las funcionalidades del panel
- Crear más servidores de diferentes tipos
- Configurar Database Hosts para bases de datos MySQL
- Añadir usuarios adicionales con permisos específicos
- Configurar backups automáticos
- Explorar la API para automatización
- Leer la documentación completa en las otras secciones

### ¿Necesitas Ayuda?

Si encuentras problemas durante la instalación:

- Revisa la sección Troubleshooting en la documentación
- Verifica los logs del panel
- Consulta el FAQ para problemas comunes
- Únete al Discord de la comunidad para obtener ayuda

## Configuración Avanzada del Panel

### Archivo de Configuración

El archivo de configuración principal se encuentra en:

Puedes editar este archivo para personalizar la configuración del panel.

- **native**: /etc/SkyPanel/config.json (instalación nativa)
- **docker**: /etc/SkyPanel/config.json (dentro del contenedor Docker)
### Configuración de Base de Datos

Aether Panel soporta SQLite (por defecto) y MySQL/MariaDB.

#### SQLite (Por Defecto)

SQLite se configura automáticamente y no requiere configuración adicional. Es ideal para instalaciones pequeñas.

#### MySQL/MariaDB

Para usar MySQL, edita el archivo de configuración:

Asegúrate de que la base de datos y el usuario existan antes de iniciar el panel.

```text
{
  "panel": {
    "database": {
      "dialect": "mysql",
      "url": "usuario:contraseña@tcp(host:3306)/basedatos?charset=utf8mb4&parseTime=true"
    }
  }
}
```

### Configuración de Puertos

Los puertos por defecto son:

Puedes cambiar estos puertos editando el archivo de configuración o las variables de entorno.

- 8080: Panel web (HTTP)
- 5657: SFTP para transferencia de archivos

