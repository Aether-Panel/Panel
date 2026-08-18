# Referencia de Comandos CLI

Aether Panel incluye una CLI construida con **Cobra** que permite gestionar el panel, crear usuarios, ejecutar migraciones y más.

> **Nota:** El binario se compila desde `cmd/panel/main.go`. El nombre interno del comando raíz es `SkyPanel`, pero el archivo binario se genera como `skypanel` (o `skypanel.exe` en Windows).

---

## 0. Uso con Docker

Si ejecutas el panel dentro de un contenedor Docker (imagen `ghcr.io/aether-panel/panel`), no puedes ejecutar el binario directamente en el host. Usa `docker exec` para interactuar con la CLI del contenedor en ejecución.

### Ejecutar Comandos en el Contenedor

```bash
# Acceder al shell del contenedor
docker exec -it skypanel sh

# O ejecutar comandos directamente
docker exec -it skypanel /SkyPanel/bin/SkyPanel version
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --admin
docker exec -it skypanel /SkyPanel/bin/SkyPanel db upgrade
```

> **Ruta del binario dentro del contenedor:** `/SkyPanel/bin/SkyPanel`

### Usando docker-compose

Con el archivo `docker-compose.yml` incluido en el repositorio:

```bash
# Iniciar servicios
docker compose up -d

# Ejecutar CLI en el contenedor en ejecución
docker compose exec skypanel /SkyPanel/bin/SkyPanel user add --admin

# Ver logs
docker compose logs -f skypanel
```

### Ejemplo: Crear Admin en Docker

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add \
  --name "admin" \
  --email "admin@example.com" \
  --admin
```

### Ejemplo: Migrar Base de Datos en Docker

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel db upgrade
```

### Notas para Docker

- El binario dentro del contenedor está en `/SkyPanel/bin/SkyPanel` (con S mayúscula).
- La config dentro del contenedor está en `/etc/SkyPanel/config.json`.
- Los datos persistentes están en `/var/lib/SkyPanel/`.
- Puedes verificar que el contenedor esté funcionando con `docker ps` antes de ejecutar comandos.

---

## 1. Compilación (sin Docker)

### Requisitos Previos
- Go 1.25+
- Acceso a `cmd/panel/main.go`

### Build Básico
```bash
go build -o skypanel ./cmd/panel
```

### Build con Versión Personalizada
```bash
go build -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Version=1.2.0' -X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Hash=$(git rev-parse --short HEAD)'" -o skypanel ./cmd/panel
```

### Usando Makefile
```bash
make build    # Genera bin/skypanel
make run      # Ejecuta go run ./cmd/panel/main.go
```

---

## 2. Comandos Disponibles

| Comando | Descripción |
|---------|-------------|
| `run` | Inicia el panel completo (web, daemon SFTP, planificador) |
| `runService` | Inicia como servicio systemd con notificación `NOTIFY_SOCKET` |
| `version` | Muestra la versión del panel |
| `user` | Gestión de usuarios (subcomandos: `add`, `edit`) |
| `db` | Operaciones de base de datos (subcomandos: `upgrade`, `migrate`) |

### Flags Globales

| Flag | Descripción |
|------|-------------|
| `--workDir` | Cambia el directorio de trabajo antes de iniciar |
| `--config` | Ruta al archivo de configuración JSON |

Ambos flags están disponibles en **todos** los subcomandos.

---

## 3. `run` — Iniciar el Panel

```bash
./skypanel run
```

Inicializa y arranca todos los servicios del panel:

1. Carga la configuración desde `config.json`.
2. Conecta a la base de datos (SQLite, MySQL, PostgreSQL o SQL Server).
3. Inicia el **servidor HTTP** (API REST + panel web) en el puerto configurado (default `8080`).
4. Inicia el **servidor SFTP** (puerto default `5657`).
5. Inicia el **planificador de tareas** (gocron).
6. Inicia la gestión de **servidores de juego** (carga servidores existentes, inicia el daemon).
7. Muestra logs en consola en tiempo real.

```bash
# Usar directorio y config personalizados
./skypanel run --workDir /opt/skypanel --config /opt/skypanel/production.json
```

---

## 4. `runService` — Servicio systemd

```bash
./skypanel runService
```

Idéntico a `run`, pero además notifica a systemd vía `NOTIFY_SOCKET`:
- Envía `READY=1` cuando el panel está listo.
- Envía `STOPPING=1` durante el apagado.

Útil para integrar con unidades systemd tipo `Type=notify`.

---

## 5. `version` — Versión

```bash
./skypanel version
```

Muestra la versión del panel. Por defecto: `SkyPanel nightly (unknown)`. Se puede personalizar en tiempo de compilación con `-ldflags` (ver sección 1).

---

## 6. `user` — Gestión de Usuarios

### 6.1. `user add` — Crear Usuario

```bash
./skypanel user add --name "admin" --email "admin@example.com" --admin --password "clave_segura"
```

Todos los flags son **opcionales**. Si se omite alguno, el sistema lo solicitará de forma interactiva:

| Flag | Descripción |
|------|-------------|
| `--name` | Nombre de usuario |
| `--email` | Correo electrónico |
| `--admin` | Otorga permisos de administrador |
| `--password` | Contraseña (si se omite, se solicita con confirmación) |

El comando:
1. Valida el formato del username, email y fortaleza de la contraseña.
2. Conecta a la base de datos.
3. Crea el usuario con permisos `ScopeLogin` + `ScopeAdmin` (si `--admin`).

### 6.2. `user edit` — Editar Usuario (Interactivo)

```bash
./skypanel user edit
```

Comando **totalmente interactivo** (sin flags). Pasos:

1. Solicita el nombre de usuario a editar.
2. Muestra un menú interactivo con opciones:

   | Opción | Acción |
   |--------|--------|
   | **Username** | Cambiar nombre de usuario |
   | **Email** | Cambiar correo electrónico |
   | **Password** | Cambiar contraseña |
   | **Admin Status** | Agregar o quitar permisos de administrador |
   | **Remove 2FA** | Deshabilitar autenticación de dos factores |
   | **Quit** | Salir |

3. Permite realizar múltiples cambios en la misma sesión.

---

## 7. `db` — Base de Datos

### 7.1. `db upgrade` — Migraciones

```bash
./skypanel db upgrade
```

Ejecuta las migraciones de esquema de base de datos. Útil después de actualizar el panel a una nueva versión.

- **SQLite:** Realiza un backup automático del archivo (`skypanel.db.0.backup`, `skypanel.db.1.backup`, ...) antes de migrar.
- **Otros dialectos:** Ejecuta migraciones directamente.
- Si la migración falla, restaura automáticamente el backup (SQLite).

### 7.2. `db migrate` — Cambiar de Motor (Experimental)

```bash
./skypanel db migrate
```

> **Nota:** Este comando actualmente es un **stub** y no ejecuta ninguna acción. Está diseñado para migrar datos entre dialectos de base de datos (ej: SQLite → MySQL) en el futuro.

---

## Resumen Rápido

```bash
# Compilar
go build -o skypanel ./cmd/panel

# Versión
./skypanel version

# Crear admin
./skypanel user add --name admin --email admin@example.com --admin

# Editar usuario (interactivo)
./skypanel user edit

# Iniciar panel
./skypanel run

# Iniciar como servicio
./skypanel runService

# Migrar base de datos
./skypanel db upgrade

# Con flags globales
./skypanel --workDir /data/skypanel --config /data/skypanel/config.json run
```
