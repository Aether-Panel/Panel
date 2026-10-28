# install.sh 4.0.0 → 4.1.0

Build `20261026-hardening`. Un solo archivo bash, mensajes en español, mismas rutas y valores por defecto:
`/opt/skypanel`, rama `dev2.0`, puertos 8080/5657, contenedor `skypanel`. Una instalación hecha con 4.0.0
sigue funcionando al ejecutar 4.1.0, y repetir el instalador no borra datos ni regenera secretos.

## Problemas encontrados (de más a menos grave)

1. **Contraseñas de la base de datos públicas en todas las instalaciones.**
   - *Antes:* se pedían `--db-pass` y `--db-root-pass` (o se usaba `skypanel_password` sin terminal), pero nunca se escribían en ningún sitio. Todos los paneles arrancan con las del `.env` versionado y público (`secreto_root_super_seguro`…). MariaDB está también en la red `skypanel-servers`, así que cualquier servidor de juego (código de los usuarios, plugins) puede entrar como root en la base de datos del panel.
   - *Ahora:* en una base de datos nueva se generan contraseñas aleatorias (128 bits) y se guardan en `/opt/skypanel/.env` con permisos 600. Si la base de datos ya existe, no se toca nada, porque cambiar el `.env` dejaría al panel sin acceso, pero se avisa de que siguen las contraseñas públicas. La nueva opción `--rotate-db-passwords` las cambia de forma segura: primero `ALTER USER` en MariaDB, después el `.env`, y antes guarda las nuevas en el archivo de credenciales por si algo falla a mitad.

2. **"Desinstalar solo (mantener datos)" borraba todos los datos.**
   - *Antes:* el modo básico hacía `rm -rf /opt/skypanel`. Ahí están `storage/` (MariaDB, servidores, backups, extensiones y la llave SFTP). Los modos "datos" y "todo" borraban volúmenes Docker y `/var/lib/SkyPanel`, que la instalación Docker no usa.
   - *Ahora:*
     - `basic`: hace `docker compose down` y quita el sitio de Nginx. Conserva el código, `.env` y `storage/`, y `--update` lo vuelve a levantar.
     - `data`: borra `storage/` y los contenedores de servidores de juego.
     - `full`: borra todo.
     - Sin terminal, los modos que borran datos exigen `--yes`. Con terminal hay que escribir `BORRAR`.

3. **Volver a ejecutar el instalador borraba la cuenta admin.**
   - *Antes:* en modo automático (`curl | bash`) siempre se ejecutaba `user add --force`, que borra y recrea el usuario con ese email o nombre. Sin terminal la contraseña era la fija `skypanel_password`, y se imprimía en pantalla. Sin dominio el email era `admin@`, que es inválido.
   - *Ahora:* en una instalación existente no se crean ni se tocan cuentas salvo que se pida (`--admin-email`, `--create-admin`). Reemplazar un usuario exige `--replace-admin`. Si no se indica contraseña, se genera una y va a `/etc/SkyPanel/credenciales.txt` (600, carpeta 700), nunca a la pantalla. La contraseña se pasa al contenedor por variable de entorno, no en la línea de comandos de `docker exec` ni en un `curl -d`.

4. **Certificados SSL.**
   - *Antes:*
     - `--uninstall-all` borraba **todos** los certificados de Let's Encrypt de la máquina, también los de otros sitios.
     - Cada ejecución forzaba un certificado nuevo con `--renew-by-default`, lo que choca con los límites de Let's Encrypt.
     - No había `deploy-hook`, así que tras la renovación automática Nginx seguía sirviendo el certificado viejo hasta caducar.
   - *Ahora:* solo se borra el certificado del dominio del panel. Se usan `--keep-until-expiring` y `--deploy-hook "systemctl reload nginx"`, y si el certificado ya existe no se pide otro. Sin email se usa `--register-unsafely-without-email` en lugar de inventar `admin@dominio`. Antes de pedir el certificado se comprueba que el DNS apunta al servidor.

5. **Opciones que no hacían nada.**
   - *Antes:* `--port`, `--sftp-port`, las preguntas de puertos y todo `--db-mode external` se ignoraban, porque `docker-compose.yml` tiene los puertos fijos. El resumen mostraba una URL con un puerto en el que el panel no escuchaba.
   - *Ahora:* los puertos, `--bind`, la BD externa y el cambio de puerto de MariaDB se escriben en `docker-compose.override.yml`. Ese archivo no está en git, así que sobrevive a `panelUpdate.sh`, y Docker Compose lo carga solo. La contraseña de la BD externa no se copia en él: se toma del `.env` por interpolación. El puerto SFTP también se escribe en `daemon.sftp.host`, para que el panel escuche y anuncie el correcto. Hace falta Compose ≥ 2.24.4 para `ports: !override`; está comprobado que las versiones anteriores dejan además publicados 8080/5657 sin avisar, así que el instalador se niega con un mensaje claro.

