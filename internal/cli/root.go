// Package cli assembles agent-notion's cobra command tree on top of
// lib-agent-cli (shared root, persistent flags, output contract).
package cli

import (
	libcli "github.com/shhac/lib-agent-cli/cli"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func newRoot(version string) *cobra.Command {
	g := &libcli.Globals{}
	root := libcli.NewRoot(libcli.Options{
		Use:           "agent-notion",
		Short:         "Notion CLI for humans and LLMs",
		Version:       version,
		Globals:       g,
		DefaultFormat: output.FormatNDJSON,
		UnknownHint:   "run 'agent-notion --help' for usage",
	})

	registerAuth(root, g)

	return root
}

// Run builds the root command and executes it, rendering any error through the
// family output contract and setting the exit code.
func Run(version string) {
	libcli.Run(newRoot(version))
}
