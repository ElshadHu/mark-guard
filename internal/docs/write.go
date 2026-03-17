// Package docs handles reading and writing markdown documentation files
package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteUpdate writes updated documentation content to a file.
// The path is relative to repoRoot.
func WriteUpdate(repoRoot, relPath, content string) error {
	if content == "" {
		return fmt.Errorf("refusing to write empty content to %s", relPath)
	}
	absPath := filepath.Join(repoRoot, relPath)

	// ensure resolved path is inside the repo root
	if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(repoRoot)) {
		return fmt.Errorf("path traversal blocked: %s resolves outside repo root", relPath)
	}

	// Verify the file exists and get its permissions
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("target file does not exist: %s (mark-guard updates existing docs, not creates new ones)", relPath)
		}
		return fmt.Errorf("stat %s: %w", relPath, err)
	}

	// Ensure content ends with newline
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	// Preserve original file permissions instead of hardcoding 0o644
	if err := os.WriteFile(absPath, []byte(content), info.Mode().Perm()); err != nil {
		return fmt.Errorf("writing %s: %w", relPath, err)
	}
	return nil
}

// WriteNew creates a new documentation file. The path is relative to repoRoot.
func WriteNew(repoRoot, relPath, content string, force bool) error {
	if content == "" {
		return fmt.Errorf("refusing to write empty content to %s", relPath)
	}

	absPath := filepath.Join(repoRoot, relPath)

	// Ensure resolved path is inside the repo root (same guard as WriteUpdate)
	if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(repoRoot)) {
		return fmt.Errorf("path traversal blocked: %s resolves outside repo root", relPath)
	}

	// Check if file already exists
	if _, err := os.Stat(absPath); err == nil && !force {
		return fmt.Errorf("file already exists: %s (use --force to overwrite)", relPath)
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Ensure content ends with newline
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", relPath, err)
	}
	return nil
}

// AppendToFile appends content to an existing file. The path is relative to
// repoRoot. The file must already exist — this never creates a new file.
func AppendToFile(repoRoot, relPath, content string) error {
	if content == "" {
		return fmt.Errorf("refusing to append empty content to %s", relPath)
	}

	absPath := filepath.Join(repoRoot, relPath)

	// Ensure resolved path is inside the repo root
	if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(repoRoot)) {
		return fmt.Errorf("path traversal blocked: %s resolves outside repo root", relPath)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("target file does not exist: %s (use 'generate --output <dir>' to create new files)", relPath)
		}
		return fmt.Errorf("stat %s: %w", relPath, err)
	}

	// Ensure content starts with a blank line separator and ends with a newline
	if !strings.HasPrefix(content, "\n") {
		content = "\n" + content
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	f, err := os.OpenFile(absPath, os.O_APPEND|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("opening %s: %w", relPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("appending to %s: %w", relPath, err)
	}
	return nil
}
