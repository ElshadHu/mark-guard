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

// errPathTraversal is returned when a path resolves outside the repo root.
var errPathTraversal = errors.New("path traverses outside repository root")

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
	dir     string
	baseRef string
}

// ChangedFile represents a Go file that differs from the base ref.
type ChangedFile struct {
	Path       string // relative to repo root
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

	absDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
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

// ChangedGoFiles returns Go files that differ between the base ref and the working tree.
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

// changedPaths returns a deduplicated, sorted list of changed .go file paths.
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

func (c *Client) buildChangedFile(path string) (ChangedFile, error) {
	cf := ChangedFile{Path: path}

	old, err := c.readFileAtRef(path)
	if err != nil {
		return ChangedFile{}, fmt.Errorf("read %s at %s: %w", path, c.baseRef, err)
	}
	cf.OldContent = old

	cur, err := c.readFileOnDisk(path)
	if err != nil {
		return ChangedFile{}, fmt.Errorf("read %s on disk: %w", path, err)
	}
	cf.NewContent = cur

	return cf, nil
}

// readFileAtRef returns file content at the base ref, or "" for new files.
func (c *Client) readFileAtRef(path string) (string, error) {
	content, err := c.runGit(gitCmdShow, c.baseRef+":"+path)
	if err == nil {
		return content, nil
	}
	if strings.Contains(err.Error(), "exit code 128") {
		return "", nil
	}
	return "", err
}

// readFileOnDisk returns file content from the working tree, or "" for deleted files.
func (c *Client) readFileOnDisk(path string) (string, error) {
	absPath, err := c.safePath(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// safePath validates that path stays within c.dir after joining.
func (c *Client) safePath(path string) (string, error) {
	joined := filepath.Join(c.dir, path)
	cleaned := filepath.Clean(joined)
	if !strings.HasPrefix(cleaned, c.dir+string(filepath.Separator)) && cleaned != c.dir {
		return "", fmt.Errorf("%w: %s", errPathTraversal, path)
	}
	return cleaned, nil
}

// runGit runs a git command and returns stdout with trailing newlines trimmed.
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
