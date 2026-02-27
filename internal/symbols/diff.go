// Package symbols extracts exported Go symbols from source files
package symbols

import (
	"fmt"
	"sort"
	"strings"
)

// Diff compares old and cur symbol slices and returns a list of changes
func Diff(old, cur []Symbol) []SymbolDiff {
	oldMap := indexByName(old)
	curMap := indexByName(cur)
	var diffs []SymbolDiff

	// Pass 1: added (in cur not in old)
	for name, sym := range curMap {
		if _, exists := oldMap[name]; !exists {
			diffs = append(diffs, SymbolDiff{
				Name:   name,
				Kind:   ChangeAdded,
				Symbol: sym,
			})
		}
	}

	// Pass 2: removed (in old not cur)
	for name, sym := range oldMap {
		if _, exists := curMap[name]; !exists {
			diffs = append(diffs, SymbolDiff{
				Name:         name,
				Kind:         ChangeRemoved,
				Symbol:       sym,
				OldSignature: sym.Signature,
			})
		}
	}

	// Pass 3: modified (in both, but changed)
	for name, curSym := range curMap {
		oldSym, exists := oldMap[name]
		if !exists {
			continue
		}
		changes := compareSymbols(oldSym, curSym)
		if len(changes) > 0 {
			diffs = append(diffs, SymbolDiff{
				Name:         name,
				Kind:         ChangeModified,
				Symbol:       curSym,
				OldSignature: oldSym.Signature,
				Changes:      changes,
			})
		}
	}

	// Sort for deterministic output:
	// added first, then removed, then modified
	// within each group, sort alphabetically by name
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Kind != diffs[j].Kind {
			return diffs[i].Kind < diffs[j].Kind
		}
		return diffs[i].Name < diffs[j].Name
	})
	return diffs
}

// indexByName creates a map from symbol name to symbol
func indexByName(symbols []Symbol) map[string]Symbol {
	m := make(map[string]Symbol, len(symbols))
	for i := range symbols {
		m[symbols[i].Name] = symbols[i]
	}
	return m
}

// compareSymbols returns a list of changes between old and cur versions of the same symbol
// returns nil if the symbols are identical
func compareSymbols(old, cur Symbol) []FieldChange {
	var changes []FieldChange
	// Kind change struct -> interface example
	if old.Kind != cur.Kind {
		changes = append(changes, FieldChange{
			Description: fmt.Sprintf("kind changed from %s to %s", old.Kind, cur.Kind),
		})
	}

	// Compare params
	changes = append(changes, compareParams(old.Params, cur.Params)...)
	// Compare returns
	changes = append(changes, compareReturns(old.Returns, cur.Returns)...)
	// Compare struct fields
	changes = append(changes, compareFields(old.Fields, cur.Fields)...)
	// Compare interface methods
	changes = append(changes, compareMethods(old.Methods, cur.Methods)...)

	if old.Recv != cur.Recv && (old.Recv != "" || cur.Recv != "") {
		changes = append(changes, FieldChange{
			Description: fmt.Sprintf("receiver changed from %s to %s", old.Recv, cur.Recv),
		})
	}

	// Fallback: if signature changed but no specific sub-changes were detected,
	// report the signature change directly.
	if old.Signature != cur.Signature && len(changes) == 0 {
		changes = append(changes, FieldChange{
			Description: fmt.Sprintf("signature changed from %q to %q", old.Signature, cur.Signature),
		})
	}

	return changes
}

// compareParams compares parameter lists and returns specific changes
func compareParams(old, cur []Param) []FieldChange {
	if len(old) == 0 && len(cur) == 0 {
		return nil
	}

	var changes []FieldChange
	oldMap := indexParamsByName(old)
	curMap := indexParamsByName(cur)
	for i := range cur {
		key := paramKey(cur[i])
		if _, exists := oldMap[key]; !exists {
			desc := fmt.Sprintf("parameter %s added", formatParam(cur[i]))
			changes = append(changes, FieldChange{Description: desc})
		}
	}

	// Detect removed params
	for i := range old {
		key := paramKey(old[i])
		if _, exists := curMap[key]; !exists {
			desc := fmt.Sprintf("parameter %s removed", formatParam(old[i]))
			changes = append(changes, FieldChange{Description: desc})
		}
	}

	// Detect type changes (same name, different type)
	for i := range old {
		if old[i].Name == "" {
			continue // unnamed params handled by positional comparison
		}
		for j := range cur {
			if old[i].Name == cur[j].Name && old[i].Type != cur[j].Type {
				desc := fmt.Sprintf("parameter %s type changed from %s to %s", old[i].Name, old[i].Type, cur[j].Type)
				changes = append(changes, FieldChange{Description: desc})
			}
		}
	}
	// Detect reordering: if names match but positions differ
	if len(old) == len(cur) && len(changes) == 0 {
		for i := range old {
			if old[i].Name != cur[i].Name || old[i].Type != cur[i].Type {
				changes = append(changes, FieldChange{
					Description: fmt.Sprintf("parameter at position %d changed from %s to %s",
						i, formatParam(old[i]), formatParam(cur[i])),
				})
			}
		}
	}
	return changes
}

