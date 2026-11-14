# Mapa del mundo

La pestaña **Mapa** instala en un clic un mapa web del mundo de Minecraft (BlueMap en 3D o Dynmap en 2D) y lo muestra dentro del panel, con los jugadores en directo. El mapa funciona como plugin o mod en el propio servidor de juego; Aether Panel lo descarga, configura su servidor web y lo sirve al navegador a través de un proxy aislado y firmado.

---

## Qué hace

- Detecta si el servidor puede cargar un mapa: Paper (y Spigot, Bukkit, Purpur, Pufferfish, Folia), Fabric (y Quilt), Forge o NeoForge.
- Ofrece dos mapas:
  - **BlueMap**: vista 3D y jugadores en directo. Descarga los recursos de Minecraft desde Mojang, por lo que su instalación exige aceptar expresamente el EULA de Mojang. El primer renderizado tarda y consume CPU.
  - **Dynmap**: mapa 2D clásico en vista cenital, con jugadores y chat; más ligero.
- Descarga de Modrinth la versión más reciente que encaja con el cargador y la versión de Minecraft del servidor, guarda antes un punto de restauración y reinicia el servidor si está encendido.
- Ajusta el puerto web del mapa y (en BlueMap) la descarga de recursos, y después muestra el mapa en un iframe, con **Abrir en una pestaña nueva**, **Pantalla completa** y **Recargar el mapa**.
- Quita el mapa (**Quitar mapa**) conservando lo que ya se había dibujado.

---

## Cómo funciona

### Detección de la plataforma

El nodo decide qué versión instalar mirando, por este orden, el cargador del modpack instalado (véase [Modpacks y plugins](modpacks-and-plugins.md)), la variable `modlauncher` de la plantilla y los pasos de instalación que descargan el juego (`paperdl`, `fabricdl`, `forgedl`, `neoforgedl`, `mojangdl`). Un servidor vanilla (solo `mojangdl`) o un proxy (Velocity, Waterfall) no puede cargar un mapa. La versión de Minecraft se toma del modpack, de la variable de versión del servidor, de la versión que informa el servidor en marcha o, si la variable vale `latest`, de la versión estable más reciente.

### Instalación

`POST /api/servers/:serverId/map/install` responde `202` y el nodo continúa en segundo plano, mostrando cada paso en la consola:

1. Se intenta crear un punto de restauración (motivo `plugin`); si hay una copia en curso, la instalación continúa sin él.
2. Se consulta en Modrinth el proyecto `bluemap` o `dynmap` para la plataforma y la versión del juego. En Paper se prueban los cargadores `paper`, después `spigot`/`bukkit` y por último `folia`. Se elige la versión estable más reciente (o la beta más reciente si no hay estable; las alfa nunca se usan).
3. Se borra cualquier jar anterior de BlueMap o Dynmap en la carpeta de destino, para que solo se cargue un mapa.
4. El jar principal se descarga únicamente desde `cdn.modrinth.com`, con un máximo de 128 MB, y se comprueba con el SHA-512 de Modrinth. Se guarda en `plugins/` en Paper y en `mods/` en Fabric, Forge y NeoForge.
5. El panel registra lo instalado en `<carpeta de servidores>/<id>.worldmap` (tipo, versión, puerto, consentimiento, ruta del jar y plataforma), fuera de la carpeta del propio servidor.
6. Si el servidor está encendido, se detiene y se vuelve a arrancar para cargar el mapa; si no, el mapa arranca la próxima vez que se encienda.

### Configuración en cada arranque

Los mapas escriben sus archivos de configuración en su primer arranque, así que el panel los ajusta después. Mientras haya un mapa instalado, cada ejecución del servidor se revisa una vez: el nodo espera hasta 3 minutos a que aparezcan los archivos, fija los valores siguientes y ejecuta en la consola `bluemap reload` (BlueMap) o `dynmap reload` (Dynmap, si cambió el puerto).

