/**
 * Notion AI client — methods for interacting with Notion's AI endpoints.
 * All methods call V3HttpClient directly (AI is v3-only, no official API equivalent).
 *
 * Reference: design-docs/notion-ai-api.md
 */
import type { V3HttpClient } from "./client.ts";
import type {
  AiModel,
  ChatResult,
  InferenceTranscript,
  NdjsonEvent,
  RunInferenceParams,
  RunInferenceTranscriptRequest,
  SyncRecordEntry,
  ThreadMessage,
  ThreadRecord,
  TranscriptConfigItem,
  TranscriptContextItem,
  TranscriptItem,
  TranscriptUserItem,
} from "./ai-types.ts";
import {
  isAgentInference,
  isPatch,
  isPatchStart,
  isTitle,
} from "./ai-types.ts";
import { parseNdjson } from "./ndjson.ts";
import { CliError } from "../../lib/errors.ts";

// --- Default config flags for AI inference ---

const DEFAULT_CONFIG_FLAGS = {
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
} as const;

// --- Lang tag helpers ---

/** Strip Notion's internal language tag from AI responses */
export function stripLangTag(s: string): string {
  return s.replace(/^<lang\s+[^>]*\/>\s*/i, "");
}

/** Check if string starts with an incomplete lang tag still being streamed */
export function isIncompleteLangTag(s: string): boolean {
  return /^<lang\b/i.test(s) && !/>/.test(s);
}

// --- Model resolution ---

/**
 * Resolve a model name (codename or display name) to its codename.
 * Falls back to `configDefault` if no `modelFlag` provided.
 * Returns undefined if no model specified anywhere (let the API pick).
 */
export async function resolveModel(
  models: AiModel[],
  modelFlag: string | undefined,
  configDefault: string | undefined,
): Promise<string | undefined> {
  const input = modelFlag ?? configDefault;
  if (!input) return undefined;

  // Exact codename match
  const byCodename = models.find((m) => m.model === input);
  if (byCodename) return byCodename.model;

  // Case-insensitive display name match
  const lower = input.toLowerCase();
  const byDisplayName = models.find(
    (m) => m.modelMessage.toLowerCase() === lower,
  );
  if (byDisplayName) return byDisplayName.model;

  // Partial match on display name
  const byPartial = models.find((m) =>
    m.modelMessage.toLowerCase().includes(lower),
  );
  if (byPartial) return byPartial.model;

  throw new CliError(
    `Unknown model "${input}". Run 'ai model list --raw' to see available model codenames.`,
  );
}

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

export type { ChatResult, ThreadMessage };

/**
 * Fetch the content of an AI chat thread by ID.
 * Uses the `thread` table for the thread record and `thread_message` for messages.
 * Each thread_message has a `step` field with `type` and `value`.
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
  // Fetch thread record
  // syncRecordValues nests records as: recordMap.thread[id].value.value = { ...actual data }
  let threadRecord: ThreadRecord | undefined;
  try {
    const result = await client.syncRecordValuesForPointers([
      { id: threadId, table: "thread", spaceId },
    ]);
    const entry = result.recordMap.thread?.[threadId] as SyncRecordEntry<ThreadRecord> | undefined;
    threadRecord = entry?.value?.value;
  } catch {
    // thread table failed
  }

  if (!threadRecord) {
    return {
      messages: [],
      raw: { _note: "Thread not found in 'thread' table" },
    };
  }

  // Title is at data.title
  const title = threadRecord.data?.title ?? threadRecord.title;

  // Message IDs are in the `messages` field
  const messageIds = threadRecord.messages ?? [];

  if (messageIds.length === 0) {
    return { messages: [], title };
  }

  // Fetch all thread_message records
  // Same nesting: recordMap.thread_message[id].value.value = { ...actual data }
  let rawMsgTable: Record<string, SyncRecordEntry<Record<string, unknown>>> | undefined;
  try {
    const msgResult = await client.syncRecordValuesForPointers(
      messageIds.map((id) => ({ id, table: "thread_message", spaceId })),
    );
    rawMsgTable = msgResult.recordMap.thread_message as
      Record<string, SyncRecordEntry<Record<string, unknown>>> | undefined;
  } catch {
    // thread_message table failed
  }

  if (!rawMsgTable) {
    return { messages: [], title };
  }

  const messages: ThreadMessage[] = [];
  for (const id of messageIds) {
    const rec = rawMsgTable[id]?.value?.value;
    if (!rec) continue;

    const step = rec.step as Record<string, unknown> | undefined;
    if (!step) continue;

    const stepType = step.type as string;
    const msg = parseThreadMessage(id, stepType, step, rec);
    if (msg) messages.push(msg);
  }

  return { messages, title };
}

/**
 * Parse a thread_message record's `step` into a ThreadMessage.
 * Returns null for message types we skip (config, context, title, record-map).
 */
