# Preguntas frecuentes

## General

### ¿Qué es Aether Panel?

Un panel autoalojado para gestionar servidores de juego. Se instala en un servidor propio; ejecuta cada servidor de juego en su propio contenedor Docker y ofrece al administrador, a su personal y a sus clientes una interfaz web para gestionarlos: consola, archivos, copias de seguridad, tareas programadas, bases de datos, SFTP y más. El backend está escrito en Go (Gin y GORM) y la interfaz web en Astro y React. Ver [Conceptos básicos](./concepts.md).

### ¿Es gratuito?

El código se publica con la licencia Apache 2.0. El panel tiene una activación de licencia (**Configuración**, `panel.license.key`), pero hoy nada en el panel lee la licencia guardada: no activa ni bloquea ninguna función. Ver [Sistema de licencias](14-license.md).

### ¿Qué se puede ejecutar?

Cualquier cosa que describa una plantilla y que Docker pueda ejecutar. La instalación Docker lee las plantillas de la comunidad de `https://templates.aetherpanel.es/templates.json`; también se pueden subir plantillas propias al repositorio **Local** (`templates.local.edit`). El asistente de nuevo servidor tiene ayudas extra para Minecraft: listas de versiones reales de Mojang y PaperMC y la imagen de Java que corresponde. Los modpacks, el mapa del mundo y el editor de propiedades son para servidores de Minecraft: Java Edition.

### ¿Hace falta Docker?

Sí. El propio panel se instala con Docker (no hay instalación nativa) y los servidores de juego solo se ejecutan en contenedores Docker: la instalación Docker desactiva los entornos que se ejecutan en el host (`docker.disallowHost`). El instalador instala Docker si falta.

### ¿En qué idiomas está la interfaz?

En inglés (el idioma por defecto) y en español. Cada usuario elige su idioma en **Configuración de la cuenta**; se recuerda en el navegador.

