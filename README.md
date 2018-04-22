# SemanticMerge external parser wrapper in Go

## Usage

```go
package main

import (
	"flag"
	"github.com/imuli/go-semantic/ast"
	"github.com/imuli/go-semantic/api"
)

func Parse(source io.Reader, name string) (ast.File, error) {
	// read from source and parse into an abstract syntax tree here
}

func main() {
	flag.Parse()
	api.Run(Parse)
}
```

