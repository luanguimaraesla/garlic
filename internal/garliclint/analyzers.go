package garliclint

import "golang.org/x/tools/go/analysis"

// DefaultAnalyzers returns the analyzer set shipped by garliclint.
func DefaultAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{PropagationAnalyzer}
}

// AllAnalyzers returns every implemented analyzer, including those outside
// the shipped DefaultAnalyzers set.
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
