package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/neural-chilli/qp/internal/config"
)

func (r *Runner) runSequential(ctx context.Context, taskName string, task config.Task, needs []Result, started time.Time, opts Options) (Result, error) {
	steps := make([]StepResult, 0, len(task.Steps))
	overallStatus := StatusPass
	overallCode := 0

	for i, step := range task.Steps {
		stepRes, err := r.runStep(ctx, i, step, opts)
		if err != nil {
			return Result{}, err
		}
		steps = append(steps, stepRes)
		if stepRes.Status != StatusPass && stepRes.Status != StatusSkipped {
			overallStatus = stepRes.Status
			overallCode = stepRes.ExitCode
			if !task.ContinueOnError {
				for j := i + 1; j < len(task.Steps); j++ {
					steps = append(steps, cancelledStep(j, task.Steps[j]))
				}
				break
			}
		}
	}

	finished := time.Now()
	return Result{
		Task:       taskName,
		Type:       "pipeline",
		Needs:      needs,
		Parallel:   false,
		Status:     overallStatus,
		ExitCode:   overallCode,
		Errors:     collectStepErrors(steps),
		DurationMS: finished.Sub(started).Milliseconds(),
		StartedAt:  started.UTC().Format(time.RFC3339),
		FinishedAt: finished.UTC().Format(time.RFC3339),
		Steps:      steps,
	}, nil
}

func (r *Runner) runParallel(parent context.Context, taskName string, task config.Task, needs []Result, started time.Time, opts Options) (Result, error) {
	warningText := ""
	if task.ContinueOnError {
		warningText = "warning: continue_on_error is ignored for parallel tasks in v1"
		if !opts.JSON && opts.Stderr != nil {
			fmt.Fprintln(opts.Stderr, warningText)
		}
	}

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	steps := make([]StepResult, len(task.Steps))
	errCh := make(chan error, len(task.Steps))
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := false

	for i, step := range task.Steps {
		wg.Add(1)
		go func(i int, step string) {
			defer wg.Done()
			stepRes, err := r.runStep(ctx, i, step, opts)
			if err != nil {
				errCh <- err
				return
			}

			mu.Lock()
			steps[i] = stepRes
			if stepRes.Status != StatusPass && stepRes.Status != StatusSkipped && stepRes.Status != StatusCancelled {
				failed = true
				cancel()
			}
			mu.Unlock()
		}(i, step)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return Result{}, err
		}
	}

	overallStatus := StatusPass
	overallCode := 0
	if failed {
		overallStatus = StatusFail
		overallCode = 1
	}
	for i, step := range steps {
		if step.Name == "" {
			steps[i] = cancelledStep(i, task.Steps[i])
		}
		if step.ExitCode != 0 && overallCode == 0 {
			overallCode = step.ExitCode
		}
	}

	finished := time.Now()
	return Result{
		Task:       taskName,
		Type:       "pipeline",
		Needs:      needs,
		Parallel:   true,
		Status:     overallStatus,
		ExitCode:   overallCode,
		Stderr:     warningText,
		Errors:     collectStepErrors(steps),
		DurationMS: finished.Sub(started).Milliseconds(),
		StartedAt:  started.UTC().Format(time.RFC3339),
		FinishedAt: finished.UTC().Format(time.RFC3339),
		Steps:      steps,
	}, nil
}

func (r *Runner) runNeeds(ctx context.Context, task config.Task, opts Options) ([]Result, *Result, error) {
	if len(task.Needs) == 0 {
		return nil, nil, nil
	}
	results := make([]Result, 0, len(task.Needs))
	for _, depName := range task.Needs {
		result, err := r.runTask(ctx, depName, opts)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, result)
		if result.Status != StatusPass {
			failed := result
			return results, &failed, nil
		}
	}
	return results, nil, nil
}

func (r *Runner) runStep(ctx context.Context, index int, step string, opts Options) (StepResult, error) {
	if _, ok := r.cfg.Tasks[step]; ok {
		result, err := r.runTask(ctx, step, opts)
		if err != nil {
			return StepResult{}, err
		}
		return resultToStepResult(index, step, result), nil
	}

	outcome, err := r.runCommand(ctx, inlineStepName(index), config.Task{}, step, opts, inlineStepName(index))
	if err != nil {
		return StepResult{}, err
	}
	return toStepResult(index, inlineStepName(index), step, outcome), nil
}

func toStepResult(index int, name, resolved string, outcome runOutcome) StepResult {
	duration := outcome.finished.Sub(outcome.started).Milliseconds()
	started := outcome.started.UTC().Format(time.RFC3339)
	finished := outcome.finished.UTC().Format(time.RFC3339)
	return StepResult{
		Index:       index,
		Name:        name,
		Type:        "cmd",
		ResolvedCmd: strPtr(resolved),
		Status:      outcome.status,
		ExitCode:    outcome.exitCode,
		Stdout:      strPtr(outcome.stdout),
		Stderr:      strPtr(outcome.stderr),
		Errors:      nil,
		DurationMS:  &duration,
		StartedAt:   &started,
		FinishedAt:  &finished,
	}
}

func resultToStepResult(index int, name string, result Result) StepResult {
	duration := result.DurationMS
	started := result.StartedAt
	finished := result.FinishedAt
	stdout := result.Stdout
	stderr := result.Stderr
	if stderr == "" && len(result.Steps) > 0 {
		stderr = nestedStepsStderr(result.Steps)
	}
	return StepResult{
		Index:       index,
		Name:        name,
		Type:        result.Type,
		ResolvedCmd: result.ResolvedCmd,
		Parallel:    result.Parallel,
		Status:      result.Status,
		ExitCode:    result.ExitCode,
		Stdout:      strPtr(stdout),
		Stderr:      strPtr(stderr),
		Errors:      collectResultErrors(result),
		DurationMS:  &duration,
		StartedAt:   &started,
		FinishedAt:  &finished,
		Steps:       append([]StepResult(nil), result.Steps...),
	}
}

func nestedStepsStderr(steps []StepResult) string {
	var parts []string
	for _, step := range steps {
		if step.Stderr != nil && *step.Stderr != "" {
			parts = append(parts, fmt.Sprintf("[%s]\n%s", step.Name, strings.TrimRight(*step.Stderr, "\n")))
		}
		if nested := nestedStepsStderr(step.Steps); nested != "" {
			parts = append(parts, nested)
		}
	}
	return strings.Join(parts, "\n")
}

func cancelledStep(index int, name string) StepResult {
	return StepResult{
		Index:    index,
		Name:     name,
		Status:   StatusCancelled,
		ExitCode: 130,
	}
}

func inlineStepName(index int) string {
	return fmt.Sprintf("step-%d", index+1)
}
