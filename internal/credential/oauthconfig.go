package credential

import "github.com/shhac/agent-notion/internal/config"

const oauthSecretAccount = "oauth_client_secret"

// DefaultRedirectURI is the OAuth callback the registered Notion integration
// must allow. The port must match the callback server's default.
const DefaultRedirectURI = "http://localhost:9876/callback"

// StoreOAuthConfig persists the registered OAuth client. The secret goes to
// the keychain when available (config holds the placeholder), otherwise into
// config.json. Returns "keychain" or "config".
func StoreOAuthConfig(clientID, clientSecret string, kc KeychainStore) (string, error) {
	storage := "config"
	if kc != nil && kc.Available() && kc.Set(oauthSecretAccount, clientSecret) == nil {
		clientSecret = config.KeychainPlaceholder
		storage = "keychain"
	}

	cfg := config.Read()
	cfg.OAuth = &config.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  DefaultRedirectURI,
	}
	if err := config.Write(cfg); err != nil {
		return "", err
	}
	return storage, nil
}

// ResolveOAuthClientSecret returns the stored client secret, reading the
// keychain when config holds the placeholder.
func ResolveOAuthClientSecret(cfg config.Config, kc Keychain) (string, bool) {
	if cfg.OAuth == nil || cfg.OAuth.ClientSecret == "" {
		return "", false
	}
	if cfg.OAuth.ClientSecret == config.KeychainPlaceholder {
		return kc.Get(oauthSecretAccount)
	}
	return cfg.OAuth.ClientSecret, true
}
