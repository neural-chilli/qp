package runner

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neural-chilli/qp/internal/config"
)

func TestRunCmdTaskUsesInvocationEnvOverride(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"env": {
			Desc: "env",
			Cmd:  "printf %s \"$FOO\"",
			Env:  map[string]string{"FOO": "task"},
		},
	})

	result, err := r.Run("env", Options{Stdout: io.Discard, Stderr: io.Discard, Env: map[string]string{"FOO": "override"}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "override" {
		t.Fatalf("Stdout = %q, want override", result.Stdout)
	}
}

func TestRunCmdTaskRunsDependenciesBeforeCommand(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"setup": {Desc: "setup", Cmd: `printf setup > ready.txt`},
		"build": {Desc: "build", Cmd: `cat ready.txt`, Needs: []string{"setup"}},
	}}, repoRoot)

	result, err := r.Run("build", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "setup" {
		t.Fatalf("Stdout = %q, want dependency output source", result.Stdout)
	}
	if len(result.Needs) != 1 || result.Needs[0].Task != "setup" || result.Needs[0].Status != StatusPass {
		t.Fatalf("Needs = %+v, want passing setup dependency", result.Needs)
	}
}

func TestRunCmdTaskSkipsCommandWhenDependencyFails(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	marker := filepath.Join(repoRoot, "main-ran.txt")
	r := New(&config.Config{Tasks: map[string]config.Task{
		"setup": {Desc: "setup", Cmd: "exit 1"},
		"build": {Desc: "build", Cmd: "printf ran > main-ran.txt", Needs: []string{"setup"}},
	}}, repoRoot)

	result, err := r.Run("build", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", result.Status)
	}
	if result.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", result.ExitCode)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("main command should not have run; stat err = %v", err)
	}
}

func TestRunTaskAllowsPipelineDependencies(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"test":  {Desc: "test", Cmd: `printf ok > ready.txt`},
		"check": {Desc: "check", Steps: []string{"test"}},
		"ship":  {Desc: "ship", Cmd: "cat ready.txt", Needs: []string{"check"}},
	}}, repoRoot)

	result, err := r.Run("ship", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "ok" {
		t.Fatalf("Stdout = %q, want pipeline dependency to run first", result.Stdout)
	}
	if len(result.Needs) != 1 || result.Needs[0].Task != "check" || result.Needs[0].Type != "pipeline" {
		t.Fatalf("Needs = %+v, want pipeline dependency result", result.Needs)
	}
}

func TestRunDryRunPrintsCommand(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"build": {Desc: "build", Cmd: "echo hi"},
	}}, repoRoot)

	outFile := mustTempFile(t)
	defer outFile.Close()

	result, err := r.Run("build", Options{DryRun: true, Stdout: outFile, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 || result.Status != StatusPass {
		t.Fatalf("result = %+v, want pass", result)
	}
	if got := readFile(t, outFile.Name()); got != "echo hi\n" {
		t.Fatalf("dry run output = %q, want command", got)
	}
}

func TestRunDryRunSilentTaskSuppressesCommand(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"build": {Desc: "build", Cmd: "echo hi", Silent: true},
	}}, repoRoot)

	outFile := mustTempFile(t)
	defer outFile.Close()

	result, err := r.Run("build", Options{DryRun: true, Stdout: outFile, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 || result.Status != StatusPass {
		t.Fatalf("result = %+v, want pass", result)
	}
	if got := readFile(t, outFile.Name()); got != "" {
		t.Fatalf("dry run output = %q, want empty for silent task", got)
	}
}

func TestSequentialPipelineStopsAfterFailure(t *testing.T) {
	repoRoot := t.TempDir()
	marker := filepath.Join(repoRoot, "ran-second.txt")
	r := New(&config.Config{Tasks: map[string]config.Task{
		"fail":   {Desc: "fail", Cmd: "exit 1"},
		"second": {Desc: "second", Cmd: "echo ran > ran-second.txt"},
		"pipe":   {Desc: "pipe", Steps: []string{"fail", "second"}},
	}}, repoRoot)

	result, err := r.Run("pipe", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", result.Status)
	}
	if result.Steps[1].Status != StatusCancelled {
		t.Fatalf("second step status = %q, want cancelled", result.Steps[1].Status)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("second step should not have run; stat err = %v", err)
	}
}

func TestSequentialPipelineContinueOnErrorRunsAllSteps(t *testing.T) {
	repoRoot := t.TempDir()
	marker := filepath.Join(repoRoot, "ran-second.txt")
	r := New(&config.Config{Tasks: map[string]config.Task{
		"fail":   {Desc: "fail", Cmd: "exit 1"},
		"second": {Desc: "second", Cmd: "echo ran > ran-second.txt"},
		"pipe":   {Desc: "pipe", Steps: []string{"fail", "second"}, ContinueOnError: true},
	}}, repoRoot)

	result, err := r.Run("pipe", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", result.Status)
	}
	if result.Steps[1].Status != StatusPass {
		t.Fatalf("second step status = %q, want pass", result.Steps[1].Status)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("second step should have run; stat err = %v", err)
	}
}

func TestRunMapsTimeoutToTimeoutStatus(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"slow": {Desc: "slow", Cmd: "sleep 1", Timeout: "10ms"},
	})

	result, err := r.Run("slow", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusTimeout {
		t.Fatalf("Status = %q, want timeout", result.Status)
	}
	if result.ExitCode != 124 {
		t.Fatalf("ExitCode = %d, want 124", result.ExitCode)
	}
}

