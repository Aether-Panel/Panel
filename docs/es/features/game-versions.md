# Versión del juego

Aether Panel puede cambiar la versión de Minecraft de un servidor en un clic: detiene el servidor, guarda un punto de restauración, ajusta la nueva versión, reinstala y lo vuelve a arrancar. Las mismas listas de versiones alimentan también el formulario de creación de servidores, de modo que no se puede escribir una versión que no existe.

El cambio en un clic es una **función experimental** y viene desactivada por defecto.

---

## Qué hace

- Muestra una tarjeta **Versión del juego** en la pestaña **Configuración** del servidor con la versión instalada, si es la más reciente y un selector **Nueva versión**.
- Lista las versiones estables de la plataforma del servidor, de la más reciente a la más antigua: Minecraft vanilla (desde Mojang) o Paper, Folia, Velocity y Waterfall (desde PaperMC).
- Cambia la versión en segundo plano, con un punto de restauración previo, e informa del resultado en la tarjeta (**Listo: el servidor ya usa la versión …** o **No se pudo cambiar la versión: …**).
- Ofrece las mismas listas en el formulario de creación de servidores para el campo de versión de la plantilla.

---

## Cómo funciona

### Qué servidores pueden cambiar de versión

El nodo examina los pasos de instalación de la plantilla que se ejecutan para este servidor (los pasos condicionales se evalúan) y toma el primero que descarga el juego:

- un paso `paperdl`: la plataforma es su `project` (`paper` por defecto) y la variable de versión es la indicada en `minecraftVersion`;
- un paso `mojangdl`: la plataforma es `vanilla` y la variable es la indicada en `version`.

El valor del paso debe ser exactamente una referencia `${variable}` a una variable existente del servidor. Las plantillas que no siguen este patrón (por ejemplo, las de Forge, NeoForge o CurseForge) no muestran la tarjeta. La tarjeta muestra el software de la variable `modlauncher` cuando la plantilla la tiene (un servidor Fabric, por ejemplo, lista las versiones de vanilla).

Un servidor con un modpack instalado no puede cambiar de versión, porque el modpack fija la versión del juego; véase [Modpacks y plugins](modpacks-and-plugins.md).

### Listas de versiones

- Vanilla: `https://piston-meta.mojang.com/mc/game/version_manifest_v2.json`, entradas de tipo `release`.
- Paper, Folia, Velocity, Waterfall: `https://fill.papermc.io/v3/projects/<proyecto>/versions`, solo versiones con forma `1.21` o `1.21.4`.

Las listas se ordenan de la más reciente a la más antigua, se recortan a 80 entradas y se guardan en caché en el nodo (o en el panel) durante una hora por plataforma.

### El cambio

`POST /api/servers/:serverId/version` comprueba que la versión elegida está en la lista (o es `latest`) y se rechaza mientras haya en curso una copia, una restauración u otro cambio de versión. Responde `202` y continúa en segundo plano, mostrando cada paso en la consola:

1. Detiene el servidor si está encendido y espera a que termine el proceso.
2. Crea un punto de restauración (motivo `version`). Si falla, no se cambia nada.
3. Ajusta la variable de versión. En `paperdl`, si el paso lee el número de build de una variable, esa variable vuelve a `latest`, porque un build fijado para la versión anterior no existe para la nueva.
4. Guarda el servidor y vuelve a ejecutar la instalación de la plantilla.
5. Si la instalación falla, las variables recuperan su valor y el error indica que se guardó un punto de restauración.
6. Vuelve a arrancar el servidor si estaba encendido.

Mientras dura el cambio, la tarjeta consulta el estado cada 3 segundos. Volver a una versión anterior puede impedir abrir mundos más recientes; el punto de restauración es la forma de deshacerlo.

---

## Uso

### Desde el panel

1. Un administrador activa **Configuración › General › Funciones experimentales › Cambio de versión del juego**.
2. Abrir el servidor, ir a **Configuración** y buscar la tarjeta **Versión del juego**. Se oculta si la plantilla no admite el cambio, si hay un modpack instalado o si el usuario no tiene permiso.
3. Elegir la **Nueva versión** y pulsar **Cambiar versión**. La confirmación detalla los pasos; confirmar con **Cambiar versión**.
4. Seguir el progreso en la tarjeta o en la consola. Si falla, restaurar el punto de restauración desde **Copias de seguridad**.

