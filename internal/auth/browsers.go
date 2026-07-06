package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// windowsAppData returns %APPDATA% (roaming), falling back to a conventional
// path under home.
func windowsAppData(home string) string {
	if v := os.Getenv("APPDATA"); v != "" {
		return v
	}
	return filepath.Join(home, "AppData", "Roaming")
}

// windowsLocalAppData returns %LOCALAPPDATA%, falling back under home.
func windowsLocalAppData(home string) string {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return v
	}
	return filepath.Join(home, "AppData", "Local")
}

// chromiumPaths describes where a Chromium-family browser keeps its per-OS
// user-data directory (relative to the platform's app-support root). The
// Cookies DB is resolved under <userData>/<profile>/Network/Cookies (newer) or
// <userData>/<profile>/Cookies (older).
type chromiumPaths struct {
	darwin  string
	linux   string
	windows string
	profile string // "" → "Default"
	queries []safeStorageQuery
}

func (p chromiumPaths) userDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", p.darwin), nil
	case "linux":
		return filepath.Join(home, p.linux), nil
	case "windows":
		return filepath.Join(windowsLocalAppData(home), p.windows), nil
	default:
		return "", errors.New("unsupported OS for Chromium cookie extraction")
	}
}

// cookiesDB resolves the first existing Cookies database path.
func (p chromiumPaths) cookiesDB() (string, error) {
	userData, err := p.userDataDir()
	if err != nil {
		return "", err
	}
	profile := p.profile
	if profile == "" {
		profile = "Default"
	}
	candidates := []string{
		filepath.Join(userData, profile, "Network", "Cookies"),
		filepath.Join(userData, profile, "Cookies"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", errors.New("could not find this browser's Cookies database (is it installed and signed in?)")
}

func extractChromium(p chromiumPaths) (*Session, error) {
	cookiesPath, err := p.cookiesDB()
	if err != nil {
		return nil, err
	}
	tok, err := extractChromiumCookie(cookiesPath, p.queries)
	if err != nil {
		return nil, err
	}
	return &Session{TokenV2: tok, Source: map[string]string{"cookies_path": cookiesPath}}, nil
}

// chromiumSafeStorage returns the macOS keychain queries for a browser, always
// falling back to the generic Chrome/Chromium services.
func chromiumSafeStorage(services ...string) []safeStorageQuery {
	q := make([]safeStorageQuery, 0, len(services)+2)
	for _, s := range services {
		q = append(q, safeStorageQuery{service: s})
	}
	return append(q,
		safeStorageQuery{service: "Chrome Safe Storage"},
		safeStorageQuery{service: "Chromium Safe Storage"},
	)
}
