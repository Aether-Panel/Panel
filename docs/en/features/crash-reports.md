# Crash detection and crash reports

When a game server stops by itself with an unexpected exit code, Aether Panel treats it as a crash: it saves a report with the last console lines, warns whoever has the server open, sends the configured notifications and, if enabled, restarts the server automatically up to a limit. The report can be opened later and explained with the AI.

## What it does

- Detects crashes on the node as soon as the server process exits.
- Prints `Server crashed (exit code <n>)` in the console.
- Saves a **crash report**: time, exit code, the last 60 non-empty console lines, whether the server was restarted automatically, and which attempt in a row it was.
- Keeps the **last 10 reports** per server, newest first.
- Pushes a `crash` event over the server's WebSocket, so the open server page shows a toast at once.
- Sends the crash to the server's own notification channels, when the **crash** event is enabled there (see [Notifications](server-notifications.md)), and to the panel-wide Discord webhook (see [Discord](../general/16-discord.md)). Both are written in the notification language (`panel.settings.notificationLanguage`, see [Panel settings](panel-settings.md)) of the node that runs the server.
- Optionally **restarts the server** automatically (**Restart on Crash**), with a limit on consecutive restarts.

## How it works

### What counts as a crash

When the server process exits, the node compares the exit code with the server's expected exit code (`expectedExitCode` in the server definition, `0` when not set):

