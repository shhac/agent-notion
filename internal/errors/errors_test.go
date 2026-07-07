package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shhac/agent-notion/internal/notion/official"
	v3 "github.com/shhac/agent-notion/internal/notion/v3"
	output "github.com/shhac/lib-agent-output"
)

// v3Error produces a real *v3.HTTPError by driving the client against a
// server returning the given status.
func v3Error(t *testing.T, status int) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	defer srv.Close()
	c := v3.Client{BaseURL: srv.URL, TokenV2: "t", UserID: "u", SpaceID: "s"}
	_, err := c.LoadUserContent(context.Background())
	if err == nil {
		t.Fatalf("expected error for status %d", status)
	}
	return err
}

func officialError(t *testing.T, status int, code string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"object":"error","status":%d,"code":%q,"message":"synthetic"}`, status, code)
	}))
	defer srv.Close()
	_, err := official.Client{BaseURL: srv.URL, Token: "t"}.Me(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	return err
}

func assertClassified(t *testing.T, err error, fixableBy output.FixableBy, msgPart, hintPart string) {
	t.Helper()
	var oe *output.Error
	if !stderrors.As(err, &oe) {
		t.Fatalf("not an output.Error: %v", err)
	}
	if oe.FixableBy != fixableBy {
		t.Errorf("fixable_by = %v, want %v (err: %v)", oe.FixableBy, fixableBy, err)
	}
	if msgPart != "" && !strings.Contains(oe.Error(), msgPart) {
		t.Errorf("message %q missing %q", oe.Error(), msgPart)
	}
	if hintPart != "" && !strings.Contains(oe.Hint, hintPart) {
		t.Errorf("hint %q missing %q", oe.Hint, hintPart)
	}
}

func TestClassifyNil(t *testing.T) {
	if Classify(nil) != nil {
		t.Error("nil should stay nil")
	}
}

func TestClassifyPassesThroughClassified(t *testing.T) {
	orig := output.New("already classified", output.FixableByHuman)
	if got := Classify(orig); got != orig {
		t.Errorf("classified error rewrapped: %v", got)
	}
}

func TestClassifyV3(t *testing.T) {
	assertClassified(t, Classify(v3Error(t, 401)), output.FixableByHuman, "desktop token expired", "import-desktop")
	assertClassified(t, Classify(v3Error(t, 403)), output.FixableByHuman, "access denied", "re-import")
	assertClassified(t, Classify(v3Error(t, 404)), output.FixableByAgent, "not found", "check the ID")
	assertClassified(t, Classify(v3Error(t, 429)), output.FixableByRetry, "rate limited", "retry")
	assertClassified(t, Classify(v3Error(t, 502)), output.FixableByRetry, "v3 API error", "retry")
	assertClassified(t, Classify(v3Error(t, 400)), output.FixableByAgent, "v3 API error", "")
}

func TestClassifyOfficial(t *testing.T) {
	assertClassified(t, Classify(officialError(t, 401, "unauthorized")), output.FixableByHuman,
		"not authenticated", "auth login")
	assertClassified(t, Classify(officialError(t, 403, "restricted_resource")), output.FixableByHuman,
		"not allowed", "my-integrations")
	assertClassified(t, Classify(officialError(t, 404, "object_not_found")), output.FixableByAgent,
		"not found", "share the resource")
	assertClassified(t, Classify(officialError(t, 400, "validation_error")), output.FixableByAgent,
		"validation error: synthetic", "")
	assertClassified(t, Classify(officialError(t, 429, "rate_limited")), output.FixableByRetry,
		"rate limited", "retry")
	assertClassified(t, Classify(officialError(t, 503, "service_unavailable")), output.FixableByRetry,
		"", "retry")
	assertClassified(t, Classify(officialError(t, 400, "unknown_code")), output.FixableByAgent, "", "")
}

func TestClassifyTimeoutsAndUnknown(t *testing.T) {
	assertClassified(t, Classify(context.DeadlineExceeded), output.FixableByRetry, "", "--timeout")
	assertClassified(t, Classify(stderrors.New("something odd")), output.FixableByAgent, "something odd", "")
}
