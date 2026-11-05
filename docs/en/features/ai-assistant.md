# AI assistant and log analysis

Aether Panel can use Google Gemini to help server owners understand what is happening with their server. There are three entry points, all backed by the same Gemini API key that an administrator sets in the panel settings:

- **Server assistant**: a chat about one server that looks at its status, players, last day's performance, recent crashes and console before answering. With the file tools on (the default) it can also read and, with the user's approval, change that server's files: see [AI assistant: working with files](./ai-agent.md).
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

The server name and type are always included. This context reads no passwords, environment variables or files. In agent mode (below) the assistant can also read the server's files on demand, through its tools.

### Agent mode

When **Let the assistant work with server files** is on (`panel.settings.aiFileTools`, default `true`), the user is signed in and holds `server.files.view` on the server, the chat runs in **agent mode**: Gemini gets file tools for that server (list, read, search; with `server.files.edit` also create, copy, edit, overwrite, move, rename and delete). Reads and creations run at once; changes and deletions wait for the user's approval in a confirmation card. Each answer then lists the operations under it. Otherwise the chat answers from the context alone, as described on this page. The user guide is [AI assistant: working with files](./ai-agent.md); the architecture is in [AI agent toolkit](../backend/14-ai-toolkit.md).

## How it works

1. The browser calls the panel (`/api/servers/:serverId/ai/chat` or `/api/servers/:serverId/ai/analyze`). The request is checked for the `server.console` permission on that server.
2. The panel checks that a Gemini key is configured, validates the payload and applies the per-user rate limit.
3. For the chat, the panel gathers the context above from the node. For the analysis, it keeps the newest lines that fit the size budget.
4. The panel calls `https://generativelanguage.googleapis.com/v1beta/models/gemini-3.8-flash:generateContent` from the panel host. The key travels in the `x-goog-api-key` header, never in the URL.
5. The answer is returned to the browser. The chat history lives in the browser's `sessionStorage` (last 20 messages per server) and disappears when the tab session ends or when **New conversation** is pressed. The panel stores no conversation; in agent mode it only keeps a request waiting for approval in memory (10 minutes at most) and records the changes made in the activity log.

Prompt safety: console lines and the server context are untrusted text written by the game and its players. The panel wraps them in `<logs>…</logs>` or `<server-context>…</server-context>` and instructs the model to treat them as data only. The assistant is also told never to suggest deleting worlds or files without a backup first and never to ask for passwords or keys.

Model settings: the chat uses temperature 0.4 and at most 4096 output tokens, up to 512 of them thinking (8192 and 1024 in agent mode). The analysis requests a JSON response with the fixed schema `{ summary, suggestions[], rootCauses[] }`.

The AI requests themselves are not recorded in the server activity log. In agent mode, every change the assistant makes to the files is recorded as `ai.<tool>` (for example `ai.edit_file`), see [Activity log](./activity-and-access-log.md).

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
| `POST` | `/api/servers/:serverId/ai/chat/confirm` | `server.console` | Approve or decline the changes the assistant is waiting on (agent mode) |
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