func TestParallelPipelineCancelsOtherStepOnFailure(t *testing.T) {
	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"fail": {Desc: "fail", Cmd: "exit 1"},
		"slow": {Desc: "slow", Cmd: "sleep 1; echo done > slow.txt"},
		"par":  {Desc: "par", Steps: []string{"fail", "slow"}, Parallel: true},
	}}, repoRoot)

	result, err := r.Run("par", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", result.Status)
	}
	if result.Steps[1].Status != StatusCancelled && result.Steps[1].Status != StatusFail && result.Steps[1].Status != StatusPass {
		t.Fatalf("slow step status = %q, want cancelled, fail, or pass", result.Steps[1].Status)
	}
}

func TestParallelPipelineContinueOnErrorIncludesStructuredWarning(t *testing.T) {
	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"fail": {Desc: "fail", Cmd: "exit 1"},
		"slow": {Desc: "slow", Cmd: "sleep 1; echo done > slow.txt"},
		"par":  {Desc: "par", Steps: []string{"fail", "slow"}, Parallel: true, ContinueOnError: true},
	}}, repoRoot)

	result, err := r.Run("par", Options{Stdout: io.Discard, Stderr: io.Discard, JSON: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stderr == "" || !strings.Contains(result.Stderr, "continue_on_error is ignored") {
		t.Fatalf("Stderr = %q, want continue_on_error warning", result.Stderr)
	}
}

func TestRunCmdTaskInjectsParamsIntoEnvAndTemplate(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"add-feature": {
			Desc: "add",
			Cmd:  `printf "%s|%s" "{{params.feature}}" "$FEATURE"`,
			Params: map[string]config.Param{
				"feature": {
					Env:      "FEATURE",
					Required: true,
				},
			},
		},
	})

	result, err := r.Run("add-feature", Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Params: map[string]string{"feature": "auth"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "auth|auth" {
		t.Fatalf("Stdout = %q, want interpolated/env param", result.Stdout)
	}
}

func TestRunCmdTaskRejectsMissingRequiredParam(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"add-feature": {
			Desc: "add",
			Cmd:  "echo hi",
			Params: map[string]config.Param{
				"feature": {
					Env:      "FEATURE",
					Required: true,
				},
			},
		},
	})

	_, err := r.Run("add-feature", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err == nil {
		t.Fatal("Run() error = nil, want missing param error")
	}
	if err.Error() != `task "add-feature": missing required param "feature"` {
		t.Fatalf("Run() error = %v, want missing param message", err)
	}
}

func TestRunCmdTaskUsesCustomShell(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"echo": {
			Desc:      "echo",
			Cmd:       "printf shell-ok",
			Shell:     "/bin/sh",
			ShellArgs: []string{"-c"},
		},
	})

	result, err := r.Run("echo", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "shell-ok" {
		t.Fatalf("Stdout = %q, want custom shell output", result.Stdout)
	}
}

