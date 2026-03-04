package cli

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/ElshadHu/mark-guard/internal/config"
	"github.com/ElshadHu/mark-guard/internal/docs"
	"github.com/ElshadHu/mark-guard/internal/git"
	"github.com/ElshadHu/mark-guard/internal/llm"
	"github.com/ElshadHu/mark-guard/internal/model"
	"github.com/ElshadHu/mark-guard/internal/symbols"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// FormatOptions holds all flags for the format command
type FormatOptions struct {
	BaseRef    string
	ConfigPath string
	MaxTokens  int
	Write      bool
	Debug      bool
	Force      bool
}

// newFormatCmd creates and returns the format subcommand
func newFormatCmd() *cobra.Command {
	opts := &FormatOptions{}
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Detect changed Go exports and update docs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFormat(opts)
		},
	}
	cmd.Flags().StringVar(&opts.BaseRef, "base", "HEAD", "git ref to compare against")
	cmd.Flags().StringVar(&opts.ConfigPath, "config", ".markguard.yaml", "path to config file")
	cmd.Flags().IntVar(&opts.MaxTokens, "max-tokens", 50000, "abort if estimated tokens exceed this limit")
	cmd.Flags().BoolVar(&opts.Write, "write", false, "apply changes to doc files (default: dry-run)")
	cmd.Flags().BoolVar(&opts.Debug, "debug", false, "print diff summary, full prompt, and raw LLM response")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "bypass content-loss safety checks")
	return cmd
}

