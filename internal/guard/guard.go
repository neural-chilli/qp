package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/neural-chilli/qp/internal/config"
	"github.com/neural-chilli/qp/internal/runner"
)

type Runner struct {
	cfg      *config.Config
	repoRoot string
	tasks    *runner.Runner
}

type Report struct {
	Guard      string      `json:"guard"`
	Overall    string      `json:"overall"`
	ExitCode   int         `json:"exit_code"`
	DurationMS int64       `json:"duration_ms"`
	RanAt      string      `json:"ran_at"`
	Steps      []GuardStep `json:"steps"`
}

type GuardStep struct {
	Index       int                 `json:"index"`
	Name        string              `json:"name"`
	ResolvedCmd *string             `json:"resolved_cmd"`
	Status      string              `json:"status"`
	ExitCode    int                 `json:"exit_code"`
	Stderr      string              `json:"stderr"`
	Errors      []runner.ErrorEntry `json:"errors,omitempty"`
	DurationMS  int64               `json:"duration_ms"`
}

type CacheReport struct {
	Guard string      `json:"guard"`
	RanAt string      `json:"ran_at"`
	Steps []CacheStep `json:"steps"`
}

type CacheStep struct {
	Index       int                 `json:"index"`
	Name        string              `json:"name"`
	ResolvedCmd *string             `json:"resolved_cmd"`
	Status      string              `json:"status"`
	ExitCode    int                 `json:"exit_code"`
	Stderr      string              `json:"stderr"`
	Errors      []runner.ErrorEntry `json:"errors,omitempty"`
	DurationMS  int64               `json:"duration_ms"`
}

func New(cfg *config.Config, repoRoot string, tasks *runner.Runner) *Runner {
	return &Runner{cfg: cfg, repoRoot: repoRoot, tasks: tasks}
}

func (r *Runner) Run(name string, opts runner.Options) (Report, error) {
	guardName := name
	if guardName == "" {
		guardName = "default"
	}

	guardCfg, ok := r.cfg.Guards[guardName]
	if !ok {
		return Report{}, fmt.Errorf("unknown guard %q", guardName)
	}

	started := time.Now()
	report := Report{
		Guard:    guardName,
		Overall:  runner.StatusPass,
		ExitCode: 0,
		RanAt:    started.UTC().Format(time.RFC3339),
		Steps:    make([]GuardStep, 0, len(guardCfg.Steps)),
	}

	for i, stepName := range guardCfg.Steps {
		result, err := r.tasks.Run(stepName, opts)
		if err != nil {
			return Report{}, err
		}
		stderr := result.Stderr
		if stderr == "" {
			stderr = nestedStepsStderr(result.Steps)
		}

		reportStep := GuardStep{
			Index:       i,
			Name:        stepName,
			ResolvedCmd: result.ResolvedCmd,
			Status:      result.Status,
			ExitCode:    result.ExitCode,
			Stderr:      stderr,
			Errors:      collectResultErrors(result),
			DurationMS:  result.DurationMS,
		}
		report.Steps = append(report.Steps, reportStep)

		if reportStep.Status != runner.StatusPass && report.ExitCode == 0 {
			report.Overall = runner.StatusFail
			report.ExitCode = reportStep.ExitCode
		}
	}

	finished := time.Now()
	report.DurationMS = finished.Sub(started).Milliseconds()

	if err := r.writeCache(report); err != nil {
		return Report{}, err
	}

	return report, nil
}

func nestedStepsStderr(steps []runner.StepResult) string {
	var combined string
	for _, step := range steps {
		if step.Stderr != nil && *step.Stderr != "" {
			combined += *step.Stderr
		}
		if nested := nestedStepsStderr(step.Steps); nested != "" {
			combined += nested
		}
	}
	return combined
}

func collectResultErrors(result runner.Result) []runner.ErrorEntry {
	if len(result.Errors) > 0 {
		return append([]runner.ErrorEntry(nil), result.Errors...)
	}
	errors := collectStepErrors(result.Steps)
	if len(errors) == 0 {
		return nil
	}
	return errors
}

func collectStepErrors(steps []runner.StepResult) []runner.ErrorEntry {
	var errors []runner.ErrorEntry
	for _, step := range steps {
		if len(step.Errors) > 0 {
			errors = append(errors, step.Errors...)
			continue
		}
		errors = append(errors, collectStepErrors(step.Steps)...)
	}
	return errors
}

func (r *Runner) writeCache(report Report) error {
	cache := CacheReport{
		Guard: report.Guard,
		RanAt: report.RanAt,
		Steps: make([]CacheStep, 0, len(report.Steps)),
	}
	for _, step := range report.Steps {
		cache.Steps = append(cache.Steps, CacheStep{
			Index:       step.Index,
			Name:        step.Name,
			ResolvedCmd: step.ResolvedCmd,
			Status:      step.Status,
			ExitCode:    step.ExitCode,
			Stderr:      step.Stderr,
			Errors:      step.Errors,
			DurationMS:  step.DurationMS,
		})
	}

	stateDir := filepath.Join(r.repoRoot, ".qp")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}

	path := filepath.Join(stateDir, "last-guard.json")
	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}
