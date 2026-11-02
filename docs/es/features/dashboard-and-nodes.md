# Panel de inicio y nodos

El panel de inicio (en la barra lateral, **Inicio**) es la primera página tras iniciar sesión. Los operadores ven un resumen de toda la instalación; los usuarios normales, la lista de sus servidores. Las páginas de **Nodos** muestran cada máquina que ejecuta servidores de juego, sus recursos y discos en vivo, y avisan antes de que un nodo se quede sin disco. Esta página también describe los servicios en segundo plano que hay detrás de esas cifras: el historial de red del nodo, el muestreo de CPU, la comprobación de puertos y la descarga anticipada de imágenes.

## Qué hace

**Inicio, vista de operador** (usuarios con `nodes.view` o `users.info.search`):

- **Franja de estado**: saludo, **Servidores activos** («N de M»), **Salud global** (disponibilidad media de los servidores en 30 días) y **Tráfico de red** (velocidad en vivo de todos los nodos).
- **Avisos de poco disco** para cada nodo cuyo disco de servidores esté casi lleno (solo con `nodes.view`).
- **Indicadores de recursos**: **Media de CPU**, **Media de memoria** y **Media de almacenamiento**, en porcentaje.
- **Información del panel**: versión, número de servidores, usuarios y nodos, y si las **Altas** están abiertas o cerradas.
- **Red histórica**: descarga y subida de todos los nodos sumadas, en 1 h, 6 h o 24 h.
- **Registros de acceso**: últimos inicios de sesión y altas de cuenta (véase [Actividad y registro de accesos](activity-and-access-log.md)).

**Inicio, vista de usuario** (el resto): «Mis servidores», una tarjeta por cada servidor que el usuario puede abrir.

**Nodos** (barra lateral **Nodos**, permiso `nodes.view`):

- Lista de nodos con ubicación y número de servidores; acciones **Ver detalles**, **Editar**, **Desplegar** y **Eliminar**.
- Detalle del nodo: total de servidores (en línea/fuera de línea), **Uso de memoria**, **Uso de CPU** (núcleos/hilos), tiempo encendido, información del sistema (nombre de host, plataforma, SO, arquitectura, modelo de CPU, memoria), **Discos** y **Servidores en este nodo**.
- Diálogo **Desplegar nodo** con los pasos para instalar un nodo solo daemon.

## Cómo funciona

### De dónde salen las cifras del inicio

| Cifra | Origen |
|-------|--------|
| Servidores activos | Lista de servidores (`/api/servers`) con el estado de `/api/uptime` |
| Salud global | Media del `uptimePercent` de cada servidor según `/api/uptime` (requiere `admin` o `uptime.view`; si no, no muestra valor) |
| Media de CPU / memoria / almacenamiento | Media entre nodos de `GET /api/nodes/:id/system` (uso de CPU, memoria usada/total, porcentaje usado del disco de servidores). Si ningún nodo informa, se usa la media de los servidores en línea. El texto bajo cada valor indica cuál («media de N nodos» / «media de N servidores») |
| Tráfico de red (en vivo) | Diferencia entre dos lecturas de los contadores acumulados de bytes de cada nodo (`networkBytesRecv`/`networkBytesSent`), sumada entre nodos. La primera lectura tras abrir la página solo fija la referencia |
| Red histórica | `GET /api/nodes/network-history?range=1h\|6h\|24h` |
| Información del panel | `/api/config` (versión, altas) y los recuentos que el usuario puede ver (usuarios con `users.info.search`, nodos con `nodes.view`) |

El inicio consulta los nodos cada 5 segundos y el historial de red cada 60 segundos; ninguno consulta mientras la pestaña del navegador está oculta.

### Información del sistema del nodo y muestreo de CPU

`GET /api/nodes/:id/system` reenvía la petición a `GET /daemon/system` en el nodo. El daemon informa de los datos del host, la memoria (incluido `memoryAvailable`, libre más cachés), cada disco una sola vez (los montajes del mismo disco dentro de un contenedor se agrupan y se nombran según `/` o la carpeta de servidores), el **disco de servidores** (`serversDisk`) y los contadores de red acumulados.

El uso de CPU no se mide dentro de la petición: un muestreador en segundo plano lo lee en ventanas de 2 segundos y `/system` responde al momento con el último valor. La primera llamada tras arrancar el daemon hace una lectura corta de 250 ms.

