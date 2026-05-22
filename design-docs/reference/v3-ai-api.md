# Notion AI internal API — snapshot

> Captured 2026-02-18. Source: HAR trace from the Notion web client + third-party library analysis (PHP/Python/JS/Go wrappers around the legacy `getCompletion` endpoint).
>
> Notion AI is **not** in the official SDK or `api.notion.com/v1`. Everything here is the internal v3 surface, authenticated by `token_v2`.

## Two generations coexist

1. **Legacy `getCompletion`** — the original writing-assistant (summarize, translate, improve writing, …). Single-shot, prompt-typed, no agentic behaviour.
2. **Current `runInferenceTranscript`** — the newer agent/workflow system that powers Notion AI chat. Tools, streaming, feature flags, RAG.

The CLI's `ai chat` commands target the current system. The legacy system is documented here for completeness.

## Current: `runInferenceTranscript`

### Endpoint

```
POST https://www.notion.so/api/v3/runInferenceTranscript
Accept: application/x-ndjson
Content-Type: application/json
Cookie: token_v2={token}
x-notion-active-user-header: {userId}
x-notion-space-id: {spaceId}
```

### Request body

```json
{
  "traceId": "<uuid>",
  "spaceId": "<spaceId>",
  "transcript": [
    {
      "id": "<uuid>",
      "type": "config",
      "value": {
        "type": "workflow",
        "availableConnectors": ["gmail", "google-drive", "linear", "slack"],
        "searchScopes": [{ "type": "everything" }],
        "useWebSearch": true,
        "writerMode": false,
        "model": "<model-codename>",
        "enableAgentDiffs": true,
        "useServerUndo": true
      }
    },
    {
      "id": "<uuid>",
      "neverCompress": true,
      "type": "context",
      "value": {
        "timezone": "<iana-tz>",
        "userName": "<display-name>",
        "userId": "<userId>",
        "userEmail": "<email>",
        "spaceName": "<workspace-name>",
        "spaceId": "<spaceId>",
        "spaceViewId": "<spaceViewId>",
        "currentDatetime": "<iso8601>",
        "surface": "workflows",
        "blockId": "<page-blockId>",
        "visibleCollectionViewIds": {}
      }
    },
    {
      "id": "<uuid>",
      "type": "user",
      "value": [["the user's message"]],
      "userId": "<userId>",
      "createdAt": "<iso8601>"
    }
  ],
  "threadId": "<uuid>",
  "threadParentPointer": {
    "table": "space",
    "id": "<spaceId>",
    "spaceId": "<spaceId>"
  },
  "createThread": true,
  "generateTitle": true,
  "saveAllThreadOperations": true,
  "threadType": "workflow",
  "isPartialTranscript": false,
  "asPatchResponse": false,
  "debugOverrides": {
    "emitAgentSearchExtractedResults": true,
    "cachedInferences": {},
    "annotationInferences": {},
    "emitInferences": false
  }
}
```

### Response — NDJSON stream

`Content-Type: application/x-ndjson`. Each line is a JSON object with a `type` field. Observed types:

| Type | Description |
|---|---|
| `agent-turn-full-record-map` | Start/end of an agent turn; record version snapshot |
| `agent-tool-result` | Tool call results; `state` progresses "pending" → "applied" |
| `record-map` | Record map updates (blocks, threads, …) |
| `config` | Echoed config with server additions (e.g. resolved `model`) |
| `title` | Auto-generated thread title |
| `agent-inference` | **The model output tokens** — streamed cumulatively, not as deltas |

### `agent-inference` line shape

```json
{
  "id": "<uuid>",
  "type": "agent-inference",
  "value": [{ "type": "text", "content": "cumulative text so far" }],
  "traceId": "<uuid>",
  "startedAt": <ms>,
  "previousAttemptValues": []
}
```

`value[].content` is **cumulative**, not a delta — each line carries the entire response generated so far. The final `agent-inference` line additionally carries:

```json
{
  "finishedAt": <ms>,
  "inputTokens": <n>,
  "outputTokens": <n>,
  "cachedTokensRead": <n>,
  "maxContextTokens": <n>,
  "maxInputTokens": <n>,
  "model": "<model-codename>"
}
```

