# Sistema de licencias

Aether Panel puede verificar una clave de licencia contra una API de licencias externa y guardar el plan resultante en su configuración.

> **Estado actual:** la activación funciona (verificar, vincular, guardar), pero **ninguna parte del panel lee todavía la licencia guardada**. Ninguna función se activa ni se bloquea según el plan, no hay pantalla de licencia en el frontend y no existe revalidación periódica, caché ni gestión de caducidad. El resto de la página describe exactamente lo que hace hoy el código.

---

## API externa

`internal/services/license.go` usa una URL base fija (constante `LicenseAPIBaseURL`, no configurable):

```
https://prueba.skyhostingcloud.com/api/public/licenses
```

Las peticiones usan un `http.Client` normal con 30 segundos de límite y `Content-Type`/`Accept: application/json`.

| Llamada | Petición | Estado aceptado |
|---------|----------|-----------------|
| Verificar | `GET {base}/verify?licenseKey=<clave>` | `200` |
| Vincular | `POST {base}/verify` con `{"licenseKey","serverId","serverIp"}` | `200` o `201` |

Cualquier otro estado se devuelve como error con el código y el cuerpo de la respuesta. Al vincular, un `200`/`201` cuyo cuerpo no es JSON se da por correcto. La clave se codifica para URL en la query, así que una clave con `&`, `#` o espacios se envía completa.

### Respuesta de verificación

Campos que el panel decodifica (`LicenseVerifyResponse`):

```json
{
  "valid": true,
  "status": "active",
  "isInGracePeriod": false,
  "license": {
    "key": "ABCD-EFGH-IJKL-MNOP",
    "plan": "professional",
    "maxServers": 10,
    "usedServers": 2,
    "expiryDate": "2027-01-01T00:00:00Z",
    "daysRemaining": 61,
    "billingCycle": "monthly",
    "boundServerId": "panel-host",
    "boundServerIp": "203.0.113.10"
  },
  "user": { "email": "…", "name": "…" },
  "payment": { "hasCompletedPayment": true, "lastPaymentDate": "…" },
  "validation": { "timestamp": "…", "ip": "…" }
}
```

Solo se usan `valid`, `license.plan` y los campos de `license` que se devuelven en la respuesta de activación.

---

## Claves de configuración

Definidas en `internal/config/entries.go`. Las cuatro se pueden editar desde la API de ajustes (`settings.edit`), ver [Configuración](../backend/02-configuration.md).

| Clave | Variable de entorno | Por defecto | Descripción |
|-------|---------------------|-------------|-------------|
| `panel.license.key` | `SKYPANEL_PANEL_LICENSE_KEY` | `""` | Última clave activada |
| `panel.license.status` | `SKYPANEL_PANEL_LICENSE_STATUS` | `free` | Tipo derivado del plan: `free`, `pro` o `enterprise` |
| `panel.license.serverId` | `SKYPANEL_PANEL_LICENSE_SERVERID` | `""` | Identificador que se envía al vincular. Se genera en la primera activación |
| `panel.license.serverIp` | `SKYPANEL_PANEL_LICENSE_SERVERIP` | `""` | IP que se envía al vincular. Se genera en la primera activación |

---

## Activación: `POST /api/settings/license/activate`

Permiso: `settings.edit`.

Petición:

```json
{ "key": "ABCD-EFGH-IJKL-MNOP" }
```

Pasos (`activateLicense` en `internal/web/api/settings.go`):

1. `key` es obligatoria. Sin guiones y en mayúsculas debe tener exactamente **16 caracteres**; si no, `400 invalid license key format`.
2. **Verificar** la clave. Un error en la petición o `valid: false` devuelve `400` con `{"success": false, "message": "…"}`.
3. Completar los datos de vinculación si aún no están guardados, y guardarlos:
   - `serverId`: el nombre del host (`unknown` si no se puede leer).
   - `serverIp`: la primera IPv4 no loopback a la que resuelve el nombre del host; si no hay, la primera IPv4 no loopback de las interfaces de red; si tampoco, `127.0.0.1`.
4. **Vincular** la clave a ese `serverId`/`serverIp`. Si la vinculación falla, solo se registra en el log; la activación continúa.
5. Guardar `panel.license.key` y `panel.license.status`.

Respuesta `200`:

```json
{
  "success": true,
  "type": "pro",
  "license": {
    "key": "ABCD-EFGH-IJKL-MNOP",
    "plan": "professional",
    "maxServers": 10,
    "usedServers": 2,
    "expiryDate": "2027-01-01T00:00:00Z",
    "daysRemaining": 61,
    "billingCycle": "monthly"
  },
  "permissions": { "hasPlugins": true },
  "bound": true,
  "message": "License activated successfully"
}
```

`bound` es `false` si la llamada de vinculación falló o respondió `success: false`.

```bash
curl -X POST https://panel.example.com/api/settings/license/activate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key": "ABCD-EFGH-IJKL-MNOP"}'
```

No existe un endpoint para consultar el estado de la licencia aparte de la API genérica de ajustes (`GET /api/settings/panel.license.status`).

---

## Correspondencia de planes

`GetLicenseType` convierte la verificación en el tipo guardado:

| Verificación | `panel.license.status` guardado |
|--------------|---------------------------------|
| `valid: false` | `free` (no se guarda: la activación se detiene antes) |
| plan `personal` | `free` |
| plan `professional` | `pro` |
| plan `enterprise` | `enterprise` |
| cualquier otro plan | `pro` |

`ExtractPermissions` devuelve `hasPlugins: true` para `professional` y `enterprise`. Este valor solo aparece en la respuesta de activación; ningún endpoint ni página lo comprueba.

---

## Notas de seguridad

- La clave se guarda en texto plano en la configuración del panel. `GET /api/settings` la devuelve enmascarada (`********`), igual que los demás secretos (véase [Configuración del panel](../features/panel-settings.md#cómo-funciona)).
- Los mensajes de error del servidor de licencias (estado y cuerpo) se registran en el log y se devuelven en la respuesta de activación.
- La API de licencias se llama por HTTPS; el cliente no es el cliente restringido (protegido contra SSRF) porque la URL es una constante.

---

## Limitaciones conocidas

- El servidor de licencias es un host de pruebas fijo en el código (`prueba.skyhostingcloud.com`, véase [API externa](#api-externa)); no se puede cambiar sin recompilar.
- La licencia guardada todavía no activa ni bloquea nada (véase el aviso del principio).
