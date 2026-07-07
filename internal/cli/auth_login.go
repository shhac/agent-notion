package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	"github.com/shhac/agent-notion/internal/oauth"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// loginTimeout bounds the wait for the user to authorize in the browser.
const loginTimeout = 2 * time.Minute

func authLoginCmd(g *GlobalFlags) *cobra.Command {
	var (
		alias string
		port  int
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate a Notion workspace via OAuth (opens the browser)",
		Long: "Run the OAuth consent flow against the client stored by 'auth setup-oauth': " +
			"opens the browser, waits for the localhost callback, and stores the workspace tokens. " +
			"For internal-integration tokens use 'auth import' instead.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogin(cmd, g, alias, port)
		},
	}
	cmd.Flags().StringVar(&alias, "alias", "", "Workspace alias (default: derived from the workspace name)")
	cmd.Flags().IntVar(&port, "port", 9876, "Localhost port for the OAuth callback (falls forward up to +9)")
	return cmd
}

func runLogin(cmd *cobra.Command, g *GlobalFlags, alias string, port int) error {
	cfg := config.Read()
	kc := g.keychain()

	if cfg.OAuth == nil || cfg.OAuth.ClientID == "" {
		return output.New("OAuth not configured", output.FixableByHuman).
			WithHint("run 'agent-notion auth setup-oauth --client-id <id> --client-secret <secret>' first, " +
				"or use 'agent-notion auth import' for an internal-integration token")
	}
	clientSecret, ok := credential.ResolveOAuthClientSecret(cfg, kc)
	if !ok {
		return output.New("OAuth client secret not found", output.FixableByHuman).
			WithHint("run 'agent-notion auth setup-oauth' to reconfigure")
	}

	state, err := randomState()
	if err != nil {
		return err
	}
	srv, err := oauth.ListenCallback(port)
	if err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}
	defer srv.Close()

	authURL := oauth.AuthorizeURL(cfg.OAuth.ClientID, srv.RedirectURI(), state)
	if err := g.openBrowser(authURL); err != nil {
		_, _ = fmt.Fprintf(g.stderr, "could not open the browser; visit: %s\n", authURL)
	} else {
		_, _ = fmt.Fprintf(g.stderr, "waiting for authorization in the browser (visit %s if it did not open)\n", authURL)
	}

	ctx := cmd.Context()
	code, err := srv.Wait(ctx, state, loginTimeout)
	if err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}

	tokenClient := oauth.TokenClient{HTTP: g.httpClient(), URL: g.oauthTokenURL()}
	tok, err := tokenClient.Exchange(ctx, cfg.OAuth.ClientID, clientSecret, code, srv.RedirectURI())
	if err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}

	resolvedAlias := alias
	if resolvedAlias == "" {
		resolvedAlias = credential.DeriveAlias(tok.WorkspaceName, credential.WorkspaceAliases(cfg.Workspaces))
	}

	storage, err := credential.StoreWorkspace(resolvedAlias, config.Workspace{
		WorkspaceID:   tok.WorkspaceID,
		WorkspaceName: tok.WorkspaceName,
		WorkspaceIcon: tok.WorkspaceIcon,
		BotID:         tok.BotID,
		AuthType:      config.AuthOAuth,
		AccessToken:   tok.AccessToken,
		RefreshToken:  tok.RefreshToken,
		Owner:         ownerFromToken(tok.Owner),
	}, kc)
	if err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}

	return emitItem(g, map[string]any{
		"ok":      true,
		"storage": storage,
		"workspace": map[string]any{
			"alias":   resolvedAlias,
			"name":    tok.WorkspaceName,
			"id":      tok.WorkspaceID,
			"bot_id":  tok.BotID,
			"default": config.Read().DefaultWorkspace == resolvedAlias,
		},
		"hint": "add more workspaces with 'agent-notion auth login --alias <name>'",
	})
}

func ownerFromToken(o *oauth.Owner) *config.Owner {
	if o == nil {
		return nil
	}
	owner := &config.Owner{Type: "user"}
	owner.User.ID = o.User.ID
	owner.User.Name = o.User.Name
	owner.User.Email = o.User.Person.Email
	return owner
}

func randomState() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
