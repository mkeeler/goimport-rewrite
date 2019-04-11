package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	fset     = token.NewFileSet()
	exitCode = 0
)

type MapVar map[string]string

func (m MapVar) String() string {
	raw := map[string]string(m)
	return fmt.Sprintf("%s", raw)
}

func (m MapVar) Set(input string) error {
	parts := strings.Split(input, ":")
	if len(parts) != 2 {
		return fmt.Errorf("Invalid argument format: expecting <original import>:<new import>")
	}

	m[parts[0]] = parts[1]
	return nil
}

const parserMode = parser.ParseComments

var rewrites map[string]string = make(map[string]string)

// Taken from: https://github.com/golang/go/blob/3813edf26edb78620632dc9c7d66096e5b2b5019/src/cmd/fix/main.go#L105-114
func gofmtFile(f *ast.File) ([]byte, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Taken from: https://github.com/golang/go/blob/3813edf26edb78620632dc9c7d66096e5b2b5019/src/cmd/fix/main.go#L216-L219
func report(err error) {
	scanner.PrintError(os.Stderr, err)
	exitCode = 2
}

// Taken from: https://github.com/golang/go/blob/3813edf26edb78620632dc9c7d66096e5b2b5019/src/cmd/fix/main.go#L221-L223
func walkDir(path string) {
	filepath.Walk(path, visitFile)
}

// Taken from: https://github.com/golang/go/blob/3813edf26edb78620632dc9c7d66096e5b2b5019/src/cmd/fix/main.go#L225-L232
func visitFile(path string, f os.FileInfo, err error) error {
	if err == nil && isGoFile(f) {
		err = processFile(path, false)
	}
	if err != nil {
		report(err)
	}
	return nil
}

// Taken from: https://github.com/golang/go/blob/3813edf26edb78620632dc9c7d66096e5b2b5019/src/cmd/fix/main.go#L235-L239
func isGoFile(f os.FileInfo) bool {
	// ignore non-Go files
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}

// Taken from: https://github.com/golang/go/blob/3813edf26edb78620632dc9c7d66096e5b2b5019/src/cmd/fix/fix.go#L302-L310
// importPath returns the unquoted import path of s,
// or "" if the path is not properly quoted.
func importPath(s *ast.ImportSpec) string {
	t, err := strconv.Unquote(s.Path.Value)
	if err == nil {
		return t
	}
	return ""
}

// Take from: https://github.com/golang/go/blob/3813edf26edb78620632dc9c7d66096e5b2b5019/src/cmd/fix/fix.go#L697-L709
// Then modified to do multiple rewrites based on a map
// rewriteImport rewrites any import of path oldPath to path newPath.
func rewriteImports(f *ast.File, rewrites map[string]string) (rewrote bool) {
	for _, imp := range f.Imports {
		if newPath, needsRewriting := rewrites[importPath(imp)]; needsRewriting {
			rewrote = true
			// record old End, because the default is to compute
			// it using the length of imp.Path.Value.
			imp.EndPos = imp.End()
			imp.Path.Value = strconv.Quote(newPath)
		}
	}
	return
}

func processFile(filename string, useStdin bool) error {
	var f *os.File
	var err error

	if useStdin {
		f = os.Stdin
	} else {
		f, err = os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
	}

	src, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(fset, filename, src, parserMode)
	if err != nil {
		return err
	}

	// Rewrite the imports
	if !rewriteImports(file, rewrites) {
		return nil
	}
	fmt.Fprintf(os.Stderr, "%s: rewrote imports\n", filename)

	// format the source
	newSrc, err := gofmtFile(file)
	if err != nil {
		return err
	}

	if useStdin {
		os.Stdout.Write(newSrc)
		return nil
	}

	return ioutil.WriteFile(f.Name(), newSrc, 0)
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: goimport-rewrite -r <import path>:<new import path> [-r <import path>:<new import path>] [path ...]\n\n")
	fmt.Fprintf(os.Stderr, "The list of paths may be single files or directories. If directories all .go files within that directory will be processed\n\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(2)
}

func main() {
	flag.Var(MapVar(rewrites), "r", "Import to rewrite. Expected format is '<old path>:<new path>'.")
	flag.Usage = usage
	flag.Parse()

	if len(rewrites) < 1 {
		report(fmt.Errorf("At least one import rewrite is required"))
		os.Exit(1)
	}

	if flag.NArg() == 0 {
		if err := processFile("standard input", true); err != nil {
			report(err)
		}
		os.Exit(exitCode)
	}

	for i := 0; i < flag.NArg(); i++ {
		path := flag.Arg(i)
		switch dir, err := os.Stat(path); {
		case err != nil:
			report(err)
		case dir.IsDir():
			walkDir(path)
		default:
			if err := processFile(path, false); err != nil {
				report(err)
			}
		}
	}

	os.Exit(exitCode)
}
