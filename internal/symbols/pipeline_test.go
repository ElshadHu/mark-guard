package symbols

import (
	"strings"
	"testing"
)

// TestPipeline_MixedChanges covers added + removed + modified in one pass.
func TestPipeline_MixedChanges(t *testing.T) {
	oldSrc := `package example
func Keep() {}
func Remove() {}
func Modify(a int) {}
`
	newSrc := `package example
func Keep() {}
func Modify(a int, b string) {}
func Added() string { return "" }
`
	old, err := ExtractSymbols("example.go", oldSrc)
	if err != nil {
		t.Fatalf("ExtractSymbols old: %v", err)
	}
	cur, err := ExtractSymbols("example.go", newSrc)
	if err != nil {
		t.Fatalf("ExtractSymbols new: %v", err)
	}
	diffs := Diff(old, cur)
	counts := map[ChangeKind]int{}
	for _, d := range diffs {
		counts[d.Kind]++
	}
	if counts[ChangeAdded] != 1 || counts[ChangeRemoved] != 1 || counts[ChangeModified] != 1 {
		t.Fatalf("expected 1 added/1 removed/1 modified, got %v", counts)
	}
	summary := FormatDiffSummary(diffs)
	for _, section := range []string{"## Added", "## Removed", "## Modified"} {
		if !strings.Contains(summary, section) {
			t.Errorf("summary missing %q", section)
		}
	}
}

// TestPipeline_NewFile tests old=nil (brand new file, all symbols added)
func TestPipeline_NewFile(t *testing.T) {
	newSrc := `package example

func Alpha() {}
var MaxSize int
`
	cur, err := ExtractSymbols("example.go", newSrc)
	if err != nil {
		t.Fatalf("ExtractSymbols: %v", err)
	}
	diffs := Diff(nil, cur)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}
	for _, d := range diffs {
		if d.Kind != ChangeAdded {
			t.Errorf("expected ChangeAdded for %s, got %s", d.Name, d.Kind.ChangeToString())
		}
	}
}

// TestPipeline_DeletedFile tests new=nil (file deleted, all symbols removed).
func TestPipeline_DeletedFile(t *testing.T) {
	oldSrc := `package example
func Alpha() {}
var MaxSize int
`
	old, err := ExtractSymbols("example.go", oldSrc)
	if err != nil {
		t.Fatalf("ExtractSymbols: %v", err)
	}
	diffs := Diff(old, nil)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}

	for _, d := range diffs {
		if d.Kind != ChangeRemoved {
			t.Errorf("expected ChangeRemoved for %s, got %s", d.Name, d.Kind.ChangeToString())
		}
	}
}

// TestPipeline_NoDiff tests that identical source produces zero diffs.
func TestPipeline_NoDiff(t *testing.T) {
	src := `package example
func Hello() {}
`
	old, _ := ExtractSymbols("example.go", src)
	cur, _ := ExtractSymbols("example.go", src)
	diffs := Diff(old, cur)
	if len(diffs) != 0 {
		t.Fatalf("expected 0 diffs, got %d", len(diffs))
	}
	if s := FormatDiffSummary(diffs); s != "No changes to exported symbols" {
		t.Errorf("unexpected summary: %s", s)
	}
}

// TestPipeline_ParseError proves ExtractSymbols surfaces errors (the ones format.go swallows).
func TestPipeline_ParseError(t *testing.T) {
	_, err := ExtractSymbols("bad.go", "not valid go")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}