| Mapa | Archivo (Paper) | Archivo (Fabric/Forge/NeoForge) | Clave ajustada |
|------|-----------------|---------------------------------|----------------|
| BlueMap | `plugins/BlueMap/core.conf` | `config/bluemap/core.conf` | `accept-download: true` (solo con consentimiento) |
| BlueMap | `plugins/BlueMap/webserver.conf` | `config/bluemap/webserver.conf` | `port` |
| Dynmap | `plugins/dynmap/configuration.txt` | `dynmap/configuration.txt` | `webserver-port` |

El puerto web es el **primer puerto adicional** del servidor (`port2`, `port3`... distinto del puerto del juego), de modo que el mapa también es accesible desde fuera por ese puerto. Sin puerto adicional se usa el puerto por defecto del mapa: **8100** en BlueMap y **8123** en Dynmap; en Docker ese puerto solo es accesible dentro de la red del contenedor.

### Visualización

El mapa es una aplicación web servida por el servidor de juego a partir de archivos que controla su propietario, así que el panel nunca la muestra con la sesión del usuario:

- `GET /api/servers/:serverId/map` devuelve el estado y, si hay un mapa instalado, un `viewUrl` firmado con la forma `/api/map-view/<serverId>/<token>/`. El token es un HMAC-SHA256 del identificador del servidor y una caducidad; se mantiene igual durante una hora y es válido entre 12 y 13 horas. El secreto de firma se genera al arrancar el panel, por lo que un reinicio del panel invalida los enlaces abiertos (basta con volver a abrir la pestaña).
- `/api/map-view/...` no requiere sesión: el enlace firmado es la única credencial. El panel reenvía la petición al nodo (`/daemon/server/<id>/map/view/<ruta>`), que la pasa al servidor web del mapa (la dirección del contenedor en Docker; si no, la IP del servidor o `127.0.0.1`).
- Todas las respuestas llevan `Content-Security-Policy: sandbox allow-scripts allow-popups allow-forms allow-pointer-lock allow-downloads`, `Referrer-Policy: no-referrer` y `X-Content-Type-Options: nosniff`, y se eliminan las cookies. La página se ejecuta así en un origen opaco que no puede llamar a la API del panel. Un pequeño script insertado en las páginas HTML sustituye `localStorage`, `sessionStorage`, `document.cookie` y los workers por equivalentes en memoria cuando el sandbox los bloquea.
- Como máximo se atienden 32 peticiones del visor por servidor a la vez; las demás esperan hasta 15 segundos.

### Eliminación

`DELETE /api/servers/:serverId/map` borra el jar y el registro `.worldmap`. Las teselas ya renderizadas y la configuración del mapa se quedan en el disco, así que una reinstalación no empieza de cero. El plugin o mod se descarga de memoria en el siguiente reinicio.

---

## Uso

### Desde el panel

1. Abrir un servidor de Minecraft Java e ir a la pestaña **Mapa** (aparece en servidores de tipo `minecraft-java`).
2. En **Elige el mapa**, seleccionar BlueMap o Dynmap. Para BlueMap, marcar **Acepto que BlueMap descargue recursos de Minecraft (EULA de Mojang)**.
3. Pulsar **Instalar BlueMap** / **Instalar Dynmap**. Si el servidor está encendido, se reinicia; si está apagado, el mapa se pondrá en marcha la próxima vez que se encienda (se ofrece **Encender servidor**).
4. Esperar mientras se muestra **El mapa se está preparando…**; la pestaña se actualiza sola (cada 5 segundos mientras hay trabajo en curso y cada 15 segundos el resto del tiempo).
5. Para desinstalarlo, usar **Quitar mapa** y reiniciar el servidor.

Si el cargador no es compatible, la pestaña indica **El mapa necesita Paper, Fabric o Forge** y ofrece **Cambiar el cargador** o **Instalar un modpack**.

### Mediante la API

Detalles: [Referencia de la API](../reference/11-api-reference.md).

