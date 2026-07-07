// Package cli assembles agent-notion's cobra command tree on top of
// lib-agent-cli (shared root, persistent flags, output contract).
package cli

import (
	"fmt"

	"github.com/shhac/agent-notion/internal/auth"
	"github.com/shhac/agent-notion/internal/config"
	"github.com/shhac/agent-notion/internal/credential"
	"github.com/shhac/agent-notion/internal/oauth"
	libcli "github.com/shhac/lib-agent-cli/cli"
	agentmcp "github.com/shhac/lib-agent-mcp"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"

	// Registers the family YAML encoder so --format yaml renders.
	_ "github.com/shhac/lib-agent-cli/yaml"
)

// backendModes are the accepted --backend values.
var backendModes = []string{"auto", "official", "v3"}

func newRoot(version string) *cobra.Command {
	return newRootWithDeps(rootDeps{
		version:        version,
		keychain:       credential.DefaultKeychainStore,
		desktopExtract: auth.ExtractDesktop,
		browserImport:  auth.ImportBrowser,
		openBrowser:    oauth.OpenBrowser,
	})
}

func newRootWithDeps(deps rootDeps) *cobra.Command {
	g := &GlobalFlags{
		version:        deps.version,
		keychain:       deps.keychain,
		desktopExtract: deps.desktopExtract,
		browserImport:  deps.browserImport,
		openBrowser:    deps.openBrowser,
	}
	root := libcli.NewRoot(libcli.Options{
		Use:           "agent-notion",
		Short:         "Notion CLI for humans and LLMs",
		Version:       deps.version,
		Globals:       &g.Globals,
		DefaultFormat: output.FormatNDJSON,
		UnknownHint:   "run 'agent-notion --help' for usage",
	})

	// NewRoot binds --format/--timeout/--debug/--color and validates --format
	// up front. Extend its PersistentPreRunE to wire the stdout/stderr seams
	// tests inject via SetOut/SetErr and validate --backend; cobra runs only
	// the nearest PersistentPreRunE, so subcommands must not define their own.
	innerPreRun := root.PersistentPreRunE
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		g.stdout = cmd.OutOrStdout()
		g.stderr = cmd.ErrOrStderr()
		if err := innerPreRun(cmd, args); err != nil {
			return err
		}
		return validateBackendMode(g.Backend)
	}

	root.PersistentFlags().StringVar(&g.Backend, "backend", "auto",
		"API backend: auto (by workspace auth type), official, or v3")
	root.PersistentFlags().StringVar(&g.Expand, "expand", "",
		"Expand truncated fields (comma-separated: description,body,content)")
	root.PersistentFlags().BoolVar(&g.Full, "full", false,
		"Show full content for all truncated fields")
	root.PersistentFlags().StringVar(&g.BaseURL, "base-url", "",
		"Override the Notion API base URL (testing)")
	_ = root.PersistentFlags().MarkHidden("base-url")
	_ = root.RegisterFlagCompletionFunc("backend", fixedCompletions(backendModes...))

	registerAuth(root, g)
	registerSearch(root, g)
	registerPage(root, g)
	registerBlock(root, g)
	registerDatabase(root, g)
	registerUser(root, g)
	registerComment(root, g)
	registerExport(root, g)
	registerActivity(root, g)
	registerAI(root, g)
	registerConfig(root, g)
	registerUsage(root)
	attachDomainUsage(root)

	// Expose the data groups as MCP tools (one coarse tool per group);
	// credential/config/usage commands are deliberately left out — they are
	// not agent tasks. Read-only groups are annotated so hosts can gate
	// mutations. Added last so the server reflects the complete tree.
	exposeGroups(root, "search", "page", "block", "database", "comment", "user",
		"export", "activity", "ai")
	markReadOnly(root, "search", "user", "activity")
	skipGroups(root, "auth", "config")
	root.AddCommand(agentmcp.Command(root,
		agentmcp.WithHiddenFlags("color"),
		// Local-OAuth secrets for `mcp --oauth local` live under a separate
		// reverse-DNS service from the API credentials.
		agentmcp.WithOAuthKeyringService(config.KeychainService+".mcp"),
	))

	return root
}

// exposeGroups opts the named top-level commands into the MCP tool surface.
// A name with no matching command is skipped silently — the list is a
// curation of agent-facing groups, not a registration check.
func exposeGroups(root *cobra.Command, names ...string) {
	forEachNamed(root, names, agentmcp.Expose)
}

// markReadOnly annotates groups whose every leaf is read-only.
func markReadOnly(root *cobra.Command, names ...string) {
	forEachNamed(root, names, agentmcp.ReadOnly)
}

// skipGroups keeps the named groups out of the MCP tool surface entirely.
func skipGroups(root *cobra.Command, names ...string) {
	forEachNamed(root, names, agentmcp.Skip)
}

func forEachNamed(root *cobra.Command, names []string, mark func(*cobra.Command)) {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	for _, c := range root.Commands() {
		if want[c.Name()] {
			mark(c)
		}
	}
}

func validateBackendMode(mode string) error {
	for _, m := range backendModes {
		if mode == m {
			return nil
		}
	}
	return output.New(fmt.Sprintf("unknown backend %q, expected one of: auto, official, v3", mode),
		output.FixableByAgent)
}

// fixedCompletions returns a ValidArgsFunction offering a fixed word list.
func fixedCompletions(words ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return words, cobra.ShellCompDirectiveNoFileComp
	}
}

// Run builds the root command and executes it, rendering any error through the
// family output contract and setting the exit code.
func Run(version string) {
	libcli.Run(newRoot(version))
}
