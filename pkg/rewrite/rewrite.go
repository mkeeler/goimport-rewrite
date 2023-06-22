package rewrite

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	radix "github.com/mkeeler/go-radix"
)

func gofmtFile(f *ast.File, fset *token.FileSet) ([]byte, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func importPathFromSpec(spec *ast.ImportSpec) string {
	importPath, err := strconv.Unquote(spec.Path.Value)
	if err == nil {
		return importPath
	}
	return ""
}

func updateImportSpec(spec *ast.ImportSpec, importPath string) {
	// record old End, because the default is to compute
	// it using the length of imp.Path.Value.
	spec.EndPos = spec.End()
	spec.Path.Value = strconv.Quote(importPath)
}

type ImportRewriter interface {
	RewriteAstImports(f *ast.File) bool
	RewriteImports(filename string, source string) (bool, string, error)
}

type rewriter struct {
	mapper importMapper
}

func (rw *rewriter) RewriteAstImports(f *ast.File) bool {
	rewrote := false
	for _, spec := range f.Imports {
		original := importPathFromSpec(spec)

		desired := rw.mapper.desiredImportPath(original)

		if original != desired {
			rewrote = true
			updateImportSpec(spec, desired)
		}
	}
	return rewrote
}

func (rw *rewriter) RewriteImports(filename string, source string) (bool, string, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		return false, source, fmt.Errorf("error parsing original source: %w", err)
	}

	rewrote := rw.RewriteAstImports(file)
	if !rewrote {
		return false, source, nil
	}

	out, err := gofmtFile(file, fset)
	if err != nil {
		return false, source, fmt.Errorf("error formatting rewritten source: %w", err)
	}

	return true, string(out), nil
}

func NewExactRewriter(rules map[string]string) ImportRewriter {
	return &rewriter{
		mapper: &exactMapper{
			rules: rules,
		},
	}
}

func NewPrefixRewriter(rules map[string]string) ImportRewriter {
	mapper := &prefixMapper{
		tree: radix.New[string](),
	}

	// Initialize the radix tree
	for original, desired := range rules {
		mapper.tree.Insert(original, desired)
	}

	return &rewriter{
		mapper: mapper,
	}
}

type importMapper interface {
	desiredImportPath(string) string
}

type exactMapper struct {
	rules map[string]string
}

func (rw *exactMapper) desiredImportPath(original string) string {
	desired, found := rw.rules[original]
	if found {
		return desired
	}

	return original
}

type prefixMapper struct {
	tree *radix.Tree[string]
}

func (rw *prefixMapper) desiredImportPath(original string) string {
	originalPrefix, desiredPrefix, found := rw.tree.LongestPrefix(original)
	if !found {
		return original
	}

	return strings.Replace(original, originalPrefix, desiredPrefix, 1)
}
