# AI assistant and log analysis

Aether Panel can use Google Gemini to help server owners understand what is happening with their server. There are three entry points, all backed by the same Gemini API key that an administrator sets in the panel settings:

- **Server assistant**: a chat about one server that looks at its status, players, last day's performance, recent crashes and console before answering.
- **AI Console Analysis**: a card under the console that sends the error and warning lines you pick and returns a summary, likely root causes and suggestions.
- **Explain with AI** in a crash report: the same analysis applied to the console lines saved with a crash, together with its exit code.

The feature is optional. Without a Gemini key the panel works normally and the AI controls answer with a "not set up" message.

## What it does

| Entry point | Where | What is sent to Gemini | What comes back |
|-------------|-------|------------------------|-----------------|
| Server assistant | **Assistant** button in the server header | The conversation (up to 20 messages) plus a context block the panel builds from the server (see below) | A short Markdown answer and the list of sources the panel managed to collect |
| AI Console Analysis | **Console** tab, card below the terminal | The console lines you tick (error/warning lines detected among the last 50 lines shown) | `summary`, `rootCauses[]`, `suggestions[]` |
| Explain with AI | **Crash report** dialog (from the crash banner or the crash history on the **Overview** tab) | The console lines saved with the crash and its exit code | `summary`, `rootCauses[]`, `suggestions[]` |

Answers are written in the reader's interface language: Spanish when the UI is in Spanish, English otherwise.

### Context the assistant collects

For every chat message the panel asks the server's node for these pieces. Each one is best effort: a piece that fails or takes longer than 6 seconds is skipped and the rest are still sent. The names of the pieces that were collected are returned as `sources` and shown under the answer ("Looked at: …").

| Source | Daemon endpoint | Content added to the prompt |
|--------|-----------------|-----------------------------|
| `status` | `/daemon/server/:id/stats` | Online/offline, CPU %, memory used and its limit |
| `players` | `/daemon/server/:id/query` | Players online / maximum, version and map, per query protocol |
| `performance` | `/daemon/server/:id/metrics?range=24h` | Last 24 h: average and peak CPU, % of time online, average memory % of the limit, highest player count |
| `crashes` | `/daemon/server/:id/crashes` | Up to the 3 newest crashes (time, exit code, whether it was restarted automatically) and the last 20 console lines before the newest one |
| `console` | `/daemon/server/:id/console` | The last 60 non-empty console lines, ANSI colours removed, each cut at 300 characters |

The server name and type are always included. No passwords, environment variables or files are read.

## How it works

1. The browser calls the panel (`/api/servers/:serverId/ai/chat` or `/api/servers/:serverId/ai/analyze`). The request is checked for the `server.console` permission on that server.
2. The panel checks that a Gemini key is configured, validates the payload and applies the per-user rate limit.
3. For the chat, the panel gathers the context above from the node. For the analysis, it keeps the newest lines that fit the size budget.
4. The panel calls `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent` from the panel host. The key travels in the `x-goog-api-key` header, never in the URL.
5. The answer is returned to the browser. Nothing is stored on the server side: the chat history lives in the browser's `sessionStorage` (last 20 messages per server) and disappears when the tab session ends or when **New conversation** is pressed.

Prompt safety: console lines and the server context are untrusted text written by the game and its players. The panel wraps them in `<logs>…</logs>` or `<server-context>…</server-context>` and instructs the model to treat them as data only. The assistant is also told never to suggest deleting worlds or files without a backup first and never to ask for passwords or keys.

Model settings: the chat uses temperature 0.4 and at most 1200 output tokens. The analysis requests a JSON response with the fixed schema `{ summary, suggestions[], rootCauses[] }`.

The AI requests are not recorded in the server activity log (they change nothing on the server).

## Using it

### In the panel

**Administrator setup**

1. Get a Gemini API key from Google AI Studio.
2. Open **Settings → General**, section **Artificial intelligence**.
3. Paste it into **Gemini API Key**. The value is saved to the panel configuration file.
4. Optional: turn on **Hide AI Console Analysis** to turn the log analysis (the card under the console and **Explain with AI**) off for every user. The **Assistant** chat stays available.

**Server owners and sub-users**

- **Assistant**: press **Assistant** in the server header. Pick one of the suggested questions or type your own (up to 2000 characters). **Enter** sends, **Shift + Enter** adds a new line. **New conversation** clears the history.
- **AI Console Analysis**: on the **Console** tab, the card lists the error and warning lines detected in the console (lines containing `error`, `warn`, `fail`, `exception`, `timeout`, `denied` or `critical`). They are all ticked by default; untick what you do not want to send and press **Analyze Logs**.
- **Explain with AI**: open a crash report and press **Explain with AI**. The answer shows **Likely causes** and **How to fix it**.

The **Assistant** button is only shown to users who hold `server.console` on the server.

### Via the API

| Method | Path | Scope | Purpose |
|--------|------|-------|---------|
| `POST` | `/api/servers/:serverId/ai/chat` | `server.console` | Chat with the server assistant |
| `POST` | `/api/servers/:serverId/ai/analyze` | `server.console` | Analyse console lines (optionally from a crash). Answers `403` `ErrAIAnalysisDisabled` while **Hide AI Console Analysis** is on |

