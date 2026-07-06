package v3

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-notion/internal/mocknotion"
)

// mockClient wires a v3 Client to a mocknotion server (for streaming/thread
// tests that need RawBody NDJSON or body-conditional responses).
func mockClient(t *testing.T, s *mocknotion.Server) *Client {
	t.Helper()
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return &Client{
		HTTP:    ts.Client(),
		BaseURL: ts.URL + "/api/v3",
		TokenV2: "fake-token",
		UserID:  "user-1",
		SpaceID: "space-1",
	}
}

// =============================================================================
// AI client endpoints
// =============================================================================

func TestGetAvailableModelsEndpoint(t *testing.T) {
	resp := GetAvailableModelsResponse{Models: []AIModel{
		{Model: "oatmeal-cookie", ModelMessage: "GPT-5.2", ModelFamily: "openai", DisplayGroup: "intelligent"},
	}}
	c, cap := newTestClient(t, resp, 200)
	got, err := c.GetAvailableModels(ctx(), "space-1")
	if err != nil {
		t.Fatal(err)
	}
	if cap.body["spaceId"] != "space-1" {
		t.Errorf("spaceId = %#v", cap.body["spaceId"])
	}
	if len(got.Models) != 1 || got.Models[0].Model != "oatmeal-cookie" {
		t.Errorf("models = %#v", got.Models)
	}
}

