/**
 * Notion AI client — methods for interacting with Notion's AI endpoints.
 * All methods call V3HttpClient directly (AI is v3-only, no official API equivalent).
 *
 * Reference: design-docs/notion-ai-api.md
 */
import type { V3HttpClient } from "./client.ts";
import type {
  AiModel,
  InferenceTranscript,
  NdjsonEvent,
  RunInferenceTranscriptRequest,
  TranscriptConfigItem,
  TranscriptContextItem,
  TranscriptUserItem,
} from "./ai-types.ts";
import { parseNdjson } from "./ndjson.ts";

// --- Model listing ---

export async function getAvailableModels(
  client: V3HttpClient,
  spaceId: string,
): Promise<AiModel[]> {
  const response = await client.getAvailableModels(spaceId);
  return response.models;
}

// --- Transcript listing ---

export async function getInferenceTranscripts(
  client: V3HttpClient,
  spaceId: string,
  limit?: number,
): Promise<{
  transcripts: InferenceTranscript[];
  unreadThreadIds: string[];
  hasMore: boolean;
}> {
  const response = await client.getInferenceTranscriptsForUser({
    spaceId,
    limit,
  });
  return {
    transcripts: response.transcripts,
    unreadThreadIds: response.unreadThreadIds,
    hasMore: response.hasMore,
  };
}

// --- Mark transcript seen ---

export async function markTranscriptSeen(
  client: V3HttpClient,
  spaceId: string,
  threadId: string,
): Promise<{ ok: boolean }> {
  return client.markInferenceTranscriptSeen({ spaceId, threadId });
}

// --- Thread content retrieval ---

export type ThreadMessage = {
  id: string;
  role: string;
  content: string;
  createdAt?: number;
};

/**
 * Fetch the content of an AI chat thread by ID.
 * Tries multiple record table names since Notion's internal schema isn't documented.
 * Returns the raw recordMap for debugging if no messages are parseable.
 */
export async function getThreadContent(
  client: V3HttpClient,
  threadId: string,
  spaceId: string,
): Promise<{
  messages: ThreadMessage[];
  title?: string;
  raw?: Record<string, unknown>;
}> {
  // Try table names one at a time since invalid table names cause 400 errors
  const candidateTables = [
    "inference_transcript",
    "thread",
  ];

  let threadRecord: Record<string, unknown> | undefined;
  let foundTable: string | undefined;
  const errors: string[] = [];

  for (const table of candidateTables) {
    try {
      const result = await client.syncRecordValuesForPointers([
        { id: threadId, table, spaceId },
      ]);
      const tableData = result.recordMap[table];
      if (tableData?.[threadId]?.value) {
        threadRecord = tableData[threadId].value as Record<string, unknown>;
        foundTable = table;
        break;
      }
    } catch (err: unknown) {
      errors.push(`${table}: ${(err as Error).message}`);
    }
  }

  if (!threadRecord) {
    return {
      messages: [],
      raw: {
        _note: "No record found. Table probing results:",
        _errors: errors,
      },
    };
  }

  const title = threadRecord.title as string | undefined;
  const messageIds = (threadRecord.content as string[] | undefined) ?? [];

  if (messageIds.length === 0) {
    return {
      messages: [],
      title,
      raw: { _foundTable: foundTable, record: threadRecord },
    };
  }

  // Fetch messages using the same table pattern (thread → thread_message)
  const msgTable = foundTable === "thread" ? "thread_message" : `${foundTable}_message`;

  let msgRecordMap: Record<string, unknown> = {};
  try {
    const msgResult = await client.syncRecordValuesForPointers(
      messageIds.map((id) => ({ id, table: msgTable, spaceId })),
    );
    msgRecordMap = msgResult.recordMap as unknown as Record<string, unknown>;
  } catch {
    // If the message table doesn't exist, return what we have
  }

  const messages: ThreadMessage[] = [];
  const msgTableData = (msgRecordMap as Record<string, Record<string, { value: Record<string, unknown> }>>)[msgTable];
  if (msgTableData) {
    for (const id of messageIds) {
      const rec = msgTableData[id]?.value;
      if (!rec) continue;

      const content =
        extractMessageContent(rec.value) ??
        extractMessageContent(rec.text) ??
        (rec.content as string | undefined) ??
        "";

      messages.push({
        id,
        role: (rec.type as string) ?? "unknown",
        content,
        createdAt: rec.created_time as number | undefined,
      });
    }
  }

  return {
    messages,
    title,
    ...(messages.length === 0 ? { raw: { _foundTable: foundTable, record: threadRecord, msgTable, msgRecordMap } } : {}),
  };
}

/**
 * Try to extract text content from various message value shapes.
 * Notion thread messages may use [[text]], [{type, content}], or plain strings.
 */
function extractMessageContent(value: unknown): string | undefined {
  if (!value) return undefined;
  if (typeof value === "string") return value;

  // Array of [{type: "text", content: "..."}] (agent-inference style)
  if (Array.isArray(value) && value.length > 0) {
    if (typeof value[0] === "object" && value[0]?.content) {
      return value.map((v: { content?: string }) => v.content ?? "").join("");
    }
    // [[text]] (user message style)
    if (Array.isArray(value[0]) && typeof value[0][0] === "string") {
      return value.map((v: string[]) => v[0]).join("");
    }
  }
  return undefined;
}

// --- Streaming chat (runInferenceTranscript) ---

