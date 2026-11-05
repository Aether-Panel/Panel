# Copias en S3 periódicas y por tarea

Fecha: 2026-11-05 · Estado: listo para aprobar

## Por qué

Hoy, con el S3 activado (Configuración › Copias externas), **cada** copia del servidor se sube al bucket: las manuales, las de cada ejecución de una tarea y nada más. Eso tiene dos problemas:

- Una tarea que copia cada hora sube 24 copias al día al S3 (coste, ancho de banda, tiempo de subida de hasta 2 h por copia), cuando para un desastre basta con una de vez en cuando.
- Un servidor sin tareas de copia no tiene **ninguna** copia en el S3 salvo que alguien la haga a mano.

Además, al podar las copias automáticas de una tarea («conservar N»), `DeleteBackup` borra la local **y** la del S3, así que el S3 nunca guarda más historia que el servidor.

Lo que pide el usuario: que el S3 reciba copias *de vez en cuando* (cada 12/24/48/72 h o más) aunque el servidor no tenga tareas, y que en una tarea de copia se pueda decidir que solo algunas ejecuciones (cada fin de semana, cada 15 días…) suban al S3 y el resto se quede en el servidor.

## Objetivos

1. **Copia periódica al S3 sin tareas.** Cada servidor recibe una copia en el S3 cada X horas, hecha por el propio nodo.
   - El admin fija el valor por defecto en Configuración › Copias externas: desactivada, 12, 24, 48, 72 h, 7, 14 o 30 días.
   - Cada servidor puede cambiarlo en su pestaña Copias: «por defecto», desactivada u otro intervalo.
2. **Tareas: frecuencia de subida al S3.** El paso «Copia de seguridad» de una tarea tiene un desplegable. Las ejecuciones a las que no les toca subir solo guardan la copia en el servidor.
   - Opciones: Nunca / Siempre / Cada fin de semana / Cada semana / Cada 15 días / Cada mes.
3. **Retención propia del S3.**
   - Cada tarea tiene «conservar en S3» (1-50, por defecto 4); al subir una copia nueva se borran del bucket las más antiguas de esa tarea.
   - La copia periódica tiene su propio «conservar en S3»: el admin pone el valor por defecto (4) y cada servidor puede cambiarlo.
   - Podar las copias locales («conservar N» de la tarea) ya **no** borra las del S3.
4. **Copias manuales: solo en el servidor.** Ya no se suben al S3 (decisión del usuario).

## No objetivos

- No cambia cómo se configura el bucket ni que un nodo remoto necesite sus propias claves `daemon.backups.s3.*` (limitación ya documentada en `docs/*/features/backups.md`). Las claves nuevas siguen la misma regla.
- No hay botón «subir esta copia al S3» para una copia manual (posible mejora futura).
- No se reescriben copias ya subidas ni se renombran archivos existentes.
- La poda local de las copias de tareas sigue contando todas las `auto-` juntas, como hoy.

## Anti-objetivos (sería un fallo aunque funcione)

- Que todos los servidores de un nodo hagan su copia periódica a la vez y saturen disco, CPU o red.
- Que una tarea existente deje de subir al S3 sin que nadie lo haya pedido: las tareas guardadas antes de este cambio, sin el campo nuevo, se comportan como «Siempre».
- Que podar el servidor o el S3 borre copias de otra tarea, de la copia periódica o hechas a mano.
- Que una subida fallida haga fallar la tarea o deje el servidor bloqueado.

## Decisiones tomadas por el usuario

- Lo de «cada X horas sin tarea» es una copia periódica automática que hace el nodo, no un límite a las subidas.
- El intervalo: valor global del admin, y cada servidor puede cambiarlo.
- En tareas: desplegable de frecuencias (Nunca / Siempre / Cada fin de semana / Cada semana / Cada 15 días / Cada mes).
- Retención del S3: número propio de copias (por defecto 4); podar el servidor ya no borra el S3.
- Las copias manuales se quedan solo en el servidor.
- La copia periódica se hace aunque el servidor de juego esté encendido (en caliente, como una copia manual); nunca lo para.
- Avisos de la copia periódica: en silencio si va bien; al dueño y a Discord solo si fallan la copia o la subida. `endBackup` necesita un camino silencioso para el éxito. Las copias manuales y de tareas siguen avisando como hoy.

