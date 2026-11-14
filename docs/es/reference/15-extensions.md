# Extensiones

Una extensión añade funciones al panel sin tocar su código. Puede hacer dos cosas:

- **Páginas**: archivos HTML que el panel muestra como una pestaña en cada servidor o como una entrada del menú de administración.
- **Eventos**: el panel avisa a una dirección `https://` de la extensión cuando pasa algo (un servidor se crea, arranca, se cae…).

Las instala un administrador en **Configuración → Extensiones**, subiendo un `.zip`. Una extensión recién instalada está **apagada**: no muestra nada ni recibe eventos hasta que un administrador la enciende.

Código: `internal/extensions` (manifiesto, instalación, entrega de eventos), `internal/web/api/extensions.go` (API y servicio de archivos), `client/frontend/src/features/extensions` (el marco).

## El paquete

Un `.zip` con `extension.json` en la raíz (o dentro de una única carpeta) y los archivos de sus páginas:

```
mi-extension.zip
├── extension.json
├── index.html
└── js/app.js
```

Límites: 20 MB comprimido, 50 MB descomprimido, 500 archivos y 64 KB para `extension.json`. No se admiten enlaces simbólicos, barras invertidas ni rutas que salgan de la carpeta; las entradas `__MACOSX` se ignoran. El tamaño descomprimido se controla al escribir, digan lo que digan los tamaños que declara el zip.

### `extension.json`

```json
{
  "id": "estadisticas",
  "name": "Estadísticas de jugadores",
  "version": "1.0.0",
  "author": "Su nombre",
  "description": "Gráficas de jugadores por servidor.",
  "pages": [
    { "id": "jugadores", "title": "Jugadores", "placement": "server", "path": "index.html" },
    { "id": "resumen", "title": "Resumen", "placement": "admin", "path": "admin.html" }
  ],
  "webhook": "https://mi-servicio.example.com/aether",
  "events": ["server.started", "server.stopped", "server.crashed"]
}
```