func TestRunCmdTaskUsesCustomShellArgs(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"strict": {
			Desc:      "strict",
			Cmd:       "printf strict-ok",
			Shell:     "/bin/sh",
			ShellArgs: []string{"-eu", "-c"},
		},
	})

	result, err := r.Run("strict", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "strict-ok" {
		t.Fatalf("Stdout = %q, want custom shell arg output", result.Stdout)
	}
}

func TestRunCmdTaskUsesDefaultWorkingDir(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	workdir := filepath.Join(repoRoot, "app")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := New(&config.Config{
		Defaults: config.DefaultsConfig{Dir: "app"},
		Tasks: map[string]config.Task{
			"pwd": {Desc: "pwd", Cmd: "pwd"},
		},
	}, repoRoot)

	result, err := r.Run("pwd", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := strings.TrimSpace(result.Stdout)
	if resolved, err := filepath.EvalSymlinks(workdir); err == nil {
		workdir = resolved
	}
	if got != workdir {
		t.Fatalf("Stdout = %q, want default workdir %q", got, workdir)
	}
}

func TestRunTaskExpressionSequenceExecutesInOrder(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"setup": {Desc: "setup", Cmd: `printf setup > state.txt`},
		"test":  {Desc: "test", Cmd: "cat state.txt"},
		"check": {Desc: "check", Run: "setup -> test"},
	}}, repoRoot)

	result, err := r.Run("check", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
	if len(result.Steps) != 2 || result.Steps[0].Name != "setup" || result.Steps[1].Name != "test" {
		t.Fatalf("Steps = %+v, want setup then test", result.Steps)
	}
}

func TestRunTaskExpressionParallelBranches(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"lint":  {Desc: "lint", Cmd: "printf lint"},
		"test":  {Desc: "test", Cmd: "printf test"},
		"check": {Desc: "check", Run: "par(lint, test)"},
	}}, repoRoot)

	result, err := r.Run("check", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("Steps = %+v, want 2 parallel branches", result.Steps)
	}
}

func TestRunTaskSkipsWhenConditionFalse(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"deploy": {
			Desc: "deploy",
			Cmd:  "echo deploy",
			When: `env("QP_RUN_DEPLOY") == "1"`,
		},
	})

	result, err := r.Run("deploy", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusSkipped {
		t.Fatalf("Status = %q, want skipped", result.Status)
	}
	if result.SkipReason == "" {
		t.Fatal("SkipReason = empty, want reason")
	}
}

func TestRunTaskExpressionWhenChoosesFalseBranch(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"ship":   {Desc: "ship", Cmd: `printf ship`},
		"notify": {Desc: "notify", Cmd: `printf notify`},
		"flow":   {Desc: "flow", Run: `when(env("QP_CAN_SHIP") == "1", ship, notify)`},
	}}, repoRoot)

	result, err := r.Run("flow", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
	if len(result.Steps) == 0 {
		t.Fatalf("Steps = %+v, want executed branch", result.Steps)
	}
	if result.Steps[0].Name != "when:false" {
		t.Fatalf("first step = %+v, want when:false", result.Steps[0])
	}
}

