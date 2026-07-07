package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	browsercookies "github.com/shhac/lib-agent-browsercookies"
)

// desktopSafeStorageServices are the macOS keychain services holding the Notion
// Desktop app's cookie-encryption password, most-specific first.
var desktopSafeStorageServices = []string{
	"Notion Safe Storage",
	"Chrome Safe Storage",
	"Chromium Safe Storage",
}

// desktopCookiePaths returns candidate Cookies DB paths for the Notion Desktop
// (Electron) app, most-specific first. The Electron partition layout is Notion
// policy, so it lives here rather than in the shared library.
func desktopCookiePaths(home, goos string) ([]string, error) {
	var base string
	switch goos {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support", "Notion")
	case "linux":
		base = filepath.Join(home, ".config", "Notion")
	case "windows":
		base = filepath.Join(windowsAppData(home), "Notion")
	default:
		return nil, errors.New("unsupported OS for Notion Desktop extraction")
	}
	return []string{
		filepath.Join(base, "Partitions", "notion", "Cookies"),
		filepath.Join(base, "Network", "Cookies"),
		filepath.Join(base, "Cookies"),
	}, nil
}

func windowsAppData(home string) string {
	if v := os.Getenv("APPDATA"); v != "" {
		return v
	}
	return filepath.Join(home, "AppData", "Roaming")
}

// ExtractDesktop reads the token_v2 cookie from the Notion Desktop app.
func ExtractDesktop() (*Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return extractDesktop(home, runtime.GOOS)
}

// extractDesktop is the testable core: a fake Platform lets tests drive
// extraction against a fixture store on any host.
func extractDesktop(home, goos string, opts ...browsercookies.Option) (*Session, error) {
	paths, err := desktopCookiePaths(home, goos)
	if err != nil {
		return nil, err
	}
	res, err := browsercookies.ExtractChromiumStore(
		browsercookies.ChromiumStore{Paths: paths, Services: desktopSafeStorageServices},
		notionTarget,
		opts...,
	)
	if err != nil {
		return nil, err
	}
	return &Session{TokenV2: res.Value, Source: res.Source}, nil
}