function parseThreadMessage(
  id: string,
  stepType: string,
  step: Record<string, unknown>,
  rec: Record<string, unknown>,
): ThreadMessage | null {
  const createdAt = rec.created_time as number | undefined;

  switch (stepType) {
    case "user": {
      const value = step.value as unknown;
      const content = extractRichText(value) ?? "";
      return { id, role: "user", content, createdAt };
    }

    case "agent-inference": {
      const value = step.value as Array<{ type: string; content: string }> | undefined;
      const textEntry = value?.find((v) => v.type === "text");
      const raw = textEntry?.content ?? "";
      const content = stripLangTag(raw);
      return { id, role: "assistant", content, createdAt };
    }

    case "agent-tool-result": {
      const toolName = step.toolName as string | undefined;
      const state = step.state as string | undefined;
      const error = step.error as string | undefined;
      const content = error
        ? `Tool "${toolName}" failed: ${error}`
        : `Tool "${toolName}" completed`;
      return { id, role: "tool", content, createdAt, toolName, toolState: state };
    }

    // Skip config, context, title, agent-turn-full-record-map
    default:
      return null;
  }
}

/**
 * Extract text from Notion rich-text array format: [["text"], ["more text"]].
 */
function extractRichText(value: unknown): string | undefined {
  if (!value) return undefined;
  if (typeof value === "string") return value;
  if (Array.isArray(value) && value.length > 0) {
    if (Array.isArray(value[0]) && typeof value[0][0] === "string") {
      return value.map((v: string[]) => v[0]).join("");
    }
  }
  return undefined;
}

// --- Streaming chat (runInferenceTranscript) ---

export type { RunInferenceParams };

const STREAM_TIMEOUT = 120_000; // 2 minutes for streaming responses

/**
 * Build the transcript items (config, context, user) for an inference request.
 */
function buildTranscriptItems(params: RunInferenceParams): TranscriptItem[] {
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
      ...DEFAULT_CONFIG_FLAGS,
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

  return [configItem, contextItem, userItem];
}