La página de detalle del nodo consulta `/system` cada 3 segundos mientras está visible.

### Historial de red

Cada daemon lee sus contadores de red cada 10 segundos, los acumula por minuto, conserva 24 horas de puntos de un minuto y los guarda cada 5 minutos en `network.history` dentro de su carpeta de datos (`daemon.data.root`), de modo que el historial sobrevive a los reinicios. Solo cuentan las interfaces físicas: se omiten el loopback, las subinterfaces VLAN y las interfaces virtuales (`docker*`, `br-*`, `veth*`, `virbr*`, `cni*`, `flannel*`, `cali*`, `tun*`, `tap*`, `wg*`, `tailscale*`, `bond*` y similares) para no contar dos veces el mismo tráfico. Un hueco de más de 5 minutos entre lecturas (host suspendido) no se reparte en el minuto.

Cuando el daemon corre en un contenedor, su propio `/proc` solo ve el tráfico del contenedor. Con `daemon.hostProc` definido (docker-compose monta el `/proc` del host en solo lectura en `/host/proc` y define `SKYPANEL_DAEMON_HOSTPROC=/host/proc`), el daemon lee los contadores del host en `<hostProc>/1/net/dev`.

`GET /daemon/network/history?range=` devuelve los puntos promediados al paso del rango: 1 minuto para `1h`, 5 minutos para `6h` y 15 minutos para `24h`. El `GET /api/nodes/network-history` del panel consulta todos los nodos en paralelo (10 segundos de espera máxima cada uno), suma los puntos paso a paso e informa de `nodes` (consultados) y `unavailable` (sin respuesta, por ejemplo un nodo antiguo o caído). Un paso sin datos de ningún nodo se dibuja como un hueco, no como una línea.

### Aviso de poco disco

El daemon marca `serversDisk.low = true` cuando al disco que contiene la carpeta de servidores le quedan **menos de 5 GB o menos del 10 %** libres. El panel muestra entonces «Al nodo «X» le queda poco disco» en el inicio, en la lista de nodos y en la página de ese nodo.

Por otro lado, `daemon.minFreeDiskMB` (2048 por defecto) se aplica a las instalaciones: si al empezar una instalación el disco de servidores tiene menos espacio libre, se rechaza con `ErrNodeDiskFull` y la consola del servidor muestra «The node only has X GB free, and an install needs at least Y GB. Free some space on the node and install again.». Véase [Servidores](../backend/04-servers.md).

### Comprobación de puertos en el asistente de creación

Mientras se escriben puertos en el asistente de nuevo servidor, este llama a `POST /api/nodes/:id/ports/check` 400 ms después del último cambio. Para cada puerto pedido, el panel responde si está ocupado y por qué:

| Motivo | Significado |
|--------|-------------|
| `range` | Por debajo de 1024 |
| `panel` | El puerto público, privado o SFTP del propio nodo, o un contenedor cuyo nombre empieza por `skypanel` (panel, base de datos, Adminer) |
| `server` | Puerto principal o extra de otro servidor de ese nodo (según la base de datos del panel o un puerto publicado por su contenedor) |
| `container` | Cualquier otro contenedor del nodo que publique ese puerto del host (según `GET /daemon/ports`) |

Un puerto ocupado incluye `suggestion`: el siguiente puerto (como mínimo 1024) libre y no pedido en otro campo del mismo formulario; el asistente lo ofrece como «Usar N». Quién ocupa el puerto (`by`) solo se devuelve a los administradores (permiso `admin`). Si el nodo no puede responder a `/daemon/ports` (sin Docker o versión antigua), la comprobación se limita a los registros del panel. La petición admite `exclude` con el identificador de un servidor cuyos propios puertos se ignoran.

### Estado de los servidores en nodos remotos

El daemon de un nodo remoto funciona sin el panel, así que el panel le pregunta por el estado de sus servidores: cada 30 segundos llama a `GET /daemon/servers/status` en todos los nodos remotos a la vez (una petición por nodo, con 10 segundos de tiempo máximo; una ronda nunca se solapa con la anterior). Solo lo hace el proceso que ejecuta el panel. Las respuestas se registran como disponibilidad igual que las de los servidores del propio panel (véase [Seguimiento de disponibilidad](../general/13-uptime.md)), de modo que la **Salud global** y las cifras de disponibilidad también cubren los servidores de nodos remotos. Un nodo que no responde no registra nada; un periodo que lleva más de 3 minutos sin confirmarse se cierra en su última confirmación, así que una caída del nodo no cuenta ni como encendido ni como apagado, y empieza un periodo nuevo cuando el nodo vuelve a responder. Un nodo con un daemon antiguo sin esa ruta (responde `404`) se trata como un nodo que no responde.

