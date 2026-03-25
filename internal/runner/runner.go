package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	celpkg "github.com/neural-chilli/qp/internal/cel"
	"github.com/neural-chilli/qp/internal/config"
)

const (
	StatusPass      = "pass"
	StatusFail      = "fail"
	StatusSkipped   = "skipped"
	StatusCancelled = "cancelled"
	StatusTimeout   = "timeout"
)

type Options struct {
	JSON        bool
	DryRun      bool
	Verbose     bool
	Quiet       bool
	NoCache     bool
	AllowUnsafe bool
	Stdout      io.Writer
	Stderr      io.Writer
	Env         map[string]string
	Params      map[string]string
	Events      *EventStream
}

type Runner struct {
	cfg          *config.Config
	repoRoot     string
	globalEnv    map[string]string
	envFileCount int
	envFileFound bool
	branch       string
	tag          string
	secrets      map[string]string
	celEngine    *celpkg.Engine
}

type Result struct {
	Task        string       `json:"task"`
	Type        string       `json:"type"`
	Needs       []Result     `json:"needs,omitempty"`
	ResolvedCmd *string      `json:"resolved_cmd,omitempty"`
	Parallel    bool         `json:"parallel,omitempty"`
	Status      string       `json:"status"`
	ExitCode    int          `json:"exit_code"`
	Stdout      string       `json:"stdout,omitempty"`
	Stderr      string       `json:"stderr,omitempty"`
	Errors      []ErrorEntry `json:"errors,omitempty"`
	SkipReason  string       `json:"skip_reason,omitempty"`
	Cached      bool         `json:"cached,omitempty"`
	DurationMS  int64        `json:"duration_ms"`
	StartedAt   string       `json:"started_at"`
	FinishedAt  string       `json:"finished_at"`
	Steps       []StepResult `json:"steps,omitempty"`
}

type StepResult struct {
	Index       int          `json:"index"`
	Name        string       `json:"name"`
	Type        string       `json:"type,omitempty"`
	ResolvedCmd *string      `json:"resolved_cmd"`
	Parallel    bool         `json:"parallel,omitempty"`
	Status      string       `json:"status"`
	ExitCode    int          `json:"exit_code"`
	Stdout      *string      `json:"stdout"`
	Stderr      *string      `json:"stderr"`
	Errors      []ErrorEntry `json:"errors,omitempty"`
	SkipReason  string       `json:"skip_reason,omitempty"`
	DurationMS  *int64       `json:"duration_ms"`
	StartedAt   *string      `json:"started_at"`
	FinishedAt  *string      `json:"finished_at"`
	Steps       []StepResult `json:"steps,omitempty"`
}

type ErrorEntry struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

type runOutcome struct {
	status   string
	exitCode int
	stdout   string
	stderr   string
	started  time.Time
	finished time.Time
}

func New(cfg *config.Config, repoRoot string) *Runner {
	globalEnv, envCount, envFound := loadEnvFile(filepath.Join(repoRoot, cfg.EnvFile))
	return &Runner{
		cfg:          cfg,
		repoRoot:     repoRoot,
		globalEnv:    globalEnv,
		envFileCount: envCount,
		envFileFound: envFound,
		branch:       detectGitBranch(repoRoot),
		tag:          detectGitTag(repoRoot),
		secrets:      cfg.SecretValues(),
		celEngine:    celpkg.New(),
	}
}

func (r *Runner) Run(taskName string, opts Options) (Result, error) {
	if opts.Events != nil && opts.Stderr != nil && r.cfg.EnvFile != "" && r.envFileFound {
		_, _ = fmt.Fprintf(opts.Stderr, "[qp] loaded %d vars from %s\n", r.envFileCount, r.cfg.EnvFile)
	}
	return r.runTask(context.Background(), taskName, opts)
}

