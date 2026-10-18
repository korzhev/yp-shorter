// Command staticlint runs a collection of static analyzers for Go code.
//
// The collection includes vet.Suite, additional analyzers from passes,
// modernize.Suite, errcheck, the custom noexit analyzer, and analyzers from
// staticcheck.Analyzers whose names start with SA or ST.
//
// # Usage
//
// Run from the module root to check all packages:
//
//	go run ./cmd/staticlint ./...
//
// Check only for direct calls to os.Exit in the main function of package main:
//
//	go run ./cmd/staticlint -noexit ./...
//
// Display help for flags and available analyzers:
//
//	go run ./cmd/staticlint -help
package main

import (
	"strings"

	"github.com/kisielk/errcheck/errcheck"
	"github.com/korzhev/yp-shorter/cmd/staticlint/noexit"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/findcall"
	"golang.org/x/tools/go/analysis/passes/gofix"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/inline"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/analysis/passes/modernize"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/pkgfact"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/usesgenerics"
	"honnef.co/go/tools/staticcheck"
)

// main collects the analyzers and delegates to multichecker,
// which processes command-line arguments and runs the checks.
func main() {
	mychecks := []*analysis.Analyzer{}
	// Public passes analyzers not included in vet.Suite (x/tools v0.52.0).
	mychecks = append(mychecks,
		atomicalign.Analyzer,
		buildssa.Analyzer,
		ctrlflow.Analyzer,
		deepequalerrors.Analyzer,
		fieldalignment.Analyzer,
		findcall.Analyzer,
		gofix.Analyzer,
		httpmux.Analyzer,
		inline.Analyzer,
		inspect.Analyzer,
		nilness.Analyzer,
		pkgfact.Analyzer,
		reflectvaluecompare.Analyzer,
		shadow.Analyzer,
		sortslice.Analyzer,
		unusedwrite.Analyzer,
		usesgenerics.Analyzer,
		// Third-party analyzer.
		errcheck.Analyzer,
		noexit.Analyzer,
	)
	mychecks = append(mychecks, modernize.Suite...)
	for _, v := range staticcheck.Analyzers {
		// Include checks with the selected name prefixes.
		if strings.HasPrefix(v.Analyzer.Name, "SA") || strings.HasPrefix(v.Analyzer.Name, "ST") {
			mychecks = append(mychecks, v.Analyzer)
		}
	}
	multichecker.Main(mychecks...)
}
