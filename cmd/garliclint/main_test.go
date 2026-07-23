//go:build unit

package main

import (
	"runtime/debug"
	"testing"
)

func TestEffectiveVersion(t *testing.T) {
	tests := []struct {
		name          string
		linkerVersion string
		buildInfo     *debug.BuildInfo
		want          string
	}{
		{
			name:          "linker version",
			linkerVersion: "v1.2.3",
			buildInfo:     &debug.BuildInfo{Main: debug.Module{Version: "v4.5.6"}},
			want:          "v1.2.3",
		},
		{
			name:          "installed module version",
			linkerVersion: "dev",
			buildInfo:     &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}},
			want:          "v1.2.3",
		},
		{
			name:          "local checkout",
			linkerVersion: "dev",
			buildInfo: &debug.BuildInfo{
				Main:     debug.Module{Version: "v1.2.3-0.20260716205051-eff39174d49f+dirty"},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "eff39174"}},
			},
			want: "dev",
		},
		{
			name:          "development build",
			linkerVersion: "dev",
			buildInfo:     &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			want:          "dev",
		},
		{
			name:          "unavailable build information",
			linkerVersion: "dev",
			want:          "dev",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := effectiveVersion(test.linkerVersion, test.buildInfo); got != test.want {
				t.Fatalf("effectiveVersion() = %q, want %q", got, test.want)
			}
		})
	}
}
