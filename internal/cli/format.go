package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/ElshadHu/mark-guard/internal/config"
	"github.com/ElshadHu/mark-guard/internal/docs"
	"github.com/ElshadHu/mark-guard/internal/git"
	"github.com/ElshadHu/mark-guard/internal/llm"
	"github.com/ElshadHu/mark-guard/internal/symbols"
	"github.com/spf13/cobra"
)

var (
	baseRef    string
	configPath string
)

// newFormatCmd creates and returns the format subcommand
func newFormatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Detect changed Go exports and update docs",
		RunE:  runFormat,
	}
	cmd.Flags().StringVar(&baseRef, "base", "HEAD", "git ref to compare against")
	cmd.Flags().StringVar(&configPath, "config", ".markguard.yaml", "path to config file")
	return cmd
}

func runFormat(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Init git client
	gitClient, err := git.NewClient("", baseRef)
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
	diffSummary := symbols.FormatDiffSummary(allDiffs)
	if diffSummary == "No changes to exported symbols" {
		fmt.Println("Go files changed, but no exported API changes detected")
		return nil
	}
	fmt.Printf("%d exported API change(s) detected\n", len(allDiffs))

	// Scan docs
	mappings := bridgeMappings(cfg.Docs.Mappings)
	scanResult, err := docs.Scan(&docs.ScanOptions{
		RepoRoot:         gitClient.RepoRoot(),
		Paths:            cfg.Docs.Paths,
		Exclude:          cfg.Docs.Exclude,
		Mappings:         mappings,
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

	// Build prompt
	docInputs := bridgeDocInputs(scanResult.Docs)
	req := llm.BuildPrompt(diffSummary, docInputs)

	// Call LLM
	fmt.Printf("updating docs via %s...\n", cfg.LLM.Model)
	client, err := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKeyEnv, cfg.LLM.Model)
	if err != nil {
		return fmt.Errorf("init LLM client: %w", err)
	}
	resp, err := client.Complete(context.Background(), *req)
	if err != nil {
		return fmt.Errorf("LLM request: %w", err)
	}

	// Parse response and write back
	docPaths := make([]string, len(scanResult.Docs))
	for i := range scanResult.Docs {
		docPaths[i] = scanResult.Docs[i].Path
	}

	updates, err := llm.ParseResponse(resp, docPaths)
	if err != nil {
		return fmt.Errorf("parsing LLM response: %w", err)
	}

	repoRoot := gitClient.RepoRoot()
	for path, content := range updates {
		if err := docs.WriteUpdate(repoRoot, path, content); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		fmt.Printf("✓ updated %s\n", path)
	}

	// Token report
	if resp.Usage != nil {
		fmt.Printf("tokens — input: %d, output: %d, total: %d\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
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
			// new file or unparseable old version
			oldSyms = nil
		}
		newSyms, err := symbols.ExtractSymbols(files[i].Path, files[i].NewContent)
		if err != nil {
			newSyms = nil
		}
		fileDiffs := symbols.Diff(oldSyms, newSyms)
		allDiffs = append(allDiffs, fileDiffs...)
	}
	return allDiffs, changedCodePaths
}

// bridgeMappings converts config.DocMapping to docs.DocMapping
// These are separate types to avoid config <-> docs circular imports
func bridgeMappings(cfgMapping []config.DocMapping) []docs.DocMapping {
	mappings := make([]docs.DocMapping, len(cfgMapping))
	for i, m := range cfgMapping {
		mappings[i] = docs.DocMapping{Docs: m.Docs, Code: m.Code}
	}
	return mappings
}

// bridgeDocInputs converts docs.DocFile to llm.DocInput.
// These are separate types to avoid docs↔llm circular imports.
func bridgeDocInputs(docFiles []docs.DocFile) []llm.DocInput {
	inputs := make([]llm.DocInput, len(docFiles))
	for i, d := range docFiles {
		inputs[i] = llm.DocInput{Path: d.Path, Content: d.Content}
	}
	return inputs
}