## Diseño

### Nombres de archivo (lo que distingue cada serie)

| Origen | Nombre | ¿Sube al S3? |
|---|---|---|
| Manual | `<uuid>.tar.gz` (como hoy) | No |
| Tarea | `auto-<taskId>-<uuid>.tar.gz` (antes `auto-<uuid>`) | Según la frecuencia de la tarea |
| Copia periódica | `offsite-<uuid>.tar.gz` (prefijo nuevo `OffsiteBackupPrefix`) | Siempre (es su razón de ser) |
| Punto de restauración | `restore-…` (como hoy) | No |

Llevar el `taskId` en el nombre permite que la frecuencia y la retención del S3 de una tarea miren solo sus propias copias en el bucket.

- Las `auto-<uuid>` antiguas siguen siendo copias de tarea a todos los efectos: `isSystemBackup` y la poda local. Ninguna tarea las reconoce como suyas, así que la retención del S3 no las toca: se quedan hasta que alguien las borre.
- El panel debe reconocer `offsite-` como copia del sistema (`isSystemBackup` en `internal/web/api/tasks.go`), con el nombre «Copia periódica (S3)» igual que hace con «Automatic backup».
- El `taskId` ya cumple `^[A-Za-z0-9_-]{1,64}$`, que admite `-`. Por eso, para saber de qué tarea es un archivo **no** basta comparar el principio del nombre: la tarea `a` coincidiría con las copias de `a-b`. El id se obtiene quitando del nombre el final fijo `-<uuid de 36 caracteres>.tar.gz`.
- El operador de la tarea hoy no conoce su id. El scheduler debe pasárselo al crear las operaciones, siempre él y nunca leído de los argumentos guardados: si no, un usuario sin permisos de admin podría poner el id de otra tarea y podar sus copias del bucket.
- **Subidas en curso.** La subida va en segundo plano, después de que `endBackup` libere `backupState`. Mientras sube, la siguiente ejecución de la tarea o el trabajador periódico no la ven en el bucket. Hay que llevar en memoria qué series (tarea o periódica, por servidor) tienen una subida en marcha y no empezar otra de la misma serie hasta que acabe.

### Cuándo sube al S3 una ejecución de tarea

Argumentos nuevos de la operación `backup` (junto a `keep`):

- `offsite`: `never` | `always` | `weekend` | `weekly` | `biweekly` | `monthly`.
  - Si falta, vale `always` (compatibilidad con las tareas existentes).
  - Una tarea nueva creada desde el panel trae `weekly` por defecto.
- `offsiteKeep`: 1-50, por defecto 4.
- Las tareas de usuarios sin permisos de admin solo aceptan los argumentos de `userTaskOperations` (`internal/web/api/tasks.go`), así que hay que añadir `offsite` y `offsiteKeep` allí y validarlos en `validateUserTaskStep`.

La decisión se toma **después** de escribir la copia local. Se calcula con la última copia de **esa tarea** en el bucket (la de `ModTime` más reciente entre los objetos `auto-<taskId>-*`) y en la zona horaria del scheduler del servidor:

| Valor | Sube si… |
|---|---|
| `never` | nunca |
| `always` | siempre |
| `weekend` | hoy es sábado o domingo **y** no hay copia de la tarea desde el sábado 00:00 de este fin de semana |
| `weekly` | no hay copia de la tarea desde el lunes 00:00 de esta semana |
| `biweekly` | la última copia de la tarea tiene 15 días o más, con 2 h de margen para que una tarea diaria a la misma hora no se salte un día |
| `monthly` | no hay copia de la tarea desde el día 1 00:00 de este mes |

