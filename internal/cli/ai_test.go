package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

func TestAIModelList(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("getAvailableModels", map[string]any{"models": []any{
		map[string]any{"model": "oatmeal-cookie", "modelMessage": "GPT-5.2", "modelFamily": "openai", "displayGroup": "intelligent"},
		map[string]any{"model": "stale", "modelMessage": "Old", "modelFamily": "x", "displayGroup": "fast", "isDisabled": true},
	}})

	out, _, err := runCLI(t, "", "--base-url", url, "ai", "model", "list")
	if err != nil {
		t.Fatal(err)
	}
	lines := decodeLines(t, out)
	if len(lines) != 1 { // disabled model filtered out
		t.Fatalf("lines = %d:\n%s", len(lines), out)
	}
	if lines[0]["name"] != "GPT-5.2" || lines[0]["family"] != "openai" || lines[0]["tier"] != "intelligent" {
		t.Errorf("model = %v", lines[0])
	}
}

func TestAIChatList(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("getInferenceTranscriptsForUser", map[string]any{
		"transcripts": []any{map[string]any{
			"id": "t1", "title": "Chat One", "created_at": 1, "updated_at": 2,
			"created_by_display_name": "Jane", "type": "thread",
		}},
		"threadIds": []any{"t1"}, "unreadThreadIds": []any{"t1"}, "hasMore": false,
	})

	out, _, err := runCLI(t, "", "--base-url", url, "ai", "chat", "list")
	if err != nil {
		t.Fatal(err)
	}
	lines := decodeLines(t, out)
	if len(lines) != 2 { // 1 transcript + @meta
		t.Fatalf("lines = %d:\n%s", len(lines), out)
	}
	if lines[0]["id"] != "t1" || lines[0]["title"] != "Chat One" {
		t.Errorf("transcript = %v", lines[0])
	}
	meta := lines[1]["@meta"].(map[string]any)
	if meta["has_more"] != false || !reflect.DeepEqual(meta["unread_thread_ids"], []any{"t1"}) {
		t.Errorf("@meta = %v", meta)
	}
}

func TestAIChatGet(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	thread := map[string]any{"id": "thread-1", "data": map[string]any{"title": "Example Thread"}, "messages": []any{"msg-1"}}
	userMsg := map[string]any{"id": "msg-1", "created_time": 1700000000000,
		"step": map[string]any{"type": "user", "value": []any{[]any{"hello there"}}}}
	s.HandleWhen("syncRecordValuesMain",
		func(body json.RawMessage) bool { return bytes.Contains(body, []byte("thread_message")) },
		mocknotion.Response{Body: mocknotion.RecordMapBody(map[string]map[string]any{
			"thread_message": {"msg-1": userMsg},
		})},
	)
	s.HandleBody("syncRecordValuesMain", mocknotion.RecordMapBody(map[string]map[string]any{
		"thread": {"thread-1": thread},
	}))

	out, _, err := runCLI(t, "", "--base-url", url, "ai", "chat", "get", "thread-1")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["title"] != "Example Thread" {
		t.Errorf("title = %v", item["title"])
	}
	msgs := item["messages"].([]any)
	if len(msgs) != 1 || msgs[0].(map[string]any)["content"] != "hello there" {
		t.Errorf("messages = %v", msgs)
	}
}

func TestAIChatMarkRead(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	s.HandleBody("markInferenceTranscriptSeen", map[string]any{"ok": true})

	out, _, err := runCLI(t, "", "--base-url", url, "ai", "chat", "mark-read", "t1")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["ok"] != true {
		t.Errorf("mark-read output = %v", item)
	}
}

func TestAIChatSendStreaming(t *testing.T) {
	isolateState(t)
	seedV3Session(t)
	s, url := newMockServer(t)
	ndjson := `{"type":"agent-inference","id":"i","value":[{"type":"text","content":"Hello"}],"traceId":"t","startedAt":1,"previousAttemptValues":[]}
{"type":"title","value":"My Title"}
{"type":"agent-inference","id":"i","value":[{"type":"text","content":"Hello world"}],"traceId":"t","startedAt":1,"previousAttemptValues":[],"finishedAt":2,"inputTokens":10,"model":"m"}
`
	s.Handle("runInferenceTranscript", mocknotion.Response{RawBody: []byte(ndjson)})

	stdout, stderr, err := runCLI(t, "", "--base-url", url, "ai", "chat", "send", "hi", "--stream")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, stdout)[0]
	if item["response"] != "Hello world" || item["title"] != "My Title" || item["model"] != "m" {
		t.Errorf("chat send output = %v", item)
	}
	if item["thread_id"] == nil || item["thread_id"] == "" {
		t.Errorf("missing thread_id: %v", item)
	}
	// Streamed deltas went to stderr.
	if !strings.Contains(stderr, "Hello world") {
		t.Errorf("stderr should carry streamed text, got: %q", stderr)
	}

	// New-thread request shape.
	calls := s.CallsFor("runInferenceTranscript")
	var body map[string]any
	if err := json.Unmarshal(calls[len(calls)-1].Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["createThread"] != true || body["asPatchResponse"] != false {
		t.Errorf("request flags = %v", body)
	}
}
