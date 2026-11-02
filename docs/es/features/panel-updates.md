# Actualizaciones del panel

Aether Panel puede consultar en GitHub si hay una versión más reciente de la rama desde la que se instaló y actualizarse a sí mismo, junto con todos los nodos registrados, desde **Configuración → Actualizaciones**. Esta página explica cómo decide la comprobación que hay una actualización, qué se ejecuta realmente en cada máquina y cómo seguir el proceso o resolver sus fallos.

## Qué hace

- **Comprobación de actualizaciones**: compara el commit que ejecuta el panel con el último commit de su rama en GitHub (`Aether-Panel/Panel`) e indica si hay una actualización y cuántos commits le faltan a la instalación.
- **Actualización con un clic**: lanza el script de actualización en la máquina del panel y pide a cada nodo remoto que haga lo mismo.
- **Seguimiento del progreso**: muestra, por nodo, si la actualización está en curso, su código de salida y las últimas líneas de su registro.
- **Aviso de actualización**: quien tiene `settings.edit` recibe un aviso «Actualización disponible» una vez por sesión del navegador cuando la comprobación encuentra una.

## Cómo funciona

### La comprobación (`GET /api/settings/update-check`)

1. El panel lee el commit desplegado de la carpeta `.git` del repositorio, que `docker-compose.yml` monta en solo lectura en `/repo/.git` (`./.git:/repo/.git:ro`). Se admiten referencias empaquetadas (`packed-refs`).
2. La rama es la que tiene activa ese repositorio. Si HEAD está separado (detached), no se puede leer o tiene un nombre no seguro, se usa `main`.
3. El panel pide a la API de GitHub el último commit de esa rama. Solo se compara esa rama, porque es la que descarga el script de actualización.
4. Si los commits difieren, el panel usa la API de comparación de GitHub. Solo se ofrece actualización cuando la rama va **por delante** *y* cambia al menos un archivo (un commit de merge que no cambia nada no cuenta). `behindBy` es el número de commits que le faltan a la instalación.
5. Si el commit desplegado no existe en GitHub (commits locales), el panel comprueba la ascendencia en local con `git merge-base --is-ancestor`. Git se ejecuta en un contenedor efímero `alpine/git:latest` que reutiliza los montajes del contenedor del panel cuando el socket de Docker está disponible; si no, mediante un binario de git en una ruta fija.
6. **Filtro de verificación**: antes de ofrecer la actualización, el panel lee las comprobaciones (check runs) de GitHub sobre el nuevo commit. Mientras alguna siga en marcha, el resultado lleva `verification: "pending"` y no se ofrece; si alguna falló (`failure`, `timed_out`, `startup_failure`, `action_required`), lleva `verification: "failed"` y tampoco se ofrece. Un commit sin comprobaciones, o un GitHub que no responde a esta consulta, no retiene la actualización.

La respuesta contiene `current`, `latest`, `version`, `branch`, `updateAvailable`, `behindBy` y, cuando procede, `reason` y `verification`. Las consultas a GitHub son anónimas y caducan a los 15 segundos.

### La actualización (`POST /api/settings/update-panel`)

Para cada nodo registrado:

- **Nodo local** (la máquina del panel; se lanza una sola vez): el panel usa el socket de Docker para arrancar un contenedor efímero `alpine:latest` con red del host y el sistema de archivos raíz del host montado en `/host`. Dentro ejecuta:

  ```bash
  chroot /host bash -c "cd /opt/skypanel && chmod +x tools/panelUpdate/panelUpdate.sh && ./tools/panelUpdate/panelUpdate.sh"
  ```

  Si la imagen no existe, se descarga antes.
- **Nodo remoto**: el panel llama a `POST /daemon/update` en ese nodo (autenticado con el JWT del panel) y el nodo arranca el mismo contenedor en su propia máquina.

Cada proceso guarda en memoria el estado de su última actualización (en curso, ID del contenedor, inicio y fin, código de salida, las últimas 100 líneas del registro con un máximo de 8000 caracteres y el error) y consulta el contenedor cada 2 segundos. Cuando el contenedor termina, se elimina.

### Qué hace `panelUpdate.sh`

El script está en `tools/panelUpdate/panelUpdate.sh` y espera la instalación en `/opt/skypanel` con un contenedor llamado `skypanel`:

