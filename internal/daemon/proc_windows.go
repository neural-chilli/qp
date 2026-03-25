//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func startDetached(exePath string, args []string, logPath string) (int, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()

	cmd := exec.Command(exePath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func isProcessRunning(pid int) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), fmt.Sprintf(" %d ", pid)) || strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("%d", pid)), nil
}

func terminateProcess(pid int) error {
	cmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/T", "/F")
	return cmd.Run()
}
