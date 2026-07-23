package manuallogger

import "go.uber.org/zap"

func invalid() *zap.Logger {
	return zap.NewNop() // want "\\[G2.03\\]"
}

func alsoInvalid() *zap.Logger {
	return zap.New() // want "\\[G2.03\\]"
}

type tracerFactory struct{}

func (tracerFactory) NewNop() int { return 0 }

func acceptable(f tracerFactory) int {
	return f.NewNop()
}
