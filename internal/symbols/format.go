package symbols

import (
	"fmt"
	"sort"
	"strings"
)

// FormatSymbolList renders all symbols as an LLM-friendly text block,
// grouped by kind: Types -> Functions -> Methods -> Constants -> Variables.
// This is used by the generate command to describe an entire package.
func FormatSymbolList(syms []Symbol) string {
	if len(syms) == 0 {
		return "No exported symbols"
	}

	groups := map[SymbolKind][]Symbol{}
	for i := range syms {
		groups[syms[i].Kind] = append(groups[syms[i].Kind], syms[i])
	}

	// sort symbols within each group alphabetically
	for k := range groups {
		sort.Slice(groups[k], func(i, j int) bool {
			return groups[k][i].Name < groups[k][j].Name
		})
	}

	var sb strings.Builder

	// order: structs, interfaces, type aliases/defs, funcs, methods, consts, vars
	sections := []struct {
		title string
		kinds []SymbolKind
	}{
		{"Types", []SymbolKind{KindStruct, KindInterface, KindTypeAlias, KindTypeDef}},
		{"Functions", []SymbolKind{KindFunc}},
		{"Methods", []SymbolKind{KindMethod}},
		{"Constants", []SymbolKind{KindConst}},
		{"Variables", []SymbolKind{KindVar}},
	}

	for _, sec := range sections {
		var items []Symbol
		for _, k := range sec.kinds {
			items = append(items, groups[k]...)
		}
		if len(items) == 0 {
			continue
		}

		fmt.Fprintf(&sb, "## %s\n\n", sec.title)
		for i := range items {
			sb.WriteString(items[i].Signature)
			sb.WriteString("\n")
			if items[i].Doc != "" {
				// indent doc comment under the signature
				for _, line := range strings.Split(strings.TrimSpace(items[i].Doc), "\n") {
					sb.WriteString("  // ")
					sb.WriteString(line)
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
