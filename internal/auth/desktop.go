package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// desktopSafeStorageQueries are the macOS keychain services holding the Notion
// Desktop app's cookie-encryption password.
var desktopSafeStorageQueries = []safeStorageQuery{
	{service: "Notion Safe Storage"},
	{service: "Chrome Safe Storage"},
	{service: "Chromium Safe Storage"},
}

// desktopCookiePaths returns candidate Cookies DB paths for the Notion Desktop
// (Electron) app, most-specific first.
func desktopCookiePaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var bases []string
	switch runtime.GOOS {
	case "darwin":
		bases = []string{filepath.Join(home, "Library", "Application Support", "Notion")}
	case "linux":
		bases = []string{filepath.Join(home, ".config", "Notion")}
	case "windows":
		bases = []string{filepath.Join(windowsAppData(home), "Notion")}
	default:
		return nil, errors.New("unsupported OS for Notion Desktop extraction")
	}

	var paths []string
	for _, base := range bases {
		// Notion Desktop uses an Electron partition; also try the plain layout.
		paths = append(paths,
			filepath.Join(base, "Partitions", "notion", "Cookies"),
			filepath.Join(base, "Network", "Cookies"),
			filepath.Join(base, "Cookies"),
		)
	}
	return paths, nil
}

// ExtractDesktop reads the token_v2 cookie from the Notion Desktop app.
func ExtractDesktop() (*Session, error) {
	paths, err := desktopCookiePaths()
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		tok, err := extractChromiumCookie(path, desktopSafeStorageQueries)
		if err != nil {
			lastErr = err
			continue
		}
		return &Session{TokenV2: tok, Source: map[string]string{"cookies_path": path}}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("Notion Desktop cookie store not found; is the Notion app installed and signed in?")
}
