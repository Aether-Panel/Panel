# AI Integration (Genkit)

Aether Panel integrates Google Genkit for AI-powered log analysis and troubleshooting.

---

## Overview

- **Framework**: Google Genkit (TypeScript)
- **Model**: Google Gemini (configurable)
- **Features**: Log summarization, troubleshooting tips
- **UI**: "AI Analyze" button in server console view

---

## Configuration

```json
{
  "panel": {
    "settings": {
      "geminiApiKey": "YOUR_GEMINI_API_KEY"
    }
  }
}
```

**Environment Variable:**
```bash
SKYPANEL_PANEL_SETTINGS_GEMINIAPIKEY=your-api-key
```

**Get API Key:** [Google AI Studio](https://aistudio.google.com/apikey)

---

## Genkit Setup (`client/frontend/src/ai/genkit.ts`)

```typescript
import { genkit } from 'genkit';
import { googleAI } from '@genkit-ai/google-genai';

export const ai = genkit({
  plugins: [
    googleAI({
      apiKey: import.meta.env.VITE_GEMINI_API_KEY,
    }),
  ],
  model: googleAI.model('gemini-1.5-flash'),
});
```

---

## AI Flows (`client/frontend/src/ai/flows/`)

### 1. Summarize Server Alerts (`summarize-server-alerts.ts`)

**Input:**
```typescript
interface SummarizeAlertsInput {
  logs: string[];  // Recent server log lines
}
```

**Output:**
```typescript
interface SummarizeAlertsOutput {
  summary: string;           // 2-3 sentence summary
  rootCauses: string[];      // Identified root causes
  suggestions: string[];     // Actionable next steps
}
```

**Prompt Template:**
```
Analyze these server logs and provide:
1. A 2-3 sentence summary of what's happening
2. List of likely root causes
3. Actionable suggestions for resolution

Logs:
{{logs}}
```

**Use Case:** "AI Analyze" button in console view when errors detected

---

### 2. Generate Troubleshooting Tips (`generate-troubleshooting-tips.ts`)

**Input:**
```typescript
interface TroubleshootingInput {
  logs: string[];      // Error logs
  context: {           // Server context
    type: string;      // Server type (minecraft, etc.)
    version: string;   // Server version
    mods: string[];    // Installed mods/plugins
  };
}
```

**Output:**
```typescript
interface TroubleshootingOutput {
  tips: string[];      // Numbered troubleshooting steps
}
```

**Prompt Template:**
```
Given these error logs from a {{type}} server (version {{version}}) with mods: {{mods}}:
{{logs}}

Provide 5-7 specific, actionable troubleshooting steps ordered by likelihood.
Each step should be a single clear command or action.
```

---

## API Endpoint

```typescript
// POST /api/ai/analyze
// Body: { logs: string[] }
// Response: { summary, rootCauses, suggestions }

export async function POST(request: Request) {
  const { logs } = await request.json();
  const result = await summarizeServerAlerts({ logs });
  return Response.json(result);
}
```

---

## Frontend Integration

### Server Console View (`features/servers/[id]/console-view.tsx`)

```tsx
const { analyzeLogs } = useAIAnalysis();

const handleAIAnalyze = async () => {
  const recentLogs = getRecentLogs(100); // Last 100 lines
  const analysis = await analyzeLogs(recentLogs);
  setAIAnalysis(analysis);
};
```

**Button State:**
- Loading spinner during analysis
- Error toast on failure
- Results displayed in modal/drawer

### AI Analysis Modal

```tsx
<Dialog open={showAI} onOpenChange={setShowAI}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>AI Analysis</DialogTitle>
    </DialogHeader>
    <DialogDescription>
      <div className="space-y-4">
        <section>
          <h4>Summary</h4>
          <p>{analysis.summary}</p>
        </section>
        <section>
          <h4>Root Causes</h4>
          <ul>{analysis.rootCauses.map(c => <li key={c}>{c}</li>)}</ul>
        </section>
        <section>
          <h4>Suggestions</h4>
          <ol>{analysis.suggestions.map(s => <li key={s}>{s}</li>)}</ol>
        </section>
      </div>
    </DialogDescription>
  </DialogContent>
</Dialog>
```

---

## API Endpoints

| Method | Path | Scope | Description |
|--------|------|-------|-------------|
| POST | `/api/ai/analyze` | Authenticated | Analyze logs |
| POST | `/api/servers/:id/ai/analyze` | `server.console` | Server-specific analysis |

### Request
```json
{
  "logs": [
    "[10:30:15] [Server thread/ERROR]: Could not bind to port 25565",
    "[10:30:15] [Server thread/ERROR]: java.net.BindException: Address already in use"
  ]
}
```

### Response
```json
{
  "summary": "Server failed to start because port 25565 is already in use. This typically happens when a previous server instance didn't shut down properly.",
  "rootCauses": [
    "Previous server process still running",
    "Port not released after crash",
    "Another application using port 25565"
  ],
  "suggestions": [
    "Run 'killall java' to stop any remaining server processes",
    "Check for other applications using port 25565 with 'netstat -tulpn | grep 25565'",
    "Change server port in settings if port conflict persists",
    "Restart the server after ensuring port is free"
  ]
}
```

---

## Prompt Engineering

### Design Principles

1. **Specificity**: Prompts ask for exact format (numbered lists, specific sections)
2. **Context Awareness**: Includes server type, version, mods
3. **Actionable Output**: Each suggestion is a concrete command/action
4. **Safety**: No destructive commands without confirmation

### Prompt Versioning

Store prompts in version-controlled files:
```
ai/flows/
├── summarize-server-alerts.ts    # v1.2
├── generate-troubleshooting-tips.ts  # v1.0
```

---

## Error Handling

| Error | Cause | Handling |
|-------|-------|----------|
| `API_KEY_INVALID` | Invalid/missing API key | Show config error in UI |
| `QUOTA_EXCEEDED` | API quota exceeded | Show retry timer |
| `MODEL_OVERLOADED` | High demand | Retry with exponential backoff |
| `CONTENT_FILTERED` | Safety filter triggered | Fallback to generic response |

---

## Rate Limiting

- **Gemini Free Tier**: 60 RPM, 1500 RPD
- **Panel-side**: 10 requests/minute per user
- **Server-side**: Queue with 30s cooldown per server

---

## Cost Optimization

- Use `gemini-1.5-flash` (cheaper than Pro)
- Limit log input to 2000 chars
- Cache repeated analyses (1 hour TTL)
- Batch multiple log analyses

---

## Testing

### Unit Tests
```typescript
// ai/flows/summarize-server-alerts.test.ts
import { summarizeServerAlerts } from './summarize-server-alerts';

test('identifies port conflict', async () => {
  const result = await summarizeServerAlerts({
    logs: ['Address already in use: 25565']
  });
  expect(result.rootCauses).toContainEqual(
    expect.stringContaining('port')
  );
});
```

### Integration Test
```bash
# Test with real API
curl -X POST http://localhost:8080/api/ai/analyze \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"logs":["Address already in use"]}'
```

---

## Prompt Examples

### Minecraft Crash
**Input:**
```
[14:23:12] [Server thread/ERROR]: Encountered an unexpected exception
java.lang.OutOfMemoryError: Java heap space
```

**Output:**
```json
{
  "summary": "Server crashed due to Java heap space exhaustion. The allocated heap memory is insufficient for the current server load.",
  "rootCauses": [
    "Insufficient -Xmx heap allocation",
    "Memory leak in plugin/mod",
    "Too many entities/chunks loaded"
  ],
  "suggestions": [
    "Increase -Xmx in server settings (e.g., -Xmx4G)",
    "Check for memory leaks in recently added plugins",
    "Reduce view-distance and entity-activation-range in server.properties",
    "Use a memory profiler like VisualVM to identify leaks"
  ]
}
```

### Plugin Error
**Input:**
```
[15:42:01] [Server thread/ERROR]: Could not load 'plugins/EssentialsX.jar'
org.bukkit.plugin.InvalidPluginException: java.lang.UnsupportedClassVersionError
```

**Output:**
```json
{
  "summary": "EssentialsX plugin failed to load due to Java version mismatch. The plugin requires a newer Java version than currently installed.",
  "rootCauses": [
    "Java version too old for plugin",
    "Plugin compiled for Java 17+, running on Java 11"
  ],
  "suggestions": [
    "Upgrade to Java 17+ (install temurin-17-jdk)",
    "Update JAVA_HOME to point to Java 17",
    "Restart server after Java upgrade",
    "Check plugin compatibility matrix for required Java version"
  ]
}
```

---

## Monitoring

- **Latency**: Track AI response time (target < 5s)
- **Token Usage**: Monitor input/output tokens
- **Success Rate**: Track failed analyses
- **User Feedback**: Thumbs up/down on results

---

## Future Enhancements

- [ ] Multi-language support (Spanish prompts)
- [ ] Custom model fine-tuning
- [ ] Proactive alerts (AI detects patterns before crash)
- [ ] Integration with external knowledge bases
- [ ] Voice/narration of analysis