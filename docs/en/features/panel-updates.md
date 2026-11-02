# Panel updates

Aether Panel can check GitHub for a newer version of the branch it was installed from and update itself, together with every registered node, from **Settings → Updates**. This page explains how the check decides that an update exists, what the update actually runs on each machine, and how to follow or troubleshoot it.

## What it does

- **Update check**: compares the commit the panel is running with the latest commit of its branch on GitHub (`Aether-Panel/Panel`) and says whether an update is available and how many commits behind the install is.
- **One-click update**: starts the update script on the panel's own host and asks every remote node to do the same.
- **Progress tracking**: shows, per node, whether the update is running, its exit code and the last lines of its log.
- **Update notice**: users with `settings.edit` get an "Update available" toast once per browser session when the check finds an update.

## How it works

### The check (`GET /api/settings/update-check`)

1. The panel reads the deployed commit from the repository's `.git` folder, which `docker-compose.yml` mounts read-only at `/repo/.git` (`./.git:/repo/.git:ro`). Packed refs are supported.
2. The branch is the one checked out in that repository. If HEAD is detached, unreadable or has an unsafe name, `main` is used.
3. The panel asks the GitHub API for the tip of that branch. Only that branch is compared, because it is the one the update script pulls.
4. If the commits differ, the panel calls GitHub's compare API. An update is offered only when the branch is **ahead** *and* at least one file changed (a merge commit that changes nothing is not an update). `behindBy` is the number of commits the install lacks.
5. If the deployed commit does not exist on GitHub (local commits), the panel checks ancestry locally with `git merge-base --is-ancestor`. Git runs in an ephemeral `alpine/git:latest` container that reuses the panel container's mounts when the Docker socket is available, or else through a git binary at a fixed path.
6. **Verification gate**: before offering an update, the panel reads GitHub's check runs for the new commit. While any check is still running the result is `verification: "pending"` and no update is offered; if one failed (`failure`, `timed_out`, `startup_failure`, `action_required`) the result is `verification: "failed"` and no update is offered. A commit with no checks, or a GitHub that does not answer this question, does not hold the update back.

The response has the fields `current`, `latest`, `version`, `branch`, `updateAvailable`, `behindBy` and, when relevant, `reason` and `verification`. The GitHub calls are anonymous and time out after 15 seconds.

### The update (`POST /api/settings/update-panel`)

For each registered node:

- **Local node** (the machine running the panel, triggered only once): the panel uses the Docker socket to start an ephemeral `alpine:latest` container with host networking and the host's root filesystem mounted at `/host`. Inside it runs:

  ```bash
  chroot /host bash -c "cd /opt/skypanel && chmod +x tools/panelUpdate/panelUpdate.sh && ./tools/panelUpdate/panelUpdate.sh"
  ```

  The image is pulled first if it is missing.
