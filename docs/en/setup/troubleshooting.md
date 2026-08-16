# Guía de Solución de Problemas (Troubleshooting)

Este documento detalla las soluciones a problemas comunes encontrados durante el despliegue y operación de Aether Panel.

---

## 1. Aislamiento con Unshare (Ubuntu 24.04+)

**Problema:** Error `Permission denied` al iniciar el "security jail" o el servidor de juego.

**Causa:** Ubuntu 24.04+ restringe por defecto el uso de *unprivileged user namespaces* (`unshare`), que el panel usa para aislar procesos de servidores de juego.

**Síntoma en logs:**
```
[ERROR] error starting server testserver: fork/exec /bin/bash: operation not permitted
```

#### Solutions:

*   **Opción A (Recomendada para producción):** Habilitar los namespaces en el kernel:
    ```bash
    sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
    ```
*   **Opción B (Desactivar aislamiento):** Agregar en `config.json`:
    ```json
    {
        "panel": {
            "security": {
                "disableUnshare": true
            }
        }
    }
    ```
    También se puede configurar por servidor en su JSON individual con `"disableUnshare": true` en la sección de entorno (tty).

---

## 2. Conexión a Docker

**Problema:** El panel no puede conectarse al motor Docker.

**Síntoma en logs:**
```
[ERROR] Cannot connect to the Docker daemon
```

**Causa:** El panel usa el SDK de Docker con `client.FromEnv()`, que lee las variables de entorno estándar de Docker.

#### Solutions:

*   **Verificar que Docker está corriendo:**
    ```bash
    docker info
    ```
*   **Verificar permisos del socket:** El usuario que ejecuta el panel debe tener permisos para acceder al socket de Docker:
    ```bash
    sudo usermod -aG docker $USER
    # Cerrar sesión y volver a entrar
    ```
*   **Usar Docker socket personalizado:** Configurar vía variable de entorno:
    ```bash
    export DOCKER_HOST=unix:///var/run/docker.sock
    ```
*   **Ejecutando el panel dentro de Docker:** El panel detecta automáticamente `SKYPANEL_PLATFORM=docker` y omite la verificación de Docker, continuando sin el motor Docker interno.

---

## 3. SFTP — Conexión y Autenticación

**Problema:** No se puede conectar via SFTP al panel.

**Puerto por defecto:** `5657`

#### Solutions:

**Error: `incorrect username or password`**
*   **Autenticación por base de datos:** El formato de usuario es `email#serverId` (ej: `user@example.com#abc123`). Verificar que el usuario tenga el permiso `ScopeServerSftp` asignado.
*   **Autenticación por OAuth2:** Verificar que el servidor de autenticación OAuth2 esté accesible y devuelva el scope `sftp` para el servidor correspondiente.

**Error: `error talking to auth server`**
*   Verificar que `daemon.auth.url` apunte a la URL correcta del panel (default: `http://localhost:8080`).
*   Verificar que `daemon.auth.clientId` y `daemon.auth.clientSecret` estén configurados.

**Error: `no access` / `invalid response from authorization server`**
*   El servidor OAuth2 rechazó las credenciales o el scope solicitado no está disponible.

**Error: `connection refused`**
*   El puerto SFTP (5657) no está abierto o el panel no está corriendo.
*   Verificar con: `ss -tlnp | grep 5657`

---

## 4. Base de Datos

**Problema:** Error de conexión a la base de datos al iniciar el panel.

**Dialectos soportados:** `sqlite3`, `mysql`, `postgresql`, `sqlserver`

**Síntomas comunes:**

*   `dial tcp 127.0.0.1:3306: connect: connection refused` — MySQL/MariaDB no está corriendo.
*   `could not load driver` — Dialecto incorrecto o controlador no compilado.

#### Solutions:

*   **Para SQLite (recomendado para pruebas):**
    ```json
    {
        "panel": {
            "database": {
                "dialect": "sqlite3",
                "url": "skypanel.db"
            }
        }
    }
    ```
*   **Para MySQL/MariaDB:** Verificar que el servicio esté corriendo y que el usuario tenga permisos:
    ```bash
    mysql -u skypanel -p -h 127.0.0.1 skypanel
    ```