// compareReturns compares return value lists
func compareReturns(old, cur []Param) []FieldChange {
	if len(old) == 0 && len(cur) == 0 {
		return nil
	}

	var changes []FieldChange
	if len(old) != len(cur) {
		changes = append(changes, FieldChange{
			Description: fmt.Sprintf("return values changed from (%s) to (%s)", formatParamList(old), formatParamList(cur)),
		})
		return changes
	}
	// Same count — compare positionally
	for i := range old {
		if old[i].Type != cur[i].Type {
			changes = append(changes, FieldChange{
				Description: fmt.Sprintf("return value at position %d type changed from %s to %s",
					i, old[i].Type, cur[i].Type),
			})
		}
		if old[i].Name != cur[i].Name && (old[i].Name != "" || cur[i].Name != "") {
			changes = append(changes, FieldChange{
				Description: fmt.Sprintf("return value at position %d name changed from %s to %s",
					i, old[i].Name, cur[i].Name),
			})
		}
	}
	return changes
}

// compareFields compares struct field lists
func compareFields(old, cur []Field) []FieldChange {
	if len(old) == 0 && len(cur) == 0 {
		return nil
	}

	var changes []FieldChange
	oldMap := make(map[string]Field)
	for i := range old {
		oldMap[old[i].Name] = old[i]
	}
	curMap := make(map[string]Field)
	for i := range cur {
		curMap[cur[i].Name] = cur[i]
	}
	// Added fields
	for i := range cur {
		if _, exists := oldMap[cur[i].Name]; !exists {
			changes = append(changes, FieldChange{
				Description: fmt.Sprintf("field %s %s added", cur[i].Name, cur[i].Type),
			})
		}
	}

	// Removed fields
	for i := range old {
		if _, exists := curMap[old[i].Name]; !exists {
			changes = append(changes, FieldChange{
				Description: fmt.Sprintf("field %s %s removed", old[i].Name, old[i].Type),
			})
		}
	}

	// Type changes — iterate old slice (not map) for deterministic order
	for i := range old {
		nf, exists := curMap[old[i].Name]
		if !exists {
			continue
		}
		if old[i].Type != nf.Type {
			changes = append(changes, FieldChange{
				Description: fmt.Sprintf("field %s type changed from %s to %s", old[i].Name, old[i].Type, nf.Type),
			})
		}
	}

	return changes
}

// compareMethods compares interface method lists
func compareMethods(old, cur []Field) []FieldChange {
	if len(old) == 0 && len(cur) == 0 {
		return nil
	}

	var changes []FieldChange

	oldMap := make(map[string]Field)
	for i := range old {
		oldMap[old[i].Name] = old[i]
	}
	curMap := make(map[string]Field)
	for i := range cur {
		curMap[cur[i].Name] = cur[i]
	}

	// Added methods
	for i := range cur {
		if _, exists := oldMap[cur[i].Name]; !exists {
			changes = append(changes, FieldChange{
				Description: fmt.Sprintf("method %s %s added", cur[i].Name, cur[i].Type),
			})
		}
	}

	// Removed methods
	for i := range old {
		if _, exists := curMap[old[i].Name]; !exists {
			changes = append(changes, FieldChange{
				Description: fmt.Sprintf("method %s %s removed", old[i].Name, old[i].Type),
			})
		}
	}

	// Signature changes — iterate old slice (not map) for deterministic order
	for i := range old {
		nm, exists := curMap[old[i].Name]
		if !exists {
			continue
		}
		if old[i].Type != nm.Type {
			changes = append(changes, FieldChange{
				Description: fmt.Sprintf("method %s signature changed from %s to %s", old[i].Name, old[i].Type, nm.Type),
			})
		}
	}

	return changes
}

// paramKey returns a key for the parameter lookup
func paramKey(p Param) string {
	if p.Name != "" {
		return p.Name
	}
	return ":" + p.Type
}

// indexParamsByName creates a lookup map for params
func indexParamsByName(params []Param) map[string]Param {
	m := make(map[string]Param, len(params))
	for i := range params {
		m[paramKey(params[i])] = params[i]
	}
	return m
}

// formatParam renders a parameter as "name type" or just "type" if unnamed.
func formatParam(p Param) string {
	if p.Name != "" {
		return p.Name + " " + p.Type
	}
	return p.Type
}

// formatParamList renders a parameter list as "name type, name type, ...".
func formatParamList(params []Param) string {
	parts := make([]string, len(params))
	for i := range params {
		parts[i] = formatParam(params[i])
	}
	return strings.Join(parts, ", ")
}

// FormatDiffSummary renders a list of symbol diffs as a  readable summary
// This output feeds directly into the LLM prompt
func FormatDiffSummary(diffs []SymbolDiff) string {
	if len(diffs) == 0 {
		return "No changes to exported symbols"
	}

	var added, removed, modified []SymbolDiff

	for i := range diffs {
		switch diffs[i].Kind {
		case ChangeAdded:
			added = append(added, diffs[i])
		case ChangeRemoved:
			removed = append(removed, diffs[i])
		case ChangeModified:
			modified = append(modified, diffs[i])
		}
	}

	var buf strings.Builder

	if len(added) > 0 {
		buf.WriteString("## Added\n")
		for i := range added {
			buf.WriteString("- ")
			buf.WriteString(added[i].Symbol.Signature)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}
	if len(removed) > 0 {
		buf.WriteString("## Removed\n")
		for i := range removed {
			buf.WriteString("- ")
			buf.WriteString(removed[i].OldSignature)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	if len(modified) > 0 {
		buf.WriteString("## Modified\n")
		for i := range modified {
			for j := range modified[i].Changes {
				buf.WriteString("- ")
				buf.WriteString(modified[i].Symbol.Kind.String())
				buf.WriteString(" ")
				buf.WriteString(modified[i].Name)
				buf.WriteString(": ")
				buf.WriteString(modified[i].Changes[j].Description)
				buf.WriteString("\n")
			}
		}
		buf.WriteString("\n")
	}

	return strings.TrimRight(buf.String(), "\n")
}
