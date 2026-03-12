// Package symbols extracts exported Go symbols from source files
package symbols

// SymbolKind classifies what kind of exported declaration a symbol is
type SymbolKind int

// SymbolKind values classify exported declarations
const (
	KindFunc SymbolKind = iota
	KindMethod
	KindStruct
	KindInterface
	KindTypeAlias
	KindTypeDef
	KindConst
	KindVar
)

// String returns a readable label for the kind
func (k SymbolKind) String() string {
	switch k {
	case KindFunc:
		return "func"
	case KindMethod:
		return "method"
	case KindStruct:
		return "struct"
	case KindInterface:
		return "interface"
	case KindTypeAlias:
		return "type alias"
	case KindTypeDef:
		return "type"
	case KindConst:
		return "const"
	case KindVar:
		return "var"
	default:
		return "unknown"
	}
}

// Param represents a single parameter or return value
type Param struct {
	// Name is the parameter name
	Name string
	// Type is the rendered Go type expression
	Type string
}

// Field represents a struct field or interface method
type Field struct {
	// Name is the field/method name
	Name string
	// Type is the rendered Go type expression
	Type string
	// Doc is the field-level doc comment
	Doc string
}

// Symbol represents a single exported declaration extraced from a Go file
type Symbol struct {
	// Name is the symbol's identifier
	Name string
	// Kind classifies the symbol
	Kind SymbolKind
	// Signature is the rendered Go source of the declaration
	Signature string
	// Doc is the doc comment text
	Doc string
	// Recv is the receiver type for methods
	Recv string
	// Params hold structured parameters for functions/methods
	Params []Param
	// Returns holds structured return values
	Returns []Param
	// Fields hold struct fields
	Fields []Field
	// Methods holds interface method signatures
	Methods []Field
	// Group identifies the const/var declaration group
	Group string
}

// ChangeKind classifies how a symbol changed between two versions
type ChangeKind int

// ChangeKind shows the change type
const (
	ChangeAdded ChangeKind = iota
	ChangeRemoved
	ChangeModified
	ChangeRenamed
)

// ChangeToString returns a readable label for the change kind
func (c ChangeKind) ChangeToString() string {
	switch c {
	case ChangeAdded:
		return "added"
	case ChangeRemoved:
		return "removed"
	case ChangeModified:
		return "modified"
	case ChangeRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// FieldChange describes a single sub-change within a modified symbol
type FieldChange struct {
	Description string
}

// SymbolDiff describes how a single exported symbol changed
type SymbolDiff struct {
	// Name is the symbol identifier
	Name string
	// set only for ChangeRenamed
	OldName string
	Kind    ChangeKind
	// Symbol holds the symbol data
	Symbol       Symbol
	OldSignature string
	// Changes lists specific sub-changes for modified symbols
	Changes []FieldChange
}
