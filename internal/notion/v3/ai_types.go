// Notion AI request/response types. Wire-format field names are the API
// contract — do not rename. AI is v3-only (no official REST equivalent).

package v3

import "encoding/json"

// --- Models ---

// AIModel is one selectable Notion AI model.
type AIModel struct {
	Model        string             `json:"model"`        // internal codename, e.g. "oatmeal-cookie"
	ModelMessage string             `json:"modelMessage"` // human name, e.g. "GPT-5.2"
	ModelFamily  string             `json:"modelFamily"`  // openai | anthropic | gemini
	DisplayGroup string             `json:"displayGroup"` // fast | intelligent
	IsDisabled   bool               `json:"isDisabled,omitempty"`
	MarkdownChat *AIModelCapability `json:"markdownChat,omitempty"`
	Workflow     *AIModelWorkflow   `json:"workflow,omitempty"`
}

// AIModelCapability flags a per-surface capability.
type AIModelCapability struct {
	Beta bool `json:"beta,omitempty"`
}

// AIModelWorkflow describes the workflow-surface capability.
type AIModelWorkflow struct {
	FinalModelName string `json:"finalModelName,omitempty"`
	Beta           bool   `json:"beta,omitempty"`
}

// GetAvailableModelsResponse is the getAvailableModels reply.
type GetAvailableModelsResponse struct {
	Models []AIModel `json:"models"`
}

// --- Inference transcripts (conversation list) ---

// InferenceTranscript is one AI chat thread summary.
type InferenceTranscript struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	CreatedAt            int64  `json:"created_at"`
	UpdatedAt            int64  `json:"updated_at"`
	CreatedByDisplayName string `json:"created_by_display_name"`
	Type                 string `json:"type"`
}

// GetInferenceTranscriptsResponse is the getInferenceTranscriptsForUser reply.
type GetInferenceTranscriptsResponse struct {
	Transcripts     []InferenceTranscript `json:"transcripts"`
	ThreadIDs       []string              `json:"threadIds"`
	UnreadThreadIDs []string              `json:"unreadThreadIds"`
	HasMore         bool                  `json:"hasMore"`
}

// MarkTranscriptSeenResponse is the markInferenceTranscriptSeen reply.
type MarkTranscriptSeenResponse struct {
	OK bool `json:"ok"`
}

// --- runInferenceTranscript request ---

// ThreadParentPointer roots a new thread under a space.
type ThreadParentPointer struct {
	Table   string `json:"table"`
	ID      string `json:"id"`
	SpaceID string `json:"spaceId"`
}

// DebugOverrides carries the request's debug flags (always sent).
type DebugOverrides struct {
	EmitAgentSearchExtractedResults bool           `json:"emitAgentSearchExtractedResults"`
	CachedInferences                map[string]any `json:"cachedInferences"`
	AnnotationInferences            map[string]any `json:"annotationInferences"`
	EmitInferences                  bool           `json:"emitInferences"`
}

// RunInferenceTranscriptRequest is the streaming chat request envelope. The
// transcript items (config/context/user) are built as maps because the config
// value spreads a large fixed flag block; see buildInferenceRequest.
type RunInferenceTranscriptRequest struct {
	TraceID                       string               `json:"traceId"`
	SpaceID                       string               `json:"spaceId"`
	Transcript                    []any                `json:"transcript"`
	ThreadID                      string               `json:"threadId"`
	ThreadParentPointer           *ThreadParentPointer `json:"threadParentPointer,omitempty"`
	CreateThread                  bool                 `json:"createThread"`
	GenerateTitle                 bool                 `json:"generateTitle"`
	SaveAllThreadOperations       bool                 `json:"saveAllThreadOperations"`
	ThreadType                    string               `json:"threadType"`
	IsPartialTranscript           bool                 `json:"isPartialTranscript"`
	AsPatchResponse               bool                 `json:"asPatchResponse"`
	IsUserInAnySalesAssistedSpace bool                 `json:"isUserInAnySalesAssistedSpace"`
	IsSpaceSalesAssisted          bool                 `json:"isSpaceSalesAssisted"`
	DebugOverrides                *DebugOverrides      `json:"debugOverrides,omitempty"`
}

// --- runInferenceTranscript NDJSON stream events ---

// NdjsonEvent is one decoded stream line: its discriminant type plus the raw
// JSON, decoded on demand by the typed accessors.
type NdjsonEvent struct {
	Type string
	Raw  json.RawMessage
}

