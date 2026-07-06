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
	root.AddCommand(authCmd)
}

func authStatusCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the resolved Notion credential (never prints the token)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			res, ok := credential.Resolve(config.Read(), g.keychain())
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
