// Command garliclint checks Garlic error-propagation conventions.
//
// It can be installed with go install
// github.com/luanguimaraesla/garlic/cmd/garliclint@<tag> after the first Garlic
// release that includes this command. Until then, build or run it from source.
package main

import (
	"fmt"
	"os"

	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/luanguimaraesla/garlic/garliclint"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" || arg == "-V" {
			fmt.Printf("garliclint version %s\n", version)
			return
		}
	}
	multichecker.Main(garliclint.DefaultAnalyzers()...)
}