6. **La URL maestra no se configuraba.**
   - *Antes:* `panel.settings.masterUrl` quedaba en `http://localhost:8080`, y los enlaces de recuperación de contraseña y el SSO de facturación se construyen solo con ella.
   - *Ahora:* se guarda siempre: `https://dominio` con SSL, `http://dominio` sin SSL, o `http://IP:puerto` sin dominio, y se puede forzar con `--master-url`. Al re-ejecutar se respeta la que el admin haya cambiado en Ajustes, salvo que se indique un dominio, IP, puerto o URL nuevos. `--update` la rellena en instalaciones antiguas que no la tenían. Se escribe en el `config.json` del panel (`storage/skypanel-data/config.json`) con el contenedor parado y no como variable `SKYPANEL_*`, porque las variables de entorno tienen prioridad y anularían los cambios hechos desde Ajustes.

7. **Fallos de bash con `set -euo pipefail` que abortaban o corrompían respuestas.** Los marcados con *(probado)* se reprodujeron con el script 4.0.0 en un contenedor:
   - `clear` sin `TERM` (cloud-init, `ssh host 'curl … | bash'`) abortaba el script *(probado)*.
   - `--domain` sin valor fallaba con `$2: unbound variable` *(probado)*. Ahora hay un mensaje claro y se acepta `--opcion=valor`.
   - En modo `-y` nunca se preguntaban los datos "sensibles": `ask_input` comparaba con `"true"` pero se llamaba con `"sensitive"` *(probado)*.
   - Los mensajes de error de `ask_password` y `ask_yes_no` se capturaban dentro de la respuesta, porque salían por stdout dentro de `$(...)`. Una contraseña mal confirmada acababa conteniendo el texto del error.
   - `require_password` nunca llegaba a su comprobación de `$?`, y usaba `eval`.
   - `log_debug` devolvía 1 con debug desactivado y abortaba el script.
   - Si todas las webs de IP pública fallaban, `SERVER_IP` se quedaba con basura en lugar de caer a localhost.
   - Los `echo` sin `-e` mostraban los códigos de color literales en el resumen.
   - Ahora las preguntas devuelven el valor en una variable (`printf -v`) y hay un `trap ERR` que dice en qué paso y línea falló y recuerda que se puede repetir sin perder nada.

8. **Firewall: riesgo de quedarse fuera por SSH.**
   - *Antes:* se ejecutaba `ufw --force enable` y después se permitía solo el 22. Con SSH en otro puerto, la sesión se cortaba.
   - *Ahora:* se detectan los puertos SSH (`sshd -T` y la conexión actual) y se permiten **antes** de activar el firewall. Solo se abren el puerto web (no si `--bind 127.0.0.1`), el SFTP y el 80/443 con dominio. El resumen avisa de que Docker publica sus puertos por delante de ufw y firewalld.

9. **Nginx.**
   - *Antes:*
     - En Fedora/RHEL no existe `sites-available`, así que el script fallaba al escribir la configuración.
     - Se borraban el sitio `default` y `conf.d/default.conf`, rompiendo otros sitios de la máquina.
     - `client_max_body_size` quedaba en 1 MB, así que las subidas de archivos o backups más grandes fallaban detrás de Nginx.
     - Faltaba `X-Forwarded-Proto` en HTTP.
     - El panel veía a todos los usuarios con la misma IP, la de la puerta de enlace Docker, lo que afecta a los límites por IP, al captcha y a los registros.
     - Si `nginx -t` fallaba, la configuración rota se quedaba puesta.
   - *Ahora:*
     - En RHEL se usa `conf.d/skypanel.conf` y se activa `httpd_can_network_connect` con SELinux.
     - No se borra ningún otro sitio.
     - `client_max_body_size 0` y timeouts largos para la consola y los websockets.
     - Si `nginx -t` falla, se restaura la configuración anterior.
     - Se configura `security.trustedproxies` con la puerta de enlace de la red Docker del panel. Solo confía en conexiones que vienen del propio host.

