package garliclint

import "golang.org/x/tools/go/analysis"

// DefaultAnalyzers returns the analyzer set shipped by garliclint.
func DefaultAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{PropagationAnalyzer}
}

// AllAnalyzers returns the full registry of implemented analyzers, for future
// in-module integrations and for widening DefaultAnalyzers, which stays the
// shipped set.
func AllAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		PropagationAnalyzer,
		StdlibErrorsAnalyzer,
		EctxParamAnalyzer,
		ZapErrorAnalyzer,
		ManualLoggerAnalyzer,
		ManualTxAnalyzer,
		RawValidatorAnalyzer,
		ManualWriteErrorAnalyzer,
		UnitTestTagAnalyzer,
	}
}
