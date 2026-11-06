# Ajustes repartidos a los nodos — Implementation Plan

**Goal:** the panel pushes its node-level settings (S3 off-site backups and the CurseForge key) to every remote node automatically, so no node needs manual config.

**Spec:** docs/superpowers/specs/2026-11-06-node-settings-sync-design.md (binding).

**Tech:** Go (gin, viper-backed config entries, testify), React/Astro + TS, i18n JSON.

## Global constraints
- Synced keys: every `daemon.backups.s3.*` entry (enabled, endpoint, region, bucket, accessKey, secretKey, prefix, insecure, periodicHours, periodicKeep) and `daemon.curseforge.key` — one list in internal/config used by both sides.
- Node routes `GET`/`PUT /daemon/settings` behind `middleware.ValidateJWT` (panel-signed, node audience). GET returns only `{hash}` (sha256 hex of canonical JSON of current values); never values. PUT validates everything first (unknown key → 400, invalid periodic/keep → 400), then applies with `Set(value, true)`; 204.
- Panel: skip LocalNode (id 0). States per node: synced / pending / outdated (404) / failed (node 4xx). Push on settings save (async, never blocks the save response) and every 10 min (first pass 1 min after start); one pass at a time, a request during a pass schedules one more; 10 s per node.
- `GET /api/settings/node-sync` (same permission as reading panel settings) → `[{id,name,state,error?,checkedAt}]`, no setting values.
- Secrets never logged. Comments in English explaining why. UI texts es/en, locale files keep no trailing newline, responsive.
- Test with scratchpad `gotest.sh` / `verify.sh go|fe` (no local Go).

## Task 1 — Node: the synced-key list and `/daemon/settings`
Files: internal/config (list + helpers: `SyncedNodeSettings` list of entries, `NodeSettingsValues() map[string]interface{}`, `NodeSettingsHash(map) string` canonical sha256), internal/web/daemon/root.go (routes + handlers + swagger), tests (internal/config, internal/web/daemon).
Tests: hash stable regardless of map order and changes when any value changes; GET returns only hash; PUT with all keys applies them (values readable after) and returns 204; PUT with an unknown key → 400 and nothing applied; PUT with periodicHours 25 or periodicKeep 0 → 400 and nothing applied; request without/with wrong-audience token → 401 (reuse existing ValidateJWT test helpers if any).
Commit: `feat(nodes): a node takes the panel's node settings from the panel`.

## Task 2 — Panel: push the settings to remote nodes
Files: internal/services/node_settings.go (+test), internal/web/api/settings.go (trigger after saving a synced key; new GET /api/settings/node-sync route), cmd/panel/run.go (start the worker on the panel process).
Tests (fake node HTTP server like existing node tests): hash equal → no PUT, state synced; hash differs → PUT with full payload, synced; 404 → outdated; 400 → failed with message; connection refused → pending; LocalNode skipped; saving a synced key triggers a pass without waiting for it; a non-synced key does not; concurrent triggers don't overlap passes; node-sync endpoint never contains setting values.
Commit: `feat(nodes): the panel keeps every node's settings in line with its own`.

## Task 3 — UI and docs
Files: client/frontend/src/features/settings/offsite-tab.tsx (replace the remote-nodes notice with the per-node status list; one line explaining S3 + CurseForge are sent to every node automatically), locales es/en, docs es/en per the spec.
Verify `verify.sh fe`. Commit: `feat(frontend): settings show which nodes are up to date` and `docs: node settings reach every node by themselves`.

## Task 4 — Updates drop a stale token override by themselves
File: tools/panelUpdate/panelUpdate.sh (~lines 58-106). Today a node's update re-applies an active `SKYPANEL_TOKEN_PUBLIC=` line from docker-compose.yml; that env var beats `token.public` in config.json, so a node whose line points at an old panel address keeps refusing the panel after every update (it happened to the user's node «Nodonegro»).
Change: when the node's config.json (storage/skypanel-data/config.json; read with jq if present, else a grep that tolerates formatting) already has a non-empty `token.public`, do NOT re-apply the saved override — leave the line commented — and log one line saying so. If config.json lacks `token.public` (nodes set up by hand long ago), keep today's behaviour. Same rule as install.sh 4.2.3's `enable_node_compose`.
Verify on the VPS with a scratch copy of a compose file + config.json (both cases) — never on /opt/skypanel. shellcheck clean (no new warnings).
Commit: `fix(update): a node update stops re-applying a token override its config already covers`.
