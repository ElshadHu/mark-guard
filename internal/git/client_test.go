package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestRepo creates a temp git repo with one commited .go file and returns the path
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s (%v)", args, out, err)
		}
	}

	// seed file so HEAD exists
	writeFile(t, dir, "committed.go", "package main\n")
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "init")
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitAdd(t *testing.T, dir, path string) {
	t.Helper()
	cmd := exec.Command("git", "add", path)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s (%v)", out, err)
	}
}

func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s (%v)", out, err)
	}
}

func TestChangedGoFiles_ModifiedFile(t *testing.T) {
	dir := initTestRepo(t)
	// mutate the committed file on disk (unstaged)
	writeFile(t, dir, "committed.go", "package main\n\nfunc hello() {}\n")
	c, err := NewClient(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	files, err := c.ChangedGoFiles()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 changed file, got %d", len(files))
	}

	f := files[0]
	if f.Path != "committed.go" {
		t.Errorf("path = %q, want committed.go", f.Path)
	}
	if f.OldContent != "package main" {
		t.Errorf("OldContent mismatch:\ngot:  %q\nwant: %q", f.OldContent, "package main")
	}
	if f.NewContent != "package main\n\nfunc hello() {}\n" {
		t.Errorf("NewContent mismatch:\ngot:  %q\nwant: %q", f.NewContent, "package main\n\nfunc hello() {}\n")
	}
}

func TestChangedGoFiles_NoChanges(t *testing.T) {
	dir := initTestRepo(t)

	c, err := NewClient(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	files, err := c.ChangedGoFiles()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 changed files, got %d: %+v", len(files), files)
	}
}

func TestChangedGoFiles_CustomBaseRef(t *testing.T) {
	dir := initTestRepo(t)

	// create a second commit on a branch, then diff against the first
	writeFile(t, dir, "second.go", "package second\n")
	gitAdd(t, dir, "second.go")
	gitCommit(t, dir, "add second")

	// tag the current HEAD so we have a named ref
	cmd := exec.Command("git", "tag", "v0.1.0", "HEAD~1")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git tag failed: %s (%v)", out, err)
	}

	c, err := NewClient(dir, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}

	files, err := c.ChangedGoFiles()
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, f := range files {
		if f.Path == "second.go" {
			found = true
		}
	}
	if !found {
		t.Error("second.go should appear when diffing against v0.1.0")
	}
}

func TestChangedGoFiles_MixedBag(t *testing.T) {
	dir := initTestRepo(t)

	// modify committed file
	writeFile(t, dir, "committed.go", "package main\n\nvar x = 1\n")

	// add untracked .go
	writeFile(t, dir, "untracked.go", "package extra\n")

	// add non-Go files (must not appear)
	writeFile(t, dir, "readme.md", "# hi")
	writeFile(t, dir, "Makefile", "all:")
	writeFile(t, dir, "go.sum", "fake")

	// add a staged-only .go
	writeFile(t, dir, "staged_only.go", "package staged\n")
	gitAdd(t, dir, "staged_only.go")

	c, err := NewClient(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	files, err := c.ChangedGoFiles()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"committed.go":   false,
		"untracked.go":   false,
		"staged_only.go": false,
	}

	for _, f := range files {
		if filepath.Ext(f.Path) != ".go" {
			t.Errorf("non-Go file leaked: %s", f.Path)
		}
		if _, ok := want[f.Path]; ok {
			want[f.Path] = true
		}
	}

	for name, seen := range want {
		if !seen {
			t.Errorf("%s not detected", name)
		}
	}
}

func TestChangedGoFiles_Subdirectories(t *testing.T) {
	dir := initTestRepo(t)

	writeFile(t, dir, filepath.Join("pkg", "deep", "nested", "leaf.go"), "package nested\n")

	c, err := NewClient(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	files, err := c.ChangedGoFiles()
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, f := range files {
		if f.Path == filepath.Join("pkg", "deep", "nested", "leaf.go") {
			found = true
		}
	}
	if !found {
		t.Error("nested .go file not detected")
	}
}

func TestNewClient_NotARepo(t *testing.T) {
	dir := t.TempDir() // not initialised as a git repo

	_, err := NewClient(dir, "HEAD")
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
}
