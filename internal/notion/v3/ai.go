// Notion AI domain logic — lang-tag helpers, model resolution, transcript
// listing, and thread content. The streaming chat pipeline lives in
// ai_stream.go and the JSON-patch engine in ai_patch.go. AI is v3-only.

package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// --- Lang tag helpers ---

var (
	langTagPrefix     = regexp.MustCompile(`(?i)^<lang\s+[^>]*/>\s*`)
	incompleteLangTag = regexp.MustCompile(`(?i)^<lang\b`)
)

// StripLangTag removes Notion's leading internal language tag from a response.
func StripLangTag(s string) string {
	return langTagPrefix.ReplaceAllString(s, "")
}

// IsIncompleteLangTag reports whether s begins a lang tag still being streamed
// (opened but not yet closed with ">").
func IsIncompleteLangTag(s string) bool {
	return incompleteLangTag.MatchString(s) && !strings.Contains(s, ">")
}

// --- Model resolution ---

// ResolveModel resolves a model name (codename or display name) to its
// codename, falling back to configDefault when modelFlag is empty. Returns ""
// when neither is set (let the API pick), or an error for an unknown name.
func ResolveModel(models []AIModel, modelFlag, configDefault string) (string, error) {
	input := modelFlag
	if input == "" {
		input = configDefault
	}
	if input == "" {
		return "", nil
	}

	for _, m := range models {
		if m.Model == input {
			return m.Model, nil
		}
	}
	lower := strings.ToLower(input)
	for _, m := range models {
		if strings.ToLower(m.ModelMessage) == lower {
			return m.Model, nil
		}
	}
	for _, m := range models {
		if strings.Contains(strings.ToLower(m.ModelMessage), lower) {
			return m.Model, nil
		}
	}
	return "", fmt.Errorf("Unknown model %q. Run 'ai model list --raw' to see available model codenames.", input)
}

// --- Listing wrappers ---

