package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

var version = "dev"

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, stdout, stderr *os.File) int {
	var noColor bool
	args, noColor = stripGlobalFlags(args)
	prevNoColor := forceNoColor.Load()
	forceNoColor.Store(noColor)
	defer forceNoColor.Store(prevNoColor)

	if code, ok := maybeProxyToDaemon(args, stdout, stderr); ok {
		return code
	}

	if len(args) == 0 {
		cfg, _, err := loadConfig()
		if err == nil && cfg.Default != "" {
			return runTask([]string{cfg.Default}, stdout, stderr)
		}
		printUsage(stdout)
		return 0
	}

	if args[0] == "--version" {
		_, _ = stdout.WriteString("qp " + resolvedVersion() + "\n")
		return 0
	}

	switch args[0] {
	case "__complete":
		return runComplete(args[1:], stdout, stderr)
	case "__daemon":
		if len(args) > 1 && args[1] == "serve" {
			return runDaemonServe(stdout, stderr)
		}
		printError(stderr, fmt.Errorf("invalid daemon invocation"))
		return 1
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "completion":
		return runCompletion(args[1:], stdout, stderr)
	case "help":
		return runHelp(args[1:], stdout, stderr)
	case "agent-brief":
		return runAgentBrief(args[1:], stdout, stderr)
	case "docs":
		return runDocs(args[1:], stdout, stderr)
	case "explain":
		return runExplain(args[1:], stdout, stderr)
	case "guard":
		return runGuard(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "watch":
		return runWatch(args[1:], stdout, stderr)
	case "context":
		return runContext(args[1:], stdout, stderr)
	case "prompt":
		return runPrompt(args[1:], stdout, stderr)
	case "plan":
		return runPlan(args[1:], stdout, stderr)
	case "diff-plan":
		return runDiffPlan(args[1:], stdout, stderr)
	case "scope":
		return runScope(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "repair":
		return runRepair(args[1:], stdout, stderr)
	case "daemon":
		return runDaemon(args[1:], stdout, stderr)
	case "setup":
		return runSetup(args[1:], stdout, stderr)
	default:
		return runTask(args, stdout, stderr)
	}
}

func resolvedVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	var revision string
	var modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" {
		return version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified == "true" {
		return revision + "-dirty"
	}
	return revision
}
