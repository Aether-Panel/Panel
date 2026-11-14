# Referencia de comandos CLI

Todos los comandos y flags del binario `SkyPanel` (`cmd/panel`). Para saber qué hace cada comando en detalle, cómo se carga la configuración y cuándo usarlos, véase [CLI](../backend/07-cli.md).

```text
SkyPanel [--workDir <dir>] [--config <archivo>] <comando> [subcomando] [flags]
```

En Docker, anteponga a cada comando `docker exec -it skypanel /SkyPanel/bin/SkyPanel`.

---

## Flags globales

Disponibles en todos los comandos.

| Flag | Por defecto | Descripción |
|------|-------------|-------------|
| `--workDir <dir>` | directorio actual | Directorio al que cambiar antes de nada |
| `--config <archivo>` | `SKYPANEL_CONFIG`, después `/etc/SkyPanel/config.json` si existe y después `config.json` | Archivo de configuración |
| `-h`, `--help` | | Ayuda del comando (la añade Cobra) |

No existe un flag `--version`; use `version`.

---

## Comandos

| Comando | Descripción |
|---------|-------------|
| `run` | Arranca el panel y/o el daemon. Oculto en `--help`. |
| `runService` | Igual que `run`, con mensajes `sd_notify` para systemd |
| `version` | Muestra la versión |
| `user add` | Crea un usuario |
| `user edit` | Edita un usuario de forma interactiva |
| `db upgrade` | Ejecuta las migraciones de la base de datos, con barra de progreso y copia de SQLite |
| `db migrate` | Ejecuta las migraciones pendientes de la base de datos, igual que `run` |
| `help [comando]` | Comando de ayuda de Cobra |

`user` y `db` por sí solos solo muestran la lista de sus subcomandos. El autocompletado de shell (`completion`) está desactivado.

---

## `run`

```bash
SkyPanel run
```

Sin flags propios. Arranca lo que activen `panel.enable` y `daemon.enable`, ejecuta las migraciones de la base de datos cuando el panel está activo y escucha en `web.host` hasta recibir SIGINT/SIGTERM. Detalles: [Arrancar el panel](../backend/07-cli.md#arrancar-el-panel-run-y-runservice).

## `runService`

```bash
SkyPanel runService --config /etc/SkyPanel/config.json
```

Sin flags propios. Como `run`; envía `READY=1` y `STOPPING=1` a `$NOTIFY_SOCKET` (unidades `Type=notify`).

## `version`

```bash
SkyPanel version
# SkyPanel nightly (unknown)
```

Muestra `SkyPanel <versión> (<hash>)`; ambos se fijan con `-ldflags` al compilar.

## `user add`

```bash
SkyPanel user add [--name <usuario>] [--email <email>] [--password <contraseña>] [--admin] [--force]
```

| Flag | Tipo | Descripción |
|------|------|-------------|
| `--name` | texto | Nombre de usuario: obligatorio, ASCII imprimible, de 1 a 100 caracteres |
| `--email` | texto | Email, hasta 255 caracteres |
| `--password` | texto | Contraseña, de 8 a 72 bytes. Si no se indica, se pide dos veces |
| `--admin` | booleano | Concede también el scope `admin` (se ignora en el modo totalmente interactivo, donde lo decide una pregunta) |
| `--force` | booleano | Borra antes cualquier usuario con el mismo email o nombre de usuario y después lo crea |

El usuario recibe siempre el scope `login`. Los valores que falten se preguntan; si se indica al menos uno de `--name`, `--email` o `--password`, un valor no válido pasado como flag termina el comando con código 1. Véase [Crear usuarios](../backend/07-cli.md#crear-usuarios-user-add).

```bash
# Primer administrador, sin preguntas (la contraseña queda en el historial de la shell)
SkyPanel user add --name admin --email admin@example.com --password 'una-contraseña-larga' --admin

# Totalmente interactivo
SkyPanel user add
```

## `user edit`

```bash
SkyPanel user edit
```

Sin flags. Pide un nombre de usuario y después ofrece: **Username**, **Email**, **Password** (cierra las sesiones del usuario), **Admin Status** (`yes`/`no`/`cancel`), **Remove 2FA** y **Quit**. Solo puede ejecutarlo `root` o un miembro del grupo `SkyPanel`. Un nuevo nombre de usuario o correo debe superar las mismas comprobaciones que en la API (nombre obligatorio, ASCII imprimible, de 1 a 100 caracteres; un correo válido de hasta 255 caracteres); un valor rechazado no se guarda y el usuario conserva el anterior. No tener permiso, una base de datos inaccesible o un nombre de usuario desconocido terminan el comando con código 1. Véase [Editar usuarios](../backend/07-cli.md#editar-usuarios-user-edit).

## `db upgrade`

```bash
SkyPanel db upgrade
```

Sin flags. Ejecuta las migraciones del esquema con una barra de progreso. Con SQLite guarda antes `<archivo>.<N>.backup` y lo restaura si la actualización falla. Se omite (código 0) cuando `panel.enable` es `false`; código 1 si falla.

## `db migrate`

```bash
SkyPanel db migrate
```

Sin flags ni argumentos. Aplica las migraciones pendientes del esquema, el mismo paso que da `run` antes de arrancar, sin barra de progreso ni copia de SQLite. Muestra `Database migrations complete` y termina con 0; se omite (código 0, «Panel not enabled, skipping migrations») cuando `panel.enable` es `false`; termina con 1 si no se puede conectar con la base de datos o falla una migración. El entrypoint de Docker lo ejecuta antes de `run`.

---

## Códigos de salida

| Código | Cuándo |
|--------|--------|
| `0` | Éxito, o un comando que solo muestra la ayuda |
| `1` | Cualquier comando que falle: un comando o flag desconocido, argumentos incorrectos, un `user add` o `user edit` que falla (valor de flag no válido, base de datos inaccesible, usuario desconocido, sin permiso), un fallo de `db upgrade` o `db migrate` |
| `2` | Un pánico inesperado (la traza se escribe en el log) |

Cobra muestra el error como `Error: <mensaje>`.

---

## Resumen rápido

```bash
SkyPanel version
SkyPanel user add --name admin --email admin@example.com --admin
SkyPanel user edit
SkyPanel db upgrade
SkyPanel --workDir /var/lib/SkyPanel --config /etc/SkyPanel/config.json run

# Docker
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin
docker compose logs -f skypanel
```
