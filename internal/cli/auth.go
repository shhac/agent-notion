package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/shhac/agent-notion/internal/auth"
	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	v3 "github.com/shhac/agent-notion/internal/notion/v3"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// registerAuth wires the `auth` command group.
func registerAuth(root *cobra.Command, g *GlobalFlags) {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Notion authentication",
	}
	authCmd.AddCommand(
		authStatusCmd(g),
		setupOAuthCmd(g),
		authLoginCmd(g),
		authImportCmd(g),
		authLogoutCmd(g),
		authWorkspaceCmd(g),
		importDesktopCmd(g),
		importBrowserCmd(g),
	)
	addDomainUsage("auth", authUsageText)
	root.AddCommand(authCmd)
}

func authStatusCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the resolved Notion credential (never prints the token)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := config.Read()

			// Mirror the auto-backend resolution order: a stored v3 desktop
			// session wins, then the official-API credential.
			if _, ok := credential.ResolveV3Token(cfg, g.keychain()); ok && cfg.V3 != nil {
				item := map[string]any{
					"authenticated": true,
					"source":        "desktop",
					"auth_type":     string(config.AuthDesktop),
				}
				if cfg.V3.SpaceName != "" {
					item["workspace"] = cfg.V3.SpaceName
				}
				if cfg.V3.UserName != "" {
					item["user"] = cfg.V3.UserName
				}
				return emitItem(g, item)
			}

			res, ok := credential.Resolve(cfg, g.keychain())
			if !ok {
				return output.New("no Notion credential configured", output.FixableByHuman).
					WithHint("run 'agent-notion auth login', import a desktop token, or set NOTION_TOKEN")
			}

			item := map[string]any{
				"authenticated": true,
				"source":        string(res.Source),
			}
			if res.Workspace != "" {
				item["workspace"] = res.Workspace
			}
			if res.AuthType != "" {
				item["auth_type"] = string(res.AuthType)
			}
			return emitItem(g, item)
		},
	}
}

func importDesktopCmd(g *GlobalFlags) *cobra.Command {
	var skipValidation bool
	cmd := &cobra.Command{
		Use:   "import-desktop",
		Short: "Import the token_v2 session from the Notion Desktop app",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := g.desktopExtract()
			if err != nil {
				return output.Wrap(err, output.FixableByHuman).
					WithHint("open Notion Desktop and sign in, then retry")
			}
			return finishImport(cmd, g, sess, skipValidation)
		},
	}
	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false,
		"Store the token without validating it against Notion (leaves identity empty)")
	return cmd
}

func importBrowserCmd(g *GlobalFlags) *cobra.Command {
	var (
		skipValidation bool
		profile        string
	)
	cmd := &cobra.Command{
		Use:   "import-browser <browser>",
		Short: "Import the token_v2 session from a browser cookie store",
		Long:  browserLongHelp(),
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			names := make([]string, 0)
			for _, b := range auth.SupportedBrowsers() {
				names = append(names, b.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := g.browserImport(args[0], profile)
			if err != nil {
				return output.Wrap(err, output.FixableByHuman).
					WithHint("sign in to notion.so in that browser, then retry")
			}
			return finishImport(cmd, g, sess, skipValidation)
		},
	}
	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false,
		"Store the token without validating it against Notion (leaves identity empty)")
	cmd.Flags().StringVar(&profile, "profile", "",
		"Firefox-family profile directory name (default: auto-detect)")
	return cmd
}

// finishImport validates (unless skipped), stores, and reports an extracted
// session. The token itself is never printed.
func finishImport(cmd *cobra.Command, g *GlobalFlags, sess *auth.Session, skipValidation bool) error {
	extractedAt := time.Now().UTC().Format(time.RFC3339)

	var info v3.SessionInfo
	if skipValidation {
		warnf(g, "--skip-validation stores the token without an identity lookup; "+
			"user_id and space_id will be empty and some commands may fail")
	} else {
		var err error
		info, err = v3.ValidateDesktopToken(cmd.Context(), g.httpClient(), g.v3BaseURL(), sess.TokenV2)
		if err != nil {
			return output.Wrap(err, output.FixableByHuman).
				WithHint("the token may be expired; sign in again and re-import, or pass --skip-validation")
		}
	}

	storage, err := credential.StoreV3Session(config.V3Session{
		TokenV2:     sess.TokenV2,
		UserID:      info.UserID,
		UserEmail:   info.UserEmail,
		UserName:    info.UserName,
		SpaceID:     info.SpaceID,
		SpaceName:   info.SpaceName,
		SpaceViewID: info.SpaceViewID,
		ExtractedAt: extractedAt,
	}, g.keychain())
	if err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}

	item := map[string]any{
		"ok":           true,
		"storage":      storage,
		"extracted_at": extractedAt,
	}
	if info.UserName != "" {
		item["user"] = info.UserName
	}
	if info.UserEmail != "" {
		item["email"] = info.UserEmail
	}
	if info.SpaceName != "" {
		item["space"] = info.SpaceName
	}
	if info.SpaceID != "" {
		item["space_id"] = info.SpaceID
	}
	if len(sess.Source) > 0 {
		item["source"] = sess.Source
	}
	return emitItem(g, item)
}

