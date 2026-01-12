# 🐳 SkyPanel - Configuración Docker Completa

## ✅ Archivos Creados

He creado una configuración completa de Docker para que puedas probar SkyPanel fácilmente:

### 📄 Archivos Principales

1. **`docker-compose.yml`** - Configuración de producción
2. **`docker-compose.dev.yml`** - Configuración de desarrollo/pruebas
3. **`docker-test.sh`** - Script completo de gestión
4. **`quickstart-docker.sh`** - Script de inicio rápido
5. **`README.docker.md`** - Documentación completa
6. **`.dockerignore`** - Optimización de build

---

## 🚀 Inicio Rápido (3 Pasos)

### Opción 1: Script Automático (Más Fácil)

```bash
# Ejecutar todo automáticamente
./quickstart-docker.sh
```

Este script hará:
- ✅ Verificar Docker
- ✅ Construir la imagen (10-15 min)
- ✅ Iniciar el contenedor
- ✅ Mostrar URLs de acceso

### Opción 2: Paso a Paso

```bash
# 1. Construir imagen
./docker-test.sh build

# 2. Iniciar contenedor
./docker-test.sh start

# 3. Crear usuario admin
./docker-test.sh admin
```

---

## 🎯 Acceso al Panel

Una vez iniciado:

- **Panel Web**: http://localhost:8080
- **Gatus (Monitoring)**: http://localhost:8081
- **SFTP**: localhost:5657

---

## 📋 Comandos Disponibles

### Script de Gestión (`docker-test.sh`)

```bash
./docker-test.sh build      # Construir imagen
./docker-test.sh start      # Iniciar contenedor
./docker-test.sh stop       # Detener contenedor
./docker-test.sh restart    # Reiniciar
./docker-test.sh logs       # Ver logs en tiempo real
./docker-test.sh status     # Ver estado
./docker-test.sh shell      # Abrir shell
./docker-test.sh admin      # Crear usuario admin
./docker-test.sh clean      # Limpiar todo
./docker-test.sh rebuild    # Reconstruir
./docker-test.sh help       # Ver ayuda
```

### Docker Compose Directo

```bash
# Desarrollo
docker-compose -f docker-compose.dev.yml up -d
docker-compose -f docker-compose.dev.yml logs -f
docker-compose -f docker-compose.dev.yml down

# Producción
docker-compose up -d
docker-compose logs -f
docker-compose down
```

---

## 📊 Estructura de Datos

### Modo Desarrollo
```
dev-data/
├── config/     # Configuración
├── data/       # Base de datos y servidores
└── logs/       # Logs
```

### Modo Producción
```
Volúmenes Docker:
- skypanel-config
- skypanel-data
- skypanel-logs
```

---

## 🔧 Configuración

### Puertos Expuestos

- **8080** - Panel Web
- **5657** - SFTP
- **8081** - Gatus (Monitoring)

### Variables de Entorno

Edita `docker-compose.yml` o `docker-compose.dev.yml`:

```yaml
environment:
  - GIN_MODE=release
  - PUFFER_WEB_HOST=0.0.0.0:8080
  - PUFFER_PANEL_REGISTRATIONENABLED=true
  - PUFFER_PANEL_SETTINGS_COMPANYNAME=SkyPanel
```

---

## 🛠️ Gestión del Contenedor

### Ver Logs

```bash
# Con el script
./docker-test.sh logs

# Directo
docker logs -f skypanel-dev
```

### Ejecutar Comandos

```bash
# Shell interactivo
./docker-test.sh shell

# Comando específico
docker exec skypanel-dev /SkyPanel/bin/SkyPanel version
```

### Crear Usuario Admin

```bash
# Con el script (interactivo)
./docker-test.sh admin

# Manualmente
docker exec -it skypanel-dev /SkyPanel/bin/SkyPanel user add \
  --email admin@example.com \
  --password tu-contraseña \
  --admin
```

---

## 🔄 Backup y Restauración

### Hacer Backup

```bash
# Desarrollo (archivos locales)
tar -czf backup-$(date +%Y%m%d).tar.gz dev-data/

# Producción (volúmenes Docker)
docker run --rm \
  -v skypanel-data:/data \
  -v $(pwd):/backup \
  alpine tar -czf /backup/backup-$(date +%Y%m%d).tar.gz /data
```

### Restaurar Backup

```bash
# Desarrollo
tar -xzf backup-YYYYMMDD.tar.gz

# Producción
docker run --rm \
  -v skypanel-data:/data \
  -v $(pwd):/backup \
  alpine tar -xzf /backup/backup-YYYYMMDD.tar.gz -C /
```

---

## 🐛 Solución de Problemas

### El contenedor no inicia

```bash
# Ver logs
docker logs skypanel-dev

# Verificar configuración
docker-compose -f docker-compose.dev.yml config

# Verificar puertos
sudo netstat -tulpn | grep -E '8080|5657|8081'
```

### Error de permisos de Docker

```bash
# Agregar usuario al grupo docker
sudo usermod -aG docker $USER
newgrp docker

# Verificar
docker ps
```

### No puedo acceder al panel

```bash
# Verificar que está corriendo
./docker-test.sh status

# Verificar conectividad
curl http://localhost:8080

# Ver logs
./docker-test.sh logs
```

---

## 🧹 Limpieza

### Limpiar Todo

```bash
# Con el script (interactivo)
./docker-test.sh clean

# Manualmente
docker-compose -f docker-compose.dev.yml down -v
docker rmi skypanel:latest
rm -rf dev-data/
```

### Solo Detener

```bash
./docker-test.sh stop
```

---

## 📚 Documentación Completa

Para más detalles, consulta:

```bash
# Ver documentación completa
cat README.docker.md

# Ver ayuda del script
./docker-test.sh help
```

---

## 🎯 Próximos Pasos

1. **Iniciar**: `./quickstart-docker.sh`
2. **Crear Admin**: `./docker-test.sh admin`
3. **Acceder**: http://localhost:8080
4. **Crear Servidor**: Desde el panel web
5. **Monitorear**: http://localhost:8081

---

## 💡 Tips

- **Primera construcción**: Tarda 10-15 minutos
- **Datos persistentes**: Se guardan en `dev-data/` o volúmenes Docker
- **Logs en tiempo real**: `./docker-test.sh logs`
- **Reinicio rápido**: `./docker-test.sh restart`
- **Limpiar y empezar de nuevo**: `./docker-test.sh clean && ./quickstart-docker.sh`

---

## 🆘 Ayuda

Si tienes problemas:

1. Verifica que Docker esté corriendo: `docker ps`
2. Revisa los logs: `./docker-test.sh logs`
3. Verifica el estado: `./docker-test.sh status`
4. Lee la documentación: `cat README.docker.md`

---

¡Disfruta probando SkyPanel en Docker! 🚀
