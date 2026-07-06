package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func registerUsage(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "usage",
		Short: "LLM-optimized usage overview",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), rootUsageText)
			return err
		},
	})
}

// addDomainUsage registers a group's LLM usage card. Group files call it
// from their registerX function (before attachDomainUsage runs), keeping
// each group self-contained instead of editing a central map.
func addDomainUsage(name, text string) { domainUsage[name] = text }

// attachDomainUsage adds a `usage` subcommand to each command group that has
// a detail card in domainUsage. Call after all groups are registered.
func attachDomainUsage(root *cobra.Command) {
	for _, sub := range root.Commands() {
		text, ok := domainUsage[sub.Name()]
		if !ok {
			continue
		}
		sub.AddCommand(&cobra.Command{
			Use:   "usage",
			Short: "Detailed " + sub.Name() + " documentation for LLMs",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), text)
				return err
			},
		})
	}
}
