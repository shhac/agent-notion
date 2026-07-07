package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// geckoBaseDir returns the directory that holds a Firefox-family app's profile
// directories. Note the layout differs by OS: macOS and Windows nest profiles
// under a "Profiles" subdirectory (so the darwin/windows names include it),
// while Linux keeps them directly under the app dir.
func geckoBaseDir(darwin, linux, windows string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", darwin), nil
	case "linux":
		return filepath.Join(home, linux), nil
	case "windows":
		return filepath.Join(windowsAppData(home), windows), nil
	default:
		return "", errors.New("unsupported OS for gecko cookie extraction")
	}
}

// extractGecko reads token_v2 from a Firefox-family profile's cookies.sqlite.
// profile selects a specific profile directory; empty means auto-discover.
func extractGecko(baseDir, profile string) (*Session, error) {
	profiles, err := geckoProfiles(baseDir, profile)
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if tok, ok := geckoCookie(p); ok {
			return &Session{TokenV2: tok, Source: map[string]string{"profile": p}}, nil
		}
	}
	return nil, errors.New("no Notion token_v2 cookie found; sign in to notion.so in this browser and retry")
}

func geckoCookie(profilePath string) (string, bool) {
	dbPath := filepath.Join(profilePath, "cookies.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		return "", false
	}
	copyPath, cleanup, err := copySqliteForRead(dbPath)
	if err != nil {
		return "", false
	}
	defer cleanup()

	rows, err := queryReadonlySqlite(copyPath,
		"select value from moz_cookies where "+hostClause("host")+" and name = '"+cookieName+
			"' order by length(value) desc")
	if err != nil {
		return "", false
	}
	for _, row := range rows {
		// Firefox stores the cookie value verbatim; send it as-is. Notion's
		// token_v2 embeds a percent-encoded prefix (e.g. "v03%3A…") that is
		// part of the value — URL-decoding it corrupts the token.
		if v := rowString(row, "value"); v != "" {
			return v, true
		}
	}
	return "", false
}

// geckoProfiles returns candidate profile directories, default-first. When
// want is set, only that profile (by directory name) is returned.
func geckoProfiles(baseDir, want string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, errors.New("no Firefox-family profiles found on disk")
	}
	var profiles []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if want != "" {
			if name == want || strings.HasSuffix(name, "."+want) {
				profiles = append(profiles, filepath.Join(baseDir, name))
			}
			continue
		}
		if strings.Contains(name, ".") { // Firefox profile dirs look like "xxxx.default-release"
			profiles = append(profiles, filepath.Join(baseDir, name))
		}
	}
	// Prefer default-release / default profiles first.
	sort.SliceStable(profiles, func(i, j int) bool {
		return geckoRank(profiles[i]) < geckoRank(profiles[j])
	})
	if len(profiles) == 0 {
		return nil, errors.New("no matching Firefox-family profile found")
	}
	return profiles, nil
}

func geckoRank(path string) int {
	base := filepath.Base(path)
	switch {
	case strings.HasSuffix(base, ".default-release"):
		return 0
	case strings.HasSuffix(base, ".default"):
		return 1
	default:
		return 2
	}
}