*   **Para PostgreSQL:** Verificar `pg_hba.conf` permita conexiones desde `localhost`.
*   **Para SQL Server:** Verificar que TCP/IP esté habilitado en la configuración del servidor.

---

## 5. Puertos en Uso

**Problema:** Error `address already in use` al iniciar el panel.

**Puertos por defecto:**
| Servicio | Puerto | Config Key |
|----------|--------|------------|
| HTTP (Web) | `8080` | `web.host` |
| SFTP | `5657` | `daemon.sftp.host` |

#### Solutions:

*   **Verificar qué proceso está usando el puerto:**
    ```bash
    ss -tlnp | grep -E '8080|5657'
    ```
*   **Cambiar el puerto** en `config.json`:
    ```json
    {
        "web": {
            "host": "0.0.0.0:9090"
        },
        "daemon": {
            "sftp": {
                "host": "0.0.0.0:6565"
            }
        }
    }
    ```
*   También se puede cambiar vía variables de entorno:
    ```bash
    export SKYPANEL_WEB_HOST=0.0.0.0:9090
    export SKYPANEL_DAEMON_SFTP_HOST=0.0.0.0:6565
    ```

---

## 6. Permisos de Archivos (UID/GID)

**Problema:** Archivos creados por el panel tienen dueños incorrectos o errores de permisos.

**Comportamiento:** El panel asigna UID/GID a los archivos del servidor según el usuario configurado. Si el UID es `-1`, no se cambia la propiedad (usa el usuario del proceso).

#### Solutions:

*   **Verificar el UID/GID del proceso del panel:**
    ```bash
    ps aux | grep skypanel
    ```
*   **Los contenedores Docker** heredan el UID/GID del proceso del panel automáticamente.
*   **Entorno TTY con unshare:** El proceso dentro del jail se ejecuta como root (UID 0) mapeado al usuario real del sistema. Los archivos creados dentro del jail pertenecerán al usuario real fuera del jail.
*   **Si hay errores de permisos** al leer/escribir archivos del servidor, verificar que el usuario del panel tenga acceso a los directorios `servers/`, `cache/` y `binaries/`.

---

## 7. CORS — Conexiones desde el Frontend

**Problema:** El frontend no puede hacer peticiones a la API (errores CORS en consola del navegador).

**Comportamiento:** El panel permite **todos los orígenes** (`AllowOriginFunc` siempre retorna `true`). Esto es intencional para soportar despliegues donde el frontend y backend están en dominios separados.

#### Solutions:

*   Si hay errores CORS, verificar que el frontend esté usando la URL correcta de la API.
*   Verificar que los headers `Authorization` y `Content-Type` estén incluidos en las peticiones.
*   Si se usa un proxy reverso (nginx, Caddy) que modifica headers, asegurarse de que no quite los headers CORS.

---

## 8. Variables de Entorno y Configuración

**Problema:** El panel no usa los valores que configuraste.

**Comportamiento:** Todas las configuraciones de `config.json` se pueden sobrescribir con variables de entorno con prefijo `SKYPANEL_` y reemplazando `.` por `_`.

### Ejemplos:

| Variable de Entorno | Config JSON |
|---------------------|-------------|
| `SKYPANEL_WEB_HOST` | `web.host` |
| `SKYPANEL_DAEMON_SFTP_HOST` | `daemon.sftp.host` |
| `SKYPANEL_PANEL_DATABASE_URL` | `panel.database.url` |
| `SKYPANEL_PANEL_DATABASE_DIALECT` | `panel.database.dialect` |
| `SKYPANEL_LOGS` | `logs` |
| `SKYPANEL_PANEL_SETTINGS_COMPANYNAME` | `panel.settings.companyName` |

Las variables de entorno **tienen prioridad** sobre el archivo `config.json`.

---

## 9. Logs y Depuración

**Problema:** Necesitas más información para diagnosticar un error.

**Comportamiento:** El panel escribe logs en `logs/skypanel.log` con rotación automática al recibir `SIGUSR1`.

**Niveles de log:**
| Prefijo | Nivel | Destino |
|---------|-------|---------|
| `[ERROR]` | Error | Stderr + archivo |
| `[INFO]` | Info | Stdout + archivo |
| `[DEBUG]` | Debug | Stdout + archivo |
| `[SERVER]` | Servidor | Stdout + archivo |

#### Solutions:

