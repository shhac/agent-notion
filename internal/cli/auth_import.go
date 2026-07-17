package cli

import (
	"strings"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	"github.com/shhac/agent-notion/internal/notion/official"
	"github.com/shhac/lib-agent-cli/creds"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func authImportCmd(g *GlobalFlags) *cobra.Command {
	var (
		token string
		alias string
	)
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Store an internal-integration token (from --token or stdin)",
		Long: "Store a Notion internal-integration token (ntn_… or secret_…) as a workspace. " +
			"The token is validated against the API before storing. " +
			"Pass it with --token or pipe it on stdin.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runImportToken(cmd, g, token, alias)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Integration token (default: read from stdin)")
	cmd.Flags().StringVar(&alias, "alias", "", "Workspace alias (default: derived from the workspace name)")
	return cmd
}

func runImportToken(cmd *cobra.Command, g *GlobalFlags, token, alias string) error {
	token, err := creds.ReadSecret(cmd.InOrStdin(), token)
	if err != nil {
		return err
	}
	if token == "" {
		return output.New("expected an integration token via --token or stdin", output.FixableByAgent).
			WithHint("create one at https://www.notion.so/my-integrations and share pages with it")
	}

	if !strings.HasPrefix(token, "ntn_") && !strings.HasPrefix(token, "secret_") {
		warnf(g, "token does not start with 'ntn_' or 'secret_'; proceeding anyway")
	}

	client := official.Client{HTTP: g.httpClient(), BaseURL: g.officialBaseURL(), Token: token}
	bot, err := client.Me(cmd.Context())
	if err != nil {
		return output.New("invalid token: the API rejected it", output.FixableByHuman).
			WithHint("check the token is correct and the integration has access to a workspace")
	}

	cfg := config.Read()
	name := bot.WorkspaceName
	if name == "" {
		name = alias
	}
	if name == "" {
		name = "default"
	}
	resolvedAlias := alias
	if resolvedAlias == "" {
		resolvedAlias = credential.DeriveAlias(name, credential.WorkspaceAliases(cfg.Workspaces))
	}

	storage, err := credential.StoreWorkspace(resolvedAlias, config.Workspace{
		WorkspaceID:   bot.ID,
		WorkspaceName: name,
		BotID:         bot.ID,
		AuthType:      config.AuthInternalIntegration,
		AccessToken:   token,
	}, g.keychain())
	if err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}

	return emitItem(g, map[string]any{
		"ok":      true,
		"storage": storage,
		"workspace": map[string]any{
			"alias":     resolvedAlias,
			"name":      name,
			"id":        bot.ID,
			"auth_type": string(config.AuthInternalIntegration),
			"default":   config.Read().DefaultWorkspace == resolvedAlias,
		},
	})
}
