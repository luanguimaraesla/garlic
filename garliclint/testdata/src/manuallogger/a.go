package manuallogger

import "go.uber.org/zap"

func invalid() *zap.Logger {
	return zap.NewNop() // want "\\[G2.03\\]"
}
