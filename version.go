package main

import (
	"fmt"
	"runtime/debug"
)

const develBuildVersion = "(devel)"

// Version identifies the application version printed by -version. It defaults
// to "dev" and can be overridden at build time with -ldflags.
var Version = "dev"

// readBuildInfo is a test seam for reading Go build metadata embedded in the
// compiled binary.
var readBuildInfo = debug.ReadBuildInfo

// currentVersion returns the best available application version string. It
// prefers an explicit linker-injected version, then the Go build info module
// version, and finally falls back to VCS metadata or the default Version.
func currentVersion() string {
	if Version != "" && Version != "dev" {
		return Version
	}

	buildInfo, ok := readBuildInfo()
	if !ok || buildInfo == nil {
		return Version
	}

	if buildInfo.Main.Version != "" && buildInfo.Main.Version != develBuildVersion {
		return buildInfo.Main.Version
	}

	revision := ""
	modified := false
	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	if revision == "" {
		return Version
	}

	if len(revision) > 12 {
		revision = revision[:12]
	}

	if modified {
		return fmt.Sprintf("%s-dirty", revision)
	}

	return revision
}
