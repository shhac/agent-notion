package cli

import (
	"context"
	stderrors "errors"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	agenterrors "github.com/shhac/agent-notion/internal/errors"
	"github.com/shhac/agent-notion/internal/notion"
	"github.com/shhac/agent-notion/internal/notion/official"
	v3 "github.com/shhac/agent-notion/internal/notion/v3"
	"github.com/shhac/agent-notion/internal/oauth"
	output "github.com/shhac/lib-agent-output"
)

// backendHandle carries a resolved backend plus the provenance withBackend
// needs for the 401 refresh path.
type backendHandle struct {
	backend   notion.Backend
	workspace string
	authType  config.AuthType
}

// newBackend resolves the credential state and --backend mode into a backend.
// Auto order mirrors the TS factory: a v3 desktop session wins, then the
// official API credential (env var or default workspace).
func (g *GlobalFlags) newBackend() (backendHandle, error) {
	switch g.Backend {
	case "v3":
		return g.newV3BackendHandle()
	case "official":
		return g.newOfficialBackendHandle()
	default: // auto
		if handle, err := g.newV3BackendHandle(); err == nil {
			return handle, nil
		}
		return g.newOfficialBackendHandle()
	}
}

func (g *GlobalFlags) newV3BackendHandle() (backendHandle, error) {
	client, sess, err := g.newV3Client()
	if err != nil {
		return backendHandle{}, err
	}
	return backendHandle{
		backend:   v3.NewBackend(client),
		workspace: sess.SpaceName,
		authType:  config.AuthDesktop,
	}, nil
}

func (g *GlobalFlags) newOfficialBackendHandle() (backendHandle, error) {
	cred, ok := credential.Resolve(config.Read(), g.keychain())
	if !ok {
		return backendHandle{}, output.New("not authenticated", output.FixableByHuman).
			WithHint("run 'agent-notion auth login', 'agent-notion auth import', or import a desktop token")
	}
	return backendHandle{
		backend:   official.NewBackend(g.officialClient(cred.Key)),
		workspace: cred.Workspace,
		authType:  cred.AuthType,
	}, nil
}

// newV3Client resolves the stored desktop session into a v3 client, for the
// backend and for v3-only groups (export, activity, backlinks, history, ai).
func (g *GlobalFlags) newV3Client() (*v3.Client, *config.V3Session, error) {
	cfg := config.Read()
	if cfg.V3 == nil {
		return nil, nil, output.New("this operation requires a v3 desktop session", output.FixableByHuman).
			WithHint("run 'agent-notion auth import-desktop' (or import-browser) first")
	}
	tokenV2, ok := credential.ResolveV3Token(cfg, g.keychain())
	if !ok {
		return nil, nil, output.New("desktop token not found", output.FixableByHuman).
			WithHint("run 'agent-notion auth import-desktop' to set it up again")
	}
	return &v3.Client{
		HTTP:    g.httpClient(),
		BaseURL: g.v3BaseURL(),
		TokenV2: tokenV2,
		UserID:  cfg.V3.UserID,
		SpaceID: cfg.V3.SpaceID,
	}, cfg.V3, nil
}

func (g *GlobalFlags) officialClient(token string) official.Client {
	return official.Client{HTTP: g.httpClient(), BaseURL: g.officialBaseURL(), Token: token}
}

// withBackend runs fn against the resolved backend, retrying once through the
// OAuth refresh path on an unauthorized failure — the TS withBackend port.
// Errors come back classified ({error, fixable_by, hint}); command bodies
// just `return err`. Classification runs strictly after the isUnauthorized
// check, which needs the raw *APIError/*HTTPError.
func withBackend[T any](ctx context.Context, g *GlobalFlags, fn func(b notion.Backend) (T, error)) (T, error) {
	var zero T

	handle, err := g.newBackend()
	if err != nil {
		return zero, err
	}

	result, err := fn(handle.backend)
	// Desktop tokens can't be refreshed programmatically, so v3 auth failures
	// go straight to classification (classifyV3 owns the 401-expired vs
	// 403-access-denied guidance) like every other error.
	if err == nil || !isUnauthorized(err) || handle.backend.Kind() == "v3" {
		return result, agenterrors.Classify(err)
	}

	// Internal integrations can't refresh either.
	if handle.authType == config.AuthInternalIntegration {
		return zero, output.New("token is invalid or revoked", output.FixableByHuman).
			WithHint("run 'agent-notion auth import' to re-authenticate")
	}
	if handle.workspace == "" {
		return zero, output.New("not authenticated", output.FixableByHuman).
			WithHint("run 'agent-notion auth login' to connect")
	}

	tokenClient := oauth.TokenClient{HTTP: g.httpClient(), URL: g.oauthTokenURL()}
	newToken, ok := credential.RefreshOrRecover(ctx, handle.workspace, g.keychain(), tokenClient)
	if !ok {
		return zero, output.New("token expired and refresh failed", output.FixableByHuman).
			WithHint("run 'agent-notion auth login' to re-authenticate")
	}

	result, err = fn(official.NewBackend(g.officialClient(newToken)))
	return result, agenterrors.Classify(err)
}

// withV3Client resolves the stored desktop session and runs fn against the
// v3 client — the seam for v3-only commands (backlinks, history, export,
// activity, ai). Errors come back classified, like withBackend.
func withV3Client[T any](g *GlobalFlags, fn func(c *v3.Client, sess *config.V3Session) (T, error)) (T, error) {
	var zero T
	client, sess, err := g.newV3Client()
	if err != nil {
		return zero, err
	}
	result, err := fn(client, sess)
	return result, agenterrors.Classify(err)
}

// isUnauthorized reports whether err is an auth failure worth a refresh
// attempt: official 401/unauthorized, or v3 401 — and 403, which the v3 API
// returns for expired desktop tokens.
func isUnauthorized(err error) bool {
	var apiErr *official.APIError
	if stderrors.As(err, &apiErr) {
		return apiErr.Status == 401 || apiErr.Code == "unauthorized"
	}
	var v3Err *v3.HTTPError
	if stderrors.As(err, &v3Err) {
		return v3Err.Status == 401 || v3Err.Status == 403
	}
	return false
}
