// Command garliclint checks Garlic error-propagation conventions.
//
// It can be installed with go install
// github.com/luanguimaraesla/garlic/cmd/garliclint@<tag> after the first Garlic
// release that includes this command. Until then, build or run it from source.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/luanguimaraesla/garlic/garliclint"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" || arg == "-V" {
			fmt.Printf("garliclint version %s\n", currentVersion())
			return
		}
	}
	multichecker.Main(garliclint.DefaultAnalyzers()...)
}

func currentVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	return effectiveVersion(version, buildInfo)
}

func effectiveVersion(linkerVersion string, buildInfo *debug.BuildInfo) string {
	if linkerVersion != "dev" || buildInfo == nil || isLocalCheckout(buildInfo) || buildInfo.Main.Version == "" || buildInfo.Main.Version == "(devel)" {
		return linkerVersion
	}
	return buildInfo.Main.Version
}

func isLocalCheckout(buildInfo *debug.BuildInfo) bool {
	for _, setting := range buildInfo.Settings {
		if setting.Key == "vcs.revision" {
			return true
		}
	}
	return false
}
