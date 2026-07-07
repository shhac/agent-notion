package auth

import (
	"strings"
	"testing"
)

func TestImportBrowserUnknownName(t *testing.T) {
	_, err := ImportBrowser("netscape", "")
	if err == nil {
		t.Fatal("expected error for unknown browser")
	}
	if !strings.Contains(err.Error(), "unknown browser") {
		t.Errorf("error = %v", err)
	}
	// The hint should list real supported names.
	if !strings.Contains(err.Error(), "chrome") || !strings.Contains(err.Error(), "firefox") {
		t.Errorf("error should list supported browsers: %v", err)
	}
}

func TestSupportedBrowsersCoversFamilies(t *testing.T) {
	names := map[string]BrowserInfo{}
	for _, b := range SupportedBrowsers() {
		names[b.Name] = b
	}
	for _, want := range []string{"chrome", "brave", "edge", "arc", "chromium", "firefox", "zen", "safari"} {
		if _, ok := names[want]; !ok {
			t.Errorf("missing browser %q", want)
		}
	}
	// Only gecko browsers advertise profile support.
	if !names["firefox"].SupportsProfile {
		t.Error("firefox should support --profile")
	}
	if names["chrome"].SupportsProfile {
		t.Error("chrome should not advertise --profile")
	}
}
