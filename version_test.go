package main

import (
	"runtime/debug"
	"testing"
)

func TestCurrentVersion(t *testing.T) {
	testCases := []struct {
		name        string
		version     string
		buildInfo   *debug.BuildInfo
		buildInfoOK bool
		wantVersion string
	}{
		{
			name:        "linker injected version wins",
			version:     "v1.2.3",
			buildInfo:   &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}},
			buildInfoOK: true,
			wantVersion: "v1.2.3",
		},
		{
			name:        "module version from go install",
			version:     "dev",
			buildInfo:   &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}},
			buildInfoOK: true,
			wantVersion: "v1.2.3",
		},
		{
			name:    "devel build falls back to vcs revision",
			version: "dev",
			buildInfo: &debug.BuildInfo{
				Main: debug.Module{Version: develBuildVersion},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdef1234567890"},
				},
			},
			buildInfoOK: true,
			wantVersion: "abcdef123456",
		},
		{
			name:    "dirty devel build appends suffix",
			version: "dev",
			buildInfo: &debug.BuildInfo{
				Main: debug.Module{Version: develBuildVersion},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdef1234567890"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			buildInfoOK: true,
			wantVersion: "abcdef123456-dirty",
		},
		{
			name:        "missing build info falls back to version",
			version:     "dev",
			buildInfo:   nil,
			buildInfoOK: false,
			wantVersion: "dev",
		},
		{
			name:        "missing revision falls back to version",
			version:     "dev",
			buildInfo:   &debug.BuildInfo{Main: debug.Module{Version: develBuildVersion}},
			buildInfoOK: true,
			wantVersion: "dev",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			productionVersion := Version
			productionReadBuildInfo := readBuildInfo
			Version = tc.version
			readBuildInfo = func() (*debug.BuildInfo, bool) {
				return tc.buildInfo, tc.buildInfoOK
			}
			defer func() {
				Version = productionVersion
				readBuildInfo = productionReadBuildInfo
			}()

			got := currentVersion()
			if got != tc.wantVersion {
				t.Fatalf("currentVersion() = %q, want %q", got, tc.wantVersion)
			}
		})
	}
}
