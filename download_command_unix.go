//go:build unix

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// commandContext is a test seam for creating yt-dlp commands.
var commandContext = newProcessGroupCommand

// processGroupKillGrace is the time allowed for yt-dlp and its children to
// exit after SIGTERM before SIGKILL is sent to the process group.
var processGroupKillGrace = 2 * time.Second

// newProcessGroupCommand creates a command in its own process group so canceling
// the context can terminate yt-dlp and children it starts, such as ffmpeg.
func newProcessGroupCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	killGrace := processGroupKillGrace
	cmd.Cancel = func() error {
		return terminateProcessGroup(cmd, killGrace)
	}
	cmd.WaitDelay = killGrace + time.Second

	return cmd
}

func terminateProcessGroup(cmd *exec.Cmd, killGrace time.Duration) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}

	if err := signalProcessGroup(pgid, syscall.SIGTERM); err != nil {
		return err
	}
	if waitForProcessGroupExit(pgid, killGrace) {
		return nil
	}
	return signalProcessGroup(pgid, syscall.SIGKILL)
}

func signalProcessGroup(pgid int, signal syscall.Signal) error {
	// A negative PID targets every process in the process group.
	err := syscall.Kill(-pgid, signal)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func waitForProcessGroupExit(pgid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		// Signal 0 does not kill anything; it only checks whether the group exists.
		err := syscall.Kill(-pgid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		if timeout <= 0 || time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}
