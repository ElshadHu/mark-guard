package cli

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ElshadHu/mark-guard/internal/config"
	"github.com/ElshadHu/mark-guard/internal/docs"
	"github.com/ElshadHu/mark-guard/internal/llm"
	"github.com/ElshadHu/mark-guard/internal/symbols"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// GenerateOptions holds all flags for the generate command.
type GenerateOptions struct {
	ConfigPath string
	OutputDir  string
	MaxTokens  int
	Write      bool
	Debug      bool
	Force      bool
}

func newGenerateCmd() *cobra.Command {
	opts := &GenerateOptions{}
	cmd := &cobra.Command{
		Use:   "generate [paths...]",
		Short: "Generate initial API docs from exported Go symbols",
		Long: `Generate creates markdown documentation for Go packages that have no
docs yet. It parses all .go files in the target paths, extracts every
exported symbol, and sends them to the LLM to produce an API reference.

When --output points to a .md file (e.g. README.md), all generated docs
are appended to that file. When it points to a directory, one file per
package is created inside it.

By default it runs in dry-run mode. Pass --write to apply changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"."}
			}
			return runGenerate(opts, args)
		},
	}
	cmd.Flags().StringVar(&opts.ConfigPath, "config", ".markguard.yaml", "path to config file")
	cmd.Flags().StringVar(&opts.OutputDir, "output", "", "output directory or .md file to append to (default: first entry in config docs.paths)")
	cmd.Flags().IntVar(&opts.MaxTokens, "max-tokens", 50000, "abort if estimated tokens exceed this limit")
	cmd.Flags().BoolVar(&opts.Write, "write", false, "apply changes (default: dry-run)")
	cmd.Flags().BoolVar(&opts.Debug, "debug", false, "print symbol list, full prompt, and raw LLM response")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite existing doc files (directory mode only)")
	return cmd
}

// packageSymbols groups symbols by their package name.
type packageSymbols struct {
	// Go package name (e.g. "llm")
	Name string
	// directory relative to repo root (e.g. "internal/llm")
	Dir string
	// all exported symbols in the package
	Symbols []symbols.Symbol
}

// generateResult holds the output for a single package.
type generateResult struct {
	relPath string
	pkgName string
	content string
}

// generateError records a per-package failure without aborting the pipeline.
type generateError struct {
	pkgName string
	err     error
}

func runGenerate(opts *GenerateOptions, paths []string) error {
	// Load config
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve output target: a .md file to append to, or a directory for per-package files.
	outputTarget := opts.OutputDir
	if outputTarget == "" {
		outputTarget = cfg.Generate.Output
	}
	if outputTarget == "" && len(cfg.Docs.Paths) > 0 {
		outputTarget = cfg.Docs.Paths[0]
	}
	if outputTarget == "" {
		outputTarget = "docs/"
	}
	// appendMode: output is a single .md file; all content is appended there.
	appendMode := strings.HasSuffix(outputTarget, ".md")
	var outputDir string
	if !appendMode {
		// Ensure trailing slash so filepath.Join works cleanly
		outputDir = strings.TrimRight(outputTarget, "/") + "/"
	}

	// Discover and parse all Go files -> group by package
	repoRoot, err := resolveRepoRoot()
	if err != nil {
		return err
	}
	packages, parseErrs, err := discoverPackages(repoRoot, paths)
	if err != nil {
		return err
	}

	// Surface every parse error so nothing is silently lost
	if len(parseErrs) > 0 {
		fmt.Fprintf(os.Stderr, "%d file(s) had errors during discovery:\n", len(parseErrs))
		for _, pe := range parseErrs {
			fmt.Fprintf(os.Stderr, "  ✗ %s\n", pe)
		}
	}

	if len(packages) == 0 {
		fmt.Println("no Go packages with exported symbols found")
		return nil
	}

	// Estimate token budget (rough: signature bytes / 4)
	var totalBytes int
	for i := range packages {
		summary := symbols.FormatSymbolList(packages[i].Symbols)
		totalBytes += len(summary)
	}
	estimatedTokens := totalBytes / 4
	fmt.Printf("found %d package(s), %d exported symbol(s) (est. %d tokens)\n",
		len(packages), countSymbols(packages), estimatedTokens)

	if estimatedTokens > opts.MaxTokens {
		return fmt.Errorf("estimated %d tokens exceeds --max-tokens %d\n"+
			"  Narrow scope: pass specific package paths as arguments",
			estimatedTokens, opts.MaxTokens)
	}

	// Init LLM client
	client, err := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKeyEnv, cfg.LLM.Model)
	if err != nil {
		return fmt.Errorf("init LLM client: %w", err)
	}

	fmt.Printf("generating docs for %d package(s) via %s...\n", len(packages), cfg.LLM.Model)

	// Generate docs in parallel — collect successes AND failures instead of
	// aborting the entire pipeline on the first error.
	var mu sync.Mutex
	var results []generateResult
	var genErrors []generateError

	g, ctx := errgroup.WithContext(context.Background())
	for i := range packages {
		pkg := packages[i]
		g.Go(func() error {
			summary := symbols.FormatSymbolList(pkg.Symbols)
			if opts.Debug {
				fmt.Printf("\n== Symbols for package %s ==\n%s\n", pkg.Name, summary)
			}

			req := llm.BuildGeneratePrompt(summary, pkg.Name)
			if opts.Debug {
				fmt.Printf("\n== Prompt for %s (first 500 chars) ==\n", pkg.Name)
				msg := req.Messages[len(req.Messages)-1].Content
				if len(msg) > 500 {
					msg = msg[:500] + "\n...(truncated)"
				}
				fmt.Println(msg)
			}

			resp, err := client.Complete(ctx, *req)
			if err != nil {
				mu.Lock()
				genErrors = append(genErrors, generateError{pkgName: pkg.Name, err: err})
				mu.Unlock()
				// Don't return the error — let other packages continue.
				return nil
			}
			if opts.Debug && len(resp.Choices) > 0 {
				fmt.Printf("\n== Raw LLM Response for %s ==\n%s\n", pkg.Name, resp.Choices[0].Message.Content)
			}

			content, err := llm.ParseGenerateResponse(resp)
			if err != nil {
				mu.Lock()
				genErrors = append(genErrors, generateError{pkgName: pkg.Name, err: err})
				mu.Unlock()
				return nil
			}

			// In append mode the relPath is the target file; in directory mode
			// it's <outputDir>/<pkgName>.md.
			var outPath string
			if appendMode {
				outPath = outputTarget
			} else {
				outPath = filepath.Join(outputDir, pkg.Name+".md")
			}
			mu.Lock()
			results = append(results, generateResult{relPath: outPath, pkgName: pkg.Name, content: content})
			mu.Unlock()
			return nil
		})
	}
	// errgroup.Wait blocks until all goroutines finish.
	_ = g.Wait()

	// Report per-package failures
	if len(genErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d package(s) failed:\n", len(genErrors))
		for _, ge := range genErrors {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", ge.pkgName, ge.err)
		}
	}

	// If every package failed, hard error
	if len(results) == 0 {
		return fmt.Errorf("all %d package(s) failed — no docs generated", len(genErrors))
	}

	sortResults(results)

	if appendMode {
		return writeAppendMode(repoRoot, outputTarget, results, genErrors, opts.Write)
	}
	return writeDirectoryMode(repoRoot, results, genErrors, opts.Write, opts.Force)
}

// writeAppendMode concatenates all generated docs and appends them to a
// single existing .md file (e.g. README.md).
func writeAppendMode(repoRoot, targetFile string, results []generateResult, genErrors []generateError, write bool) error {
	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(r.content)
		if i < len(results)-1 {
			sb.WriteString("\n\n---\n")
		}
	}
	combined := sb.String()

	if !write {
		fmt.Println("\n== Dry Run (pass --write to append) ==")
		fmt.Printf("  would append to: %s (%d bytes across %d package(s))\n",
			targetFile, len(combined), len(results))
		for _, r := range results {
			fmt.Printf("    - %s (%d bytes)\n", r.pkgName, len(r.content))
		}
		return nil
	}

	if err := docs.AppendToFile(repoRoot, targetFile, combined); err != nil {
		return fmt.Errorf("appending to %s: %w", targetFile, err)
	}
	fmt.Printf("✓ appended %d package(s) to %s\n", len(results), targetFile)

	if len(genErrors) > 0 {
		fmt.Printf("%d package(s) failed — check errors above.\n", len(genErrors))
	}
	return nil
}

// writeDirectoryMode writes one .md file per package into the output directory.
func writeDirectoryMode(repoRoot string, results []generateResult, genErrors []generateError, write bool, force bool) error {
	if !write {
		fmt.Println("\n== Dry Run (pass --write to create files) ==")
		for _, r := range results {
			fmt.Printf("  would create: %s (%d bytes)\n", r.relPath, len(r.content))
		}
		return nil
	}

	var wrote, writeErrors int
	for _, r := range results {
		if err := docs.WriteNew(repoRoot, r.relPath, r.content, force); err != nil {
			fmt.Fprintf(os.Stderr, "✗ write failed: %v\n", err)
			writeErrors++
			continue
		}
		fmt.Printf("✓ created %s\n", r.relPath)
		wrote++
	}

	totalFailed := len(genErrors) + writeErrors
	if totalFailed > 0 {
		fmt.Printf("\n%d file(s) generated, %d failed. Check errors above.\n", wrote, totalFailed)
	} else {
		fmt.Printf("\n%d file(s) generated\n", wrote)
	}
	return nil
}

// sortResults sorts by package name for deterministic output ordering.
func sortResults(results []generateResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].pkgName < results[j-1].pkgName; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}

// resolveRepoRoot returns the current working directory with symlinks resolved.
func resolveRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks: %w", err)
	}
	return resolved, nil
}

// discoverPackages walks the given paths and extracts exported symbols,
func discoverPackages(repoRoot string, paths []string) ([]packageSymbols, []error, error) {
	pkgMap := make(map[string]*packageSymbols)
	var parseErrors []error

	for _, p := range paths {
		absPath := p
		if !filepath.IsAbs(p) {
			absPath = filepath.Join(repoRoot, p)
		}

		err := filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip directories that should never be scanned
			if d.IsDir() {
				name := d.Name()
				if name == "vendor" || name == "testdata" || name == ".git" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}

			// Only .go files, skip tests
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				parseErrors = append(parseErrors, fmt.Errorf("read %s: %w", path, readErr))
				return nil
			}
			if isGeneratedFile(string(content)) {
				return nil
			}

			relPath, err := filepath.Rel(repoRoot, path)
			if err != nil {
				relPath = path
			}

			syms, err := symbols.ExtractSymbols(relPath, string(content))
			if err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("parse %s: %w", relPath, err))
				return nil
			}
			if len(syms) == 0 {
				return nil
			}

			dir := filepath.Dir(relPath)
			pkgName := detectPackageName(string(content))

			if existing, ok := pkgMap[dir]; ok {
				existing.Symbols = append(existing.Symbols, syms...)
			} else {
				pkgMap[dir] = &packageSymbols{
					Name:    pkgName,
					Dir:     dir,
					Symbols: syms,
				}
			}
			return nil
		})
		if err != nil {
			return nil, parseErrors, fmt.Errorf("walking %s: %w", p, err)
		}
	}

	result := make([]packageSymbols, 0, len(pkgMap))
	for _, ps := range pkgMap {
		result = append(result, *ps)
	}
	sortPackages(result)
	return result, parseErrors, nil
}

// detectPackageName extracts the package name using a lightweight parse.
func detectPackageName(src string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.PackageClauseOnly)
	if err != nil || f.Name == nil {
		return "unknown"
	}
	return f.Name.Name
}

// isGeneratedFile checks for the standard "Code generated" header.
func isGeneratedFile(content string) bool {
	for _, line := range strings.SplitN(content, "\n", 20) {
		if strings.HasPrefix(line, "// Code generated") && strings.HasSuffix(line, "DO NOT EDIT.") {
			return true
		}
	}
	return false
}

// sortPackages sorts by directory path for deterministic output.
func sortPackages(pkgs []packageSymbols) {
	for i := 1; i < len(pkgs); i++ {
		for j := i; j > 0 && pkgs[j].Dir < pkgs[j-1].Dir; j-- {
			pkgs[j], pkgs[j-1] = pkgs[j-1], pkgs[j]
		}
	}
}

// countSymbols returns the total number of symbols across all packages.
func countSymbols(pkgs []packageSymbols) int {
	var n int
	for i := range pkgs {
		n += len(pkgs[i].Symbols)
	}
	return n
}
