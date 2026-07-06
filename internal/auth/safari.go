package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// extractSafari reads token_v2 from Safari's Cookies.binarycookies store.
// macOS only; requires Full Disk Access to read the sandboxed container.
func extractSafari() (*Session, error) {
	if runtime.GOOS != "darwin" {
		return nil, errors.New("Safari cookie extraction is only supported on macOS")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	candidates := []string{
		filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library", "Cookies", "Cookies.binarycookies"),
		filepath.Join(home, "Library", "Cookies", "Cookies.binarycookies"),
	}

	var lastErr error
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsPermission(err) {
				lastErr = errors.New("permission denied reading Safari cookies; grant Full Disk Access to your terminal in System Settings › Privacy & Security")
			}
			continue
		}
		cookies, err := parseBinaryCookies(data)
		if err != nil {
			lastErr = err
			continue
		}
		for _, c := range cookies {
			if c.Name == cookieName && strings.Contains(c.Domain, "notion.so") && c.Value != "" {
				val, _ := decodeMaybe(c.Value)
				return &Session{TokenV2: val, Source: map[string]string{"cookies_path": path}}, nil
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no Notion token_v2 cookie found in Safari; sign in to notion.so and retry")
}
