# Primeros pasos

Esta guía va desde un servidor Linux limpio hasta un primer servidor de juego funcionando en Aether Panel. Cada paso enlaza con la página que tiene todos los detalles.

## 1. Comprobar los requisitos

| | Mínimo |
|---|---|
| Sistema operativo | Linux: Ubuntu 22.04/24.04 (y posteriores), Debian 12, Rocky/Alma/RHEL 9; también Arch, openSUSE Tumbleweed, RHEL 8 y Fedora |
| Arquitectura | amd64 o arm64 |
| Recursos | 2 vCPU y 2 GB de RAM para el propio panel (o swap), más lo que necesiten los servidores de juego |
| Acceso | root o `sudo` |
| Red | Una IP pública. Un dominio es opcional pero recomendable (ver el paso 4). |

El instalador instala Docker y Docker Compose v2 si faltan. La lista completa, con los sistemas probados, está en la [Guía de instalación](../setup/01-installation.md#requisitos-del-sistema).

## 2. Instalar el panel

Ejecute como root (o con `sudo`):

```bash
bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"
```

El instalador comprueba la máquina (arquitectura, RAM, disco, puertos libres), instala Docker si hace falta, clona el repositorio en `/opt/skypanel`, genera contraseñas aleatorias para la base de datos, construye la imagen y arranca los contenedores. Pregunta:

- un **dominio** o solo la IP (con dominio configura Nginx y, si se quiere, un certificado de Let's Encrypt);
- los **puertos** (por defecto, 8080 para el panel y 5657 para SFTP) y la **base de datos** (la MariaDB incluida o un MySQL/MariaDB externo);
- el **correo del administrador**. Si no se le da una contraseña, genera una.

Para instalar sin preguntas, con dominio y HTTPS:

```bash
curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --non-interactive \
  --domain panel.example.com --ssl --email admin@example.com --admin-email admin@example.com
```

Ejecutar de nuevo el instalador ofrece actualizar, reinstalar o desinstalar. Todas sus opciones están en la [Guía de instalación](../setup/01-installation.md).

> No hay instalación nativa (sin Docker). La imagen siempre se construye a partir del `Dockerfile` del repositorio.

## 3. Iniciar sesión

Al terminar, el instalador muestra la dirección del panel y dónde están las credenciales:

- **Dirección**: `https://panel.example.com` con dominio; si no, `http://SU_IP:8080`.
- **Credenciales**: una contraseña generada se guarda en `/etc/SkyPanel/credenciales.txt` (legible solo por root), junto con las contraseñas de la base de datos.

```bash
sudo cat /etc/SkyPanel/credenciales.txt
```

Abra la dirección, inicie sesión con el correo y la contraseña del administrador y, si la contraseña era generada, cámbiela en **Configuración de la cuenta**.

Si no se pudo crear la cuenta de administrador (por ejemplo, porque el panel aún no estaba listo), créela a mano:

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user add --name admin --email admin@example.com --admin
```

Pide la contraseña (de 8 a 72 bytes). La [referencia de la CLI](../reference/14-cli-commands.md) recoge todos los comandos.

## 4. Proteger el panel

Conviene hacerlo antes de dar acceso a nadie más:

1. **Usar un dominio con HTTPS.** Un panel en `http://IP:8080` envía su cookie de sesión a todos los puertos de esa IP, incluidos los servidores de juego de los clientes. Apunte un dominio al servidor e instale con `--domain … --ssl`, o escriba `https://su-dominio` en **Configuración → General → URL principal**: el panel configura Nginx, el certificado y el firewall, y publica su propio puerto solo en `127.0.0.1`. El porqué y el cómo: [Seguridad](./security.md#exponer-el-panel-con-un-dominio-y-https).
2. **Configurar la URL principal** aunque no haya dominio (`http://SU_IP:8080`): los enlaces para restablecer la contraseña y de acceso directo se construyen con ella.
3. **Activar la 2FA** del administrador en **Configuración de la cuenta → Autenticación de dos pasos**, y guardar los códigos de recuperación.
4. **Decidir sobre el registro.** El registro público está activado por defecto (`panel.registrationEnabled`). Desactívelo en **Configuración → General** si las cuentas las crea usted o un sistema de facturación. La comprobación anti-bots de los formularios de inicio de sesión y registro está activada por defecto ([Captcha / anti-bots](17-turnstile.md)).

El instalador abre SSH, SFTP (5657), 80/443 y, cuando no está publicado en `127.0.0.1`, el puerto del panel con ufw o firewalld. Los puertos de los servidores de juego no necesitan reglas: Docker los publica él mismo. No abra MariaDB (3307) ni Adminer (8081); los dos escuchan solo en `127.0.0.1`. Ver [Firewall y puertos](./security.md#firewall-y-puertos).

## 5. Comprobar el nodo

La máquina donde corre el panel ya es un nodo: el **nodo local** (`LocalNode`) aparece en **Nodos** sin configurar nada. Su dirección sale de la URL principal. Todos los servidores pueden ejecutarse ahí.

Para ejecutar servidores en otras máquinas, instale en ellas la misma imagen como nodo y regístrelas en **Nodos**: ver [Instalación como nodo secundario](../setup/01-installation.md) y [Panel de inicio y nodos](../features/dashboard-and-nodes.md).

## 6. Crear el primer servidor

Vaya a **Servidores** y pulse **Añadir servidor** (solo los administradores crean servidores desde cero; los clientes dividen sus propios servidores con Splitter). También puede pulsar **Usar plantilla** en una plantilla de **Plantillas** para abrir el asistente con esa plantilla ya elegida. El asistente tiene tres pasos.

### Paso 1: Entorno

| Campo | Qué poner |
|-------|-----------|
| **Nombre del servidor** | Cualquier nombre; es lo que ven los usuarios. El ID del servidor se forma con él más un sufijo aleatorio. |
| **Nodo** | Dónde se ejecuta el servidor. Cada nodo muestra su dirección pública. |
| **Entorno** | Siempre **Docker**: la instalación del panel solo ejecuta servidores en contenedores (`docker.disallowHost`). No hay nada que elegir. |
| **Usuarios con acceso** | Opcional. El **primer** usuario elegido pasa a ser el **propietario** del servidor; los demás se añaden como miembros. Déjelo vacío para un servidor propio. |

Qué reciben esos usuarios: el propietario tiene el conjunto completo del propietario (consola, archivos, copias, tareas, subusuarios…); los demás reciben los mismos permisos de cliente salvo las copias de seguridad. Ninguno puede cambiar límites, puertos ni la definición. Ver [Propietario y subusuarios](./scopes-reference.md#propietario-y-subusuarios).

### Paso 2: Plantilla

1. Elija el **repositorio de plantillas**: **Comunidad** (el índice de `templates.aetherpanel.es`, que configura la instalación Docker) o **Local** (plantillas subidas a su panel).
2. Elija la **plantilla** (por ejemplo, una variante de Minecraft). El asistente carga sus detalles; espere a que termine.

### Paso 3: Configuración

**Recursos.** CPU (porcentaje de un núcleo: 100 es un núcleo, 200 dos), RAM (MB), disco (MB) y prioridad de disco. Empiezan con los valores de la plantilla (o 100 % de CPU, 1024 MB, 10240 MB y 500). El asistente avisa, bajo los campos, cuando:

- la RAM supera la que tiene el nodo (en rojo), la que tiene libre en ese momento o queda por debajo de lo que recomienda la plantilla (en ámbar);
- al nodo le queda poco disco o tiene menos disco libre que el límite indicado.

**Variables de la plantilla.** Un campo por cada variable que define la plantilla (las obligatorias llevan `*`):

- La **versión** del juego es una lista de versiones reales (de Mojang o PaperMC) cuando la plantilla descarga el juego desde ellos; `latest` sigue disponible.
- En Minecraft, la imagen de **Java** sigue a la versión elegida (26.1+ → Java 25, 1.20.5–1.21.x → 21, 1.17–1.20.4 → 17, anteriores → 8). Se puede cambiar igualmente; si no coincide, aparece un aviso.
- Los campos de **puerto** se comprueban contra el nodo mientras se escribe: un puerto por debajo de 1024, uno que usa el panel, uno que tiene otro servidor o uno que publica otro contenedor se marca, con un botón **Usar** para el siguiente puerto libre. Los administradores ven además quién lo ocupa.
- Antes de crear se validan los valores: campos obligatorios, números enteros dentro de los límites de la plantilla y la **EULA** de Minecraft, que debe aceptarse (sin ella el servidor se detiene nada más arrancar).

**Puertos extra.** **Añadir puerto** añade puertos además del principal; se convierten en `port2`, `port3`… y se publican por TCP y UDP. También se comprueban contra el nodo.

Pulse **Crear servidor**. Si algo está mal, el asistente marca el campo y lleva el foco a él; no se crea nada.

### Qué pasa después

1. El panel crea el servidor en el nodo y, con `panel.servers.autoInstall` activado (el valor por defecto), **empieza la instalación enseguida**: la plantilla descarga el juego y sus archivos. No hace falta pulsar Instalar.
2. Abra el servidor (el asistente avisa de que se está instalando) y siga el progreso en la **Consola**. Una primera instalación puede tardar unos minutos; las imágenes de ejecución suelen estar ya descargadas, porque los nodos las descargan por adelantado (`docker.prefetchImages`).
3. Cuando termine la instalación, pulse **Iniciar**. Si el servidor no arranca, la consola dice por qué (por ejemplo, la EULA sin aceptar, un puerto ocupado o falta de memoria), y aparece un aviso de caída si se detiene de forma inesperada.
4. Conéctese desde el juego a la dirección que muestra la página del servidor (la dirección del nodo y el puerto principal).

El juego se ejecuta en su contenedor con un usuario sin privilegios (uid 988), con su propio `/tmp`, un límite de procesos y capacidades eliminadas. Ver [Contenedores de los servidores de juego](./security.md#contenedores-de-los-servidores-de-juego).

## 7. Siguientes pasos

- Dar acceso a alguien: pestaña **Usuarios** del servidor ([Referencia de scopes](./scopes-reference.md)).
- Archivos y SFTP: pestañas **Archivos** y **Acceso SFTP** ([Conceptos básicos: SFTP](./concepts.md#sftp)).
- Copias de seguridad y tareas programadas: [Copias de seguridad](../features/backups.md), [Tareas programadas](../features/scheduled-tasks.md).
- Bases de datos para los servidores: [Hosts de bases de datos](../features/database-hosts.md).
- Correo, avisos de Discord y el asistente de IA: [Correo](15-email.md), [Discord](16-discord.md), [Asistente de IA](../features/ai-assistant.md).
- Vender servidores desde WHMCS o Paymenter: [Integración con facturación](../features/provisioning-and-sso.md).
- Mantener el panel actualizado: **Configuración → Actualizaciones**, o el instalador con `--update` ([Actualizaciones del panel](../features/panel-updates.md)).
- Si algo no funciona: [Solución de problemas](../setup/troubleshooting.md) y las [Preguntas frecuentes](./faq.md).
