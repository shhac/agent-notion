package cli

import (
	"bytes"
	"encoding/json"
	"errors"

	output "github.com/shhac/lib-agent-output"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/auth"
	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	"github.com/shhac/agent-notion/internal/mocknotion"
)

// runCLIWithDeps executes a root built with injected seams; callers must have
// run isolateState first. Zero-value deps fields fall back to production
// implementations.
func runCLIWithDeps(t *testing.T, deps rootDeps, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	if deps.version == "" {
		deps.version = "test"
	}
	if deps.keychain == nil {
		deps.keychain = credential.DefaultKeychainStore
	}
	if deps.desktopExtract == nil {
		deps.desktopExtract = func() (*auth.Session, error) {
			return nil, errors.New("desktop extraction unavailable in tests")
		}
	}
	if deps.browserImport == nil {
		deps.browserImport = func(string, string) (*auth.Session, error) {
			return nil, errors.New("browser import unavailable in tests")
		}
	}
	if deps.openBrowser == nil {
		deps.openBrowser = func(string) error {
			return errors.New("no browser in tests")
		}
	}
	root := newRootWithDeps(deps)
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errBuf.String(), err
}

func newMockServer(t *testing.T) (*mocknotion.Server, string) {
	t.Helper()
	s := mocknotion.New()
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return s, ts.URL
}

func TestAuthImportAgainstMockNotion(t *testing.T) {
	isolateState(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/users/me", map[string]any{
		"object": "user", "id": "bot-1", "name": "Test Integration",
		"type": "bot", "bot": map[string]any{"workspace_name": "Test Space"},
	})

	out, _, err := runCLI(t, "", "--base-url", url, "auth", "import", "--token", "ntn_test_token_123")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	ws := item["workspace"].(map[string]any)
	if ws["alias"] != "test-space" || ws["name"] != "Test Space" || ws["default"] != true {
		t.Errorf("workspace = %v", ws)
	}

	calls := s.CallsFor("GET /v1/users/me")
	if len(calls) != 1 {
		t.Fatalf("users/me calls = %d", len(calls))
	}
	if got := calls[0].Header.Get("Authorization"); got != "Bearer ntn_test_token_123" {
		t.Errorf("Authorization = %q", got)
	}

	cfg := config.Read()
	stored := cfg.Workspaces["test-space"]
	if stored.AccessToken != "ntn_test_token_123" || stored.AuthType != config.AuthInternalIntegration {
		t.Errorf("stored workspace = %+v", stored)
	}
}

// TestAuthImportReadsTokenFromStdin covers the non-interactive machine form:
// the secret is piped on stdin (kept off argv) rather than passed via --token.
func TestAuthImportReadsTokenFromStdin(t *testing.T) {
	isolateState(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/users/me", map[string]any{
		"object": "user", "id": "bot-1", "name": "Test Integration",
		"type": "bot", "bot": map[string]any{"workspace_name": "Test Space"},
	})

	out, _, err := runCLI(t, "  ntn_piped_token_123\n", "--base-url", url, "auth", "import")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	ws := item["workspace"].(map[string]any)
	if ws["alias"] != "test-space" || ws["name"] != "Test Space" {
		t.Errorf("workspace = %v", ws)
	}

	calls := s.CallsFor("GET /v1/users/me")
	if len(calls) != 1 {
		t.Fatalf("users/me calls = %d", len(calls))
	}
	if got := calls[0].Header.Get("Authorization"); got != "Bearer ntn_piped_token_123" {
		t.Errorf("Authorization = %q (stdin token not trimmed/forwarded)", got)
	}
	if got := config.Read().Workspaces["test-space"].AccessToken; got != "ntn_piped_token_123" {
		t.Errorf("stored token = %q", got)
	}
}

// TestAuthImportFlagWinsOverStdin pins the precedence: an explicit --token
// takes precedence over whatever is piped on stdin.
func TestAuthImportFlagWinsOverStdin(t *testing.T) {
	isolateState(t)
	s, url := newMockServer(t)
	s.HandleBody("GET /v1/users/me", map[string]any{
		"object": "user", "id": "bot-1", "name": "Test Integration",
		"type": "bot", "bot": map[string]any{"workspace_name": "Test Space"},
	})

	_, _, err := runCLI(t, "ntn_stdin_token", "--base-url", url, "auth", "import", "--token", "ntn_flag_token")
	if err != nil {
		t.Fatal(err)
	}
	calls := s.CallsFor("GET /v1/users/me")
	if len(calls) != 1 {
		t.Fatalf("users/me calls = %d", len(calls))
	}
	if got := calls[0].Header.Get("Authorization"); got != "Bearer ntn_flag_token" {
		t.Errorf("Authorization = %q, want the --token value to win", got)
	}
}

