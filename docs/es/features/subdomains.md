# Subdominios automáticos (Cloudflare)

Con una zona de Cloudflare conectada, cada servidor puede tener su propia dirección, como `survival.play.example.com`, en lugar de una IP y un puerto. El panel crea, actualiza y elimina los registros DNS por sí mismo a través de la API de Cloudflare. Esta página cubre la configuración para administradores y el uso diario para propietarios de servidores; el funcionamiento interno de cómo los registros siguen al servidor se resume en [Servidores — Subdominio (Cloudflare)](../backend/04-servers.md#subdominio-cloudflare).

## Qué hace

- Permite al propietario de un servidor elegir un nombre (`<nombre>.<dominio base>`) desde la configuración del servidor, con comprobación de disponibilidad en tiempo real.
- Opcionalmente asigna un subdominio de forma automática a **cada servidor nuevo**, derivado de su nombre.
- Crea los registros DNS en la zona de Cloudflare del administrador:
  - Un registro `A`, `AAAA` o `CNAME` con el nombre del subdominio, que apunta al nodo del servidor.
  - En los servidores de Minecraft Java, un registro `SRV` (`_minecraft._tcp.<nombre>.<dominio base>`) con el puerto principal, para que los jugadores puedan conectarse **sin escribir el puerto**.
- Mantiene los registros al día cuando cambian el puerto principal, el tipo o el nodo del servidor, la dirección del nodo o el Destino DNS, y los elimina cuando se quita el subdominio o se borra el servidor.
- Si el administrador cambia el dominio base o la zona, mueve los subdominios existentes al dominio nuevo.
- Muestra la dirección en la cabecera del servidor y en su tarjeta de la lista de servidores en lugar de `IP:puerto`, con un botón para copiarla en la cabecera.

## Cómo funciona

### Registros

Todos los registros se crean como **solo DNS** (nube gris, `proxied: false`), con TTL automático y el comentario `Aether Panel: server <serverId>`. Nunca pasan por el proxy, porque el tráfico de los juegos no es HTTP.

El registro `SRV` se añade cuando el tipo del servidor es `minecraft` o `minecraft-java` y el servidor tiene puerto principal. Sus datos son prioridad `0`, peso `5`, el puerto principal y, como destino, el propio subdominio; si el subdominio es un `CNAME`, el destino es el nombre del nodo, porque un registro SRV no debe apuntar a un alias (RFC 2782). Los demás juegos (incluido Bedrock) no leen registros SRV, así que su dirección sigue necesitando el puerto: el panel muestra `nombre.dominio:puerto` en esos casos.

### Hacia dónde apunta el nombre

El destino se elige en este orden:

1. El host público del nodo, si es una dirección IP pública (`A` para IPv4, `AAAA` para IPv6).
2. En el nodo local, `panel.settings.nodeIp` si es una IP pública.
3. El host público del nodo como `CNAME`, si es un nombre de host utilizable (no `localhost`, no una IP y que no termine en `.local`, `.internal` ni `.localhost`).
4. El ajuste **Destino DNS** (`panel.dns.target`), si está definido: una IP genera un registro `A`/`AAAA` y un nombre de host, un `CNAME`. Solo se usa para los nodos que no tienen ninguna de las direcciones anteriores.

Así cada servidor sigue a su propio nodo, y el nombre del nodo puede ser de otro dominio que los subdominios. Por ejemplo, con el nodo registrado como `usa.example.com` y el dominio base `example.es`, el servidor `pablo` queda en `pablo.example.es` → `CNAME usa.example.com`. El nombre del nodo debe ser **solo DNS** (nube gris) allí donde esté alojado; el panel no lo crea.

Si no se cumple ninguno, el cambio se rechaza con `ErrSubdomainNoTarget`. Nunca se usan como destino direcciones privadas, de bucle local, de enlace local ni sin especificar.

### Asignar y cambiar un nombre

Al guardar un nombre, el panel:

1. Lo valida (véase [Límites](#límites)) y comprueba que ningún otro servidor lo use.
2. Lista los registros de la zona con esos mismos nombres. Si existe alguno que no pertenece a este servidor, el nombre se considera **en uso**: los registros creados fuera del panel nunca se sobrescriben.
3. Crea los registros nuevos, guarda el nombre y los ID de los registros en el servidor y solo entonces elimina los registros antiguos. Si algo falla antes de guardar, los registros recién creados se eliminan y el servidor conserva su dirección anterior.

Los cambios se procesan de uno en uno en todo el panel, de modo que dos servidores no pueden reclamar a la vez el mismo nombre libre.

### Asignación automática

Con **Asignar un subdominio a cada servidor nuevo** activado, un servidor creado desde el panel o mediante la [API de aprovisionamiento](../reference/11-api-reference.md) recibe un subdominio en segundo plano, poco después de crearse. El nombre es el del servidor en minúsculas, sin tildes y con cualquier otro carácter convertido en guion, recortado a 32 caracteres (`Mi Survival` → `mi-survival`). Si queda con menos de 3 caracteres se usa `srv-<serverId>`. Si el nombre está en uso, reservado o ya existe en la zona, se añade `-2`, `-3`… (hasta 20 intentos). Los fallos solo se escriben en el registro del panel.

### Sincronización de los registros

Los registros se actualizan en segundo plano (alrededor de medio segundo después de guardar el cambio) cuando:

- cambia el puerto principal o el tipo del servidor (`PUT /api/servers/:serverId/definition`),
- un administrador cambia el puerto principal (`PUT /api/servers/:serverId/data`),
- se cambia el puerto principal en los ajustes de puertos (`PUT /api/servers/:serverId/port-settings`),
- el servidor se transfiere a otro nodo,
- un administrador cambia el host público del nodo (`PUT /api/nodes/:id`), o la IP del nodo maestro o la URL base en el caso del nodo local (solo los servidores de ese nodo),
- un administrador cambia el **Destino DNS** (todos los servidores con subdominio).

El nombre no cambia nunca; un registro `A` puede convertirse en `CNAME` (o al revés) y el registro `SRV` se crea o se elimina según haga falta. Es de mejor esfuerzo: si Cloudflare falla, el error se registra y el cambio del servidor no se deshace.

Al borrar un servidor (desde el panel o desde la API de aprovisionamiento) se eliminan sus registros. Si en ese momento Cloudflare ya no está configurado, los registros permanecen en la zona y se escribe un mensaje en el registro.

### Cambiar el dominio base o la zona

Cada servidor guarda el dominio base y la zona en los que se crearon sus registros (`dns_domain`, `dns_zone_id`). Al guardar un dominio base o un ID de zona distintos, el panel mueve en segundo plano, de uno en uno, todos los subdominios: crea el mismo nombre bajo el dominio nuevo, lo guarda y elimina los registros antiguos de su zona original (con el mismo token, que necesita permiso sobre ambas zonas). Hasta que un servidor se mueve, el panel sigue mostrando su dirección antigua, que funciona.

Si el nombre ya existe en la zona nueva fuera del panel, ese servidor no se mueve y conserva su dirección antigua; el motivo queda en el registro del panel. Los subdominios creados antes de esta versión no tienen guardado su dominio: el panel lo anota con los ajustes vigentes justo antes de guardarlos.

### Dominio del panel detrás del proxy de Cloudflare

Esta función es independiente del dominio del propio panel (la URL base). Si el dominio del panel pasa por el proxy de Cloudflare (nube naranja), no lleva a la IP real del servidor y los puertos de juego no pueden usarlo. El servicio de dominio del instalador lo detecta y guarda la IP real del servidor, que el panel usa entonces como dirección del nodo local. Si no es el caso en una instalación concreta, hay que poner la IP pública del servidor en **IP global del nodo maestro** (o en **Destino DNS**).

## Uso

### Desde el panel

**Administrador: conectar Cloudflare** (requiere `settings.edit`)

1. En Cloudflare, ir a **Mi perfil → Tokens de API → Crear token** y usar la plantilla **Editar DNS de zona** (permiso *Zona → DNS → Editar*). En *Recursos de zona*, limitarlo a la zona del dominio. Copiar el token: solo se muestra una vez.
2. Copiar el **ID de zona** desde la página *Información general* del dominio (columna derecha, apartado *API*). Tiene 32 caracteres hexadecimales.
3. En el panel, abrir **Configuración → Dominios** y rellenar:
   - **Dominio base**: la zona o un subdominio de ella, por ejemplo `play.example.com` (sin `http://` ni barras).
   - **Destino DNS (opcional)**: solo hace falta para nodos sin dirección pública (una IP privada o un nombre interno).
   - **Token de API** e **ID de zona**.
4. Pulsar **Probar conexión**. La prueba usa los valores en pantalla, estén guardados o no: lee la zona, comprueba que el dominio base pertenece a ella y crea y borra un registro `TXT` temporal (`_aether-panel-test.<dominio>`). Si todo va bien, muestra el nombre de la zona.
5. Opcionalmente, activar **Asignar un subdominio a cada servidor nuevo** y guardar.

Los subdominios quedan activos cuando están definidos el token, el ID de zona y el dominio base.

**Propietario del servidor: elegir una dirección** (requiere `server.flags.edit` o `server.data.edit` sobre el servidor)

1. Abrir la pestaña **Configuración** del servidor y buscar la tarjeta **Dirección del servidor**. Está oculta si el administrador no ha configurado Cloudflare.
2. Escribir un nombre. El estado se actualiza mientras se escribe: **Disponible**, **Es la dirección actual**, **Ya lo usa otro servidor**, **Ese nombre está reservado** o el error de formato.
3. Pulsar **Guardar**. **Ver registros DNS** muestra los registros que gestiona el panel.
4. Para volver a `IP:puerto`, pulsar **Quitar** y confirmar (**¿Quitar? Pulsa de nuevo**).

Los cambios de DNS tardan alrededor de un minuto en llegar a los jugadores. Los usuarios sin el permiso indicado ven la dirección en la tarjeta de la lista de servidores, pero siguen viendo `IP:puerto` en la cabecera del servidor.

**Nodos con nombre propio.** Si el host público de un nodo es un nombre (`usa.example.com`), la lista de servidores muestra ese nombre en lugar de la IP en las tarjetas de sus servidores sin subdominio. Para el nodo local, el nombre se pone en **Configuración → General → IP global del nodo maestro**.

### Mediante la API

| Método | Ruta | Permiso | Uso |
|--------|------|---------|-----|
| `GET` | `/api/servers/:serverId/subdomain` | `server.flags.edit` o `server.data.edit` | Si los subdominios están activos, el dominio base, el subdominio del servidor, la dirección completa, `srv`, el puerto y los registros previstos. |
| `GET` | `/api/servers/:serverId/subdomain/check?name=<nombre>` | `server.flags.edit` o `server.data.edit` | `{"available": bool, "reason": "" \| "current" \| "invalid" \| "reserved" \| "taken"}`. No consulta a Cloudflare. |
| `PUT` | `/api/servers/:serverId/subdomain` | `server.flags.edit` o `server.data.edit` | Cuerpo `{"subdomain": "survival"}`. Crea los registros y devuelve la misma vista que `GET`. |
| `DELETE` | `/api/servers/:serverId/subdomain` | `server.flags.edit` o `server.data.edit` | Elimina los registros. Responde `204`. |
| `POST` | `/api/settings/test/cloudflare` | `settings.edit` | Prueba la conexión. El cuerpo opcional sustituye a los valores guardados de `panel.dns.cloudflare.token`, `panel.dns.cloudflare.zoneId` y `panel.dns.baseDomain` (un token `********`, el valor que la página recibe en lugar del guardado, usa el guardado). Devuelve `{"zone": "<nombre de la zona>"}`. |

Las claves de Cloudflare se guardan con los endpoints de configuración del panel (`POST /api/settings`). La información de cada servidor (`GET /api/servers`, `GET /api/servers/:serverId`) incluye `subdomain` (la dirección completa, si tiene registros) y `subdomainSrv` (`true` cuando la dirección no necesita el puerto). Los detalles completos están en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

- Un dominio cuyo DNS esté alojado en Cloudflare y un token de API con *Zona → DNS → Editar* sobre esa zona.
- El panel necesita salida HTTPS hacia `api.cloudflare.com`.
- Los nodos deben ser accesibles desde internet en la dirección a la que apuntan los registros.

| Clave | Valor por defecto | Variable de entorno | Descripción |
|-------|-------------------|---------------------|-------------|
| `panel.dns.cloudflare.token` | `""` | `SKYPANEL_PANEL_DNS_CLOUDFLARE_TOKEN` | Token de API. Solo se envía en la cabecera `Bearer` y se sustituye por `[token]` en cualquier mensaje de error. |
| `panel.dns.cloudflare.zoneId` | `""` | `SKYPANEL_PANEL_DNS_CLOUDFLARE_ZONEID` | ID de zona, 32 caracteres hexadecimales. |
| `panel.dns.baseDomain` | `""` | `SKYPANEL_PANEL_DNS_BASEDOMAIN` | Dominio bajo el que se crean los nombres. |
| `panel.dns.target` | `""` | `SKYPANEL_PANEL_DNS_TARGET` | IP o nombre de host al que apuntar cuando un nodo no tiene ni IP pública ni un nombre utilizable. Vacío: la dirección pública de cada nodo. |
| `panel.dns.autoAssign` | `false` | `SKYPANEL_PANEL_DNS_AUTOASSIGN` | Asignar automáticamente un subdominio a los servidores nuevos. |

Las cinco se pueden editar desde **Configuración → Dominios**. El token se guarda en el archivo de configuración del panel y se devuelve a los administradores con `settings.edit`. Véase [Configuración](../backend/02-configuration.md).

## Límites

- **Formato del nombre**: entre 3 y 32 caracteres, letras minúsculas, números y guiones, sin guion al principio ni al final. La entrada se pasa a minúsculas y se recortan los espacios.
- **Nombres reservados**: `www`, `mail`, `panel`, `api`, `admin`, `ftp`, `ns1`–`ns4`, `smtp`, `imap`, `pop`, `pop3`, `webmail`, `mx`, `dns`, `cdn`, `status`, `dashboard`, `billing`, `support`, `help`, `docs`, `autodiscover`, `autoconfig`, `localhost`, `root`, `node`, `sftp`, `ssh`, `vpn`, `cpanel`, `whm`, `app`, `auth`, `login`, `static`, `assets`, `dev`, `test`.
- Un subdominio por servidor, y cada nombre solo puede usarlo un servidor.
- Un servidor puede cambiar o quitar su subdominio como máximo una vez cada **10 segundos**.
- Cada llamada a Cloudflare caduca a los 15 segundos; un cambio completo está limitado a 45 segundos.
- Solo una zona y un dominio base activos por panel; al cambiarlos, los subdominios se mueven (véase [Cambiar el dominio base o la zona](#cambiar-el-dominio-base-o-la-zona)).
- El registro SRV solo se crea para Minecraft Java (`minecraft`, `minecraft-java`).

## Problemas conocidos y soluciones

Los errores del panel llevan un código estable (entre paréntesis) y un mensaje en inglés; el panel muestra cada uno traducido al idioma de quien lo mira, con los valores que lleva el error (puerto, dominio, mensaje de Cloudflare) ya rellenados.

| Mensaje | Causa y solución |
|---------|------------------|
| `Los subdominios automáticos no están configurados en este panel.` (`ErrSubdomainDisabled`, 400) | Falta el token, el ID de zona o el dominio base. Completar **Configuración → Dominios**. |
| `El subdominio solo puede tener letras minúsculas, números y guiones…` (`ErrSubdomainInvalid`, 400) | Nombre demasiado corto o largo, o con caracteres no válidos. |
| `Ese subdominio está reservado.` (`ErrSubdomainReserved`, 400) | El nombre está en la lista de reservados. Elegir otro. |
| `Ese subdominio ya está en uso.` (`ErrSubdomainTaken`, 409) | Lo usa otro servidor **o** ya existe en la zona, fuera del panel, un registro con ese nombre (o con su nombre SRV `_minecraft._tcp.`). Borrar el registro ajeno en Cloudflare o elegir otro nombre. |
| `Espera unos segundos antes de volver a cambiar el subdominio.` (`ErrSubdomainRateLimited`, 429) | Dos cambios en menos de 10 segundos. Esperar y reintentar. |
| `El nodo de este servidor no tiene una dirección pública; configura un destino DNS…` (`ErrSubdomainNoTarget`, 422) | El nodo está registrado con una IP privada, `localhost` o un nombre interno. Registrar el nodo con su IP o su nombre público, o poner en **Destino DNS** la IP o el nombre público. |
| `Faltan el token de la API o el ID de zona de Cloudflare.` (`ErrCloudflareNotConfigured`, 400, prueba) | El token o el ID de zona están vacíos. |
| `El ID de zona de Cloudflare debe tener 32 caracteres hexadecimales.` (`ErrCloudflareZoneID`) | ID de zona incorrecto (a menudo se pega el ID de la cuenta). Copiar el *ID de zona* de la página de información general del dominio. |
| `El dominio base no es un nombre de dominio válido.` (`ErrDNSBaseDomainInvalid`, 400, prueba) | El dominio base contiene `http://`, barras o caracteres no válidos. |
| `El dominio <dominio> no pertenece a la zona <zona>.` (`ErrCloudflareDomainNotInZone`, 422, prueba) | El dominio base no es la zona ni un subdominio de ella. Usar un dominio dentro de la zona elegida. |
| `Cloudflare respondió: … (código <n>)` (`ErrCloudflareRefusedCode`, o `ErrCloudflareRefused` si no hay código; 422) | Cloudflare rechazó la llamada. El panel muestra el mensaje y el código de Cloudflare tal como llegan; las causas habituales son un token no válido o un token sin permiso de edición de DNS en esa zona. Volver a crear el token con la plantilla *Editar DNS de zona* limitada a la zona correcta. |
| `No se pudo contactar con Cloudflare: …` (`ErrCloudflareUnreachable`) | El panel no llega a `api.cloudflare.com` (cortafuegos, DNS o sin salida a internet). |
| La dirección resuelve pero los jugadores no pueden conectarse | El registro apunta a un nombre con proxy o a una IP incorrecta. Abrir **Ver registros DNS** y revisar el destino; si es el nombre del nodo, ponerlo en **solo DNS** (nube gris) o registrar el nodo con su IP pública. Comprobar también que el puerto del juego está abierto en el nodo. |
| Tras cambiar el dominio base, algún servidor sigue con la dirección antigua | El nombre ya existía en la zona nueva fuera del panel, o el token no tiene permiso sobre la zona antigua o la nueva. El motivo está en el registro del panel. Borrar el registro ajeno o elegir otro nombre desde la tarjeta **Dirección del servidor**. |
| Quedan registros en la zona tras borrar un servidor | Cloudflare no estaba configurado o la llamada falló al borrar el servidor. Eliminar los registros a mano; llevan el comentario `Aether Panel: server <serverId>`. |

## Véase también

- [Servidores — Subdominio (Cloudflare)](../backend/04-servers.md#subdominio-cloudflare)
- [Instalación](../setup/01-installation.md)
- [Configuración](../backend/02-configuration.md)
- [Referencia de permisos](../general/scopes-reference.md)
- [Referencia de la API](../reference/11-api-reference.md)
