package cli

import (
	"strings"

	"github.com/shhac/agent-notion/internal/credential"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func setupOAuthCmd() *cobra.Command {
	var clientID, clientSecret string
	cmd := &cobra.Command{
		Use:   "setup-oauth",
		Short: "Store the OAuth app credentials used by 'auth login'",
		Long: "Store the client id/secret of a public Notion integration " +
			"(register one at https://www.notion.so/my-integrations). " +
			"The secret goes to the OS keychain when available.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id := strings.TrimSpace(clientID)
			secret := strings.TrimSpace(clientSecret)
			if id == "" {
				return output.New("client ID cannot be empty", output.FixableByAgent)
			}
			if secret == "" {
				return output.New("client secret cannot be empty", output.FixableByAgent)
			}

			storage, err := credential.StoreOAuthConfig(id, secret, credential.DefaultKeychainStore())
			if err != nil {
				return output.Wrap(err, output.FixableByHuman)
			}

			item := map[string]any{
				"ok":               true,
				"oauth_configured": true,
				"client_id":        id,
				"secret_storage":   storage,
			}
			if storage == "config" {
				item["warning"] = "client secret stored in plaintext config (keychain unavailable on this platform)"
			}
			return output.NewNDJSONWriter(cmd.OutOrStdout()).WriteItem(item)
		},
	}
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth app client ID")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "OAuth app client secret")
	_ = cmd.MarkFlagRequired("client-id")
	_ = cmd.MarkFlagRequired("client-secret")
	return cmd
}
