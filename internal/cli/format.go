package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var baseRef string

var formatCmd = &cobra.Command{
	Use:   "format",
	Short: "Detect code changes and update documentation",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("detecting changes...")
		return nil
	},
}

func init() {
	formatCmd.Flags().StringVar(&baseRef, "base", "HEAD", "git ref to compare against")
	rootCmd.AddCommand(formatCmd)
}
