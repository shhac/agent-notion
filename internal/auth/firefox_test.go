package auth

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newProfile creates a Firefox profile dir named profileName under base; when
// cookieValue is non-empty it also writes a cookies.sqlite with a token_v2 row.
func newProfile(t *testing.T, base, profileName, cookieValue string) string {
	t.Helper()
	dir := filepath.Join(base, profileName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if cookieValue != "" {
		writeMozCookies(t, filepath.Join(dir, "cookies.sqlite"), cookieValue)
	}
	return dir
}

func writeMozCookies(t *testing.T, path, value string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE moz_cookies (host TEXT, name TEXT, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO moz_cookies (host, name, value) VALUES ('.notion.so', 'token_v2', ?)`, value); err != nil {
		t.Fatal(err)
	}
}

func TestGeckoRank(t *testing.T) {
	cases := map[string]int{
		"/x/abc.default-release": 0,
		"/x/abc.default":         1,
		"/x/abc.dev-edition":     2,
		"/x/plain":               2,
	}
	for path, want := range cases {
		if got := geckoRank(path); got != want {
			t.Errorf("geckoRank(%q) = %d, want %d", path, got, want)
		}
	}
}

func TestGeckoProfilesOrdersDefaultFirst(t *testing.T) {
	base := t.TempDir()
	newProfile(t, base, "ccc.other", "")
	newProfile(t, base, "bbb.default", "")
	newProfile(t, base, "aaa.default-release", "")

	profiles, err := geckoProfiles(base, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Fatalf("profiles = %v", profiles)
	}
	if filepath.Base(profiles[0]) != "aaa.default-release" || filepath.Base(profiles[1]) != "bbb.default" {
		t.Errorf("order = %v, want default-release then default first", profiles)
	}
}

func TestGeckoProfilesExactAndSuffixMatch(t *testing.T) {
	base := t.TempDir()
	newProfile(t, base, "work", "")        // exact
	newProfile(t, base, "abcd.work", "")   // ".work" suffix
	newProfile(t, base, "workspace", "")   // not a match
	newProfile(t, base, "xyz.default", "") // unrelated

	profiles, err := geckoProfiles(base, "work")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range profiles {
		got[filepath.Base(p)] = true
	}
	if !got["work"] || !got["abcd.work"] {
		t.Errorf("want work + abcd.work, got %v", profiles)
	}
	if got["workspace"] || got["xyz.default"] {
		t.Errorf("unexpected profile matched: %v", profiles)
	}
}

// TestGeckoProfilesZenNaming covers Zen's profile directory names, which are
// title-cased and parenthesized (e.g. "abcd0000.Default (release)") rather
// than Firefox's "xxxx.default-release" — they must still be discovered.
func TestGeckoProfilesZenNaming(t *testing.T) {
	base := t.TempDir()
	newProfile(t, base, "abcd0000.Default (release)", "")
	newProfile(t, base, "efgh1111.Default Profile", "")

	profiles, err := geckoProfiles(base, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range profiles {
		got[filepath.Base(p)] = true
	}
	if !got["abcd0000.Default (release)"] || !got["efgh1111.Default Profile"] {
		t.Errorf("Zen profiles not discovered: %v", profiles)
	}
}

func TestGeckoProfilesErrors(t *testing.T) {
	if _, err := geckoProfiles(filepath.Join(t.TempDir(), "missing"), ""); err == nil {
		t.Error("expected error for a missing base dir")
	}
	base := t.TempDir()
	newProfile(t, base, "plainname", "") // no dot => not a profile
	if _, err := geckoProfiles(base, ""); err == nil {
		t.Error("expected error when no profile directory matches")
	}
}

func TestGeckoCookieReadsAndDecodes(t *testing.T) {
	base := t.TempDir()
	profile := newProfile(t, base, "abc.default-release", "v02%3Agecko_token%3A99")

	got, ok := geckoCookie(profile)
	if !ok || got != "v02:gecko_token:99" {
		t.Errorf("geckoCookie = %q, %v", got, ok)
	}

	// A profile without cookies.sqlite yields no cookie.
	empty := newProfile(t, base, "def.default", "")
	if _, ok := geckoCookie(empty); ok {
		t.Error("expected no cookie for a profile without cookies.sqlite")
	}
}

func TestExtractGecko(t *testing.T) {
	base := t.TempDir()
	newProfile(t, base, "abc.default-release", "v02%3Aextracted")

	sess, err := extractGecko(base, "")
	if err != nil {
		t.Fatal(err)
	}
	if sess.TokenV2 != "v02:extracted" {
		t.Errorf("token = %q", sess.TokenV2)
	}
	if sess.Source["profile"] == "" {
		t.Error("expected a profile source annotation")
	}

	// A profile with no token_v2 cookie surfaces the not-found error.
	empty := t.TempDir()
	newProfile(t, empty, "x.default-release", "")
	// Give it an empty cookies.sqlite so the profile is discovered but yields nothing.
	writeMozCookiesEmpty(t, filepath.Join(empty, "x.default-release", "cookies.sqlite"))
	if _, err := extractGecko(empty, ""); err == nil {
		t.Error("expected a not-found error when no token cookie exists")
	}
}

func writeMozCookiesEmpty(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE moz_cookies (host TEXT, name TEXT, value TEXT)`); err != nil {
		t.Fatal(err)
	}
}

func TestGeckoBaseDir(t *testing.T) {
	got, err := geckoBaseDir("FFDarwin", "FFLinux", "FFWin")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"darwin": "FFDarwin", "linux": "FFLinux", "windows": "FFWin"}[runtime.GOOS]
	if want != "" && !strings.Contains(got, want) {
		t.Errorf("geckoBaseDir on %s = %q, want to contain %q", runtime.GOOS, got, want)
	}
}
