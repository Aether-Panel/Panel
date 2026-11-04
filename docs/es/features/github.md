# Instalar desde GitHub

Cada usuario puede conectar su cuenta de GitHub e instalar uno de sus repositorios, también privados, en un servidor. Elige la rama principal u otra. El nodo descarga el proyecto directamente de GitHub, así que un proyecto grande no pasa por el navegador ni por SFTP. Pensado para servidores de Node.js, Python y otros lenguajes, funciona en cualquier servidor.

## Qué hace

- **Configuración → GitHub** (administrador): un único campo, el Client ID de una OAuth App de GitHub.
- **Administración → Instalación** de un servidor: la tarjeta **Instalar desde GitHub** permite:
  - conectar la cuenta con un código y desvincularla;
  - ver los repositorios de la cuenta, buscarlos y elegir rama;
  - instalar.
- La conexión es de cada usuario: sirve en todos sus servidores y nadie más ve sus repositorios privados.

## Cómo funciona

### Conexión (flujo de dispositivo de OAuth)

1. **Conectar con GitHub** pide un código a GitHub (`POST https://github.com/login/device/code`, permiso `repo`). El panel guarda el código de dispositivo en memoria y solo envía al navegador el código para el usuario.
2. El usuario abre `github.com/login/device`, inicia sesión y escribe el código.
3. La página pregunta al panel cada pocos segundos. El panel consulta a GitHub como mucho al ritmo que GitHub indica y, cuando recibe el token:
   - lee la cuenta;
   - cifra el token con AES-GCM, con una clave derivada de `panel.sessionKey`;
   - lo guarda en `github_accounts`, una fila por usuario.

El token nunca llega al navegador. `repo` es el único permiso de las OAuth Apps que da acceso a repositorios privados; GitHub no ofrece uno de solo lectura. **Desvincular** borra la fila. Para revocar también el acceso en GitHub: *Settings → Applications → Authorized OAuth Apps*. Si GitHub deja de aceptar el token, el panel borra la conexión y la tarjeta vuelve a ofrecer **Conectar con GitHub**.

### Instalación

1. El panel comprueba que la cuenta ve el repositorio y la rama (`GET /repos/{owner}/{repo}` y `/branches/{branch}`). Si no se elige rama, usa la principal.
2. Pide a GitHub el archivo de la rama (`GET /repos/{owner}/{repo}/tarball/{rama}`) sin seguir la redirección. GitHub responde con una dirección de `codeload.github.com` que lleva un token temporal, válido solo para ese archivo y unos minutos.
3. Envía al nodo solo esa dirección, junto con el nombre del repositorio y la rama. **El token del usuario nunca sale del panel**, y el nodo solo acepta direcciones `https://codeload.github.com`.
4. El nodo, en segundo plano:
   1. descarga el archivo en la carpeta temporal del servidor (`<servidor>.tmp`, que cuenta para su límite de disco y no aparece en el gestor de archivos ni en las copias);
   2. detiene el servidor si está encendido;
   3. guarda un **punto de restauración** (`restore-github-…`, en Copias de seguridad);
   4. vacía la carpeta del servidor;
   5. descomprime el archivo sin la carpeta raíz que añade GitHub y borra la descarga.
5. El servidor no se vuelve a encender solo. La tarjeta y la consola muestran cada paso y los MB descargados.

Mientras dura, el servidor no admite otra instalación de GitHub, un modpack, un cambio de versión ni una reinstalación. Si la descarga falla, no se toca nada. Se trae el contenido de la rama, no la carpeta `.git`. Para actualizar el proyecto se vuelve a instalar.

## Uso

### Administrador: crear la OAuth App

1. En GitHub: **Settings → Developer settings → OAuth Apps → New OAuth App**. Puede ser de una cuenta o de una organización.
2. Pon un nombre, que verán los usuarios al conectar. En *Homepage URL* y *Authorization callback URL*, la dirección del panel: el flujo de dispositivo no usa la URL de retorno.
3. Tras registrarla, marca **Enable Device Flow** y guarda.
4. Copia el **Client ID** en **Configuración → GitHub** y guarda. No hace falta *Client secret*.