1. Busca `docker-compose` o `docker compose`; si no hay ninguno, falla.
2. Elige la rama en este orden: la variable de entorno `AETHER_BRANCH`, `GIT_BRANCH=` en `/etc/SkyPanel/installer.conf` (lo escribe el instalador), la rama activa del repositorio y, por último, `main`. **Una actualización nunca cambia de rama.**
3. Si `docker-compose.yml` tiene activo `SKYPANEL_PANEL_ENABLE=false` (nodo solo daemon), lo recuerda junto con el valor de `SKYPANEL_TOKEN_PUBLIC` para volver a aplicarlos tras la descarga.
4. Hace copia de `.env`, ejecuta `git checkout -- .` y `git fetch origin`, comprueba que existe `origin/<rama>` y después `git checkout <rama>` y `git pull --ff-only`. Al terminar restaura `.env`, de modo que las contraseñas guardadas allí se conservan.
5. Vuelve a aplicar, si hace falta, la configuración de nodo solo daemon en `docker-compose.yml`.
6. Construye la nueva imagen **mientras el panel anterior sigue en marcha** (`COMPOSE_PROFILES=local-db docker compose build`). Si la compilación falla, sigue funcionando la versión anterior.
7. Reinicia con `COMPOSE_PROFILES=local-db docker compose up -d`.
8. Reduce la caché de compilación de Docker a 3 GB (`docker builder prune`) para que las actualizaciones sucesivas no llenen el disco.

Como `git checkout -- .` descarta los cambios locales en archivos versionados, cualquier edición manual de archivos versionados distintos de `.env` (por ejemplo, `docker-compose.yml`) se pierde al actualizar.

### Estado (`GET /api/settings/update-status`)

El panel devuelve una entrada por nodo con `name`, `address`, `local`, `online`, `running`, `containerId`, `startedAt`, `finishedAt`, `exitCode`, `log` y `error`. Para los nodos remotos llama a `GET /daemon/update-status`; un nodo inaccesible aparece con `online: false` y el motivo en `error`.

## Uso

### Desde el panel

1. Abra **Configuración → Actualizaciones** (pestaña «Actualizaciones», tarjeta «Actualizaciones del sistema»). La comprobación se ejecuta al abrir la pestaña; **Buscar actualizaciones** la repite.
2. La tarjeta muestra **Actual** (versión del panel con el commit desplegado abreviado entre paréntesis) y **Última** (commit abreviado de la punta de la rama), además de una línea de estado:
   - «El sistema está actualizado», «Actualización disponible» (con «La rama tiene N cambio(s) que este panel aún no tiene.»), «Versión nueva en verificación» (comprobaciones aún en marcha en GitHub), «No se pudo comprobar si hay actualizaciones» o «No se pudo verificar la versión actual».
3. Pulse **Actualizar panel** y confirme con **Actualizar ahora**. El botón está desactivado mientras el sistema esté actualizado.
4. Durante la actualización, la página consulta el estado cada 3 segundos y lista cada nodo («Este panel» marca el local) con «Actualizando…», «Completado», «Error (código N)» o «No responde», y un control «Ver registros» con el final del registro.
5. Cuando el propio panel se reinicia, la página muestra «El panel se está reiniciando…» y sigue consultando hasta que vuelve a responder. Si todos los nodos terminan sin errores, la página se recarga sola a los pocos segundos para cargar el nuevo frontend.

### Mediante la API

| Método | Ruta | Permiso | Propósito |
|--------|------|---------|-----------|
| `GET` | `/api/settings/update-check` | `settings.edit` | Comparar el commit desplegado con la punta de la rama en GitHub |
| `POST` | `/api/settings/update-panel` | `settings.edit` | Lanzar la actualización en la máquina del panel y en cada nodo |
| `GET` | `/api/settings/update-status` | `settings.edit` | Estado, código de salida y final del registro de cada nodo |
| `POST` | `/daemon/update` | JWT del panel | Lanzar la actualización en ese nodo (lo llama el panel) |
| `GET` | `/daemon/update-status` | JWT del panel | Estado de la actualización de ese nodo (lo llama el panel) |

`POST /api/settings/update-panel` responde `{"message": "Update initiated on the panel and all nodes.", "nodes": [...]}`; el nodo que no se pudo lanzar incluye un campo `error`. Los formatos completos están en la [referencia de la API](../reference/11-api-reference.md).