// InferenceValue is one entry in an agent-inference event's value array.
type InferenceValue struct {
	Type             string `json:"type"` // text | thinking
	Content          string `json:"content"`
	EncryptedContent string `json:"encryptedContent,omitempty"`
	ID               string `json:"id,omitempty"`
}

// AgentInferenceEvent is the decoded form of an "agent-inference" event.
type AgentInferenceEvent struct {
	Type             string           `json:"type"`
	ID               string           `json:"id"`
	Value            []InferenceValue `json:"value"`
	TraceID          string           `json:"traceId"`
	StartedAt        int64            `json:"startedAt"`
	FinishedAt       *int64           `json:"finishedAt,omitempty"`
	InputTokens      *int             `json:"inputTokens,omitempty"`
	OutputTokens     *int             `json:"outputTokens,omitempty"`
	CachedTokensRead *int             `json:"cachedTokensRead,omitempty"`
	Model            string           `json:"model,omitempty"`
}

// PatchOp is one JSON-pointer patch operation in a "patch" event.
type PatchOp struct {
	O string `json:"o"` // x = text append, a = add, r = replace/remove
	P string `json:"p"` // JSON pointer, e.g. "/s/2/value/0/content"
	V any    `json:"v,omitempty"`
}

// IsAgentInference reports whether this is an agent-inference event.
func (e NdjsonEvent) IsAgentInference() bool { return e.Type == "agent-inference" }

// AgentInference decodes the event as an AgentInferenceEvent.
func (e NdjsonEvent) AgentInference() (AgentInferenceEvent, bool) {
	var a AgentInferenceEvent
	if json.Unmarshal(e.Raw, &a) != nil {
		return AgentInferenceEvent{}, false
	}
	return a, true
}

// Title returns the value of a "title" event, ok=false for other types.
func (e NdjsonEvent) Title() (string, bool) {
	if e.Type != "title" {
		return "", false
	}
	var t struct {
		Value string `json:"value"`
	}
	_ = json.Unmarshal(e.Raw, &t)
	return t.Value, true
}

// decodeEvent parses one NDJSON line into an NdjsonEvent.
func decodeEvent(raw json.RawMessage) (NdjsonEvent, bool) {
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &head) != nil {
		return NdjsonEvent{}, false
	}
	return NdjsonEvent{Type: head.Type, Raw: raw}, true
}

// --- Thread content ---

// ThreadMessage is one normalized message in an AI chat thread.
type ThreadMessage struct {
	ID        string `json:"id"`
	Role      string `json:"role"` // user | assistant | tool | system
	Content   string `json:"content"`
	CreatedAt *int64 `json:"created_at,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	ToolState string `json:"tool_state,omitempty"`
}

// ThreadContent is a fetched AI chat thread.
type ThreadContent struct {
	Messages []ThreadMessage `json:"messages"`
	Title    string          `json:"title,omitempty"`
	// Found is false when the thread record could not be located (the TS
	// returned a raw._note string; a bool is the Go-idiomatic equivalent).
	Found bool `json:"found"`
}

// --- Chat result ---

// ChatTokens is the token accounting for a completed inference.
type ChatTokens struct {
	Input  int  `json:"input"`
	Output *int `json:"output,omitempty"`
	Cached *int `json:"cached,omitempty"`
}

// ChatResult is the accumulated output of an inference stream.
type ChatResult struct {
	Response string      `json:"response"`
	Title    string      `json:"title,omitempty"`
	Model    string      `json:"model,omitempty"`
	Tokens   *ChatTokens `json:"tokens,omitempty"`
}

// --- Inference params ---

// AIUser is the caller identity sent as inference context.
type AIUser struct {
	ID    string
	Name  string
	Email string
}

// AISpace is the workspace sent as inference context.
type AISpace struct {
	ID     string
	Name   string
	ViewID string
}

// RunInferenceParams configures a chat inference.
type RunInferenceParams struct {
	Message string
	Model   string
	// ThreadID continues an existing thread; empty starts a new one.
	ThreadID string
	// IsNewThread overrides the new-thread decision; nil defaults to
	// (ThreadID == "").
	IsNewThread *bool
	// PageID sets the current page as context.
	PageID string
	// NoSearch disables workspace/web search.
	NoSearch bool
	// ReadOnly puts the AI in ask/answer mode: the page-editing tools are
	// disabled so a prompt cannot mutate a document. This is the CLI default.
	ReadOnly bool
	User     AIUser
	Space    AISpace
}
