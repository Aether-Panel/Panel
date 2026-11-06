# Ajustes del panel repartidos a los nodos

Fecha: 2026-11-06 · Estado: aprobado por el usuario en el chat

## Por qué

Algunos ajustes que se editan en el panel los usa el **daemon** de cada nodo:
- las copias externas (S3), con su copia periódica;
- la clave de CurseForge.

Hoy la página Configuración solo escribe el `config.json` del panel, que es su nodo local. Cada nodo extra necesita las mismas claves puestas **a mano** en su propio `config.json`. El usuario quiere que una actualización deje todo funcionando sin pasos manuales.

## Decisiones del usuario

- **Mismo S3 para todos.** Lo que se guarda en Configuración vale para todos los nodos. No hay excepciones por nodo, así que el panel sobrescribe lo que un nodo tuviera puesto a mano.
- **Un único mecanismo** para todos los ajustes de daemon que edita el panel: S3 y CurseForge. Un ajuste nuevo de este tipo se añade a la lista y ya está.

## Diseño

### Lista de claves repartidas (fuente única)

Un solo slice en `internal/config`, que usan el panel (para leer y empaquetar) y el nodo (como lista blanca de lo que acepta):
- `daemon.backups.s3.*` (todas): `enabled`, `endpoint`, `region`, `bucket`, `accessKey`, `secretKey`, `prefix`, `insecure`, `periodicHours` y `periodicKeep`;
- `daemon.curseforge.key`.

### Nodo (daemon)

**`PUT /daemon/settings`**, con `middleware.ValidateJWT`, la misma protección que `/daemon/update`:
- Solo acepta tokens que el panel firma para ese nodo; el destinatario se deriva del secreto del nodo.
- Recibe un objeto `{ "<clave>": valor }` con exactamente las claves de la lista.
  - Una clave que no esté en la lista da 400 y no se aplica nada.
  - Los valores se validan con las mismas reglas que en el panel. Por ejemplo, `periodicHours` y `periodicKeep` con `ValidPeriodicHours` y el rango 1-50.
- Cada valor se guarda con el `Set(value, true)` de su entrada de configuración, que lo persiste en el `config.json` del nodo y lo aplica en memoria sin reiniciar.
  - El S3 ya se lee en cada llamada con `offsite.FromSettings()`.
  - La copia periódica lee `EffectivePeriodic` en cada vuelta.
- Responde 204.

**`GET /daemon/settings`**, también con `ValidateJWT`:
- Devuelve `{ "hash": "<sha256 hex>" }`, calculado sobre el JSON canónico de los valores actuales de esas claves.
- Nunca devuelve los valores, porque incluyen secretos.

**Nodo antiguo:** responde 404 a las dos rutas.

### Panel

**Servicio de reparto.** Por ejemplo, en `internal/services/node_settings.go`:
- `NodeSettingsPayload()` lee los valores de la lista desde la configuración del panel y calcula el mismo hash.
- Para cada **nodo remoto** (se excluye `LocalNode`, que es el propio panel y comparte su `config.json`):
  1. `GET /daemon/settings`;
  2. si el hash no coincide, `PUT /daemon/settings` con el paquete completo;
  3. se guarda en memoria el estado del nodo:
     - `synced`: hashes iguales, o PUT correcto;
     - `pending`: error de red o nodo apagado; se reintenta;
     - `outdated`: 404, nodo sin actualizar;
     - `failed`: el nodo rechazó el paquete; se guarda el mensaje.

**Cuándo se reparte:**
- **Al guardar en Configuración** una clave de la lista, ya sea con un solo ajuste o con varios a la vez. El reparto va en segundo plano: no hace esperar la respuesta de guardado y le sigue una pasada a todos los nodos.
- **Cada 10 minutos**, con un trabajador del panel. La primera pasada es 1 minuto después de arrancar. Así se ponen al día:
  - un nodo que estaba apagado al guardar;
  - un nodo nuevo recién unido;
  - un nodo que se acaba de actualizar.

  El trabajador procesa los nodos de uno en uno, con un tiempo máximo por nodo (10 s).
- **Nunca dos pasadas a la vez:** si ya hay una en marcha, la nueva se encola como «otra pasada después».

**API para la interfaz:** `GET /api/settings/node-sync` devuelve, por cada nodo remoto, `{ id, name, state, error?, checkedAt }`.
- Lleva el mismo permiso que leer la configuración del panel.
- No incluye ningún valor de ajuste.

### Interfaz

En Configuración › Copias externas, el aviso actual «Nodos remotos: … cada nodo remoto lee las claves `daemon.backups.s3.*` de su propio config.json» se sustituye por una lista pequeña «Nodos» con una línea por nodo remoto:

| Estado | Texto |
|---|---|
| `synced` | «Al día» |
| `pending` | «Pendiente, se reintenta solo» |
| `outdated` | «Nodo sin actualizar: actualízalo para que reciba los ajustes» |
| `failed` | «Error: <mensaje>» |

- La misma información sirve para CurseForge: basta con un texto que diga que estos ajustes y la clave de CurseForge se envían solos a todos los nodos.
- Si no hay nodos remotos, no se muestra nada.
- Debe ser responsive y llevar los textos en es y en.

### Documentación

Hay que actualizar en es y en:
- `features/backups.md` (requisitos y nodos remotos);
- `features/panel-settings.md`;
- `backend/02-configuration.md`;
- `reference/api/09-daemon.md` (las rutas nuevas);
- la mención de CurseForge en nodos remotos donde exista.

Se quitan las instrucciones manuales y se explica que basta con actualizar el nodo.

## Anti-objetivos

- Que un usuario o un servidor de juego pueda escribir esos ajustes en un nodo: solo el panel puede, mediante `ValidateJWT` con el destinatario del nodo.
- Que un GET devuelva secretos.
- Que guardar en Configuración se bloquee esperando a nodos lentos o apagados.
- Que un paquete con claves fuera de la lista o con valores inválidos deje el nodo a medio aplicar: se valida todo antes de guardar nada.
