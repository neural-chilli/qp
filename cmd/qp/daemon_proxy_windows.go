//go:build windows

package main

import (
	"fmt"
	"os"

	"github.com/neural-chilli/qp/internal/daemon"
)

func maybeProxyToDaemon(args []string, stdout, stderr *os.File) (int, bool) {
	if os.Getenv(daemon.BypassEnvVar) == "1" {
		return 0, false
	}
	if len(args) == 0 || isDaemonCommand(args[0]) {
		return 0, false
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, false
	}
	status, err := daemon.New(homeDir).Status()
	if err != nil || !status.Running {
		fmt.Fprintln(stderr, "Tip: run 'qp setup --windows' for faster execution (~2ms vs ~1000ms)")
		return 0, false
	}

	cwd, err := os.Getwd()
	if err != nil {
		return 0, false
	}
	code, err := daemon.Proxy(args, cwd, stdout, stderr)
	if err != nil {
		return 0, false
	}
	return code, true
}

func isDaemonCommand(name string) bool {
	switch name {
	case "daemon", "setup", "__daemon":
		return true
	default:
		return false
	}
}
