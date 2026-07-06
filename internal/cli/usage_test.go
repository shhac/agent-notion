package cli

import (
	"strings"
	"testing"
)

func TestRootUsageCommand(t *testing.T) {
	isolateState(t)
	out, _, err := runCLI(t, "", "usage")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "agent-notion:") || !strings.Contains(out, "auth") {
		t.Errorf("root usage missing expected content:\n%s", out)
	}
}

func TestAuthUsageCommand(t *testing.T) {
	isolateState(t)
	out, _, err := runCLI(t, "", "auth", "usage")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"setup-oauth", "login", "import", "logout", "workspace", "import-desktop"} {
		if !strings.Contains(out, want) {
			t.Errorf("auth usage missing %q", want)
		}
	}
}
