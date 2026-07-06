package oauth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

type waitOutcome struct {
	code string
	err  error
}

// startWait binds an ephemeral callback server and runs Wait in the
// background, returning the server and the pending outcome.
func startWait(t *testing.T, state string, timeout time.Duration) (*CallbackServer, <-chan waitOutcome) {
	t.Helper()
	s, err := ListenCallback(0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	done := make(chan waitOutcome, 1)
	go func() {
		code, err := s.Wait(context.Background(), state, timeout)
		done <- waitOutcome{code, err}
	}()
	return s, done
}

func await(t *testing.T, done <-chan waitOutcome) waitOutcome {
	t.Helper()
	select {
	case o := <-done:
		return o
	case <-time.After(10 * time.Second):
		t.Fatal("Wait did not return")
		return waitOutcome{}
	}
}

func callbackURL(s *CallbackServer, query string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/callback?%s", s.Port(), query)
}

func TestWaitResolvesCodeOnValidCallback(t *testing.T) {
	s, done := startWait(t, "test-state-123", 5*time.Second)

	resp, err := http.Get(callbackURL(s, "code=auth_code_xyz&state=test-state-123"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	o := await(t, done)
	if o.err != nil {
		t.Fatal(o.err)
	}
	if o.code != "auth_code_xyz" {
		t.Errorf("code = %q", o.code)
	}
}

func TestWaitRejectsStateMismatch(t *testing.T) {
	s, done := startWait(t, "correct-state", 5*time.Second)

	resp, err := http.Get(callbackURL(s, "code=auth_code&state=wrong-state"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	o := await(t, done)
	if o.err == nil || !strings.Contains(o.err.Error(), "state mismatch") {
		t.Errorf("err = %v, want state mismatch", o.err)
	}
}

func TestWaitRejectsProviderError(t *testing.T) {
	s, done := startWait(t, "test-state", 5*time.Second)

	resp, err := http.Get(callbackURL(s, "error=access_denied"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	o := await(t, done)
	if o.err == nil || !strings.Contains(o.err.Error(), "access_denied") {
		t.Errorf("err = %v, want access_denied", o.err)
	}
}

func TestWaitRejectsMissingCode(t *testing.T) {
	s, done := startWait(t, "test-state", 5*time.Second)

	resp, err := http.Get(callbackURL(s, "state=test-state"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	o := await(t, done)
	if o.err == nil || !strings.Contains(o.err.Error(), "no authorization code") {
		t.Errorf("err = %v, want missing-code error", o.err)
	}
}

func TestWaitKeepsServingAfterNonCallbackPath(t *testing.T) {
	s, done := startWait(t, "test-state", 5*time.Second)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/other", s.Port()))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	resp, err = http.Get(callbackURL(s, "code=cleanup&state=test-state"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if o := await(t, done); o.err != nil || o.code != "cleanup" {
		t.Errorf("outcome = %+v", o)
	}
}

func TestWaitTimesOut(t *testing.T) {
	_, done := startWait(t, "test-state", 100*time.Millisecond)

	o := await(t, done)
	if o.err == nil || !strings.Contains(o.err.Error(), "timed out") {
		t.Errorf("err = %v, want timeout", o.err)
	}
}

func TestListenCallbackSkipsBusyPort(t *testing.T) {
	first, err := ListenCallback(0)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	second, err := ListenCallback(first.Port())
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.Port() == first.Port() {
		t.Errorf("second server reused busy port %d", first.Port())
	}
	if second.Port() < first.Port() || second.Port() > first.Port()+9 {
		t.Errorf("second port %d outside range %d-%d", second.Port(), first.Port(), first.Port()+9)
	}
}

func TestRedirectURI(t *testing.T) {
	s, err := ListenCallback(0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	want := fmt.Sprintf("http://localhost:%d/callback", s.Port())
	if got := s.RedirectURI(); got != want {
		t.Errorf("RedirectURI = %q, want %q", got, want)
	}
}
