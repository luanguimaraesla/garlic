//go:build unit
// +build unit

package keyvals

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/luanguimaraesla/garlic/logging"
)

// newTestLogger returns a debug-level zap logger writing JSON lines to
// buf, exposed as garlic's logging.Logger alias.
func newTestLogger(buf *bytes.Buffer) *logging.Logger {
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, zapcore.AddSync(buf), zapcore.DebugLevel)
	return zap.New(core)
}

func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	for dec.More() {
		var entry map[string]any
		require.NoError(t, dec.Decode(&entry))
		out = append(out, entry)
	}
	return out
}

func TestNewLoggerNilUsesGlobal(t *testing.T) {
	got := NewLogger(nil)
	require.NotNil(t, got)
	require.NotNil(t, got.sugar)
}

func TestLoggerRoutesLevels(t *testing.T) {
	cases := []struct {
		name      string
		call      func(*Logger)
		wantLevel string
		wantMsg   string
	}{
		{"debug", func(l *Logger) { l.Debug("d", "k", "v") }, "debug", "d"},
		{"info", func(l *Logger) { l.Info("i", "k", "v") }, "info", "i"},
		{"warn", func(l *Logger) { l.Warn("w", "k", "v") }, "warn", "w"},
		{"error", func(l *Logger) { l.Error("e", "k", "v") }, "error", "e"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			adapter := NewLogger(newTestLogger(buf))

			tc.call(adapter)

			entries := decodeLines(t, buf)
			require.Len(t, entries, 1)
			assert.Equal(t, tc.wantLevel, entries[0]["level"])
			assert.Equal(t, tc.wantMsg, entries[0]["msg"])
			assert.Equal(t, "v", entries[0]["k"])
		})
	}
}

func TestLoggerWithAccumulatesFields(t *testing.T) {
	buf := &bytes.Buffer{}
	adapter := NewLogger(newTestLogger(buf))

	child := adapter.With("workflow", "wf-1")
	grandchild := child.With("activity", "act-1")

	grandchild.Info("running", "attempt", 2)

	entries := decodeLines(t, buf)
	require.Len(t, entries, 1)
	entry := entries[0]
	assert.Equal(t, "running", entry["msg"])
	assert.Equal(t, "wf-1", entry["workflow"])
	assert.Equal(t, "act-1", entry["activity"])
	assert.EqualValues(t, 2, entry["attempt"])
}

func TestLoggerWithDoesNotMutateParent(t *testing.T) {
	buf := &bytes.Buffer{}
	adapter := NewLogger(newTestLogger(buf))

	_ = adapter.With("workflow", "wf-1")
	adapter.Info("parent")

	entries := decodeLines(t, buf)
	require.Len(t, entries, 1)
	_, hasWorkflow := entries[0]["workflow"]
	assert.False(t, hasWorkflow, "parent logger must not inherit child fields")
}