func TestGetInferenceTranscriptsEndpoint(t *testing.T) {
	resp := GetInferenceTranscriptsResponse{
		Transcripts:     []InferenceTranscript{{ID: "t1", Title: "Chat"}},
		UnreadThreadIDs: []string{"t1"},
		HasMore:         true,
	}
	c, cap := newTestClient(t, resp, 200)
	got, err := c.GetInferenceTranscriptsForUser(ctx(), "space-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	tpp := cap.body["threadParentPointer"].(map[string]any)
	if tpp["table"] != "space" || tpp["id"] != "space-1" || tpp["spaceId"] != "space-1" {
		t.Errorf("threadParentPointer = %#v", tpp)
	}
	if cap.body["limit"] != float64(50) {
		t.Errorf("default limit = %#v, want 50", cap.body["limit"])
	}
	if len(got.Transcripts) != 1 || !got.HasMore {
		t.Errorf("resp = %#v", got)
	}
}

func TestMarkInferenceTranscriptSeenEndpoint(t *testing.T) {
	c, cap := newTestClient(t, MarkTranscriptSeenResponse{OK: true}, 200)
	got, err := c.MarkInferenceTranscriptSeen(ctx(), "space-1", "thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cap.body["spaceId"] != "space-1" || cap.body["threadId"] != "thread-1" {
		t.Errorf("body = %#v", cap.body)
	}
	if !got.OK {
		t.Error("want ok true")
	}
}

// =============================================================================
// GetThreadContent (two syncRecordValues calls, body-conditional)
// =============================================================================

func TestGetThreadContent(t *testing.T) {
	thread := map[string]any{
		"id":       "thread-1",
		"data":     map[string]any{"title": "Example Thread"},
		"messages": []any{"msg-1", "msg-2"},
	}
	userMsg := map[string]any{
		"id":           "msg-1",
		"created_time": 1700000000000,
		"step":         map[string]any{"type": "user", "value": []any{[]any{"hello there"}}},
	}
	agentMsg := map[string]any{
		"id":           "msg-2",
		"created_time": 1700000001000,
		"step":         map[string]any{"type": "agent-inference", "value": []any{map[string]any{"type": "text", "content": "hi!"}}},
	}

	t.Run("parses thread and messages", func(t *testing.T) {
		s := mocknotion.New()
		// thread_message must be matched before thread (its name contains "thread").
		s.HandleWhen("syncRecordValuesMain",
			func(body json.RawMessage) bool { return bytes.Contains(body, []byte("thread_message")) },
			mocknotion.Response{Body: mocknotion.RecordMapBody(map[string]map[string]any{
				"thread_message": {"msg-1": userMsg, "msg-2": agentMsg},
			})},
		)
		s.HandleBody("syncRecordValuesMain", mocknotion.RecordMapBody(map[string]map[string]any{
			"thread": {"thread-1": thread},
		}))

		c := mockClient(t, s)
		res, err := GetThreadContent(ctx(), c, "thread-1", "space-1")
		if err != nil {
			t.Fatal(err)
		}
		if !res.Found || res.Title != "Example Thread" {
			t.Fatalf("res = %#v", res)
		}
		eq(t, res.Messages, []ThreadMessage{
			{ID: "msg-1", Role: "user", Content: "hello there", CreatedAt: ptrInt64(1700000000000)},
			{ID: "msg-2", Role: "assistant", Content: "hi!", CreatedAt: ptrInt64(1700000001000)},
		})
	})

	t.Run("not found", func(t *testing.T) {
		s := mocknotion.New()
		s.HandleBody("syncRecordValuesMain", mocknotion.RecordMapBody(map[string]map[string]any{}))
		c := mockClient(t, s)
		res, err := GetThreadContent(ctx(), c, "thread-x", "space-1")
		if err != nil {
			t.Fatal(err)
		}
		if res.Found || len(res.Messages) != 0 {
			t.Errorf("res = %#v", res)
		}
	})

	t.Run("title with no messages", func(t *testing.T) {
		emptyThread := map[string]any{"id": "thread-1", "data": map[string]any{"title": "Empty Thread"}, "messages": []any{}}
		s := mocknotion.New()
		s.HandleBody("syncRecordValuesMain", mocknotion.RecordMapBody(map[string]map[string]any{
			"thread": {"thread-1": emptyThread},
		}))
		c := mockClient(t, s)
		res, err := GetThreadContent(ctx(), c, "thread-1", "space-1")
		if err != nil {
			t.Fatal(err)
		}
		if res.Title != "Empty Thread" || len(res.Messages) != 0 || !res.Found {
			t.Errorf("res = %#v", res)
		}
	})
}

// =============================================================================
// RunInferenceChat (streaming over NDJSON RawBody)
// =============================================================================

func TestRunInferenceChatNewThread(t *testing.T) {
	ndjson := []byte(`{"type":"agent-inference","id":"i","value":[{"type":"text","content":"Hello"}],"traceId":"t","startedAt":1,"previousAttemptValues":[]}
{"type":"title","value":"My Title"}
{"type":"agent-inference","id":"i","value":[{"type":"text","content":"Hello world"}],"traceId":"t","startedAt":1,"previousAttemptValues":[],"finishedAt":2,"inputTokens":10,"outputTokens":5,"model":"m"}
`)
	s := mocknotion.New()
	s.Handle("runInferenceTranscript", mocknotion.Response{RawBody: ndjson, Header: map[string]string{"Content-Type": "application/x-ndjson"}})
	c := mockClient(t, s)

	var chunks []string
	res, err := RunInferenceChat(ctx(), c, RunInferenceParams{
		Message: "hi",
		User:    AIUser{ID: "u1", Name: "Jane", Email: "jane@example.com"},
		Space:   AISpace{ID: "space-1", Name: "WS"},
	}, func(chunk string) { chunks = append(chunks, chunk) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Response != "Hello world" || res.Title != "My Title" || res.Model != "m" {
		t.Errorf("res = %#v", res)
	}
	if res.Tokens == nil || res.Tokens.Input != 10 {
		t.Errorf("tokens = %#v", res.Tokens)
	}
	eq(t, chunks, []string{"Hello", " world"})

	// New thread request shape.
	body := lastBody(t, s, "runInferenceTranscript")
	if body["createThread"] != true || body["asPatchResponse"] != false {
		t.Errorf("request flags = createThread:%v asPatchResponse:%v", body["createThread"], body["asPatchResponse"])
	}
}

func TestRunInferenceChatReplyPatchStream(t *testing.T) {
	// Reply threads receive patch events; RunInferenceStream normalizes them.
	ndjson := []byte(`{"type":"patch-start","data":{"s":[]},"version":1}
{"type":"patch","v":[{"o":"a","p":"/s/-","v":{"type":"agent-inference","id":"i","value":[{"type":"text","content":"Reply text"}],"traceId":"t","startedAt":1,"previousAttemptValues":[]}}]}
`)
	s := mocknotion.New()
	s.Handle("runInferenceTranscript", mocknotion.Response{RawBody: ndjson})
	c := mockClient(t, s)

	res, err := RunInferenceChat(ctx(), c, RunInferenceParams{
		Message:  "again",
		ThreadID: "thread-9",
		User:     AIUser{ID: "u1"},
		Space:    AISpace{ID: "space-1"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Response != "Reply text" {
		t.Errorf("response = %q", res.Response)
	}
	body := lastBody(t, s, "runInferenceTranscript")
	if body["asPatchResponse"] != true || body["createThread"] != false {
		t.Errorf("reply flags = %#v", body)
	}
}
