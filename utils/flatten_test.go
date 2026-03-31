//go:build unit
// +build unit

package utils

import (
	"testing"
)

type innerConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

type baseConfig struct {
	Logging innerConfig `mapstructure:"logging"`
}

type appConfigSquash struct {
	BaseConfig  baseConfig `mapstructure:",squash"`
	BindAddress string     `mapstructure:"bind_address"`
}

type appConfigNested struct {
	Base        baseConfig `mapstructure:"base"`
	BindAddress string     `mapstructure:"bind_address"`
}

func TestFlattenStructSquash(t *testing.T) {
	cfg := appConfigSquash{
		BaseConfig: baseConfig{
			Logging: innerConfig{
				Level:    "info",
				Encoding: "json",
			},
		},
		BindAddress: ":8080",
	}

	result := FlattenStruct(cfg)

	expected := map[string]interface{}{
		"logging.level":    "info",
		"logging.encoding": "json",
		"bind_address":     ":8080",
	}

	if len(result) != len(expected) {
		t.Fatalf("expected %d keys, got %d: %v", len(expected), len(result), result)
	}

	for k, v := range expected {
		got, ok := result[k]
		if !ok {
			t.Errorf("missing key %q in result: %v", k, result)
			continue
		}
		if got != v {
			t.Errorf("key %q: expected %v, got %v", k, v, got)
		}
	}
}

func TestFlattenStructNested(t *testing.T) {
	cfg := appConfigNested{
		Base: baseConfig{
			Logging: innerConfig{
				Level:    "debug",
				Encoding: "console",
			},
		},
		BindAddress: ":9090",
	}

	result := FlattenStruct(cfg)

	expected := map[string]interface{}{
		"base.logging.level":    "debug",
		"base.logging.encoding": "console",
		"bind_address":          ":9090",
	}

	if len(result) != len(expected) {
		t.Fatalf("expected %d keys, got %d: %v", len(expected), len(result), result)
	}

	for k, v := range expected {
		got, ok := result[k]
		if !ok {
			t.Errorf("missing key %q in result: %v", k, result)
			continue
		}
		if got != v {
			t.Errorf("key %q: expected %v, got %v", k, v, got)
		}
	}
}
