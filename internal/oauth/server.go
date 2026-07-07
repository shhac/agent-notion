// Package oauth implements the Notion OAuth login flow: the localhost
// callback server the browser redirects to, the authorize-URL builder, and
// the token endpoint calls (code exchange + refresh).
package oauth

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"sync"
	"time"
)

const successHTML = `<!DOCTYPE html>
<html><head><title>agent-notion</title></head>
<body style="font-family:system-ui;text-align:center;padding:60px">
<h2>Authorized</h2>
<p>You can close this tab and return to the terminal.</p>
</body></html>`

func errorHTML(msg string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><title>agent-notion</title></head>
<body style="font-family:system-ui;text-align:center;padding:60px">
<h2>Error</h2>
<p>%s</p>
</body></html>`, html.EscapeString(msg))
}

// CallbackServer waits on 127.0.0.1 for the OAuth redirect. Bind with
// ListenCallback before building the authorize URL so the redirect URI always
// carries the port that was actually acquired.
type CallbackServer struct {
	listener net.Listener
	port     int
}

// ListenCallback binds the first free port in [startPort, startPort+9].
// startPort 0 binds an ephemeral port (used by tests).
func ListenCallback(startPort int) (*CallbackServer, error) {
	maxPort := startPort + 9
	for port := startPort; port <= maxPort; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return &CallbackServer{listener: l, port: l.Addr().(*net.TCPAddr).Port}, nil
		}
	}
	return nil, fmt.Errorf(
		"could not find an available port for the OAuth callback (tried %d-%d); use --port to pick one",
		startPort, maxPort)
}

// Port is the bound localhost port.
func (s *CallbackServer) Port() int { return s.port }

// RedirectURI is the callback URL to register in the authorize request.
func (s *CallbackServer) RedirectURI() string {
	return fmt.Sprintf("http://localhost:%d/callback", s.port)
}

// Close releases the port. Safe after Wait, which closes on return.
func (s *CallbackServer) Close() { _ = s.listener.Close() }

type callbackResult struct {
	code string
	err  error
}

// Wait serves until the OAuth redirect lands on /callback, then returns the
// authorization code. A state mismatch, a provider error parameter, a missing
// code, or the timeout elapsing fail the flow; other paths get a 404 and keep
// the server waiting.
func (s *CallbackServer) Wait(ctx context.Context, expectedState string, timeout time.Duration) (string, error) {
	results := make(chan callbackResult, 1)
	var once sync.Once
	settle := func(r callbackResult) { once.Do(func() { results <- r }) }

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()

		if provErr := q.Get("error"); provErr != "" {
			msg := "Notion OAuth error: " + provErr
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, errorHTML(msg))
			settle(callbackResult{err: fmt.Errorf("%s", msg)})
			return
		}
		if q.Get("state") != expectedState {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, errorHTML("OAuth state mismatch — possible CSRF attack. Please try again."))
			settle(callbackResult{err: fmt.Errorf("OAuth state mismatch — possible CSRF attack. Please try again.")})
			return
		}
		code := q.Get("code")
		if code == "" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, errorHTML("No authorization code received."))
			settle(callbackResult{err: fmt.Errorf("no authorization code received from Notion")})
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, successHTML)
		settle(callbackResult{code: code})
	})}

	go func() { _ = server.Serve(s.listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	select {
	case r := <-results:
		return r.code, r.err
	case <-time.After(timeout):
		return "", fmt.Errorf("OAuth flow timed out after %d seconds. Please try again.", int(timeout.Seconds()))
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
