package mocknotion

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(t *testing.T, url, path string, body string, header map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, raw
}

func TestV3RoutingQueueAndStickiness(t *testing.T) {
	s := New()
	s.Handle("loadPageChunk",
		Response{Body: map[string]any{"first": true}},
		Response{Body: map[string]any{"second": true}},
	)
	ts := httptest.NewServer(s)
	defer ts.Close()

	var got map[string]any
	for i, want := range []string{"first", "second", "second"} { // last is sticky
		_, raw := post(t, ts.URL, "/api/v3/loadPageChunk", `{"page":{"id":"page-1"}}`, nil)
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got[want] != true {
			t.Errorf("call %d: body = %v, want %q", i, got, want)
		}
	}

	calls := s.CallsFor("loadPageChunk")
	if len(calls) != 3 {
		t.Fatalf("calls = %d", len(calls))
	}
	if !bytes.Contains(calls[0].Body, []byte("page-1")) {
		t.Errorf("recorded body = %s", calls[0].Body)
	}
}

func TestOfficialRoutingByMethodAndPath(t *testing.T) {
	s := New()
	s.HandleBody("GET /v1/users/me", map[string]any{"id": "bot-1"})
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/users/me")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Contains(raw, []byte("bot-1")) {
		t.Errorf("status=%d body=%s", resp.StatusCode, raw)
	}
}

func TestUnhandledKeyReturns404(t *testing.T) {
	ts := httptest.NewServer(New())
	defer ts.Close()

	resp, raw := post(t, ts.URL, "/api/v3/search", `{}`, nil)
	if resp.StatusCode != http.StatusNotFound || !bytes.Contains(raw, []byte("mock_unhandled")) {
		t.Errorf("status=%d body=%s", resp.StatusCode, raw)
	}
}

func TestHandleWhenMatchesBodyBeforePlainQueue(t *testing.T) {
	s := New()
	s.HandleBody("syncRecordValuesMain", map[string]any{"which": "plain"})
	s.HandleWhen("syncRecordValuesMain",
		func(body json.RawMessage) bool { return bytes.Contains(body, []byte(`"discussion"`)) },
		Response{Body: map[string]any{"which": "discussion"}},
	)
	ts := httptest.NewServer(s)
	defer ts.Close()

	var got map[string]any
	_, raw := post(t, ts.URL, "/api/v3/syncRecordValuesMain", `{"requests":[{"pointer":{"table":"discussion","id":"d1"}}]}`, nil)
	_ = json.Unmarshal(raw, &got)
	if got["which"] != "discussion" {
		t.Errorf("conditional not matched: %v", got)
	}

	_, raw = post(t, ts.URL, "/api/v3/syncRecordValuesMain", `{"requests":[{"pointer":{"table":"block","id":"b1"}}]}`, nil)
	_ = json.Unmarshal(raw, &got)
	if got["which"] != "plain" {
		t.Errorf("plain queue not used: %v", got)
	}
}

func TestExpectTokenV2RejectsBadCookie(t *testing.T) {
	s := New()
	s.ExpectTokenV2 = "good-token"
	s.HandleBody("loadUserContent", map[string]any{"ok": true})
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, _ := post(t, ts.URL, "/api/v3/loadUserContent", `{}`, map[string]string{"Cookie": "token_v2=bad"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad cookie status = %d, want 401", resp.StatusCode)
	}

	resp, _ = post(t, ts.URL, "/api/v3/loadUserContent", `{}`, map[string]string{"Cookie": "token_v2=good-token"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("good cookie status = %d, want 200", resp.StatusCode)
	}
	// The rejected call must not have consumed the fixture.
	if len(s.CallsFor("loadUserContent")) != 2 {
		t.Errorf("calls = %d", len(s.CallsFor("loadUserContent")))
	}
}

func TestExpectBearerRejectsOfficialCalls(t *testing.T) {
	s := New()
	s.ExpectBearer = "ntn_good"
	s.HandleBody("GET /v1/users/me", map[string]any{"id": "bot-1"})
	ts := httptest.NewServer(s)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || !bytes.Contains(raw, []byte("unauthorized")) {
		t.Errorf("status=%d body=%s", resp.StatusCode, raw)
	}
}

func TestRawBodyServedVerbatim(t *testing.T) {
	s := New()
	ndjson := []byte("{\"type\":\"token\"}\n{\"type\":\"done\"}\n")
	s.Handle("runInference", Response{RawBody: ndjson, Header: map[string]string{"Content-Type": "application/x-ndjson"}})
	ts := httptest.NewServer(s)
	defer ts.Close()

	resp, raw := post(t, ts.URL, "/api/v3/runInference", `{}`, nil)
	if !bytes.Equal(raw, ndjson) {
		t.Errorf("raw body = %q", raw)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("content type = %q", ct)
	}
}

func TestBodyBuilders(t *testing.T) {
	body := PageChunkBody(map[string]map[string]any{
		"block": {"b1": BlockEntity("b1", "page", map[string]any{"content": []string{"b2"}})},
	})

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		RecordMap map[string]map[string]struct {
			Value map[string]any `json:"value"`
			Role  string         `json:"role"`
		} `json:"recordMap"`
		Cursor map[string]any `json:"cursor"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	entry := decoded.RecordMap["block"]["b1"]
	if entry.Role != "reader" || entry.Value["id"] != "b1" || entry.Value["type"] != "page" {
		t.Errorf("entry = %+v", entry)
	}
	if decoded.Cursor == nil {
		t.Error("cursor missing")
	}

	wrapped := RoleWrappedEntry(map[string]any{"id": "x"}, "space-1")
	if wrapped["spaceId"] != "space-1" {
		t.Errorf("wrapped = %v", wrapped)
	}
	inner, ok := wrapped["value"].(map[string]any)
	if !ok || inner["role"] != "reader" {
		t.Errorf("inner = %v", inner)
	}

}

func TestReset(t *testing.T) {
	s := New()
	s.HandleBody("search", map[string]any{})
	ts := httptest.NewServer(s)
	defer ts.Close()
	_, _ = post(t, ts.URL, "/api/v3/search", `{}`, nil)

	s.Reset()
	if len(s.Calls()) != 0 {
		t.Error("calls not cleared")
	}
	resp, _ := post(t, ts.URL, "/api/v3/search", `{}`, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("queue not cleared: status = %d", resp.StatusCode)
	}
}
