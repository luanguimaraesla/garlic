//go:build unit

package garliclint

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestPropagation(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), PropagationAnalyzer, "propagation", "propagationfallback")
}

func TestDefaultAnalyzers(t *testing.T) {
	analyzers := DefaultAnalyzers()
	if len(analyzers) != 1 || analyzers[0] != PropagationAnalyzer {
		t.Fatalf("DefaultAnalyzers() = %v, want only PropagationAnalyzer", analyzers)
	}
}

func TestStdlibErrors(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), StdlibErrorsAnalyzer, "stdliberrors")
}

func TestEctxParam(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), EctxParamAnalyzer, "ectxparam")
}

func TestZapError(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ZapErrorAnalyzer, "zaperror")
}

func TestManualLogger(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ManualLoggerAnalyzer, "manuallogger")
}

func TestManualTx(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ManualTxAnalyzer, "manualtx")
}

func TestRawValidator(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), RawValidatorAnalyzer, "rawvalidator")
}

func TestManualWriteError(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ManualWriteErrorAnalyzer, "manualwriteerr")
}

func TestUnitTestTag(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), UnitTestTagAnalyzer, "unittesttag")
}
