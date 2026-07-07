package auth

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	browsercookies "github.com/shhac/lib-agent-browsercookies"
	_ "modernc.org/sqlite"
)

// writeChromiumCookieDB writes a minimal Chromium Cookies DB with one plaintext
// cookie row at <dir>/Cookies. A plaintext value keeps the fixture free of
// keychain/decryption concerns. Synthetic data only — never real tokens.
func writeChromiumCookieDB(t *testing.T, dir, host, name, value string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "Cookies"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE cookies (host_key TEXT, name TEXT, value TEXT, encrypted_value BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO cookies (host_key, name, value, encrypted_value) VALUES (?, ?, ?, ?)`,
		host, name, value, nil); err != nil {
		t.Fatal(err)
	}
}

// fakePlatform is a hermetic Platform: fake home, no env, empty keychain — so
// no test ever reads the real secret store.
func fakePlatform(goos, home string) browsercookies.Platform {
	return browsercookies.Platform{
		GOOS:     goos,
		Home:     home,
		Getenv:   func(string) string { return "" },
		Keychain: func([]string) []string { return nil },
	}
}

func chromeProfileDir(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestImportBrowserExtractsToken(t *testing.T) {
	home := t.TempDir()
	writeChromiumCookieDB(t, chromeProfileDir(t, home), ".notion.so", "token_v2", "tok-plaintext")

	sess, err := importBrowser("chrome", "", browsercookies.WithPlatform(fakePlatform("darwin", home)))
	if err != nil {
		t.Fatal(err)
	}
	if sess.TokenV2 != "tok-plaintext" {
		t.Errorf("token = %q", sess.TokenV2)
	}
	if sess.Source["cookies_path"] == "" {
		t.Error("missing provenance")
	}
}

// TestImportBrowserVerbatimAndTwoDomains pins Notion policy through the wiring:
// the value is returned byte-for-byte (no URL-decode), and the token is found on
// app.notion.com (the Desktop domain) as well as notion.so.
func TestImportBrowserVerbatimAndTwoDomains(t *testing.T) {
	home := t.TempDir()
	writeChromiumCookieDB(t, chromeProfileDir(t, home), ".app.notion.com", "token_v2", "v03%3Adesktop")

	sess, err := importBrowser("chrome", "", browsercookies.WithPlatform(fakePlatform("darwin", home)))
	if err != nil {
		t.Fatalf("app.notion.com not matched: %v", err)
	}
	if sess.TokenV2 != "v03%3Adesktop" {
		t.Errorf("expected verbatim value, got %q", sess.TokenV2)
	}
}

func TestExtractDesktopFromFixture(t *testing.T) {
	home := t.TempDir()
	partition := filepath.Join(home, "Library", "Application Support", "Notion", "Partitions", "notion")
	if err := os.MkdirAll(partition, 0o755); err != nil {
		t.Fatal(err)
	}
	writeChromiumCookieDB(t, partition, ".app.notion.com", "token_v2", "desktop-tok")

	sess, err := extractDesktop(home, "darwin", browsercookies.WithPlatform(fakePlatform("darwin", home)))
	if err != nil {
		t.Fatal(err)
	}
	if sess.TokenV2 != "desktop-tok" {
		t.Errorf("token = %q", sess.TokenV2)
	}
}

func TestDesktopCookiePathsPerOS(t *testing.T) {
	cases := map[string]string{
		"darwin": filepath.Join("/home", "Library", "Application Support", "Notion"),
		"linux":  filepath.Join("/home", ".config", "Notion"),
	}
	for goos, base := range cases {
		paths, err := desktopCookiePaths("/home", goos)
		if err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
		if paths[0] != filepath.Join(base, "Partitions", "notion", "Cookies") {
			t.Errorf("%s: first path = %q", goos, paths[0])
		}
	}
	if _, err := desktopCookiePaths("/home", "plan9"); err == nil {
		t.Error("expected an unsupported-OS error")
	}
}
