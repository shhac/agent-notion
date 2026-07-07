package credential

import (
	"context"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/oauth"
)

// RefreshAccessToken refreshes an OAuth workspace's token pair against the
// Notion token endpoint and atomically stores both new tokens. It reports
// false — without distinguishing why, matching the TS — when the workspace is
// not OAuth-typed, credentials are incomplete, or the endpoint refuses.
func RefreshAccessToken(ctx context.Context, alias string, kc KeychainStore, tc oauth.TokenClient) (string, bool) {
	cfg := config.Read()
	ws, ok := cfg.Workspaces[alias]
	if !ok || ws.AuthType != config.AuthOAuth {
		return "", false
	}

	refreshToken := ws.RefreshToken
	if refreshToken == config.KeychainPlaceholder {
		refreshToken, _ = kc.Get(refreshTokenAccount(alias))
	}
	if refreshToken == "" {
		return "", false
	}

	if cfg.OAuth == nil || cfg.OAuth.ClientID == "" {
		return "", false
	}
	clientSecret, ok := ResolveOAuthClientSecret(cfg, kc)
	if !ok {
		return "", false
	}

	tok, err := tc.Refresh(ctx, cfg.OAuth.ClientID, clientSecret, refreshToken)
	if err != nil {
		return "", false
	}
	if err := UpdateWorkspaceTokens(alias, tok.AccessToken, tok.RefreshToken, kc); err != nil {
		return "", false
	}
	return tok.AccessToken, true
}

// RefreshOrRecover refreshes the workspace's access token, falling back to
// re-reading the keychain in case a parallel process already refreshed. When
// both fail the stale tokens are cleared so the user is prompted to log in
// again.
func RefreshOrRecover(ctx context.Context, alias string, kc KeychainStore, tc oauth.TokenClient) (string, bool) {
	if token, ok := RefreshAccessToken(ctx, alias, kc, tc); ok {
		return token, true
	}

	if fresh, ok := kc.Get(accessTokenAccount(alias)); ok && fresh != "" {
		return fresh, true
	}

	_ = ClearWorkspaceTokens(alias, kc)
	return "", false
}
