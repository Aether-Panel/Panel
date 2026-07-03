# Documentación Docker

Esta guía detalla cómo desplegar Aether Panel utilizando Docker y Docker Compose.

## Requisitos Previos

- **Docker** 20.10+
- **Docker Compose** 2.0+

## Instalación Rápida

La forma recomendada de ejecutar Aether Panel es mediante Docker Compose.

### 1. Iniciar el Servicio

Ejecuta el siguiente comando en la raíz del proyecto (donde se encuentra `docker-compose.yml`):

```bash
docker-compose up -d
```

Esto descargará las imágenes necesarias, creará los volúmenes y levantará los servicios en segundo plano.

### 2. Crear Usuario Administrador

Una vez que el contenedor esté corriendo, necesitas crear un usuario administrativo para acceder al panel.

Ejecuta el siguiente comando:

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --password 'admin123' --admin
```

> **Importante:** Recuerda cambiar `admin@example.com` y `'admin123'` por tus credenciales seguras.

### 3. Acceder al Panel

El panel estará disponible en:
**http://localhost:8080**

---

## Configuración Técnica

### Puertos

El contenedor expone los siguientes puertos:

| Puerto | Servicio | Descripción |
|--------|----------|-------------|
| `8080` | Panel Web | Interfaz de usuario y API |
| `5657` | SFTP | Transferencia de archivos |

### Volúmenes (Persistencia)

Los datos se persisten utilizando volúmenes de Docker:

- `skypanel-data`: Almacena la base de datos (SQLite), configuraciones y datos de los servidores de juegos.
- `skypanel-config`: Almacena archivos de configuración específicos.

### Variables de Entorno

Puedes configurar el comportamiento del contenedor mediante variables de entorno en el archivo `docker-compose.yml`:

```yaml
environment:
  - PUFFER_PANEL_SETTINGS_COMPANYNAME=Aether Panel
  - PUFFER_WEB_HOST=0.0.0.0:8080
  - GIN_MODE=release
```

---

## Gestión del Contenedor

### Ver Logs
Para ver los logs del panel en tiempo real:

```bash
docker-compose logs -f
```

### Detener el Panel
Para detener los contenedores ordenadamente:

```bash
docker-compose down
```

### Reiniciar el Panel
```bash
docker-compose restart
```

### Actualizar Imagen
Para descargar la última versión y reiniciar:

```bash
docker-compose pull
docker-compose up -d
```

---

## Solución de Problemas

### Error: "Bind for 0.0.0.0:8080 failed: port is already allocated"
El puerto 8080 está ocupado por otro proceso. Edita el archivo `docker-compose.yml` y cambia el mapeo de puertos.
Por ejemplo, para usar el puerto 9000:
```yaml
ports:
  - "9000:8080"
```

### Error de Permisos con Docker
Si recibes errores de "permission denied" al intentar ejecutar docker:
1. Asegúrate de que tu usuario pertenece al grupo `docker`:
   ```bash
   sudo usermod -aG docker $USER
   ```
2. Cierra sesión y vuelve a entrar o ejecuta `newgrp docker`.

---

## Construcción Manual de la Imagen

Si prefieres construir la imagen Docker localmente desde el código fuente en lugar de descargarla:

```bash
docker-compose build
docker-compose up -d
```