func TestRunTaskExpressionSwitchChoosesMatchingCase(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"build-api": {Desc: "build-api", Cmd: `printf api`},
		"build-web": {Desc: "build-web", Cmd: `printf web`},
		"flow":      {Desc: "flow", Run: `switch(param("target"), "api": build-api, "web": build-web)`},
	}}, repoRoot)

	result, err := r.Run("flow", Options{Stdout: io.Discard, Stderr: io.Discard, Params: map[string]string{"target": "web"}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
	if len(result.Steps) == 0 || result.Steps[0].Name != "switch:web" {
		t.Fatalf("Steps = %+v, want switch:web", result.Steps)
	}
}

func TestRunTaskExpressionSwitchSkipsWhenNoCaseMatches(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"build-api": {Desc: "build-api", Cmd: `printf api`},
		"flow":      {Desc: "flow", Run: `switch(param("target"), "api": build-api)`},
	}}, repoRoot)

	result, err := r.Run("flow", Options{Stdout: io.Discard, Stderr: io.Discard, Params: map[string]string{"target": "web"}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusSkipped {
		t.Fatalf("Status = %q, want skipped", result.Status)
	}
	if len(result.Steps) == 0 || result.Steps[0].Name != "switch" {
		t.Fatalf("Steps = %+v, want switch skip step", result.Steps)
	}
}

func TestRunCmdTaskInterpolatesVarsAndTemplates(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Vars: map[string]string{
			"name": "world",
		},
		Templates: config.Templates{
			Snippets: map[string]string{
				"greeting": "hello {{vars.name}}",
			},
		},
		Tasks: map[string]config.Task{
			"greet": {
				Desc: "greet",
				Cmd:  `printf "{{template.greeting}}"`,
			},
		},
	}, repoRoot)

	result, err := r.Run("greet", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "hello world" {
		t.Fatalf("Stdout = %q, want hello world", result.Stdout)
	}
}

func TestRunCmdTaskInterpolatesDeepTemplateChains(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Vars: map[string]string{
			"name": "world",
		},
		Templates: config.Templates{
			Snippets: map[string]string{
				"a": "{{template.b}}",
				"b": "{{template.c}}",
				"c": "{{template.d}}",
				"d": "{{template.e}}",
				"e": "hello {{vars.name}}",
			},
		},
		Tasks: map[string]config.Task{
			"greet": {
				Desc: "greet",
				Cmd:  `printf "{{template.a}}"`,
			},
		},
	}, repoRoot)

	result, err := r.Run("greet", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "hello world" {
		t.Fatalf("Stdout = %q, want hello world", result.Stdout)
	}
}

func TestRunCmdTaskSilentOmitsResolvedCmd(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Tasks: map[string]config.Task{
			"quiet": {
				Desc:   "quiet",
				Cmd:    `printf hello`,
				Silent: true,
			},
		},
	}, repoRoot)

	result, err := r.Run("quiet", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ResolvedCmd != nil {
		t.Fatalf("ResolvedCmd = %q, want nil for silent task", *result.ResolvedCmd)
	}
}

func TestRunCmdTaskRunsDeferOnSuccess(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Tasks: map[string]config.Task{
			"integration": {
				Desc:  "integration",
				Cmd:   `printf ok`,
				Defer: `printf done > cleanup.txt`,
			},
		},
	}, repoRoot)

	result, err := r.Run("integration", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
	if got, err := os.ReadFile(filepath.Join(repoRoot, "cleanup.txt")); err != nil || string(got) != "done" {
		t.Fatalf("cleanup marker = %q, err = %v, want done marker", string(got), err)
	}
}

func TestRunCmdTaskRunsDeferOnFailure(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Tasks: map[string]config.Task{
			"integration": {
				Desc:  "integration",
				Cmd:   `exit 1`,
				Defer: `printf done > cleanup.txt`,
			},
		},
	}, repoRoot)

	result, err := r.Run("integration", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", result.Status)
	}
	if got, err := os.ReadFile(filepath.Join(repoRoot, "cleanup.txt")); err != nil || string(got) != "done" {
		t.Fatalf("cleanup marker = %q, err = %v, want done marker", string(got), err)
	}
}

func TestRunCmdTaskDeferFailureDoesNotOverrideMainStatus(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Tasks: map[string]config.Task{
			"integration": {
				Desc:  "integration",
				Cmd:   `printf ok`,
				Defer: `exit 2`,
			},
		},
	}, repoRoot)

	result, err := r.Run("integration", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want main command status pass", result.Status)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want main command exit code 0", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "defer command failed") {
		t.Fatalf("Stderr = %q, want defer failure log", result.Stderr)
	}
}

