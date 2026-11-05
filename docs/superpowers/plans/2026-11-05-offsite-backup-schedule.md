# Copias en S3 periódicas y por tarea — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** El S3 deja de recibir todas las copias. Recibe una copia periódica de cada servidor (cada X horas, sin tareas) y las ejecuciones de tareas a las que les toque según su frecuencia, con retención propia.

**Architecture:**
- **Nodo (`internal/servers`):**
  - una capa fina sobre el bucket que en las pruebas se cambia por uno falso;
  - funciones puras que deciden cuándo subir y qué podar;
  - la operación `backup` de tareas, ampliada;
  - un trabajador periódico, uno por nodo, que hace la copia `offsite-` y la sube.
- **Ajustes por servidor:** en `<id>.offsite`, con endpoints del daemon a los que el panel reenvía sin más.
- **Interfaz:** React en tres pantallas.

**Tech Stack:** Go 1.26 (gin, gocron, minio-go, testify), React/Astro + TypeScript, i18n JSON.

**Spec:** `docs/superpowers/specs/2026-11-05-offsite-backup-schedule-design.md` (léelo antes de empezar; manda sobre este plan).

## Global Constraints

- Prefijos de archivo:
  - copia de tarea: `auto-<taskId>-<uuid>.tar.gz`;
  - copia periódica: `offsite-<uuid>.tar.gz` (`OffsiteBackupPrefix = "offsite-"`);
  - manual: `<uuid>.tar.gz`;
  - punto de restauración: `restore-…`.
- Frecuencias de tarea: `never` | `always` | `weekend` | `weekly` | `biweekly` | `monthly`. Si falta, vale `always`. Una tarea nueva creada en el panel trae `weekly`.
- `offsiteKeep` y `periodicKeep`: 1-50, por defecto 4.
- Horas de la copia periódica (`periodicHours`): 0 (desactivada), 12, 24, 48, 72, 168, 336, 720. Por defecto 0. En el servidor, `null` significa «por defecto».
- Claves de configuración: `daemon.backups.s3.periodicHours` y `daemon.backups.s3.periodicKeep`.
- `biweekly`: la última copia tiene ≥ 15 días − 2 h. `weekend`: sábado o domingo y nada desde el sábado 00:00. `weekly`: nada desde el lunes 00:00. `monthly`: nada desde el día 1 00:00. Todo en la zona horaria del scheduler del servidor.
- Trabajador periódico:
  - recorre los servidores cada 10 minutos, uno a uno;
  - el primer recorrido empieza 5 minutos después de arrancar el nodo;
  - tras un fallo, la espera es de 1 h, 2 h, 4 h… hasta 24 h como máximo.
- La copia periódica:
  - se hace aunque el juego esté encendido;
  - no se hace mientras el servidor se instala (`IsInstalling`), está pausado (`Scheduler.IsPaused`) o tiene otra copia o restauración en curso;
  - avisa solo si falla;
  - tras subirse, se borra del disco del nodo (también si la subida falla).
- Las copias manuales ya no suben al S3. La poda local no toca el bucket. `DeleteBackup`, cuando lo pide un usuario, sigue borrando las dos copias.
- Borrar una tarea o un servidor no toca el bucket.
- El `taskId` lo pone siempre el scheduler, nunca se lee de los argumentos guardados de la tarea.
- Mensajes de consola del nodo en inglés, como los de hoy. Textos de la interfaz en `es.json` y `en.json`. La interfaz debe ser responsive en móvil, tablet y escritorio.
- Verificar con `/private/tmp/claude-501/-Users-rubenmaso-Developer-paneltambres/2c190f69-223e-4c8c-a860-02747c59afbe/scratchpad/verify.sh go|fe` (VPS; no hay Go local).

## Review Focus

