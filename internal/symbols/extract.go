// Package symbols extracts exported Go symbols from source files
package symbols

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
)

// ExtractSymbols parses Go source code and returns all exported symbols.
func ExtractSymbols(filename, src string) ([]Symbol, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src,
		parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	var symbols []Symbol
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if sym, ok := extractFunc(fset, d); ok {
				symbols = append(symbols, sym)
			}
		case *ast.GenDecl:
			symbols = append(symbols, extractGenDecl(fset, d)...)
		}
	}
	return symbols, nil
}

// exprToString renders an ast.Expr to its Go source representation
func exprToString(fset *token.FileSet, expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		return fmt.Sprintf("<%T>", expr)
	}
	return buf.String()
}

// fieldToString renders a field list as it appears in source
func fieldListToString(fset *token.FileSet, fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, fl); err != nil {
		return ""
	}
	return buf.String()
}

// extractParams builds a []Param from a FieldList
func extractParams(fset *token.FileSet, fl *ast.FieldList) []Param {
	if fl == nil {
		return nil
	}
	var params []Param
	for _, field := range fl.List {
		typeStr := exprToString(fset, field.Type)
		if len(field.Names) == 0 {
			// unnamed param/return
			params = append(params, Param{Type: typeStr})
		} else {
			for _, name := range field.Names {
				params = append(params, Param{Name: name.Name, Type: typeStr})
			}
		}
	}
	return params
}

// extractFunc for functions and methods
func extractFunc(fset *token.FileSet, fn *ast.FuncDecl) (Symbol, bool) {
	if !fn.Name.IsExported() {
		return Symbol{}, false
	}
	sym := Symbol{
		Name:    fn.Name.Name,
		Kind:    KindFunc,
		Doc:     docText(fn.Doc),
		Params:  extractParams(fset, fn.Type.Params),
		Returns: extractParams(fset, fn.Type.Results),
	}
	if fn.Recv != nil && fn.Recv.NumFields() > 0 {
		sym.Kind = KindMethod
		sym.Recv = receiverType(fset, fn.Recv)
		sym.Name = sym.Recv + "." + fn.Name.Name
	}
	sym.Signature = buildFunctionSignature(fset, fn)
	return sym, true
}

// docText extracts the text content from a comment group.
func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return cg.Text()
}

// pickDoc returns the first non-nil comment group from the candidates.
func pickDoc(candidates ...*ast.CommentGroup) *ast.CommentGroup {
	for _, c := range candidates {
		if c != nil {
			return c
		}
	}
	return nil
}

// receiverType extracts the receiver type name
func receiverType(fset *token.FileSet, recv *ast.FieldList) string {
	if recv == nil || recv.NumFields() == 0 {
		return ""
	}
	typ := recv.List[0].Type
	// Unwrap the pointer
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if ident, ok := typ.(*ast.Ident); ok {
		return ident.Name
	}
	// Fallback for complex receivers (generic types)
	return exprToString(fset, typ)
}

// buildFunctionSignature renders a function or method signature without the body.
func buildFunctionSignature(fset *token.FileSet, fn *ast.FuncDecl) string {
	var buf bytes.Buffer
	buf.WriteString("func ")
	if fn.Recv != nil {
		buf.WriteString(fieldListToString(fset, fn.Recv))
		buf.WriteString(" ")
	}
	buf.WriteString(fn.Name.Name)
	buf.WriteString(fieldListToString(fset, fn.Type.Params))
	if fn.Type.Results != nil && fn.Type.Results.NumFields() > 0 {
		// Single unnamed return: no parens
		if fn.Type.Results.NumFields() == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			buf.WriteString(" ")
			buf.WriteString(exprToString(fset, fn.Type.Results.List[0].Type))
		} else {
			buf.WriteString(" ")
			buf.WriteString(fieldListToString(fset, fn.Type.Results))
		}
	}

	return buf.String()
}

// extractGenDecl extracts exported symbols from a generic declaration
func extractGenDecl(fset *token.FileSet, gd *ast.GenDecl) []Symbol {
	if gd.Tok == token.IMPORT {
		return nil
	}
	var symbols []Symbol
	// for grouped const/var blocks, determine the group name
	groupName := firstExportedName(gd)

	for _, spec := range gd.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if sym, ok := extractTypeSpec(fset, gd, s); ok {
				symbols = append(symbols, sym)
			}
		case *ast.ValueSpec:
			symbols = append(symbols, extractValueSpec(fset, gd, s, groupName)...)
		}
	}
	return symbols
}

