package cli

import (
	"io"
	"net/http"
	"time"

	"github.com/shhac/agent-notion/internal/auth"
	"github.com/shhac/agent-notion/internal/credential"
	libcli "github.com/shhac/lib-agent-cli/cli"
)

// GlobalFlags carries the family persistent flags plus agent-notion's own,
// and the injected seams commands reach external state through. Constructor
// injection (not package globals) so test roots are hermetic and
// parallelizable — the agent-slack pattern.
type GlobalFlags struct {
	libcli.Globals // Format, TimeoutMS, Debug, Color

	// BaseURL (hidden --base-url) points both API surfaces — the official
	// REST host and the v3 host — at one server, which is exactly what
	// internal/mocknotion serves in tests.
	BaseURL string
	// Backend (--backend) forces the API backend: auto (default, dispatch by
	// the workspace's auth type), official, or v3.
	Backend string

	// Injected seams — wired by newRoot, substituted by tests.
	version        string
	keychain       func() credential.KeychainStore
	desktopExtract func() (*auth.Session, error)
	browserImport  func(browser, profile string) (*auth.Session, error)
	openBrowser    func(url string) error
	stdout         io.Writer
	stderr         io.Writer
}

// rootDeps are the production defaults newRoot wires; tests build roots via
// newRootWithDeps with fakes.
type rootDeps struct {
	version        string
	keychain       func() credential.KeychainStore
	desktopExtract func() (*auth.Session, error)
	browserImport  func(browser, profile string) (*auth.Session, error)
	openBrowser    func(url string) error
}

// httpClient returns a client honoring --timeout, or nil (callers treat nil
// as http.DefaultClient).
func (g *GlobalFlags) httpClient() *http.Client {
	if g.TimeoutMS <= 0 {
		return nil
	}
	return &http.Client{Timeout: time.Duration(g.TimeoutMS) * time.Millisecond}
}

// officialBaseURL is the REST host override ("" = the real API).
func (g *GlobalFlags) officialBaseURL() string { return g.BaseURL }

// v3BaseURL is the v3 host override ("" = the real API). mocknotion routes
// /api/v3/<endpoint>, so the same --base-url serves both surfaces.
func (g *GlobalFlags) v3BaseURL() string {
	if g.BaseURL == "" {
		return ""
	}
	return g.BaseURL + "/api/v3"
}

// oauthTokenURL is the OAuth token endpoint override ("" = the real one).
func (g *GlobalFlags) oauthTokenURL() string {
	if g.BaseURL == "" {
		return ""
	}
	return g.BaseURL + "/v1/oauth/token"
}