func (r *Runner) runTask(ctx context.Context, taskName string, opts Options) (Result, error) {
	task, ok := r.cfg.Tasks[taskName]
	if !ok {
		return Result{}, fmt.Errorf("unknown task %q", taskName)
	}
	if opts.Events != nil {
		opts.Events.EmitStart(taskName)
	}
	if err := requireSafetyApproval(taskName, task.SafetyLevel(), opts); err != nil {
		return Result{}, err
	}

	started := time.Now()
	if task.When != "" {
		ok, err := r.celEngine.EvalBool(task.When, r.celVars(opts))
		if err != nil {
			return Result{}, fmt.Errorf("task %q: when evaluation failed: %w", taskName, err)
		}
		if !ok {
			finished := time.Now()
			result := Result{
				Task:       taskName,
				Type:       task.Type(),
				Status:     StatusSkipped,
				ExitCode:   0,
				SkipReason: fmt.Sprintf("when condition is false: %s", task.When),
				DurationMS: finished.Sub(started).Milliseconds(),
				StartedAt:  started.UTC().Format(time.RFC3339),
				FinishedAt: finished.UTC().Format(time.RFC3339),
			}
			if opts.Events != nil {
				opts.Events.EmitSkipped(taskName, result.SkipReason)
			}
			return result, nil
		}
	}

	needs, depFailure, err := r.runNeeds(ctx, task, opts)
	if err != nil {
		return Result{}, err
	}
	if depFailure != nil {
		finished := time.Now()
		result := Result{
			Task:       taskName,
			Type:       task.Type(),
			Needs:      needs,
			Status:     depFailure.Status,
			ExitCode:   depFailure.ExitCode,
			Errors:     collectResultErrors(*depFailure),
			DurationMS: finished.Sub(started).Milliseconds(),
			StartedAt:  started.UTC().Format(time.RFC3339),
			FinishedAt: finished.UTC().Format(time.RFC3339),
		}
		if opts.Events != nil {
			opts.Events.EmitSkipped(taskName, "dependency failed")
		}
		return result, nil
	}

	if task.Cmd != "" {
		stepName := taskName
		paramValues, err := resolveParamValues(task, opts.Params)
		if err != nil {
			return Result{}, fmt.Errorf("task %q: %w", taskName, err)
		}
		resolved := interpolateTaskValue(task.Cmd, paramValues, map[string]string(r.cfg.Vars), r.cfg.Templates.Snippets, r.secrets)
		if task.CacheEnabled() && !opts.NoCache && !opts.DryRun {
			contentHash := ""
			if paths := task.CachePaths(); len(paths) > 0 {
				hash, err := hashCachePaths(r.repoRoot, paths)
				if err != nil {
					return Result{}, fmt.Errorf("task %q: cache path hashing failed: %w", taskName, err)
				}
				contentHash = hash
			}
			cacheKey := makeCacheKey(cacheKeyInput{
				TaskName:    taskName,
				Task:        task,
				ResolvedCmd: resolved,
				Params:      paramValues,
				Env:         interpolateEnv(task.Env, paramValues, map[string]string(r.cfg.Vars), r.cfg.Templates.Snippets, r.secrets),
				WorkDir:     r.resolveTaskDir(task),
				Profile:     strings.Join(r.cfg.ActiveProfiles(), ","),
				ExtraEnv:    opts.Env,
				ContentHash: contentHash,
			})
			if !hasFreshDependency(needs) {
				if cached, ok := readCachedResult(r.repoRoot, cacheKey); ok {
					cached.Cached = true
					if cached.Stdout != "" && opts.Stdout != nil {
						_, _ = io.WriteString(opts.Stdout, cached.Stdout)
					}
					if cached.Stderr != "" && opts.Stderr != nil {
						_, _ = io.WriteString(opts.Stderr, cached.Stderr)
					}
					if opts.Events != nil {
						opts.Events.EmitSkipped(taskName, "cache hit")
					}
					return cached, nil
				}
			}
			result, err := r.executeCommandTask(ctx, taskName, stepName, task, resolved, paramValues, needs, opts)
			if err != nil {
				return Result{}, err
			}
			if result.Status == StatusPass {
				writeCachedResult(r.repoRoot, cacheKey, result)
			}
			return result, nil
		}
		return r.executeCommandTask(ctx, taskName, stepName, task, resolved, paramValues, needs, opts)
	}
	if task.Run != "" {
		result, err := r.runFromExpression(ctx, taskName, task, needs, started, opts)
		if err == nil && opts.Events != nil {
			opts.Events.EmitDone(taskName, result.Status, result.DurationMS)
		}
		return result, err
	}

	if task.Parallel {
		result, err := r.runParallel(ctx, taskName, task, needs, started, opts)
		if err == nil && opts.Events != nil {
			opts.Events.EmitDone(taskName, result.Status, result.DurationMS)
		}
		return result, err
	}
	result, err := r.runSequential(ctx, taskName, task, needs, started, opts)
	if err == nil && opts.Events != nil {
		opts.Events.EmitDone(taskName, result.Status, result.DurationMS)
	}
	return result, err
}

