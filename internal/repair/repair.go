package repair

import (
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
	"github.com/neural-chilli/qp/internal/guard"
	"github.com/neural-chilli/qp/internal/runner"
)

type Generator struct {
	cfg      *config.Config
	repoRoot string
	guard    *guard.Runner
}

type Options struct {
	GuardName   string
	AllowUnsafe bool
}

type ScopeInfo struct {
	Name  string   `json:"name"`
	Desc  string   `json:"desc,omitempty"`
	Paths []string `json:"paths"`
}

type Failure struct {
	Task        string              `json:"task"`
	Description string              `json:"description,omitempty"`
	Status      string              `json:"status"`
	ExitCode    int                 `json:"exit_code"`
	DurationMS  int64               `json:"duration_ms"`
	Scope       *ScopeInfo          `json:"scope,omitempty"`
	Errors      []runner.ErrorEntry `json:"errors,omitempty"`
	Stderr      string              `json:"stderr,omitempty"`
}

type Output struct {
	Guard               string    `json:"guard"`
	Overall             string    `json:"overall"`
	ExitCode            int       `json:"exit_code"`
	RanAt               string    `json:"ran_at"`
	Failures            []Failure `json:"failures"`
	GitDiff             string    `json:"git_diff,omitempty"`
	SuggestedNextAction string    `json:"suggested_next_action"`
	Markdown            string    `json:"markdown"`
}

func New(cfg *config.Config, repoRoot string, guardRunner *guard.Runner) *Generator {
	return &Generator{cfg: cfg, repoRoot: repoRoot, guard: guardRunner}
}

func (g *Generator) Generate(opts Options) (Output, error) {
	report, err := g.guard.Run(opts.GuardName, runner.Options{
		AllowUnsafe: opts.AllowUnsafe,
		Stdout:      io.Discard,
		Stderr:      io.Discard,
	})
	if err != nil {
		return Output{}, err
	}

	failures := g.failures(report)
	gitDiff := truncateRepairLines(g.gitDiffLines(), g.gitDiffCap())
	suggested := g.suggestedNextAction(report, failures)
	markdown := render(report, failures, gitDiff, suggested)

	return Output{
		Guard:               report.Guard,
		Overall:             report.Overall,
		ExitCode:            report.ExitCode,
		RanAt:               report.RanAt,
		Failures:            failures,
		GitDiff:             gitDiff,
		SuggestedNextAction: suggested,
		Markdown:            markdown,
	}, nil
}

func (g *Generator) failures(report guard.Report) []Failure {
	failures := make([]Failure, 0)
	for _, step := range report.Steps {
		if step.Status == runner.StatusPass {
			continue
		}
		failure := Failure{
			Task:       step.Name,
			Status:     step.Status,
			ExitCode:   step.ExitCode,
			DurationMS: step.DurationMS,
			Errors:     step.Errors,
			Stderr:     strings.TrimSpace(step.Stderr),
		}
		if task, ok := g.cfg.Tasks[step.Name]; ok {
			failure.Description = task.Desc
			if task.Scope != "" {
				scopeDef := g.cfg.Scopes[task.Scope]
				paths := append([]string(nil), scopeDef.Paths...)
				failure.Scope = &ScopeInfo{Name: task.Scope, Desc: scopeDef.Desc, Paths: paths}
			}
		}
		failures = append(failures, failure)
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].Task < failures[j].Task })
	return failures
}

func (g *Generator) suggestedNextAction(report guard.Report, failures []Failure) string {
	if report.Overall == runner.StatusPass {
		return "Guard passed. No repair is needed right now."
	}
	if len(failures) == 0 {
		return "Inspect the failing guard steps, then rerun `qp repair` after making a targeted fix."
	}
	first := failures[0]
	if first.Scope != nil {
		return fmt.Sprintf("Start with `%s` in scope `%s`, fix the reported errors, then rerun `qp repair %s`.", first.Task, first.Scope.Name, report.Guard)
	}
	return fmt.Sprintf("Start with `%s`, fix the reported errors, then rerun `qp repair %s`.", first.Task, report.Guard)
}

