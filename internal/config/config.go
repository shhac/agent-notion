// Package config reads and writes agent-notion's config.json.
//
// The on-disk shape is byte-compatible with the retired TypeScript
// implementation (see git history): same path, same JSON keys, same 0600 mode and
// trailing newline. The two binaries can therefore share one config file
// during the migration.
package config

import (
	"path/filepath"

	"github.com/shhac/lib-agent-cli/creds"
	"github.com/shhac/lib-agent-cli/xdg"
)

// AppName is the XDG application name: config lives in <config>/agent-notion/.
const AppName = "agent-notion"

// KeychainService is the OS keychain service the TS CLI wrote to. Reusing it
// lets the Go binary read existing stored tokens without a migration step.
const KeychainService = "app.paulie.agent-notion"

// KeychainPlaceholder is the sentinel stored in config.json in place of a
// secret that actually lives in the keychain.
const KeychainPlaceholder = "__KEYCHAIN__"

// AuthType is how a workspace's token was obtained.
type AuthType string

const (
	AuthOAuth               AuthType = "oauth"
	AuthInternalIntegration AuthType = "internal_integration"
	AuthDesktop             AuthType = "desktop"
)

// Owner is the user a workspace token belongs to.
type Owner struct {
	Type string `json:"type"`
	User struct {
		ID    string `json:"id"`
		Name  string `json:"name,omitempty"`
		Email string `json:"email,omitempty"`
	} `json:"user"`
}

// Workspace is a single authenticated Notion workspace. AccessToken and
// RefreshToken hold either the real secret or KeychainPlaceholder.
type Workspace struct {
	WorkspaceID   string   `json:"workspace_id"`
	WorkspaceName string   `json:"workspace_name"`
	WorkspaceIcon string   `json:"workspace_icon,omitempty"`
	BotID         string   `json:"bot_id"`
	AuthType      AuthType `json:"auth_type"`
	AccessToken   string   `json:"access_token"`
	RefreshToken  string   `json:"refresh_token,omitempty"`
	Owner         *Owner   `json:"owner,omitempty"`
}

// OAuthConfig holds the registered OAuth client used for token refresh.
type OAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
}

// Truncation bounds LLM-facing field lengths.
type Truncation struct {
	MaxLength int `json:"max_length,omitempty"`
}

// AISettings holds ai-subcommand defaults.
type AISettings struct {
	DefaultModel string `json:"default_model,omitempty"`
}

// Settings holds user-tunable defaults.
type Settings struct {
	PageSize   int         `json:"page_size,omitempty"`
	MaxDepth   int         `json:"max_depth,omitempty"`
	Truncation *Truncation `json:"truncation,omitempty"`
	AI         *AISettings `json:"ai,omitempty"`
}

// V3Session is the desktop (token_v2) session for the unofficial API.
type V3Session struct {
	TokenV2     string `json:"token_v2"`
	UserID      string `json:"user_id"`
	UserEmail   string `json:"user_email"`
	UserName    string `json:"user_name"`
	SpaceID     string `json:"space_id"`
	SpaceName   string `json:"space_name"`
	SpaceViewID string `json:"space_view_id,omitempty"`
	ExtractedAt string `json:"extracted_at"`
}

// Config is the full config.json document.
type Config struct {
	OAuth            *OAuthConfig         `json:"oauth,omitempty"`
	DefaultWorkspace string               `json:"default_workspace,omitempty"`
	Workspaces       map[string]Workspace `json:"workspaces,omitempty"`
	Settings         *Settings            `json:"settings,omitempty"`
	V3               *V3Session           `json:"v3,omitempty"`
}

// Dir is the config directory: $XDG_CONFIG_HOME/agent-notion or
// ~/.config/agent-notion.
func Dir() string { return xdg.ConfigDir(AppName) }

// Path is the config.json path.
func Path() string { return filepath.Join(Dir(), "config.json") }

func store() creds.Store { return creds.Store{Path: Path()} }

// Read loads config.json. A missing, empty, or corrupt file yields an empty
// Config (matching the TS readConfig, which never errors on a bad file).
func Read() Config {
	var c Config
	if err := store().Load(&c); err != nil {
		return Config{}
	}
	return c
}

// Write saves config.json (0600, indented, trailing newline — creds.Store
// matches the TS writeConfig byte-for-byte).
func Write(c Config) error { return store().Save(c) }
