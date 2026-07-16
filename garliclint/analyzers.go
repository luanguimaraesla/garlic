package garliclint

import "golang.org/x/tools/go/analysis"

// DefaultAnalyzers returns the analyzer set shipped by garliclint.
func DefaultAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{PropagationAnalyzer}
}

// New returns all Garlic convention analyzers. Configuration is reserved for
// future rule-level configuration.
func New(_ any) ([]*analysis.Analyzer, error) {
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
	}, nil
}
