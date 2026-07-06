// Package credential resolves the active Notion access token from the
// environment, config.json, and the OS keychain — in the same order the TS
// implementation used, so existing credentials keep working unchanged.
package credential

import (
	"strings"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/lib-agent-cli/creds"
)

// Source is where a resolved token came from.
type Source string

const (
	SourceEnvironment Source = "environment"
	SourceKeychain    Source = "keychain"
	SourceConfig      Source = "config"
)

// Keychain reads secrets by account name. *keyring.Keyring satisfies it;
// tests supply a fake.
type Keychain interface {
	Get(account string) (string, bool)
}

// Resolved is a successfully located access token and its provenance. The
// token itself is never logged or printed.
type Resolved struct {
	Key       string
	Source    Source
	Workspace string
	AuthType  config.AuthType
}

// DefaultKeychain reads the OS keychain service the TS CLI wrote to.
func DefaultKeychain() Keychain { return creds.NewKeychain(config.KeychainService) }

// Resolve mirrors the TS resolveAccessToken order:
//  1. NOTION_API_KEY / NOTION_TOKEN environment variables,
//  2. the default workspace's token — from the keychain when config holds the
//     placeholder, otherwise the plaintext value in config.
//
// The bool is false when no credential is configured.
func Resolve(cfg config.Config, kc Keychain) (Resolved, bool) {
	if env := strings.TrimSpace(creds.Getenv("NOTION_API_KEY", "NOTION_TOKEN")); env != "" {
		return Resolved{Key: env, Source: SourceEnvironment}, true
	}

	alias := cfg.DefaultWorkspace
	if alias == "" {
		return Resolved{}, false
	}
	ws, ok := cfg.Workspaces[alias]
	if !ok {
		return Resolved{}, false
	}

	if ws.AccessToken == config.KeychainPlaceholder {
		key, ok := kc.Get("access_token:" + alias)
		if !ok || key == "" {
			return Resolved{}, false
		}
		return Resolved{Key: key, Source: SourceKeychain, Workspace: alias, AuthType: ws.AuthType}, true
	}

	if ws.AccessToken != "" {
		return Resolved{Key: ws.AccessToken, Source: SourceConfig, Workspace: alias, AuthType: ws.AuthType}, true
	}

	return Resolved{}, false
}
