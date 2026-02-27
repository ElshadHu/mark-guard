// Package git provides a client for interacting with git repos
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Git subcommands and flags used by this package.
const (
	gitCmdDiff     = "diff"
	gitCmdLsFiles  = "ls-files"
	gitCmdShow     = "show"
	gitCmdRevParse = "rev-parse"

	gitFlagNameOnly        = "--name-only"
	gitFlagOthers          = "--others"
	gitFlagExcludeStandard = "--exclude-standard"
	gitFlagShowToplevel    = "--show-toplevel"

	gitPathFilterGo = "*.go"
)

// Client interacts with git in a specific repository.
type Client struct {
	// repo root (resolved via git rev-parse)
	dir     string
	baseRef string
}

// ChangedFile represents a Go file that differs from the base ref.
type ChangedFile struct {
	// relative to repo root
	Path       string
	OldContent string
	NewContent string
}

// NewClient creates a git client for the repository at dir.
func NewClient(dir, baseRef string) (*Client, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		dir = wd
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path: %w", err)
	}

	c := &Client{dir: absDir, baseRef: baseRef}

	root, err := c.runGit(gitCmdRevParse, gitFlagShowToplevel)
	if err != nil {
		return nil, fmt.Errorf("not a git repository (or any parent): %w", err)
	}

	c.dir = root
	return c, nil
}

// RepoRoot returns the absolute path to the repository root.
func (c *Client) RepoRoot() string {
	return c.dir
}

// ChangedGoFiles returns Go files that differ between the base ref and the
func (c *Client) ChangedGoFiles() ([]ChangedFile, error) {
	paths, err := c.changedPaths()
	if err != nil {
		return nil, err
	}

	files := make([]ChangedFile, 0, len(paths))
	for i := range paths {
		cf, err := c.buildChangedFile(paths[i])
		if err != nil {
			return nil, fmt.Errorf("process %s: %w", paths[i], err)
		}
		files = append(files, cf)
	}

	return files, nil
}

// changedPaths returns a deduplicated, sorted list of .go file paths that
func (c *Client) changedPaths() ([]string, error) {
	diffOutput, err := c.runGit(gitCmdDiff, gitFlagNameOnly, c.baseRef, "--", gitPathFilterGo)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	untrackedOutput, err := c.runGit(gitCmdLsFiles, gitFlagOthers, gitFlagExcludeStandard, "--", gitPathFilterGo)
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	seen := make(map[string]struct{})
	var paths []string

	for _, line := range splitLines(diffOutput) {
		if line == "" {
			continue
		}
		if _, exists := seen[line]; !exists {
			seen[line] = struct{}{}
			paths = append(paths, line)
		}
	}

	for _, line := range splitLines(untrackedOutput) {
		if line == "" {
			continue
		}
		if _, exists := seen[line]; !exists {
			seen[line] = struct{}{}
			paths = append(paths, line)
		}
	}

	sort.Strings(paths)
	return paths, nil
}

// buildChangedFile constructs a ChangedFile by reading old content from git
func (c *Client) buildChangedFile(path string) (ChangedFile, error) {
	cf := ChangedFile{Path: path}
	cf.OldContent = c.readFileAtRef(path)
	cf.NewContent = c.readFileOnDisk(path)
	return cf, nil
}

// readFileAtRef returns the file content at the base ref.
// Returns empty string if the file does not exist at that ref (new file).
func (c *Client) readFileAtRef(path string) string {
	content, err := c.runGit(gitCmdShow, c.baseRef+":"+path)
	if err != nil {
		// File doesn't exist at base ref
		return ""
	}
	return content
}

// readFileOnDisk returns the file content on the working tree.
// Returns empty string if the file does not exist (deleted file)
func (c *Client) readFileOnDisk(path string) string {
	absPath := filepath.Join(c.dir, path)
	data, err := os.ReadFile(absPath)
	if err != nil {
		// File doesn't exist on disk
		return ""
	}
	return string(data)
}

// runGit runs a git command with the given arguments in the client's
// repository directory. Returns stdout with trailing newlines trimmed.
func (c *Client) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = c.dir

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf(
				"git %s: exit code %d (stderr: %s)",
				strings.Join(args, " "),
				exitErr.ExitCode(),
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return strings.TrimRight(string(output), "\n"), nil
}

// splitLines splits output by newlines, handling both \n and \r\n.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(s, "\n")
}
