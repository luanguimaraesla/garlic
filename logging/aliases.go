package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Type aliases for zap types so consumers can use the logging package
// instead of importing go.uber.org/zap and go.uber.org/zap/zapcore
// directly.
type (
	Logger          = zap.Logger
	SugaredLogger   = zap.SugaredLogger
	Field           = zap.Field
	Level           = zap.AtomicLevel
	ObjectEncoder   = zapcore.ObjectEncoder
	ObjectMarshaler = zapcore.ObjectMarshaler
	ArrayMarshaler  = zapcore.ArrayMarshaler
)

// Field constructors matching the most common zap field functions.
var (
	String   = zap.String
	Strings  = zap.Strings
	Int      = zap.Int
	Int64    = zap.Int64
	Float64  = zap.Float64
	Bool     = zap.Bool
	Error    = zap.Error
	Duration = zap.Duration
	Time     = zap.Time
	Any      = zap.Any
	Object   = zap.Object
	Array    = zap.Array
	Stringer = zap.Stringer
)

// NewNop creates a no-op Logger useful for testing.
var NewNop = zap.NewNop