1. **Id de tarea con `-`:** las tareas `a` y `a-b` no deben ver nunca las copias de la otra en el bucket (Task 1 lo prueba en `taskIDFromBackupName`; Task 3 lo prueba en la poda).
2. **Dos ejecuciones casi seguidas:** si la primera aún está subiendo, la segunda no sube otra (Task 3: `TestTaskBackup_SkipsWhileTheSameSeriesUploads`).
3. **Bucket caído:** el trabajador no escribe ninguna copia y aplica la espera creciente (Task 5: `TestPeriodic_UnreachableBucketWritesNothingAndBacksOff`).
4. **Tarea guardada antes del cambio** (sin `offsite`): sigue subiendo en cada ejecución (Task 3: `TestTaskBackup_OldTaskStillUploadsEveryRun`).
5. **Servidor trasladado de nodo:** su ajuste `.offsite` llega al nodo nuevo (Task 4: `TestServerMove_CopiesOffsiteSettings`).

---

### Task 1: Reglas puras (frecuencia, nombres, poda, valores)

**Files:**
- Create: `internal/servers/offsite_policy.go`
- Test: `internal/servers/offsite_policy_test.go`

**Interfaces:**
- Produces:
  - `type OffsiteFrequency string` y sus constantes `OffsiteNever`, `OffsiteAlways`, `OffsiteWeekend`, `OffsiteWeekly`, `OffsiteBiweekly`, `OffsiteMonthly`.
  - `ParseOffsiteFrequency(v interface{}) (OffsiteFrequency, error)`: `nil` o `""` dan `OffsiteAlways`; un valor desconocido da error.
  - `OffsiteKeep(v interface{}) (int, error)`: `nil` da 4; fuera de 1-50 da error.
  - `ValidPeriodicHours(h int) bool`.
  - `offsiteDue(f OffsiteFrequency, last time.Time, now time.Time) bool`: `last` cero significa que no hay copia; las fechas del calendario salen de `now.Location()`.
  - `taskBackupPrefix(taskID string) string`: devuelve `"auto-" + taskID + "-"`.
  - `taskIDFromBackupName(name string) (string, bool)`.
  - `newestBeyond(objects []offsite.Object, keep int) []string`: los nombres a borrar, empezando por el más antiguo.
  - `const OffsiteBackupPrefix = "offsite-"`.

- [ ] **Step 1: Escribe las pruebas (tablas)**

```go
func TestOffsiteDue(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Madrid")
	sat := time.Date(2026, 11, 7, 3, 0, 0, 0, loc) // sábado
	cases := []struct{ f OffsiteFrequency; last, now time.Time; want bool }{
		{OffsiteNever, time.Time{}, sat, false},
		{OffsiteAlways, sat.Add(-time.Minute), sat, true},
		{OffsiteWeekend, time.Time{}, sat.AddDate(0, 0, -1), false},                 // viernes
		{OffsiteWeekend, time.Time{}, sat, true},
		{OffsiteWeekend, sat, sat.AddDate(0, 0, 1), false},                          // domingo, ya subió el sábado
		{OffsiteWeekend, sat.AddDate(0, 0, -7), sat, true},
		{OffsiteWeekly, time.Date(2026, 11, 2, 1, 0, 0, 0, loc), sat, false},        // lunes de esta semana
		{OffsiteWeekly, time.Date(2026, 11, 1, 23, 0, 0, 0, loc), sat, true},        // domingo anterior
		{OffsiteBiweekly, sat.Add(-15*24*time.Hour + time.Hour), sat, true},         // dentro del margen de 2 h
		{OffsiteBiweekly, sat.Add(-14 * 24 * time.Hour), sat, false},
		{OffsiteMonthly, time.Date(2026, 11, 1, 0, 30, 0, 0, loc), sat, false},
		{OffsiteMonthly, time.Date(2026, 10, 31, 23, 0, 0, 0, loc), sat, true},
	}
	for i, c := range cases {
		assert.Equal(t, c.want, offsiteDue(c.f, c.last, c.now), "case %d (%s)", i, c.f)
	}
}

func TestTaskIDFromBackupName(t *testing.T) {
	u := "0b7c7a52-3d0e-4f5b-9a55-8f7f1f2d3c4e"
	id, ok := taskIDFromBackupName("auto-a-b-" + u + ".tar.gz")
	assert.True(t, ok); assert.Equal(t, "a-b", id)
	id, ok = taskIDFromBackupName("auto-a-" + u + ".tar.gz")
	assert.True(t, ok); assert.Equal(t, "a", id)
	_, ok = taskIDFromBackupName("auto-" + u + ".tar.gz") // copia antigua sin tarea
	assert.False(t, ok)
	_, ok = taskIDFromBackupName("offsite-" + u + ".tar.gz")
	assert.False(t, ok)
}
```

