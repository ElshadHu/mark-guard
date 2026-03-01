package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Edit describes one surgical text replacement within a single doc file
// The LLM only emits the exact bytes that need to change, never the full file
type Edit struct {
	File    string `json:"file"`              // relative doc path
	Section string `json:"section,omitempty"` // nearest heading, context only, not used for matching
	Action  string `json:"action"`            // "replace" | "insert_after" | "delete"
	OldText string `json:"old_text"`          // text that must exist in the file
	NewText string `json:"new_text"`          // text to write in its place
}

// EditResponse is the top-level JSON shape the LLM must return
type EditResponse struct {
	Edits []Edit `json:"edits"`
}

// ParseEdits unmarshals the raw LLM content string into an EditResponse.
// It tolerates optional ```json … ``` fences that some models add despite instructions.
func ParseEdits(raw string) (*EditResponse, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		start := strings.Index(raw, "\n")
		end := strings.LastIndex(raw, "```")
		if start != -1 && end > start {
			raw = strings.TrimSpace(raw[start+1 : end])
		}
	}

	var er EditResponse
	if err := json.Unmarshal([]byte(raw), &er); err != nil {
		return nil, fmt.Errorf("LLM response is not valid JSON: %w\nraw: %.200s", err, raw)
	}
	return &er, nil
}

// ApplyEdits applies a list of edits to a snapshot of doc contents
func ApplyEdits(docs map[string]string, edits []Edit) (map[string]string, []error) {
	// Shallow-copy the snapshot so the caller's map is not mutated
	result := make(map[string]string, len(docs))
	for k, v := range docs {
		result[k] = v
	}

	var errs []error
	for _, e := range edits {
		content, ok := result[e.File]
		if !ok {
			errs = append(errs, fmt.Errorf("edit references unknown file %q", e.File))
			continue
		}

		switch e.Action {
		case "replace":
			if !strings.Contains(content, e.OldText) {
				errs = append(errs, fmt.Errorf("%s: old_text not found verbatim: %q", e.File, e.OldText))
				continue
			}
			result[e.File] = strings.Replace(content, e.OldText, e.NewText, 1)

		case "delete":
			if !strings.Contains(content, e.OldText) {
				errs = append(errs, fmt.Errorf("%s: old_text not found verbatim: %q", e.File, e.OldText))
				continue
			}
			result[e.File] = strings.Replace(content, e.OldText, "", 1)

		case "insert_after":
			if !strings.Contains(content, e.OldText) {
				errs = append(errs, fmt.Errorf("%s: anchor not found verbatim: %q", e.File, e.OldText))
				continue
			}
			result[e.File] = strings.Replace(content, e.OldText, e.OldText+"\n"+e.NewText, 1)

		default:
			errs = append(errs, fmt.Errorf("%s: unknown action %q (want replace|insert_after|delete)", e.File, e.Action))
		}
	}
	return result, errs
}
