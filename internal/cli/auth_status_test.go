package cli

import (
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/config"
)

// TestAuthStatusNeverPrintsToken pins the repo's top security invariant: the
// resolved credential's material must not appear anywhere in the output.
func TestAuthStatusNeverPrintsToken(t *testing.T) {
	isolateState(t)
	const sentinel = "ntn_sentinel_secret_value"
	if err := config.Write(config.Config{
		DefaultWorkspace: "acme",
		Workspaces: map[string]config.Workspace{
			"acme": {
				WorkspaceID: "ws-1", WorkspaceName: "Acme", BotID: "bot-1",
				AuthType: config.AuthInternalIntegration, AccessToken: sentinel,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	out, errOut, err := runCLI(t, "", "auth", "status")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["authenticated"] != true || item["source"] != "config" || item["workspace"] != "acme" {
		t.Errorf("status = %v", item)
	}
	if strings.Contains(out, sentinel) || strings.Contains(errOut, sentinel) {
		t.Fatal("auth status leaked the token")
	}
}

// TestAuthStatusReportsV3Session pins that a stored desktop session is
// reported (it's what --backend auto resolves first), not "no credential".
func TestAuthStatusReportsV3Session(t *testing.T) {
	isolateState(t)
	seedV3Session(t)

	out, _, err := runCLI(t, "", "auth", "status")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["authenticated"] != true || item["source"] != "desktop" ||
		item["auth_type"] != "desktop" || item["workspace"] != "Desk Space" {
		t.Errorf("status = %v", item)
	}
}

func TestParsePropertiesTable(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
		wantKey string
	}{
		{"empty is nil", "", false, ""},
		{"valid object", `{"Status": "Done"}`, false, "Status"},
		{"invalid JSON", `{oops`, true, ""},
		{"array is rejected", `["a"]`, true, ""},
	}
	for _, c := range cases {
		props, err := parseProperties(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if c.wantKey != "" {
			if _, ok := props[c.wantKey]; !ok {
				t.Errorf("%s: key %q missing: %v", c.name, c.wantKey, props)
			}
		} else if props != nil {
			t.Errorf("%s: expected nil map, got %v", c.name, props)
		}
	}
}