Añade también, de forma breve:
- `ParseOffsiteFrequency`: `nil` da `always` y `"daily"` da error.
- `OffsiteKeep`: `nil` da 4; 0 y 51 dan error.
- `ValidPeriodicHours`: 24 es válido y 25 no.
- `newestBeyond`: con 6 objetos y `keep` 4, devuelve los 2 más antiguos.

- [ ] **Step 2: Ejecuta las pruebas y comprueba que fallan**

Comando: `verify.sh go`. Debe salir FAIL de compilación en `internal/servers` (los símbolos aún no existen).

- [ ] **Step 3: Implementa `offsite_policy.go`.** Las reglas del calendario:
  - `weekend`: se calcula el sábado 00:00 de este fin de semana con `now.Weekday()` en `now.Location()`.
  - `weekly`: lunes 00:00 (con `Weekday` 0 = domingo, se retroceden 6 días).
  - `monthly`: día 1 00:00.
  - `taskIDFromBackupName`: exige el prefijo `auto-` y el sufijo `.tar.gz`, quita los 37 caracteres finales (`-` y el uuid), valida el uuid con `uuid.FromString` y exige que el id restante no esté vacío.

- [ ] **Step 4: Ejecuta las pruebas y comprueba que pasan.** `verify.sh go`: las pruebas nuevas en PASS y gofmt/vet/lint limpios.

- [ ] **Step 5: Commit** `feat(backups): rules for when a backup goes off-site and which copies to prune`

---

### Task 2: Capa del bucket; las copias manuales y la poda local no tocan el S3

**Files:**
- Create: `internal/servers/offsite_store.go`
- Modify: `internal/servers/server.go`:
  - `finishBackup` (~1713);
  - `uploadOffsite` (~1781);
  - `mergeOffsite` (~1844);
  - `pruneAutoBackups` (~1872);
  - `DeleteBackup` (~1895);
  - `locateBackup` (~1928).
- Test: `internal/servers/offsite_store_test.go` (bucket falso) y `internal/servers/backup_test.go`

**Interfaces:**
- Produces:
  - `type offsiteBucket interface { Configured() bool; Reachable(ctx context.Context) error; Upload(ctx context.Context, serverID, name, path string) error; List(ctx context.Context, serverID string) ([]offsite.Object, error); Delete(ctx context.Context, serverID, name string) error }`.
  - `var bucket offsiteBucket = settingsBucket{}`: la versión real, que lee `offsite.FromSettings()` en cada llamada. `Reachable` usa `BucketExists`; añade `offsite.Reachable(ctx, cfg) error` en `internal/offsite`.
  - `useFakeBucket(t *testing.T) *fakeBucket`: helper de las pruebas que guarda los objetos en memoria con `ModTime`, cuenta las subidas y permite simular fallos con `failUpload` y `failReach`.
  - `(p *Server) deleteLocalBackup(fileName string) error`: borra el archivo y su marca `.failed-`, sin tocar el bucket.
  - **Series en curso:** `startSeries(serverID, series string) bool` y `endSeries(serverID, series string)`, un conjunto protegido por mutex. La serie es el `taskId` o `"periodic"`.
- Cambios de comportamiento:
  - `finishBackup` ya no llama a `uploadOffsite`.
  - `pruneAutoBackups` usa `deleteLocalBackup`.
  - `mergeOffsite`, `DeleteBackup` y `locateBackup` usan `bucket`.

- [ ] **Step 1: Escribe las pruebas que fallan**
  - `TestManualBackup_StaysLocal`: con el bucket falso configurado, `StartBackup()` y después `waitIdle`. Comprueba `fake.uploads == 0` y que el archivo local existe.
  - `TestPruneAutoBackups_LeavesTheBucketAlone`: siembra en local y en el bucket 3 copias `auto-t1-<uuid>`; ejecuta `pruneAutoBackups(1)`. Comprueba que en local queda 1 y en el bucket siguen 3.
  - `TestDeleteBackup_ByAUserRemovesBothCopies`: `DeleteBackup(name)` quita la copia local y el objeto del bucket.

