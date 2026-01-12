# 🐳 SkyPanel - Guía de Docker

Esta guía te ayudará a ejecutar SkyPanel en contenedores Docker de forma fácil y controlada.

## 📋 Requisitos Previos

- **Docker** 20.10+ instalado
- **Docker Compose** 2.0+ instalado
- Al menos **2GB de RAM** disponible
- Al menos **5GB de espacio en disco**

### Instalar Docker (si no lo tienes)

```bash
# Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Reinicia la sesión o ejecuta
newgrp docker

# Verificar instalación
docker --version
docker-compose --version
```

## 🚀 Inicio Rápido

### Opción 1: Script Automatizado (Recomendado)

```bash
# 1. Dar permisos de ejecución
chmod +x docker-test.sh

# 2. Construir la imagen
./docker-test.sh build

# 3. Iniciar el contenedor
./docker-test.sh start

# 4. Crear usuario administrador
./docker-test.sh admin

# 5. Acceder al panel
# Abre tu navegador en: http://localhost:8080
```

### Opción 2: Docker Compose Manual

```bash
# Desarrollo/Pruebas
docker-compose -f docker-compose.dev.yml up -d

# Producción
docker-compose up -d
```

### Opción 3: Docker Run Directo

```bash
# Construir imagen
docker build -t skypanel:latest .

# Ejecutar contenedor
docker run -d \
  --name skypanel \
  -p 8080:8080 \
  -p 5657:5657 \
  -p 8081:8081 \
  -v skypanel-data:/var/lib/SkyPanel \
  -v skypanel-config:/etc/SkyPanel \
  -v skypanel-logs:/var/log/SkyPanel \
  skypanel:latest
```

## 🎮 Comandos del Script

El script `docker-test.sh` proporciona comandos fáciles de usar:

```bash
# Construcción
./docker-test.sh build          # Construir imagen

# Control del contenedor
./docker-test.sh start          # Iniciar
./docker-test.sh stop           # Detener
./docker-test.sh restart        # Reiniciar
./docker-test.sh status         # Ver estado

# Monitoreo
./docker-test.sh logs           # Ver logs en tiempo real

# Administración
./docker-test.sh admin          # Crear usuario admin
./docker-test.sh shell          # Abrir shell en el contenedor

# Limpieza
./docker-test.sh clean          # Limpiar todo
./docker-test.sh rebuild        # Reconstruir e iniciar

# Ayuda
./docker-test.sh help           # Ver ayuda completa
```

## 🌐 Acceso al Panel

Una vez iniciado, puedes acceder a:

- **Panel Web**: http://localhost:8080
- **Gatus (Monitoring)**: http://localhost:8081
- **SFTP**: `localhost:5657`

### Credenciales Iniciales

Debes crear un usuario administrador:

```bash
./docker-test.sh admin
```

O manualmente:

```bash
docker exec -it skypanel-dev /SkyPanel/bin/SkyPanel user add \
  --email admin@example.com \
  --password tu-contraseña-segura \
  --admin
```

## 📊 Gestión del Contenedor

### Ver Logs

```bash
# Logs en tiempo real
./docker-test.sh logs

# O con docker-compose
docker-compose -f docker-compose.dev.yml logs -f

# Solo las últimas 100 líneas
docker logs --tail 100 skypanel-dev
```

### Verificar Estado

```bash
# Con el script
./docker-test.sh status

# Manualmente
docker ps | grep skypanel
docker stats skypanel-dev
```

### Ejecutar Comandos

```bash
# Shell interactivo
./docker-test.sh shell

# Ejecutar comando específico
docker exec skypanel-dev /SkyPanel/bin/SkyPanel version

# Listar usuarios
docker exec skypanel-dev /SkyPanel/bin/SkyPanel user list
```

## 🗂️ Volúmenes y Datos

### Ubicación de Datos

**Modo Desarrollo** (`docker-compose.dev.yml`):
```
./dev-data/
├── config/     # Configuración
├── data/       # Base de datos y servidores
└── logs/       # Logs
```

**Modo Producción** (`docker-compose.yml`):
```
Volúmenes Docker nombrados:
- skypanel-config
- skypanel-data
- skypanel-logs
```

### Backup de Datos

```bash
# Desarrollo (archivos locales)
tar -czf skypanel-backup-$(date +%Y%m%d).tar.gz dev-data/

# Producción (volúmenes Docker)
docker run --rm \
  -v skypanel-data:/data \
  -v $(pwd):/backup \
  alpine tar -czf /backup/skypanel-backup-$(date +%Y%m%d).tar.gz /data
```