*   **Ver logs en tiempo real:**
    ```bash
    tail -f logs/skypanel.log
    ```
*   **Forzar rotación de logs** (sin reiniciar el panel):
    ```bash
    kill -USR1 $(pidof SkyPanel)
    ```
*   **Aumentar nivel de detalle:** Iniciar con `GIN_MODE=debug` para ver todas las rutas HTTP:
    ```bash
    GIN_MODE=debug ./SkyPanel run
    ```

---

## 10. SSL/TLS (HTTPS)

**Problema:** Necesitas HTTPS para producción.

**Comportamiento:** El panel **no incluye soporte nativo para HTTPS**. Escucha únicamente en HTTP plano.

### Solución:

Usar un proxy reverso para terminación SSL (nginx, Caddy, Traefik):

**Ejemplo con Caddy:**
```caddyfile
panel.tudominio.com {
    reverse_proxy localhost:8080
}
```

**Ejemplo con nginx:**
```nginx
server {
    listen 443 ssl;
    server_name panel.tudominio.com;

    ssl_certificate /etc/ssl/certs/panel.crt;
    ssl_certificate_key /etc/ssl/private/panel.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Si usas un proxy de confianza, configurar en `config.json`:
```json
{
    "security": {
        "trustedProxies": ["127.0.0.1/32", "10.0.0.0/8"],
        "trustedProxyHeader": "X-Forwarded-For"
    }
}
```

---

## 11. Migraciones de Base de Datos

**Problema:** Error al ejecutar migraciones o el panel no arranca después de una actualización.

#### Solutions:

*   **Ejecutar migraciones manualmente:**
    ```bash
    ./SkyPanel db upgrade
    ```
    Este comando realiza un backup automático antes de migrar.
*   **Si la migración falla**, verificar que el usuario de base de datos tenga permisos para crear/modificar tablas.
*   **SQLite:** El archivo `skypanel.db` debe tener permisos de escritura para el usuario del panel.

---

## 12. Plantillas (Templates)

**Problema:** El panel carga el índice de plantillas pero falla al descargar plantillas individuales.

**Causa:** La URL configurada en `templates.url` apunta a un servidor que solo tiene el `templates.json` pero no los archivos JSON de cada plantilla.

**Solución:** Asegurarse de que el servidor de plantillas tenga la estructura completa. Si `templates.json` referencia `minecraft/minecraft.json`, ese archivo debe ser accesible en la misma ruta base.

---

## 13. AI Asistente (Google GenAI)

**Problema:** El asistente AI no responde o muestra errores.

**Causa:** No se ha configurado la API Key de Google Gemini.

**Solución:** Configurar en `config.json`:
```json
{
    "panel": {
        "settings": {
            "geminiApiKey": "tu-api-key-de-gemini"
        }
    }
}
```
O vía variable de entorno:
```bash
export SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY=tu-api-key-de-gemini
```

---

## 14. Archivo de Configuración

**Problema:** El panel no encuentra o ignora el archivo de configuración.

**Comportamiento:** Por defecto, el panel busca `config.json` en el directorio de trabajo actual. Se puede especificar una ruta personalizada con la flag `--config` o la variable de entorno `SKYPANEL_CONFIG`.

### Archivos de configuración incluidos:

*   `config.json` — Configuración principal (personalizable).
*   `config.docker.json` — Configución predefinida para entorno Docker.
*   `config.linux.json` — Configuración predefinida para Linux (SQLite local).

```bash
./SkyPanel run --config config.linux.json
```

---

## 15. Resolución de Problemas General

Si ninguna de las soluciones anteriores resuelve tu problema:

1.  **Revisar los logs completos:**
    ```bash
    cat logs/skypanel.log | grep ERROR
    ```
2.  **Verificar la versión del panel:**
    ```bash
    ./SkyPanel version
    ```
3.  **Verificar conectividad de red:** Asegurarse de que los puertos necesarios (8080, 5657) estén accesibles desde los clientes.
4.  **Verificar espacio en disco:** El panel necesita espacio para logs, caché de plantillas y servidores de juego.
5.  **Reportar el problema** en [Discord](https://discord.gg/aetherpanel) o abrir un issue en [GitHub](https://github.com/Aether-Panel/Panel/issues) incluyendo los logs relevantes.
