package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/neural-chilli/qp/internal/daemon"
)

func runSetup(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	windows := fs.Bool("windows", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--windows": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	if runtime.GOOS != "windows" {
		if *windows {
			fmt.Fprintln(stdout, "qp setup --windows is only available on Windows; no action taken.")
			return 0
		}
		fmt.Fprintln(stdout, "No setup required on this platform. qp runs directly.")
		return 0
	}

	if !*windows {
		fmt.Fprintln(stdout, "Tip: run `qp setup --windows` to enable daemon mode.")
		return 0
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		printError(stderr, err)
		return 1
	}
	exePath, err := os.Executable()
	if err != nil {
		printError(stderr, err)
		return 1
	}
	manager := daemon.New(homeDir)
	status, err := manager.Start(exePath)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	profilePath, err := installWindowsPowerShellShim()
	if err != nil {
		printError(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "✓ Started qp daemon")
	fmt.Fprintf(stdout, "  pid: %d\n", status.PID)
	fmt.Fprintf(stdout, "  log: %s\n", status.LogPath)
	fmt.Fprintln(stdout, "✓ Added qp function to PowerShell profile")
	fmt.Fprintf(stdout, "  profile: %s\n", profilePath)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Restart your terminal to activate.")
	return 0
}

func runDaemon(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		printError(stderr, fmt.Errorf("daemon command is required: start|stop|status|restart"))
		return 1
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		printError(stderr, err)
		return 1
	}
	exePath, err := os.Executable()
	if err != nil {
		printError(stderr, err)
		return 1
	}
	manager := daemon.New(homeDir)

	switch args[0] {
	case "start":
		status, err := manager.Start(exePath)
		if err != nil {
			printError(stderr, err)
			return 1
		}
		if status.Running {
			fmt.Fprintf(stdout, "qp daemon running (pid %d)\n", status.PID)
			fmt.Fprintf(stdout, "log: %s\n", status.LogPath)
		}
		return 0
	case "stop":
		status, err := manager.Stop()
		if err != nil {
			printError(stderr, err)
			return 1
		}
		if status.PID == 0 {
			fmt.Fprintln(stdout, "qp daemon is not running")
			return 0
		}
		fmt.Fprintf(stdout, "qp daemon stopped (pid %d)\n", status.PID)
		return 0
	case "restart":
		status, err := manager.Restart(exePath)
		if err != nil {
			printError(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "qp daemon restarted (pid %d)\n", status.PID)
		fmt.Fprintf(stdout, "log: %s\n", status.LogPath)
		return 0
	case "status":
		status, err := manager.Status()
		if err != nil {
			printError(stderr, err)
			return 1
		}
		if !status.Running {
			fmt.Fprintln(stdout, "qp daemon is not running")
			fmt.Fprintf(stdout, "log: %s\n", status.LogPath)
			return 0
		}
		uptime := time.Since(status.StartedAt).Round(time.Second)
		fmt.Fprintf(stdout, "qp daemon is running (pid %d)\n", status.PID)
		fmt.Fprintf(stdout, "uptime: %s\n", uptime)
		fmt.Fprintf(stdout, "log: %s\n", status.LogPath)
		return 0
	default:
		printError(stderr, fmt.Errorf("unknown daemon command %q", args[0]))
		return 1
	}
}

func runDaemonServe(stdout, stderr *os.File) int {
	_, _ = stdout, stderr
	exePath, err := os.Executable()
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if err := daemon.Serve(exePath, stderr); err != nil {
		printError(stderr, err)
		return 1
	}
	return 0
}
