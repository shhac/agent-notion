package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	"github.com/shhac/agent-notion/internal/notion/official"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func authImportCmd() *cobra.Command {
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
			return runImportToken(cmd, token, alias)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Integration token (default: read from stdin)")
	cmd.Flags().StringVar(&alias, "alias", "", "Workspace alias (default: derived from the workspace name)")
	return cmd
}

func runImportToken(cmd *cobra.Command, token, alias string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		raw, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		trimmed = strings.TrimSpace(string(raw))
	}
	if trimmed == "" {
		return output.New("expected an integration token via --token or stdin", output.FixableByAgent).
			WithHint("create one at https://www.notion.so/my-integrations and share pages with it")
	}

	if !strings.HasPrefix(trimmed, "ntn_") && !strings.HasPrefix(trimmed, "secret_") {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
			"warning: token does not start with 'ntn_' or 'secret_'; proceeding anyway")
	}

	bot, err := official.Client{Token: trimmed}.Me(cmd.Context())
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
		resolvedAlias = credential.DeriveAlias(name, workspaceAliases(cfg))
	}

	storage, err := credential.StoreWorkspace(resolvedAlias, config.Workspace{
		WorkspaceID:   bot.ID,
		WorkspaceName: name,
		BotID:         bot.ID,
		AuthType:      config.AuthInternalIntegration,
		AccessToken:   trimmed,
	}, credential.DefaultKeychainStore())
	if err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}

	return output.NewNDJSONWriter(cmd.OutOrStdout()).WriteItem(map[string]any{
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