### Tool results

Tools are invoked automatically by the agent. Observed: `view`.

```json
{
  "type": "agent-tool-result",
  "toolName": "view",
  "toolType": "view",
  "input": {
    "urls": ["user://<userId>", "https://www.notion.so/<pageId>"],
    "fast_mode": true
  },
  "state": "applied",
  "result": {
    "entities": [{ "type": "page", "id": "<uuid>" }],
    "sanitizedPages": { "<pageId>": "<page xml content>" }
  }
}
```

`sanitizedPages` content uses a custom XML format to represent page content, discussions, and comments.

### Config feature flags (observed at capture time)

```
enableAgentAutomations, enableAgentIntegrations, enableCustomAgents,
enableExperimentalIntegrations, enableAgentViewNotificationsTool,
enableAgentDiffs, enableTurnLevelTitleDiff, enableAgentCreateDbTemplate,
enableCsvAttachmentSupport, enableDatabaseAgents, enableAgentThreadTools,
enableRunAgentTool, enableSetupModeTool, enableAgentDashboards,
enableAgentCardCustomization, enableSystemPromptAsPage,
enableUserSessionContext, enableScriptAgentAdvanced, enableScriptAgent,
enableScriptAgentIntegrations, enableScriptAgentCustomAgentTools,
enableAgentGenerateImage, enableSpeculativeSearch, enableQueryCalendar,
enableQueryMail, enableMailExplicitToolCalls, enableAgentVerification,
enableUpdatePageV2Tool, enableUpdatePageAutofixer,
enableUpdatePageMarkdownTree, enableUpdatePageOrderUpdates,
enableAgentSupportPropertyReorder
```

Flag set evolves quickly. Treat this as illustrative, not exhaustive.

## Supporting endpoints

### `getAvailableModels`

```
POST /api/v3/getAvailableModels
Body: { "spaceId": "<spaceId>" }
```

Response includes a list of models with food-themed codenames; the `modelMessage` is the human label. Codenames seen at capture time:

| Codename | Label | Family | Tier |
|---|---|---|---|
| `oatmeal-cookie` | GPT-5.2 | openai | fast |
| `vertex-gemini-2.5-flash` | Gemini 2.5 Flash | gemini | fast |
| `almond-croissant-low` | Sonnet 4.6 | anthropic | fast |
| `anthropic-sonnet-alt-thinking` | Sonnet 4.5 | anthropic | intelligent |
| `avocado-froyo-medium` | Opus 4.6 | anthropic | intelligent |
| `gateau-roule` | Gemini 3 Pro | gemini | intelligent |

Codenames rotate as Notion swaps backing models. Never hardcode them.

### `getAiPickableModels`

```
POST /api/v3/getAiPickableModels
Body: {}
```

Returns a flat array of model codenames the user is allowed to pick.

### `getChatTranscriptSessionHistoryForUser`

```
POST /api/v3/getChatTranscriptSessionHistoryForUser
Body: { "spaceId": "<spaceId>" }
```

Returns sessions from the **legacy** chat system with full step data. Record types: `assistant_chat_session`, `assistant_chat_step`. Each step carries `serializedXMLEvaluatorState`, `serializedTranscriptSteps`, `messages`, and `assistantOperations`/`operations`.

The legacy system's command vocabulary (preserved here because the records still exist in older workspaces):

```
load-database, load-page, load, query-database, search, search-people,
chat, create, search-databases, load-slack, insert-before, insert-after,
insert-inside, delete, set-title, set-attribute, set-tag-name,
set-property, replace, create-page
```

### `getInferenceTranscriptsForUser`

```
POST /api/v3/getInferenceTranscriptsForUser
Body: {
  "threadParentPointer": { "table": "space", "id": "<spaceId>", "spaceId": "<spaceId>" },
  "limit": 50
}
```

Recent "workflow" (current system) transcripts:

```json
{
  "transcripts": [
    {
      "id": "<uuid>",
      "title": "<auto-generated title>",
      "created_at": <ms>,
      "updated_at": <ms>,
      "created_by_display_name": "<display-name>",
      "type": "workflow"
    }
  ],
  "threadIds": ["<uuid>"],
  "unreadThreadIds": [],
  "hasMore": false
}
```