func TestRunCmdTaskUsesCacheWhenEnabled(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"cache-test": {
			Desc:  "cache test",
			Cmd:   `if [ -f marker.txt ]; then printf second; else printf first; touch marker.txt; fi`,
			Cache: &config.TaskCache{Enabled: true},
		},
	}}, repoRoot)

	first, err := r.Run("cache-test", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if first.Stdout != "first" {
		t.Fatalf("first stdout = %q, want first", first.Stdout)
	}

	second, err := r.Run("cache-test", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if second.Stdout != "first" {
		t.Fatalf("second stdout = %q, want cached first", second.Stdout)
	}
	if !second.Cached {
		t.Fatal("second result not marked cached")
	}
}

func TestRunCmdTaskNoCacheBypassesCache(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"cache-test": {
			Desc:  "cache test",
			Cmd:   `if [ -f marker.txt ]; then printf second; else printf first; touch marker.txt; fi`,
			Cache: &config.TaskCache{Enabled: true},
		},
	}}, repoRoot)

	if _, err := r.Run("cache-test", Options{Stdout: io.Discard, Stderr: io.Discard}); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	second, err := r.Run("cache-test", Options{Stdout: io.Discard, Stderr: io.Discard, NoCache: true})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if second.Stdout != "second" {
		t.Fatalf("second stdout = %q, want second", second.Stdout)
	}
	if second.Cached {
		t.Fatal("second result unexpectedly marked cached")
	}
}