10. **Instalación de Docker.**
    - *Antes:* Mint/Pop usaban una URL de repositorio que no existe y RHEL/Rocky/Alma usaban el repositorio de Fedora. Todos los errores se ocultaban con `2>/dev/null || true`, que acababa en un fallo confuso más adelante. La llave GPG no se verificaba, se preferían `docker-compose` v1 y `docker compose` v2 por igual, y apt fallaba si `unattended-upgrades` tenía el bloqueo de dpkg (habitual en VPS recién creados).
    - *Ahora:* se elige el repositorio correcto para cada distribución. La llave GPG se comprueba contra la huella oficial de Docker. Se espera el bloqueo de dpkg (`DPkg::Lock::Timeout=600`). Si falta, se instala el plugin Compose v2, y los errores de paquetes ya no se ocultan. Las dependencias se reducen a lo necesario (`curl`, `git`, `jq`, `iproute2`, `nginx`/`certbot` solo si hacen falta); sobraban `build-essential`, `gcc` y `sqlite`.

11. **Git y actualizaciones.**
    - *Antes:* si la rama no existía, se cambiaba a `main` sin avisar. Se usaba `git pull` sin `--ff-only`. Se borraban `docker-compose.dev.yml`, que está versionado, y cualquier `docker-compose.override.yml` del usuario. Los cambios locales de `docker-compose.yml` se descartaban sin copia. Se hacía `docker compose down` **antes** de una compilación de 5-15 minutos, con el panel caído todo ese tiempo.
    - *Ahora:* si la rama no existe hay un error claro. Se usa `--ff-only`. Antes de descartar cambios locales se guarda un parche en `/etc/SkyPanel/backups/`. El `.env` del servidor y la configuración de nodo esclavo se conservan igual que en `panelUpdate.sh`. Primero se compila con el panel en marcha y después se recrean los contenedores.

12. **Comprobaciones previas y supuestos obsoletos.**
    - *Antes:*
      - `free -g` redondea hacia abajo, así que 1,9 GB contaba como 1.
      - El disco solo se miraba en `/`.
      - No se comprobaban la arquitectura, systemd ni los puertos ocupados.
      - No había bloqueo contra dos ejecuciones a la vez.
      - Se buscaba `"3306:3306"` para cambiarlo por 3307, pero el compose actual ya publica `127.0.0.1:3307`, así que no hacía nada.
      - Las rutas `/var/lib/SkyPanel`, `/etc/SkyPanel` y `/var/log/SkyPanel` y los volúmenes `skypanel_*` no los usa la instalación Docker.
      - La verificación de login con `curl` ya no puede funcionar porque el captcha integrado está activo.
    - *Ahora:*
      - En arm64 se avisa de que el Dockerfile actual compila para amd64 (ver "Para decidir").
      - Se mide RAM + swap en MB y se recomienda swap para compilar.
      - Se mide el disco en `/opt` y en `/var/lib/docker`.
      - Solo se admiten amd64 y arm64, y se avisa si falta systemd o si se ejecuta dentro de un contenedor.
      - Se comprueban los puertos 8080, 5657, 80 y 443, distinguiendo si los ocupa el propio panel o Nginx. Si el 3307 está ocupado, se elige otro para MariaDB.
      - Hay un bloqueo con `flock`.
      - Se espera a que el panel responda (estado y HTTP, 5 minutos) y, si el contenedor cae, se muestran sus logs.

## Novedades

- **Modos y opciones:**
  - `--non-interactive`, que nunca pregunta.
  - `--dry-run`, que muestra cada comando y archivo que se cambiaría (sin secretos) y no toca nada.
  - `--version`, `--no-domain`, `--no-ssl`, `--ip`, `--master-url`, `--bind`, `--admin-name`, `--no-admin`, `--replace-admin`, `--rotate-db-passwords`.
  - Todas las opciones aceptan también `AETHER_*` como variable de entorno, lo recomendado para contraseñas: el instalador avisa si se pasan como argumentos, porque quedan en el historial y en `ps`.