- Si no se puede listar el bucket, la ejecución no sube nada, lo deja en el log de la consola y la tarea no falla. La siguiente ejecución lo vuelve a intentar.
- La subida sigue siendo en segundo plano, como hoy (`uploadOffsite`). Al terminar bien, se borran del bucket las copias de esa tarea más allá de `offsiteKeep`, de la más antigua a la más reciente.

### Copia periódica al S3 (sin tareas)

Claves nuevas del daemon, que se editan en Configuración › Copias externas:

- `daemon.backups.s3.periodicHours`: entero; 0 = desactivada (valor por defecto 0, así nada cambia hasta que el admin lo encienda).
- `daemon.backups.s3.periodicKeep`: entero 1-50, por defecto 4.

Ajuste por servidor, guardado en el archivo del scheduler (`<id>.cron`), que ya es por servidor y vive en el nodo. El traslado entre nodos **no** copia ese archivo (`copyTasks` en `internal/web/api/transfer_logic.go` vuelve a crear las tareas una a una), así que el traslado (y `extransfer`, si aplica) debe copiar también este ajuste con `GET`/`PUT /daemon/server/:id/offsite`:

```json
"offsite": { "periodicHours": null, "periodicKeep": null }
```

- `null` significa «por defecto». Con `periodicHours: 0`, ese servidor no hace copia periódica.
- Valores permitidos en los dos sitios: 0, 12, 24, 48, 72, 168, 336, 720.

Funcionamiento en el nodo:

- **Un único trabajador por nodo.** Cada 10 minutos recorre los servidores de uno en uno.
- **Cuándo hace la copia de un servidor.** Solo si se cumplen todas estas condiciones:
  - el S3 está configurado en ese nodo y el intervalo efectivo es mayor que 0;
  - no hay otra copia o restauración en curso (`backupState`);
  - el servidor no se está instalando (`GetEnvironment().IsInstalling()`, como en `IsIdle`) ni trasladando (el scheduler no está en `Paused`, que solo marcan los traslados);
  - la última copia `offsite-` de ese servidor en el bucket es más antigua que el intervalo.
- **De dónde sale la «última copia».** Se guarda en memoria. La primera vez se lee del bucket y después se actualiza con cada subida buena, para no listar el bucket cada 10 minutos.
- **La copia y su subida.** La copia se hace con el mismo código que hoy (`beginBackup(OffsiteBackupPrefix)` + `writeArchive`), espera a que la subida termine y entonces poda el bucket a `periodicKeep` copias `offsite-`.
- **Tras subir, se borra la copia local** *(decisión mía, ver abajo)*: esta copia existe para estar fuera del nodo, y así no ocupa el disco del servidor.
  - Si la subida falla, la copia local se borra igualmente, para no llenar el disco con reintentos.
  - Antes de escribir una copia, el trabajador hace una comprobación barata del bucket (`BucketExists` o un listado del prefijo del servidor). Si el bucket no responde, no se escribe nada.
  - Tras un fallo, la espera crece por servidor: 1 h, 2 h, 4 h… hasta un máximo de 24 h. Una subida buena la reinicia. Así, un bucket caído un día entero no fabrica copias que luego se tiran.
- **Al arrancar el nodo**, el primer recorrido espera unos minutos, para no hacer copias justo durante el arranque.

### Lo que deja de pasar

- `finishBackup` ya no sube **todas** las copias. Sube solo las de una tarea a la que le toca y las periódicas.
- La poda local (`pruneAutoBackups`) borra solo el archivo local, con una variante de `DeleteBackup` que no toca el bucket.
- `DeleteBackup`, cuando lo pide un usuario desde el panel, sigue borrando las dos copias, como hoy.

### API

**Daemon (nodo), nuevos:**

- `GET /daemon/server/:id/offsite`: devuelve `{ periodicHours, periodicKeep, effectiveHours, effectiveKeep, configured, lastPeriodic }`.
  - `configured` dice si ese nodo tiene el S3 configurado.
- `PUT /daemon/server/:id/offsite`: recibe `{ periodicHours, periodicKeep }`, y acepta `null` para volver al valor por defecto.