// GetAvailableModels returns the AI models for a space.
func GetAvailableModels(ctx context.Context, c *Client, spaceID string) ([]AIModel, error) {
	resp, err := c.GetAvailableModels(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// TranscriptList is the trimmed AI thread listing the CLI renders.
type TranscriptList struct {
	Transcripts     []InferenceTranscript
	UnreadThreadIDs []string
	HasMore         bool
}

// GetInferenceTranscripts lists the user's AI chat threads.
func GetInferenceTranscripts(ctx context.Context, c *Client, spaceID string, limit int) (TranscriptList, error) {
	resp, err := c.GetInferenceTranscriptsForUser(ctx, spaceID, limit)
	if err != nil {
		return TranscriptList{}, err
	}
	return TranscriptList{
		Transcripts:     resp.Transcripts,
		UnreadThreadIDs: resp.UnreadThreadIDs,
		HasMore:         resp.HasMore,
	}, nil
}

// MarkTranscriptSeen marks a chat thread as read.
func MarkTranscriptSeen(ctx context.Context, c *Client, spaceID, threadID string) (bool, error) {
	resp, err := c.MarkInferenceTranscriptSeen(ctx, spaceID, threadID)
	if err != nil {
		return false, err
	}
	return resp.OK, nil
}

// --- Thread content ---

// GetThreadContent fetches an AI chat thread and its messages. When the thread
// record cannot be found, Found is false and Messages is empty.
func GetThreadContent(ctx context.Context, c *Client, threadID, spaceID string) (ThreadContent, error) {
	threadRec := fetchRecord(ctx, c, "thread", threadID, spaceID)
	if threadRec == nil {
		return ThreadContent{Messages: []ThreadMessage{}, Found: false}, nil
	}

	title := threadTitle(threadRec)
	messageIDs := stringSlice(threadRec["messages"])
	if len(messageIDs) == 0 {
		return ThreadContent{Messages: []ThreadMessage{}, Title: title, Found: true}, nil
	}

	msgTable := fetchTable(ctx, c, "thread_message", messageIDs, spaceID)
	if msgTable == nil {
		return ThreadContent{Messages: []ThreadMessage{}, Title: title, Found: true}, nil
	}

	messages := []ThreadMessage{}
	for _, id := range messageIDs {
		entry, ok := msgTable[id]
		if !ok {
			continue
		}
		var rec map[string]any
		if json.Unmarshal(entry.Value, &rec) != nil {
			continue
		}
		step, _ := rec["step"].(map[string]any)
		if step == nil {
			continue
		}
		stepType, _ := step["type"].(string)
		if msg, ok := ParseThreadMessage(id, stepType, step, rec); ok {
			messages = append(messages, msg)
		}
	}

	return ThreadContent{Messages: messages, Title: title, Found: true}, nil
}

// fetchRecord syncs a single record and returns its entity as a generic map.
func fetchRecord(ctx context.Context, c *Client, table, id, spaceID string) map[string]any {
	resp, err := c.SyncRecordValuesForPointers(ctx, []SyncPointer{{ID: id, Table: table, SpaceID: spaceID}})
	if err != nil {
		return nil
	}
	entry, ok := resp.RecordMap[table][id]
	if !ok {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(entry.Value, &m) != nil {
		return nil
	}
	return m
}

// fetchTable syncs a set of records and returns the named table (nil on error).
func fetchTable(ctx context.Context, c *Client, table string, ids []string, spaceID string) Table {
	pointers := make([]SyncPointer, 0, len(ids))
	for _, id := range ids {
		pointers = append(pointers, SyncPointer{ID: id, Table: table, SpaceID: spaceID})
	}
	resp, err := c.SyncRecordValuesForPointers(ctx, pointers)
	if err != nil {
		return nil
	}
	return resp.RecordMap[table]
}

// threadTitle reads data.title, falling back to title.
func threadTitle(rec map[string]any) string {
	if data, ok := rec["data"].(map[string]any); ok {
		if t, ok := data["title"].(string); ok {
			return t
		}
	}
	if t, ok := rec["title"].(string); ok {
		return t
	}
	return ""
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ParseThreadMessage converts a thread_message step into a ThreadMessage.
// ok is false for skipped step types (config, context, title, record-map, …).
func ParseThreadMessage(id, stepType string, step, rec map[string]any) (ThreadMessage, bool) {
	var createdAt *int64
	if v, ok := rec["created_time"].(float64); ok {
		t := int64(v)
		createdAt = &t
	}

	switch stepType {
	case "user":
		content, _ := ExtractRichText(step["value"])
		return ThreadMessage{ID: id, Role: "user", Content: content, CreatedAt: createdAt}, true

	case "agent-inference":
		content := ""
		if arr, ok := step["value"].([]any); ok {
			for _, v := range arr {
				m, ok := v.(map[string]any)
				if !ok || m["type"] != "text" {
					continue
				}
				content, _ = m["content"].(string)
				break
			}
		}
		return ThreadMessage{ID: id, Role: "assistant", Content: StripLangTag(content), CreatedAt: createdAt}, true

	case "agent-tool-result":
		toolName, _ := step["toolName"].(string)
		state, _ := step["state"].(string)
		errStr, _ := step["error"].(string)
		content := fmt.Sprintf(`Tool "%s" completed`, toolName)
		if errStr != "" {
			content = fmt.Sprintf(`Tool "%s" failed: %s`, toolName, errStr)
		}
		return ThreadMessage{ID: id, Role: "tool", Content: content, CreatedAt: createdAt, ToolName: toolName, ToolState: state}, true

	default:
		return ThreadMessage{}, false
	}
}

// ExtractRichText pulls plain text from Notion's [["a"],["b"]] rich-text form
// or from a bare string. ok is false when the value is neither.
func ExtractRichText(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	if s, ok := value.(string); ok {
		return s, true
	}
	arr, ok := value.([]any)
	if !ok || len(arr) == 0 {
		return "", false
	}
	first, ok := arr[0].([]any)
	if !ok || len(first) == 0 {
		return "", false
	}
	if _, ok := first[0].(string); !ok {
		return "", false
	}
	var b strings.Builder
	for _, v := range arr {
		if inner, ok := v.([]any); ok && len(inner) > 0 {
			if s, ok := inner[0].(string); ok {
				b.WriteString(s)
			}
		}
	}
	return b.String(), true
}
