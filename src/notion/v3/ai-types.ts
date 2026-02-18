/**
 * TypeScript types for Notion AI API request/response shapes.
 * Reference: design-docs/notion-ai-api.md
 */

// --- Models ---

export type AiModel = {
  /** Internal codename (e.g., "oatmeal-cookie") */
  model: string;
  /** Human-readable name (e.g., "GPT-5.2") */
  modelMessage: string;
  /** Provider family: "openai" | "anthropic" | "gemini" */
  modelFamily: string;
  /** "fast" or "intelligent" */
  displayGroup: string;
  /** Whether the model is currently disabled */
  isDisabled?: boolean;
  /** Capabilities per surface */
  markdownChat?: { beta?: boolean };
  workflow?: { finalModelName?: string; beta?: boolean };
};

export type GetAvailableModelsResponse = {
  models: AiModel[];
};

// --- Inference Transcripts (conversation list) ---

export type InferenceTranscript = {
  id: string;
  title: string;
  created_at: number;
  updated_at: number;
  created_by_display_name: string;
  type: string;
};

export type GetInferenceTranscriptsResponse = {
  transcripts: InferenceTranscript[];
  threadIds: string[];
  unreadThreadIds: string[];
  hasMore: boolean;
};

// --- Mark transcript seen ---

export type MarkTranscriptSeenResponse = {
  ok: boolean;
};

// --- runInferenceTranscript request ---

export type TranscriptConfigItem = {
  id: string;
  type: "config";
  value: {
    type: "workflow";
    availableConnectors?: string[];
    searchScopes?: Array<{ type: string }>;
    useWebSearch?: boolean;
    writerMode?: boolean;
    model?: string;
    enableAgentDiffs?: boolean;
    useServerUndo?: boolean;
    [key: string]: unknown;
  };
};

export type TranscriptContextItem = {
  id: string;
  neverCompress: true;
  type: "context";
  value: {
    timezone: string;
    userName: string;
    userId: string;
    userEmail: string;
    spaceName: string;
    spaceId: string;
    spaceViewId?: string;
    currentDatetime: string;
    surface: string;
    blockId?: string;
  };
};

export type TranscriptUserItem = {
  id: string;
  type: "user";
  value: string[][];
  userId: string;
  createdAt: string;
};

export type TranscriptItem =
  | TranscriptConfigItem
  | TranscriptContextItem
  | TranscriptUserItem;

export type RunInferenceTranscriptRequest = {
  traceId: string;
  spaceId: string;
  transcript: TranscriptItem[];
  threadId: string;
  threadParentPointer?: {
    table: string;
    id: string;
    spaceId: string;
  };
  createThread: boolean;
  generateTitle: boolean;
  saveAllThreadOperations: boolean;
  threadType: string;
  isPartialTranscript: boolean;
  asPatchResponse: boolean;
  isUserInAnySalesAssistedSpace?: boolean;
  isSpaceSalesAssisted?: boolean;
  debugOverrides?: {
    emitAgentSearchExtractedResults?: boolean;
    cachedInferences?: Record<string, unknown>;
    annotationInferences?: Record<string, unknown>;
    emitInferences?: boolean;
  };
};

// --- runInferenceTranscript NDJSON stream events ---

export type NdjsonEvent =
  | AgentInferenceEvent
  | AgentToolResultEvent
  | ConfigEvent
  | TitleEvent
  | RecordMapEvent
  | AgentTurnEvent
  | PatchStartEvent
  | PatchEvent;

export type AgentInferenceEvent = {
  type: "agent-inference";
  id: string;
  value: Array<{
    type: "text" | "thinking";
    content: string;
    encryptedContent?: string;
    id?: string;
  }>;
  traceId: string;
  startedAt: number;
  previousAttemptValues: unknown[];
  /** Present only on final event */
  finishedAt?: number;
  inputTokens?: number;
  outputTokens?: number;
  cachedTokensRead?: number;
  maxContextTokens?: number;
  maxInputTokens?: number;
  model?: string;
};

export type AgentToolResultEvent = {
  type: "agent-tool-result";
  toolName: string;
  toolType: string;
  input: Record<string, unknown>;
  state: "pending" | "applied";
  result?: Record<string, unknown>;
};

export type ConfigEvent = {
  type: "config";
  [key: string]: unknown;
};

export type TitleEvent = {
  type: "title";
  value?: string;
  [key: string]: unknown;
};

export type RecordMapEvent = {
  type: "record-map";
  [key: string]: unknown;
};

export type AgentTurnEvent = {
  type: "agent-turn-full-record-map";
  [key: string]: unknown;
};

export type PatchStartEvent = {
  type: "patch-start";
  data: {
    s: Array<Record<string, unknown>>;
  };
  version: number;
};

export type PatchOp = {
  /** "x" = text append, "a" = add, "r" = replace/remove */
  o: "x" | "a" | "r";
  /** JSON pointer path, e.g. "/s/2/value/0/content" */
  p: string;
  /** Value to append/add/replace */
  v?: unknown;
};

export type PatchEvent = {
  type: "patch";
  v: PatchOp[];
};

// --- Type guards for NdjsonEvent ---

export function isAgentInference(e: NdjsonEvent): e is AgentInferenceEvent {
  return e.type === "agent-inference";
}

export function isAgentToolResult(e: NdjsonEvent): e is AgentToolResultEvent {
  return e.type === "agent-tool-result";
}

export function isPatchStart(e: NdjsonEvent): e is PatchStartEvent {
  return e.type === "patch-start";
}

export function isPatch(e: NdjsonEvent): e is PatchEvent {
  return e.type === "patch";
}

export function isTitle(e: NdjsonEvent): e is TitleEvent {
  return e.type === "title";
}

// --- syncRecordValues intermediate types ---

/** Shape returned by syncRecordValues for record entries (double .value.value nesting) */
export type SyncRecordEntry<T> = {
  spaceId?: string;
  value?: { value?: T; role?: string };
};

export type ThreadRecord = {
  id: string;
  messages?: string[];
  data?: { title?: string };
  title?: string;
  [key: string]: unknown;
};

// --- Thread message type ---

export type ThreadMessage = {
  id: string;
  role: "user" | "assistant" | "tool" | "system";
  content: string;
  createdAt?: number;
  /** For tool messages: the tool name */
  toolName?: string;
  /** For tool messages: whether it succeeded */
  toolState?: string;
};

// --- Chat result (from processInferenceStream) ---

export type ChatResult = {
  response: string;
  title?: string;
  model?: string;
  tokens?: { input: number; output?: number; cached?: number };
};

// --- RunInference params ---

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
    viewId?: string;
  };
};