### Mediante la API

Detalles: [Referencia de la API](../reference/11-api-reference.md).

| Método | Ruta | Scope | Finalidad |
|--------|------|-------|-----------|
| GET | `/api/servers/:serverId/version` | `server.view` | `VersionInfo`: `platform`, `software`, `variable`, `current`, `versions`, `changing`, `lastError`. Responde `204` si el servidor no puede cambiar de versión y `502` si no se pueden obtener las listas |
| POST | `/api/servers/:serverId/version` | `server.install` + función experimental | Cuerpo `{"version":"1.21.4"}` (o `"latest"`); responde `202` |
| GET | `/api/game-versions/:platform` | `login` | Versiones estables de `vanilla`, `paper`, `folia`, `velocity` o `waterfall` (las usa el formulario de creación de servidores) |

---

## Requisitos y configuración

| Clave | Valor por defecto | Variable de entorno | Notas |
|-------|-------------------|---------------------|-------|
| `panel.experimental.versionSwitch` | `false` | `SKYPANEL_PANEL_EXPERIMENTAL_VERSIONSWITCH` | Habilita `POST /api/servers/:serverId/version` y muestra la tarjeta. Se puede cambiar en **Configuración › General › Funciones experimentales**. |

- El nodo (y, para `/api/game-versions`, el panel) debe poder conectar con `piston-meta.mojang.com` y `fill.papermc.io`.
- Una plantilla basada en `paperdl` o `mojangdl` con la versión en una variable, como se describe arriba.

Véase [Configuración](../backend/02-configuration.md).

---

## Límites

- Solo se ofrecen versiones estables; no aparecen snapshots, pre-releases ni release candidates.
- Se listan como máximo las 80 versiones más recientes.
- Un solo cambio por servidor a la vez, y nunca durante una copia, una restauración o la instalación de un modpack.
- Al desactivar la función experimental se bloquean los cambios nuevos (el `POST` devuelve `403`); el `GET` sigue funcionando.
- Las versiones antiguas de Minecraft pueden rechazar mundos guardados por versiones más nuevas.

---

## Problemas conocidos y soluciones

| Mensaje | Causa y solución |
|---------|------------------|
| `this experimental feature is turned off in the panel settings` (403) | Activar **Cambio de versión del juego** en las funciones experimentales. |
| `this server's template does not let the panel change its game version` (400) | La plantilla no tiene un paso `paperdl`/`mojangdl` compatible, o hay un modpack instalado. |
| `that version is not available for this server` (400) | La versión no está en la lista actual. Recargar la tarjeta y elegir una de la lista. |
| `a version change is already running` (409) | Esperar a que termine. |
| `a backup or restore is already running` (409) | Esperar a que termine la copia o la restauración. |
| `could not create a restore point: ...` | No hay espacio para el punto de restauración, o empezó una copia mientras tanto. Liberar espacio y reintentar. |
| `the new version could not be installed (a restore point was saved): ...` | Falló la descarga o la instalación. Revisar la consola; si hace falta, restaurar desde **Copias de seguridad**. |
| `unknown game platform; use vanilla, paper, folia, velocity or waterfall` (400) | Plataforma incorrecta en `/api/game-versions/:platform`. |
| `502 Bad Gateway` en las rutas de versión | No se pudo contactar con Mojang o PaperMC. La tarjeta permanece oculta hasta que vuelvan a responder; el formulario de creación pasa a un campo de texto libre. |

---

## Véase también

- [Modpacks y plugins](modpacks-and-plugins.md)
- [Mapa del mundo](world-map.md)
- [Copias de seguridad](backups.md): puntos de restauración
- [Operaciones](../backend/06-operations.md): `paperdl` y `mojangdl`
- [Referencia de la API](../reference/11-api-reference.md) · [Referencia de scopes](../general/scopes-reference.md) · [Configuración](../backend/02-configuration.md)