- **Funciones recientes del panel**, escritas en su `config.json` igual que las guarda la pantalla de Ajustes (claves en minúscula, con el contenedor parado, el archivo con permisos 600 y una copia `.bak-installer`). El panel las lee de su archivo de configuración al arrancar:
  - URL maestra (`panel.settings.masterurl`), siempre.
  - Anti-bots: el captcha integrado (`panel.captcha.builtin`) viene activo y el resumen lo indica. `--turnstile-site-key` y `--turnstile-secret-key` activan Cloudflare Turnstile en su lugar (`panel.turnstile.*`).
  - Subdominios automáticos: `--cf-dns-token`, `--cf-zone-id`, `--dns-base-domain`, `--dns-target`, `--dns-auto-assign` (`panel.dns.*`).
  - Copias en S3: `--s3-endpoint`, `--s3-region`, `--s3-bucket`, `--s3-access-key`, `--s3-secret-key`, `--s3-prefix`, `--s3-insecure` (`daemon.backups.s3.*`).
  - `--curseforge-key` (`daemon.curseforge.key`).
  - `--log-retention-days N` (`logRetentionDays`, por defecto 14). No se puede cambiar desde Ajustes.
- **Persistencia de datos:** comprobado que todo lo que usa el panel (servidores, backups, extensiones, logs con rotación diaria, `sftp.key` y el propio `config.json`) vive en `/opt/skypanel/storage/skypanel-data` gracias a los montajes de `docker-compose.yml`, y sobrevive a recrear el contenedor. El instalador no necesita crear volúmenes. Antes, la desinstalación "básica" era la que lo destruía (punto 2).
- **`/etc/SkyPanel/installer.conf`** (600, sin secretos) guarda las opciones de la instalación: dominio, SSL, puertos, rama, IP y URL. Al re-ejecutar o con `--update` se usan como valores por defecto. En instalaciones de 4.0.0 el dominio y el SSL se deducen de la configuración de Nginx.
- **Resumen final:**
  - URL y URL maestra.
  - Puertos, incluidos el SFTP con llave SSH y Adminer solo en local por túnel SSH.
  - Estado del anti-bots y aviso del registro público.
  - Dónde están las credenciales.
  - Próximos pasos: crear el admin con un comando que pide la contraseña sin dejarla en el historial, configurar el correo, y Turnstile, DNS y S3 en Ajustes.
  - Cómo actualizar (desde el panel con `panelUpdate.sh`, o `--update`) y cómo desinstalar.
  - Aviso si la rama instalada no es `dev2.0`.
- Las instrucciones usan `curl -fsSL`. Con `curl -s`, un error HTTP (una página 404 en HTML) se ejecutaría como script.

## Sin cambios a propósito

- La rama por defecto sigue siendo `dev2.0`, que es la que usa `panelUpdate.sh`. Ahora se puede cambiar con `-b` o `AETHER_BRANCH`, y al re-ejecutar se mantiene la rama ya instalada.
- Se mantienen la ruta `/opt/skypanel`, que el actualizador del panel (`internal/update`) tiene fija, los nombres de contenedores y las redes, los puertos 8080/5657 por defecto, y `COMPOSE_PROFILES=local-db` (no hace nada, pero es igual que `panelUpdate.sh`).
- Se mantienen el proxy de Nginx en el puerto 80 también en modo IP (función de 4.0.0), la activación de ufw/firewalld y el menú interactivo con sus opciones. `-f/--force` sigue existiendo, pero ya no destruye nada: solo recrea contenedores.
- El `.env` sigue versionado. Cada servidor guarda el suyo y el instalador lo conserva igual que `panelUpdate.sh`, que hace copia antes del `git pull` y la restaura después.
- En instalaciones existentes, `--rotate-db-passwords` es opcional y no automático: cambiar contraseñas sin pedirlo podría dejar el panel sin base de datos si algo falla a mitad. Se avisa claramente.
- No se añadió modo "solo nodo" (daemon) en la instalación ni creación de swap, HSTS, IPv6 en Nginx o `cookies.secure`: cambian el comportamiento y conviene decidirlos aparte.

## Para decidir (fuera del instalador)

- **Distribución del script:** publicar un SHA-256 o una firma junto a `install.sh`.
- **MariaDB en la red `skypanel-servers`:** los servidores de juego llegan a la base de datos del panel. Con contraseñas propias el riesgo baja mucho, pero conviene un usuario o instancia aparte para los servidores.

Ya resuelto en el repositorio: el `Dockerfile` en arm64 (sin valores por defecto en `BUILDPLATFORM`/`TARGETPLATFORM`), `panelUpdate.sh` (usa la rama instalada, compila antes de parar el panel y sin el `sed` de 3306), el registro y el tema forzados en `docker-compose.yml`, y la documentación de instalación.

# install.sh 4.1.0 → 4.1.1

Arreglos encontrados al instalar de verdad en Ubuntu 22.04, 24.04, 26.04 y 26.10, Debian 12 y Rocky Linux 9 (máquinas con systemd, instalación completa hasta el inicio de sesión del administrador).

