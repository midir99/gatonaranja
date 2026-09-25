//go:build unix

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestNewProcessGroupCommandConfiguresCancellation(t *testing.T) {
	cmd := commandContext(context.Background(), "echo")

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr = nil, want process group configuration")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("SysProcAttr.Setpgid = false, want true")
	}
	if cmd.Cancel == nil {
		t.Fatal("Cancel = nil, want process group cancellation")
	}
	if cmd.WaitDelay == 0 {
		t.Fatal("WaitDelay = 0, want bounded wait after cancellation")
	}
}

func TestNewProcessGroupCommandKillsChildProcesses(t *testing.T) {
	productionProcessGroupKillGrace := processGroupKillGrace
	processGroupKillGrace = 50 * time.Millisecond
	t.Cleanup(func() { processGroupKillGrace = productionProcessGroupKillGrace })

	ctx, cancel := context.WithCancel(context.Background())
	childPIDFile := filepath.Join(t.TempDir(), "child.pid")
	cmd := commandContext(ctx, os.Args[0], "-test.run=TestDownloadCommandHelperProcess", "--", "spawn-child")
	cmd.Env = append(os.Environ(), "GO_WANT_DOWNLOAD_COMMAND_HELPER_PROCESS=1", "CHILD_PID_FILE="+childPIDFile)

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Run()
	}()

	childPID := waitForDownloadCommandPIDFile(t, childPIDFile)
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("command error = nil, want cancellation error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("command did not finish after context cancellation")
	}

	waitForDownloadCommandProcessExit(t, childPID)
}

func TestDownloadCommandHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_DOWNLOAD_COMMAND_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	separatorIndex := -1
	for i, arg := range args {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}
	if separatorIndex == -1 || separatorIndex+1 >= len(args) {
		fmt.Fprint(os.Stderr, "missing helper arguments")
		os.Exit(2)
	}

	mode := args[separatorIndex+1]
	switch mode {
	case "spawn-child":
		childPIDFile := os.Getenv("CHILD_PID_FILE")
		if childPIDFile == "" {
			fmt.Fprint(os.Stderr, "missing CHILD_PID_FILE")
			os.Exit(2)
		}

		cmd := exec.Command(os.Args[0], "-test.run=TestDownloadCommandHelperProcess", "--", "block")
		cmd.Env = append(os.Environ(), "GO_WANT_DOWNLOAD_COMMAND_HELPER_PROCESS=1")
		if err := cmd.Start(); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(2)
		}
		if err := os.WriteFile(childPIDFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(2)
		}
		select {}
	case "block":
		select {}
	default:
		fmt.Fprint(os.Stderr, "unknown helper mode")
		os.Exit(2)
	}
}

func waitForDownloadCommandPIDFile(t *testing.T, pidFile string) int {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pidBytes, err := os.ReadFile(pidFile)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
			if err != nil {
				t.Fatalf("child pid file contains %q, want PID integer", string(pidBytes))
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for child pid file %q", pidFile)
	return 0
}

func waitForDownloadCommandProcessExit(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !downloadCommandProcessExists(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("process %d is still alive after command context cancellation", pid)
}

func downloadCommandProcessExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}