func TestImportDesktopViaSeamAndMockValidation(t *testing.T) {
	isolateState(t)
	s, url := newMockServer(t)
	s.HandleBody("getSpaces", mocknotion.GetSpacesBody("user-1", map[string]map[string]any{
		"notion_user": {
			"user-1": map[string]any{"id": "user-1", "email": "test@example.com", "name": "Test User"},
		},
		"space": {
			"space-1": map[string]any{"id": "space-1", "name": "Test Space", "plan_type": "team"},
		},
		"space_view": {},
	}))

	deps := rootDeps{desktopExtract: func() (*auth.Session, error) {
		return &auth.Session{TokenV2: "v2-test-token", Source: map[string]string{"path": "fake"}}, nil
	}}
	out, _, err := runCLIWithDeps(t, deps, "", "--base-url", url, "auth", "import-desktop")
	if err != nil {
		t.Fatal(err)
	}
	item := decodeLines(t, out)[0]
	if item["user"] != "Test User" || item["space"] != "Test Space" || item["storage"] != "config" {
		t.Errorf("import output = %v", item)
	}
	if strings.Contains(out, "v2-test-token") {
		t.Error("output leaked the token")
	}

	if got, _ := credential.ResolveV3Token(config.Read(), credential.DefaultKeychainStore()); got != "v2-test-token" {
		t.Errorf("stored v3 token = %q", got)
	}
}

// An expired/revoked token fails getSpaces with a 401: import must surface
// the re-import hint and store nothing.
func TestImportDesktopValidationFailure(t *testing.T) {
	isolateState(t)
	s, url := newMockServer(t)
	s.Handle("getSpaces", mocknotion.Response{Status: 401, Body: map[string]any{
		"errorId": "mock", "name": "UnauthorizedError",
	}})

	deps := rootDeps{desktopExtract: func() (*auth.Session, error) {
		return &auth.Session{TokenV2: "v2-expired-token", Source: map[string]string{"path": "fake"}}, nil
	}}
	_, _, err := runCLIWithDeps(t, deps, "", "--base-url", url, "auth", "import-desktop")
	if err == nil {
		t.Fatal("expected import to fail on 401 validation")
	}
	if !strings.Contains(err.Error(), "token validation failed: HTTP 401") {
		t.Errorf("err = %v", err)
	}
	var outErr *output.Error
	if !errors.As(err, &outErr) || !strings.Contains(outErr.Hint, "the token may be expired") {
		t.Errorf("missing expired-token hint: %+v", outErr)
	}
	if strings.Contains(err.Error(), "v2-expired-token") {
		t.Error("error leaked the token")
	}
	if got, _ := credential.ResolveV3Token(config.Read(), credential.DefaultKeychainStore()); got != "" {
		t.Errorf("token should not be stored after failed validation, got %q", got)
	}
}

func TestWorkspaceListFormatJSONEnvelope(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	out, _, err := runCLI(t, "", "auth", "workspace", "list", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("--format json did not produce one JSON document: %v\n%s", err, out)
	}
	if len(envelope.Data) != 2 || envelope.Data[0]["alias"] != "acme" {
		t.Errorf("envelope = %+v", envelope)
	}
	if !strings.Contains(out, "\n  ") {
		t.Errorf("--format json should pretty-print:\n%s", out)
	}
}

func TestEmitItemFormatYAML(t *testing.T) {
	isolateState(t)
	seedWorkspaces(t)

	out, _, err := runCLI(t, "", "auth", "workspace", "switch", "beta", "--format", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default_workspace: beta") {
		t.Errorf("--format yaml output:\n%s", out)
	}
}

func TestBackendFlagValidation(t *testing.T) {
	isolateState(t)

	_, _, err := runCLI(t, "", "--backend", "bogus", "auth", "status")
	if err == nil || !strings.Contains(err.Error(), "unknown backend") {
		t.Errorf("err = %v", err)
	}

	for _, mode := range backendModes {
		if _, _, err := runCLI(t, "", "--backend", mode, "usage"); err != nil {
			t.Errorf("--backend %s rejected: %v", mode, err)
		}
	}
}

func TestTimeoutFlagBuildsClient(t *testing.T) {
	g := &GlobalFlags{}
	if g.httpClient() != nil {
		t.Error("zero timeout should yield nil client")
	}
	g.TimeoutMS = 1500
	hc := g.httpClient()
	if hc == nil || hc.Timeout.Milliseconds() != 1500 {
		t.Errorf("client timeout = %v", hc)
	}
}