- [ ] **Step 2: Ejecútalas.** `verify.sh go`: deben FALLAR.

- [ ] **Step 3: Implementa la capa y los cambios de `server.go`.** `uploadOffsite` desaparece de `finishBackup`; su lógica pasa a Task 3.

- [ ] **Step 4: Ejecútalas.** `verify.sh go`: PASS, y siguen pasando todas las de `backup_test.go`.

- [ ] **Step 5: Commit** `refactor(backups): a bucket seam; manual backups and local pruning stay off the bucket`

---

### Task 3: Tareas: nombre con el taskId, frecuencia de subida y retención en S3

**Files:**
- Modify:
  - `internal/servers/task_operations.go`: `backupOperation` y la fábrica `backup`.
  - `internal/servers/scheduler.go`: `runScheduledTask` y un nuevo `(s *Scheduler) Location() *time.Location`.
  - `internal/servers/server.go`: `RunAutoBackup`.
  - `internal/web/api/tasks.go`: `userTaskOperations["backup"].keys` y `validateUserTaskStep`.
- Test: `internal/servers/tasks_test.go`, `internal/web/api/tasks_test.go` (o el archivo de pruebas de tareas que exista)

**Interfaces:**
- Consumes (Tasks 1-2):
  - `ParseOffsiteFrequency`, `OffsiteKeep`, `offsiteDue`;
  - `taskIDFromBackupName`, `taskBackupPrefix`, `newestBeyond`;
  - `bucket`, `startSeries` y `endSeries`.
- Produces:
  - `(p *Server) RunAutoBackup(opts AutoBackupOptions) (string, error)`, con `type AutoBackupOptions struct { Keep int; TaskID string; Offsite OffsiteFrequency; OffsiteKeep int; Now func() time.Time }`. Si `Now` es `nil`, usa `time.Now`.
  - Al crear las operaciones, `runScheduledTask` copia el mapa `Metadata` de cada paso `backup` y le pone `taskId = id`, sobrescribiendo lo que trajera.
  - La fábrica lee `keep`, `offsite`, `offsiteKeep` y `taskId`. Si falta `taskId` (una tarea lanzada fuera del scheduler), el nombre de la copia es `auto-<uuid>` y nunca se sube.

**Flujo de `RunAutoBackup`, después de escribir la copia local y de la poda local de siempre:**
1. Sale sin subir si:
   - la frecuencia es `never`;
   - falta `TaskID`;
   - `bucket.Configured()` es falso;
   - `startSeries(id, TaskID)` es falso (ya hay una subida de esa tarea en curso).
2. `bucket.List`: el último `ModTime` de los objetos de esa tarea es aquel cuyo `taskIDFromBackupName` sea igual a `TaskID`.
   - Si el listado falla, escribe en la consola `Off-site copy skipped: <err>`, llama a `endSeries` y sale; la tarea no falla.
3. Si `offsiteDue(freq, last, Now().In(Scheduler.Location()))` es falso, llama a `endSeries` y sale.
4. En segundo plano (`go`):
   - `bucket.Upload`;
   - si sale bien, borra con `bucket.Delete` los objetos de esa tarea que devuelva `newestBeyond(..., OffsiteKeep)`;
   - al final, `endSeries` y los mismos mensajes de consola de hoy (`Uploading backup to off-site storage`, …).