| Campo | Obligatorio | Qué es |
|-------|-------------|--------|
| `id` | sí | De 2 a 40 letras minúsculas, números o guiones, empezando por letra o número. Instalar otro paquete con el mismo `id` lo actualiza. |
| `name` | sí | Hasta 60 caracteres. |
| `version` | sí | Hasta 20 caracteres entre letras, números, `.`, `+` y `-`, por ejemplo `1.0.0`. |
| `author` | no | Hasta 60 caracteres. Se muestra en Configuración. |
| `description` | no | Hasta 300 caracteres. Se muestra en Configuración. |
| `pages` | si no hay `webhook` | Hasta 10 (ver abajo). |
| `webhook` | si hay `events` | Dirección `https://` que recibe los eventos, sin `usuario:contraseña@`. El administrador puede cambiarla en Configuración. |
| `events` | no | Los eventos que quiere recibir ([lista abajo](#eventos)). Requiere `webhook`. |
| `servers` | no | Los servidores para los que es la extensión ([ver abajo](#solo-para-algunos-servidores)). Sin él, todos. |

Una extensión necesita al menos una página o un webhook. Un campo que el panel no conoce hace que la instalación falle, para que una errata no pase desapercibida.

Cada página:

| Campo | Qué es |
|-------|--------|
| `id` | De 1 a 40 letras minúsculas, números o guiones, único dentro de la extensión |
| `title` | De 1 a 40 caracteres: el texto de la pestaña o del menú |
| `placement` | `server` (pestaña en cada servidor) o `admin` (menú de administración, solo administradores) |
| `path` | Un archivo `.html` del paquete, como ruta relativa limpia (`index.html`, `pages/a.html`) |

### Solo para algunos servidores

Una extensión hecha para un juego o un tipo de servidor concreto lo dice con `servers`, y entonces sus páginas de servidor solo salen en esos servidores y sus eventos solo llegan de ellos. Las páginas de administración no cambian.

```json
"servers": { "types": ["armareforger"] }
```

| Campo | Qué es |
|-------|--------|
| `types` | Tipos de servidor: el `type` de la plantilla con la que se creó (`armareforger`, `minecraft-java`, `nodejs`…) |
| `icons` | Juegos: el `icon` de la plantilla, para los que comparten tipo (`csgo`, `rust` y `tf2` son todos `srcds`) |

Un servidor encaja si su tipo está en `types` **o** su icono en `icons`. Cada lista admite hasta 30 valores, en minúsculas (se convierten), con letras, números, `.`, `_` y `-`. `servers` vacío hace que la instalación falle. Un servidor cuyo tipo e icono no se conocen no encaja con ninguna extensión que tenga `servers`.

## Instalar, actualizar y desinstalar

- **Instalar**: el paquete se comprueba, se descomprime en una carpeta temporal y después se mueve a `<panel.extensions.folder>/<id>` (por defecto `<data>/extensions/<id>`). Una extensión nueva recibe su propia clave de firma aleatoria y empieza apagada.
- **Actualizar**: subir un paquete con un `id` ya instalado sustituye sus archivos de forma atómica (una actualización fallida deja intacta la versión anterior) y conserva su interruptor de encendido, su clave de firma y, si el administrador lo cambió, su webhook.
- **Desinstalar**: borra el registro y la carpeta de la extensión.

Los cambios se aplican al momento: la lista de extensiones activas se recarga tras cada instalación, actualización, cambio de interruptor o desinstalación.

## Páginas

El panel muestra cada página dentro de un marco **aislado**: `sandbox="allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-modals allow-downloads"`, sin `allow-same-origin`, y `referrerpolicy="no-referrer"`. Sus scripts se ejecutan, pero en un origen opaco, así que no pueden leer la sesión del administrador ni llamar a la API del panel en su nombre. Tampoco tienen `localStorage` ni cookies propias. Un enlace con `target="_blank"` abre el otro sitio en una pestaña normal (`allow-popups-to-escape-sandbox`), con su propia sesión: así una página puede llevar a su servicio sin quedar aislada.

Los archivos se sirven desde `/api/extension-files/<id>/<ruta>` con el mismo sandbox como `Content-Security-Policy`, de modo que el aislamiento se mantiene aunque alguien abra un archivo directamente. Solo se sirven archivos normales de una extensión **encendida** (cualquier otra cosa responde `404`), con `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: SAMEORIGIN` y `Cache-Control: no-cache`; el tipo de contenido se deduce de la extensión del archivo. Los archivos de las páginas son públicos mientras la extensión está encendida: no ponga claves ni secretos en ellos.

Al cargarse, la página recibe un mensaje con el contexto, y lo vuelve a recibir cada vez que el usuario pasa a otro servidor, cambia el idioma o cambia el tema:

```js
window.addEventListener('message', (event) => {
  if (event.source !== window.parent || event.data?.type !== 'aether:context') return;
  const { serverId, panelUrl, language, theme, colors } = event.data;
  // serverId: el servidor de la pestaña (null en una página de administración)
  // panelUrl: la dirección del panel (https://panel.ejemplo.com), la de Ajustes: la misma que el campo panel de los eventos; la página no puede verla por sí misma
  // language: "es" o "en"; theme: "dark" o "light" (el que había al enviar el mensaje)
  // colors: los colores del tema del panel, ya resueltos (rgb(...)): background, foreground, card,
  //   cardForeground, muted, mutedForeground, primary, primaryForeground, border, destructive,
  //   success; además radius ("0.625rem") y font (la familia tipográfica del panel)
});
```

Para que la página parezca parte del panel: deje el `body` transparente (el marco ya tiene el fondo y el borde de una tarjeta del panel), declare `<meta name="color-scheme" content="light dark">` (si no, el navegador pinta un fondo opaco cuando el tema del panel y el de la página no coinciden), y aplique `colors` como variables CSS:

```js
for (const [name, value] of Object.entries(colors ?? {})) {
  document.documentElement.style.setProperty(`--aether-${name}`, value);
}
document.documentElement.style.colorScheme = theme;
// en el CSS: button { background: var(--aether-primary); color: var(--aether-primaryForeground);
//   border-radius: var(--aether-radius); } body { font-family: var(--aether-font); }
```

Para ajustar la altura del marco a su contenido, la página puede contestar:

```js
parent.postMessage({ type: 'aether:resize', height: document.body.scrollHeight }, '*');
```

El panel solo acepta ese mensaje desde el marco de la propia página (cuyo origen siempre es el opaco `"null"`) y mantiene la altura entre 200 y 4000 píxeles.

Si una página necesita datos, los pide a su propio servicio (el del webhook, por ejemplo).

## Eventos

| Evento | Cuándo | Campos adicionales |
|--------|--------|--------------------|
| `server.created` | Se crea un servidor (desde el panel, o desde WHMCS/Paymenter a través de `/api/v1`). | — |
| `server.deleted` | Se borra un servidor. | — |
| `server.started` | El servidor queda encendido. | `running: true` |
| `server.stopped` | El servidor queda apagado. | `running: false` |
| `server.crashed` | El servidor se cae solo. | `exitCode`, `restarted`, `lines` (hasta las 12 últimas líneas de la consola) |
| `server.backup` | Termina una copia de seguridad. | `success` |
| `server.resources` | La CPU o la memoria se mantienen altas. | `resource`, `value`, `threshold` |

Cada evento es un `POST` con JSON, `Content-Type: application/json` y `User-Agent: AetherPanel-Webhook/1`:

```json
{
  "event": "server.crashed",
  "extension": "estadisticas",
  "timestamp": "2026-10-26T18:04:11Z",
  "panel": "https://panel.example.com",
  "server": { "id": "a1b2c3d4", "name": "Survival", "type": "minecraft-java", "icon": "minecraft-java" },
  "title": "💥 Caída del servidor",
  "message": "Survival se ha caído (código de salida 1). No se reinició automáticamente.",
  "exitCode": 1,
  "restarted": false,
  "lines": ["..."]
}
```

Todo evento lleva `event`, `extension`, `timestamp` (UTC, RFC 3339) y `server`, con su `id`, su `name` y, cuando se conocen, el `type` y el `icon` de su plantilla. Los eventos del nodo (`started`, `stopped`, `crashed`, `backup`, `resources`) llevan además el `title` y el `message` de las notificaciones de servidor del panel, que hoy están escritos en español; base la lógica en `event` y en los campos adicionales, no en ese texto.

Entrega:

- Cada intento tiene un tiempo máximo de 10 segundos. Si falla o la respuesta no es `2xx`, el panel lo reintenta a los 5 y después a los 30 segundos; tras el tercer fallo el evento solo se registra en el log. Conteste rápido y haga el trabajo después.
- Como máximo hay 32 entregas en curso a la vez entre todas las extensiones. Por encima de eso, los eventos nuevos se descartan (y se registran) en lugar de encolarse.
- Los eventos se envían sea cual sea la configuración de notificaciones del propietario del servidor.

Los eventos `server.started`, `server.stopped`, `server.crashed`, `server.backup` y `server.resources` salen del nodo donde corre el servidor. Solo se envían desde un nodo que también ejecuta el panel (el caso normal con un solo equipo); un nodo solo daemon no los entrega.

### Comprobar la firma

Cada evento lleva la cabecera `X-Aether-Signature: sha256=<hex>`: el HMAC-SHA256 del cuerpo tal cual llega, con la clave de firma que muestra Configuración → Extensiones. Rechace los avisos cuya firma no coincida:

```js
import crypto from 'node:crypto';

function firmaValida(cuerpo, cabecera, clave) {
  const esperada = 'sha256=' + crypto.createHmac('sha256', clave).update(cuerpo).digest('hex');
  return cabecera?.length === esperada.length &&
    crypto.timingSafeEqual(Buffer.from(cabecera), Buffer.from(esperada));
}
```

El botón **Enviar prueba** de Configuración manda un evento `test` firmado (solo con `event`, `extension`, `timestamp` y `panel`) para comprobar la dirección y la clave.

### Comprobar de qué panel viene

La clave de firma prueba que quien envía tiene esa clave, no de qué panel viene: el administrador de cualquier panel ve la suya. Un servicio que recibe eventos de muchos paneles (por ejemplo, una extensión que instalan empresas de hosting para sus clientes) necesita saber cuál de ellos envía, y para eso cada evento lleva además:

- `panel` en el cuerpo: la dirección del panel (`panel.settings.masterUrl`, sin barra final y con `https://` delante si se escribió solo el nombre).
- La cabecera `X-Aether-Panel-Signature: ed25519=<base64>`: la firma Ed25519 del cuerpo tal cual llega, hecha con la **clave de identidad del panel**. Es la misma clave con la que firma las transferencias entre paneles; se crea una vez y se guarda en `panel.exTransferKey`.

El panel publica su clave pública en su propia dirección, sin sesión:

```
GET https://panel.ejemplo.com/.well-known/aether-panel.json
{ "url": "https://panel.ejemplo.com", "publicKey": "<base64>", "algorithm": "ed25519", "signatureHeader": "X-Aether-Panel-Signature" }
```

El servicio descarga esa clave por HTTPS de la dirección del panel con el que se vincula (el HTTPS prueba que viene de ese dominio), la guarda, y comprueba con ella cada evento. Además, `panel` dentro del cuerpo firmado debe ser esa misma dirección. Nadie más puede firmar en nombre de ese panel: la clave privada no sale de él.

```js
import crypto from 'node:crypto';
function delPanel(cuerpo, cabecera, clavePublicaBase64) {
  const firma = Buffer.from(String(cabecera).replace(/^ed25519=/, ''), 'base64');
  const clave = crypto.createPublicKey({ key: Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), Buffer.from(clavePublicaBase64, 'base64')]), format: 'der', type: 'spki' });
  return crypto.verify(null, cuerpo, clave, firma);
}
```

Si el panel cambia de clave (por ejemplo, al reinstalarlo sin su configuración), el servicio tiene que volver a descargarla.

## API

| Método | Ruta | Quién | Respuesta |
|--------|------|-------|-----------|
| GET | `/api/extensions` | administradores | Todas las extensiones con sus páginas (cada una con su `url`), `events`, `manifestWebhook`, `webhookUrl`, `enabled` y la clave de firma (`secret`) |
| POST | `/api/extensions` | administradores | Instala o actualiza: el zip en el campo `file` de un formulario multipart. `200` con la extensión, o `400` con `ErrExtensionInvalid` y el motivo |
| PUT | `/api/extensions/:id` | administradores | `{ "enabled": true, "webhookUrl": "https://..." }`, ambos opcionales. Un `webhookUrl` vacío detiene los eventos. `404` si no está instalada |
| DELETE | `/api/extensions/:id` | administradores | Desinstala. `204`, o `404` |
| POST | `/api/extensions/:id/test` | administradores | Envía un evento de prueba. `204`; `400` con `ErrExtensionNoWebhook` o `ErrExtensionWebhook` (el error de la entrega) |
| GET | `/api/extensions/active` | cualquier usuario con sesión | Páginas de las extensiones encendidas: `[{ id, name, pages, servers }]` (`servers` solo si la extensión lo declara). Las de administración, solo para administradores |
| GET, HEAD | `/api/extension-files/:id/*path` | público | Los archivos de las páginas, siempre con `Content-Security-Policy: sandbox ...` |

«Administradores» significa el scope `admin`; «cualquier usuario con sesión», el scope `login`.