Las mismas respuestas alimentan el informe del nodo que se envía al webhook de Discord de nodos (`panel.notifications.discordWebhookNode`): un nodo remoto muestra su número real de servidores encendidos y apagados (de la última consulta, o de una llamada directa si esa consulta tiene más de 60 segundos), y un nodo que no responde recibe un informe en rojo con el estado **Sin respuesta** y el número de servidores que el panel tiene en él. Véase [Webhooks de Discord](../general/16-discord.md).

### Descarga anticipada de imágenes

En los nodos con Docker, el daemon descarga las imágenes de `docker.prefetchImages` 2 minutos después de arrancar y luego una vez al día (con un límite de 30 minutos por pasada). Si una descarga sustituye una copia anterior, la imagen vieja se borra cuando ningún contenedor la usa. Así se evita un primer arranque largo en los servidores que usan esas imágenes.

## Uso

### Desde el panel

- **Inicio**: abra el panel de inicio y elija **1 h**, **6 h** o **24 h** en **Red histórica**. Un aviso de poco disco enlaza con el nodo.
- **Nodos → Añadir nodo**: indique nombre, host público, puerto público y puerto SFTP; marque la opción de host privado distinto si el panel llega al nodo por otra dirección.
- **Nodos → nodo → Desplegar nodo**: muestra el `docker-compose.yml` y los comandos para un nodo solo daemon (`SKYPANEL_PANEL_ENABLE=false`). Requiere `nodes.deploy` porque revela el secreto del nodo.
- **Editar nodo / Borrar nodo** en la página del nodo. El nodo local no se puede editar («No se puede editar»); el host que muestra sale de la URL maestra del panel en la configuración.

### Mediante la API

| Método | Ruta | Permiso | Propósito |
|--------|------|---------|-----------|
| `GET` | `/api/nodes` | `nodes.view` | Listar nodos |
| `POST` | `/api/nodes` | `nodes.create` | Crear un nodo |
| `GET` / `PUT` / `DELETE` | `/api/nodes/:id` | `nodes.view` / `nodes.edit` / `nodes.delete` | Leer, editar y borrar un nodo |
| `GET` | `/api/nodes/:id/features` | `nodes.view` | Entornos, soporte de Docker, SO, arquitectura y versión |
| `GET` | `/api/nodes/:id/system` | `nodes.view` | Información del sistema en vivo |
| `GET` | `/api/nodes/network-history?range=1h\|6h\|24h` | `nodes.view` | Tráfico de todos los nodos sumado |
| `POST` | `/api/nodes/:id/ports/check` | `server.create` | Qué puertos están ocupados, por qué y una alternativa libre |
| `GET` | `/api/nodes/:id/deployment` | `nodes.deploy` | ID de cliente y secreto para un daemon remoto |
| `GET` | `/daemon/system`, `/daemon/ports`, `/daemon/network/history` | JWT del panel | Fuentes del lado del nodo que usa el panel |

Ejemplo de comprobación de puertos:

```json
POST /api/nodes/3/ports/check
{ "ports": { "SERVER_PORT": 25565, "extra:0": 8080 }, "exclude": "" }

{ "ports": { "SERVER_PORT": { "busy": true, "reason": "server", "by": "Survival", "suggestion": 25566 },
             "extra:0": { "busy": false } } }
```

Los detalles completos están en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

| Clave | Valor por defecto | Variable de entorno | Efecto |
|-------|-------------------|---------------------|--------|
| `daemon.minFreeDiskMB` | `2048` | `SKYPANEL_DAEMON_MINFREEDISKMB` | MB libres que debe conservar el disco de servidores para que empiece una instalación; `0` desactiva la comprobación |
| `daemon.hostProc` | `""` | `SKYPANEL_DAEMON_HOSTPROC` | Dónde está montado el `/proc` del host, para medir el tráfico real del nodo desde un contenedor (`/host/proc` en docker-compose) |
| `docker.prefetchImages` | `ghcr.io/aether-panel/steamcmd:latest`, `ghcr.io/aether-panel/ubuntu:latest`, `ghcr.io/aether-panel/generic:latest` | `SKYPANEL_DOCKER_PREFETCHIMAGES` (separadas por espacios) | Imágenes que se mantienen descargadas y al día en los nodos con Docker; una lista vacía lo desactiva |

