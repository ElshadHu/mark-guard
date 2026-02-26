// Package docs handles reading and writing markdown documentation files
package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DocFile represents a markdown file found during scanning
type DocFile struct {
	// Path is relative to the repo root
	Path string
	// Content is the full file content
	Content string
}

// DocMapping links doc files to code paths.
// If any changed code file has a path starting with a Code prefix,
// the mapped Docs files are included in the scan result.
type DocMapping struct {
	Docs []string
	Code []string
}

// ScanOptions groups all inputs for the Scan function.
type ScanOptions struct {
	// RepoRoot is the base directory for resolving relative paths
	RepoRoot string
	// Paths are directories and files to scan for md files
	Paths []string
	// Exclude are relative paths to skip
	Exclude []string
	// Mappings link doc files to code paths
	Mappings []DocMapping
	// ChangedCodePaths are file paths from git diff
	ChangedCodePaths []string
}

// ScanResult contains the selected docs and size metadata
// for token budget estimation.
type ScanResult struct {
	// Docs are the selected md files
	Docs []DocFile
	// TotalBytes is the sum of all doc content sizes
	TotalBytes int
	// EstimatedTokens is a rough estimate: TotalBytes / 4
	EstimatedTokens int
}

// Scan finds and reads markdown files from the configured paths,
// filtered by config-based mappings to changed code paths.
// If no mappings are configured or no mapping matches,
// all docs are returned (small repo / zero-config fallback).
func Scan(opts *ScanOptions) (*ScanResult, error) {
	allDocs, err := collectDocs(opts.RepoRoot, opts.Paths, opts.Exclude)
	if err != nil {
		return nil, fmt.Errorf("collecting docs: %w", err)
	}

	if len(opts.Mappings) == 0 {
		return buildResult(allDocs), nil
	}

	// Find which doc paths are mapped to the changed code
	mappedDocPaths := matchMappings(opts.Mappings, opts.ChangedCodePaths)
	if len(mappedDocPaths) == 0 {
		return buildResult(allDocs), nil
	}

	var matched []DocFile
	for i := range allDocs {
		if mappedDocPaths[allDocs[i].Path] {
			matched = append(matched, allDocs[i])
		}
	}

	// If mapped docs were configured but none were found in allDocs
	// (e.g., mapping references a file not under paths), fallback
	if len(matched) == 0 {
		return buildResult(allDocs), nil
	}

	return buildResult(matched), nil
}

// buildResult constructs a ScanResult with token estimation.
func buildResult(docs []DocFile) *ScanResult {
	total := 0
	for i := range docs {
		total += len(docs[i].Content)
	}
	return &ScanResult{
		Docs:            docs,
		TotalBytes:      total,
		EstimatedTokens: total / 4,
	}
}

// matchMappings returns the set of doc paths
// that are mapped to any of the changed code paths.
func matchMappings(mappings []DocMapping, changedCodePaths []string) map[string]bool {
	result := make(map[string]bool)
	for i := range mappings {
		if mappingMatches(mappings[i], changedCodePaths) {
			for _, docPath := range mappings[i].Docs {
				result[docPath] = true
			}
		}
	}
	return result
}

// mappingMatches returns true if any changed code path
// starts with any of the mapping's code prefixes.
func mappingMatches(m DocMapping, changedCodePaths []string) bool {
	for _, codePath := range changedCodePaths {
		for _, prefix := range m.Code {
			if strings.HasPrefix(codePath, prefix) {
				return true
			}
		}
	}
	return false
}

// collectDocs walks the configured paths and reads all .md files,
// skipping excluded paths.
func collectDocs(repoRoot string, paths, exclude []string) ([]DocFile, error) {
	excludeSet := make(map[string]bool, len(exclude))
	for i := range exclude {
		excludeSet[exclude[i]] = true
	}

	var docs []DocFile
	seen := make(map[string]bool)

	for i := range paths {
		absPath := filepath.Join(repoRoot, paths[i])

		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Path doesn't exist - skip silently.
				// This is expected: user may configure "README.md"
				// but the repo doesn't have one.
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", paths[i], err)
		}

		if !info.IsDir() {
			doc, err := readSingleFile(absPath, paths[i], excludeSet, seen)
			if err != nil {
				return nil, err
			}
			if doc != nil {
				docs = append(docs, *doc)
			}
			continue
		}

		// Directory: walk recursively
		walkErr := filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("walking %s: %w", path, err)
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}

			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return fmt.Errorf("computing relative path for %s: %w", path, err)
			}

			if excludeSet[rel] || seen[rel] {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", rel, err)
			}

			seen[rel] = true
			docs = append(docs, DocFile{Path: rel, Content: string(content)})
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("scanning %s: %w", paths[i], walkErr)
		}
	}

	return docs, nil
}

// readSingleFile reads a single .md file if it passes filters.
// Returns nil (no error) if the file should be skipped.
func readSingleFile(absPath, rel string, excludeSet, seen map[string]bool) (*DocFile, error) {
	if excludeSet[rel] || seen[rel] {
		return nil, nil
	}
	if !strings.HasSuffix(rel, ".md") {
		return nil, nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", rel, err)
	}

	seen[rel] = true
	return &DocFile{Path: rel, Content: string(content)}, nil
}
