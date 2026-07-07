package credential

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/lib-agent-cli/creds"
)

// KeychainStore is the full secret-store surface workspace CRUD needs.
// *keyring.Keyring satisfies it; tests supply a fake.
type KeychainStore interface {
	Keychain
	KeychainWriter
	Delete(account string) error
	DeleteAll() error
}

// DefaultKeychainStore opens the OS keychain service shared with the TS CLI.
func DefaultKeychainStore() KeychainStore { return creds.NewKeychain(config.KeychainService) }

func accessTokenAccount(alias string) string  { return "access_token:" + alias }
func refreshTokenAccount(alias string) string { return "refresh_token:" + alias }

// StoreWorkspace persists a workspace whose AccessToken/RefreshToken fields
// hold plaintext secrets. Both tokens go to the keychain when available
// (config holds the placeholder); if either write fails the partial entries
// are removed and both fall back to plaintext config. The first stored
// workspace becomes the default. Returns "keychain" or "config".
func StoreWorkspace(alias string, ws config.Workspace, kc KeychainStore) (string, error) {
	ws.AccessToken, ws.RefreshToken = placeTokens(alias, ws.AccessToken, ws.RefreshToken, kc)

	cfg := config.Read()
	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]config.Workspace{}
	}
	cfg.Workspaces[alias] = ws
	if cfg.DefaultWorkspace == "" {
		cfg.DefaultWorkspace = alias
	}
	if err := config.Write(cfg); err != nil {
		return "", err
	}
	return storageKind(ws.AccessToken), nil
}

// placeTokens writes both tokens to the keychain when it is available,
// returning the placeholder-substituted values to persist in config. On any
// keychain-write failure the partial entries are removed and both tokens fall
// back to plaintext — the placement decision is all-or-nothing so config and
// keychain can never disagree about where a token lives.
func placeTokens(alias, accessToken, refreshToken string, kc KeychainStore) (access, refresh string) {
	if kc == nil || !kc.Available() {
		return accessToken, refreshToken
	}

	stored := kc.Set(accessTokenAccount(alias), accessToken) == nil &&
		(refreshToken == "" || kc.Set(refreshTokenAccount(alias), refreshToken) == nil)
	if !stored {
		_ = kc.Delete(accessTokenAccount(alias))
		_ = kc.Delete(refreshTokenAccount(alias))
		return accessToken, refreshToken
	}

	access = config.KeychainPlaceholder
	refresh = refreshToken
	if refreshToken != "" {
		refresh = config.KeychainPlaceholder
	}
	return access, refresh
}

// storageKind reports where a placed access token lives.
func storageKind(placedAccessToken string) string {
	if placedAccessToken == config.KeychainPlaceholder {
		return "keychain"
	}
	return "config"
}

// RemoveWorkspace deletes a workspace's keychain entries and config record,
// reassigning the default (alphabetically first survivor) when it pointed at
// the removed alias.
func RemoveWorkspace(alias string, kc KeychainStore) error {
	cfg := config.Read()
	if _, ok := cfg.Workspaces[alias]; !ok {
		return unknownWorkspaceError(alias, cfg)
	}

	if kc != nil {
		_ = kc.Delete(accessTokenAccount(alias))
		_ = kc.Delete(refreshTokenAccount(alias))
	}

	delete(cfg.Workspaces, alias)
	if cfg.DefaultWorkspace == alias {
		cfg.DefaultWorkspace = firstAlias(cfg.Workspaces)
	}
	return config.Write(cfg)
}

// SetDefaultWorkspace points the default at an existing alias.
func SetDefaultWorkspace(alias string) error {
	cfg := config.Read()
	if _, ok := cfg.Workspaces[alias]; !ok {
		return unknownWorkspaceError(alias, cfg)
	}
	cfg.DefaultWorkspace = alias
	return config.Write(cfg)
}

// ClearAll wipes every keychain entry under the service and resets the config
// file to empty.
func ClearAll(kc KeychainStore) error {
	if kc != nil {
		_ = kc.DeleteAll()
	}
	return config.Write(config.Config{})
}

// UpdateWorkspaceTokens atomically swaps a workspace's access (and optionally
// refresh) token, keeping the keychain-vs-config placement consistent with
// StoreWorkspace (including partial-failure cleanup, which this variant
// previously lacked). A missing workspace is a silent no-op, matching the TS.
func UpdateWorkspaceTokens(alias, accessToken, refreshToken string, kc KeychainStore) error {
	cfg := config.Read()
	ws, ok := cfg.Workspaces[alias]
	if !ok {
		return nil
	}

	access, refresh := placeTokens(alias, accessToken, refreshToken, kc)
	ws.AccessToken = access
	if refreshToken != "" {
		ws.RefreshToken = refresh
	}
	cfg.Workspaces[alias] = ws
	return config.Write(cfg)
}

// ClearWorkspaceTokens drops a workspace's tokens (keychain + config) without
// removing the workspace record, marking it as needing re-authentication.
func ClearWorkspaceTokens(alias string, kc KeychainStore) error {
	cfg := config.Read()
	ws, ok := cfg.Workspaces[alias]
	if !ok {
		return nil
	}

	if kc != nil {
		_ = kc.Delete(accessTokenAccount(alias))
		_ = kc.Delete(refreshTokenAccount(alias))
	}

	ws.AccessToken = ""
	ws.RefreshToken = ""
	cfg.Workspaces[alias] = ws
	return config.Write(cfg)
}

var nonAliasChars = regexp.MustCompile(`[^a-z0-9]+`)

// DeriveAlias turns a workspace name into a unique kebab-case alias: lowered,
// non-alphanumerics collapsed to '-', trimmed, capped at 32 chars, "default"
// when empty, and suffixed -2..-99 (then a timestamp) on collision.
func DeriveAlias(name string, existing []string) string {
	alias := nonAliasChars.ReplaceAllString(strings.ToLower(name), "-")
	alias = strings.Trim(alias, "-")
	if len(alias) > 32 {
		alias = alias[:32]
	}
	if alias == "" {
		alias = "default"
	}

	taken := make(map[string]bool, len(existing))
	for _, a := range existing {
		taken[a] = true
	}
	if !taken[alias] {
		return alias
	}
	for i := 2; i <= 99; i++ {
		if candidate := fmt.Sprintf("%s-%d", alias, i); !taken[candidate] {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", alias, time.Now().UnixMilli())
}

func unknownWorkspaceError(alias string, cfg config.Config) error {
	valid := firstAliases(cfg.Workspaces)
	if valid == "" {
		valid = "(none)"
	}
	return fmt.Errorf("unknown workspace: '%s'. Valid workspaces: %s", alias, valid)
}

func firstAlias(workspaces map[string]config.Workspace) string {
	aliases := WorkspaceAliases(workspaces)
	if len(aliases) == 0 {
		return ""
	}
	return aliases[0]
}

func firstAliases(workspaces map[string]config.Workspace) string {
	return strings.Join(WorkspaceAliases(workspaces), ", ")
}

// WorkspaceAliases returns the configured workspace aliases, sorted.
func WorkspaceAliases(workspaces map[string]config.Workspace) []string {
	aliases := make([]string, 0, len(workspaces))
	for a := range workspaces {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	return aliases
}