Los umbrales de poco disco (5 GB / 10 %), los intervalos de muestreo y las 24 horas de historial están fijados en el código. Todas las claves están en [Configuración](../backend/02-configuration.md).

## Límites

- La velocidad de red en vivo necesita dos consultas (unos 5 segundos) antes de mostrar un valor.
- El historial de red empieza vacío en un nodo nuevo y se llena en los minutos siguientes; cubre como máximo 24 horas.
- Un nodo con un daemon antiguo sin `/daemon/network/history` cuenta como «sin responder» en el pie del historial.
- Sin `daemon.hostProc` en un daemon dentro de un contenedor, el tráfico mostrado es solo el del propio contenedor del panel, no el de los servidores de juego.
- La comprobación de puertos es orientativa: no reserva puertos.
- Los servidores de un nodo con un daemon antiguo (sin `/daemon/servers/status`) no tienen datos de disponibilidad hasta que se actualiza el nodo.

## Problemas conocidos y soluciones

| Síntoma | Causa | Solución |
|---------|-------|----------|
| La red histórica muestra «Aún no hay historial» | El nodo acaba de empezar a registrar | Esperar unos minutos |
| El pie del historial indica «N sin responder» | Un nodo está caído o tiene una versión antigua | Revisar el nodo; actualizarlo ([Actualizaciones del panel](panel-updates.md)) |
| «No se pudo cargar el historial» | El panel no respondió a `/api/nodes/network-history` | Se reintenta cada minuto; revisar el registro del panel |
| Tráfico casi nulo con los servidores en uso | Daemon en contenedor sin `daemon.hostProc` | Montar `/proc:/host/proc:ro` y definir `SKYPANEL_DAEMON_HOSTPROC=/host/proc` |
| La salud global no muestra valor | El usuario no tiene `admin`/`uptime.view`, o aún no hay datos de disponibilidad | Conceder `uptime.view` o esperar a que haya datos |
| Página del nodo: «No se puede contactar con este nodo…» | El daemon está parado o el panel no llega a él | Arrancar el daemon; revisar host, puertos y cortafuegos |
| «El nodo rechazó las credenciales del panel…» (`502`, `ErrNodeAuthFailed`) en un servidor de un nodo remoto | El nodo rechazó el token del panel: su `daemon.auth.clientSecret` no es el secreto del nodo en el panel, o la hora de una de las dos máquinas es incorrecta. El registro del panel indica el nodo | Volver a copiar el secreto desde **Desplegar nodo** en la configuración del nodo y reiniciarlo; sincronizar la hora de ambas máquinas (NTP). Véase [Solución de problemas](../setup/troubleshooting.md) |
| Informe de Discord del nodo: **Sin respuesta** | El nodo no respondió a la consulta de estado (caído, inaccesible o con un daemon antiguo) | Revisar el nodo; actualizarlo si es antiguo |
| «Al nodo «X» le queda poco disco» | Quedan menos de 5 GB o del 10 % en el disco de servidores | Liberar espacio o ampliar el disco |
| La instalación falla con `ErrNodeDiskFull` | Espacio libre por debajo de `daemon.minFreeDiskMB` | Liberar espacio o bajar la clave (`0` la desactiva) |
| Registro del daemon: `Could not download the runtime image …` | El registro de imágenes no responde durante la descarga anticipada | Revisar el acceso a `ghcr.io`; la pasada diaria siguiente lo reintenta |
| Registro del daemon: `Could not save the node's network history` | La carpeta de datos no admite escritura | Revisar los permisos de `daemon.data.root` |

## Véase también

- [Conceptos básicos: despliegue multinodo](../general/concepts.md)
- [Servidores: límite de disco y entornos](../backend/04-servers.md)
- [Actividad y registro de accesos](activity-and-access-log.md)
- [Seguimiento de disponibilidad](../general/13-uptime.md)
- [Actualizaciones del panel](panel-updates.md)
- [Configuración](../backend/02-configuration.md)
- [Referencia de la API](../reference/11-api-reference.md)