func browserLongHelp() string {
	var b strings.Builder
	b.WriteString("Import the token_v2 session cookie from a browser's on-disk cookie store.\n\n")
	b.WriteString("Supported browsers:\n")
	for _, info := range auth.SupportedBrowsers() {
		fmt.Fprintf(&b, "  %-9s %s\n", info.Name, info.Summary)
	}
	return strings.TrimRight(b.String(), "\n")
}

const authUsageText = `agent-notion auth — Manage Notion authentication and workspaces

SUBCOMMANDS
  auth status                                                  Show the resolved credential (never prints tokens)
  auth setup-oauth --client-id <id> --client-secret <secret>   Store OAuth app credentials
  auth login [--alias <name>] [--port <port>]                  OAuth login flow (opens the browser)
  auth import [--token <token>] [--alias <name>]               Store an internal-integration token (stdin ok)
  auth logout [--all] [--workspace <alias>] --yes              Remove credentials
  auth workspace list                                          List workspaces (one NDJSON record each)
  auth workspace switch <alias>                                Set the default workspace
  auth workspace set-default <alias>                           Alias for switch
  auth workspace remove <alias> --yes                          Remove a workspace
  auth import-desktop [--skip-validation]                      Import token_v2 from the Notion Desktop app
  auth import-browser <browser> [--profile <p>]                Import token_v2 from a browser cookie store

AUTH SOURCES (checked in order)
  1. NOTION_API_KEY or NOTION_TOKEN environment variable
  2. Default workspace token — OS keychain, else config file
     (~/.config/agent-notion/config.json)

SETUP-OAUTH
  Register a public integration at https://www.notion.so/my-integrations and
  store its client credentials. The secret goes to the OS keychain when
  available, plaintext config otherwise (a warning field says which).
  Returns: {ok, oauth_configured, client_id, secret_storage}

LOGIN (OAuth)
  Requires setup-oauth first. Binds a localhost callback (default port 9876,
  falls forward to 9885), opens the browser for consent, exchanges the code,
  and stores the tokens. The first workspace becomes the default.
  Returns: {ok, storage, workspace: {alias, name, id, bot_id, default}, hint}

IMPORT (internal integration)
  Token from --token or stdin; ntn_ or secret_ prefix expected (warned
  otherwise). Validated against the API (users/me) before storing.
  Returns: {ok, storage, workspace: {alias, name, id, auth_type, default}}

LOGOUT / WORKSPACE REMOVE
  Destructive: refused without --yes. logout targets the default workspace,
  --workspace <alias> a specific one; --all wipes every workspace, the OAuth
  config, and all keychain entries.
  Returns: {ok, removed, remaining_workspaces, default_workspace?, warning?}
  Returns (--all): {ok, cleared: "all"}

WORKSPACE
  list: one record per workspace: {alias, name, auth_type, default}
  switch/set-default: {ok, default_workspace}

IMPORT-DESKTOP / IMPORT-BROWSER
  Read the token_v2 session cookie for Notion's unofficial API (used by
  history/activity/backlinks/ai once ported). Validated via getSpaces unless
  --skip-validation. Browsers: chrome, brave, edge, arc, chromium, firefox,
  zen, safari.
  Returns: {ok, storage, extracted_at, user?, email?, space?, space_id?, source?}

OUTPUT
  NDJSON records on stdout; errors on stderr as {error, fixable_by, hint}
  with exit 1. Tokens are never printed. OAuth access tokens refresh
  automatically once API commands land.`
