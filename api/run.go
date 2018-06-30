package api

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/imuli/go-semantic/ast"
	"golang.org/x/text/encoding/ianaindex"
	"io"
	"io/ioutil"
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
var parser func([]byte, string) (*ast.File, error)

func init() {
	flag.IntVar(&protocol, "proto", 2, "SemanticMerge protocol `version` (1 or 2)")
}

func runParser(source io.Reader, name string, codeName string, dest io.Writer) error {
	if codeName == "" {
		codeName = "UTF-8"
	}
	// look up encoding names
	code, err := ianaindex.IANA.Encoding(codeName)
	if err != nil {
		return err
	}
	if code != nil { // hack around encodings without entries in .../encoding/ianaindex
		source = code.NewDecoder().Reader(source)
	}
	// read the file into memory
	buf, err := ioutil.ReadAll(source)
	if err != nil {
		return err
	}
	// run the source through the decoder
	file, err := parser(buf, name)
	if err != nil {
		return err
	}

	// clean up syntax tree
	vitals := ast.MakeVitals(buf)
	file = vitals.CleanFile(file)
	if file == nil {
		return errors.New("CleanFile failure")
	}

	// convert to yaml
	b, err := yaml.Marshal(file)
	if err != nil {
		return err
	}
	_, err = dest.Write(b)
	return err
}

func shellParser(sourceFile string, codeName string, destFile string) error {
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

	err = runParser(source, sourceFile, codeName, dest)
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

	// parse input two or three lines at a time until receiving "end"
	// 1. source filename
	// 2. protocol v2 adds source file encoding here
	// 3. destination filename
	scanner := bufio.NewScanner(os.Stdin)
	var sourceFile string = ""
	var codeName string = ""
	var destFile string = ""

	for scanner.Scan() {
		line := scanner.Text()
		switch true {
		case line == "end":
			return
		case sourceFile == "":
			sourceFile = line
		case protocol > 1 && codeName == "":
			codeName = line
		case destFile == "":
			destFile = line

			// launch the parser
			err := shellParser(sourceFile, codeName, destFile)
			if err == nil {
				fmt.Print("OK\n")
			} else {
				fmt.Print("KO\n")
			}

			// clear variables to read another batch
			sourceFile = ""
			codeName = ""
			destFile = ""
		}
	}
}

func Run(parse func([]byte, string) (*ast.File, error)) {
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
