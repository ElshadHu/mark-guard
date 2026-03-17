package symbols

import (
	"strings"
	"testing"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name         string
		oldSrc       string
		newSrc       string
		wantAdded    int
		wantRemoved  int
		wantModified int
		check        func(t *testing.T, diffs []SymbolDiff)
	}{
		{
			name:      "Added symbol detected",
			oldSrc:    "package test\n",
			newSrc:    "package test\nfunc NewFunc() {}",
			wantAdded: 1,
			check: func(t *testing.T, diffs []SymbolDiff) {
				if diffs[0].Name != "NewFunc" || diffs[0].Kind != ChangeAdded {
					t.Errorf("Expected NewFunc to be added")
				}
			},
		},
		{
			name:        "Removed symbol detected",
			oldSrc:      "package test\nfunc OldFunc() {}",
			newSrc:      "package test\n",
			wantRemoved: 1,
			check: func(t *testing.T, diffs []SymbolDiff) {
				if diffs[0].Name != "OldFunc" || diffs[0].Kind != ChangeRemoved {
					t.Errorf("Expected OldFunc to be removed")
				}
			},
		},
		{
			name:         "Modified: parameter added",
			oldSrc:       "package test\nfunc A(i int) {}",
			newSrc:       "package test\nfunc A(i int, s string) {}",
			wantModified: 1,
			check: func(t *testing.T, diffs []SymbolDiff) {
				if !strings.Contains(diffs[0].Changes[0].Description, "parameter s string added") {
					t.Errorf("Unexpected change description: %s", diffs[0].Changes[0].Description)
				}
			},
		},
		{
			name:         "Modified: struct field added",
			oldSrc:       "package test\ntype S struct{ A int }",
			newSrc:       "package test\ntype S struct{ A int; B string }",
			wantModified: 1,
			check: func(t *testing.T, diffs []SymbolDiff) {
				if !strings.Contains(diffs[0].Changes[0].Description, "field B string added") {
					t.Errorf("Unexpected change description: %s", diffs[0].Changes[0].Description)
				}
			},
		},
		{
			name:         "Modified: struct field type changed",
			oldSrc:       "package test\ntype S struct{ A int }",
			newSrc:       "package test\ntype S struct{ A string }",
			wantModified: 1,
			check: func(t *testing.T, diffs []SymbolDiff) {
				if !strings.Contains(diffs[0].Changes[0].Description, "field A type changed from int to string") {
					t.Errorf("Unexpected change description: %s", diffs[0].Changes[0].Description)
				}
			},
		},
		{
			name:         "Modified: interface method added",
			oldSrc:       "package test\ntype I interface{ Do() }",
			newSrc:       "package test\ntype I interface{ Do(); Run() }",
			wantModified: 1,
			check: func(t *testing.T, diffs []SymbolDiff) {
				if !strings.Contains(diffs[0].Changes[0].Description, "method Run func() added") {
					t.Errorf("Unexpected change description: %s", diffs[0].Changes[0].Description)
				}
			},
		},
		{
			name:   "Unchanged symbol not in output",
			oldSrc: "package test\nfunc A() {}",
			newSrc: "package test\nfunc A() {}",
		},
		{
			name:   "Same source produces empty diffs",
			oldSrc: "package test\ntype S struct{ A int }\nfunc (s S) Do() {}\nconst C = 1",
			newSrc: "package test\ntype S struct{ A int }\nfunc (s S) Do() {}\nconst C = 1",
		},
		{
			name:         "Kind change (struct to interface)",
			oldSrc:       "package test\ntype Thing struct{}",
			newSrc:       "package test\ntype Thing interface{}",
			wantModified: 1,
			check: func(t *testing.T, diffs []SymbolDiff) {
				if !strings.Contains(diffs[0].Changes[0].Description, "kind changed from struct to interface") {
					t.Errorf("Unexpected change description: %s", diffs[0].Changes[0].Description)
				}
			},
		},
		{
			name:         "Return type change",
			oldSrc:       "package test\nfunc F() int { return 0 }",
			newSrc:       "package test\nfunc F() string { return \"\" }",
			wantModified: 1,
			check: func(t *testing.T, diffs []SymbolDiff) {
				if !strings.Contains(diffs[0].Changes[0].Description, "return value at position 0 type changed from int to string") {
					t.Errorf("Unexpected change description: %s", diffs[0].Changes[0].Description)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldSyms, err := ExtractSymbols("old.go", tc.oldSrc)
			if err != nil {
				t.Fatalf("ExtractSymbols(old): %v", err)
			}
			newSyms, err := ExtractSymbols("new.go", tc.newSrc)
			if err != nil {
				t.Fatalf("ExtractSymbols(new): %v", err)
			}

			diffs := Diff(oldSyms, newSyms)

			var added, removed, modified int
			for _, d := range diffs {
				switch d.Kind {
				case ChangeAdded:
					added++
				case ChangeRemoved:
					removed++
				case ChangeModified:
					modified++
				}
			}

			if added != tc.wantAdded {
				t.Errorf("Added: got %d, want %d", added, tc.wantAdded)
			}
			if removed != tc.wantRemoved {
				t.Errorf("Removed: got %d, want %d", removed, tc.wantRemoved)
			}
			if modified != tc.wantModified {
				t.Errorf("Modified: got %d, want %d", modified, tc.wantModified)
			}

			if tc.check != nil && len(diffs) > 0 {
				tc.check(t, diffs)
			}
		})
	}
}