func render(report guard.Report, failures []Failure, gitDiff, suggested string) string {
	var sections []string
	sections = append(sections, "# qp repair")
	sections = append(sections, renderGuardStatus(report))
	sections = append(sections, renderFailures(failures))
	if gitDiff != "" {
		sections = append(sections, "## Git Diff\n\n```diff\n"+gitDiff+"\n```")
	}
	sections = append(sections, "## Suggested Next Action\n\n"+suggested)
	return strings.Join(sections, "\n\n")
}

func renderGuardStatus(report guard.Report) string {
	lines := []string{
		"## Guard Status",
		"",
		fmt.Sprintf("- Guard: `%s`", report.Guard),
		fmt.Sprintf("- Overall: `%s`", report.Overall),
		fmt.Sprintf("- Exit code: %d", report.ExitCode),
		fmt.Sprintf("- Ran at: %s", report.RanAt),
		"- Steps:",
	}
	for _, step := range report.Steps {
		lines = append(lines, fmt.Sprintf("  - `%s`: %s in %.1fs", step.Name, step.Status, float64(step.DurationMS)/1000))
	}
	return strings.Join(lines, "\n")
}

func renderFailures(failures []Failure) string {
	if len(failures) == 0 {
		return "## Failures\n\nNo failing steps."
	}
	sections := []string{"## Failures"}
	for _, failure := range failures {
		var lines []string
		lines = append(lines, fmt.Sprintf("### %s", failure.Task))
		if failure.Description != "" {
			lines = append(lines, "", failure.Description)
		}
		lines = append(lines, "", fmt.Sprintf("- Status: `%s`", failure.Status))
		lines = append(lines, fmt.Sprintf("- Exit code: %d", failure.ExitCode))
		if failure.Scope != nil {
			lines = append(lines, fmt.Sprintf("- Scope: `%s`", failure.Scope.Name))
			if failure.Scope.Desc != "" {
				lines = append(lines, fmt.Sprintf("- Scope intent: %s", failure.Scope.Desc))
			}
			lines = append(lines, fmt.Sprintf("- Paths: %s", strings.Join(failure.Scope.Paths, ", ")))
		}
		if len(failure.Errors) > 0 {
			lines = append(lines, "- Parsed errors:")
			for _, entry := range failure.Errors {
				location := entry.File
				if entry.Line > 0 {
					location = fmt.Sprintf("%s:%d", entry.File, entry.Line)
					if entry.Column > 0 {
						location = fmt.Sprintf("%s:%d", location, entry.Column)
					}
				}
				if location != "" {
					lines = append(lines, fmt.Sprintf("  - `%s`: %s", location, entry.Message))
				} else {
					lines = append(lines, fmt.Sprintf("  - %s", entry.Message))
				}
			}
		} else if failure.Stderr != "" {
			lines = append(lines, "- Stderr:", "", "```text", truncateRepairLines(strings.Split(failure.Stderr, "\n"), 20), "```")
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	return strings.Join(sections, "\n\n")
}

func (g *Generator) gitDiffLines() []string {
	cmd := exec.Command("git", "diff", "--", ".")
	cmd.Dir = g.repoRoot
	raw, err := cmd.Output()
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func (g *Generator) gitDiffCap() int {
	return g.cfg.Context.Caps.GitDiffLines
}

func truncateRepairLines(lines []string, cap int) string {
	if len(lines) == 0 {
		return ""
	}
	if cap <= 0 || len(lines) <= cap {
		return strings.Join(lines, "\n")
	}
	marker := fmt.Sprintf("[Lines %d-%d omitted]", cap+1, len(lines))
	return strings.Join(append(lines[:cap], marker), "\n")
}
