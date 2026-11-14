# Aislamiento del kernel: openat2 y unshare

Dos funciones del kernel de Linux protegen el nodo de lo que se ejecuta en él:

- **`openat2`** mantiene cada archivo que el daemon abre para un servidor dentro de la carpeta de ese servidor.
- **`unshare`** (espacios de nombres de usuario y de montaje) encierra los servidores que se ejecutan directamente en el nodo, con el entorno `host`.

Los servidores del entorno `docker`, el predeterminado, los aísla Docker: ver [Usuario y aislamiento del contenedor](../backend/04-servers.md#usuario-y-aislamiento-del-contenedor). Cómo usa `openat2` el paquete `files` se explica en [Archivos del servidor y archivos comprimidos](19-files-compression.md).

## openat2

### Comprobación del kernel al arrancar

Al arrancar el daemon (`utils.DetermineKernelSupport()`, llamada desde `daemon()` en `cmd/panel/run.go`), se comprueba que el kernel tenga `openat2` (añadido en Linux 5.6):

1. Prueba la llamada tal como la usa el servidor de archivos: abre `/` y, desde ahí, `.` con `openat2` y `RESOLVE_BENEATH`. Si funciona, `openat2` está disponible.
2. `ENOSYS` (el kernel no la tiene), `EOPNOTSUPP` o `EPERM` significan que no está disponible. `EPERM` es lo que devuelve un filtro seccomp que la bloquea (por ejemplo, un perfil de Docker antiguo): con él, cada acceso a archivos fallaría igual, así que cuenta como no disponible aunque el kernel la tenga.
3. Cualquier otro error deja la prueba sin resultado y se consulta `/proc/kallsyms`, buscando el símbolo `do_sys_openat2` (sea cual sea su tipo, `t` o `T`). Si el archivo no se puede abrir, `openat2` cuenta como no disponible.
4. Si `openat2` no está disponible, el daemon **no arranca**:

```
OpenAt2 is not supported. Cowardly not starting to avoid security issues.
```

Para arrancar de todos modos hay que activar `security.forceOpenat` (ver más abajo).

### Para qué se usa

Con `openat2`, cada apertura del servidor de archivos usa `RESOLVE_BENEATH`, relativa a la carpeta del servidor abierta una sola vez. El kernel rechaza entonces cualquier ruta, `..` o enlace simbólico que lleve fuera de esa carpeta, también cuando una carpeta se sustituye por un enlace mientras la petición está en curso.

### Alternativa: `security.forceOpenat`

| Clave | Variable de entorno | Por defecto |
|-------|---------------------|-------------|
| `security.forceOpenat` | `SKYPANEL_SECURITY_FORCEOPENAT` | `false` |

Con `true`, el daemon se salta la comprobación, escribe en el log `WARNING: OPENAT2 SUPPORT NOT ENABLED. OVERRIDE IS BEING USED.` y usa `openat` normal: cada parte de la ruta se abre una a una con `O_NOFOLLOW`. Sigue sin salir de la carpeta, pero **cualquier** enlace simbólico en el camino falla, aunque apunte dentro del servidor. Úsela solo en kernels sin `openat2` o donde un filtro seccomp la bloquee y no se pueda cambiar.

No hay alternativa automática: sin `openat2` y sin esta clave, el daemon no arranca.

### `internal/sys`

`sys.SyscallMode` convierte un `os.FileMode` de Go en el modo que esperan las llamadas al sistema, conservando los bits de permisos y los bits setuid, setgid y sticky. El servidor de archivos lo usa en `openat2`, `openat` y `mkdirat`.

## unshare: la jaula del entorno `host`

Los servidores con el entorno `host` (los nombres antiguos `tty` y `standard` se cambian a `host` al cargar el servidor) se ejecutan como procesos del daemon, sin contenedor. Salvo que se desactive, cada uno arranca dentro de una jaula (`internal/servers/tty/tty.go`).

### Cómo se construye la jaula

El daemon escribe un pequeño script de Bash en una carpeta temporal (`/tmp/unshare-pp-*`) y lo ejecuta en espacios de nombres nuevos:

| Espacio de nombres | Indicador |
|--------------------|-----------|
| Usuario | `CLONE_NEWUSER` |
| Montaje | `CLONE_NEWNS` |
| Cgroup | `CLONE_NEWCGROUP` |
| IPC | `CLONE_NEWIPC` |
| Nombre de equipo (UTS) | `CLONE_NEWUTS` |

El proceso tiene además su propia tabla de descriptores de archivo (`CLONE_FILES`). **No** hay espacio de nombres de PID ni de red: el servidor usa la red del nodo y su `/proc` es el del nodo.

Mapeo de usuarios: el usuario y el grupo `0` dentro de la jaula son el usuario y el grupo del propio daemon fuera (un mapeo de un solo ID). El servidor cree que es root, pero en el nodo solo tiene los permisos del usuario del daemon, y los archivos que crea pertenecen a ese usuario.

Después, el script construye una raíz nueva en la carpeta temporal y cambia a ella:

| Dentro de la jaula | Procede de |
|--------------------|------------|
| `/bin`, `/lib` | Montaje enlazado de las carpetas del nodo. |
| `/lib64` | Montaje enlazado, solo en `amd64`. |
| `/usr`, `/etc`, `/dev`, `/proc` | Montaje enlazado recursivo de las carpetas del nodo. |
| `/tmp` | Un `tmpfs` nuevo de 50 MB. |
| La carpeta del servidor | Montaje enlazado, en la misma ruta absoluta. |
| La carpeta de binarios (`daemon.data.binaries`) y la de caché (`daemon.data.cache`) | Montaje enlazado, en la misma ruta absoluta. |
| Cada ruta de `mounts` del servidor | Montaje enlazado, en la misma ruta. |
| El destino de `/etc/resolv.conf` | Montaje enlazado, cuando `/etc/resolv.conf` es un enlace simbólico (systemd-resolved), para que el DNS siga funcionando. |

A continuación, `pivot_root` convierte esa carpeta en la raíz, la raíz antigua se desmonta (`umount -l`) y se elimina, y el comando de arranque se ejecuta desde la carpeta del servidor. El comando y sus argumentos se entrecomillan uno a uno, así que el shell no los interpreta; un comando de arranque vacío o que contenga un NUL o un salto de línea se rechaza.

El resto del nodo (`/home`, `/root`, `/var`, las carpetas de otros servidores...) no es visible. Dentro de la jaula, la carpeta de binarios se pone la primera en el `PATH`; una carpeta de binarios relativa se convierte en absoluta (`/` + ruta).

El script necesita `bash`, `mount`, `umount`, `mkdir`, `touch`, `rm` y `pivot_root` en el nodo (util-linux y coreutils).

### Ajustes

| Dónde | Clave | Por defecto | Efecto |
|-------|-------|-------------|--------|
| `config.json` | `security.disableUnshare` (`SKYPANEL_SECURITY_DISABLEUNSHARE`) | `false` | `true` desactiva la jaula para todos los servidores `host` del nodo. |
| Entorno del servidor | `disableUnshare` | `false` | `true` desactiva la jaula para ese servidor. |
| Entorno del servidor | `mounts` | vacío | Rutas adicionales del nodo que se montan dentro de la jaula. |

```json
{
  "environment": {
    "type": "host",
    "disableUnshare": false,
    "mounts": ["/opt/shared-maps"]
  }
}
```

Sin la jaula, el comando se ejecuta directamente con el usuario del daemon, en la carpeta del servidor, con acceso a todo lo que ese usuario puede alcanzar.

### Desactivar los entornos en el host: `docker.disallowHost`

`docker.disallowHost` (por defecto `false`) desactiva los entornos que se ejecutan en el propio nodo (`host`, `tty`, `standard`, `bubblewrap`). Cuando está activado, el nodo no los ofrece, y crear, redefinir, instalar o arrancar un servidor con uno de ellos se rechaza con `environment "<tipo>" is disabled on this node (docker.disallowHost)`.

### Ubuntu 24.04 y posteriores

Ubuntu 24.04 restringe los espacios de nombres de usuario sin privilegios mediante AppArmor. La jaula no llega a arrancar y la consola muestra:

```
ERROR: Permission denied while starting the security jail.
Your OS (e.g. Ubuntu 24.04) might be restricting unprivileged user namespaces.
FIX 1 (Recommended): Run 'sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0'
FIX 2: Add '"security": {"disableUnshare": true}' to your config.json and restart.
```

Para conservar la primera solución tras reiniciar, póngala en un archivo de `/etc/sysctl.d/`:

```bash
echo "kernel.apparmor_restrict_unprivileged_userns=0" | sudo tee /etc/sysctl.d/99-aether-userns.conf
sudo sysctl --system
```

Más información en [Solución de problemas](../setup/troubleshooting.md).

## Resumen

| Protección | Se aplica a | Se puede desactivar |
|------------|-------------|---------------------|
| `openat2` + `RESOLVE_BENEATH` | Cada archivo de servidor que abre el daemon (gestor de archivos, SFTP, archivos comprimidos, copias de seguridad, descargas) | Solo se sustituye por `openat` con `security.forceOpenat` |
| Jaula `unshare` | Servidores del entorno `host` | `security.disableUnshare` o `disableUnshare` por servidor |
| Aislamiento de Docker | Servidores del entorno `docker` | Ver [Usuario y aislamiento del contenedor](../backend/04-servers.md#usuario-y-aislamiento-del-contenedor) |