### `markInferenceTranscriptSeen`

```
POST /api/v3/markInferenceTranscriptSeen
Body: { "spaceId": "<spaceId>", "threadId": "<threadId>" }
Response: { "ok": true }
```

### `warmVectorDBCache` / `warmSearchCache`

```
POST /api/v3/warmVectorDBCache
POST /api/v3/warmSearchCache
Body: { "spaceId": "<spaceId>" }
Response: {}
```

Pre-warms the RAG vector database and search index for the workspace. Notion's UI calls these before AI interactions.

### `getExternalOrgData`

```
POST /api/v3/getExternalOrgData
Body: {}
```

Returns enriched company-data fields for the workspace (description, name, employee count, industry, etc.). Useful in the AI context bundle.

### `getBraintrustLogsForThread`

```
POST /api/v3/getBraintrustLogsForThread
Body: { "project": "production-workflow", "threadId": "<uuid>", "spaceId": "<spaceId>" }
```

Returned 500 in the capture — likely internal/admin only. Signals that Notion uses Braintrust for evaluation/observability.

## Legacy: `getCompletion`

The older writing-assistant endpoint, still backing several reverse-engineered third-party libraries.

### Endpoint

```
POST https://www.notion.so/api/v3/getCompletion
Cookie: token_v2={token}
Content-Type: application/json
Accept: application/json
```

### Request body

```json
{
  "id": "<uuid>",
  "model": "openai-3",
  "spaceId": "<spaceId>",
  "isSpacePermission": false,
  "context": {
    "type": "<prompt-type>",
    "selectedText": "...",
    "pageTitle": "...",
    "previousContent": "...",
    "restContent": "...",
    "prompt": "...",
    "language": "...",
    "tone": "...",
    "topic": "..."
  },
  "inferenceReason": "writer",
  "aiSessionId": "<uuid>",
  "metadata": { "blockId": "<uuid>" }
}
```

### Prompt types

| Type | Description |
|---|---|
| `helpMeWrite` | Generate content from prompt |
| `helpMeEdit` | Edit selected text per instructions |
| `helpMeDraft` | Draft content from prompt |
| `continueWriting` | Continue from existing content |
| `changeTone` | Rewrite in different tone |
| `summarize` | Summarize text |
| `improveWriting` | Improve writing quality |
| `fixSpellingGrammar` | Fix spelling and grammar |
| `translate` | Translate to target language |
| `explainThis` | Explain selected text |
| `makeLonger` | Expand text |
| `makeShorter` | Condense text |
| `findActionItems` | Extract action items |
| `simplifyLanguage` | Simplify complex text |

Topics for `helpMeWrite`: `brainstormIdeas`, `blogPost`, `outline`, `socialMediaPost`, `pressRelease`, `creativeStory`, `essay`, `poem`, `meetingAgenda`, `prosConsList`, `jobDescription`, `salesEmail`, `recruitingEmail`.

Tones for `changeTone`: `professional`, `casual`, `straightforward`, `confident`, `friendly`.

### Response

Streaming NDJSON. Each line has `type` and `completion`. Concatenate `completion` values where `type === "success"`.

## Notes that aged worth keeping

- The `ai_block` block type exists in Notion's internal format but the official API rejects it as `"Unsupported block type: ai_block"`.
- Conversations are stored as `thread` records with `thread_message` children (current system) or `assistant_chat_session`/`assistant_chat_step` (legacy).
- Workspace must have an active Notion AI subscription for these endpoints to return anything useful.

## Third-party libraries (legacy `getCompletion` only)

| Library | Language |
|---|---|
| [albertcht/notion-ai](https://github.com/albertcht/notion-ai) | PHP |
| [Vaayne/notionai-py](https://github.com/Vaayne/notionai-py) | Python |
| [HCYT/notionAI](https://github.com/HCYT/notionAI) | JS/TS |
| [jyz0309/NotionAI-go](https://github.com/jyz0309/NotionAI-go) | Go |

None of these implemented `runInferenceTranscript` at the time of capture.
