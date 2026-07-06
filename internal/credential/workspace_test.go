package credential

import (
	"errors"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/config"
)

// fakeStore is an in-memory KeychainStore. failSet makes every Set fail so
// tests can exercise the plaintext-config fallback.
type fakeStore struct {
	data    map[string]string
	failSet bool
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string]string{}} }

func (f *fakeStore) Available() bool { return true }

func (f *fakeStore) Get(account string) (string, bool) {
	v, ok := f.data[account]
	return v, ok
}

func (f *fakeStore) Set(account, secret string) error {
	if f.failSet {
		return errors.New("keychain unavailable")
	}
	f.data[account] = secret
	return nil
}

func (f *fakeStore) Delete(account string) error {
	delete(f.data, account)
	return nil
}

func (f *fakeStore) DeleteAll() error {
	f.data = map[string]string{}
	return nil
}

func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENT_NOTION_NO_KEYCHAIN", "1")
}

func testWorkspace(id, name string) config.Workspace {
	return config.Workspace{
		WorkspaceID:   id,
		WorkspaceName: name,
		BotID:         "bot-" + id,
		AuthType:      config.AuthInternalIntegration,
		AccessToken:   "ntn_" + id,
	}
}

func TestStoreWorkspaceKeychainAndAutoDefault(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()

	storage, err := StoreWorkspace("test-ws", testWorkspace("ws-1", "Test Workspace"), kc)
	if err != nil {
		t.Fatal(err)
	}
	if storage != "keychain" {
		t.Errorf("storage = %q, want keychain", storage)
	}
	if kc.data["access_token:test-ws"] != "ntn_ws-1" {
		t.Errorf("keychain access token = %q", kc.data["access_token:test-ws"])
	}

	cfg := config.Read()
	ws, ok := cfg.Workspaces["test-ws"]
	if !ok {
		t.Fatal("workspace not stored")
	}
	if ws.AccessToken != config.KeychainPlaceholder {
		t.Errorf("config access_token = %q, want placeholder", ws.AccessToken)
	}
	if ws.WorkspaceName != "Test Workspace" || ws.AuthType != config.AuthInternalIntegration {
		t.Errorf("workspace fields = %+v", ws)
	}
	if cfg.DefaultWorkspace != "test-ws" {
		t.Errorf("default = %q, want first workspace auto-set", cfg.DefaultWorkspace)
	}
}

func TestStoreWorkspaceSecondDoesNotOverrideDefault(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()

	mustStore(t, "ws-1", testWorkspace("id-1", "First"), kc)
	mustStore(t, "ws-2", testWorkspace("id-2", "Second"), kc)

	cfg := config.Read()
	if cfg.DefaultWorkspace != "ws-1" {
		t.Errorf("default = %q, want ws-1", cfg.DefaultWorkspace)
	}
	if len(cfg.Workspaces) != 2 {
		t.Errorf("workspaces = %d, want 2", len(cfg.Workspaces))
	}
}

func TestStoreWorkspaceFallsBackToConfigPlaintext(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	kc.failSet = true

	ws := testWorkspace("id-1", "First")
	ws.AuthType = config.AuthOAuth
	ws.RefreshToken = "refresh-1"
	storage, err := StoreWorkspace("ws-1", ws, kc)
	if err != nil {
		t.Fatal(err)
	}
	if storage != "config" {
		t.Errorf("storage = %q, want config", storage)
	}
	got := config.Read().Workspaces["ws-1"]
	if got.AccessToken != "ntn_id-1" || got.RefreshToken != "refresh-1" {
		t.Errorf("plaintext tokens not stored: %+v", got)
	}
	if len(kc.data) != 0 {
		t.Errorf("partial keychain entries left behind: %v", kc.data)
	}
}

func TestRemoveWorkspaceReassignsDefault(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	mustStore(t, "ws-1", testWorkspace("id-1", "First"), kc)
	mustStore(t, "ws-2", testWorkspace("id-2", "Second"), kc)

	if err := RemoveWorkspace("ws-1", kc); err != nil {
		t.Fatal(err)
	}

	cfg := config.Read()
	if _, ok := cfg.Workspaces["ws-1"]; ok {
		t.Error("ws-1 still present")
	}
	if cfg.DefaultWorkspace != "ws-2" {
		t.Errorf("default = %q, want ws-2", cfg.DefaultWorkspace)
	}
	if _, ok := kc.data["access_token:ws-1"]; ok {
		t.Error("keychain entry for ws-1 not deleted")
	}
}

func TestRemoveWorkspaceUnknownAlias(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	mustStore(t, "ws-1", testWorkspace("id-1", "First"), kc)

	err := RemoveWorkspace("nonexistent", kc)
	if err == nil || !strings.Contains(err.Error(), "unknown workspace") {
		t.Fatalf("err = %v, want unknown workspace", err)
	}
	if !strings.Contains(err.Error(), "ws-1") {
		t.Errorf("error should list valid workspaces: %v", err)
	}
}

func TestSetDefaultWorkspace(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	mustStore(t, "ws-1", testWorkspace("id-1", "First"), kc)
	mustStore(t, "ws-2", testWorkspace("id-2", "Second"), kc)

	if err := SetDefaultWorkspace("ws-2"); err != nil {
		t.Fatal(err)
	}
	if got := config.Read().DefaultWorkspace; got != "ws-2" {
		t.Errorf("default = %q", got)
	}

	if err := SetDefaultWorkspace("nonexistent"); err == nil {
		t.Fatal("expected error for unknown alias")
	}
}