### Restaurar Backup

```bash
# Desarrollo
tar -xzf skypanel-backup-YYYYMMDD.tar.gz

# Producción
docker run --rm \
  -v skypanel-data:/data \
  -v $(pwd):/backup \
  alpine tar -xzf /backup/skypanel-backup-YYYYMMDD.tar.gz -C /
```

## 🔧 Configuración Avanzada

### Variables de Entorno

Puedes personalizar el comportamiento editando `docker-compose.yml`:

```yaml
environment:
  - GIN_MODE=release                    # release o debug
  - PUFFER_WEB_HOST=0.0.0.0:8080       # Host y puerto
  - PUFFER_PANEL_REGISTRATIONENABLED=true  # Permitir registro
  - PUFFER_PANEL_SETTINGS_COMPANYNAME=Mi Empresa
  - PUFFER_PANEL_SETTINGS_DEFAULTTHEME=SkyPanel
```

### Puertos Personalizados

```yaml
ports:
  - "9000:8080"   # Panel en puerto 9000
  - "2222:5657"   # SFTP en puerto 2222
  - "9001:8081"   # Gatus en puerto 9001
```

### Límites de Recursos

```yaml
deploy:
  resources:
    limits:
      cpus: '4'
      memory: 4G
    reservations:
      cpus: '1'
      memory: 1G
```

## 🐛 Solución de Problemas

### El contenedor no inicia

```bash
# Ver logs de error
docker logs skypanel-dev

# Verificar configuración
docker-compose -f docker-compose.dev.yml config

# Verificar puertos en uso
sudo netstat -tulpn | grep -E '8080|5657|8081'
```

### Error de permisos

```bash
# Agregar usuario al grupo docker
sudo usermod -aG docker $USER
newgrp docker

# O ejecutar con sudo
sudo docker-compose -f docker-compose.dev.yml up -d
```

### No puedo acceder al panel

```bash
# Verificar que el contenedor está corriendo
docker ps | grep skypanel

# Verificar logs
docker logs skypanel-dev

# Verificar conectividad
curl http://localhost:8080
```

### Base de datos corrupta

```bash
# Detener contenedor
./docker-test.sh stop

# Eliminar base de datos
rm -f dev-data/data/database.db*

# Reiniciar
./docker-test.sh start
```

## 🔄 Actualización

```bash
# 1. Detener contenedor
./docker-test.sh stop

# 2. Hacer backup
tar -czf backup-$(date +%Y%m%d).tar.gz dev-data/

# 3. Actualizar código (git pull, etc.)

# 4. Reconstruir
./docker-test.sh rebuild
```

## 🧹 Limpieza Completa

```bash
# Eliminar todo (contenedor, imágenes, volúmenes)
./docker-test.sh clean

# O manualmente
docker-compose -f docker-compose.dev.yml down -v
docker rmi skypanel:latest
rm -rf dev-data/
```

## 📚 Recursos Adicionales

- **Documentación oficial**: [docs/README.md](docs/README.md)
- **API Reference**: [docs/11-api-reference.md](docs/11-api-reference.md)
- **Docker Docs**: https://docs.docker.com/

## 🆘 Obtener Ayuda

```bash
# Ver ayuda del script
./docker-test.sh help

# Ver comandos disponibles de SkyPanel
docker exec skypanel-dev /SkyPanel/bin/SkyPanel --help

# Ver versión
docker exec skypanel-dev /SkyPanel/bin/SkyPanel version
```

## 📝 Notas Importantes

1. **Primer inicio**: La primera construcción puede tardar 10-15 minutos
2. **Recursos**: Asegúrate de tener suficiente RAM y CPU disponible
3. **Puertos**: Los puertos 8080, 5657 y 8081 deben estar libres
4. **Datos**: Los datos se guardan en volúmenes persistentes
5. **Seguridad**: Cambia las contraseñas por defecto en producción

## 🎯 Próximos Pasos

1. ✅ Construir imagen: `./docker-test.sh build`
2. ✅ Iniciar contenedor: `./docker-test.sh start`
3. ✅ Crear admin: `./docker-test.sh admin`
4. ✅ Acceder al panel: http://localhost:8080
5. ✅ Crear tu primer servidor de juego

¡Disfruta de SkyPanel! 🚀
