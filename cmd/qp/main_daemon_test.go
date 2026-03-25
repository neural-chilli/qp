package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunSetupNoopOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows behavior only")
	}

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)
	code := run([]string{"setup"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(setup) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "No setup required on this platform") {
		t.Fatalf("stdout = %q, want non-windows noop message", got)
	}
}

func TestRunSetupWindowsFlagNoopOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows behavior only")
	}

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)
	code := run([]string{"setup", "--windows"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(setup --windows) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "only available on Windows") {
		t.Fatalf("stdout = %q, want windows-only message", got)
	}
}

func TestRunDaemonStatusWhenNotRunning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)
	code := run([]string{"daemon", "status"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(daemon status) code = %d, want 0; stderr=%s", code, readStderr())
	}
	out := readStdout()
	if !strings.Contains(out, "qp daemon is not running") {
		t.Fatalf("stdout = %q, want not-running status", out)
	}
	if !strings.Contains(out, filepath.Join(home, ".qp", "daemon", "daemon.log")) {
		t.Fatalf("stdout = %q, want daemon log path", out)
	}
}
