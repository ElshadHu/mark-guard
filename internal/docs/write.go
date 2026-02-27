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

	// Verify the file exists
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("target file does not exist: %s (mark-guard updates existing docs, not creates new ones)", relPath)
		}
		return fmt.Errorf("stat %s: %w", relPath, err)
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
