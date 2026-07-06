package oauth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser launches the system browser at url. Callers should print the
// URL as a fallback when this errors — the flow still works by manual visit.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
