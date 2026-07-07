package auth

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newCookiesDB builds a Chromium Cookies SQLite DB in dir and returns its path.
// meta.version is written only when metaVersion > 0.
func newCookiesDB(t *testing.T, dir string, metaVersion int, host, value string, encrypted []byte) string {
	t.Helper()
	path := filepath.Join(dir, "Cookies")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	exec := func(q string, args ...any) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	exec(`CREATE TABLE meta (key TEXT, value TEXT)`)
	if metaVersion > 0 {
		exec(`INSERT INTO meta (key, value) VALUES ('version', ?)`, metaVersion)
	}
	exec(`CREATE TABLE cookies (host_key TEXT, name TEXT, value TEXT, encrypted_value BLOB)`)
	exec(`INSERT INTO cookies (host_key, name, value, encrypted_value) VALUES (?, 'token_v2', ?, ?)`,
		host, value, encrypted)
	return path
}

// withStubPasswords swaps the Safe Storage seam for the test's duration.
func withStubPasswords(t *testing.T, passwords ...string) {
	t.Helper()
	prev := safeStoragePasswordsFn
	safeStoragePasswordsFn = func([]safeStorageQuery, string) []string { return passwords }
	t.Cleanup(func() { safeStoragePasswordsFn = prev })
}

func TestExtractChromiumCookiePlaintext(t *testing.T) {
	dir := t.TempDir()
	path := newCookiesDB(t, dir, 0, ".notion.so", "v02:plaintext_token", nil)

	got, err := extractChromiumCookie(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v02:plaintext_token" {
		t.Errorf("plaintext cookie = %q", got)
	}
}

// TestExtractChromiumCookieAppNotionCom pins the desktop-app fix: Notion's
// current domain is app.notion.com (the Desktop app's only token host), which
// the old "%notion.so"-only host filter missed entirely.
func TestExtractChromiumCookieAppNotionCom(t *testing.T) {
	path := newCookiesDB(t, t.TempDir(), 0, ".app.notion.com", "v03%3Adesktop_token", nil)

	got, err := extractChromiumCookie(path, nil)
	if err != nil {
		t.Fatalf("app.notion.com host not matched: %v", err)
	}
	if got != "v03%3Adesktop_token" {
		t.Errorf("cookie = %q, want verbatim", got)
	}
}

func TestExtractChromiumCookieCBCRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses the DPAPI branch, not CBC")
	}
	const token = "v02%3Auser_token%3Aabc123" // percent-encoding is part of the token
	enc := append([]byte("v10"), encryptCBC(t, []byte(token), "keychain-pass", chromiumIterations())...)
	path := newCookiesDB(t, t.TempDir(), 20, ".notion.so", "", enc)

	withStubPasswords(t, "keychain-pass")
	got, err := extractChromiumCookie(path, []safeStorageQuery{{service: "Notion Safe Storage"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v02%3Auser_token%3Aabc123" {
		t.Errorf("decrypted cookie = %q", got)
	}
}

func TestExtractChromiumCookieStripsV24Prefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses the DPAPI branch, not CBC")
	}
	// meta.version >= 24 => decrypted value carries a 32-byte SHA-256(host) prefix.
	plain := append(make([]byte, 32), []byte("real-token")...)
	enc := append([]byte("v10"), encryptCBC(t, plain, "pw", chromiumIterations())...)
	path := newCookiesDB(t, t.TempDir(), 24, ".notion.so", "", enc)

	withStubPasswords(t, "pw")
	got, err := extractChromiumCookie(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "real-token" {
		t.Errorf("v24-stripped cookie = %q", got)
	}
}

func TestExtractChromiumCookieRetriesPasswords(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses the DPAPI branch, not CBC")
	}
	enc := append([]byte("v10"), encryptCBC(t, []byte("the-token"), "second-pw", chromiumIterations())...)
	path := newCookiesDB(t, t.TempDir(), 0, ".notion.so", "", enc)

	// The first (wrong) password fails PKCS#7 unpadding; the loop tries the next.
	withStubPasswords(t, "first-wrong-pw", "second-pw")
	got, err := extractChromiumCookie(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "the-token" {
		t.Errorf("retry-decrypted cookie = %q", got)
	}
}

func TestExtractChromiumCookieNoRows(t *testing.T) {
	// A cookies table with no token_v2 row.
	dir := t.TempDir()
	path := filepath.Join(dir, "Cookies")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE cookies (host_key TEXT, name TEXT, value TEXT, encrypted_value BLOB)`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	_, err = extractChromiumCookie(path, nil)
	if err == nil || !strings.Contains(err.Error(), "no Notion token_v2 cookie") {
		t.Errorf("err = %v, want no-cookie", err)
	}
}

func TestExtractChromiumCookieNoPasswords(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses the DPAPI branch, not CBC")
	}
	enc := append([]byte("v10"), encryptCBC(t, []byte("t"), "pw", chromiumIterations())...)
	path := newCookiesDB(t, t.TempDir(), 0, ".notion.so", "", enc)

	withStubPasswords(t) // no passwords available
	_, err := extractChromiumCookie(path, nil)
	if err == nil || !strings.Contains(err.Error(), "Safe Storage password") {
		t.Errorf("err = %v, want no-password", err)
	}
}

func TestReadCookieMetaVersion(t *testing.T) {
	t.Run("string row", func(t *testing.T) {
		path := newCookiesDB(t, t.TempDir(), 0, ".notion.so", "x", nil)
		writeMetaString(t, path, "value TEXT", "'24'")
		if v := readCookieMetaVersion(path); v != 24 {
			t.Errorf("string version = %d", v)
		}
	})
	t.Run("int64 row", func(t *testing.T) {
		path := newCookiesDB(t, t.TempDir(), 0, ".notion.so", "x", nil)
		writeMetaString(t, path, "value INTEGER", "24")
		if v := readCookieMetaVersion(path); v != 24 {
			t.Errorf("int64 version = %d", v)
		}
	})
	t.Run("absent", func(t *testing.T) {
		path := newCookiesDB(t, t.TempDir(), 0, ".notion.so", "x", nil)
		if v := readCookieMetaVersion(path); v != 0 {
			t.Errorf("absent version = %d, want 0", v)
		}
	})
}

// writeMetaString rebuilds the meta table with a chosen column type so the
// driver returns the value as a string vs int64, then inserts one version row.
func writeMetaString(t *testing.T, path, valueColDef, literal string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, q := range []string{
		`DROP TABLE IF EXISTS meta`,
		`CREATE TABLE meta (key TEXT, ` + valueColDef + `)`,
		`INSERT INTO meta (key, value) VALUES ('version', ` + literal + `)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
}