- **Remote node**: the panel calls `POST /daemon/update` on that node (authenticated with the panel's JWT), and the node starts the same container on its own host.

Each process keeps the state of its latest update in memory (running, container ID, start/finish times, exit code, the last 100 log lines capped at 8000 characters, error) and polls the container every 2 seconds. When the container exits it is removed.

### What `panelUpdate.sh` does

The script lives at `tools/panelUpdate/panelUpdate.sh` and expects the install at `/opt/skypanel` with a container named `skypanel`:

1. Finds `docker-compose` or `docker compose`; fails if neither exists.
2. Picks the branch, in this order: the `AETHER_BRANCH` environment variable, `GIT_BRANCH=` in `/etc/SkyPanel/installer.conf` (written by the installer), the branch checked out in the repository, then `main`. **An update never switches branch.**
3. If `docker-compose.yml` has `SKYPANEL_PANEL_ENABLE=false` active (a daemon-only node), it remembers that and the `SKYPANEL_TOKEN_PUBLIC` value so it can apply them again after the pull.
4. Backs up `.env`, runs `git checkout -- .`, `git fetch origin`, checks that `origin/<branch>` exists, then `git checkout <branch>` and `git pull --ff-only`. `.env` is restored afterwards, so passwords kept there survive.
5. Re-applies the daemon-only settings in `docker-compose.yml` if needed.
6. Builds the new image **while the old panel keeps running** (`COMPOSE_PROFILES=local-db docker compose build`). If the build fails, the running version stays up.
7. Restarts with `COMPOSE_PROFILES=local-db docker compose up -d`.
8. Prunes the Docker build cache down to 3 GB (`docker builder prune`), so repeated updates do not fill the disk.

Because `git checkout -- .` discards local changes to tracked files, any manual edit to tracked files other than `.env` (for example `docker-compose.yml`) is lost on update.

### Status (`GET /api/settings/update-status`)

The panel returns one entry per node with `name`, `address`, `local`, `online`, `running`, `containerId`, `startedAt`, `finishedAt`, `exitCode`, `log` and `error`. For remote nodes it calls `GET /daemon/update-status`; a node that cannot be reached is returned with `online: false` and the reason in `error`.

## Using it

### In the panel

1. Open **Settings → Updates** (tab "Updates", card "System updates"). The check runs when the tab opens; **Check for updates** runs it again.
2. The card shows **Current** (panel version with the short deployed commit in parentheses) and **Latest** (short commit of the branch tip), plus a status line:
   - "System is up to date", "Update available" (with "The branch has N change(s) this panel does not have yet."), "New version being verified" (checks still running on GitHub), "Couldn't check for updates" or "Unable to verify the current version".
3. Click **Update panel** and confirm with **Update now**. The button is disabled while the system is up to date.
4. While the update runs, the page polls the status every 3 seconds and lists each node ("This panel" marks the local one) with "Updating…", "Completed", "Failed (exit N)" or "Unreachable", and a "View logs" control for the log tail.
5. When the panel itself restarts the page shows "Panel is restarting…" and keeps polling until it answers again. When every node finished without errors, the page reloads itself after a few seconds to load the new frontend.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `GET` | `/api/settings/update-check` | `settings.edit` | Compare the deployed commit with the branch tip on GitHub |
| `POST` | `/api/settings/update-panel` | `settings.edit` | Start the update on the panel host and on every node |
| `GET` | `/api/settings/update-status` | `settings.edit` | Update state, exit code and log tail of every node |
| `POST` | `/daemon/update` | panel JWT | Start the update on that node (called by the panel) |
| `GET` | `/daemon/update-status` | panel JWT | Update state of that node (called by the panel) |

`POST /api/settings/update-panel` answers `{"message": "Update initiated on the panel and all nodes.", "nodes": [...]}`, where a node that could not be triggered carries an `error`. See the [API reference](../reference/11-api-reference.md) for the full request and response formats.

## Requirements and configuration

- A Docker install made by the installer, with the repository at **`/opt/skypanel`** on every machine (panel and nodes). The script path and the install folder are fixed in the code.
- The Docker socket mounted in the panel/daemon container (`/var/run/docker.sock`, as in `docker-compose.yml`).
- `./.git` mounted at `/repo/.git` (read-only) for the check to know the current commit. Without it the check answers with an empty `current` and the UI shows "Unable to verify the current version".
- Outbound HTTPS from the panel to `api.github.com` (check) and from every machine to GitHub (git fetch) and to the image registries used by the build.
- No configuration keys control this feature. The branch comes from the repository or `/etc/SkyPanel/installer.conf`; `AETHER_BRANCH` is read by the script only if it is present in the environment where the script runs. See [Configuration](../backend/02-configuration.md) for the rest of the panel settings.

## Limits

- GitHub allows 60 anonymous API requests per hour per IP. Each check uses up to three (tip, compare, check runs). The automatic check in the app shell runs once per browser session.
- Only the state of the **latest** update of each process is kept, and only in memory: restarting the panel or daemon clears it.
- The update rebuilds the image on every machine, which takes several minutes and needs disk space for the build (the cache is pruned to 3 GB afterwards).
- Panel and servers may be unreachable for a few minutes during the restart.
- Bare-metal (non-Docker) installs are not covered: the trigger always uses Docker and `/opt/skypanel`.

## Troubleshooting

| Symptom / message | Cause | Fix |
|-------------------|-------|-----|
| "Couldn't check for updates" with the GitHub rate-limit text (`reason: github_rate_limited`) | GitHub answered 403/429: the 60 requests/hour quota for this IP is used | Wait and retry. Updating still works |
| "Couldn't check for updates" (`github_unreachable` / `github_error`) | No route to `api.github.com`, or an unexpected answer | Check outbound HTTPS/DNS from the panel container; see the panel log (`Update check failed to reach GitHub`) |
| "Unable to verify the current version" | `/repo/.git` is not mounted or has no readable HEAD | Restore the `./.git:/repo/.git:ro` volume in `docker-compose.yml` |
| "New version being verified" | GitHub checks on the new commit are still running | Wait for them to finish; the update is offered when they pass |
| Toast "Update incomplete: Could not trigger the update on: …" | A remote node did not answer `POST /daemon/update` or returned an error | Check that the node is online and reachable from the panel; update it by hand if needed |
| Node row "Failed (exit N)" | `panelUpdate.sh` failed on that machine | Open "View logs". Common lines: `No se encontró docker compose.`, `No se detectó el contenedor de skypanel en ejecución.`, `La rama X no existe en GitHub. No se actualiza nada.`, or a build error |
| `failed to connect to docker` / `failed to pull alpine:latest` / `failed to create update container` | The Docker socket is not mounted or the image cannot be pulled | Mount `/var/run/docker.sock`; check registry access |
| `git pull --ff-only` fails | Local commits on the host diverge from the branch | Resolve the repository in `/opt/skypanel` manually, then retry |

To update by hand on any machine, run the same script from the host: `cd /opt/skypanel && ./tools/panelUpdate/panelUpdate.sh`.

## See also

- [Core concepts: panel update propagation](../general/concepts.md#panel-update-propagation)
- [Installation](../setup/01-installation.md)
- [Configuration](../backend/02-configuration.md)
- [API reference](../reference/11-api-reference.md)
- [Dashboard and nodes](dashboard-and-nodes.md)
