package llm

import (
	"testing"
)

func TestParseEdits_ValidJSONWithOneEdit(t *testing.T) {
	raw := `{"edits":[{"file":"README.md","action":"replace","old_text":"# Title","new_text":"# New Title"}]}`
	result, err := ParseEdits(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(result.Edits))
	}
	if result.Edits[0].File != "README.md" {
		t.Errorf("expected file README.md, got %q", result.Edits[0].File)
	}
	if result.Edits[0].Action != "replace" {
		t.Errorf("expected action replace, got %q", result.Edits[0].Action)
	}
	if result.Edits[0].OldText != "# Title" {
		t.Errorf("expected old_text '# Title', got %q", result.Edits[0].OldText)
	}
	if result.Edits[0].NewText != "# New Title" {
		t.Errorf("expected new_text '# New Title', got %q", result.Edits[0].NewText)
	}
}

func TestParseEdits_EmptyEditsArray(t *testing.T) {
	raw := `{"edits":[]}`
	result, err := ParseEdits(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Edits) != 0 {
		t.Fatalf("expected 0 edits, got %d", len(result.Edits))
	}
}

func TestParseEdits_JsonFenceStripped(t *testing.T) {
	raw := "```json\n{\"edits\":[{\"file\":\"README.md\",\"action\":\"replace\",\"old_text\":\"foo\",\"new_text\":\"bar\"}]}\n```"
	result, err := ParseEdits(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(result.Edits))
	}
	if result.Edits[0].File != "README.md" {
		t.Errorf("expected file README.md, got %q", result.Edits[0].File)
	}
	if result.Edits[0].NewText != "bar" {
		t.Errorf("expected new_text 'bar', got %q", result.Edits[0].NewText)
	}
}

func TestParseEdits_MalformedJSON(t *testing.T) {
	raw := `{invalid json}`
	_, err := ParseEdits(raw)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestParseEdits_EmptyString(t *testing.T) {
	raw := ""
	_, err := ParseEdits(raw)
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

func TestParseEdits_WhitespaceOnly(t *testing.T) {
	raw := "   \n\t  "
	_, err := ParseEdits(raw)
	if err == nil {
		t.Fatal("expected error for whitespace-only string, got nil")
	}
}

func TestParseEdits_PureProse(t *testing.T) {
	raw := "This is just some prose text without any JSON."
	_, err := ParseEdits(raw)
	if err == nil {
		t.Fatal("expected error for pure prose, got nil")
	}
}

func TestParseEdits_UnknownExtraFieldsIgnored(t *testing.T) {
	raw := `{"edits":[{"file":"README.md","action":"replace","old_text":"foo","new_text":"bar","unknown_field":"ignored","another_unknown":123}]}`
	result, err := ParseEdits(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(result.Edits))
	}
	if result.Edits[0].File != "README.md" {
		t.Errorf("expected file README.md, got %q", result.Edits[0].File)
	}
}

func TestParseEdits_UnicodeInText(t *testing.T) {
	raw := `{"edits":[{"file":"README.md","action":"replace","old_text":"Привет","new_text":"世界"}]}`
	result, err := ParseEdits(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(result.Edits))
	}
	if result.Edits[0].OldText != "Привет" {
		t.Errorf("expected old_text 'Привет', got %q", result.Edits[0].OldText)
	}
	if result.Edits[0].NewText != "世界" {
		t.Errorf("expected new_text '世界', got %q", result.Edits[0].NewText)
	}
}

func TestApplyEdits_ReplacesOnlyFirstOccurrence(t *testing.T) {
	docs := map[string]string{
		"README.md": "foo bar foo baz",
	}
	edits := []Edit{
		{File: "README.md", Action: "replace", OldText: "foo", NewText: "qux"},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if result["README.md"] != "qux bar foo baz" {
		t.Errorf("expected 'qux bar foo baz', got %q", result["README.md"])
	}
}

func TestApplyEdits_UnknownFile(t *testing.T) {
	docs := map[string]string{
		"README.md": "some content",
	}
	edits := []Edit{
		{File: "unknown.md", Action: "replace", OldText: "foo", NewText: "bar"},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0] == nil {
		t.Fatal("expected error, got nil")
	}
	_ = result
}

func TestApplyEdits_UnknownAction(t *testing.T) {
	docs := map[string]string{
		"README.md": "some content",
	}
	edits := []Edit{
		{File: "README.md", Action: "unknown", OldText: "foo", NewText: "bar"},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	_ = result
}

func TestApplyEdits_OldTextNotFound(t *testing.T) {
	docs := map[string]string{
		"README.md": "some content",
	}
	edits := []Edit{
		{File: "README.md", Action: "replace", OldText: "not present", NewText: "bar"},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	_ = result
}

func TestApplyEdits_ErrorsCollectedOtherEditsStillApply(t *testing.T) {
	docs := map[string]string{
		"README.md":  "foo bar",
		"CHANGES.md": "hello world",
	}
	edits := []Edit{
		{File: "README.md", Action: "replace", OldText: "not found", NewText: "qux"},
		{File: "CHANGES.md", Action: "replace", OldText: "hello", NewText: "goodbye"},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if result["CHANGES.md"] != "goodbye world" {
		t.Errorf("expected 'goodbye world', got %q", result["CHANGES.md"])
	}
}

func TestApplyEdits_MultipleEditsSameFile(t *testing.T) {
	docs := map[string]string{
		"README.md": "foo bar baz",
	}
	edits := []Edit{
		{File: "README.md", Action: "replace", OldText: "foo", NewText: "qux"},
		{File: "README.md", Action: "replace", OldText: "bar", NewText: "quux"},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if result["README.md"] != "qux quux baz" {
		t.Errorf("expected 'qux quux baz', got %q", result["README.md"])
	}
}

func TestApplyEdits_EmptyEdits(t *testing.T) {
	docs := map[string]string{
		"README.md": "some content",
	}
	edits := []Edit{}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if result["README.md"] != "some content" {
		t.Errorf("expected unchanged content, got %q", result["README.md"])
	}
}

func TestApplyEdits_OriginalDocsNotMutated(t *testing.T) {
	docs := map[string]string{
		"README.md": "original content",
	}
	originalContent := docs["README.md"]
	edits := []Edit{
		{File: "README.md", Action: "replace", OldText: "original", NewText: "modified"},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if docs["README.md"] != originalContent {
		t.Errorf("original docs were mutated: expected %q, got %q", originalContent, docs["README.md"])
	}
	if result["README.md"] != "modified content" {
		t.Errorf("expected 'modified content', got %q", result["README.md"])
	}
}

func TestApplyEdits_DeleteAction(t *testing.T) {
	docs := map[string]string{
		"README.md": "foo bar baz",
	}
	edits := []Edit{
		{File: "README.md", Action: "delete", OldText: "bar "},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if result["README.md"] != "foo baz" {
		t.Errorf("expected 'foo baz', got %q", result["README.md"])
	}
}

func TestApplyEdits_InsertAfterAction(t *testing.T) {
	docs := map[string]string{
		"README.md": "foo bar",
	}
	edits := []Edit{
		{File: "README.md", Action: "insert_after", OldText: "foo", NewText: "baz"},
	}
	result, errs := ApplyEdits(docs, edits)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if result["README.md"] != "foo\nbaz bar" {
		t.Errorf("expected 'foo\nbaz bar', got %q", result["README.md"])
	}
}
