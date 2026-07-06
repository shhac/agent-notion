package official

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/me" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer ntn_test_token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Notion-Version"); got != APIVersion {
			t.Errorf("Notion-Version = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "user",
			"id":     "bot-user-1",
			"name":   "Test Integration",
			"type":   "bot",
			"bot":    map[string]any{"workspace_name": "Test Space"},
		})
	}))
	defer srv.Close()

	bot, err := Client{BaseURL: srv.URL, Token: "ntn_test_token"}.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if bot.ID != "bot-user-1" || bot.Name != "Test Integration" || bot.WorkspaceName != "Test Space" {
		t.Errorf("bot = %+v", bot)
	}
}

func TestMeRejectedToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "error", "status": 401, "code": "unauthorized"})
	}))
	defer srv.Close()

	if _, err := (Client{BaseURL: srv.URL, Token: "bad"}).Me(context.Background()); err == nil {
		t.Fatal("expected error for rejected token")
	}
}