| Situation | Result |
|---|---|
| Someone stopped or killed the server (panel button, API, a task's `stop` step) | Not a crash, whatever the exit code. |
| The process exits with the expected code on its own (for example `stop` typed in the console) | Graceful stop. Not a crash. If **Restart on Stop** (`autoRestartOnGraceful`) is on, the server is started again. |
| The process exits with any other code without being asked | **Crash.** |

### Automatic restart

- Controlled by the server flag **Restart on Crash** (`autoRestartOnCrash` in the API, stored as `autorecover` in the server definition). It is off by default unless the server's template turns it on.
- After a crash, the node restarts the server only while the crash counter is below `daemon.data.crashLimit` (default **3**). Each automatic restart increases the counter.
- The counter goes back to zero when the server exits without crashing, or when it had been running for **more than 10 minutes** before the crash. Crashes a week apart therefore do not use up the automatic restarts.
- The restart happens after the server's post-execution steps. If those steps fail, the server is not restarted.
- When the limit is reached, the server stays stopped and the report says it was not restarted.

### Where reports are stored

Reports are saved on the node in `<daemon.data.servers>/<serverId>.crashes` (JSON, mode `0600`, because console output can contain player names and IPs). Each line is cut at 500 characters. Only the newest 10 reports are kept.

## Using it

### In the panel

- **Toast.** While the server page is open, a crash shows a toast "The server crashed (code `<n>`)." with what the exit code usually means and whether the server was restarted. For an automatic restart, the toast reads the saved report to show the real attempt number and limit (the same ones as the dialog); if it cannot read it (for example without `server.console`), it just says "Restarted".
- **Banner.** On the **Console** tab, the latest crash appears as a banner ("The server crashed at `<time>`") for 24 hours, with **View report** and **Dismiss**. Dismissing it is remembered in that browser.
- **Crash history.** The **Overview** tab has a **Crash history** card with the latest reports. The performance history chart in the same tab marks the crashes on its timeline.
- **Crash report** dialog: **Exit code**, **What it means**, **Recovery** (restarted by itself, attempt N of M, or not restarted) and the **Last console lines**, with error-looking lines highlighted.
- **Explain with AI.** In the dialog, the **AI explanation** section sends the report's lines and exit code to the AI and shows a summary, likely causes and **How to fix it**. It requires the Gemini key (see below) and is not shown when an administrator turns on **Hide AI Console Analysis**. See also the [AI assistant](ai-assistant.md).
- **Turning on automatic restart.** **Settings** tab → **Restart on Crash** ("Restart the server when it crashes."). Requires `server.flags.edit`.

The panel shows these meanings for common exit codes:

| Code | Meaning shown |
|---|---|
| `1` | The game exited with an error |
| `134` | The process aborted (SIGABRT) |
| `137` | Out of memory or force-killed (SIGKILL) |
| `139` | The program crashed on memory access (segfault) |
| `143` | Terminated by the system (SIGTERM) |
| other | Unexpected exit |

### Via the API

Full details are in the [API reference](../reference/11-api-reference.md).

| Method | Path | Scope | Purpose |
|---|---|---|---|
| `GET` | `/api/servers/:serverId/crashes` | `server.console` | The saved reports, newest first. |
| `POST` | `/api/servers/:serverId/ai/analyze` | `server.console` | Explain log lines with the AI (used by **Explain with AI**). |
| `GET` | `/api/servers/:serverId/flags` | `server.flags.view` | Read `autoStart`, `autoRestartOnCrash`, `autoRestartOnGraceful`. |
| `POST` | `/api/servers/:serverId/flags` | `server.flags.edit` | Change those flags, for example `{"autoRestartOnCrash": true}`. |

A report as returned by `GET /crashes`:

```json
{
  "id": "20261030T182241.123456789",
  "time": "2026-10-30T18:22:41.123456789Z",
  "exitCode": 137,
  "lines": ["[18:22:39] [Server thread/INFO]: ...", "..."],
  "restarted": true,
  "attempt": 1,
  "limit": 3
}
```

The WebSocket event pushed to `/api/servers/:serverId/socket` has `"type": "crash"` and the data `{"id", "exitCode", "restarted"}`.

## Requirements and configuration

| Key | Default | Env variable | Purpose |
|---|---|---|---|
| `daemon.data.crashLimit` | `3` | `SKYPANEL_DAEMON_DATA_CRASHLIMIT` | Maximum automatic restarts in a row after crashes. |
| `daemon.data.servers` | `servers` (under the data root) | `SKYPANEL_DAEMON_DATA_SERVERS` | Folder where the `.crashes` files are kept. |
| `panel.settings.geminiApiKey` | empty | `SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY` | Google Gemini key needed by **Explain with AI**. |

Crash detection itself is always on. See [Configuration](../backend/02-configuration.md) for how keys and environment variables are set.

## Limits

- 10 reports per server; 60 console lines per report; 500 characters per line.
- A report only holds what was still in the console buffer when the server died. If the game wrote its crash details to a file (for example a Minecraft `crash-reports/` file) and not to the console, open that file in the file manager.
- The automatic restart counter is kept in memory on the node; restarting the daemon resets it.
- The banner's dismissal is stored in the browser's local storage, so it reappears in another browser or device.

## Troubleshooting

| Symptom or message | Cause and fix |
|---|---|
| The server keeps crashing and then stays off | It reached `daemon.data.crashLimit` automatic restarts in under 10 minutes each. Read the report, fix the cause, then start the server. |
| Exit code `137` | The container ran out of memory or was killed. Raise the server's memory or reduce what it loads (mods, view distance). |
| A stop from the panel is not reported | Expected: requested stops and kills are never crashes. |
| A server that exits with a non-zero code on a normal stop is reported as crashed | Set `expectedExitCode` in the server definition to the code the game uses on a clean exit. |
| **Crash history** is empty for some users | `GET /crashes` requires `server.console`; users without it see no reports. |
| "The AI is not set up. An administrator can add the Gemini key in Settings." | `panel.settings.geminiApiKey` is empty. The API returns `Gemini API Key is not configured. Please add it in Settings.` |
| "Could not get an explanation. Try again in a moment." | The call to Gemini failed (network, quota or timeout). Try again later. |
| Automatic restart did not happen although it is enabled | The limit was reached, or the server's post-execution steps failed (the console shows `Failed to run post-execution steps`). |

## See also

- [Notifications](server-notifications.md)
- [AI assistant](ai-assistant.md)
- [Discord](../general/16-discord.md)
- [Servers: lifecycle](../backend/04-servers.md)
- [API reference](../reference/11-api-reference.md)
- [Configuration](../backend/02-configuration.md)
