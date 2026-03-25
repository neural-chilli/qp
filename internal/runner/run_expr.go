package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/neural-chilli/qp/internal/config"
)

func (r *Runner) runFromExpression(ctx context.Context, taskName string, task config.Task, needs []Result, started time.Time, opts Options) (Result, error) {
	expr, err := config.ParseRunExpr(task.Run)
	if err != nil {
		return Result{}, fmt.Errorf("task %q: invalid run expression: %w", taskName, err)
	}
	root, err := r.runExprNode(ctx, expr, opts)
	if err != nil {
		return Result{}, err
	}

	finished := time.Now()
	steps := root.Steps
	if len(steps) == 0 {
		steps = []StepResult{root}
	}
	return Result{
		Task:       taskName,
		Type:       "pipeline",
		Needs:      needs,
		Parallel:   false,
		Status:     root.Status,
		ExitCode:   root.ExitCode,
		Errors:     collectStepErrors(steps),
		DurationMS: finished.Sub(started).Milliseconds(),
		StartedAt:  started.UTC().Format(time.RFC3339),
		FinishedAt: finished.UTC().Format(time.RFC3339),
		Steps:      steps,
	}, nil
}

func (r *Runner) runExprNode(ctx context.Context, node config.RunExpr, opts Options) (StepResult, error) {
	switch n := node.(type) {
	case config.RunRef:
		result, err := r.runTask(ctx, n.Name, opts)
		if err != nil {
			return StepResult{}, err
		}
		return resultToStepResult(0, n.Name, result), nil
	case config.RunSeq:
		return r.runExprSequence(ctx, n, opts)
	case config.RunPar:
		return r.runExprParallel(ctx, n, opts)
	case config.RunWhen:
		ok, err := r.celEngine.EvalBool(n.Expr, r.celVars(opts))
		if err != nil {
			return StepResult{}, fmt.Errorf("when(%s): %w", n.Expr, err)
		}
		if ok {
			step, err := r.runExprNode(ctx, n.True, opts)
			if err != nil {
				return StepResult{}, err
			}
			step.Name = "when:true"
			return step, nil
		}
		if n.False != nil {
			step, err := r.runExprNode(ctx, n.False, opts)
			if err != nil {
				return StepResult{}, err
			}
			step.Name = "when:false"
			return step, nil
		}
		started := time.Now()
		finished := time.Now()
		duration := finished.Sub(started).Milliseconds()
		startedAt := started.UTC().Format(time.RFC3339)
		finishedAt := finished.UTC().Format(time.RFC3339)
		return StepResult{
			Name:       "when",
			Status:     StatusSkipped,
			ExitCode:   0,
			SkipReason: fmt.Sprintf("when condition is false: %s", n.Expr),
			DurationMS: &duration,
			StartedAt:  &startedAt,
			FinishedAt: &finishedAt,
		}, nil
	case config.RunSwitch:
		value, err := r.celEngine.Eval(n.Expr, r.celVars(opts))
		if err != nil {
			return StepResult{}, fmt.Errorf("switch(%s): %w", n.Expr, err)
		}
		resolved := fmt.Sprint(value)
		for _, c := range n.Cases {
			if c.Value != resolved {
				continue
			}
			step, err := r.runExprNode(ctx, c.Expr, opts)
			if err != nil {
				return StepResult{}, err
			}
			step.Name = "switch:" + c.Value
			return step, nil
		}
		started := time.Now()
		finished := time.Now()
		duration := finished.Sub(started).Milliseconds()
		startedAt := started.UTC().Format(time.RFC3339)
		finishedAt := finished.UTC().Format(time.RFC3339)
		return StepResult{
			Name:       "switch",
			Status:     StatusSkipped,
			ExitCode:   0,
			SkipReason: fmt.Sprintf("switch had no matching case for value %q", resolved),
			DurationMS: &duration,
			StartedAt:  &startedAt,
			FinishedAt: &finishedAt,
		}, nil
	default:
		return StepResult{}, fmt.Errorf("unsupported run expression node")
	}
}

func (r *Runner) runExprSequence(ctx context.Context, seq config.RunSeq, opts Options) (StepResult, error) {
	started := time.Now()
	steps := make([]StepResult, 0, len(seq.Nodes))
	overallStatus := StatusPass
	overallCode := 0

	for i, child := range seq.Nodes {
		step, err := r.runExprNode(ctx, child, opts)
		if err != nil {
			return StepResult{}, err
		}
		step.Index = i
		steps = append(steps, step)
		if step.Status != StatusPass && step.Status != StatusSkipped {
			overallStatus = step.Status
			overallCode = step.ExitCode
			for j := i + 1; j < len(seq.Nodes); j++ {
				steps = append(steps, dagCancelledStep(j, seq.Nodes[j]))
			}
			break
		}
	}

	finished := time.Now()
	duration := finished.Sub(started).Milliseconds()
	startedAt := started.UTC().Format(time.RFC3339)
	finishedAt := finished.UTC().Format(time.RFC3339)
	return StepResult{
		Index:      0,
		Name:       "seq",
		Type:       "pipeline",
		Status:     overallStatus,
		ExitCode:   overallCode,
		DurationMS: &duration,
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
		Steps:      steps,
	}, nil
}

func (r *Runner) runExprParallel(parent context.Context, par config.RunPar, opts Options) (StepResult, error) {
	started := time.Now()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	steps := make([]StepResult, len(par.Nodes))
	errCh := make(chan error, len(par.Nodes))
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := false

	for i, child := range par.Nodes {
		wg.Add(1)
		go func(i int, child config.RunExpr) {
			defer wg.Done()
			step, err := r.runExprNode(ctx, child, opts)
			if err != nil {
				errCh <- err
				return
			}
			step.Index = i
			mu.Lock()
			steps[i] = step
			if step.Status != StatusPass && step.Status != StatusSkipped && step.Status != StatusCancelled {
				failed = true
				cancel()
			}
			mu.Unlock()
		}(i, child)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return StepResult{}, err
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
			steps[i] = dagCancelledStep(i, par.Nodes[i])
		}
		if step.ExitCode != 0 && overallCode == 0 {
			overallCode = step.ExitCode
		}
	}

	finished := time.Now()
	duration := finished.Sub(started).Milliseconds()
	startedAt := started.UTC().Format(time.RFC3339)
	finishedAt := finished.UTC().Format(time.RFC3339)
	return StepResult{
		Index:      0,
		Name:       "par",
		Type:       "pipeline",
		Parallel:   true,
		Status:     overallStatus,
		ExitCode:   overallCode,
		DurationMS: &duration,
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
		Steps:      steps,
	}, nil
}

func dagCancelledStep(index int, node config.RunExpr) StepResult {
	return StepResult{
		Index:    index,
		Name:     dagNodeName(node),
		Status:   StatusCancelled,
		ExitCode: 130,
	}
}

func dagNodeName(node config.RunExpr) string {
	switch n := node.(type) {
	case config.RunRef:
		return n.Name
	case config.RunSeq:
		return "seq"
	case config.RunPar:
		return "par"
	case config.RunWhen:
		return "when"
	case config.RunSwitch:
		return "switch"
	default:
		return "step"
	}
}
