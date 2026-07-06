package config

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate points config at a throwaway XDG dir so tests never touch the real
// config or keychain.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AGENT_NOTION_NO_KEYCHAIN", "1")
	return filepath.Join(dir, AppName, "config.json")
}

func TestReadMissingFileIsEmpty(t *testing.T) {
	isolate(t)
	if got := Read(); got.DefaultWorkspace != "" || got.Workspaces != nil {
		t.Fatalf("expected empty config, got %+v", got)
	}
}

func TestReadCorruptFileIsEmpty(t *testing.T) {
	path := isolate(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Read(); got.DefaultWorkspace != "" {
		t.Fatalf("corrupt file should read as empty, got %+v", got)
	}
}

func TestReadExistingTSConfig(t *testing.T) {
	path := isolate(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// A config.json as the TS binary writes it, with a keychain placeholder.
	raw := `{
  "default_workspace": "acme",
  "workspaces": {
    "acme": {
      "workspace_id": "ws-1",
      "workspace_name": "Acme",
      "bot_id": "bot-1",
      "auth_type": "oauth",
      "access_token": "__KEYCHAIN__",
      "refresh_token": "__KEYCHAIN__"
    }
  }
}
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := Read()
	if cfg.DefaultWorkspace != "acme" {
		t.Fatalf("default_workspace = %q", cfg.DefaultWorkspace)
	}
	ws, ok := cfg.Workspaces["acme"]
	if !ok {
		t.Fatal("workspace acme missing")
	}
	if ws.AuthType != AuthOAuth {
		t.Errorf("auth_type = %q", ws.AuthType)
	}
	if ws.AccessToken != KeychainPlaceholder {
		t.Errorf("access_token = %q, want placeholder", ws.AccessToken)
	}
}

func TestWriteThenReadRoundTrips(t *testing.T) {
	isolate(t)
	want := Config{
		DefaultWorkspace: "acme",
		Workspaces: map[string]Workspace{
			"acme": {
				WorkspaceID:   "ws-1",
				WorkspaceName: "Acme",
				BotID:         "bot-1",
				AuthType:      AuthInternalIntegration,
				AccessToken:   KeychainPlaceholder,
			},
		},
	}
	if err := Write(want); err != nil {
		t.Fatal(err)
	}

	got := Read()
	if got.DefaultWorkspace != want.DefaultWorkspace {
		t.Errorf("default_workspace = %q", got.DefaultWorkspace)
	}
	if got.Workspaces["acme"].AuthType != AuthInternalIntegration {
		t.Errorf("auth_type = %q", got.Workspaces["acme"].AuthType)
	}

	// File must be 0600 (secrets live here).
	info, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %o, want 600", perm)
	}
}
