// imports.go - drops the template's unused imports from generated code
package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// pruneImports removes every import the file never references and gofmts the result.
//
// The file template imports a fixed superset of packages (io is only used by streaming
// methods, for example), so without this step the output does not compile until the
// caller runs goimports over it. Doing it here keeps generated files buildable as they
// are written, so consumers no longer need a goimports/alignment pass after `make protos`.
//
// Only imports are touched: no packages are looked up or added, which keeps generation
// hermetic and fast. Blank (_) and dot (.) imports are always kept.
func pruneImports(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	used := usedPackageNames(file)

	// Lines to delete: each unused import spec, or the whole declaration when none of
	// its imports survive. Deleting lines (rather than AST nodes) keeps the template's
	// grouping intact without leaving blank lines where the imports were.
	drop := map[int]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		kept := 0
		for _, spec := range gen.Specs {
			imp := spec.(*ast.ImportSpec)
			if name := importName(imp); name == "_" || name == "." || used[name] {
				kept++
				continue
			}
			for l := fset.Position(imp.Pos()).Line; l <= fset.Position(imp.End()).Line; l++ {
				drop[l] = true
			}
		}
		if kept == 0 {
			for l := fset.Position(gen.Pos()).Line; l <= fset.Position(gen.End()).Line; l++ {
				drop[l] = true
			}
		}
	}
	if len(drop) == 0 {
		return format.Source(src)
	}

	lines := bytes.SplitAfter(src, []byte("\n"))
	var out bytes.Buffer
	for i, line := range lines {
		if !drop[i+1] {
			out.Write(line)
		}
	}
	return format.Source(dropEmptyImportGroups(out.Bytes()))
}

// dropEmptyImportGroups collapses runs of blank lines inside import blocks, which appear
// when every import of a group was removed.
func dropEmptyImportGroups(src []byte) []byte {
	lines := strings.SplitAfter(string(src), "\n")
	var out strings.Builder
	inImports, prevBlank := false, false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "import ("):
			inImports, prevBlank = true, true
		case inImports && trimmed == ")":
			inImports = false
		case inImports && trimmed == "":
			if prevBlank {
				continue
			}
			prevBlank = true
			out.WriteString(line)
			continue
		}
		if inImports && trimmed != "" && !strings.HasPrefix(trimmed, "import (") {
			prevBlank = false
		}
		out.WriteString(line)
	}
	return []byte(out.String())
}

// usedPackageNames collects every identifier used as the left side of a selector
// (pkg.Name) that is not resolved to a local declaration.
func usedPackageNames(file *ast.File) map[string]bool {
	used := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil {
			used[id.Name] = true
		}
		return true
	})
	return used
}

// importName is the name an import is referred to by in the file: its explicit alias, or
// the package name assumed from its path the same way goimports assumes it.
func importName(imp *ast.ImportSpec) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	path, err := strconv.Unquote(imp.Path.Value)
	if err != nil {
		return ""
	}
	return assumedPackageName(path)
}

// assumedPackageName mirrors goimports' importPathToAssumedName: the last path element,
// skipping a /vN major-version suffix, with "go-" prefixes and ".go"-style suffixes
// dropped (github.com/nats-io/nats.go -> nats).
func assumedPackageName(path string) string {
	parts := strings.Split(path, "/")
	base := parts[len(parts)-1]
	if len(parts) > 1 && isMajorVersion(base) {
		base = parts[len(parts)-2]
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, func(r rune) bool {
		return !(r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9'))
	}); i >= 0 {
		base = base[:i]
	}
	return base
}

func isMajorVersion(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, r := range s[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
