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

// --- Streaming chat (runInferenceTranscript) ---

export type RunInferenceParams = {
  message: string;
  model?: string;
  threadId?: string;
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
): Promise<AsyncIterable<NdjsonEvent>> {
  const traceId = crypto.randomUUID();
  const threadId = params.threadId ?? crypto.randomUUID();
  const isNewThread = !params.threadId;

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
      enableAgentDiffs: false,
      useServerUndo: false,
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

  const response = await client.postStream(
    "runInferenceTranscript",
    body,
    { timeout: STREAM_TIMEOUT },
  );

  return parseNdjson<NdjsonEvent>(response);
}
