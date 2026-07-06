package credential

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shhac/agent-notion/internal/config"
)

type fakeKeychainWriter struct {
	available bool
	setErr    error
	stored    map[string]string
}

func (f *fakeKeychainWriter) Available() bool { return f.available }
func (f *fakeKeychainWriter) Set(account, secret string) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.stored == nil {
		f.stored = map[string]string{}
	}
	f.stored[account] = secret
	return nil
}

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AGENT_NOTION_NO_KEYCHAIN", "1")
}

func TestStoreV3SessionUsesKeychainWhenAvailable(t *testing.T) {
	isolate(t)
	kc := &fakeKeychainWriter{available: true}

	storage, err := StoreV3Session(config.V3Session{TokenV2: "tok-secret", UserID: "u1", SpaceName: "Acme"}, kc)
	if err != nil {
		t.Fatal(err)
	}
	if storage != "keychain" {
		t.Errorf("storage = %q", storage)
	}
	if kc.stored["v3:token_v2"] != "tok-secret" {
		t.Errorf("keychain got %q", kc.stored["v3:token_v2"])
	}

	// Config stores the placeholder, not the secret.
	cfg := config.Read()
	if cfg.V3 == nil || cfg.V3.TokenV2 != config.KeychainPlaceholder {
		t.Errorf("config v3 = %+v", cfg.V3)
	}
	if cfg.V3.SpaceName != "Acme" {
		t.Errorf("session info not persisted: %+v", cfg.V3)
	}
}

func TestStoreV3SessionFallsBackToConfig(t *testing.T) {
	isolate(t)
	kc := &fakeKeychainWriter{available: false}

	storage, err := StoreV3Session(config.V3Session{TokenV2: "tok-plain"}, kc)
	if err != nil {
		t.Fatal(err)
	}
	if storage != "config" {
		t.Errorf("storage = %q", storage)
	}
	cfg := config.Read()
	if cfg.V3.TokenV2 != "tok-plain" {
		t.Errorf("expected plaintext token in config, got %q", cfg.V3.TokenV2)
	}

	// And the file must be 0600.
	info, err := os.Stat(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "agent-notion", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o", info.Mode().Perm())
	}
}

func TestStoreV3SessionKeychainErrorFallsBack(t *testing.T) {
	isolate(t)
	kc := &fakeKeychainWriter{available: true, setErr: errors.New("keychain locked")}

	storage, err := StoreV3Session(config.V3Session{TokenV2: "tok"}, kc)
	if err != nil {
		t.Fatal(err)
	}
	if storage != "config" {
		t.Errorf("storage = %q (should fall back on Set error)", storage)
	}
}

func TestResolveV3Token(t *testing.T) {
	kc := fakeKeychain{"v3:token_v2": "resolved-secret"}

	got, ok := ResolveV3Token(config.Config{V3: &config.V3Session{TokenV2: config.KeychainPlaceholder}}, kc)
	if !ok || got != "resolved-secret" {
		t.Errorf("placeholder resolution: %q %v", got, ok)
	}

	got, ok = ResolveV3Token(config.Config{V3: &config.V3Session{TokenV2: "plain"}}, kc)
	if !ok || got != "plain" {
		t.Errorf("plaintext resolution: %q %v", got, ok)
	}

	if _, ok := ResolveV3Token(config.Config{}, kc); ok {
		t.Error("no session should resolve to false")
	}
}
