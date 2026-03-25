//go:build !windows

package daemon

import (
	"errors"
	"os"
	"os/exec"
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
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func isProcessRunning(pid int) (bool, error) {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EPERM) {
		return true, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	return false, err
}

func terminateProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}