func hasFreshDependency(needs []Result) bool {
	for _, need := range needs {
		if need.Status != StatusPass {
			continue
		}
		if need.Type == "cmd" {
			if !need.Cached {
				return true
			}
			continue
		}
		if hasFreshStep(need.Steps) || hasFreshDependency(need.Needs) {
			return true
		}
	}
	return false
}

func (r *Runner) executeCommandTask(ctx context.Context, taskName, stepName string, task config.Task, resolved string, paramValues map[string]string, needs []Result, opts Options) (Result, error) {
	outcome, err := r.runCommandWithRetry(ctx, stepName, task, resolved, opts, "")
	if err != nil {
		return Result{}, err
	}
	outcome = r.runDeferredCommand(ctx, stepName, task, paramValues, opts, outcome)
	result := Result{
		Task:        taskName,
		Type:        "cmd",
		Needs:       needs,
		ResolvedCmd: visibleResolvedCmd(task, resolved),
		Status:      outcome.status,
		ExitCode:    outcome.exitCode,
		Stdout:      outcome.stdout,
		Stderr:      outcome.stderr,
		Errors:      extractErrors(task.ErrorFormat, outcome.stderr),
		DurationMS:  outcome.finished.Sub(outcome.started).Milliseconds(),
		StartedAt:   outcome.started.UTC().Format(time.RFC3339),
		FinishedAt:  outcome.finished.UTC().Format(time.RFC3339),
	}
	if opts.Events != nil {
		opts.Events.EmitDone(taskName, result.Status, result.DurationMS)
	}
	return result, nil
}

func hasFreshStep(steps []StepResult) bool {
	for _, step := range steps {
		if step.Status != StatusPass {
			continue
		}
		if step.Type == "cmd" && step.SkipReason == "" {
			return true
		}
		if hasFreshStep(step.Steps) {
			return true
		}
	}
	return false
}

func (r *Runner) celVars(opts Options) map[string]any {
	env := map[string]string{}
	for _, pair := range os.Environ() {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		env[parts[0]] = parts[1]
	}
	return map[string]any{
		"env":       env,
		"branch":    r.branch,
		"tag":       r.tag,
		"profile":   r.cfg.ActiveProfile(),
		"repo_root": r.repoRoot,
		"os":        runtime.GOOS,
		"params":    opts.Params,
		"vars":      map[string]string(r.cfg.Vars),
		"var":       map[string]string(r.cfg.Vars),
	}
}

func detectGitBranch(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectGitTag(repoRoot string) string {
	cmd := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func visibleResolvedCmd(task config.Task, resolved string) *string {
	if task.Silent {
		return nil
	}
	return strPtr(resolved)
}

func (r *Runner) runDeferredCommand(ctx context.Context, label string, task config.Task, paramValues map[string]string, opts Options, outcome runOutcome) runOutcome {
	if strings.TrimSpace(task.Defer) == "" || opts.DryRun {
		return outcome
	}

	deferTask := task
	deferTask.Timeout = ""
	deferTask.Defer = ""
	deferCmd := interpolateTaskValue(task.Defer, paramValues, map[string]string(r.cfg.Vars), r.cfg.Templates.Snippets, r.secrets)
	deferOutcome, err := r.runCommand(ctx, label+":defer", deferTask, deferCmd, opts, "")
	if err != nil {
		msg := fmt.Sprintf("[qp] defer command failed for task %q: %v\n", label, err)
		if opts.Stderr != nil {
			_, _ = io.WriteString(opts.Stderr, msg)
		}
		outcome.stderr += msg
		return outcome
	}
	if deferOutcome.status != StatusPass {
		msg := fmt.Sprintf("[qp] defer command failed for task %q with status %s (exit %d)\n", label, deferOutcome.status, deferOutcome.exitCode)
		if opts.Stderr != nil {
			_, _ = io.WriteString(opts.Stderr, msg)
		}
		outcome.stderr += msg
	}
	return outcome
}