| Método | Ruta | Scope | Finalidad |
|--------|------|-------|-----------|
| GET | `/api/servers/:serverId/map` | `server.view` | Estado: `supported`, `reason` (`vanilla`/`unknown`), `platform`, `gameVersion`, `installed`, `version`, `installing`, `lastError`, `running`, `ready`, `viewUrl` |
| POST | `/api/servers/:serverId/map/install` | `server.files.edit` | Cuerpo `{"kind":"bluemap"|"dynmap","acceptDownload":true}`; responde `202` |
| DELETE | `/api/servers/:serverId/map` | `server.files.edit` | Quitar el mapa; responde `204` |
| GET, HEAD | `/api/map-view/:serverId/:token/*path` | ninguno (enlace firmado) | Proxy del visor que usa el iframe |

---

## Requisitos y configuración

- Un servidor de Minecraft Java con Paper (o una variante compatible con Paper), Fabric, Quilt, Forge o NeoForge.
- El nodo debe poder conectar con `api.modrinth.com` y `cdn.modrinth.com`; además, BlueMap descarga los recursos de Mojang desde el servidor de juego.
- Opcional pero recomendable: un puerto adicional en el servidor (**Configuración › Puertos**) para que el mapa tenga su propio puerto accesible desde fuera.
- Esta función no tiene claves de configuración.

---

## Límites

- Un solo mapa por servidor; al instalar uno se borra el jar del otro.
- Solo una instalación por servidor a la vez, y nunca durante una copia o una restauración.
- El panel solo ajusta el puerto web y el `accept-download` de BlueMap; el resto de opciones quedan en los archivos del propio mapa.
- Los enlaces del visor caducan a las 12-13 horas y se invalidan cuando se reinicia el panel.
- El servidor web del mapa debe responder por HTTP sin cifrar en el nodo; el panel prueba las direcciones candidatas con un tiempo de espera de 1,5 segundos y guarda la dirección encontrada durante 30 segundos.

---

## Problemas conocidos y soluciones

| Mensaje | Causa y solución |
|---------|------------------|
| `the world map needs a Paper, Fabric, Forge or NeoForge server` | Servidor vanilla, proxy o cargador desconocido. Cambiar el cargador o instalar un modpack. |
| `installing BlueMap needs consent to download Minecraft's resources (Mojang's EULA)` | No se envió `acceptDownload`. Marcar la casilla del EULA. |
| `the map must be bluemap or dynmap` | Valor de `kind` incorrecto en la petición. |
| `a map install is already running` / `a backup or restore is already running` | Esperar a que termine la otra tarea. |
| `<Mapa> has no build for <plataforma> <versión> yet` | Modrinth no tiene versión para ese cargador y esa versión del juego. Probar el otro mapa u otra versión del juego. |
| `could not tell which Minecraft version this server runs` | Fijar la variable de versión a una versión estable, o arrancar el servidor una vez para que se pueda leer su versión. |
| `the file does not match Modrinth's checksum` / `refusing to download from <host>` | Descarga dañada o desde un servidor inesperado. Reintentar más tarde. |
| `the map's web server does not answer` (502 en el iframe) | El mapa aún está arrancando, o el puerto es incorrecto o está bloqueado. Esperar y pulsar **Recargar el mapa**; revisar en la consola los errores del propio mapa. |
| `this map link is not valid any more; open the map again from the panel` (403) | El enlace caducó o el panel se reinició. Volver a abrir la pestaña Mapa. |
| `too many map requests at once` (503) | Más de 32 peticiones simultáneas para un servidor. Recargar más tarde. |
| `no world map is installed on this server` (404 al quitar) | No hay nada que quitar; puede que el jar se borrara a mano. |

---

## Véase también

- [Modpacks y plugins](modpacks-and-plugins.md)
- [Versión del juego](game-versions.md)
- [Copias de seguridad](backups.md): puntos de restauración
- [Seguridad](../general/security.md) · [Referencia de la API](../reference/11-api-reference.md) · [Referencia de scopes](../general/scopes-reference.md)