In agent mode the answer also has `steps` (the operations run) and `pending` (the changes waiting for approval, or `null`). Both shapes and the confirmation route are in [Servers → AI](../reference/api/03-servers.md#post-apiserversserveridaichat).

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
| `panel.settings.aiFileTools` | `true` | `SKYPANEL_PANEL_SETTINGS_AIFILETOOLS` | Lets the assistant work with the files of the server it is opened on (agent mode), within the user's file permissions. Off, the assistant only answers questions. |

The three keys can be changed from **Settings → General** (`settings.edit`). See [Configuration](../backend/02-configuration.md) for how keys, the config file and environment variables relate.

Privacy: the console lines, server context and questions are sent to Google, and in agent mode also the content of the files the assistant reads. Tell your users if your terms require it.

## Limits

| Limit | Value |
|-------|-------|
| Rate limit | 30 AI requests per user every 10 minutes, chat, approvals and analysis counted together (in memory, reset when the panel restarts) |
| Chat history sent | Up to 20 messages, each up to 2000 characters |
| Analysis input | The newest 200 lines, further trimmed to 32 KiB |
| Console lines in the chat context | 60 (300 characters each) |
| Crash detail in the chat context | 3 newest crashes; 20 console lines of the newest |
| Timeouts | Analysis 60 s; chat 90 s overall, 6 s per context piece; in agent mode 120 s per question or approval |
| Model | `gemini-3.8-flash` (fixed in code, not configurable) |

Other things to know:

- **Hide AI Console Analysis** applies to every user (the web reads it from the public config, `GET /api/config`) and on the server too: `/ai/analyze` refuses the call. It does not affect the **Assistant** button, which stays available while a key is set.
- Requests made with an OAuth2 client (no user attached) share a single rate-limit bucket.

## Chat failures

When a question cannot be answered, the chat says so twice: a toast, and a red **What you asked for could not be done** card in the conversation that stays there. The card says what happened in plain words, lists the steps done before it stopped (agent mode), and notes when changes were made, which stay and are in the activity log. The last failed question has a **Try again** button that asks it once more; it is not shown when asking again cannot help (key, setting, permission, or the key out of quota).

Before giving up, the panel tries each Gemini call up to three times, 1 s and then 3 s apart, while Gemini is overloaded (5xx) or cannot be reached, and after a `429` that asks to wait 10 s or less.

| Card text (beginning) | `reason` | Cause | What to do |
|---|---|---|---|
| *The connection to the panel was lost…* / *There is no internet connection…* | none (no answer) | The browser lost its connection before the answer came | Check the connection and try again. |
| *The panel did not answer (code 502/503/504)…* | none | A proxy in front of the panel answered: the panel is restarting, updating or down | Wait and try again. |
| *Gemini rejected the API key…* | `keyRejected` | The key is invalid, revoked, restricted to other APIs or IPs, or the Generative Language API is not enabled | An administrator checks the key in Google AI Studio and in **Settings → General**. |
| *The Gemini key reached its usage limit…* | `quota` | Requests per minute or per day of the key's plan are used up (the free tier is low) | Wait, or raise the plan of the key. |
| *Gemini is overloaded or unavailable right now…* | `unavailable` | Gemini answered 5xx three times | Try again in a moment. |
| *The panel could not connect to Gemini…* | `unreachable` | The panel host cannot reach Google (firewall, DNS, proxy) | Allow outbound HTTPS to `generativelanguage.googleapis.com`. |
| *Gemini rejected the request…* | `rejected` | Gemini refused the request for another reason (for example an unknown model) | Read the panel log line `The AI service did not accept the assistant's request (…)`. |
| *The answer did not fit in Gemini's limit…* | `tooLong` | The answer (for example a large file to write) used all 8192 output tokens | Ask for the task in parts, one file at a time. |
| *Gemini refused to answer because of its safety filters…* | `blocked` | Google's safety filters stopped the answer | Rephrase the request. |
| *Gemini answered with nothing…*, *…could not be read*, *Gemini tried an operation that could not be read…* | `empty`, `unreadable`, `malformed` | The model gave no usable answer | Try again, or put it in other words. |
| *The assistant took too long and stopped…* | `timeout` | The task passed its time limit (120 s in agent mode, 90 s in plain chat) | Ask for the task in parts. |
| *The assistant failed with an internal error…* | `failed` | Something unexpected; the cause is in the panel log | Read the log line `The server assistant failed: …`. |

The panel log always has the exact cause of a Gemini refusal, with Google's status and message, for example `The AI service did not accept the assistant's request (status 429 RESOURCE_EXHAUSTED: Quota exceeded…)`. The key never appears in it.

## Troubleshooting

| Message | Cause | Fix |
|---------|-------|-----|
| `Gemini API Key is not configured. Please add it in Settings.` (UI: "The AI is not set up…") | `panel.settings.geminiApiKey` is empty | Add the key in **Settings → General → Artificial intelligence**. |
| `too many AI requests, try again in a few minutes` (HTTP 429; UI: "You asked a lot of questions in a row…") | More than 30 requests in 10 minutes by the same user | Wait; the window slides, so requests become available again as older ones age out. |
| `the AI log analysis is turned off in the panel settings` (HTTP 403, `ErrAIAnalysisDisabled`) | An administrator turned on **Hide AI Console Analysis** | Turn it off in **Settings → General → Artificial intelligence** if you want the analysis. |
| HTTP 403 (UI: "You need access to this server's console to use the assistant.") | The user lacks `server.console` on that server | Grant the console permission to the sub-user. |
| `a conversation has between 1 and 20 messages`, `each message has between 1 and 2000 characters`, `the last message must be the user's question`, `messages are from the user or the assistant` | Invalid chat payload (API clients) | Fix the request body. The panel UI already trims history to 20 messages. |
| `No logs provided for analysis` / `Invalid payload` | Empty or malformed analysis request | Send a non-empty `logs` array. |
| Chat: a red card *What you asked for could not be done* | See [Chat failures](#chat-failures) | Follow the card. |
| Log analysis: `Failed to connect to AI API` | The panel host cannot reach Google (firewall, DNS, proxy) | Allow outbound HTTPS to `generativelanguage.googleapis.com`. |
| Log analysis: `AI API returned status N` | Google refused the call: invalid or revoked key (400/403), quota exhausted (429), service error (5xx) | Check the key and its quota in Google AI Studio. |
| Log analysis: `AI returned invalid JSON structure`, `Empty response from AI` | The model produced no usable answer (for example, blocked by Google's safety filters) | Retry, or send fewer or different lines. |
| `This request expired. Ask again.` (HTTP 409, `ErrAIConfirmationInvalid`) | A change waiting for approval expired (10 minutes), was already answered, was replaced or the panel restarted | Ask again. See [AI assistant: working with files](./ai-agent.md#troubleshooting). |
| The answer says it has no information about something | The context piece was not collected (node offline, query disabled, no metrics yet) | Check "Looked at: …" under the answer to see which sources were available. |

## See also

- [AI assistant: working with files](./ai-agent.md) — agent mode, approvals and limits.
- [Crash reports](./crash-reports.md) — how crashes are detected and stored.
- [Scopes reference](../general/scopes-reference.md) — `server.console` and the other server permissions.
- [Configuration](../backend/02-configuration.md) — all configuration keys.
- [API reference](../reference/11-api-reference.md) — authentication and error format.