- [ ] **Step 1: Escribe las pruebas que fallan**, con el bucket falso y `Now` fijo:
  - `TestTaskBackup_NamesCarryTheTask`: el nombre empieza por `auto-t1-` y `taskIDFromBackupName` devuelve `t1`.
  - `TestTaskBackup_WeeklyUploadsOncePerWeek`:
    - primera ejecución (martes): sube;
    - segunda (miércoles): no sube;
    - con `Now` del martes siguiente: sube.
  - `TestTaskBackup_OldTaskStillUploadsEveryRun`: sin `offsite` en los argumentos de la fábrica, dos ejecuciones dan dos subidas.
  - `TestTaskBackup_NeverStaysLocal`: con `offsite: never`, ninguna subida.
  - `TestTaskBackup_PrunesOnlyItsOwnSeries`:
    - el bucket tiene 5 copias de `a`, 5 de `a-b` y 1 `auto-<uuid>` antigua;
    - la tarea `a` (`always`, `offsiteKeep` 2) sube una;
    - quedan 2 de `a`, 5 de `a-b` y 1 antigua.
  - `TestTaskBackup_SkipsWhileTheSameSeriesUploads`: con el bucket falso bloqueado en `Upload` (un canal), una segunda ejecución inmediata no sube.
  - `TestScheduler_InjectsTaskID`: aunque los argumentos guardados traigan `taskId: "otra"`, la operación que se crea al ejecutar la tarea `t1` recibe `t1`, y el mapa guardado de la tarea no cambia.
  - En la API: `validateUserTaskStep` acepta `{"keep":5,"offsite":"weekly","offsiteKeep":4}` y rechaza `offsite:"daily"`, `offsiteKeep:0` y `taskId:"x"` (clave no permitida).

- [ ] **Step 2: Ejecútalas.** `verify.sh go`: deben FALLAR.

- [ ] **Step 3: Implementa lo descrito en *Interfaces* y en el flujo.** `backupOperation` guarda `AutoBackupOptions`. La lista de claves de usuario queda en `keep`, `offsite` y `offsiteKeep`, nunca `taskId`.

- [ ] **Step 4: Ejecútalas.** `verify.sh go`: PASS.

- [ ] **Step 5: Commit** `feat(tasks): a backup step chooses how often its copy goes off-site, with its own retention`

---

### Task 4: Ajustes de la copia periódica (global y por servidor), endpoints y traslado

**Files:**
- Modify:
  - `internal/config/entries.go`: añadir `BackupsS3PeriodicHours = asInt("daemon.backups.s3.periodicHours", 0)` y `BackupsS3PeriodicKeep = asInt("daemon.backups.s3.periodicKeep", 4)`.
  - `internal/web/api/settings.go`:
    - añadir las dos claves a `editableIntEntries`;
    - validar antes de guardar: `ValidPeriodicHours` y 1-50; si no son válidas, responder 400 con el error de validación de siempre.
  - `internal/servers/server_loader.go:279`: añadir `".offsite"` a las extensiones que se borran.
  - `internal/web/daemon/server.go`: rutas `GET` y `PUT /:serverId/offsite`, más el `OPTIONS`, con su swagger.
  - `internal/web/api/servers.go`:
    - `GET` con `ScopeServerBackupView`, `PUT` con `ScopeServerBackupCreate`;
    - las dos con `ResolveServerPanel` y `proxyServerRequest`, más el `OPTIONS`.
  - `internal/web/api/transfer_logic.go`: `copyOffsite()` justo después de `copyTasks()` en `prepare`. Revisa si `extransfer` necesita lo mismo y, si lo necesita, añádelo.
- Create: `internal/servers/offsite_settings.go`
- Test: `internal/servers/offsite_settings_test.go`, `internal/web/api/settings_test.go` (o el que exista), `internal/web/api/transfer_logic_test.go`

**Interfaces:**
- Produces:
  - `type OffsiteSettings struct { PeriodicHours *int \`json:"periodicHours"\`; PeriodicKeep *int \`json:"periodicKeep"\` }`.
  - `(p *Server) OffsiteSettings() (OffsiteSettings, error)` y `(p *Server) SetOffsiteSettings(s OffsiteSettings) error`: valida los valores y escribe `<ServersFolder>/<id>.offsite` con permisos 0600 bajo un mutex, con el mismo patrón que `.notify`.
  - `(p *Server) EffectivePeriodic() (hours, keep int)`: el valor del servidor si no es `nil`; si no, el de la configuración global.
  - `GET /daemon/server/:id/offsite` responde `{ periodicHours, periodicKeep, effectiveHours, effectiveKeep, configured, lastPeriodic }`. `lastPeriodic` es el RFC3339 de la memoria del trabajador (Task 5), o `null`.
  - `PUT` recibe `OffsiteSettings`. Valores inválidos dan 400.

