// Package model holds shared domain types used across multiple package,
// The main purpose of this module is to avoid circular dependency
package model

// DocMapping links doc file to code paths
type DocMapping struct {
	Docs []string
	Code []string
}

// DocFile is a md found during scanning
type DocFile struct {
	Path    string // relative to repo root
	Content string
}

// DocInput is a doc file as sent to the LLM
type DocInput struct {
	Path    string
	Content string
}
