package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/mkeeler/goimport-rewrite/pkg/rewrite"
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

// Taken from: https://github.com/golang/go/blob/3813edf26edb78620632dc9c7d66096e5b2b5019/src/cmd/fix/main.go#L235-L239
func isGoFile(f os.FileInfo) bool {
	// ignore non-Go files
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: goimport-rewrite -r <import path>:<new import path> [-r <import path>:<new import path>] [path ...]\n\n")
	fmt.Fprintf(os.Stderr, "The list of paths may be single files or directories. If directories all .go files within that directory will be processed\n\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(2)
}

func main() {
	prefix := false
	rewrites := make(map[string]string)

	flag.Var(MapVar(rewrites), "r", "Import to rewrite. Expected format is '<old path>:<new path>'.")
	flag.BoolVar(&prefix, "prefix", false, "Perform prefix replacements instead of exact matching/replacement")
	flag.Usage = usage
	flag.Parse()

	if len(rewrites) < 1 {
		fmt.Fprintf(os.Stderr, "At least one import rewrite is required\n")
		os.Exit(1)
	}

	var processor rewriteProcessor
	if prefix {
		processor.rewriter = rewrite.NewPrefixRewriter(rewrites)
	} else {
		processor.rewriter = rewrite.NewExactRewriter(rewrites)
	}

	if flag.NArg() == 0 {
		if err := processor.processFile("standard input", true); err != nil {
			fmt.Fprintf(os.Stderr, "error processing standard input: %v\n", err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	for i := 0; i < flag.NArg(); i++ {
		path := flag.Arg(i)

		dir, err := os.Stat(path)
		if err == nil {
			if dir.IsDir() {
				err = processor.walkDir(path)
			} else {
				err = processor.processFile(path, false)
			}
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "encountered a fatal error: %v\n", err)
			os.Exit(2)
		}
	}
	os.Exit(0)
}

type rewriteProcessor struct {
	rewriter rewrite.ImportRewriter
}

func (p *rewriteProcessor) processFile(filename string, useStdin bool) error {
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

	rewritten, output, err := p.rewriter.RewriteImports(filename, string(src))
	if err != nil {
		return err
	}

	if !rewritten {
		return nil
	}

	fmt.Fprintf(os.Stderr, "%s: rewrote imports\n", filename)

	if useStdin {
		os.Stdout.Write([]byte(output))
		return nil
	}

	return ioutil.WriteFile(filename, []byte(output), 0)
}

func (p *rewriteProcessor) walkDir(path string) error {
	return filepath.Walk(path, p.visitFile)
}

func (p *rewriteProcessor) visitFile(path string, f os.FileInfo, err error) error {
	if err == nil && isGoFile(f) {
		err = p.processFile(path, false)
	}
	if err != nil {
		return err
	}
	return nil
}
