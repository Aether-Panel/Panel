# CLI (interfaz de línea de comandos)

Aether Panel se distribuye como un único binario de Go compilado desde `cmd/panel` con [Cobra](https://github.com/spf13/cobra). El mismo binario ejecuta el panel y el daemon, y tiene algunos comandos de administración (usuarios, base de datos). El comando raíz se sigue llamando `SkyPanel`.

Esta página explica cómo funciona la CLI y cuándo usar cada comando. La lista completa de comandos y flags, con ejemplos, está en la [Referencia de comandos CLI](../reference/14-cli-commands.md).

---

## Dónde está el binario

| Instalación | Binario | Configuración |
|-------------|---------|---------------|
| Imagen Docker (`docker-compose.yml`, el instalador) | `/SkyPanel/bin/SkyPanel` dentro del contenedor `skypanel` | `/etc/SkyPanel/config.json` dentro del contenedor, más las variables `SKYPANEL_*` del compose |
| Unidad de systemd (`internal/systemd/servicefiles/skypanel.service`) | `/usr/sbin/SkyPanel` | `--config /etc/SkyPanel/config.json` |
| Compilado a mano (`go build ./cmd/panel`, `make build`) | donde se coloque (`make build` compila el paquete `./cmd/panel` en `bin/skypanel`; `make run` ejecuta `go run ./cmd/panel run`) | véase [Carga de la configuración](#carga-de-la-configuración) |

Con Docker, cada comando se ejecuta dentro del contenedor:

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel <comando>
```

`-it` es necesario para las preguntas interactivas de `user add` y `user edit`.

---

## Carga de la configuración

Antes de ejecutar cualquier comando, el binario:

1. Cambia a `--workDir` si se indicó (se detiene con un error si la carpeta no existe).
2. Carga el archivo de configuración, en este orden: `--config`, la variable de entorno `SKYPANEL_CONFIG`, `/etc/SkyPanel/config.json` si existe y, si no, `config.json` en el directorio de trabajo. Si no puede leer el archivo, muestra `Error loading config, this may impact features` y continúa con los valores por defecto y las variables de entorno.

Cualquier clave puede venir también del entorno: `SKYPANEL_` más la clave en mayúsculas con los puntos convertidos en guiones bajos (`web.host` → `SKYPANEL_WEB_HOST`, `security.blockPrivateDownloads` → `SKYPANEL_SECURITY_BLOCKPRIVATEDOWNLOADS`). Claves y valores por defecto: [Configuración](./02-configuration.md).

---

## Arrancar el panel: `run` y `runService`

`run` es lo que arranca la imagen Docker (`exec /SkyPanel/bin/SkyPanel run` al final de su entrypoint). Está oculto en `SkyPanel --help`, pero funciona con normalidad. `runService` hace lo mismo y además se comunica con systemd.

Qué hace un arranque, según `panel.enable` y `daemon.enable` (ambos `true` por defecto):

**Panel** (`panel.enable`)
1. Carga el servicio de correo, pasa el SFTP a autenticación contra la base de datos y conecta los eventos de los servidores con las [extensiones](../reference/15-extensions.md).
2. Se conecta a la base de datos y **ejecuta las migraciones del esquema** (lo mismo que `db migrate`; `db upgrade` añade una barra de progreso y la copia de SQLite). Un fallo detiene el arranque.
3. Crea `panel.sessionKey` si está vacío y lo guarda; después prepara el almacén de cookies de sesión.

**Daemon** (`daemon.enable`)
1. Comprueba que el kernel admite `openat2`; si no, el proceso se niega a arrancar salvo que `security.forceOpenat` sea `true` (véase [Seguridad](./09-security.md#acceso-a-archivos)).
2. Arranca el servidor SFTP (`daemon.sftp.host`, por defecto `0.0.0.0:5657`).
3. Prepara los montajes de Docker, lanza en segundo plano la descarga previa de imágenes (`docker.prefetchImages`) y el historial de red del nodo.
4. Crea las carpetas de servidores, binarios y caché, añade la de binarios al `PATH`, carga todos los servidores y pone en cola los que tienen arranque automático.

Después escucha HTTP en `web.host` (por defecto `0.0.0.0:8080`). Si el panel está desactivado y `token.public` está vacío, deja un aviso en el log, porque ese nodo no podrá validar los tokens del panel.

En Docker (`SKYPANEL_PLATFORM=docker`, que fija la imagen) un daemon que no arranca no detiene el proceso: el panel web sigue funcionando sin él. En otros entornos el arranque se aborta.

**Apagado** (SIGINT o SIGTERM): las peticiones HTTP en curso disponen de hasta 10 segundos, se detiene el servidor SFTP, se pide a todos los servidores de juego que se detengan **a la vez** (cada uno tiene hasta un minuto antes de forzarlo) y se cierran las conexiones a la base de datos. `docker-compose.yml` concede 90 segundos al contenedor (`stop_grace_period`) para ello.

`runService` envía `READY=1` a `$NOTIFY_SOCKET` al terminar de arrancar y `STOPPING=1` al apagarse, para unidades con `Type=notify`. Sin `NOTIFY_SOCKET` se comporta como `run`.

---

## Gestión de usuarios

### Crear usuarios: `user add`

Crea una cuenta directamente en la base de datos, con el scope `login` y, opcionalmente, `admin`. Es la forma de crear el primer administrador (el instalador lo hace automáticamente):

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add \
  --name admin --email admin@example.com --admin
```

Cómo se combinan flags y preguntas:

- Si **no** se indica ninguno de `--name`, `--email` o `--password`, todo se pregunta de forma interactiva, incluido «Set as Admin» (en este modo decide la pregunta, no `--admin`).
- Si se indica **alguno**, el comando funciona en modo flags: los valores que falten se siguen preguntando, pero un valor dado en la línea de comandos que no supera la validación termina el comando con código 1 en lugar de volver a preguntar. `--admin` se toma del flag.
- Cuando se pregunta la contraseña, se pide dos veces.

La validación es la misma que en la web: nombre de usuario obligatorio, ASCII imprimible, de 1 a 100 caracteres; un email válido de hasta 255 caracteres; una contraseña de 8 a 72 **bytes** (el límite de bcrypt; las letras acentuadas y los emoji ocupan más de un byte).

`--force` borra cualquier usuario existente con el mismo email **y** cualquiera con el mismo nombre de usuario antes de crear el nuevo. Sirve para restablecer una cuenta de administrador, teniendo en cuenta que borra de verdad la cuenta anterior: con ella se van sus permisos, clientes OAuth2, sesiones, códigos de recuperación, claves SSH y preferencias, y los servidores que poseía quedan sin propietario.

### Editar usuarios: `user edit`

Totalmente interactivo, sin flags. Pide un **nombre de usuario** (no un email) y después muestra un menú hasta elegir **Quit**:

| Opción | Efecto |
|--------|--------|
| Username | Cambia el nombre de usuario |
| Email | Cambia el email |
| Password | Fija una contraseña nueva (8–72 bytes) y **cierra todas las sesiones abiertas** de ese usuario |
| Admin Status | `yes` añade `admin`, `no` lo quita, `cancel` no hace nada; `login` se conserva siempre |
| Remove 2FA | Desactiva la verificación en dos pasos, para quien perdió su dispositivo y sus códigos de recuperación |
| Quit | Sale |

`user edit` solo se ejecuta como `root` o como miembro del grupo `SkyPanel`; cualquier otro recibe «You do not have permission to use this command». En la imagen Docker el proceso corre como el usuario `SkyPanel`, así que `docker exec` funciona. Un nuevo nombre de usuario o email se comprueba con las mismas reglas que la API y `user add` (nombre obligatorio, ASCII imprimible, de 1 a 100 caracteres; un email válido de hasta 255 caracteres); un valor rechazado muestra el error, no se guarda y el usuario conserva el anterior. No tener permiso, una base de datos inaccesible o un nombre de usuario desconocido terminan el comando con código 1.

---

## Base de datos: `db upgrade`

Las migraciones ya se ejecutan en cada arranque de `run`/`runService`, así que `db upgrade` solo hace falta para migrar sin arrancar el panel (por ejemplo, antes de cambiar de binario o para ver la barra de progreso).

- Con el panel desactivado (`panel.enable=false`) muestra «Panel not enabled, skipping upgrade» y termina con 0.
- Con **SQLite**, primero copia el archivo de la base de datos a `<archivo>.<N>.backup`, con el primer `N` libre (`skypanel.db.0.backup`, `skypanel.db.1.backup`…). Si la actualización falla, se restaura esa copia.
- Con MySQL/MariaDB, PostgreSQL o SQL Server no hay copia automática: haga antes una copia de la base de datos.
- Código de salida 1 ante cualquier fallo.

`db migrate` aplica las migraciones pendientes igual que `run` (sin barra de progreso ni copia de SQLite) y termina: con 0 si todo va bien o si el panel está desactivado, con 1 si no se puede conectar con la base de datos o falla una migración. El entrypoint de Docker lo ejecuta antes de `run`, así que una migración fallida se notifica por separado antes de que arranque el panel.

Cualquier comando de la CLI que falle (un comando o flag desconocido, argumentos incorrectos, un error devuelto por `user add`, `user edit`, `db migrate` o `db upgrade`) termina con código 1; un pánico inesperado termina con 2. Tabla completa: [Referencia de comandos CLI → Códigos de salida](../reference/14-cli-commands.md#códigos-de-salida).

---

## Versión y compilación

`SkyPanel version` muestra `SkyPanel <versión> (<hash>)`. Ambos valores se fijan al compilar; un `go build` sin más da `SkyPanel nightly (unknown)`:

```bash
go build -ldflags "\
  -X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Version=v3.0.0' \
  -X 'github.com/SkyPanel/SkyPanel/v3/pkg/skypanel.Hash=$(git rev-parse --short HEAD)'" \
  -o SkyPanel ./cmd/panel
```

El Dockerfile fija además `internal/config.curseforgeKey` (la clave por defecto de la API de CurseForge) a partir de un argumento de compilación. El módulo requiere Go 1.26 (`go.mod`) y antes hay que compilar el frontend: `client/frontend/dist` va incrustado en el binario.

---

## Relacionado

- [Referencia de comandos CLI](../reference/14-cli-commands.md)
- [Configuración](./02-configuration.md)
- [Base de datos](./03-database.md)
- [Seguridad](../general/security.md) (recuperación de la verificación en dos pasos con `user edit`)