- **Rocky/Alma 9: el panel no llegaba a MySQL.** El instalador encendía firewalld después de instalar Docker. Al arrancar, firewalld borra las reglas que Docker creó para sus redes, y Docker solo las repone si ya encontró firewalld en marcha. Ahora, si el instalador acaba de encender firewalld, reinicia Docker. Los servidores donde firewalld ya estaba activo no cambian.
- **Rocky/Alma 9: `dnf` fallaba por `curl` frente a `curl-minimal`.** curl solo se instala si falta.
- **Límite de descargas de Docker Hub.** Las imágenes se descargan antes de arrancar, con 3 intentos. Si Docker Hub responde 429, el instalador lo explica (esperar o `docker login`) en vez de fallar sin más.
- **Versiones sin soporte.** El aviso ahora sale con Ubuntu anterior a 22.04 y Debian anterior a 12: ya no tienen parches de seguridad (Debian 11 terminó en agosto de 2026 y sus repositorios se archivaron).

# install.sh 4.1.1 → 4.1.2

Encontrado al instalar en un VPS real (Ubuntu 26.04) y apuntar un dominio con el proxy de Cloudflare.

- **El puerto 80 no se abría sin dominio.** El instalador pone Nginx en el 80 delante del panel también cuando no hay dominio, pero el firewall solo abría 22, 8080 y 5657. Un dominio con Cloudflare daba 522. Ahora el 80 se abre siempre que haya Nginx, y el 443 cuando hay dominio.
- **Un dominio sin configurar caía en la página de bienvenida de Nginx.** Sin dominio, el sitio del panel solo contestaba a la IP y el sitio `default` de la distribución se quedaba con el resto. Ahora el sitio del panel es el predeterminado del puerto 80: contesta por la IP y por cualquier dominio que apunte al servidor. Se quita solo el enlace `sites-enabled/default` (el archivo sigue en `sites-available`); si otro sitio ya es el predeterminado, no se toca.
- **IPv6.** Nginx escucha también en `[::]:80` y `[::]:443`.

# install.sh 4.1.2 → 4.1.3

