// Package cli defines the mark-guard CLI commands
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "mark-guard",
		Short: "Keep your docs in sync with your Go code",
	}
	rootCmd.AddCommand(newFormatCmd())
	rootCmd.AddCommand(newInitCmd())
	return rootCmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of mark-guard",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("mark-guard " + Version)
		},
	}
}