// firstExportedName returns the first exported name in a GenDecl
func firstExportedName(gd *ast.GenDecl) string {
	if len(gd.Specs) <= 1 {
		return "" // not a grouped block
	}
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range vs.Names {
			if name.IsExported() {
				return name.Name
			}
		}
	}
	return ""
}

// extractTypeSpec extracts an exported type declaration
func extractTypeSpec(fset *token.FileSet, gd *ast.GenDecl, ts *ast.TypeSpec) (Symbol, bool) {
	if !ts.Name.IsExported() {
		return Symbol{}, false
	}

	sym := Symbol{
		Name: ts.Name.Name,
		Doc:  docText(pickDoc(ts.Doc, ts.Comment, gd.Doc)),
	}

	switch t := ts.Type.(type) {
	case *ast.StructType:
		sym.Kind = KindStruct
		sym.Signature = "type " + ts.Name.Name + " struct " + exprToString(fset, t)
		sym.Fields = extractStructFields(fset, t.Fields)
	case *ast.InterfaceType:
		sym.Kind = KindInterface
		sym.Signature = "type " + ts.Name.Name + " interface " + exprToString(fset, t)
		sym.Methods = extractInterfaceMethods(fset, t.Methods)
	default:
		if ts.Assign.IsValid() {
			sym.Kind = KindTypeAlias
			sym.Signature = "type " + ts.Name.Name + " = " + exprToString(fset, ts.Type)
		} else {
			sym.Kind = KindTypeDef
			sym.Signature = "type " + ts.Name.Name + " " + exprToString(fset, ts.Type)
		}
	}
	return sym, true
}

// extractValueSpec extracts exported constants or variables from a value
func extractValueSpec(fset *token.FileSet, gd *ast.GenDecl, vs *ast.ValueSpec, group string) []Symbol {
	var symbols []Symbol
	kind := KindVar

	if gd.Tok == token.CONST {
		kind = KindConst
	}

	for _, name := range vs.Names {
		if !name.IsExported() {
			continue
		}
		sym := Symbol{
			Name:  name.Name,
			Kind:  kind,
			Doc:   docText(pickDoc(vs.Doc, vs.Comment)),
			Group: group,
		}

		keyword := "var"
		if kind == KindConst {
			keyword = "const"
		}
		if vs.Type != nil {
			sym.Signature = keyword + " " + name.Name + " " + exprToString(fset, vs.Type)
		} else {
			sym.Signature = keyword + " " + name.Name
		}
		symbols = append(symbols, sym)
	}
	return symbols
}

// extractInterfaceMethods returns declared methods from an interface type.
func extractInterfaceMethods(fset *token.FileSet, fl *ast.FieldList) []Field {
	if fl == nil {
		return nil
	}
	var methods []Field
	for _, field := range fl.List {
		if len(field.Names) == 0 {
			// Embedded interface skip it. The embedded interface's own methods
			continue
		}
		for _, name := range field.Names {
			methods = append(methods, Field{
				Name: name.Name,
				Type: exprToString(fset, field.Type),
			})
		}
	}
	return methods
}

// extractStructFields returns exported fields from a struct type.
func extractStructFields(fset *token.FileSet, fl *ast.FieldList) []Field {
	if fl == nil {
		return nil
	}
	var fields []Field
	for _, field := range fl.List {
		typeStr := exprToString(fset, field.Type)
		doc := ""
		if field.Doc != nil {
			doc = field.Doc.Text()
		} else if field.Comment != nil {
			doc = field.Comment.Text()
		}

		if len(field.Names) == 0 {
			// Embedded field
			fields = append(fields, Field{
				Name: typeStr,
				Type: typeStr,
				Doc:  doc,
			})
			continue
		}
		for _, name := range field.Names {
			if name.IsExported() {
				fields = append(fields, Field{
					Name: name.Name,
					Type: typeStr,
					Doc:  doc,
				})
			}
		}
	}
	return fields
}
