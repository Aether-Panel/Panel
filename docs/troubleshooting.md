# Guía de Solución de Problemas (Troubleshooting)

Este documento detalla las soluciones a problemas comunes encontrados durante el despliegue de Aether Panel, especialmente en entornos modernos como Ubuntu 24.04.

## 1. Compatibilidad con Ubuntu 24.04 (Seguridad)

**Problema:** Error `Permission denied` al intentar iniciar el "security jail" o procesos.
**Causa:** Ubuntu 24.04 restringe por defecto el uso de *unprivileged user namespaces* (unshare), una característica que el panel usa para aislar procesos.

### Soluciones:

*   **Opción A (Recomendada para producción):** Habilitar los namespaces en el kernel ejecutando:
    ```bash
    sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
    ```
*   **Opción B (Configuración del panel):** Desactivar la característica de aislamiento en el archivo `config.json`:
    ```json
    "panel": {
        "security": {
            "disableUnshare": true
        }
    }
    ```

## 2. Rutas y Permisos (Portabilidad)

**Problema:** Errores de `mkdir /home/esteban: permission denied` o `sftp.key: no such file or directory`.
**Causa:** Uso de rutas absolutas hardcodeadas (ej: `/home/esteban/Descargas/...`) que no existen en el servidor de destino.

**Solución:** Utilizar rutas relativas en el archivo `config.json`. Esto permite que el panel funcione en cualquier directorio sin importar el usuario del sistema.
*   **Mal:** `"root": "/home/esteban/Descargas/pufferpanel/data"`
*   **Bien:** `"root": "data"`

## 3. Errors 404 en Plantillas (Templates)

**Problema:** El panel lee el índice `templates.json` pero falla al cargar plantillas individuales con error 404.
**Causa:** Solo se ha subido el archivo `templates.json` al servidor web, pero no las carpetas y archivos JSON de cada plantilla.

**Solución:** El servidor de plantillas debe tener la estructura completa. Si `templates.json` referencia `eco9/eco9.json`, el archivo debe ser accesible en esa ruta exacta. No basta con subir el índice.

## 4. Conexión a Base de Datos

**Problema:** Error `dial tcp 127.0.0.1:3306: connect: connection refused`.
**Causa:** El panel intenta conectar a MySQL localmente pero el servicio no está corriendo o la configuración de red es incorrecta.

### Soluciones:
*   **Para MySQL:** Asegúrate de usar la IP correcta (ej: la IP de la máquina o `localhost`) y que el usuario tenga permisos de acceso remoto si es necesario.
*   **Para SQLite (Fácil despliegue):** Cambia el dialecto a `sqlite3` para evitar depender de un servidor externo:
    ```json
    "database": {
        "dialect": "sqlite3",
        "url": "skypanel.db"
    }
    ```