func TestRunCmdTaskCachePathsInvalidatesOnFileChange(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	inputFile := filepath.Join(repoRoot, "input.txt")
	if err := os.WriteFile(inputFile, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New(&config.Config{Tasks: map[string]config.Task{
		"cache-paths": {
			Desc:  "cache paths",
			Cmd:   `cat input.txt`,
			Cache: &config.TaskCache{Enabled: true, Paths: []string{"input.txt"}},
		},
	}}, repoRoot)

	first, err := r.Run("cache-paths", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if first.Stdout != "first" {
		t.Fatalf("first stdout = %q, want first", first.Stdout)
	}

	if err := os.WriteFile(inputFile, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := r.Run("cache-paths", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if second.Cached {
		t.Fatal("second result unexpectedly marked cached")
	}
	if second.Stdout != "second" {
		t.Fatalf("second stdout = %q, want second", second.Stdout)
	}
}

func TestRunCmdTaskCacheBypassesWhenDependencyRunsFresh(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	depInput := filepath.Join(repoRoot, "dep.txt")
	if err := os.WriteFile(depInput, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New(&config.Config{Tasks: map[string]config.Task{
		"dep": {
			Desc:  "dep",
			Cmd:   `cat dep.txt`,
			Cache: &config.TaskCache{Enabled: true, Paths: []string{"dep.txt"}},
		},
		"down": {
			Desc:  "down",
			Cmd:   `if [ -f down-count.txt ]; then c=$(cat down-count.txt); else c=0; fi; c=$((c+1)); printf %s "$c" > down-count.txt; printf %s "$c"`,
			Needs: []string{"dep"},
			Cache: &config.TaskCache{Enabled: true},
		},
	}}, repoRoot)

	first, err := r.Run("down", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if first.Stdout != "1" {
		t.Fatalf("first stdout = %q, want 1", first.Stdout)
	}
	if first.Cached {
		t.Fatal("first result unexpectedly cached")
	}

	second, err := r.Run("down", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if second.Stdout != "1" {
		t.Fatalf("second stdout = %q, want cached 1", second.Stdout)
	}
	if !second.Cached {
		t.Fatal("second result not marked cached")
	}

	if err := os.WriteFile(depInput, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := r.Run("down", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("third Run() error = %v", err)
	}
	if third.Cached {
		t.Fatal("third result unexpectedly cached")
	}
	if third.Stdout != "2" {
		t.Fatalf("third stdout = %q, want rerun value 2", third.Stdout)
	}
}

func TestRunCmdTaskRetriesUntilSuccess(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"flaky": {
			Desc:       "flaky",
			Cmd:        `if [ -f retry-count.txt ]; then c=$(cat retry-count.txt); else c=0; fi; c=$((c+1)); printf %s "$c" > retry-count.txt; if [ "$c" -lt 3 ]; then echo fail >&2; exit 1; fi; printf ok`,
			Retry:      3,
			RetryDelay: "1ms",
		},
	}}, repoRoot)

	result, err := r.Run("flaky", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
	if result.Stdout != "ok" {
		t.Fatalf("Stdout = %q, want ok", result.Stdout)
	}
}

func TestRunCmdTaskRetryOnExitCodeCondition(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"flaky": {
			Desc:    "flaky",
			Cmd:     `if [ -f retry-count.txt ]; then c=$(cat retry-count.txt); else c=0; fi; c=$((c+1)); printf %s "$c" > retry-count.txt; exit 1`,
			Retry:   2,
			RetryOn: []string{"exit_code:2"},
		},
	}}, repoRoot)

	result, err := r.Run("flaky", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", result.ExitCode)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, "retry-count.txt"))
	if err != nil {
		t.Fatalf("ReadFile(retry-count.txt) error = %v", err)
	}
	if string(raw) != "1" {
		t.Fatalf("retry-count = %q, want no retry", string(raw))
	}
}

func TestRunCmdTaskInterpolatesAndRedactsSecrets(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "super-secret-token")
	if err := os.WriteFile(filepath.Join(repoRoot, "qp.yaml"), []byte(`
secrets:
  openai_key:
    from: env
    env: OPENAI_API_KEY
tasks:
  reveal:
    desc: reveal
    cmd: printf "{{secret.openai_key}}"; printf "{{secret.openai_key}}" >&2
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(filepath.Join(repoRoot, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	r := New(cfg, repoRoot)
	var eventOut bytes.Buffer
	result, err := r.Run("reveal", Options{Stdout: io.Discard, Stderr: io.Discard, Events: NewEventStream(&eventOut)})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(result.Stdout, "super-secret-token") || strings.Contains(result.Stderr, "super-secret-token") {
		t.Fatalf("result output leaked secret: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "***") || !strings.Contains(result.Stderr, "***") {
		t.Fatalf("redacted output missing mask: stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if strings.Contains(eventOut.String(), "super-secret-token") {
		t.Fatalf("events leaked secret: %q", eventOut.String())
	}
}

func TestRunCmdTaskDoesNotRedactShortSecretValues(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("SHORT_SECRET", "token")
	if err := os.WriteFile(filepath.Join(repoRoot, "qp.yaml"), []byte(`
secrets:
  api_key:
    from: env
    env: SHORT_SECRET
tasks:
  reveal:
    desc: reveal
    cmd: printf "{{secret.api_key}}"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(filepath.Join(repoRoot, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	r := New(cfg, repoRoot)
	result, err := r.Run("reveal", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "token" {
		t.Fatalf("Stdout = %q, want short secret to remain visible", result.Stdout)
	}
}

func TestRunTaskWhenCanUseVars(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Vars: map[string]string{
			"deploy_region": "eu-west-1",
		},
		Tasks: map[string]config.Task{
			"deploy": {
				Desc: "deploy",
				Cmd:  "echo deploy",
				When: `vars.deploy_region == "us-east-1"`,
			},
		},
	}, repoRoot)

	result, err := r.Run("deploy", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusSkipped {
		t.Fatalf("Status = %q, want skipped", result.Status)
	}
}

func TestRunTaskWhenCanUseOSVar(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Tasks: map[string]config.Task{
			"platform": {
				Desc: "platform",
				Cmd:  "echo platform",
				When: `os == "` + runtime.GOOS + `"`,
			},
		},
	}, repoRoot)

	result, err := r.Run("platform", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
}

func TestRunTaskWhenCanUseProfileFunction(t *testing.T) {
	repoRoot := t.TempDir()
	cfg := &config.Config{
		Profiles: config.Profiles{
			Entries: map[string]config.Profile{
				"prod": {},
			},
		},
		Tasks: map[string]config.Task{
			"deploy": {
				Desc: "deploy",
				Cmd:  "echo deploy",
				When: `profile() == "prod"`,
			},
		},
	}
	if err := cfg.ApplyProfiles([]string{"prod"}); err != nil {
		t.Fatalf("ApplyProfiles() error = %v", err)
	}
	r := New(cfg, repoRoot)

	result, err := r.Run("deploy", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
}

func TestRunTaskWhenCanUseParamFunctions(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{
		Tasks: map[string]config.Task{
			"deploy": {
				Desc: "deploy",
				Cmd:  "echo deploy",
				When: `has_param("region") && param("region") == "eu-west-1"`,
			},
		},
	}, repoRoot)

	result, err := r.Run("deploy", Options{Stdout: io.Discard, Stderr: io.Discard, Params: map[string]string{"region": "eu-west-1"}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
}

func TestRunCmdTaskDirOverridesDefaultWorkingDir(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	defaultDir := filepath.Join(repoRoot, "app")
	overrideDir := filepath.Join(repoRoot, "tools")
	for _, dir := range []string{defaultDir, overrideDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := New(&config.Config{
		Defaults: config.DefaultsConfig{Dir: "app"},
		Tasks: map[string]config.Task{
			"pwd": {Desc: "pwd", Cmd: "pwd", Dir: "tools"},
		},
	}, repoRoot)

	result, err := r.Run("pwd", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := strings.TrimSpace(result.Stdout)
	if resolved, err := filepath.EvalSymlinks(overrideDir); err == nil {
		overrideDir = resolved
	}
	if got != overrideDir {
		t.Fatalf("Stdout = %q, want override workdir %q", got, overrideDir)
	}
}

func TestPrefixedWriterPrefixesFirstLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writer := prefixedWriter("step-1", &buf)
	if _, err := writer.Write([]byte("hello\nworld\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := buf.String(); got != "[step-1] hello\n[step-1] world\n" {
		t.Fatalf("output = %q, want prefixed lines", got)
	}
}

func TestRunCmdTaskExtractsGoTestErrors(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"test": {
			Desc:        "test",
			Cmd:         `printf '%s\n' 'internal/runner/runner_test.go:47: got pass, want fail' >&2; exit 1`,
			ErrorFormat: "go_test",
		},
	})

	result, err := r.Run("test", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
	}
	if got := result.Errors[0]; got.File != "internal/runner/runner_test.go" || got.Line != 47 || got.Message != "got pass, want fail" || got.Severity != "error" {
		t.Fatalf("Errors[0] = %+v, want parsed go test error", got)
	}
}

func TestRunCmdTaskExtractsGenericErrors(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"lint": {
			Desc:        "lint",
			Cmd:         `printf '%s\n' 'src/app.ts:12:7: missing semicolon' >&2; exit 1`,
			ErrorFormat: "generic",
		},
	})

	result, err := r.Run("lint", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
	}
	if got := result.Errors[0]; got.File != "src/app.ts" || got.Line != 12 || got.Column != 7 || got.Message != "missing semicolon" || got.Severity != "error" {
		t.Fatalf("Errors[0] = %+v, want parsed generic error", got)
	}
}

func TestRunCmdTaskFallsBackToGenericErrorParsing(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"lint": {
			Desc:        "lint",
			Cmd:         `printf '%s\n' 'src/app.ts:12:7: missing semicolon' >&2; exit 1`,
			ErrorFormat: "go_test",
		},
	})

	result, err := r.Run("lint", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
	}
	if got := result.Errors[0]; got.File != "src/app.ts" || got.Line != 12 || got.Column != 7 || got.Message != "missing semicolon" || got.Severity != "error" {
		t.Fatalf("Errors[0] = %+v, want parsed generic fallback error", got)
	}
}

func TestRunPipelineStepIncludesStructuredErrors(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"test": {
			Desc:        "test",
			Cmd:         `printf '%s\n' 'pkg/mod.py:9: assertion failed' >&2; exit 1`,
			ErrorFormat: "pytest",
		},
		"check": {
			Desc:  "check",
			Steps: []string{"test"},
		},
	})

	result, err := r.Run("check", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(result.Steps))
	}
	if len(result.Steps[0].Errors) != 1 {
		t.Fatalf("len(Steps[0].Errors) = %d, want 1", len(result.Steps[0].Errors))
	}
	if got := result.Steps[0].Errors[0]; got.File != "pkg/mod.py" || got.Line != 9 || got.Message != "assertion failed" {
		t.Fatalf("Steps[0].Errors[0] = %+v, want parsed pytest error", got)
	}
}

func newTestRunner(t *testing.T, tasks map[string]config.Task) *Runner {
	t.Helper()
	return New(&config.Config{Tasks: tasks}, t.TempDir())
}

func mustTempFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "runner-out-*")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
