# Extensiones

Una extensión añade funciones al panel sin tocar su código. Puede hacer dos cosas:

- **Páginas**: archivos HTML que el panel muestra como una pestaña en cada servidor o como una entrada del menú de administración.
- **Eventos**: el panel avisa a una dirección `https://` de la extensión cuando pasa algo (un servidor se crea, arranca, se cae…).

Las instala un administrador en **Ajustes → Extensiones**, subiendo un `.zip`. Una extensión recién instalada está **apagada**: no muestra nada ni recibe eventos hasta que la enciendes.

## El paquete

Un `.zip` con `extension.json` en la raíz (o dentro de una única carpeta) y los archivos de sus páginas:

```
mi-extension.zip
├── extension.json
├── index.html
└── js/app.js
```

Límites: 20 MB comprimido, 50 MB descomprimido y 500 archivos. No se admiten enlaces simbólicos ni rutas que salgan de la carpeta.

### `extension.json`

```json
{
  "id": "estadisticas",
  "name": "Estadísticas de jugadores",
  "version": "1.0.0",
  "author": "Tu nombre",
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
| `id` | sí | 2 a 40 letras minúsculas, números o guiones. Instalar otro paquete con el mismo `id` lo actualiza. |
| `name` | sí | Hasta 60 caracteres. |
| `version` | sí | Hasta 20 caracteres, por ejemplo `1.0.0`. |
| `author`, `description` | no | Se muestran en Ajustes. |
| `pages` | si no hay `webhook` | Hasta 10. `placement` es `server` (pestaña en cada servidor) o `admin` (menú de administración, solo administradores). `path` es un `.html` del paquete. |
| `webhook` | si hay `events` | Dirección `https://` que recibe los eventos. El administrador puede cambiarla en Ajustes. |
| `events` | no | Los eventos que quiere recibir (lista abajo). |

Un campo que el panel no conoce hace que la instalación falle, para que una errata no pase desapercibida.

## Páginas

El panel muestra cada página dentro de un marco **aislado** (`sandbox` sin `allow-same-origin`): sus scripts se ejecutan, pero en un origen opaco, así que no pueden leer la sesión del administrador ni llamar a la API del panel en su nombre. Tampoco tienen `localStorage` ni cookies propias.

Al cargarse, la página recibe un mensaje con el contexto:

```js
window.addEventListener('message', (event) => {
  if (event.data?.type !== 'aether:context') return;
  const { serverId, language, theme } = event.data;
  // serverId: el servidor de la pestaña (null en una página de administración)
  // language: "es" o "en"; theme: "dark" o "light"
});
```

Para ajustar la altura del marco a su contenido, la página puede contestar:

```js
parent.postMessage({ type: 'aether:resize', height: document.body.scrollHeight }, '*');
```

Si una página necesita datos, los pide a su propio servicio (el del webhook, por ejemplo). Los archivos de las páginas son públicos mientras la extensión está encendida: no pongas claves ni secretos en ellos.

## Eventos

| Evento | Cuándo |
|--------|--------|
| `server.created` | Se crea un servidor (también desde WHMCS/Paymenter). |
| `server.deleted` | Se borra un servidor. |
| `server.started` | El servidor queda encendido. |
| `server.stopped` | El servidor queda apagado. |
| `server.crashed` | El servidor se cae solo. Incluye `exitCode`, `restarted` y las últimas líneas de la consola en `lines`. |
| `server.backup` | Termina una copia de seguridad. `success` dice si salió bien. |
| `server.resources` | La CPU o la memoria se mantienen altas. Incluye `resource`, `value` y `threshold`. |

Cada evento es un `POST` con JSON:

```json
{
  "event": "server.crashed",
  "extension": "estadisticas",
  "timestamp": "2026-10-26T18:04:11Z",
  "server": { "id": "a1b2c3d4", "name": "Survival" },
  "title": "💥 Caída del servidor",
  "message": "Survival se ha caído (código de salida 1). No se reinició automáticamente.",
  "exitCode": 1,
  "restarted": false,
  "lines": ["..."]
}
```

Si la respuesta no es `2xx`, el panel lo reintenta a los 5 y a los 30 segundos. Contesta rápido y haz el trabajo después.

Los eventos `server.started`, `server.stopped`, `server.crashed`, `server.backup` y `server.resources` salen del nodo donde corre el servidor. Solo se envían desde un nodo que también ejecuta el panel (el caso normal con un solo equipo).

### Comprobar la firma

Cada evento lleva la cabecera `X-Aether-Signature: sha256=<hex>`: el HMAC-SHA256 del cuerpo tal cual llega, con la clave de firma que muestra Ajustes → Extensiones. Rechaza los avisos cuya firma no coincida:

```js
import crypto from 'node:crypto';

function firmaValida(cuerpo, cabecera, clave) {
  const esperada = 'sha256=' + crypto.createHmac('sha256', clave).update(cuerpo).digest('hex');
  return cabecera?.length === esperada.length &&
    crypto.timingSafeEqual(Buffer.from(cabecera), Buffer.from(esperada));
}
```

El botón **Enviar prueba** de Ajustes manda un evento `test` firmado para comprobar la dirección y la clave.

## API

| Método | Ruta | Quién |
|--------|------|-------|
| GET | `/api/extensions` | administradores: todas, con su clave de firma |
| POST | `/api/extensions` | administradores: instala o actualiza (campo `file` de un formulario multipart) |
| PUT | `/api/extensions/:id` | administradores: `{ "enabled": true, "webhookUrl": "https://..." }` |
| DELETE | `/api/extensions/:id` | administradores: desinstala |
| POST | `/api/extensions/:id/test` | administradores: envía un evento de prueba |
| GET | `/api/extensions/active` | cualquier usuario: páginas de las extensiones encendidas (las de administración, solo a administradores) |
| GET | `/api/extension-files/:id/*` | los archivos de las páginas, siempre con `Content-Security-Policy: sandbox` |
