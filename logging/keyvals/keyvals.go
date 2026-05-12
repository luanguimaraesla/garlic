package keyvals

import (
	"github.com/luanguimaraesla/garlic/logging"
)

// Logger adapts a sugared zap logger to the keyvals-style logger
// interface (Debug/Info/Warn/Error with msg + kv variadics, plus
// With). It is structurally compatible with go.temporal.io/sdk/log.Logger
// and similar interfaces from go-kit log and log15.
type Logger struct {
	sugar *logging.SugaredLogger
}

// NewLogger wraps l so it can be passed to libraries expecting a
// keyvals-style logger. If l is nil, the global logger is used.
func NewLogger(l *logging.Logger) *Logger {
	if l == nil {
		l = logging.Global()
	}
	return &Logger{sugar: l.Sugar()}
}

func (l *Logger) Debug(msg string, kv ...any) { l.sugar.Debugw(msg, kv...) }
func (l *Logger) Info(msg string, kv ...any)  { l.sugar.Infow(msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.sugar.Warnw(msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.sugar.Errorw(msg, kv...) }

// With returns a Logger that always logs the supplied key-value pairs.
// The returned value itself satisfies the keyvals interface, so chained
// calls keep working.
func (l *Logger) With(kv ...any) *Logger {
	return &Logger{sugar: l.sugar.With(kv...)}
}
