package main

import (
	"fmt"
	"os/exec"
)

var lookPath = exec.LookPath

// ValidateRequiredDependencies verifies that the external commands
// required by the bot are available in the system PATH.
func ValidateRequiredDependencies() error {
	dependencies := []string{
		"ffmpeg",
		"yt-dlp",
	}
	for _, dep := range dependencies {
		_, err := lookPath(dep)
		if err != nil {
			return fmt.Errorf("dependency %s is not installed in the system: %w", dep, err)
		}
	}
	return nil
}
