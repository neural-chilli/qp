package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	qpdocs "github.com/neural-chilli/qp"
	"github.com/neural-chilli/qp/internal/config"
	contextpkg "github.com/neural-chilli/qp/internal/context"
	"github.com/neural-chilli/qp/internal/guard"
	"github.com/neural-chilli/qp/internal/initcmd"
	planpkg "github.com/neural-chilli/qp/internal/plan"
	"github.com/neural-chilli/qp/internal/prompt"
	"github.com/neural-chilli/qp/internal/runner"
	"github.com/neural-chilli/qp/internal/scope"
)

func runHelp(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	cfg, _, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	name := args[0]
	if resolved, ok := cfg.ResolveTaskName(name); ok {
		printTaskHelp(stdout, name, resolved, cfg, cfg.Tasks[resolved], aliasesForTask(cfg.Aliases, resolved), isDefaultTask(cfg, resolved))
		return 0
	}
	if group, ok := cfg.Groups[name]; ok {
		printGroupHelp(stdout, name, group, cfg)
		return 0
	}
	if guardCfg, ok := cfg.Guards[name]; ok {
		printGuardHelp(stdout, name, guardCfg)
		return 0
	}

	printError(stderr, unknownTaskError(name, cfg))
	return 1
}

func runDocs(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("docs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listOnly := fs.Bool("list", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--list": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	if *listOnly {
		for _, name := range qpdocs.DocNames() {
			fmt.Fprintln(stdout, name)
		}
		return 0
	}

	name := "readme"
	if fs.NArg() > 0 {
		name = fs.Arg(0)
	}

	doc, err := qpdocs.Doc(name)
	if err != nil {
		printError(stderr, err)
		return 1
	}

	fmt.Fprintln(stdout, strings.TrimRight(doc, "\n"))
	return 0
}

func runVersion(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *jsonOut {
		return printJSON(stdout, map[string]any{
			"name":    "qp",
			"version": resolvedVersion(),
		})
	}

	fmt.Fprintf(stdout, "qp %s\n", resolvedVersion())
	return 0
}

func runInit(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fromRepo := fs.Bool("from-repo", false, "")
	docs := fs.Bool("docs", false, "")
	harness := fs.Bool("harness", false, "")
	codemap := fs.Bool("codemap", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--from-repo": false, "--docs": false, "--harness": false, "--codemap": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	message, err := initcmd.Run(repoRoot, initcmd.Options{
		FromRepo: *fromRepo,
		Docs:     *docs,
		Harness:  *harness,
		Codemap:  *codemap,
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}

	fmt.Fprintln(stdout, message)
	return 0
}

func runGuard(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("guard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	noCache := fs.Bool("no-cache", false, "")
	verbose := fs.Bool("verbose", false, "")
	quiet := fs.Bool("quiet", false, "")
	allowUnsafe := fs.Bool("allow-unsafe", false, "")
	eventsOut := fs.Bool("events", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false, "--no-cache": false, "--verbose": false, "--quiet": false, "--allow-unsafe": false, "--events": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	var name string
	if fs.NArg() > 0 {
		name = fs.Arg(0)
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	taskRunner := runner.New(cfg, repoRoot)
	var events *runner.EventStream
	if *eventsOut {
		target := io.Writer(stderr)
		if *jsonOut {
			target = stdout
		}
		events = runner.NewEventStream(target)
		guardName := name
		if guardName == "" {
			guardName = "default"
		}
		nodes, edges := guardPlanGraph(cfg, guardName)
		events.EmitPlanGraph("guard:"+guardName, nodes, edges)
	}
	report, err := guard.New(cfg, repoRoot, taskRunner).Run(name, runner.Options{
		JSON:        *jsonOut,
		NoCache:     *noCache,
		Verbose:     *verbose,
		Quiet:       *quiet,
		AllowUnsafe: *allowUnsafe,
		Stdout:      stdout,
		Stderr:      stderr,
		Events:      events,
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if events != nil {
		events.EmitComplete(report.Overall, report.DurationMS)
	}

	if *jsonOut {
		if events != nil {
			return report.ExitCode
		}
		return printJSON(stdout, report)
	}

	if !*quiet {
		printGuardReport(stdout, report)
	}
	return report.ExitCode
}

func runScope(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("scope", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	format := fs.String("format", "", "")
	coverageOut := fs.Bool("coverage", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false, "--format": true, "--coverage": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}
	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if *coverageOut {
		if *format != "" {
			printError(stderr, fmt.Errorf("--format cannot be used with --coverage"))
			return 1
		}
		coverage, err := scope.ComputeCoverage(cfg, repoRoot)
		if err != nil {
			printError(stderr, err)
			return 1
		}
		if *jsonOut {
			return printJSON(stdout, coverage)
		}
		fmt.Fprintf(stdout, "Coverage: %d covered, %d orphaned\n", len(coverage.Covered), len(coverage.Orphaned))
		if len(coverage.Covered) > 0 {
			fmt.Fprintln(stdout, "\nCovered source dirs:")
			for _, dir := range coverage.Covered {
				fmt.Fprintf(stdout, "- %s\n", dir)
			}
		}
		if len(coverage.Orphaned) > 0 {
			fmt.Fprintln(stdout, "\nOrphaned source dirs:")
			for _, dir := range coverage.Orphaned {
				fmt.Fprintf(stdout, "- %s\n", dir)
			}
		}
		return 0
	}
	if fs.NArg() == 0 {
		printError(stderr, fmt.Errorf("scope name is required"))
		return 1
	}

	result, err := scope.Get(cfg, fs.Arg(0))
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *jsonOut {
		return printJSON(stdout, result)
	}

	if *format == "prompt" {
		fmt.Fprintln(stdout, scope.FormatPrompt(result.Scope, result.Desc, result.Paths))
		return 0
	}
	if *format != "" {
		printError(stderr, fmt.Errorf("unknown scope format %q", *format))
		return 1
	}

	for _, path := range result.Paths {
		fmt.Fprintln(stdout, path)
	}
	return 0
}

func runValidate(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	suggestOut := fs.Bool("suggest", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false, "--suggest": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		if *jsonOut {
			_ = printJSON(stdout, map[string]any{
				"valid": false,
				"error": err.Error(),
			})
		}
		return 1
	}

	result := map[string]any{
		"valid":     true,
		"project":   cfg.Project,
		"repo_root": repoRoot,
	}
	if *suggestOut {
		result["suggestions"] = config.Suggest(cfg)
	}
	if *jsonOut {
		return printJSON(stdout, result)
	}

	fmt.Fprintln(stdout, "qp.yaml is valid")
	if *suggestOut {
		suggestions := config.Suggest(cfg)
		if len(suggestions) == 0 {
			fmt.Fprintln(stdout, "No suggestions.")
		} else {
			fmt.Fprintln(stdout, "Suggestions:")
			for _, suggestion := range suggestions {
				fmt.Fprintf(stdout, "- %s\n", suggestion)
			}
		}
	}
	return 0
}

func runPrompt(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("prompt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	copyOut := fs.Bool("copy", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--copy": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		printError(stderr, fmt.Errorf("prompt name is required"))
		return 1
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	rendered, warnings, err := prompt.New(cfg, repoRoot).Render(fs.Arg(0))
	if err != nil {
		printError(stderr, err)
		return 1
	}
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}

	fmt.Fprintln(stdout, rendered)

	if *copyOut {
		if err := copyToClipboard(rendered); err != nil {
			printError(stderr, err)
			return 1
		}
	}

	return 0
}

func runPlan(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	var files multiFlag
	fs.Var(&files, "file", "")
	fs.Var(&files, "files", "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false, "--file": true, "--files": true})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	files = append(files, fs.Args()...)
	if len(files) == 0 {
		printError(stderr, fmt.Errorf("at least one file path is required"))
		return 1
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	result, err := planpkg.Generate(cfg, repoRoot, files)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if *jsonOut {
		return printJSON(stdout, result)
	}
	fmt.Fprintln(stdout, result.Markdown)
	return 0
}

func runDiffPlan(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("diff-plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	result, err := planpkg.GenerateFromGitDiff(cfg, repoRoot)
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if *jsonOut {
		return printJSON(stdout, result)
	}
	fmt.Fprintln(stdout, result.Markdown)
	return 0
}

func runContext(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(stderr)
	agent := fs.Bool("agent", false, "Generate agent-focused context")
	jsonOut := fs.Bool("json", false, "Emit structured JSON")
	taskName := fs.String("task", "", "Task name required with --agent")
	about := fs.String("about", "", "Generate topic-focused context using tasks, scopes, and codemap matches")
	outPath := fs.String("out", "", "Write rendered markdown to a file")
	copyOut := fs.Bool("copy", false, "Copy rendered markdown to the clipboard")
	maxTokens := fs.Int("max-tokens", 0, "Approximate token budget; uses a rough estimate rather than a model tokenizer")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--agent": false, "--json": false, "--task": true, "--about": true, "--out": true, "--copy": false, "--max-tokens": true})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	generator := contextpkg.New(cfg, repoRoot)
	options := contextpkg.Options{
		Agent:     *agent,
		Task:      *taskName,
		About:     *about,
		MaxTokens: *maxTokens,
	}
	if *jsonOut {
		result, err := generator.GenerateJSON(options)
		if err != nil {
			printError(stderr, err)
			return 1
		}
		return printJSON(stdout, result)
	}

	rendered, err := generator.Generate(options)
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(rendered+"\n"), 0o644); err != nil {
			printError(stderr, err)
			return 1
		}
	}

	fmt.Fprintln(stdout, rendered)

	if *copyOut {
		if err := copyToClipboard(rendered); err != nil {
			printError(stderr, err)
			return 1
		}
	}

	return 0
}