func TestDiff_NilCases(t *testing.T) {
	syms, _ := ExtractSymbols("new.go", "package test\nfunc A() {}\nfunc B() {}")

	t.Run("nil old (new file) = all added", func(t *testing.T) {
		diffs := Diff(nil, syms)
		if len(diffs) != 2 {
			t.Fatalf("Expected 2 additions, got %d", len(diffs))
		}
		for _, d := range diffs {
			if d.Kind != ChangeAdded {
				t.Errorf("Expected ChangeAdded, got %v", d.Kind)
			}
		}
	})

	t.Run("nil new (deleted file) = all removed", func(t *testing.T) {
		diffs := Diff(syms, nil)
		if len(diffs) != 2 {
			t.Fatalf("Expected 2 removals, got %d", len(diffs))
		}
		for _, d := range diffs {
			if d.Kind != ChangeRemoved {
				t.Errorf("Expected ChangeRemoved, got %v", d.Kind)
			}
		}
	})
}

func TestDiff_DeterministicOrdering(t *testing.T) {
	oldSrc := "package test\nfunc A() {}\nfunc C() {}\nfunc Z() {}\ntype S struct{ A int }"
	newSrc := "package test\nfunc B() {}\nfunc C(i int) {}\nfunc Y() {}\ntype S struct{ A string }"

	oldSyms, _ := ExtractSymbols("old.go", oldSrc)
	newSyms, _ := ExtractSymbols("new.go", newSrc)

	// Run multiple times and ensure the output is exactly identical
	firstDiff := Diff(oldSyms, newSyms)
	
	for i := 0; i < 20; i++ {
		diffs := Diff(oldSyms, newSyms)
		if len(diffs) != len(firstDiff) {
			t.Fatalf("Iteration %d: length changed from %d to %d", i, len(firstDiff), len(diffs))
		}
		for j := range diffs {
			if diffs[j].Name != firstDiff[j].Name || diffs[j].Kind != firstDiff[j].Kind {
				t.Fatalf("Iteration %d: diff at index %d changed from %v %s to %v %s", 
					i, j, firstDiff[j].Kind, firstDiff[j].Name, diffs[j].Kind, diffs[j].Name)
			}
		}
	}
}

func TestFormatDiffSummary(t *testing.T) {
	t.Run("nil/empty returns no changes", func(t *testing.T) {
		if got := FormatDiffSummary(nil); got != "No changes to exported symbols" {
			t.Errorf("Expected 'No changes...', got %q", got)
		}
		if got := FormatDiffSummary([]SymbolDiff{}); got != "No changes to exported symbols" {
			t.Errorf("Expected 'No changes...', got %q", got)
		}
	})

	t.Run("contains Added/Removed/Modified sections", func(t *testing.T) {
		diffs := []SymbolDiff{
			{
				Name: "NewFunc",
				Kind: ChangeAdded,
				Symbol: Symbol{Signature: "func NewFunc()"},
			},
			{
				Name: "OldFunc",
				Kind: ChangeRemoved,
				OldSignature: "func OldFunc()",
			},
			{
				Name: "ModFunc",
				Kind: ChangeModified,
				Symbol: Symbol{Kind: KindFunc, Name: "ModFunc"},
				Changes: []FieldChange{
					{Description: "parameter i int added"},
				},
			},
		}

		summary := FormatDiffSummary(diffs)
		
		if !strings.Contains(summary, "## Added\n- func NewFunc()") {
			t.Errorf("Summary missing Added section: %s", summary)
		}
		if !strings.Contains(summary, "## Removed\n- func OldFunc()") {
			t.Errorf("Summary missing Removed section: %s", summary)
		}
		if !strings.Contains(summary, "## Modified\n- func ModFunc: parameter i int added") {
			t.Errorf("Summary missing Modified section: %s", summary)
		}
	})
}
