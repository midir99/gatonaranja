//go:build !unix

package main

import "os/exec"

// commandContext is a test seam for creating yt-dlp commands.
var commandContext = exec.CommandContext
