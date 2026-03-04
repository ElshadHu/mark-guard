// Package docs handles reading, writing, and validating markdown documentation.
package docs

import "fmt"

// ContentLossThreshold is the max fraction of original bytes that can be lost.
const ContentLossThreshold = 0.50

// SingleDeleteThreshold warns when one delete removes this fraction of a file.
const SingleDeleteThreshold = 0.30

// ValidationResult holds the outcome of a single validation check.
type ValidationResult struct {
	File     string
	Message  string
	Rejected bool // true = hard reject, false = warning only
}

// EditInput groups the fields needed to validate a single edit.
type EditInput struct {
	File    string
	Action  string
	OldText string
	NewText string
	FileLen int // byte length of the file before this edit
}

// ValidateEdit checks a single edit before it is applied.
// Returns nil if the edit looks safe.
func ValidateEdit(in *EditInput) *ValidationResult {
	// replace with empty new_text is likely accidental deletion
	if in.Action == "replace" && in.NewText == "" {
		return &ValidationResult{
			File:     in.File,
			Message:  fmt.Sprintf("replace has empty new_text for %q", truncate(in.OldText, 80)),
			Rejected: false,
		}
	}

	// single delete that wipes a large chunk of the file
	if in.Action == "delete" && in.FileLen > 0 {
		fraction := float64(len(in.OldText)) / float64(in.FileLen)
		if fraction > SingleDeleteThreshold {
			return &ValidationResult{
				File: in.File,
				Message: fmt.Sprintf(
					"single delete removes %.0f%% of file (%d of %d bytes)",
					fraction*100, len(in.OldText), in.FileLen,
				),
				Rejected: false,
			}
		}
	}

	return nil
}

// ValidateUpdates compares proposed doc contents against originals.
// Returns validation issues (warnings and rejections). Empty means all passed.
func ValidateUpdates(originals, updates map[string]string) []ValidationResult {
	var results []ValidationResult

	for path, newContent := range updates {
		oldContent, ok := originals[path]
		if !ok {
			continue
		}

		if newContent == "" {
			results = append(results, ValidationResult{
				File:     path,
				Message:  "updated content is empty",
				Rejected: true,
			})
			continue
		}

		oldLen := len(oldContent)
		newLen := len(newContent)
		if oldLen > 0 {
			loss := float64(oldLen-newLen) / float64(oldLen)
			if loss > ContentLossThreshold {
				results = append(results, ValidationResult{
					File: path,
					Message: fmt.Sprintf(
						"content shrank by %.0f%% (%d → %d bytes), exceeds %.0f%% threshold",
						loss*100, oldLen, newLen, ContentLossThreshold*100,
					),
					Rejected: true,
				})
			}
		}
	}

	return results
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