- [ ] **Step 1: Escribe las pruebas que fallan**
  - `TestOffsiteSettings_DefaultsAndOverride`:
    - sin archivo, `EffectivePeriodic` devuelve el global (24 y 4 con la configuración puesta en la prueba);
    - con `{0,nil}`, devuelve 0 y 4;
    - `SetOffsiteSettings` con 25 horas da error.
  - `TestDeletingAServerRemovesItsOffsiteFile`.
  - `TestSettings_RejectsAnInvalidPeriodicInterval`: aplicar `daemon.backups.s3.periodicHours = 25` da error; 48 se guarda.
  - `TestServerMove_CopiesOffsiteSettings`: con los dos nodos falsos que ya usan las pruebas de traslado, el destino recibe un `PUT /offsite` con el JSON leído del origen. Si el origen responde 404 (un nodo antiguo), no se copia nada y el traslado sigue.

- [ ] **Step 2: Ejecútalas.** `verify.sh go`: deben FALLAR.

- [ ] **Step 3: Implementa lo descrito en *Interfaces*.**

- [ ] **Step 4: Ejecútalas.** `verify.sh go`: PASS.

- [ ] **Step 5: Commit** `feat(backups): periodic off-site settings, globally and per server, kept across moves`

---

### Task 5: Trabajador de la copia periódica

**Files:**
- Create: `internal/servers/offsite_periodic.go`
- Modify:
  - `internal/servers/server.go`: un `endBackup` silencioso (`endBackupQuiet(success bool)`) que solo avisa si falla.
  - `cmd/panel/run.go`: arranca `servers.StartOffsitePeriodic()` después de `servers.InitService()`.
- Test: `internal/servers/offsite_periodic_test.go`

**Interfaces:**
- Consumes:
  - Task 2: `bucket`, `startSeries` y `endSeries`;
  - Task 1: `OffsiteBackupPrefix` y `newestBeyond`;
  - Task 4: `EffectivePeriodic`.
- Produces:
  - `StartOffsitePeriodic()`: un `time.Ticker` de 10 min y un primer recorrido a los 5 min, en una goroutine.
  - `periodicPass(now time.Time)`: recorre `GetAll()` en orden y llama a `runPeriodic` para cada servidor.
  - `(p *Server) runPeriodic(now time.Time) bool`: devuelve si hizo una copia.
  - `LastPeriodic(serverID string) (time.Time, bool)`: lo usa Task 4.
  - Estado en memoria, por servidor y protegido por mutex: `last time.Time`, `seeded bool`, `failures int`, `retryAt time.Time`.

**`runPeriodic`, en orden:**
1. Termina sin hacer nada si:
   - `EffectivePeriodic` da 0 horas;
   - `!bucket.Configured()`;
   - `now` es anterior a `retryAt`;
   - `Scheduler != nil && Scheduler.IsPaused()`;
   - el servidor se está instalando (`GetEnvironment().IsInstalling()`);
   - `IsBackingUp() || IsRestoring()`.
2. Si `!seeded`: `bucket.List` y `last` = el mayor `ModTime` de los objetos `offsite-`. Un error aquí cuenta como fallo (ver el paso 6).
3. Si `now.Sub(last) < hours`: termina sin hacer nada.
4. `bucket.Reachable` antes de escribir. Si falla, cuenta como fallo y termina.
5. `beginBackup(OffsiteBackupPrefix)`, `writeArchive` y `endBackupQuiet`. Después, `bucket.Upload` de forma síncrona entre `startSeries` y `endSeries` (serie `"periodic"`). Si sale bien, borra con `bucket.Delete` los objetos `offsite-` que devuelva `newestBeyond(..., keep)` y pone `last = now`.
6. Pase lo que pase, borra la copia local.
   - Si va bien, `failures = 0`.
   - Si falla: incrementa `failures`, `retryAt = now + min(2^(failures-1) h, 24 h)` y envía el aviso de fallo (`notifyBackup(false)` y la alerta de Discord, como hoy).