export type RunInferenceParams = {
  message: string;
  model?: string;
  threadId?: string;
  /** Explicitly mark as new thread (sets createThread/generateTitle). */
  isNewThread?: boolean;
  /** Page ID to set as context */
  pageId?: string;
  /** Disable workspace/web search */
  noSearch?: boolean;
  /** User identity (from v3 session) */
  user: {
    id: string;
    name: string;
    email: string;
  };
  /** Workspace info (from v3 session) */
  space: {
    id: string;
    name: string;
  };
};

const STREAM_TIMEOUT = 120_000; // 2 minutes for streaming responses

/**
 * Run a Notion AI inference transcript (chat).
 * Returns an AsyncIterable of NDJSON events for streaming consumption.
 *
 * The caller iterates events to extract AI output:
 *   for await (const event of runInferenceTranscript(client, params)) {
 *     if (event.type === "agent-inference") {
 *       // event.value[0].content has cumulative text so far
 *     }
 *   }
 *
 * The final "agent-inference" event includes finishedAt, token counts, and model.
 */
export async function runInferenceTranscript(
  client: V3HttpClient,
  params: RunInferenceParams,
  options?: { debug?: boolean },
): Promise<AsyncIterable<NdjsonEvent>> {
  const debug = options?.debug ?? false;
  const traceId = crypto.randomUUID();
  const threadId = params.threadId ?? crypto.randomUUID();
  const isNewThread = params.isNewThread ?? !params.threadId;

  const configItem: TranscriptConfigItem = {
    id: crypto.randomUUID(),
    type: "config",
    value: {
      type: "workflow",
      availableConnectors: [],
      searchScopes: params.noSearch ? [] : [{ type: "everything" }],
      useWebSearch: !params.noSearch,
      writerMode: false,
      ...(params.model ? { model: params.model } : {}),
      modelFromUser: !!params.model,
      isCustomAgent: false,
      isCustomAgentBuilder: false,
      useCustomAgentDraft: false,
      use_draft_actor_pointer: false,
      enableAgentDiffs: true,
      enableTurnLevelTitleDiff: true,
      enableAgentCreateDbTemplate: true,
      enableCsvAttachmentSupport: true,
      enableAgentCardCustomization: true,
      enableUpdatePageV2Tool: true,
      enableUpdatePageAutofixer: true,
      enableUpdatePageOrderUpdates: true,
      enableAgentSupportPropertyReorder: true,
      useServerUndo: true,
      useReadOnlyMode: false,
      useSearchToolV2: false,
      enableAgentAutomations: false,
      enableAgentIntegrations: false,
      enableCustomAgents: false,
      enableExperimentalIntegrations: false,
      enableAgentViewNotificationsTool: false,
      enableDatabaseAgents: false,
      enableAgentThreadTools: false,
      enableRunAgentTool: false,
      enableSetupModeTool: false,
      enableAgentDashboards: false,
      enableSystemPromptAsPage: false,
      enableUserSessionContext: false,
      enableScriptAgentAdvanced: false,
      enableScriptAgent: false,
      enableScriptAgentIntegrations: false,
      enableScriptAgentCustomAgentTools: false,
      enableAgentGenerateImage: false,
      enableSpeculativeSearch: false,
      enableQueryCalendar: false,
      enableQueryMail: false,
      enableMailExplicitToolCalls: false,
      enableAgentVerification: false,
      enableUpdatePageMarkdownTree: false,
      databaseAgentConfigMode: false,
    },
  };

  const now = new Date().toISOString();
  const contextItem: TranscriptContextItem = {
    id: crypto.randomUUID(),
    neverCompress: true,
    type: "context",
    value: {
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      userName: params.user.name,
      userId: params.user.id,
      userEmail: params.user.email,
      spaceName: params.space.name,
      spaceId: params.space.id,
      currentDatetime: now,
      surface: "workflows",
      ...(params.pageId ? { blockId: params.pageId } : {}),
      visibleCollectionViewIds: {},
    },
  };

  const userItem: TranscriptUserItem = {
    id: crypto.randomUUID(),
    type: "user",
    value: [[params.message]],
    userId: params.user.id,
    createdAt: now,
  };

  const body: RunInferenceTranscriptRequest = {
    traceId,
    spaceId: params.space.id,
    transcript: [configItem, contextItem, userItem],
    threadId,
    threadParentPointer: {
      table: "space",
      id: params.space.id,
      spaceId: params.space.id,
    },
    createThread: isNewThread,
    generateTitle: isNewThread,
    saveAllThreadOperations: true,
    threadType: "workflow",
    isPartialTranscript: false,
    asPatchResponse: false,
    debugOverrides: {
      emitAgentSearchExtractedResults: true,
      cachedInferences: {},
      annotationInferences: {},
      emitInferences: false,
    },
  };

  if (debug) {
    process.stderr.write(`[debug] request body:\n${JSON.stringify(body, null, 2)}\n`);
  }

  const response = await client.postStream(
    "runInferenceTranscript",
    body,
    { timeout: STREAM_TIMEOUT },
  );

  if (debug) {
    const d = (msg: string) => process.stderr.write(`[debug] ${msg}\n`);
    d(`response status: ${response.status} ${response.statusText}`);
    d(`content-type: ${response.headers.get("content-type")}`);

    // Save full body for analysis
    const text = await response.clone().text();
    d(`body length: ${text.length} chars`);
    if (text.length > 0) {
      const debugFile = "/tmp/notion-ai-debug-body.ndjson";
      await Bun.write(debugFile, text);
      d(`body saved to ${debugFile}`);
    } else {
      d(`body is EMPTY — API returned 200 with no content`);
    }
  }

  return parseNdjson<NdjsonEvent>(response, debug ? (line) => {
    process.stderr.write(`[debug:raw] ${line.slice(0, 500)}\n`);
  } : undefined);
}