## Requisitos y configuración

- Una instalación con Docker hecha por el instalador, con el repositorio en **`/opt/skypanel`** en cada máquina (panel y nodos). La ruta del script y la carpeta de instalación están fijadas en el código.
- El socket de Docker montado en el contenedor del panel o del daemon (`/var/run/docker.sock`, como en `docker-compose.yml`).
- `./.git` montado en `/repo/.git` (solo lectura) para que la comprobación conozca el commit actual. Sin él, la comprobación responde con `current` vacío y la interfaz muestra «No se pudo verificar la versión actual».
- Salida HTTPS desde el panel hacia `api.github.com` (comprobación) y desde cada máquina hacia GitHub (git fetch) y los registros de imágenes que use la compilación.
- Ninguna clave de configuración controla esta función. La rama sale del repositorio o de `/etc/SkyPanel/installer.conf`; `AETHER_BRANCH` solo se lee si está definida en el entorno donde corre el script. El resto de ajustes del panel están en [Configuración](../backend/02-configuration.md).

## Límites

- GitHub permite 60 consultas anónimas por hora y por IP. Cada comprobación usa hasta tres (punta, comparación y comprobaciones). La comprobación automática de la aplicación se hace una vez por sesión del navegador.
- Solo se guarda el estado de la **última** actualización de cada proceso, y solo en memoria: reiniciar el panel o el daemon lo borra.
- La actualización reconstruye la imagen en cada máquina; tarda varios minutos y necesita espacio en disco para la compilación (después la caché se reduce a 3 GB).
- El panel y los servidores pueden quedar inaccesibles unos minutos durante el reinicio.
- No cubre instalaciones sin Docker: el lanzamiento siempre usa Docker y `/opt/skypanel`.

## Problemas conocidos y soluciones

| Síntoma / mensaje | Causa | Solución |
|-------------------|-------|----------|
| «No se pudo comprobar si hay actualizaciones» con el texto del límite de GitHub (`reason: github_rate_limited`) | GitHub respondió 403/429: se agotaron las 60 consultas por hora de esta IP | Esperar y reintentar. La actualización sigue funcionando |
| «No se pudo comprobar si hay actualizaciones» (`github_unreachable` / `github_error`) | No hay ruta hacia `api.github.com` o la respuesta fue inesperada | Revisar la salida HTTPS y el DNS del contenedor del panel; consultar el registro del panel (`Update check failed to reach GitHub`) |
| «No se pudo verificar la versión actual» | `/repo/.git` no está montado o su HEAD no se puede leer | Restaurar el volumen `./.git:/repo/.git:ro` en `docker-compose.yml` |
| «Versión nueva en verificación» | Las comprobaciones de GitHub sobre el nuevo commit siguen en marcha | Esperar a que terminen; la actualización se ofrece cuando pasan |
| Aviso «Actualización incompleta: No se pudo iniciar la actualización en: …» | Un nodo remoto no respondió a `POST /daemon/update` o devolvió un error | Comprobar que el nodo está en línea y es accesible desde el panel; actualizarlo a mano si hace falta |
| Fila del nodo «Error (código N)» | `panelUpdate.sh` falló en esa máquina | Abrir «Ver registros». Líneas habituales: `No se encontró docker compose.`, `No se detectó el contenedor de skypanel en ejecución.`, `La rama X no existe en GitHub. No se actualiza nada.` o un error de compilación |
| `failed to connect to docker` / `failed to pull alpine:latest` / `failed to create update container` | El socket de Docker no está montado o no se puede descargar la imagen | Montar `/var/run/docker.sock`; revisar el acceso al registro |
| Falla `git pull --ff-only` | Hay commits locales en la máquina que divergen de la rama | Resolver a mano el repositorio de `/opt/skypanel` y reintentar |

Para actualizar a mano cualquier máquina, ejecute el mismo script desde el host: `cd /opt/skypanel && ./tools/panelUpdate/panelUpdate.sh`.

## Véase también

- [Conceptos básicos: propagación de actualizaciones del panel](../general/concepts.md)
- [Instalación](../setup/01-installation.md)
- [Configuración](../backend/02-configuration.md)
- [Referencia de la API](../reference/11-api-reference.md)
- [Panel de inicio y nodos](dashboard-and-nodes.md)