### Usuario: instalar un repositorio

Requiere el permiso `server.install` sobre el servidor.

1. **Administración → Instalación → Conectar con GitHub**, abre GitHub y escribe el código. La ventana se cierra sola.
2. Busca y elige el repositorio. La rama principal viene marcada; se puede elegir otra.
3. **Instalar → Borrar e instalar**.
4. Cuando termine, configura e inicia el servidor como siempre (por ejemplo `npm install` o `pip install` desde su plantilla).

## API

| Método | Ruta | Permiso | Uso |
|--------|------|---------|-----|
| `GET` | `/api/self/github` | `self.edit` | `{configured, connected, login, avatarUrl, connectedAt}` |
| `POST` | `/api/self/github/device` | `self.edit` | Empieza la conexión: `{userCode, verificationUri, expiresIn, interval}` |
| `POST` | `/api/self/github/device/poll` | `self.edit` | `{status: "pending" \| "connected" \| "expired" \| "denied"}` |
| `DELETE` | `/api/self/github` | `self.edit` | Desvincula (`204`) |
| `GET` | `/api/self/github/repos?page=N` | `self.edit` | `{repos: [{fullName, name, owner, private, defaultBranch, description, updatedAt, sizeKb}], more}`, de 100 en 100 |
| `GET` | `/api/self/github/repos/{owner}/{repo}/branches` | `self.edit` | `{defaultBranch, branches}` |
| `POST` | `/api/servers/{id}/github/install` | `server.install` | `{repository: "owner/name", branch: ""}`; `202` y sigue en segundo plano |
| `GET` | `/api/servers/{id}/github/install` | `server.install` | `{installing, step, repository, branch, downloaded, lastError}` |

Pasos (`step`): `stopping`, `downloading`, `restorePoint`, `cleaning`, `extracting`, `done`, `failed`.

## Configuración

| Clave | Por defecto | Variable de entorno | Descripción |
|-------|-------------|---------------------|-------------|
| `panel.github.clientId` | `""` | `SKYPANEL_PANEL_GITHUB_CLIENTID` | Client ID de la OAuth App (con Device Flow). Vacío: la función está desactivada. |

Si cambia `panel.sessionKey`, los tokens guardados no se pueden descifrar y cada usuario vuelve a conectar su cuenta.

## Problemas conocidos y soluciones

| Mensaje | Causa y solución |
|---------|------------------|
| «GitHub no está configurado en este panel» (`ErrGitHubNotConfigured`, 400) | Falta el Client ID en **Configuración → GitHub**. |
| «GitHub no reconoce ese Client ID…» (`ErrGitHubBadClient`, 400) | El Client ID está mal copiado o la OAuth App no existe. Cópialo de nuevo desde la app en GitHub. |
| `GitHub answered 400: device_flow_disabled` | La OAuth App no tiene marcado **Enable Device Flow**. |
| «Connect your GitHub account first» (`ErrGitHubNotConnected`, 409) | El usuario no ha conectado su cuenta, o la desvinculó. |
| «GitHub could not find that repository or branch…» (`ErrGitHubNotFound`, 404) | La rama ya no existe o la cuenta no tiene acceso. En repositorios de una organización, puede que la organización restrinja las OAuth Apps: un propietario debe aprobar la app en *Organization → Settings → Third-party access*. |
| «GitHub no longer accepts this connection» (`ErrGitHubTokenRevoked`, 422) | Se revocó el acceso en GitHub o cambió `panel.sessionKey`. Volver a conectar. |
| «A GitHub install is already running» (409) | Hay otra instalación, un modpack, una copia o un cambio de versión en marcha. Esperar a que termine. |
| La instalación falla con un error de disco | El proyecto no cabe en el límite de disco del servidor. Ampliarlo o reducir el repositorio. |

## Véase también

- [Configuración del servidor](server-settings.md)
- [Copias de seguridad y puntos de restauración](backups.md)
- [Ajustes del panel](panel-settings.md)
