package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	"github.com/shhac/agent-notion/internal/notion"
)

// testGlobals builds a GlobalFlags with production seams pointed at the
// isolated env (isolateState must have run) and the given base URL.
func testGlobals(baseURL string) *GlobalFlags {
	return &GlobalFlags{
		BaseURL:  baseURL,
		Backend:  "auto",
		keychain: credential.DefaultKeychainStore,
	}
}

func seedV3Session(t *testing.T) {
	t.Helper()
	cfg := config.Read()
	cfg.V3 = &config.V3Session{
		TokenV2:   "v2-plain-token",
		UserID:    "user-1",
		SpaceID:   "space-1",
		SpaceName: "Desk Space",
	}
	if err := config.Write(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestNewBackendAutoPrefersV3Session(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t) // official creds present too
	seedV3Session(t)

	handle, err := testGlobals("").newBackend()
	if err != nil {
		t.Fatal(err)
	}
	if handle.backend.Kind() != "v3" || handle.workspace != "Desk Space" || handle.authType != config.AuthDesktop {
		t.Errorf("handle = kind=%s workspace=%s auth=%s", handle.backend.Kind(), handle.workspace, handle.authType)
	}
}

func TestNewBackendAutoFallsBackToOfficial(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	handle, err := testGlobals("").newBackend()
	if err != nil {
		t.Fatal(err)
	}
	if handle.backend.Kind() != "official" || handle.workspace != "acme" {
		t.Errorf("handle = kind=%s workspace=%s", handle.backend.Kind(), handle.workspace)
	}
}

func TestNewBackendOfficialModeSkipsV3(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)
	seedV3Session(t)

	g := testGlobals("")
	g.Backend = "official"
	handle, err := g.newBackend()
	if err != nil {
		t.Fatal(err)
	}
	if handle.backend.Kind() != "official" {
		t.Errorf("kind = %s", handle.backend.Kind())
	}
}

func TestNewBackendV3ModeRequiresSession(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	g := testGlobals("")
	g.Backend = "v3"
	_, err := g.newBackend()
	if err == nil || !strings.Contains(err.Error(), "requires a v3 desktop session") {
		t.Errorf("err = %v", err)
	}
}

func TestNewBackendUnauthenticated(t *testing.T) {
	isolateState(t)

	_, err := testGlobals("").newBackend()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("err = %v", err)
	}
}

// TestWithBackendRefreshesOAuthTokenOn401 exercises the full refresh path:
// the stored access token is stale (the mock's bearer gate rejects it), the
// refresh endpoint issues a fresh pair, and the retried call succeeds.
func TestWithBackendRefreshesOAuthTokenOn401(t *testing.T) {
	isolateState(t)
	s, url := newMockServer(t)

	kc := credential.DefaultKeychainStore()
	if _, err := credential.StoreWorkspace("acme", config.Workspace{
		WorkspaceID: "ws-1", WorkspaceName: "Acme", BotID: "bot-1",
		AuthType: config.AuthOAuth, AccessToken: "stale-access", RefreshToken: "refresh-1",
	}, kc); err != nil {
		t.Fatal(err)
	}
	if _, err := credential.StoreOAuthConfig("client-id", "client-secret", kc); err != nil {
		t.Fatal(err)
	}

	s.ExpectBearer = "new-access"
	s.HandleBody("POST /v1/oauth/token", map[string]any{
		"access_token": "new-access", "refresh_token": "new-refresh",
	})
	s.HandleBody("GET /v1/users/me", map[string]any{
		"object": "user", "id": "bot-1", "name": "Acme Bot", "type": "bot",
	})

	me, err := withBackend(context.Background(), testGlobals(url), func(b notion.Backend) (notion.UserMe, error) {
		return b.GetMe(context.Background())
	})
	if err != nil {
		t.Fatal(err)
	}
	if me.ID != "bot-1" {
		t.Errorf("me = %+v", me)
	}

	// The refreshed pair must have been stored.
	cfg := config.Read()
	if cfg.Workspaces["acme"].AccessToken != "new-access" || cfg.Workspaces["acme"].RefreshToken != "new-refresh" {
		t.Errorf("tokens after refresh = %+v", cfg.Workspaces["acme"])
	}
	if len(s.CallsFor("GET /v1/users/me")) != 2 {
		t.Errorf("users/me calls = %d, want 2 (401 then retry)", len(s.CallsFor("GET /v1/users/me")))
	}
}

func TestWithBackendInternalIntegrationDoesNotRefresh(t *testing.T) {
	isolateState(t)
	s, url := newMockServer(t)
	seedWorkspaces(t) // acme is internal_integration with plaintext token
	s.ExpectBearer = "something-else"

	_, err := withBackend(context.Background(), testGlobals(url), func(b notion.Backend) (notion.UserMe, error) {
		return b.GetMe(context.Background())
	})
	if err == nil || !strings.Contains(err.Error(), "invalid or revoked") {
		t.Errorf("err = %v", err)
	}
	if calls := s.CallsFor("POST /v1/oauth/token"); len(calls) != 0 {
		t.Errorf("refresh attempted for internal integration: %d calls", len(calls))
	}
}
