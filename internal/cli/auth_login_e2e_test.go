package cli

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/config"
)

// TestOAuthLoginEndToEnd drives the whole login flow: the browser seam
// "authorizes" by hitting the callback server, the code is exchanged at the
// mock token endpoint, and the workspace lands in config with the owner
// mapped — the wiring that individual oauth/credential tests don't cover.
func TestOAuthLoginEndToEnd(t *testing.T) {
	isolateState(t)
	s, serverURL := newMockServer(t)
	s.HandleBody("POST /v1/oauth/token", map[string]any{
		"access_token": "oauth-access", "refresh_token": "oauth-refresh",
		"bot_id": "bot-1", "workspace_id": "ws-1", "workspace_name": "OAuth Space",
		"owner": map[string]any{
			"type": "user",
			"user": map[string]any{
				"id": "user-1", "name": "Test User",
				"person": map[string]any{"email": "test@example.com"},
			},
		},
	})

	if _, _, err := runCLI(t, "", "auth", "setup-oauth",
		"--client-id", "client-id", "--client-secret", "client-secret"); err != nil {
		t.Fatal(err)
	}

	// The browser seam plays the user: it parses the authorize URL and hits
	// the local callback with a code and the expected state.
	openBrowser := func(authURL string) error {
		parsed, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		q := parsed.Query()
		callback := q.Get("redirect_uri") + "?code=test-auth-code&state=" + q.Get("state")
		go func() {
			resp, err := http.Get(callback)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}

	out, _, err := runCLIWithDeps(t, rootDeps{openBrowser: openBrowser}, "",
		"--base-url", serverURL, "auth", "login", "--port", "0")
	if err != nil {
		t.Fatal(err)
	}

	item := decodeLines(t, out)[0]
	ws := item["workspace"].(map[string]any)
	if ws["alias"] != "oauth-space" || ws["name"] != "OAuth Space" || ws["default"] != true {
		t.Errorf("workspace = %v", ws)
	}
	if strings.Contains(out, "oauth-access") || strings.Contains(out, "oauth-refresh") {
		t.Error("login output leaked token material")
	}

	stored := config.Read().Workspaces["oauth-space"]
	if stored.AccessToken != "oauth-access" || stored.RefreshToken != "oauth-refresh" {
		t.Errorf("stored tokens = %+v", stored)
	}
	if stored.Owner == nil || stored.Owner.User.Email != "test@example.com" {
		t.Errorf("owner = %+v", stored.Owner)
	}

	calls := s.CallsFor("POST /v1/oauth/token")
	if len(calls) != 1 {
		t.Fatalf("token exchanges = %d", len(calls))
	}
	wantBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("client-id:client-secret"))
	if got := calls[0].Header.Get("Authorization"); got != wantBasic {
		t.Errorf("exchange Authorization = %q", got)
	}
	if !strings.Contains(string(calls[0].Body), "test-auth-code") {
		t.Errorf("exchange body missing the code: %s", calls[0].Body)
	}
}

func TestOwnerFromTokenNil(t *testing.T) {
	if ownerFromToken(nil) != nil {
		t.Error("nil owner should map to nil")
	}
}