- [ ] **Step 1: Escribe las pruebas que fallan**, con el bucket falso y `now` fijo:
  - `TestPeriodic_UploadsWhenDueAndRemovesTheLocalCopy`: 24 h, bucket vacío. Hay una subida `offsite-…`, ninguna copia en local y la consola no tiene aviso de éxito (o el contador de avisos es 0).
  - `TestPeriodic_WaitsForTheInterval`: `last` hace 23 h, no hay copia; a las 24 h, sí.
  - `TestPeriodic_PerServerOffAndDefaults`: el global da 24 y el servidor da 0, así que no hay copia.
  - `TestPeriodic_SkipsBusyOrPausedServers`: `Scheduler.Paused`, `IsInstalling` y una copia en curso no producen ninguna subida.
  - `TestPeriodic_KeepsOnlyItsOwnSeries`: con `periodicKeep` 2, el bucket tiene 3 `offsite-`, 2 `auto-t1-` y 1 manual. Quedan 2 `offsite-` y las demás intactas.
  - `TestPeriodic_UnreachableBucketWritesNothingAndBacksOff`: con `failReach`:
    - no se escribe ninguna copia local y `retryAt` queda en `now`+1 h;
    - un segundo fallo deja `retryAt` en +2 h;
    - tras 6 fallos, el límite es +24 h.
  - `TestPeriodic_FailedUploadStillRemovesTheLocalCopy`.

- [ ] **Step 2: Ejecútalas.** `verify.sh go`: deben FALLAR.

- [ ] **Step 3: Implementa lo descrito en *Interfaces* y en el flujo.** Usa `endBackupQuiet` también en la copia periódica que falla al escribirse.

- [ ] **Step 4: Ejecútalas.** `verify.sh go`: PASS.

- [ ] **Step 5: Commit** `feat(backups): each server gets a periodic off-site copy without needing a task`

---

### Task 6: Panel: la copia periódica en la lista de copias

**Files:**
- Modify: `internal/web/api/tasks.go`: `isSystemBackup` y `addSystemBackups`.
- Test: el archivo de pruebas de la sincronización de copias (`internal/web/api/*backup*_test.go` o `tasks_test.go`).

**Interfaces:**
- `isSystemBackup` también reconoce `servers.OffsiteBackupPrefix`.
- En `addSystemBackups`, el nombre de esa copia es `"Periodic backup (S3)"`.

- [ ] **Step 1: Escribe la prueba que falla.** `TestSyncBackups_RecordsPeriodicCopies`:
  - el nodo devuelve `offsite-<uuid>.tar.gz` con `Location "offsite"`;
  - debe crearse una fila llamada `Periodic backup (S3)`;
  - si el nodo deja de devolverla, la fila se borra.
- [ ] **Step 2: Ejecútala.** FAIL.
- [ ] **Step 3: Implementa.**
- [ ] **Step 4: Ejecútala.** `verify.sh go`: PASS.
- [ ] **Step 5: Commit** `feat(backups): the panel lists periodic off-site copies`

---

### Task 7: Interfaz (Ajustes, pestaña Copias y diálogo de tareas)

**Files:**
- Modify:
  - `client/frontend/src/features/settings/offsite-tab.tsx`;
  - `client/frontend/src/features/servers/[id]/backups-view.tsx`;
  - `client/frontend/src/features/servers/[id]/task-dialog.tsx`;
  - `client/frontend/src/lib/locales/es.json`;
  - `client/frontend/src/lib/locales/en.json`.
- Create: `client/frontend/src/features/servers/[id]/offsite-card.tsx`.

**Interfaces:**
- Consumes:
  - `GET/PUT /api/servers/:id/offsite` (Task 4);
  - las claves de configuración `daemon.backups.s3.periodicHours` y `daemon.backups.s3.periodicKeep`;
  - los argumentos del paso `backup`: `keep`, `offsite` y `offsiteKeep`.
- Opciones de intervalo, compartidas entre Ajustes y la tarjeta del servidor:

  ```ts
  export const PERIODIC_HOURS = [0, 12, 24, 48, 72, 168, 336, 720] as const;
  ```

  Las etiquetas salen de i18n: Desactivada, 12 h, 24 h, 48 h, 72 h, 7 días, 14 días, 30 días.
