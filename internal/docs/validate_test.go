package docs

import (
	"strings"
	"testing"
)

func TestValidateUpdates(t *testing.T) {
	tests := []struct {
		name       string
		originals  map[string]string
		updates    map[string]string
		wantCount  int
		wantReject bool
	}{
		{
			name:       "empty content is rejected",
			originals:  map[string]string{"README.md": "# Hello\nSome content.\n"},
			updates:    map[string]string{"README.md": ""},
			wantCount:  1,
			wantReject: true,
		},
		{
			name:       "60% loss is rejected",
			originals:  map[string]string{"docs/api.md": strings.Repeat("x", 1000)},
			updates:    map[string]string{"docs/api.md": strings.Repeat("x", 400)},
			wantCount:  1,
			wantReject: true,
		},
		{
			name:       "30% loss is acceptable",
			originals:  map[string]string{"README.md": strings.Repeat("x", 1000)},
			updates:    map[string]string{"README.md": strings.Repeat("x", 700)},
			wantCount:  0,
			wantReject: false,
		},
		{
			name:       "content growth passes",
			originals:  map[string]string{"README.md": "short"},
			updates:    map[string]string{"README.md": strings.Repeat("x", 5000)},
			wantCount:  0,
			wantReject: false,
		},
		{
			name:       "exact 50% loss is acceptable",
			originals:  map[string]string{"README.md": strings.Repeat("x", 100)},
			updates:    map[string]string{"README.md": strings.Repeat("x", 50)},
			wantCount:  0,
			wantReject: false,
		},
		{
			name:       "unknown file in updates is skipped",
			originals:  map[string]string{},
			updates:    map[string]string{"new.md": "content"},
			wantCount:  0,
			wantReject: false,
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			results := ValidateUpdates(tests[i].originals, tests[i].updates)
			if len(results) != tests[i].wantCount {
				t.Fatalf("got %d results, want %d: %v", len(results), tests[i].wantCount, results)
			}
			if tests[i].wantCount > 0 && results[0].Rejected != tests[i].wantReject {
				t.Errorf("Rejected = %v, want %v", results[0].Rejected, tests[i].wantReject)
			}
		})
	}
}

func TestValidateEdit(t *testing.T) {
	tests := []struct {
		name       string
		input      EditInput
		wantNil    bool
		wantReject bool
	}{
		{
			name: "replace with empty new_text warns",
			input: EditInput{
				File: "README.md", Action: "replace",
				OldText: "some old text", NewText: "", FileLen: 500,
			},
			wantNil:    false,
			wantReject: false,
		},
		{
			name: "normal replace passes",
			input: EditInput{
				File: "README.md", Action: "replace",
				OldText: "old", NewText: "new", FileLen: 500,
			},
			wantNil: true,
		},
		{
			name: "large delete warns",
			input: EditInput{
				File: "README.md", Action: "delete",
				OldText: strings.Repeat("x", 400), NewText: "", FileLen: 1000,
			},
			wantNil:    false,
			wantReject: false,
		},
		{
			name: "small delete passes",
			input: EditInput{
				File: "README.md", Action: "delete",
				OldText: "tiny", NewText: "", FileLen: 1000,
			},
			wantNil: true,
		},
		{
			name: "insert_after passes",
			input: EditInput{
				File: "README.md", Action: "insert_after",
				OldText: "anchor", NewText: "new stuff", FileLen: 500,
			},
			wantNil: true,
		},
		{
			name: "delete with zero fileLen passes",
			input: EditInput{
				File: "README.md", Action: "delete",
				OldText: "text", NewText: "", FileLen: 0,
			},
			wantNil: true,
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			result := ValidateEdit(&tests[i].input)
			if tests[i].wantNil && result != nil {
				t.Errorf("expected nil, got %+v", result)
			}
			if !tests[i].wantNil {
				if result == nil {
					t.Fatal("expected a result, got nil")
				}
				if result.Rejected != tests[i].wantReject {
					t.Errorf("Rejected = %v, want %v", result.Rejected, tests[i].wantReject)
				}
			}
		})
	}
}
