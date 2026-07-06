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