- Frecuencias de tarea:

  ```ts
  export const OFFSITE_FREQUENCIES = ['never','always','weekend','weekly','biweekly','monthly'] as const;
  ```

  Un paso `backup` nuevo trae `offsite: 'weekly'` y `offsiteKeep: 4`. Al editar una tarea cuyo paso no tiene `offsite`, se muestra `always`.

**Steps:**

- [ ] **Step 1: `offsite-tab.tsx`.** Una sección «Copia periódica de cada servidor», con los componentes `SettingsSection`/`SettingRow` que ya existen:
  - un select de intervalo y un número «Copias a conservar en S3» (1-50) que se guardan con `handleUpdate`;
  - el texto de ayuda incluye el aviso de nodos remotos que ya hay.

  Actualiza también el comentario JSDoc del componente: ya no sube cada copia.
- [ ] **Step 2: `offsite-card.tsx`.** La tarjeta se monta en `backups-view.tsx` encima de la lista y tiene:
  - un select con «Por defecto (`effectiveHours`)» primero y después las opciones;
  - un número «Conservar en S3», con un botón para volver al valor por defecto;
  - «Última copia periódica: <fecha>» si hay `lastPeriodic`.

  Si `configured` es falso, muestra la línea «Este nodo no tiene S3 configurado» y desactiva los controles. Para cambiar hace falta el permiso de crear copias; sin él, la tarjeta es solo de lectura. Debe verse bien con un ancho de 360 px.
- [ ] **Step 3: `task-dialog.tsx`.** Junto a «Conservar», en el paso `backup`, añade:
  - el select «Subir al S3»;
  - «Conservar en S3», oculto si la frecuencia es `never`;
  - la línea de ayuda «Las demás ejecuciones guardan la copia solo en el servidor».

  En móvil, los controles se apilan.
- [ ] **Step 4: Textos.** Añade todas las claves nuevas en `es.json` y `en.json`, en las secciones `settings.offsite.*`, `servers.backups.offsite.*` y `servers.tasks.steps.*`. Corrige el texto que diga que se sube cada copia.
- [ ] **Step 5: Verifica.** `verify.sh fe`: `fe-lint`, `fe-typecheck` y `fe-build-ok` sin errores.
- [ ] **Step 6: Prueba en el navegador**, en el VPS con la imagen nueva (método del resumen de la sesión: construir `skypanel-skypanel:nodefix` y levantarla). Revisa las tres pantallas a 360, 768, 1280 y 1920 px. Después deja `/opt/skypanel` en main.
- [ ] **Step 7: Commit** `feat(frontend): periodic off-site copies and per-task upload frequency`

---

### Task 8: Documentación

**Files:**
- Modify, en `docs/es` y `docs/en`:
  - `features/backups.md`: secciones de copias externas y de tareas, tabla de prefijos, avisos y retención;
  - `features/panel-settings.md`;
  - `backend/02-configuration.md`: las dos claves nuevas y sus valores.
- Swagger regenerado si el repositorio lo versiona (`internal/web/swagger/docs.go`). Revisa cómo lo hace CI.

- [ ] **Step 1:** Escribe la documentación (español e inglés, con el mismo contenido). Debe cubrir:
  - las copias manuales ya no suben;
  - la frecuencia de las tareas;
  - la copia periódica;
  - la retención propia;
  - qué pasa en nodos remotos.
- [ ] **Step 2:** Comprueba que los enlaces internos apuntan a anclas que existen; usa `grep` para los encabezados.
- [ ] **Step 3: Commit** `docs: periodic off-site copies and per-task upload frequency`

---

## Cierre

- `verify.sh all` limpio.
- Prueba en vivo en el VPS: con el S3 configurado y `periodicHours` en 12, fuerza `runPeriodic` a mano. Si no hay S3 de prueba, como mínimo comprueba que la tarjeta muestra «Este nodo no tiene S3 configurado».
- Después deja el VPS en main.
- PR a main, con CI y SonarCloud en verde, y merge.