func TestClearAll(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	mustStore(t, "ws-1", testWorkspace("id-1", "First"), kc)

	if err := ClearAll(kc); err != nil {
		t.Fatal(err)
	}
	cfg := config.Read()
	if cfg.DefaultWorkspace != "" || len(cfg.Workspaces) != 0 {
		t.Errorf("config not cleared: %+v", cfg)
	}
	if len(kc.data) != 0 {
		t.Errorf("keychain not cleared: %v", kc.data)
	}
}

func TestUpdateWorkspaceTokens(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	ws := testWorkspace("id-1", "First")
	ws.AuthType = config.AuthOAuth
	ws.RefreshToken = "old_refresh"
	mustStore(t, "ws-1", ws, kc)

	if err := UpdateWorkspaceTokens("ws-1", "new_access", "new_refresh", kc); err != nil {
		t.Fatal(err)
	}

	got := config.Read().Workspaces["ws-1"]
	if got.AccessToken != config.KeychainPlaceholder || got.RefreshToken != config.KeychainPlaceholder {
		t.Errorf("config tokens = %+v, want placeholders", got)
	}
	if kc.data["access_token:ws-1"] != "new_access" || kc.data["refresh_token:ws-1"] != "new_refresh" {
		t.Errorf("keychain tokens = %v", kc.data)
	}
}

func TestUpdateWorkspaceTokensMissingWorkspaceIsNoOp(t *testing.T) {
	isolateConfig(t)
	if err := UpdateWorkspaceTokens("ghost", "a", "r", newFakeStore()); err != nil {
		t.Fatal(err)
	}
}

func TestClearWorkspaceTokens(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()
	ws := testWorkspace("id-1", "First")
	ws.AuthType = config.AuthOAuth
	ws.RefreshToken = "old_refresh"
	mustStore(t, "ws-1", ws, kc)

	if err := ClearWorkspaceTokens("ws-1", kc); err != nil {
		t.Fatal(err)
	}

	got, ok := config.Read().Workspaces["ws-1"]
	if !ok {
		t.Fatal("workspace record removed; should only clear tokens")
	}
	if got.AccessToken != "" || got.RefreshToken != "" {
		t.Errorf("tokens not cleared: %+v", got)
	}
	if len(kc.data) != 0 {
		t.Errorf("keychain entries remain: %v", kc.data)
	}
}

func TestDeriveAlias(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		want     string
	}{
		{"My Workspace", nil, "my-workspace"},
		{"Test's (Workspace)!", nil, "test-s-workspace"},
		{"", nil, "default"},
		{"test", []string{"test"}, "test-2"},
		{"test", []string{"test", "test-2"}, "test-3"},
	}
	for _, c := range cases {
		if got := DeriveAlias(c.name, c.existing); got != c.want {
			t.Errorf("DeriveAlias(%q, %v) = %q, want %q", c.name, c.existing, got, c.want)
		}
	}
	if got := DeriveAlias(strings.Repeat("a", 50), nil); len(got) > 32 {
		t.Errorf("long alias not truncated: %d chars", len(got))
	}
}

func TestStoreAndResolveOAuthConfig(t *testing.T) {
	isolateConfig(t)
	kc := newFakeStore()

	storage, err := StoreOAuthConfig("test-client-id", "test-secret", kc)
	if err != nil {
		t.Fatal(err)
	}
	if storage != "keychain" {
		t.Errorf("storage = %q, want keychain", storage)
	}

	cfg := config.Read()
	if cfg.OAuth == nil || cfg.OAuth.ClientID != "test-client-id" {
		t.Fatalf("oauth config = %+v", cfg.OAuth)
	}
	if cfg.OAuth.RedirectURI != DefaultRedirectURI {
		t.Errorf("redirect_uri = %q", cfg.OAuth.RedirectURI)
	}
	if cfg.OAuth.ClientSecret != config.KeychainPlaceholder {
		t.Errorf("client_secret = %q, want placeholder", cfg.OAuth.ClientSecret)
	}

	secret, ok := ResolveOAuthClientSecret(cfg, kc)
	if !ok || secret != "test-secret" {
		t.Errorf("resolved secret = %q ok=%v", secret, ok)
	}
}

func TestResolveOAuthClientSecretPlaintextAndMissing(t *testing.T) {
	kc := newFakeStore()

	if _, ok := ResolveOAuthClientSecret(config.Config{}, kc); ok {
		t.Error("expected no secret when oauth unconfigured")
	}

	cfg := config.Config{OAuth: &config.OAuthConfig{ClientID: "id", ClientSecret: "plain"}}
	secret, ok := ResolveOAuthClientSecret(cfg, kc)
	if !ok || secret != "plain" {
		t.Errorf("plaintext secret = %q ok=%v", secret, ok)
	}
}

func mustStore(t *testing.T, alias string, ws config.Workspace, kc KeychainStore) {
	t.Helper()
	if _, err := StoreWorkspace(alias, ws, kc); err != nil {
		t.Fatal(err)
	}
}
