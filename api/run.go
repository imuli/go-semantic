package api

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/imuli/go-semantic/ast"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"io"
	"os"
)

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s [options] shell {flagfile}\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tshell interactive mode\n")
	fmt.Fprintf(os.Stderr, "  %s [options] {source}\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tparse single file\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
	os.Exit(1)
}

var protocol int
var parser func(io.Reader, string, encoding.Encoding) (ast.File, error)

func init() {
	flag.IntVar(&protocol, "proto", 2, "protocol `version` (1 or 2)")
}

func runParser(source io.Reader, name string, encoding string, dest io.Writer) error {
	// look up encoding names
	code, err := ianaindex.IANA.Encoding(encoding)
	if err != nil {
		// also use this database
		code, err = ianaindex.MIB.Encoding(encoding)
		if err != nil {
			return err
		}
	}
	ast, err := parser(source, name, code)
	if err != nil {
		return err
	}
	b, err := yaml.Marshal(ast)
	if err != nil {
		return err
	}
	_, err = dest.Write(b)
	return err
}

func shellParser(sourceFile string, encoding string, destFile string) error {
	source, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer dest.Close()

	err = runParser(source, sourceFile, encoding, dest)
	if err != nil {
		return err
	}

	return dest.Sync()
}

func shell(flagFile string) {
	// create flagFile to indicate readiness
	flag, err := os.Create(flagFile)
	if err != nil {
		return
	}
	flag.Close()

	// parse input two or three lines at a time
	// 1. source filename
	// 2. protocol v2 adds source file encoding here
	// 3. destination filename
	scanner := bufio.NewScanner(os.Stdin)
	var sourceFile string = ""
	var encoding string = ""
	var destFile string = ""

	for scanner.Scan() {
		line := scanner.Text()
		switch true {
		case sourceFile == "":
			sourceFile = line
		case protocol > 1 && encoding == "":
			encoding = line
		case destFile == "":
			destFile = line

			// launch the parser
			err := shellParser(sourceFile, encoding, destFile)
			if err == nil {
				fmt.Print("OK\n")
			} else {
				fmt.Print("KO\n")
			}

			// clear variables to read another batch
			sourceFile = ""
			encoding = ""
			destFile = ""
		}
	}
}

func Run(parse func(io.Reader, string, encoding.Encoding) (ast.File, error)) {
	parser = parse
	args := flag.Args()
	switch true {
	case len(args) == 2 && args[0] == "shell":
		flagFile := args[1]
		shell(flagFile)
	case len(args) == 1:
		sourceFile := args[0]
		f, err := os.Open(sourceFile)
		if err != nil {
			Usage()
		}
		err = runParser(f, sourceFile, "utf-8", os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	default:
		Usage()
	}
}
