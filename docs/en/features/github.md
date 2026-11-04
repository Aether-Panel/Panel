# Install from GitHub

Each user can connect their GitHub account and install one of their repositories, private ones included, into a server. They pick the default branch or another one. The node downloads the project straight from GitHub, so a big project never goes through the browser or SFTP. It is meant for Node.js, Python and other language servers, and works on any server.

## What it does

- **Settings → GitHub** (administrator): one field, the Client ID of a GitHub OAuth App.
- **Admin → Installation** of a server: the **Install from GitHub** card lets you:
  - connect the account with a code, and unlink it;
  - list the account's repositories, search them and pick a branch;
  - install.
- The connection belongs to each user: it works on all their servers, and nobody else sees their private repositories.

## How it works

### Connecting (OAuth device flow)

1. **Connect GitHub** asks GitHub for a code (`POST https://github.com/login/device/code`, scope `repo`). The panel keeps the device code in memory and sends the browser only the code for the user.
2. The user opens `github.com/login/device`, signs in and enters the code.
3. The page asks the panel every few seconds. The panel asks GitHub no faster than GitHub allows and, once it gets the token, it:
   - reads the account;
   - seals the token with AES-GCM, under a key derived from `panel.sessionKey`;
   - stores it in `github_accounts`, one row per user.

The token never reaches the browser. `repo` is the only OAuth App scope that reaches private repositories; GitHub offers no read-only one. **Unlink** deletes the row. To revoke the access on GitHub too: *Settings → Applications → Authorized OAuth Apps*. When GitHub stops accepting the token, the panel drops the connection and the card offers **Connect GitHub** again.

### Installing

1. The panel checks the account can see the repository and the branch (`GET /repos/{owner}/{repo}` and `/branches/{branch}`). With no branch chosen it uses the default one.
2. It asks GitHub for the branch's archive (`GET /repos/{owner}/{repo}/tarball/{branch}`) without following the redirect. GitHub answers with a `codeload.github.com` address carrying a short-lived token, valid for that one archive and a few minutes.
3. It sends the node only that address, with the repository name and the branch. **The user's token never leaves the panel**, and the node only accepts `https://codeload.github.com` addresses.
4. The node, in the background:
   1. downloads the archive into the server's tmp folder (`<server>.tmp`, which counts against its disk limit and is neither in the file manager nor in backups);
   2. stops the server if it is running;
   3. saves a **restore point** (`restore-github-…`, in Backups);
   4. empties the server folder;
   5. unpacks the archive without the top folder GitHub adds, and removes the download.
5. The server does not start again by itself. The card and the console show each step and the MB downloaded.

While it runs, the server accepts no other GitHub install, modpack, version change or reinstall. If the download fails, nothing changes. What arrives is the branch's content, not a `.git` folder. To update the project, install again.

## Usage

### Administrator: create the OAuth App

1. On GitHub: **Settings → Developer settings → OAuth Apps → New OAuth App**. It can belong to an account or an organisation.
2. Give it a name, which users see when they connect. For *Homepage URL* and *Authorization callback URL*, use the panel's address: the device flow does not use the callback.
3. Once registered, tick **Enable Device Flow** and save.
4. Copy the **Client ID** into **Settings → GitHub** and save. No *Client secret* is needed.

### User: install a repository

Needs `server.install` on the server.

1. **Admin → Installation → Connect GitHub**, open GitHub and enter the code. The window closes by itself.
2. Search for and pick the repository. The default branch is selected; another can be chosen.
3. **Install → Delete and install**.
4. When it finishes, set up and start the server as usual (for example `npm install` or `pip install` from its template).

## API

| Method | Route | Scope | Use |
|--------|-------|-------|-----|
| `GET` | `/api/self/github` | `self.edit` | `{configured, connected, login, avatarUrl, connectedAt}` |
| `POST` | `/api/self/github/device` | `self.edit` | Starts connecting: `{userCode, verificationUri, expiresIn, interval}` |
| `POST` | `/api/self/github/device/poll` | `self.edit` | `{status: "pending" \| "connected" \| "expired" \| "denied"}` |
| `DELETE` | `/api/self/github` | `self.edit` | Unlinks (`204`) |
| `GET` | `/api/self/github/repos?page=N` | `self.edit` | `{repos: [{fullName, name, owner, private, defaultBranch, description, updatedAt, sizeKb}], more}`, 100 at a time |
| `GET` | `/api/self/github/repos/{owner}/{repo}/branches` | `self.edit` | `{defaultBranch, branches}` |
| `POST` | `/api/servers/{id}/github/install` | `server.install` | `{repository: "owner/name", branch: ""}`; `202`, then runs in the background |
| `GET` | `/api/servers/{id}/github/install` | `server.install` | `{installing, step, repository, branch, downloaded, lastError}` |

Steps (`step`): `stopping`, `downloading`, `restorePoint`, `cleaning`, `extracting`, `done`, `failed`.

## Configuration

| Key | Default | Environment variable | Description |
|-----|---------|----------------------|-------------|
| `panel.github.clientId` | `""` | `SKYPANEL_PANEL_GITHUB_CLIENTID` | Client ID of the OAuth App (with Device Flow). Empty: the feature is off. |

If `panel.sessionKey` changes, the stored tokens can no longer be opened and each user connects again.

## Known issues and solutions

| Message | Cause and solution |
|---------|--------------------|
| "GitHub is not set up on this panel" (`ErrGitHubNotConfigured`, 400) | The Client ID is missing in **Settings → GitHub**. |
| "GitHub does not know that Client ID…" (`ErrGitHubBadClient`, 400) | The Client ID is mistyped or the OAuth App does not exist. Copy it again from the app on GitHub. |
| `GitHub answered 400: device_flow_disabled` | The OAuth App does not have **Enable Device Flow** ticked. |
| "Connect your GitHub account first" (`ErrGitHubNotConnected`, 409) | The user has not connected an account, or unlinked it. |
| "GitHub could not find that repository or branch…" (`ErrGitHubNotFound`, 404) | The branch no longer exists or the account has no access. For an organisation's repository, the organisation may restrict OAuth Apps: an owner approves the app in *Organization → Settings → Third-party access*. |
| "GitHub no longer accepts this connection" (`ErrGitHubTokenRevoked`, 422) | The access was revoked on GitHub, or `panel.sessionKey` changed. Connect again. |
| "A GitHub install is already running" (409) | Another install, a modpack, a backup or a version change is running. Wait for it to finish. |
| The install fails with a disk error | The project does not fit in the server's disk limit. Raise it or slim the repository down. |

## See also

- [Server settings](server-settings.md)
- [Backups and restore points](backups.md)
- [Panel settings](panel-settings.md)