- **El dominio de Ajustes se aplica solo.** Hasta ahora, escribir un dominio en **Ajustes → General → URL principal** solo cambiaba los enlaces del panel: Nginx, el certificado y el firewall había que configurarlos volviendo a ejecutar el instalador. Ahora el instalador deja el servicio `aether-domain` (una unidad `.path` de systemd que vigila el `config.json` del panel). Al guardar la URL ejecuta `install.sh --apply-panel-url`, que con las mismas funciones de la instalación configura Nginx, pide el certificado si la URL es `https://` y abre los puertos. Si no hay cambios, no toca nada. Un certificado fallido no se vuelve a pedir hasta pasada una hora (límite de Let's Encrypt), salvo que cambie la URL.
- El resultado queda en `domain-status.json`, junto al `config.json`, y el panel lo muestra bajo el campo (`GET /api/settings/domain-status`).
- Se instala también con `--update`, y `--uninstall` lo quita. No se instala sin systemd, en nodos solo daemon ni con `--skip-nginx`.

# install.sh 4.1.3 → 4.1.4

- **Dominio con el proxy de Cloudflare.** El panel daba como dirección de los servidores de juego y del SFTP el dominio de la URL principal. Con la nube naranja de Cloudflare ese dominio solo deja pasar la web, así que `dominio:25565` no conectaba nunca. El servicio `aether-domain` (y la instalación) comprueba ahora si el dominio apunta a este servidor; si no, lo anota en `domain-status.json` (`proxied` y `serverIp`) y el panel usa esa IP para jugadores y SFTP, salvo que en Ajustes haya una "IP del nodo maestro". Ajustes lo explica bajo la URL principal y recuerda los subdominios automáticos.
- La instalación y `--update` también escriben ese estado al terminar.

# install.sh 4.1.4 → 4.2.0

Punto de entrada único para instalar y administrar el panel: detecta el estado, ofrece las cuatro
operaciones y protege los datos en las que pueden perderlos.

## Estado y menú

- Distingue **no instalado**, **instalado pero detenido** y **funcionando**, y muestra la rama y la versión.
- Un contenedor solo cuenta como del panel si docker compose lo creó desde `/opt/skypanel` (etiqueta `com.docker.compose.project.working_dir`). Un contenedor `skypanel` de otro proyecto no se toca y se avisa.
- Menú: **Instalar**, **Actualizar**, **Reinstalar desde cero**, **Desinstalar** (más crear administrador y salir). Sin instalación, solo deja instalar.

## Instalación

- Puertos: se pregunta si usar los predeterminados; si no, se piden hasta que sean válidos, distintos entre sí, no reservados (SSH, 80/443 de Nginx, 3307 de la BD, 8081 de Adminer) y libres.
- Con dominio y HTTPS se explica qué necesita Let's Encrypt con Cloudflare: funciona con la nube naranja si el puerto 80 llega; la primera emisión falla con "Always Use HTTPS" activado; el certificado de origen de Cloudflare como alternativa.
- **Renovaciones detrás de Cloudflare:** el sitio HTTPS de Nginx también responde al reto de Let's Encrypt. Antes, con "Always Use HTTPS", la renovación llegaba por HTTPS al panel y fallaba.
- **Base de datos externa:** se comprueba la conexión y las credenciales antes de construir nada. Si la base de datos o el usuario no existen, se crean con una cuenta de administrador opcional (`--db-admin-user`, `--db-admin-pass` o `AETHER_DB_ADMIN_PASSWORD`), dando al usuario solo permisos sobre esa base de datos. La cuenta de administrador no se guarda. `localhost` se rechaza con la explicación: dentro del contenedor es el propio contenedor.
- El resumen final muestra el estado de los contenedores, la base de datos (sin contraseñas), los puertos que hay que abrir en el firewall del proveedor, dónde están los archivos y los comandos reales de administración: `SkyPanel user add`, `SkyPanel user edit`, `SkyPanel version` y docker compose.

## Actualización

- **Copia de la base de datos antes de tocar nada**, comprobada (gzip íntegro y con tablas), en `/var/backups/aether-panel/actualizaciones` (se guardan las 10 últimas). Sin copia no se actualiza, salvo `--skip-backup` o confirmación expresa.
- **Vuelta atrás automática:** se guarda la imagen actual como `:anterior`. Si la versión nueva no arranca, se devuelve el código a la versión de antes, se restaura la imagen y se levanta el panel otra vez. Se indica la copia de la base de datos y el comando para restaurarla si la versión nueva llegó a cambiarla.
- La base de datos nunca se borra ni se reinicia al actualizar.

## Reinstalación (nueva)

- `--reinstall` o la opción del menú. Explica qué se borra y qué no, ofrece una copia (base de datos + datos + configuración en `/var/backups/aether-panel/reinstalacion-FECHA`) y pide escribir `REINSTALAR` (o `--yes` sin terminal).
- Borra solo lo de esta instalación y empieza una instalación nueva que vuelve a preguntar dominio, puertos y base de datos. Una base de datos externa no se toca; se avisa de que sus datos siguen ahí. El certificado del dominio se conserva.

## Desinstalación

- Ofrece una copia antes de borrar datos.
- Borra solo lo del panel: contenedores de compose de `/opt/skypanel`, servidores de juego (etiqueta `skypanel.server`), redes, imagen `skypanel-skypanel`, sitio de Nginx, servicio `aether-domain` y las reglas del firewall del panel y del SFTP. Las de SSH, 80 y 443 se quedan, porque pueden usarlas otros sitios.
- Al terminar comprueba que no quede nada y lo dice.

## Distribuciones

- **Arch Linux** (y Manjaro, EndeavourOS…) con pacman, y **openSUSE** (Leap, Tumbleweed, SLES) con zypper. En ambas Docker viene de los repositorios de la distribución. En Arch se añade `include conf.d/*.conf` al `nginx.conf`, que no lo trae; se guarda una copia antes.
- En openSUSE se instala iptables con el motor nftables (`iptables-backend-nft`). Los núcleos nuevos (por ejemplo el 7.0 de Ubuntu 26.04) ya no traen los módulos del iptables clásico, y sin ellos Docker no arranca.
- En una distribución sin gestor de paquetes conocido se detiene explicando qué falta.

## Probado

Instalaciones reales en contenedores con systemd:
- **Ubuntu 26.04**, ciclo completo: instalar, menú con el estado, actualizar (copia de 26 tablas comprobada), actualización rota a propósito (vuelve sola a la versión anterior y el panel sigue respondiendo), reinstalar (con copia) y desinstalar todo (sin contenedores, imagen, redes ni archivos).
- **Arch Linux** y **openSUSE Tumbleweed**: instalación completa con el panel respondiendo.

## Repositorio

- `.gitattributes`: los `.sh` y el Dockerfile siempre con finales de línea LF.