export async function runInferenceTranscript(
  client: V3HttpClient,
  params: RunInferenceParams,
  options?: { debug?: boolean },
): Promise<AsyncIterable<NdjsonEvent>> {
  const debug = options?.debug ?? false;
  const traceId = crypto.randomUUID();
  const threadId = params.threadId ?? crypto.randomUUID();
  const isNewThread = params.isNewThread ?? !params.threadId;

  const body: RunInferenceTranscriptRequest = {
    traceId,
    spaceId: params.space.id,
    transcript: buildTranscriptItems(params),
    threadId,
    // threadParentPointer only needed for new threads
    ...(isNewThread
      ? {
          threadParentPointer: {
            table: "space",
            id: params.space.id,
            spaceId: params.space.id,
          },
        }
      : {}),
    createThread: isNewThread,
    generateTitle: isNewThread,
    saveAllThreadOperations: true,
    threadType: "workflow",
    // For replies: server reconstructs history from threadId
    isPartialTranscript: !isNewThread,
    asPatchResponse: !isNewThread,
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

  const rawEvents = parseNdjson<NdjsonEvent>(response, debug ? (line) => {
    process.stderr.write(`[debug:raw] ${line.slice(0, 500)}\n`);
  } : undefined);

  // For replies (asPatchResponse), normalize patch events into standard events
  if (!isNewThread) {
    return normalizePatchStream(rawEvents);
  }
  return rawEvents;
}

// --- Stream processing ---

/**
 * Process an inference event stream, accumulating the response and metadata.
 * Optionally calls `onStreamChunk` for each new text delta (for --stream mode).
 */
export async function processInferenceStream(
  events: AsyncIterable<NdjsonEvent>,
  onStreamChunk?: (text: string) => void,
): Promise<ChatResult> {
  let lastContent = "";
  let streamedLength = 0;
  let title: string | undefined;
  let model: string | undefined;
  let inputTokens: number | undefined;
  let outputTokens: number | undefined;
  let cachedTokens: number | undefined;

  for await (const event of events) {
    if (isAgentInference(event)) {
      const textEntry = event.value?.find(
        (v) => v && typeof v === "object" && v.type === "text",
      );
      const content = textEntry?.content ?? "";

      if (onStreamChunk) {
        if (!isIncompleteLangTag(content)) {
          const display = stripLangTag(content);
          if (display.length > streamedLength) {
            onStreamChunk(display.slice(streamedLength));
            streamedLength = display.length;
          }
        }
      }

      lastContent = content;

      if (event.finishedAt) {
        model = event.model;
        inputTokens = event.inputTokens;
        outputTokens = event.outputTokens;
        cachedTokens = event.cachedTokensRead;
      }
    } else if (isTitle(event)) {
      title = event.value;
    }
  }

  return {
    response: stripLangTag(lastContent),
    title,
    model,
    tokens:
      inputTokens !== undefined
        ? { input: inputTokens, output: outputTokens, cached: cachedTokens }
        : undefined,
  };
}

/**
 * Convert patch-format stream (used for thread replies) into standard NdjsonEvents.
 *
 * Patch format: patch-start initializes a slots array `s`, then patch events apply
 * JSON-pointer ops to it. We track the state and emit synthetic agent-inference
 * and record-map events.
 */
async function* normalizePatchStream(
  events: AsyncIterable<NdjsonEvent>,
): AsyncIterable<NdjsonEvent> {
  // State slots from patch-start
  let slots: Array<Record<string, unknown>> = [];

  for await (const event of events) {
    if (isPatchStart(event)) {
      slots = (event.data?.s ?? []) as Array<Record<string, unknown>>;
      continue;
    }

    if (isPatch(event)) {
      for (const op of event.v) {
        applyPatchOp(slots, op.o, op.p, op.v);
      }

      // Find the agent-inference slot and emit it as AgentInferenceEvent
      const inferenceSlot = slots.find(
        (s) => s.type === "agent-inference",
      );
      if (inferenceSlot && isAgentInference(inferenceSlot as NdjsonEvent)) {
        yield inferenceSlot as NdjsonEvent;
      }
      continue;
    }

    // Pass through record-map and other events
    yield event;
  }
}

/**
 * Apply a single patch operation to the slots array.
 * Supports: "a" (add), "x" (text append), "r" (replace).
 * Path format: /s/{index}/... or /s/-
 */
function applyPatchOp(
  slots: Array<Record<string, unknown>>,
  op: string,
  path: string,
  value: unknown,
): void {
  const parts = path.split("/").filter(Boolean); // ["s", "2", "value", "0", "content"]
  if (parts.length < 2 || parts[0] !== "s") return;
  const slotKey = parts[1]!;

  // /s/- means append to slots array
  if (slotKey === "-" && op === "a") {
    slots.push(value as Record<string, unknown>);
    return;
  }

  const slotIdx = parseInt(slotKey, 10);
  if (isNaN(slotIdx) || slotIdx >= slots.length) return;

  if (parts.length === 2) {
    if (op === "r") slots[slotIdx] = value as Record<string, unknown>;
    return;
  }

  // Navigate to the target within the slot
  let target: unknown = slots[slotIdx];
  const fieldParts = parts.slice(2);

  for (let i = 0; i < fieldParts.length - 1; i++) {
    const key = fieldParts[i]!;
    if (target == null || typeof target !== "object") return;

    if (Array.isArray(target)) {
      if (key === "-") return;
      target = target[parseInt(key, 10)];
    } else {
      target = (target as Record<string, unknown>)[key];
    }
  }

  const lastKey = fieldParts[fieldParts.length - 1]!;
  if (target == null || typeof target !== "object") return;

  if (Array.isArray(target)) {
    if (lastKey === "-" && op === "a") {
      target.push(value);
    } else {
      const idx = parseInt(lastKey, 10);
      if (op === "a") target[idx] = value;
      else if (op === "r") {
        // "r" = remove element from array (splice out)
        target.splice(idx, 1);
      } else if (op === "x" && typeof target[idx] === "string") {
        target[idx] = (target[idx] as string) + (value as string);
      }
    }
  } else {
    const obj = target as Record<string, unknown>;
    if (op === "a") obj[lastKey] = value;
    else if (op === "r") delete obj[lastKey];
    else if (op === "x" && typeof obj[lastKey] === "string") {
      obj[lastKey] = (obj[lastKey] as string) + (value as string);
    }
  }
}
