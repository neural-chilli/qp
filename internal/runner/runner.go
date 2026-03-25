package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	NoCache     bool
	AllowUnsafe bool
	Stdout      io.Writer
	Stderr      io.Writer
	Env         map[string]string
	Params      map[string]string
	Events      *EventStream
}

type Runner struct {
	cfg       *config.Config
	repoRoot  string
	globalEnv map[string]string
	branch    string
	celEngine *celpkg.Engine
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
	return &Runner{
		cfg:       cfg,
		repoRoot:  repoRoot,
		globalEnv: loadEnvFile(filepath.Join(repoRoot, cfg.EnvFile)),
		branch:    detectGitBranch(repoRoot),
		celEngine: celpkg.New(),
	}
}

func (r *Runner) Run(taskName string, opts Options) (Result, error) {
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
		resolved := interpolateTaskValue(task.Cmd, paramValues, r.cfg.Vars, r.cfg.Templates)
		if task.CacheEnabled() && !opts.NoCache && !opts.DryRun {
			cacheKey := makeCacheKey(cacheKeyInput{
				TaskName:    taskName,
				Task:        task,
				ResolvedCmd: resolved,
				Params:      paramValues,
				Env:         interpolateEnv(task.Env, paramValues, r.cfg.Vars, r.cfg.Templates),
				WorkDir:     r.resolveTaskDir(task),
				Profile:     os.Getenv("QP_PROFILE"),
				ExtraEnv:    opts.Env,
			})
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
			outcome, err := r.runCommand(ctx, stepName, task, resolved, opts, "")
			if err != nil {
				return Result{}, err
			}
			result := Result{
				Task:        taskName,
				Type:        "cmd",
				Needs:       needs,
				ResolvedCmd: strPtr(resolved),
				Status:      outcome.status,
				ExitCode:    outcome.exitCode,
				Stdout:      outcome.stdout,
				Stderr:      outcome.stderr,
				Errors:      extractErrors(task.ErrorFormat, outcome.stderr),
				DurationMS:  outcome.finished.Sub(outcome.started).Milliseconds(),
				StartedAt:   outcome.started.UTC().Format(time.RFC3339),
				FinishedAt:  outcome.finished.UTC().Format(time.RFC3339),
			}
			if result.Status == StatusPass {
				writeCachedResult(r.repoRoot, cacheKey, result)
			}
			if opts.Events != nil {
				opts.Events.EmitDone(taskName, result.Status, result.DurationMS)
			}
			return result, nil
		}
		outcome, err := r.runCommand(ctx, stepName, task, resolved, opts, "")
		if err != nil {
			return Result{}, err
		}
		result := Result{
			Task:        taskName,
			Type:        "cmd",
			Needs:       needs,
			ResolvedCmd: strPtr(resolved),
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
		"env":    env,
		"branch": r.branch,
		"params": opts.Params,
		"vars":   r.cfg.Vars,
		"var":    r.cfg.Vars,
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
