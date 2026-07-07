package credential

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/oauth"
)

// refreshEndpoint serves the token endpoint, capturing the refresh token it
// was sent. accept=false makes it refuse every request.
func refreshEndpoint(t *testing.T, accept bool, gotRefresh *string) oauth.TokenClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var grant map[string]string
		_ = json.NewDecoder(r.Body).Decode(&grant)
		if gotRefresh != nil {
			*gotRefresh = grant["refresh_token"]
		}
		if !accept {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
		})
	}))
	t.Cleanup(srv.Close)
	return oauth.TokenClient{URL: srv.URL}
}

// seedOAuthWorkspace stores an oauth workspace + client config through the
// real store paths so placeholders and keychain entries line up.
func seedOAuthWorkspace(t *testing.T, kc KeychainStore) {
	t.Helper()
	ws := config.Workspace{
		WorkspaceID:   "ws-1",
		WorkspaceName: "Test",
		BotID:         "bot-1",
		AuthType:      config.AuthOAuth,
		AccessToken:   "old-access",
		RefreshToken:  "old-refresh",
	}
	if _, err := StoreWorkspace("acme", ws, kc); err != nil {
		t.Fatal(err)
	}
	if _, err := StoreOAuthConfig("client-id", "client-secret", kc); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshAccessToken(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	seedOAuthWorkspace(t, kc)

	var sentRefresh string
	token, ok := RefreshAccessToken(context.Background(), "acme", kc, refreshEndpoint(t, true, &sentRefresh))
	if !ok || token != "new-access" {
		t.Fatalf("token = %q ok=%v", token, ok)
	}
	if sentRefresh != "old-refresh" {
		t.Errorf("endpoint got refresh token %q", sentRefresh)
	}
	if kc.data["access_token:acme"] != "new-access" || kc.data["refresh_token:acme"] != "new-refresh" {
		t.Errorf("keychain after refresh = %v", kc.data)
	}
	got := config.Read().Workspaces["acme"]
	if got.AccessToken != config.KeychainPlaceholder || got.RefreshToken != config.KeychainPlaceholder {
		t.Errorf("config after refresh = %+v", got)
	}
}

func TestRefreshAccessTokenNonOAuthWorkspace(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	if _, err := StoreWorkspace("acme", testWorkspace("ws-1", "Test"), kc); err != nil {
		t.Fatal(err)
	}

	if _, ok := RefreshAccessToken(context.Background(), "acme", kc, refreshEndpoint(t, true, nil)); ok {
		t.Error("expected refusal for internal_integration workspace")
	}
}

func TestRefreshAccessTokenMissingOAuthConfig(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	ws := testWorkspace("ws-1", "Test")
	ws.AuthType = config.AuthOAuth
	ws.RefreshToken = "old-refresh"
	if _, err := StoreWorkspace("acme", ws, kc); err != nil {
		t.Fatal(err)
	}

	if _, ok := RefreshAccessToken(context.Background(), "acme", kc, refreshEndpoint(t, true, nil)); ok {
		t.Error("expected refusal without stored OAuth client config")
	}
}

func TestRefreshOrRecoverRecoversFromKeychain(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	seedOAuthWorkspace(t, kc)
	// Simulate a parallel process having refreshed already: the endpoint
	// refuses, but the keychain holds a fresh token.
	kc.data["access_token:acme"] = "parallel-refreshed"

	token, ok := RefreshOrRecover(context.Background(), "acme", kc, refreshEndpoint(t, false, nil))
	if !ok || token != "parallel-refreshed" {
		t.Errorf("token = %q ok=%v", token, ok)
	}
}

func TestRefreshOrRecoverClearsTokensOnTotalFailure(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	seedOAuthWorkspace(t, kc)
	delete(kc.data, "access_token:acme")

	if _, ok := RefreshOrRecover(context.Background(), "acme", kc, refreshEndpoint(t, false, nil)); ok {
		t.Fatal("expected failure")
	}
	got := config.Read().Workspaces["acme"]
	if got.AccessToken != "" || got.RefreshToken != "" {
		t.Errorf("tokens not cleared: %+v", got)
	}
}
