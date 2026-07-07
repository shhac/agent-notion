package credential

import (
	"testing"

	"github.com/shhac/agent-notion/internal/config"
)

type fakeKeychain map[string]string

func (f fakeKeychain) Get(account string) (string, bool) {
	v, ok := f[account]
	return v, ok
}

func workspaceConfig(accessToken string) config.Config {
	return config.Config{
		DefaultWorkspace: "acme",
		Workspaces: map[string]config.Workspace{
			"acme": {
				WorkspaceID: "ws-1",
				AuthType:    config.AuthOAuth,
				AccessToken: accessToken,
			},
		},
	}
}

func TestResolveEnvironmentWins(t *testing.T) {
	t.Setenv("NOTION_API_KEY", "  secret-env  ")
	res, ok := Resolve(workspaceConfig(config.KeychainPlaceholder), fakeKeychain{})
	if !ok {
		t.Fatal("expected resolution")
	}
	if res.Key != "secret-env" {
		t.Errorf("key = %q, want trimmed env value", res.Key)
	}
	if res.Source != SourceEnvironment {
		t.Errorf("source = %q", res.Source)
	}
}

func TestResolveNotionTokenFallbackEnv(t *testing.T) {
	t.Setenv("NOTION_API_KEY", "")
	t.Setenv("NOTION_TOKEN", "secret-token")
	res, ok := Resolve(config.Config{}, fakeKeychain{})
	if !ok || res.Key != "secret-token" || res.Source != SourceEnvironment {
		t.Fatalf("got %+v ok=%v", res, ok)
	}
}

func TestResolveKeychainPlaceholder(t *testing.T) {
	t.Setenv("NOTION_API_KEY", "")
	t.Setenv("NOTION_TOKEN", "")
	kc := fakeKeychain{"access_token:acme": "secret-kc"}
	res, ok := Resolve(workspaceConfig(config.KeychainPlaceholder), kc)
	if !ok {
		t.Fatal("expected resolution")
	}
	if res.Key != "secret-kc" || res.Source != SourceKeychain {
		t.Errorf("got %+v", res)
	}
	if res.Workspace != "acme" || res.AuthType != config.AuthOAuth {
		t.Errorf("provenance = %+v", res)
	}
}

func TestResolveConfigPlaintext(t *testing.T) {
	t.Setenv("NOTION_API_KEY", "")
	t.Setenv("NOTION_TOKEN", "")
	res, ok := Resolve(workspaceConfig("secret-plain"), fakeKeychain{})
	if !ok || res.Key != "secret-plain" || res.Source != SourceConfig {
		t.Fatalf("got %+v ok=%v", res, ok)
	}
}

func TestResolveMissingWhenPlaceholderButKeychainEmpty(t *testing.T) {
	t.Setenv("NOTION_API_KEY", "")
	t.Setenv("NOTION_TOKEN", "")
	if _, ok := Resolve(workspaceConfig(config.KeychainPlaceholder), fakeKeychain{}); ok {
		t.Fatal("expected no resolution when keychain lacks the entry")
	}
}

func TestResolveNoDefaultWorkspace(t *testing.T) {
	t.Setenv("NOTION_API_KEY", "")
	t.Setenv("NOTION_TOKEN", "")
	if _, ok := Resolve(config.Config{}, fakeKeychain{}); ok {
		t.Fatal("expected no resolution with empty config")
	}
}

func TestResolveDefaultWorkspaceMissingFromMap(t *testing.T) {
	t.Setenv("NOTION_API_KEY", "")
	t.Setenv("NOTION_TOKEN", "")
	cfg := config.Config{DefaultWorkspace: "ghost"}
	if _, ok := Resolve(cfg, fakeKeychain{}); ok {
		t.Fatal("expected no resolution when default workspace is absent")
	}
}
