# AI assistant: working with files

The [server assistant](./ai-assistant.md) can do more than answer questions: it can look at the server's files and, with your approval, change them. Ask it in plain words ("create a `config.json` with…", "rename the server in the server list", "move the config files into a folder") and it lists folders, reads files, searches, creates, edits, copies, moves, renames and deletes for you, always inside the server you opened it on.

Anything that changes or removes existing data waits for you: the assistant shows what it wants to do and nothing happens until you press **Approve**.

How it is built (tools, function calling, isolation, confirmations) is described in [AI agent toolkit](../backend/14-ai-toolkit.md).

## What it does

| You ask for… | What the assistant uses | Needs your approval |
|---|---|---|
| Seeing what is there: list a folder, read a file, check a file's size or date, find files by name or by a word inside them | `list_directory`, `read_file`, `file_info`, `search_files` | No |
| Something new: create a text file, create a folder, copy a file or folder (for example a backup copy before a change) | `create_file`, `create_directory`, `copy_path` | No: these never replace anything that exists |
| Changing what exists: edit part of a file, replace a whole file, move or rename | `edit_file`, `write_file`, `move_path`, `rename_path` | Yes |
| Deleting a file, or a folder with everything in it | `delete_path` | Yes |

The assistant still gets the usual server context (status, players, performance, crashes and console, see [AI assistant](./ai-assistant.md#context-the-assistant-collects)), so it can combine what the console says with what is in the files.

What it **cannot** do: run commands (shell or console), start, stop or restart the server, install anything, reach another server's files or anything outside this server's folder, delete the server folder itself, read binary files (jars, worlds, images) or files bigger than 1 MiB, or change panel settings and permissions.

### Examples

**Create a file.** *"Create `plugins/MyPlugin/config.json` with `{"enabled": true}`."* The assistant creates the file (and the missing folders) straight away, because creating never replaces anything; if the file already exists, it says so instead. Under its answer: *Created plugins/MyPlugin/config.json (17 bytes)* — **Done**.

**Change a setting.** *"Change the name players see in the server list to Survival Island."* The assistant reads `server.properties`, finds the `motd=` line and proposes *Replace 1 occurrence in server.properties*. The card appears; press **Approve** and the line is changed. The previous text is kept in the file's [version history](./file-history.md), so it can be compared and restored. The game reads `server.properties` at start-up: restart the server yourself to apply it.

**Organise files.** *"Put all the `.yml` files in the server folder into a `configs` folder."* The assistant lists the folder, creates `configs` (no approval needed) and proposes one move per file. Each move has its own checkbox: untick the files you want to leave where they are and press **Approve**. With many files it may work in several turns and ask again.

**Diagnose.** *"Why does my server stop right after starting?"* The assistant looks at the console and crashes it already has, then reads or searches the files it needs (for example `logs/latest.log`, a plugin's configuration, or a search for `ERROR` in the `logs` folder). Reading needs no approval, so you get the explanation directly. If it suggests a fix to a file, it asks before applying it.

## How it works

1. You ask a question in the assistant (**Assistant** button in the server header).
2. The panel sends the conversation and the server context to Google Gemini together with the list of file tools your permissions allow. Gemini answers with text or with requests to use tools.
3. The panel runs the tools that only read or create, on the server's node, and gives the results back to Gemini. This can go back and forth several times within one question.
4. When Gemini wants to change or delete something, the panel stops and shows you the confirmation card.
5. When you answer, the panel runs what you approved (checking it again first) and Gemini continues until it has a final answer.

Every path is relative to the server folder: the assistant has no way to name another server or a folder of the machine.

### What the answer shows

Under each answer a compact list, **What the assistant did**, shows every operation: an icon for its kind (read, create, change, delete), a short sentence such as *Read server.properties (1.2 KB)* or *Moved paper.yml to configs/paper.yml*, the paths involved, and a status:

| Status | Meaning |
|---|---|
| **Done** | It ran |
| **Failed** | It was tried and failed; the reason is shown below it (for example the file changed, or does not exist) |
| **Waiting** | It is in the confirmation card |
| **Declined** | You did not approve it, so it did not run |
| **Not allowed** | Your permissions do not allow it, or it went over a limit; it did not run |

Each step is shown in the interface language ("Read server.properties", "Move paper.yml to config/paper.yml"); in English the panel shows the more detailed sentence the tool wrote (for example the size read or the number of replacements). Error messages of failed steps are in English. The assistant's own answer follows your language.

### The confirmation card

When the assistant wants to change or delete something, a card titled **The assistant wants to:** appears under its answer:

- One row per operation, with its sentence and paths. Deletions are marked in red with **Deletes or replaces data**.
- Every row starts ticked. Untick what you do not want.
- **Approve** runs the ticked operations and declines the rest (it is disabled when nothing is ticked). **Decline all** runs none of them.
- Until you answer, the question box is locked (*Approve or decline the changes above first*). The card stays if you switch tabs or reload the page in the same browser session.
- Each operation is checked again just before it runs: if the file changed in the meantime and the change no longer fits, that step fails instead of doing something different from what you saw.
- After your answer the assistant continues: it explains the result, and may propose more changes in a new card.

A request is valid for **10 minutes**, can be answered **once**, and only by **you** on **this server**. Asking something else that needs approval on the same server replaces the previous request. **New conversation** removes the card from your browser.

### Where the changes are recorded

- **Activity log.** Every change the assistant made is recorded in the server's [activity log](./activity-and-access-log.md) under your name, with the path (or `source → destination`), for example *used the assistant to edit the file `server.properties`*. They are in the **Files** filter. Reads are not recorded.
- **File history.** Edits and overwrites keep the previous text in the file's [version history](./file-history.md) under the usual rules (configuration and text files up to 1 MiB). New files, copies, moves and deletions have no version to go back to.
- Deletions cannot be undone from the panel: ask the assistant to copy the file or folder first, or take a [backup](./backups.md).

## Using it

### In the panel

**Administrators**

1. Set the Gemini API key in **Settings → General → Artificial intelligence** (see [AI assistant](./ai-assistant.md#in-the-panel)).
2. **Let the assistant work with server files** is on by default in the same section. Turn it off to keep the assistant to answering questions only.

**Server owners and sub-users**

- Open **Assistant** in the server header and ask in your own words. Name files with their path from the server folder when you know it (`config/paper-global.yml`), or ask the assistant to look for them.
- Read the confirmation card before approving: it lists exactly the files that will change.

### Via the API

Agent mode uses the same `POST /api/servers/:serverId/ai/chat` as the plain chat; its answer adds `steps` and `pending`, and `POST /api/servers/:serverId/ai/chat/confirm` answers a pending request. See [Servers → AI](../reference/api/03-servers.md#post-apiserversserveridaichat).

## Requirements and configuration

| Requirement | Why |
|---|---|
| A Gemini API key (`panel.settings.geminiApiKey`) | Without it the assistant answers "not set up" |
| **Let the assistant work with server files** on (`panel.settings.aiFileTools`, default `true`) | Off, the assistant only answers questions |
| `server.console` on the server | To open the assistant at all |
| `server.files.view` on the server | To let it look at files; without it the assistant only answers questions |
| `server.files.edit` on the server | To let it create, change, move and delete; without it, it can only look |
| Nodes updated to the panel's version | The assistant uses node routes added with this feature |

| Key | Default | Environment variable | Purpose |
|---|---|---|---|
| `panel.settings.aiFileTools` | `true` | `SKYPANEL_PANEL_SETTINGS_AIFILETOOLS` | Lets the assistant work with the files of the server it is opened on, within the user's file permissions |

Permissions are given to each sub-user in the server's **Users** tab; see [Sub-users](../general/scopes-reference.md#sub-users). The assistant uses the permissions of whoever is chatting: it can never do more than that person could do in the File Manager.

## Privacy

When the assistant uses its file tools, **what it reads is sent to Google Gemini**: the content of the files it reads, the names in the folders it lists, the lines its searches find, and the paths involved, in addition to the conversation and the server context. This only happens when an administrator has set a Gemini key, the setting is on, and someone asks the assistant something. Configuration files can hold passwords or tokens (database credentials, RCON passwords): the assistant reads what it decides it needs, so do not ask it to work with files you do not want to share with Google, or turn the setting off. See the [privacy notice](../general/privacy.md#external-services-the-panel-contacts).

The panel keeps nothing else apart from the activity entries and file versions above (and, at debug log level, the paths the assistant read); a request waiting for approval is kept in the panel's memory for up to 10 minutes.

## Limits

| Limit | Value |
|---|---|
| Files it can read or edit | Text files up to 1 MiB; binary files are refused |
| Content it can write | Up to 1 MiB per file in theory; in practice much less, because each Gemini answer is limited to 8192 output tokens |
| Folder listing | 1000 entries |
| Search | 8 folders deep, 5000 entries looked at, 200 matches, files up to 1 MiB, queries up to 200 characters |
| One question | At most 10 rounds with Gemini, 8 operations per round, 30 operations in total, 120 seconds |
| A request waiting for approval | 10 minutes; one per user and server; lost if the panel restarts |
| Rate limit | 30 AI requests per user every 10 minutes (chat, approvals and log analysis together); one question or one approval counts once |

When a question needs more than these limits, the assistant stops and says what it did so far.

Each new question sends the previous messages as text only: the assistant does not remember the content of files it read in earlier answers and reads them again when needed.

## Troubleshooting

| Message or symptom | Cause | Fix |
|---|---|---|
| *This request expired. Ask again.* (HTTP 409, `ErrAIConfirmationInvalid`) | The request is older than 10 minutes, was already answered, was replaced by a newer one on the same server, the panel restarted, or the file tools were turned off meanwhile | Ask the question again. |
| *You need access to this server's console to use the assistant.* (HTTP 403) | You lack `server.console`; on an approval, you no longer have `server.files.view` on the server | Ask the owner for the permission. |
| The assistant says it cannot see or change files; no step list appears | **Let the assistant work with server files** is off, or you lack `server.files.view` | Turn the setting on (administrators) or ask for the permission. |
| It can read but says it cannot change anything, or steps show **Not allowed** | You lack `server.files.edit` | Ask the owner for the permission. |
| *The AI is not set up…* (HTTP 400) | No Gemini key | An administrator adds it in **Settings → General → Artificial intelligence**. |
| Steps fail with *not found* for files that exist, or searches and copies fail | The server's node runs an older version without the assistant's file routes (they answer 404) | Update the node. |
| *could not reach the node that hosts this server* | The node is offline or unreachable from the panel | Check the node on the **Nodes** page. |
| *the node that hosts this server refused the panel's credentials…* | The node's secret does not match the panel, or a clock is off | See [Node authentication failures](../reference/api/01-conventions.md#node-authentication-failures). |
| A step fails with *old_text was not found* or *appears N times* | The file changed, or the text to replace is not unique | Ask again; the assistant reads the file again first. |
| *too large* or *is a binary file* | The file is over 1 MiB, or not text | Edit it in the File Manager or over SFTP. |
| *the server has used all of its disk space…* | The server's disk limit is reached | Free space, then ask again. |
| A red **What you asked for could not be done** card (and a toast) | The task stopped; the card says why: connection to the panel lost, the panel not answering, Gemini rejecting the key, its usage limit, Gemini overloaded (after 3 attempts), an answer too long, a timeout… See [the chat failures](./ai-assistant.md#chat-failures) | Follow what the card says. **Try again** asks the same question once more; it is not offered when only an administrator can fix the cause (key, setting, permission). Changes made before the failure stay, are listed under the card and are in the activity log. |

## See also

- [AI assistant and log analysis](./ai-assistant.md): the chat, its context, the key and the rate limit.
- [AI agent toolkit](../backend/14-ai-toolkit.md): architecture, tools, isolation and confirmations.
- [File history](./file-history.md): comparing and restoring edited files.
- [Activity log](./activity-and-access-log.md): the `ai.*` entries.
- [Panel settings](./panel-settings.md): the **Artificial intelligence** section.
