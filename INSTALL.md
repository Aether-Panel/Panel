# Instalador Automático de SkyPanel

Script de instalación automática para SkyPanel que detecta la IP/puerto de la VPS y configura todo el panel.

## Uso

### Instalación desde URL (recomendado)

```bash
bash <(curl -s https://tu-servidor.com/SkyPanel-install.sh)
```

O usando una IP específica:

```bash
bash <(curl -s http://192.168.0.5:8080/install.sh)
```

### Instalación local

```bash
sudo bash install.sh
```

## Variables de Entorno

Puedes personalizar la instalación con variables de entorno:

```bash
# Cambiar repositorio Git
export SkyPanel_REPO_URL="https://github.com/tu-usuario/SkyPanel.git"

# Cambiar rama
export SkyPanel_BRANCH="master"

# Cambiar puerto
export SkyPanel_PORT="8080"

# Luego ejecutar
bash install.sh
```

## Requisitos

- Ubuntu/Debian/CentOS/RHEL/Fedora
- Acceso root (sudo)
- Conexión a Internet
- Al menos 2GB RAM
- Al menos 10GB espacio en disco

## Lo que hace el script

1. **Detecta la distribución** del sistema operativo
2. **Instala dependencias** (Git, build-essential, SQLite, etc.)
3. **Instala Go 1.24+** (requerido para compilar el backend)
4. **Instala Node.js 22+ y Yarn** (requerido para compilar el frontend)
5. **Crea usuario `SkyPanel`** para ejecutar el servicio
6. **Clona el repositorio** desde GitHub (o URL personalizada)
7. **Compila el frontend** con Yarn
8. **Compila el backend** con Go
9. **Crea estructura de directorios**:
   - `/etc/SkyPanel` - Configuración
   - `/var/lib/SkyPanel` - Datos del panel
   - `/var/log/SkyPanel` - Logs
   - `/var/www/SkyPanel` - Archivos web
10. **Genera config.json** con IP/puerto detectados automáticamente
11. **Configura servicio systemd** para iniciar automáticamente
12. **Inicia el servicio** y muestra información de acceso

## Detección de IP

El script intenta detectar la IP de la VPS en este orden:

1. IP pública (usando checkip.amazonaws.com)
2. IP local (usando `hostname -I`)
3. Localhost (127.0.0.1) como fallback

## Puertos utilizados

- **8080** - Panel web (configurable con `SkyPanel_PORT`)
- **5657** - SFTP para archivos de servidores

## Después de la instalación

1. Accede al panel en `http://TU_IP:8080`
2. Crea el primer usuario administrador
3. Configura el firewall si es necesario:
   ```bash
   sudo ufw allow 8080/tcp
   sudo ufw allow 5657/tcp
   ```

## Comandos útiles

```bash
# Ver estado del servicio
sudo systemctl status SkyPanel

# Ver logs en tiempo real
sudo journalctl -u SkyPanel -f

# Reiniciar el servicio
sudo systemctl restart SkyPanel

# Detener el servicio
sudo systemctl stop SkyPanel

# Iniciar el servicio
sudo systemctl start SkyPanel
```

## Desinstalación

```bash
sudo systemctl stop SkyPanel
sudo systemctl disable SkyPanel
sudo rm /etc/systemd/system/SkyPanel.service
sudo systemctl daemon-reload

# Eliminar archivos (opcional)
sudo rm -rf /etc/SkyPanel
sudo rm -rf /var/lib/SkyPanel
sudo rm -rf /var/log/SkyPanel
sudo rm -rf /var/www/SkyPanel
sudo rm -rf /opt/SkyPanel
sudo rm /usr/local/bin/SkyPanel
sudo userdel SkyPanel
```

## Personalización

Para usar el script en tu propio servidor:

1. Sube `install.sh` a tu servidor web
2. Configura CORS si es necesario
3. Servir con el header correcto:
   ```bash
   Content-Type: text/plain; charset=utf-8
   ```
4. Usa la URL en el comando curl

## Solución de problemas

### Error: "No se pudo detectar la distribución"
- Asegúrate de estar ejecutando en Ubuntu/Debian/CentOS/RHEL/Fedora

### Error: "Go installation failed"
- Verifica que tienes conexión a Internet
- Verifica que tienes espacio en disco suficiente

### Error: "Yarn build failed"
- Asegúrate de tener al menos 2GB RAM
- Verifica los logs: `cat /tmp/SkyPanel-build.log`

### Error: "Service failed to start"
- Revisa los logs: `journalctl -u SkyPanel -n 50`
- Verifica que los puertos no estén en uso: `netstat -tulpn | grep -E '8080|5657'`

## Notas

- El script requiere conexión a Internet para descargar dependencias
- La primera instalación puede tardar 10-15 minutos dependiendo de la velocidad de Internet
- El script genera tokens aleatorios para seguridad del panel