Chat request and response:

```json
// POST /api/servers/:serverId/ai/chat
{
  "language": "en",
  "messages": [
    { "role": "user", "content": "Why is my server slow?" }
  ]
}

// 200
{ "reply": "**CPU is the bottleneck** …", "sources": ["status", "performance", "console"] }
```

Rules: 1 to 20 messages; `role` is `user` or `assistant`; each `content` has 1 to 2000 characters; the last message must be from the user.

Analysis request and response:

```json
// POST /api/servers/:serverId/ai/analyze
{ "logs": ["[ERROR] ...", "[WARN] ..."], "language": "es", "exitCode": 137 }

// 200
{ "summary": "...", "rootCauses": ["..."], "suggestions": ["..."] }
```

`logs` is required; `exitCode` is optional and changes the prompt to describe a crash. See the [API reference](../reference/11-api-reference.md) for the common error format.

## Requirements and configuration

- Outbound HTTPS from the **panel** host to `generativelanguage.googleapis.com`. Nodes do not contact Google.
- A Google Gemini API key (billing on that Google account is the administrator's responsibility).

| Key | Default | Environment variable | Purpose |
|-----|---------|----------------------|---------|
| `panel.settings.geminiApiKey` | empty | `SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY` | Gemini API key. Empty disables every AI call. |
| `panel.settings.hideAiAnalysis` | `false` | `SKYPANEL_PANEL_SETTINGS_HIDEAIANALYSIS` | Turns the log analysis off for everyone: hides the **AI Console Analysis** card and **Explain with AI**, and `/ai/analyze` answers `403`. Published in `GET /api/config` as `hideAiAnalysis`. |

Both keys can be changed from **Settings → General** (`settings.edit`). See [Configuration](../backend/02-configuration.md) for how keys, the config file and environment variables relate.

Privacy: the console lines, server context and questions are sent to Google. Tell your users if your terms require it.

## Limits

| Limit | Value |
|-------|-------|
| Rate limit | 30 AI requests per user every 10 minutes, chat and analysis counted together (in memory, reset when the panel restarts) |
| Chat history sent | Up to 20 messages, each up to 2000 characters |
| Analysis input | The newest 200 lines, further trimmed to 32 KiB |
| Console lines in the chat context | 60 (300 characters each) |
| Crash detail in the chat context | 3 newest crashes; 20 console lines of the newest |
| Timeouts | Analysis 60 s; chat 90 s overall, 6 s per context piece |
| Model | `gemini-2.5-flash` (fixed in code, not configurable) |

Other things to know:

- **Hide AI Console Analysis** applies to every user (the web reads it from the public config, `GET /api/config`) and on the server too: `/ai/analyze` refuses the call. It does not affect the **Assistant** button, which stays available while a key is set.
- Requests made with an OAuth2 client (no user attached) share a single rate-limit bucket.

## Troubleshooting

| Message | Cause | Fix |
|---------|-------|-----|
| `Gemini API Key is not configured. Please add it in Settings.` (UI: "The AI is not set up…") | `panel.settings.geminiApiKey` is empty | Add the key in **Settings → General → Artificial intelligence**. |
| `too many AI requests, try again in a few minutes` (HTTP 429; UI: "You asked a lot of questions in a row…") | More than 30 requests in 10 minutes by the same user | Wait; the window slides, so requests become available again as older ones age out. |
| `the AI log analysis is turned off in the panel settings` (HTTP 403, `ErrAIAnalysisDisabled`) | An administrator turned on **Hide AI Console Analysis** | Turn it off in **Settings → General → Artificial intelligence** if you want the analysis. |
| HTTP 403 (UI: "You need access to this server's console to use the assistant.") | The user lacks `server.console` on that server | Grant the console permission to the sub-user. |
| `a conversation has between 1 and 20 messages`, `each message has between 1 and 2000 characters`, `the last message must be the user's question`, `messages are from the user or the assistant` | Invalid chat payload (API clients) | Fix the request body. The panel UI already trims history to 20 messages. |
| `No logs provided for analysis` / `Invalid payload` | Empty or malformed analysis request | Send a non-empty `logs` array. |
| `could not reach the AI service` / `Failed to connect to AI API` | The panel host cannot reach Google (firewall, DNS, proxy) | Allow outbound HTTPS to `generativelanguage.googleapis.com`. |
| `the AI service answered with status N` / `AI API returned status N` | Google refused the call: invalid or revoked key (400/403), quota exhausted (429), service error (5xx) | Check the key and its quota in Google AI Studio. |
| `the AI service sent an empty answer`, `AI returned invalid JSON structure`, `Empty response from AI` | The model produced no usable answer (for example, blocked by Google's safety filters) | Retry, or send fewer or different lines. |
| The answer says it has no information about something | The context piece was not collected (node offline, query disabled, no metrics yet) | Check "Looked at: …" under the answer to see which sources were available. |

## See also

- [Crash reports](./crash-reports.md) — how crashes are detected and stored.
- [Scopes reference](../general/scopes-reference.md) — `server.console` and the other server permissions.
- [Configuration](../backend/02-configuration.md) — all configuration keys.
- [API reference](../reference/11-api-reference.md) — authentication and error format.