func runFormat(opts *FormatOptions) error {
	// Load config
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Init git client
	gitClient, err := git.NewClient("", opts.BaseRef)
	if err != nil {
		return fmt.Errorf("init git client: %w", err)
	}
	// Get changed go files
	files, err := gitClient.ChangedGoFiles()
	if err != nil {
		return fmt.Errorf("detect changes: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("no Go file changes detected")
		return nil
	}

	// Extract symbols and diff
	allDiffs, changedCodePaths := extractAndDiff(files)
	diffSummary := symbols.FormatDiffSummaryCompact(allDiffs)
	if diffSummary == "No changes to exported symbols" {
		fmt.Println("Go files changed, but no exported API changes detected")
		return nil
	}
	fmt.Printf("%d exported API change(s) detected\n", len(allDiffs))
	if opts.Debug {
		// Verbose version for humans — shows per-field changes
		fmt.Println("\n== Diff Summary ==")
		fmt.Println(symbols.FormatDiffSummary(allDiffs))
	}

	// Scan docs
	scanResult, err := docs.Scan(&docs.ScanOptions{
		RepoRoot:         gitClient.RepoRoot(),
		Paths:            cfg.Docs.Paths,
		Exclude:          cfg.Docs.Exclude,
		Mappings:         cfg.Docs.Mappings,
		ChangedCodePaths: changedCodePaths,
	})
	if err != nil {
		return fmt.Errorf("scanning docs: %w", err)
	}
	if len(scanResult.Docs) == 0 {
		fmt.Println("no documentation files found to update")
		return nil
	}
	fmt.Printf("scanning %d doc file(s) (est. %d tokens)\n",
		len(scanResult.Docs), scanResult.EstimatedTokens)

	// Token budget check
	if scanResult.EstimatedTokens > opts.MaxTokens {
		return fmt.Errorf("estimated %d tokens exceeds --max-tokens %d\n"+
			"  Narrow scope: add docs.exclude or docs.mappings to .markguard.yaml",
			scanResult.EstimatedTokens, opts.MaxTokens)
	}

	// Build doc inputs and capture originals for validation
	docInputs := make([]model.DocInput, len(scanResult.Docs))
	originals := make(map[string]string, len(scanResult.Docs))
	for i := range scanResult.Docs {
		docInputs[i] = model.DocInput(scanResult.Docs[i])
		originals[scanResult.Docs[i].Path] = scanResult.Docs[i].Content
	}

	fmt.Printf("updating %d doc(s) via %s (parallel)...\n", len(docInputs), cfg.LLM.Model)

	client, err := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKeyEnv, cfg.LLM.Model)
	if err != nil {
		return fmt.Errorf("init LLM client: %w", err)
	}

	g, ctx := errgroup.WithContext(context.Background())
	var mu sync.Mutex
	allUpdates := make(map[string]string)

	for i := range docInputs {
		i := i
		g.Go(func() error {
			req := llm.BuildPrompt(diffSummary, []model.DocInput{docInputs[i]})

			if opts.Debug {
				fmt.Printf("\n== Prompt for %s (first 500 chars) ==\n", docInputs[i].Path)
				msg := req.Messages[len(req.Messages)-1].Content
				if len(msg) > 500 {
					msg = msg[:500] + "\n...(truncated)"
				}
				fmt.Println(msg)
			}

			resp, err := client.Complete(ctx, *req)
			if err != nil {
				return fmt.Errorf("LLM request for %s: %w", docInputs[i].Path, err)
			}

			if opts.Debug {
				fmt.Printf("\n== Raw LLM Response for %s ==\n", docInputs[i].Path)
				if len(resp.Choices) > 0 {
					fmt.Println(resp.Choices[0].Message.Content)
				}
			}

			updates, err := llm.ParseResponse(resp, map[string]string{
				docInputs[i].Path: docInputs[i].Content,
			})
			if err != nil {
				return fmt.Errorf("parsing response for %s: %w", docInputs[i].Path, err)
			}

			mu.Lock()
			for k, v := range updates {
				allUpdates[k] = v
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Validate updates unless --force is set
	blocked := make(map[string]bool)
	if !opts.Force {
		issues := docs.ValidateUpdates(originals, allUpdates)
		for i := range issues {
			if issues[i].Rejected {
				fmt.Fprintf(os.Stderr, "✗ %s: %s — blocked\n", issues[i].File, issues[i].Message)
				blocked[issues[i].File] = true
			} else {
				fmt.Fprintf(os.Stderr, "⚠ %s: %s\n", issues[i].File, issues[i].Message)
			}
		}
	}

	if !opts.Write {
		fmt.Println("\n== Dry Run (pass --write to apply) ==")
		for path := range allUpdates {
			if blocked[path] {
				fmt.Printf("  would block: %s\n", path)
			} else {
				fmt.Printf("  would update: %s\n", path)
			}
		}
		return nil
	}

	repoRoot := gitClient.RepoRoot()
	var wrote, skipped int
	for path, content := range allUpdates {
		if blocked[path] {
			skipped++
			continue
		}
		if err := docs.WriteUpdate(repoRoot, path, content); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		fmt.Printf("✓ updated %s\n", path)
		wrote++
	}

	if skipped > 0 {
		fmt.Printf("\n%d file(s) updated, %d blocked. Use --force to override.\n", wrote, skipped)
	}

	return nil
}

// extractAndDiff processes all changed files and returns collected diffs
// and the list of changed code paths (for doc mapping)
func extractAndDiff(files []git.ChangedFile) (diffs []symbols.SymbolDiff, changedPaths []string) {
	var allDiffs []symbols.SymbolDiff
	changedCodePaths := make([]string, len(files))
	for i := range files {
		changedCodePaths[i] = files[i].Path
		oldSyms, err := symbols.ExtractSymbols(files[i].Path, files[i].OldContent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not parse old %s: %v\n", files[i].Path, err)
		}
		newSyms, err := symbols.ExtractSymbols(files[i].Path, files[i].NewContent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not parse new %s: %v\n", files[i].Path, err)
		}
		fileDiffs := symbols.Diff(oldSyms, newSyms)
		allDiffs = append(allDiffs, fileDiffs...)
	}
	return allDiffs, changedCodePaths
}
