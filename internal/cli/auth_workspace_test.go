package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/config"
)

// runCLI executes the root command with hermetic state; callers must have run
// isolateState first.
func runCLI(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRoot("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errBuf.String(), err
}

func isolateState(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENT_NOTION_NO_KEYCHAIN", "1")
	t.Setenv("NOTION_API_KEY", "")
	t.Setenv("NOTION_TOKEN", "")
}

// seedWorkspaces writes a config with two plaintext-token workspaces,
// "acme" (default) and "beta".
func seedWorkspaces(t *testing.T) {
	t.Helper()
	err := config.Write(config.Config{
		DefaultWorkspace: "acme",
		Workspaces: map[string]config.Workspace{
			"acme": {
				WorkspaceID:   "ws-acme",
				WorkspaceName: "Acme",
				BotID:         "bot-acme",
				AuthType:      config.AuthInternalIntegration,
				AccessToken:   "ntn_secret_acme_token",
			},
			"beta": {
				WorkspaceID:   "ws-beta",
				WorkspaceName: "Beta",
				BotID:         "bot-beta",
				AuthType:      config.AuthOAuth,
				AccessToken:   "ntn_secret_beta_token",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func decodeLines(t *testing.T, out string) []map[string]any {
	t.Helper()
	var items []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("output line not JSON: %v\n%s", err, line)
		}
		items = append(items, item)
	}
	return items
}

func TestWorkspaceListSortedWithDefaultAndNoTokens(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	out, _, err := runCLI(t, "", "auth", "workspace", "list")
	if err != nil {
		t.Fatal(err)
	}
	items := decodeLines(t, out)
	if len(items) != 2 {
		t.Fatalf("expected 2 records, got %d:\n%s", len(items), out)
	}
	if items[0]["alias"] != "acme" || items[1]["alias"] != "beta" {
		t.Errorf("aliases out of order: %v", items)
	}
	if items[0]["default"] != true || items[1]["default"] != false {
		t.Errorf("default flags wrong: %v", items)
	}
	if strings.Contains(out, "ntn_secret") {
		t.Errorf("list leaked token material:\n%s", out)
	}
}

func TestWorkspaceSwitch(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	out, _, err := runCLI(t, "", "auth", "workspace", "switch", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["default_workspace"] != "beta" {
		t.Errorf("output = %v", item)
	}
	if got := config.Read().DefaultWorkspace; got != "beta" {
		t.Errorf("default after switch = %q", got)
	}
}

func TestWorkspaceSwitchUnknownAlias(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "auth", "workspace", "switch", "ghost")
	if err == nil || !strings.Contains(err.Error(), "unknown workspace") {
		t.Errorf("err = %v", err)
	}
}

func TestWorkspaceRemoveRequiresYes(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "auth", "workspace", "remove", "acme")
	if err == nil || !strings.Contains(err.Error(), "removes the stored credentials") {
		t.Errorf("err = %v, want confirm-gate refusal", err)
	}
	if _, ok := config.Read().Workspaces["acme"]; !ok {
		t.Error("workspace removed despite missing --yes")
	}
}

func TestWorkspaceRemoveReassignsDefault(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	out, _, err := runCLI(t, "", "auth", "workspace", "remove", "acme", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["removed"] != "acme" || item["default_workspace"] != "beta" {
		t.Errorf("output = %v", item)
	}
	if item["warning"] == nil {
		t.Error("expected removed-default warning")
	}
	if _, ok := config.Read().Workspaces["acme"]; ok {
		t.Error("workspace still in config")
	}
}

func TestLogoutDefaultWorkspace(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	out, _, err := runCLI(t, "", "auth", "logout", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["removed"] != "acme" || item["default_workspace"] != "beta" {
		t.Errorf("output = %v", item)
	}
	remaining, _ := item["remaining_workspaces"].([]any)
	if len(remaining) != 1 || remaining[0] != "beta" {
		t.Errorf("remaining = %v", remaining)
	}
}

func TestLogoutSpecificWorkspaceRequiresYes(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	_, _, err := runCLI(t, "", "auth", "logout", "--workspace", "beta")
	if err == nil || !strings.Contains(err.Error(), "removes the stored credentials") {
		t.Errorf("err = %v, want confirm-gate refusal", err)
	}
	if _, ok := config.Read().Workspaces["beta"]; !ok {
		t.Error("workspace removed despite missing --yes")
	}
}

func TestLogoutAllClearsConfig(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	out, _, err := runCLI(t, "", "auth", "logout", "--all", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if item := decodeLines(t, out)[0]; item["cleared"] != "all" {
		t.Errorf("output = %v", item)
	}
	cfg := config.Read()
	if len(cfg.Workspaces) != 0 || cfg.DefaultWorkspace != "" {
		t.Errorf("config not cleared: %+v", cfg)
	}
}

func TestLogoutNothingConfigured(t *testing.T) {
	isolateState(t)

	_, _, err := runCLI(t, "", "auth", "logout", "--yes")
	if err == nil || !strings.Contains(err.Error(), "no workspaces configured") {
		t.Errorf("err = %v", err)
	}
}

func TestSetupOAuthStoresClient(t *testing.T) {
	isolateState(t)

	out, _, err := runCLI(t, "", "auth", "setup-oauth",
		"--client-id", "test-client-id", "--client-secret", "test-client-secret")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["client_id"] != "test-client-id" || item["oauth_configured"] != true {
		t.Errorf("output = %v", item)
	}
	// Keychain is disabled in tests, so the secret falls back to config and
	// the command must warn about it — but never echo the secret itself.
	if item["secret_storage"] != "config" || item["warning"] == nil {
		t.Errorf("expected config-storage warning: %v", item)
	}
	if strings.Contains(out, "test-client-secret") {
		t.Errorf("output leaked the client secret:\n%s", out)
	}

	cfg := config.Read()
	if cfg.OAuth == nil || cfg.OAuth.ClientID != "test-client-id" ||
		cfg.OAuth.ClientSecret != "test-client-secret" {
		t.Errorf("stored oauth = %+v", cfg.OAuth)
	}
}

func TestLoginWithoutOAuthConfig(t *testing.T) {
	isolateState(t)

	_, _, err := runCLI(t, "", "auth", "login")
	if err == nil || !strings.Contains(err.Error(), "OAuth not configured") {
		t.Errorf("err = %v", err)
	}
}

func TestImportRequiresToken(t *testing.T) {
	isolateState(t)

	_, _, err := runCLI(t, "", "auth", "import")
	if err == nil || !strings.Contains(err.Error(), "expected an integration token") {
		t.Errorf("err = %v", err)
	}
}
