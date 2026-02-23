// Package cli defines the mark-guard CLI commands
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "mark-guard",
	Short:         "Keep your docs in sync with your Go code",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
