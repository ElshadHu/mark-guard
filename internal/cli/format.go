package cli

import (
	"fmt"

	"github.com/ElshadHu/mark-guard/internal/git"
	"github.com/spf13/cobra"
)

var baseRef string

// newFormatCmd creates and returns the format subcommand
func newFormatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Detect changed Go exports and update docs",
		RunE:  runFormat,
	}
	cmd.Flags().StringVar(&baseRef, "base", "HEAD", "git ref to compare against")
	return cmd
}

func runFormat(cmd *cobra.Command, args []string) error {
	client, err := git.NewClient("", baseRef)
	if err != nil {
		return fmt.Errorf("init git client: %w", err)
	}
	files, err := client.ChangedGoFiles()
	if err != nil {
		return fmt.Errorf("detect changes: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("no Go file changes detected")
		return nil
	}
	fmt.Printf("changed Go files (base: %s):\n\n", baseRef)
	for i := range files {
		fmt.Printf("  %-40s old: %6d bytes  new: %6d bytes\n",
			files[i].Path,
			len(files[i].OldContent),
			len(files[i].NewContent),
		)
	}
	fmt.Printf("\n%d file(s) changed\n", len(files))

	return nil
}