Limitación conocida: algunos mensajes que genera el servidor están fijados en español, sea cual sea el idioma elegido, por ejemplo los embeds de [Discord](16-discord.md), los pasos de una [migración entre paneles](../features/server-transfers.md#límites) y algunos errores de la API.

## Instalación y funcionamiento

### ¿Cómo se instala?

Ejecutando el instalador como root: `bash -c "$(curl -fsSL https://install.aetherpanel.es/install.sh)"`. Ver [Primeros pasos](./getting-started.md) y la [Guía de instalación](../setup/01-installation.md).

### ¿Dónde está la contraseña del administrador?

Si la generó el instalador, en `/etc/SkyPanel/credenciales.txt` (legible solo por root), junto a las contraseñas de la base de datos.

### He olvidado la contraseña del administrador. ¿Qué hago?

Si el correo está configurado, use **He olvidado mi contraseña** en la página de inicio de sesión. Si no, en el servidor:

```bash
docker exec -it skypanel /SkyPanel/bin/SkyPanel user edit
```

Pregunta por el usuario y permite cambiar la contraseña, el correo o el nombre de usuario, dar o quitar permisos de administrador o quitar la 2FA. Ver la [referencia de la CLI](../reference/14-cli-commands.md).

### He perdido el teléfono y los códigos de recuperación de la 2FA.

Un administrador quita la 2FA de la cuenta con `SkyPanel user edit` → **Remove 2FA** (el comando de arriba). Después se entra con la contraseña y se vuelve a activar la 2FA.

### ¿Qué base de datos usa el panel?

La instalación Docker usa su propio contenedor de MariaDB 10.11 (`skypanel-mysql`). El instalador puede usar en su lugar un MySQL/MariaDB externo; comprueba la conexión y puede crear la base de datos y el usuario. El binario también admite SQLite (su valor por defecto fuera de Docker) y PostgreSQL (`panel.database.dialect`).

### ¿Dónde guarda el panel sus datos?

En `/opt/skypanel/storage` del host: `mysql-data` (la base de datos), `skypanel-config` (el archivo de configuración con las claves del panel), `skypanel-data` (servidores de juego y copias de seguridad) y `skypanel-logs`. Las copias que hace el instalador van a `/var/backups/aether-panel`.

### ¿Cómo se actualiza?

Desde **Configuración → Actualizaciones** (solo ofrece una versión cuando las comprobaciones de GitHub sobre ella han pasado, y también actualiza los nodos remotos), o con el instalador: `curl -fsSL https://install.aetherpanel.es/install.sh | sudo bash -s -- --update`. El instalador hace antes una copia de la base de datos y vuelve a la versión anterior si la nueva no arranca. Ver [Actualizaciones del panel](../features/panel-updates.md).

### ¿Se pueden gestionar varias máquinas?

Sí. La máquina del panel es el nodo local; otras máquinas ejecutan la misma imagen como nodos y se registran en **Nodos**. Ver [Conceptos básicos: Nodos](./concepts.md#nodos).

### ¿Conviene usar un dominio?

Sí, en cualquier panel con clientes. Un panel en `http://IP:8080` envía su cookie de sesión a todos los puertos de esa IP, incluidos los servidores de juego de los clientes, y sin cifrar. Con un dominio, el instalador configura HTTPS y publica el puerto del panel solo en `127.0.0.1`. Ver [Seguridad](./security.md#exponer-el-panel-con-un-dominio-y-https).

### ¿Qué puertos hay que abrir?

80 y 443 (Nginx), 5657 (SFTP) y, solo sin dominio, 8080 (el panel). Los puertos de los servidores de juego los publica Docker sin reglas de firewall. MariaDB (3307) y Adminer (8081) escuchan solo en `127.0.0.1` y deben seguir cerrados. Ver [Firewall y puertos](./security.md#firewall-y-puertos).

## Servidores

### ¿Quién puede crear servidores?

Los administradores, desde **Servidores → Añadir servidor**. Los clientes no pueden crear servidores desde cero; el propietario de un servidor puede dividirlo en otros más pequeños con **Splitter**, usando los recursos de su propio plan. Los sistemas de facturación crean servidores con la [API de aprovisionamiento](../features/provisioning-and-sso.md).

### ¿Por qué el entorno siempre es "Docker" en el asistente?

Porque la instalación del panel solo ejecuta servidores de juego en contenedores (`docker.disallowHost`). Los entornos del host se rechazan al crear, instalar o arrancar un servidor.

### ¿Hay que pulsar Instalar después de crear un servidor?

No. Con `panel.servers.autoInstall` activado (el valor por defecto), la instalación empieza en cuanto se crea el servidor; se sigue en la consola y después se pulsa **Iniciar**.

### Mi servidor no arranca. ¿Dónde miro?

En su **Consola**: el panel escribe por qué falló el arranque. Motivos habituales: la EULA de Minecraft sin aceptar, un puerto ya ocupado, el límite de disco alcanzado (hay que borrar archivos antes), falta de memoria o una imagen que aún se está descargando. El asistente de nuevo servidor comprueba puertos, memoria y disco antes de crearlo para evitar la mayoría de estos casos.

### ¿Por qué el propietario no puede cambiar la RAM, la CPU, el disco ni los puertos?

Es lo que paga el cliente, así que solo los cambia un administrador (`server.data.edit.admin`), aunque la plantilla marque esas variables como editables. El propietario puede elegir cuál de los puertos asignados es el principal y ponerles notas. Ver [Variables que solo cambia un administrador](./scopes-reference.md#variables-que-solo-cambia-un-administrador).

### ¿Por qué los archivos del servidor son del usuario 988?

Los servidores de juego se ejecutan con un usuario sin privilegios, uid:gid `988:988` (`docker.serverUser`), no como root. El panel da los archivos del servidor a ese usuario antes de cada arranque. Por SFTP y en el gestor de archivos no se nota ninguna diferencia. Ver [Contenedores de los servidores de juego](./security.md#contenedores-de-los-servidores-de-juego).

### ¿Cómo llega un servidor de juego a una base de datos MySQL?

Creando una base de datos en la pestaña **Base de datos** del servidor (hace falta un host de bases de datos, que añade un administrador en **Hosts de BD**). El panel crea la base de datos y un usuario con contraseña aleatoria y muestra los datos de conexión. La MariaDB del propio panel es accesible desde los servidores de juego como `mysql:3306`, porque comparten la red de Docker `skypanel-servers`. Los contenedores se crean de nuevo en cada arranque, así que un servidor recoge los cambios de red en su siguiente arranque. Ver [Hosts de bases de datos](../features/database-hosts.md).

### ¿Cómo doy acceso a alguien a mi servidor?

En la pestaña **Usuarios** del servidor, se añade su correo y se marcan los permisos. Si no tiene cuenta, recibe un correo para elegir contraseña (con el registro cerrado, solo los administradores pueden invitar a personas sin cuenta). Ver [Subusuarios](./scopes-reference.md#subusuarios).

### ¿Puedo mover un servidor a otro nodo o a otro panel?

Sí. Un administrador lo mueve entre nodos del mismo panel desde la pestaña **Administración**, y a otra instalación de Aether Panel desde la pestaña **Migración** (transferencia federada). Entre nodos, las copias de seguridad no viajan con el servidor: se quedan en el nodo antiguo. Ver [Transferencias de servidores](../features/server-transfers.md).

### ¿Hay límite de copias de seguridad?

Los usuarios de cada servidor pueden guardar hasta `panel.backups.maxPerServer` copias (10 por defecto; los administradores no tienen límite). Para hacer otra hay que borrar una antigua. Ver [Copias de seguridad](../features/backups.md).

## Privacidad e integraciones

### ¿El panel se conecta a servicios externos?

Solo para las funciones que se usan o se configuran. El navegador carga los avatares de los usuarios desde Kodama (`api.kodama.sh`, con un hash de un solo sentido de la cuenta como semilla, nunca el correo); el panel consulta la API de GitHub en busca de actualizaciones (una vez por sesión del navegador cuando entra alguien con `settings.edit`, y en **Configuración → Actualizaciones**), y habla con el índice de plantillas, Mojang/PaperMC, Modrinth o CurseForge al instalar o buscar archivos de juego. Gemini (IA), Cloudflare (subdominios, Turnstile), los webhooks de Discord/Telegram, los proveedores de correo y el almacenamiento S3 solo se contactan una vez configurados. La lista completa está en la [Política de privacidad](./privacy.md#servicios-externos-que-contacta-el-panel).

### ¿Qué envía el asistente de IA a Google?

Cuando un usuario pide un análisis o habla con el asistente, el panel envía las líneas de la consola del servidor (y los detalles de la caída) a la API Gemini de Google con la clave de `panel.settings.geminiApiKey`. Sin clave o sin esa acción no se envía nada. Ver [Asistente de IA](../features/ai-assistant.md).

### ¿Tiene API el panel?

Sí. La API REST está en `/api` y acepta la cookie de sesión o `Authorization: Bearer <token>`. Los administradores pueden explorarla en `/swagger/`. Los sistemas de facturación usan `/api/v1` con claves API. Ver la [Referencia de la API](../reference/11-api-reference.md).

### ¿Qué es el archivo de Gatus del repositorio?

`data/gatus/config.yaml` es una configuración de ejemplo para [Gatus](https://github.com/TwiN/gatus), un monitor de disponibilidad externo. El panel no arranca ni usa Gatus; su propio historial de disponibilidad se describe en [Seguimiento de uptime](13-uptime.md). El ejemplo escucha en `127.0.0.1:8081`, que la instalación Docker usa para Adminer, así que hay que cambiar el puerto si se ejecuta en la misma máquina.

## Ayuda

### ¿Dónde consigo ayuda?

Primero en [Solución de problemas](../setup/troubleshooting.md). Los errores y las peticiones de funciones van al repositorio de GitHub (`Aether-Panel/Panel`); la ayuda de la comunidad está en el Discord del proyecto.
