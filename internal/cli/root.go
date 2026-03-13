// Package cli defines the mark-guard CLI commands
package cli

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "mark-guard",
		Short: "Keep your docs in sync with your Go code",
	}
	rootCmd.AddCommand(newFormatCmd())
	rootCmd.AddCommand(newInitCmd())
	return rootCmd
}