**Panel, nuevos:**

- `GET /api/servers/:id/offsite` y `PUT /api/servers/:id/offsite`: reenvían la petición al nodo del servidor.
- Permisos: leer, con el mismo scope que ver las copias; cambiar, con el mismo que crear copias.

**Configuración:** `periodicHours` y `periodicKeep` se añaden a `editableIntEntries` en `internal/web/api/settings.go`. Esa lista está vacía hoy, así que el camino para enteros no se ha usado nunca. Hay que validar los valores permitidos (horas: 0/12/24/48/72/168/336/720; conservar: 1-50) y probarlo.

### Interfaz (React, responsive en móvil, tablet y escritorio)

1. **Configuración › Copias externas (`offsite-tab.tsx`).** Una sección nueva, «Copia periódica de cada servidor», con dos controles:
   - un desplegable de intervalo (Desactivada, 12 h, 24 h, 48 h, 72 h, 7 días, 14 días, 30 días);
   - «Copias a conservar en S3» (1-50).

   Debajo, el mismo aviso de nodos remotos que ya existe.
2. **Servidor › Copias (`backups-view.tsx`).** Una tarjeta pequeña, «Copia al S3», con:
   - un desplegable de intervalo con «Por defecto (24 h)» primero, y «Conservar en S3»;
   - la fecha de la última copia periódica, si existe.

   Si el nodo no tiene S3 configurado, la tarjeta lo dice y desactiva los controles. Las copias `offsite-` salen en la lista con la etiqueta S3 que ya existe.
3. **Tareas › paso Copia de seguridad (`task-dialog.tsx`).** Junto a «Conservar» aparecen:
   - el desplegable «Subir al S3» (Nunca, Siempre, Cada fin de semana, Cada semana, Cada 15 días, Cada mes);
   - «Conservar en S3», visible solo si la frecuencia no es «Nunca».

   Una línea de ayuda explica que el resto de ejecuciones se quedan en el servidor.

Textos en `es.json` y `en.json`.

### Documentación

- `docs/es` y `docs/en`: `features/backups.md` (secciones del S3 y de tareas), `features/panel-settings.md` y `backend/02-configuration.md` (las claves nuevas).
- Swagger de los endpoints nuevos.

## Decisiones mías (el usuario puede cambiarlas)

1. La copia periódica, tras subirse, **no se queda en el servidor** (solo en el S3).
2. Las tareas guardadas antes del cambio, sin `offsite`, valen **«Siempre»**, para que nadie pierda subidas que ya tenía. Las tareas nuevas traen **«Cada semana»** por defecto.
3. La copia periódica viene **desactivada** por defecto (`periodicHours = 0`) hasta que el admin elija un intervalo.
4. Un único trabajador por nodo que va servidor por servidor cada 10 minutos, para no hacer todas las copias a la vez.
5. «Cada 15 días» es una ventana móvil de 15 días con 2 h de margen; fin de semana, semana y mes son periodos del calendario (en la zona horaria del scheduler).
6. Las copias manuales dejan de subirse al S3, como se pidió. Las que ya están en el bucket se quedan.
7. Borrar una tarea **no** borra sus copias del bucket: son copias de seguridad y siguen en la lista del servidor (etiqueta S3), donde se pueden borrar a mano. Borrar un servidor tampoco toca el bucket, como hoy.
8. Si el bucket no responde, la espera entre intentos crece por servidor (1 h, 2 h, 4 h… hasta 24 h) y antes de escribir cada copia se comprueba el bucket.

## Lo que queda al constructor

- Dónde vive exactamente el trabajador periódico (`internal/servers`, arrancado donde se cargan los servidores) y cómo se detiene al apagar el nodo.
- Cómo pasa el scheduler el `taskId` a la operación `backup`.
- La forma exacta de los mensajes de consola y de los errores.
- Pruebas con un bucket falso (interfaz pequeña sobre `offsite`, o MinIO en las pruebas existentes si ya lo usan).
