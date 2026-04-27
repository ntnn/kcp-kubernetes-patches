/*
Copyright 2025 The kcp Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"strconv"
	"strings"
)

// changePackageName sets the package declaration to newPkg.
func changePackageName(file *ast.File, newPkg string) {
	file.Name.Name = newPkg
}

// addImport adds an import with the given path and optional alias.
func addImport(file *ast.File, alias, path string) {
	for _, imp := range file.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p == path {
			return
		}
	}

	newSpec := &ast.ImportSpec{
		Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)},
	}
	if alias != "" {
		newSpec.Name = ast.NewIdent(alias)
	}

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		gd.Specs = append(gd.Specs, newSpec)
		file.Imports = append(file.Imports, newSpec)
		return
	}

	gd := &ast.GenDecl{
		Tok:    token.IMPORT,
		Lparen: 1,
		Specs:  []ast.Spec{newSpec},
	}
	file.Decls = append([]ast.Decl{gd}, file.Decls...)
	file.Imports = append(file.Imports, newSpec)
}

// removeImport removes an import by path.
func removeImport(file *ast.File, path string) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		var kept []ast.Spec
		for _, spec := range gd.Specs {
			is := spec.(*ast.ImportSpec)
			p, _ := strconv.Unquote(is.Path.Value)
			if p != path {
				kept = append(kept, spec)
			}
		}
		gd.Specs = kept
	}
	var kept []*ast.ImportSpec
	for _, imp := range file.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p != path {
			kept = append(kept, imp)
		}
	}
	file.Imports = kept
}

// removeTopLevelDecl removes top-level type, func, var, or const declarations
// whose name matches one of the given names.
func removeTopLevelDecl(file *ast.File, names map[string]bool) {
	var kept []ast.Decl
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && names[d.Name.Name] {
				continue
			}
		case *ast.GenDecl:
			if d.Tok == token.TYPE || d.Tok == token.VAR || d.Tok == token.CONST {
				var keptSpecs []ast.Spec
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if names[s.Name.Name] {
							continue
						}
					case *ast.ValueSpec:
						allMatch := true
						for _, n := range s.Names {
							if !names[n.Name] {
								allMatch = false
								break
							}
						}
						if allMatch {
							continue
						}
					}
					keptSpecs = append(keptSpecs, spec)
				}
				if len(keptSpecs) == 0 {
					continue
				}
				d.Specs = keptSpecs
			}
		}
		kept = append(kept, decl)
	}
	file.Decls = kept
}

// removeMethodsOnType removes methods with the given receiver type and method names.
func removeMethodsOnType(file *ast.File, typeName string, methodNames map[string]bool) {
	var kept []ast.Decl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv != nil && methodNames[fd.Name.Name] {
			if recvTypeName(fd) == typeName {
				continue
			}
		}
		kept = append(kept, decl)
	}
	file.Decls = kept
}

func recvTypeName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// qualifyIdents rewrites bare identifiers in names to qualifier.X.
// localDefs are names defined locally that should NOT be qualified.
func qualifyIdents(file *ast.File, qualifier string, names map[string]bool, localDefs map[string]bool) {
	defIdents := collectDefIdents(file)
	rewriteIdentsInFile(file, qualifier, names, localDefs, defIdents)
}

func collectDefIdents(file *ast.File) map[*ast.Ident]bool {
	defs := make(map[*ast.Ident]bool)
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			defs[d.Name] = true
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					defs[s.Name] = true
				case *ast.ValueSpec:
					for _, n := range s.Names {
						defs[n] = true
					}
				}
			}
		}
	}
	return defs
}

func rewriteIdentsInFile(file *ast.File, q string, names, localDefs map[string]bool, defIdents map[*ast.Ident]bool) {
	for i, decl := range file.Decls {
		file.Decls[i] = rewriteIdentsInDecl(decl, q, names, localDefs, defIdents)
	}
}

func shouldQualify(id *ast.Ident, names, localDefs map[string]bool, defIdents map[*ast.Ident]bool) bool {
	return names[id.Name] && !localDefs[id.Name] && !defIdents[id]
}

func rewriteExpr(expr ast.Expr, q string, names, localDefs map[string]bool, defIdents map[*ast.Ident]bool) ast.Expr {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if shouldQualify(e, names, localDefs, defIdents) {
			return &ast.SelectorExpr{X: ast.NewIdent(q), Sel: ast.NewIdent(e.Name)}
		}
		return e
	case *ast.StarExpr:
		e.X = rewriteExpr(e.X, q, names, localDefs, defIdents)
	case *ast.ArrayType:
		e.Elt = rewriteExpr(e.Elt, q, names, localDefs, defIdents)
	case *ast.MapType:
		e.Key = rewriteExpr(e.Key, q, names, localDefs, defIdents)
		e.Value = rewriteExpr(e.Value, q, names, localDefs, defIdents)
	case *ast.SelectorExpr:
		e.X = rewriteExpr(e.X, q, names, localDefs, defIdents)
	case *ast.CallExpr:
		e.Fun = rewriteExpr(e.Fun, q, names, localDefs, defIdents)
		for i := range e.Args {
			e.Args[i] = rewriteExpr(e.Args[i], q, names, localDefs, defIdents)
		}
	case *ast.CompositeLit:
		e.Type = rewriteExpr(e.Type, q, names, localDefs, defIdents)
		for i := range e.Elts {
			e.Elts[i] = rewriteExpr(e.Elts[i], q, names, localDefs, defIdents)
		}
	case *ast.KeyValueExpr:
		e.Value = rewriteExpr(e.Value, q, names, localDefs, defIdents)
	case *ast.UnaryExpr:
		e.X = rewriteExpr(e.X, q, names, localDefs, defIdents)
	case *ast.BinaryExpr:
		e.X = rewriteExpr(e.X, q, names, localDefs, defIdents)
		e.Y = rewriteExpr(e.Y, q, names, localDefs, defIdents)
	case *ast.ParenExpr:
		e.X = rewriteExpr(e.X, q, names, localDefs, defIdents)
	case *ast.TypeAssertExpr:
		e.X = rewriteExpr(e.X, q, names, localDefs, defIdents)
		e.Type = rewriteExpr(e.Type, q, names, localDefs, defIdents)
	case *ast.IndexExpr:
		e.X = rewriteExpr(e.X, q, names, localDefs, defIdents)
		e.Index = rewriteExpr(e.Index, q, names, localDefs, defIdents)
	case *ast.SliceExpr:
		e.X = rewriteExpr(e.X, q, names, localDefs, defIdents)
	case *ast.FuncLit:
		rewriteFieldList(e.Type.Params, q, names, localDefs, defIdents)
		rewriteFieldList(e.Type.Results, q, names, localDefs, defIdents)
		rewriteStmtList(e.Body.List, q, names, localDefs, defIdents)
	case *ast.InterfaceType:
		rewriteFieldList(e.Methods, q, names, localDefs, defIdents)
	case *ast.StructType:
		rewriteFieldList(e.Fields, q, names, localDefs, defIdents)
	case *ast.FuncType:
		rewriteFieldList(e.Params, q, names, localDefs, defIdents)
		rewriteFieldList(e.Results, q, names, localDefs, defIdents)
	case *ast.ChanType:
		e.Value = rewriteExpr(e.Value, q, names, localDefs, defIdents)
	case *ast.Ellipsis:
		e.Elt = rewriteExpr(e.Elt, q, names, localDefs, defIdents)
	}
	return expr
}

func rewriteFieldList(fl *ast.FieldList, q string, names, localDefs map[string]bool, defIdents map[*ast.Ident]bool) {
	if fl == nil {
		return
	}
	for _, f := range fl.List {
		f.Type = rewriteExpr(f.Type, q, names, localDefs, defIdents)
	}
}

func rewriteStmt(stmt ast.Stmt, q string, names, localDefs map[string]bool, defIdents map[*ast.Ident]bool) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		s.X = rewriteExpr(s.X, q, names, localDefs, defIdents)
	case *ast.AssignStmt:
		for i := range s.Rhs {
			s.Rhs[i] = rewriteExpr(s.Rhs[i], q, names, localDefs, defIdents)
		}
		for i := range s.Lhs {
			s.Lhs[i] = rewriteExpr(s.Lhs[i], q, names, localDefs, defIdents)
		}
	case *ast.ReturnStmt:
		for i := range s.Results {
			s.Results[i] = rewriteExpr(s.Results[i], q, names, localDefs, defIdents)
		}
	case *ast.DeclStmt:
		if gd, ok := s.Decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					vs.Type = rewriteExpr(vs.Type, q, names, localDefs, defIdents)
					for i := range vs.Values {
						vs.Values[i] = rewriteExpr(vs.Values[i], q, names, localDefs, defIdents)
					}
				}
			}
		}
	case *ast.IfStmt:
		rewriteStmt(s.Init, q, names, localDefs, defIdents)
		s.Cond = rewriteExpr(s.Cond, q, names, localDefs, defIdents)
		rewriteStmtList(s.Body.List, q, names, localDefs, defIdents)
		rewriteStmt(s.Else, q, names, localDefs, defIdents)
	case *ast.ForStmt:
		rewriteStmt(s.Init, q, names, localDefs, defIdents)
		s.Cond = rewriteExpr(s.Cond, q, names, localDefs, defIdents)
		rewriteStmt(s.Post, q, names, localDefs, defIdents)
		rewriteStmtList(s.Body.List, q, names, localDefs, defIdents)
	case *ast.RangeStmt:
		s.X = rewriteExpr(s.X, q, names, localDefs, defIdents)
		rewriteStmtList(s.Body.List, q, names, localDefs, defIdents)
	case *ast.BlockStmt:
		if s != nil {
			rewriteStmtList(s.List, q, names, localDefs, defIdents)
		}
	case *ast.SwitchStmt:
		rewriteStmt(s.Init, q, names, localDefs, defIdents)
		s.Tag = rewriteExpr(s.Tag, q, names, localDefs, defIdents)
		rewriteStmtList(s.Body.List, q, names, localDefs, defIdents)
	case *ast.TypeSwitchStmt:
		rewriteStmt(s.Init, q, names, localDefs, defIdents)
		rewriteStmt(s.Assign, q, names, localDefs, defIdents)
		rewriteStmtList(s.Body.List, q, names, localDefs, defIdents)
	case *ast.CaseClause:
		for i := range s.List {
			s.List[i] = rewriteExpr(s.List[i], q, names, localDefs, defIdents)
		}
		rewriteStmtList(s.Body, q, names, localDefs, defIdents)
	case *ast.SelectStmt:
		rewriteStmtList(s.Body.List, q, names, localDefs, defIdents)
	case *ast.CommClause:
		rewriteStmt(s.Comm, q, names, localDefs, defIdents)
		rewriteStmtList(s.Body, q, names, localDefs, defIdents)
	case *ast.SendStmt:
		s.Chan = rewriteExpr(s.Chan, q, names, localDefs, defIdents)
		s.Value = rewriteExpr(s.Value, q, names, localDefs, defIdents)
	case *ast.GoStmt:
		s.Call = rewriteExpr(s.Call, q, names, localDefs, defIdents).(*ast.CallExpr)
	case *ast.DeferStmt:
		s.Call = rewriteExpr(s.Call, q, names, localDefs, defIdents).(*ast.CallExpr)
	case *ast.IncDecStmt:
		s.X = rewriteExpr(s.X, q, names, localDefs, defIdents)
	case *ast.LabeledStmt:
		rewriteStmt(s.Stmt, q, names, localDefs, defIdents)
	}
}

func rewriteStmtList(stmts []ast.Stmt, q string, names, localDefs map[string]bool, defIdents map[*ast.Ident]bool) {
	for _, stmt := range stmts {
		rewriteStmt(stmt, q, names, localDefs, defIdents)
	}
}

func rewriteIdentsInDecl(decl ast.Decl, q string, names, localDefs map[string]bool, defIdents map[*ast.Ident]bool) ast.Decl {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		rewriteFieldList(d.Recv, q, names, localDefs, defIdents)
		rewriteFieldList(d.Type.Params, q, names, localDefs, defIdents)
		rewriteFieldList(d.Type.Results, q, names, localDefs, defIdents)
		if d.Body != nil {
			rewriteStmtList(d.Body.List, q, names, localDefs, defIdents)
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				s.Type = rewriteExpr(s.Type, q, names, localDefs, defIdents)
			case *ast.ValueSpec:
				s.Type = rewriteExpr(s.Type, q, names, localDefs, defIdents)
				for i := range s.Values {
					s.Values[i] = rewriteExpr(s.Values[i], q, names, localDefs, defIdents)
				}
			}
		}
	}
	return decl
}

// renderFile renders the AST back to formatted Go source.
func renderFile(fset *token.FileSet, file *ast.File) ([]byte, error) {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, file); err != nil {
		return nil, fmt.Errorf("printing AST: %w", err)
	}
	return format.Source(buf.Bytes())
}

// addCopyrightModification inserts a kcp copyright line after the Kubernetes one.
func addCopyrightModification(src []byte) []byte {
	re := regexp.MustCompile(`(Copyright \d+ The Kubernetes Authors\.)`)
	return re.ReplaceAll(src, []byte("${1}\nModifications Copyright 2025 The kcp Authors."))
}

func parseSnippet(src string) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing snippet: %w\n%s", err, src)
	}
	return f, fset, nil
}

func renderDecl(fset *token.FileSet, decl ast.Decl) (string, error) {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, decl); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func toSet(ss ...string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// extractFunc extracts a function declaration by name from an AST and renders it.
func extractFunc(fset *token.FileSet, file *ast.File, funcName string) (string, error) {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil {
			continue
		}
		if fd.Name.Name == funcName {
			return renderDecl(fset, decl)
		}
	}
	return "", fmt.Errorf("function %s not found", funcName)
}

// replaceInSource does a simple string replacement in source bytes.
func replaceInSource(src []byte, old, new string) []byte {
	return bytes.ReplaceAll(src, []byte(old), []byte(new))
}

// replaceFirstInSource replaces the first occurrence only.
func replaceFirstInSource(src []byte, old, new string) []byte {
	return bytes.Replace(src, []byte(old), []byte(new), 1)
}

// insertAfterLine inserts text after the first line matching the pattern.
func insertAfterLine(src []byte, pattern, insertion string) []byte {
	lines := strings.Split(string(src), "\n")
	var result []string
	for _, line := range lines {
		result = append(result, line)
		if strings.Contains(line, pattern) {
			result = append(result, insertion)
		}
	}
	return []byte(strings.Join(result, "\n"))
}
