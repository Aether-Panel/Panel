# CLI (Command Line Interface)

El binario `SkyPanel` usa **Cobra** para la interfaz de línea de comandos.

## Uso

```bash
SkyPanel [--config archivo] [--workDir directorio] <comando>
```

## Comandos

### `run`

Inicia el servidor (panel HTTP + daemon). Es el comando principal de producción.

```bash
SkyPanel run
```

- Inicializa logging, BD, migrations
- Registra rutas HTTP
- Inicia servidores: HTTP (Gin), SFTP, Daemon
- Graceful shutdown con signal handling

### `version`

Muestra la versión del binario.

```bash
SkyPanel version
# → SkyPanel v3.x.x (ldflags)
```

La versión se inyecta con ldflags en build: `-X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Version=v3.x.x'`

### `user`

Gestión interactiva de usuarios.

#### `user add`

Crea un usuario nuevo con asistente interactivo o flags:

```bash
# Interactivo:
SkyPanel user add

# Con flags (útil para scripting en Docker):
SkyPanel user add --name admin --email admin@example.com --password secreto --admin
```

Flags: `--name`, `--email`, `--password`, `--admin`

Flujo:
1. Prompt por username (o flag)
2. Prompt por email (o flag)
3. Prompt por password con confirmación (o flag)
4. Confirmación de admin
5. Conecta a BD, crea usuario con `services.User.Create()`
6. Asigna scopes: siempre `login`, más `admin` si aplica

#### `user edit`

Editor interactivo para modificar usuarios existentes:

```bash
SkyPanel user edit
```

Opciones disponibles en el menú:
- Username
- Email
- Password
- Admin Status
- Remove 2FA
- Quit

### `db`

Operaciones de base de datos.

#### Subcomandos (definidos en `db.go`, `dbmigrate.go`, `dbupgrade.go`)

| Comando | Archivo | Descripción |
|---|---|---|
| `db migrate` | `dbmigrate.go` | Stub (sin `Run` implementado). Diseñado para migrar entre dialectos de BD en el futuro |
| `db upgrade` | `dbupgrade.go` | Ejecuta las migraciones de esquema (gormigrate + `AutoMigrate`). Con SQLite crea un backup `*.N.backup` y lo restaura si falla |

### `runService`

Comando interno para operaciones de systemd (ejecuta un servicio específico como subproceso).

```bash
SkyPanel runService <serviceName>
```

## Build

```bash
# Build estándar
go build -o SkyPanel ./cmd/panel

# Build con ldflags para versión
go build -ldflags "-X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Version=v3.0.0'" -o SkyPanel ./cmd/panel
```

## Docker

```bash
# Ejecutar comandos en contenedor en ejecución
docker exec -it <container> SkyPanel user add --name admin --email admin@example.com

# Ver logs
docker logs -f <container>
```
