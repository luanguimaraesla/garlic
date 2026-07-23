package manuallogger

import . "go.uber.org/zap"

func dotImported() *Logger {
	return NewNop() // want "\\[G2.03\\]"
}
