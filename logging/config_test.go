//go:build unit
// +build unit

package logging

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigOutputPathsTags(t *testing.T) {
	field, ok := reflect.TypeOf(Config{}).FieldByName("OutputPaths")
	require.True(t, ok)

	for _, tag := range []string{"json", "mapstructure", "yaml"} {
		assert.Equal(t, "output_paths", field.Tag.Get(tag))
	}
}

func TestConfigParseOutputPaths(t *testing.T) {
	file := filepath.Join(t.TempDir(), "explicit.log")
	config := &Config{
		Level:       "info",
		Encoding:    "json",
		OutputPaths: []string{"stderr", file},
	}

	parsed := config.Parse()

	assert.Equal(t, []string{"stderr", file}, parsed.OutputPaths)
	assert.Equal(t, []string{"stderr"}, parsed.ErrorOutputPaths)
}

func TestConfigParseDefaultOutputPaths(t *testing.T) {
	cases := []struct {
		name        string
		outputPaths []string
	}{
		{"nil", nil},
		{"empty", []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				Level:       "info",
				Encoding:    "json",
				OutputPaths: tc.outputPaths,
			}

			assert.Equal(t, []string{"stdout"}, config.Parse().OutputPaths)
		})
	}
}

func TestDefaultsOutputPaths(t *testing.T) {
	assert.Equal(t, []string{"stdout"}, Defaults().OutputPaths)
}

func TestConfigParseInvalidOutputPathFails(t *testing.T) {
	unopenable := filepath.Join(t.TempDir(), "missing", "app.log")
	config := &Config{
		Level:       "info",
		Encoding:    "json",
		OutputPaths: []string{unopenable},
	}

	parsed := config.Parse()
	require.Equal(t, []string{unopenable}, parsed.OutputPaths)

	_, err := parsed.Build()
	require.Error(t, err)
}

func TestConfigBuildWritesToFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "build.log")
	config := &Config{
		Level:       "info",
		Encoding:    "json",
		OutputPaths: []string{file},
	}

	logger, err := config.Parse().Build()
	require.NoError(t, err)

	logger.Info("built logger reached the configured file")
	require.NoError(t, logger.Sync())

	content, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(content), "built logger reached the configured file")
}

// Init's duplicate-call guard is a Fatal that exits the process, so this must
// stay the only executed Init or Global call in the logging test binary.
func TestInitAppliesConfiguredOutputPaths(t *testing.T) {
	file := filepath.Join(t.TempDir(), "global.log")

	Init(&Config{
		Level:       "info",
		Encoding:    "json",
		OutputPaths: []string{file},
	})

	logger := Global()
	logger.Info("global logger reached the configured file")
	require.NoError(t, logger.Sync())

	content, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(content), "global logger reached the configured file")
}
