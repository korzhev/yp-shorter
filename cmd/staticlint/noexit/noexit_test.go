package noexit_test

import (
	"testing"

	"github.com/korzhev/yp-shorter/cmd/staticlint/noexit"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), noexit.Analyzer,
		"direct", "alias", "dot", "allowed", "other", "runner_test")
}
