package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
	"github.com/neural-chilli/qp/internal/ordered"
)

func runCompletion(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		printError(stderr, fmt.Errorf("completion target is required"))
		return 1
	}

	switch normalizeShellName(args[0]) {
	case "install":
		return runCompletionInstall(args[1:], stdout, stderr)
	case "bash", "zsh", "fish", "powershell":
		script, err := completionScript(normalizeShellName(args[0]))
		if err != nil {
			printError(stderr, err)
			return 1
		}
		fmt.Fprint(stdout, script)
		return 0
	default:
		printError(stderr, fmt.Errorf("unknown completion target %q", args[0]))
		return 1
	}
}

func runComplete(args []string, stdout, stderr *os.File) int {
	// Completions should still work for top-level commands even when the repo
	// has no valid qp.yaml yet, so config load failures intentionally degrade
	// to a nil config instead of surfacing an error.
	cfg, _, err := loadConfig()
	if err != nil {
		cfg = nil
	}
	candidates := completionCandidates(args, cfg)
	for _, candidate := range candidates {
		fmt.Fprintln(stdout, candidate)
	}
	if err != nil && cfg == nil {
		_ = stderr
	}
	return 0
}

func runCompletionInstall(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("completion install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	shellFlag := fs.String("shell", "", "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--shell": true})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	shellName := normalizeShellName(*shellFlag)
	if shellName == "" {
		shellName = detectShell()
	}
	if shellName == "" {
		printError(stderr, fmt.Errorf("could not detect shell; use `qp completion install --shell <bash|zsh|fish|powershell>`"))
		return 1
	}

	installer, err := newCompletionInstaller(shellName)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	result, err := installer.Install()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	fmt.Fprintln(stdout, result.Message)
	for _, path := range result.Paths {
		fmt.Fprintf(stdout, "- %s\n", path)
	}
	if result.ReloadHint != "" {
		fmt.Fprintf(stdout, "\n%s\n", result.ReloadHint)
	}
	return 0
}

func completionCandidates(args []string, cfg any) []string {
	configValue, _ := cfgFromAny(cfg)
	current := ""
	if len(args) > 0 {
		current = args[len(args)-1]
	}

	if len(args) == 0 {
		return filterCompletionCandidates(topLevelCandidates(configValue), "")
	}

	if len(args) == 1 {
		return filterCompletionCandidates(topLevelCandidates(configValue), current)
	}

	command := args[0]
	rest := args[1:]
	current = rest[len(rest)-1]

	if isTaskLikeCompletion(command, configValue) {
		return filterCompletionCandidates(taskInvocationCandidates(command, rest, configValue), current)
	}

	switch command {
	case "help":
		return filterCompletionCandidates(helpCandidates(configValue), current)
	case "docs":
		return filterCompletionCandidates(append([]string{"--list"}, docCandidates()...), current)
	case "guard", "repair":
		return filterCompletionCandidates(guardCandidates(rest, configValue), current)
	case "scope":
		return filterCompletionCandidates(append([]string{"--json", "--format"}, scopeNames(configValue)...), current)
	case "prompt":
		return filterCompletionCandidates(append([]string{"--copy"}, promptNames(configValue)...), current)
	case "watch":
		return filterCompletionCandidates(watchCandidates(rest, configValue), current)
	case "init":
		return filterCompletionCandidates([]string{"--from-repo", "--docs"}, current)
	case "list":
		return filterCompletionCandidates([]string{"--json"}, current)
	case "validate", "version", "diff-plan":
		return filterCompletionCandidates([]string{"--json"}, current)
	case "context":
		return filterCompletionCandidates([]string{"--agent", "--json", "--task", "--about", "--out", "--copy", "--max-tokens"}, current)
	case "plan":
		return filterCompletionCandidates([]string{"--json", "--file", "--files"}, current)
	case "agent-brief":
		return filterCompletionCandidates([]string{"--task", "--diff", "--file", "--json", "--max-tokens"}, current)
	case "completion":
		return filterCompletionCandidates([]string{"bash", "zsh", "fish", "powershell", "install"}, current)
	case "daemon":
		return filterCompletionCandidates([]string{"start", "stop", "status", "restart"}, current)
	case "setup":
		return filterCompletionCandidates([]string{"--windows"}, current)
	default:
		return nil
	}
}

func topLevelCandidates(cfg *config.Config) []string {
	candidates := []string{
		"--no-color",
		"agent-brief",
		"completion",
		"context",
		"daemon",
		"diff-plan",
		"docs",
		"explain",
		"guard",
		"help",
		"init",
		"list",
		"plan",
		"prompt",
		"repair",
		"scope",
		"setup",
		"validate",
		"version",
		"watch",
	}
	if cfg == nil {
		return candidates
	}
	candidates = append(candidates, ordered.Keys(cfg.Tasks)...)
	for _, alias := range ordered.Keys(cfg.Aliases) {
		candidates = append(candidates, alias)
	}
	return uniqueStrings(candidates)
}

func helpCandidates(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	candidates := append([]string{}, ordered.Keys(cfg.Tasks)...)
	candidates = append(candidates, ordered.Keys(cfg.Aliases)...)
	groupNames := ordered.Keys(cfg.Groups)
	candidates = append(candidates, groupNames...)
	guardNames := ordered.Keys(cfg.Guards)
	candidates = append(candidates, guardNames...)
	return uniqueStrings(candidates)
}

func guardCandidates(args []string, cfg *config.Config) []string {
	candidates := []string{"--json", "--no-cache", "--allow-unsafe", "--events", "--no-color"}
	if cfg != nil {
		candidates = append(candidates, ordered.Keys(cfg.Guards)...)
	}
	return candidates
}

func watchCandidates(args []string, cfg *config.Config) []string {
	candidates := []string{"--path", "--allow-unsafe", "--events", "--no-color", "guard"}
	if cfg != nil {
		for _, name := range ordered.Keys(cfg.Tasks) {
			candidates = append(candidates, name)
		}
		for _, alias := range ordered.Keys(cfg.Aliases) {
			candidates = append(candidates, alias)
		}
		guards := make([]string, 0, len(cfg.Guards))
		for name := range cfg.Guards {
			guards = append(guards, "guard:"+name)
		}
		sort.Strings(guards)
		candidates = append(candidates, guards...)
	}
	return uniqueStrings(candidates)
}

func taskInvocationCandidates(name string, args []string, cfg *config.Config) []string {
	if cfg == nil {
		return []string{"--json", "--dry-run", "--no-cache", "--allow-unsafe", "--events", "--param", "--no-color"}
	}
	resolved, ok := cfg.ResolveTaskName(name)
	if !ok {
		return nil
	}
	task := cfg.Tasks[resolved]
	candidates := []string{"--json", "--dry-run", "--no-cache", "--allow-unsafe", "--events", "--param", "--no-color"}
	for _, paramName := range sortedParamNames(task.Params) {
		param := task.Params[paramName]
		if param.Position == 0 {
			candidates = append(candidates, "--"+paramName)
		}
	}
	return candidates
}

func scopeNames(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Scopes))
	for name := range cfg.Scopes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func promptNames(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Prompts))
	for name := range cfg.Prompts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func docCandidates() []string {
	return []string{"readme", "user-guide", "why-not-make", "releasing"}
}

func filterCompletionCandidates(candidates []string, prefix string) []string {
	filtered := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		if prefix != "" && !strings.HasPrefix(candidate, prefix) {
			continue
		}
		seen[candidate] = true
		filtered = append(filtered, candidate)
	}
	sort.Strings(filtered)
	return filtered
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeShellName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "install", "bash", "zsh", "fish":
		return value
	case "pwsh", "powershell", "ps":
		return "powershell"
	default:
		return value
	}
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "zsh"):
		return "zsh"
	case strings.Contains(shell, "bash"):
		return "bash"
	case strings.Contains(shell, "fish"):
		return "fish"
	}
	return ""
}

func isTaskLikeCompletion(name string, cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	_, ok := cfg.ResolveTaskName(name)
	return ok
}

func cfgFromAny(value any) (*config.Config, bool) {
	cfg, ok := value.(*config.Config)
	return cfg, ok
}
