package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
	"github.com/neural-chilli/qp/internal/guard"
	"github.com/neural-chilli/qp/internal/ordered"
	"github.com/neural-chilli/qp/internal/runner"
)

func printUsage(stdout *os.File) {
	lines := []string{
		"qp [--no-color] [<task>] [--name value] [--param name=value] [--dry-run] [--no-cache] [--allow-unsafe] [--events] [--json]",
		"If qp.yaml sets `default`, running `qp` with no task runs that task.",
		"qp agent-brief [--task <name> | --diff | --file <path>...] [--json] [--max-tokens <approx-n>]",
		"qp arch-check [--json]",
		"qp completion <bash|zsh|fish|powershell>",
		"qp completion install [--shell <bash|zsh|fish|powershell>]",
		"qp daemon <start|stop|status|restart>",
		"qp docs [name] [--list]",
		"qp diff-plan [--json]",
		"qp explain <target> [--json]",
		"qp help [task|group]",
		"qp plan [--json] [--file <path>] [files...]",
		"qp context [--agent] [--json] [--task <name>] [--about <topic>] [--out <file>] [--copy] [--max-tokens <approx-n>]",
		"qp guard [name] [--json]",
		"qp repair [name] [--json] [--copy]",
		"qp run <task> [task-flags]",
		"qp setup [--windows]",
		"qp init [--from-repo] [--docs] [--harness] [--codemap]",
		"qp list [--json]",
		"qp watch <target> [--path <glob>]",
		"qp prompt <name> [--copy]",
		"qp scope <name> [--json] [--format prompt]",
		"qp validate [--json]",
		"qp version",
	}
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
}

func printTaskHelp(stdout *os.File, invokedName, resolvedName string, cfg *config.Config, task config.Task, aliases []string, isDefault bool) {
	fmt.Fprintf(stdout, "%s\n\n", invokedName)
	fmt.Fprintf(stdout, "Description: %s\n", task.Desc)
	if invokedName != resolvedName {
		fmt.Fprintf(stdout, "Alias For: %s\n", resolvedName)
	}
	if isDefault {
		fmt.Fprintln(stdout, "Default: true")
	}
	if len(aliases) > 0 {
		fmt.Fprintf(stdout, "Aliases: %s\n", strings.Join(aliases, ", "))
	}
	if groups := config.GroupNamesForTask(cfg.Groups, resolvedName); len(groups) > 0 {
		fmt.Fprintf(stdout, "Groups: %s\n", strings.Join(groups, ", "))
	}
	if len(task.Needs) > 0 {
		fmt.Fprintf(stdout, "Needs: %s\n", strings.Join(task.Needs, ", "))
	}
	fmt.Fprintf(stdout, "Usage: %s\n", taskUsage(invokedName, task))
	fmt.Fprintf(stdout, "Type: %s\n", task.Type())
	if task.Scope != "" {
		fmt.Fprintf(stdout, "Scope: %s\n", task.Scope)
		if scopeDef, ok := cfg.Scopes[task.Scope]; ok && scopeDef.Desc != "" {
			fmt.Fprintf(stdout, "Scope Desc: %s\n", scopeDef.Desc)
		}
	}
	fmt.Fprintf(stdout, "Agent: %t\n", task.AgentEnabled())
	fmt.Fprintf(stdout, "Safety: %s\n", task.SafetyLevel())
	if task.CacheEnabled() {
		fmt.Fprintln(stdout, "Cache: true")
	}
	if task.Silent {
		fmt.Fprintln(stdout, "Silent: true")
	}
	if task.When != "" {
		fmt.Fprintf(stdout, "When: %s\n", task.When)
	}
	if task.Dir != "" {
		fmt.Fprintf(stdout, "Dir: %s\n", task.Dir)
	} else if cfg.Defaults.Dir != "" {
		fmt.Fprintf(stdout, "Dir: %s (inherited default)\n", cfg.Defaults.Dir)
	}
	if task.Shell != "" {
		fmt.Fprintf(stdout, "Shell: %s\n", task.Shell)
	}
	if len(task.ShellArgs) > 0 {
		fmt.Fprintf(stdout, "Shell Args: %s\n", strings.Join(task.ShellArgs, " "))
	}
	if task.Timeout != "" {
		fmt.Fprintf(stdout, "Timeout: %s\n", task.Timeout)
	}
	if task.Defer != "" {
		fmt.Fprintf(stdout, "Defer: %s\n", task.Defer)
	}
	if len(task.Params) > 0 {
		fmt.Fprintln(stdout, "Params:")
		for _, paramName := range sortedParamNames(task.Params) {
			param := task.Params[paramName]
			fmt.Fprintf(stdout, "- %s", paramName)
			if param.Env != "" {
				fmt.Fprintf(stdout, " (env: %s)", param.Env)
			}
			if param.Position > 0 {
				if param.Variadic {
					fmt.Fprintf(stdout, " positional[%d...]", param.Position)
				} else {
					fmt.Fprintf(stdout, " positional[%d]", param.Position)
				}
			}
			if param.Required {
				fmt.Fprint(stdout, " required")
			}
			if param.Default != "" {
				fmt.Fprintf(stdout, " default=%q", param.Default)
			}
			if param.Desc != "" {
				fmt.Fprintf(stdout, " - %s", param.Desc)
			}
			fmt.Fprintln(stdout)
		}
	}
	if task.Cmd != "" {
		fmt.Fprintf(stdout, "Command: %s\n", task.Cmd)
		return
	}
	fmt.Fprintf(stdout, "Parallel: %t\n", task.Parallel)
	fmt.Fprintln(stdout, "Steps:")
	for _, step := range task.Steps {
		fmt.Fprintf(stdout, "- %s\n", step)
	}
}

func printGuardHelp(stdout *os.File, name string, guardCfg config.Guard) {
	fmt.Fprintf(stdout, "guard %s\n\n", name)
	fmt.Fprintln(stdout, "Steps:")
	for _, step := range guardCfg.Steps {
		fmt.Fprintf(stdout, "- %s\n", step)
	}
}

func printGroupHelp(stdout *os.File, name string, group config.Group, cfg *config.Config) {
	fmt.Fprintf(stdout, "group %s\n\n", name)
	if group.Desc != "" {
		fmt.Fprintf(stdout, "Description: %s\n", group.Desc)
	}
	fmt.Fprintln(stdout, "Tasks:")
	for _, taskName := range group.Tasks {
		task := cfg.Tasks[taskName]
		fmt.Fprintf(stdout, "- %s: %s\n", taskName, task.Desc)
	}
}

func printError(stderr *os.File, err error) {
	styler := newOutputStyler(stderr)
	msg := styler.highlightDiagnostics(err.Error())
	fmt.Fprintf(stderr, "%s %s\n", styler.errorPrefix(), msg)
}

func printJSON(stdout *os.File, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return 1
	}
	return 0
}

func printGuardReport(stdout *os.File, report guard.Report) {
	styler := newOutputStyler(stdout)
	fmt.Fprintf(stdout, "[qp guard: %s]\n\n", report.Guard)
	width := 0
	for _, step := range report.Steps {
		if len(step.Name) > width {
			width = len(step.Name)
		}
	}
	for _, step := range report.Steps {
		name := styler.taskName(step.Name)
		status := styler.statusBadge(step.Status)
		timing := styler.duration(fmt.Sprintf("%.1fs", float64(step.DurationMS)/1000))
		fmt.Fprintf(stdout, "  %-*s  %s  %s\n", width, name, status, timing)
	}

	fmt.Fprintln(stdout)
	finalDuration := styler.duration(fmt.Sprintf("in %.1fs", float64(report.DurationMS)/1000))
	if report.Overall == runner.StatusPass {
		fmt.Fprintf(stdout, "%s %s\n", styler.finalStatus(true), finalDuration)
	} else {
		fmt.Fprintf(stdout, "%s %s\n", styler.finalStatus(false), finalDuration)
	}

	for _, step := range report.Steps {
		if step.Stderr == "" {
			continue
		}
		fmt.Fprintf(stdout, "\n--- %s stderr ---\n%s", step.Name, step.Stderr)
		if !strings.HasSuffix(step.Stderr, "\n") {
			fmt.Fprintln(stdout)
		}
	}
}

func printTaskTimingSummary(stdout *os.File, result runner.Result) {
	if len(result.Steps) == 0 {
		return
	}
	total := len(result.Steps)
	passed := 0
	stepTimings := make([]string, 0, total)
	for _, step := range result.Steps {
		if step.Status == runner.StatusPass {
			passed++
		}
		duration := "n/a"
		if step.DurationMS != nil {
			duration = fmt.Sprintf("%.1fs", float64(*step.DurationMS)/1000)
		}
		stepTimings = append(stepTimings, fmt.Sprintf("%s: %s", step.Name, duration))
	}

	totalDuration := fmt.Sprintf("%.1fs", float64(result.DurationMS)/1000)
	if result.Status == runner.StatusPass {
		fmt.Fprintf(stdout, "\n%d tasks passed in %s (%s)\n", passed, totalDuration, strings.Join(stepTimings, ", "))
		return
	}
	fmt.Fprintf(stdout, "\n%d/%d tasks passed in %s (%s)\n", passed, total, totalDuration, strings.Join(stepTimings, ", "))
}

type listTask struct {
	Name     string               `json:"name"`
	Desc     string               `json:"desc"`
	Type     string               `json:"type"`
	Cache    bool                 `json:"cache,omitempty"`
	Parallel bool                 `json:"parallel,omitempty"`
	Needs    []string             `json:"needs,omitempty"`
	Steps    []string             `json:"steps,omitempty"`
	Scope    *string              `json:"scope"`
	Agent    bool                 `json:"agent"`
	Safety   string               `json:"safety"`
	Default  bool                 `json:"default,omitempty"`
	Aliases  []string             `json:"aliases,omitempty"`
	Groups   []string             `json:"groups,omitempty"`
	Params   map[string]listParam `json:"params,omitempty"`
}

type listParam struct {
	Desc     string `json:"desc,omitempty"`
	Env      string `json:"env"`
	Required bool   `json:"required,omitempty"`
	Default  string `json:"default,omitempty"`
	Position int    `json:"position,omitempty"`
	Variadic bool   `json:"variadic,omitempty"`
}

type listGroup struct {
	Name  string   `json:"name"`
	Desc  string   `json:"desc,omitempty"`
	Tasks []string `json:"tasks"`
}

func aliasesForTask(aliases map[string]string, taskName string) []string {
	names := []string{}
	for alias, target := range aliases {
		if target == taskName {
			names = append(names, alias)
		}
	}
	sort.Strings(names)
	return names
}

func isDefaultTask(cfg *config.Config, taskName string) bool {
	if cfg.Default == "" {
		return false
	}
	resolved, ok := cfg.ResolveTaskName(cfg.Default)
	return ok && resolved == taskName
}

func taskUsage(name string, task config.Task) string {
	parts := []string{"qp", name}
	seen := map[string]bool{}
	for _, paramName := range positionalParamNames(task.Params) {
		param := task.Params[paramName]
		label := "<" + paramName + ">"
		if param.Variadic {
			label = "<" + paramName + "...>"
		}
		if !param.Required || param.Default != "" {
			label = "[" + label + "]"
		}
		parts = append(parts, label)
		seen[paramName] = true
	}
	for _, paramName := range sortedParamNames(task.Params) {
		if seen[paramName] {
			continue
		}
		flag := "--" + paramName + " <value>"
		if task.Params[paramName].Required {
			parts = append(parts, flag)
			continue
		}
		parts = append(parts, "["+flag+"]")
	}
	parts = append(parts, "[--dry-run]", "[--no-cache]", "[--allow-unsafe]", "[--events]", "[--json]")
	return strings.Join(parts, " ")
}

func formatListSummary(item listTask) string {
	meta := []string{item.Type}
	if item.Default {
		meta = append(meta, "default")
	}
	if item.Scope != nil && *item.Scope != "" {
		meta = append(meta, "scope:"+*item.Scope)
	}
	if len(item.Aliases) > 0 {
		meta = append(meta, "aliases:"+strings.Join(item.Aliases, ","))
	}
	if len(item.Groups) > 0 {
		meta = append(meta, "groups:"+strings.Join(item.Groups, ","))
	}
	if len(item.Needs) > 0 {
		meta = append(meta, "needs:"+strings.Join(item.Needs, ","))
	}
	if len(item.Params) > 0 {
		params := make([]string, 0, len(item.Params))
		for _, name := range sortedListParamNames(item.Params) {
			param := item.Params[name]
			label := "--" + name
			if param.Position > 0 {
				label = "<" + name + ">"
				if param.Variadic {
					label = "<" + name + "...>"
				}
				if !param.Required || param.Default != "" {
					label = "[" + label + "]"
				}
			} else if !param.Required {
				label += "?"
			}
			params = append(params, label)
		}
		meta = append(meta, "params:"+strings.Join(params, ","))
	}
	if !item.Agent {
		meta = append(meta, "agent:false")
	}
	if item.Cache {
		meta = append(meta, "cache:true")
	}
	meta = append(meta, "safety:"+item.Safety)
	return strings.Join([]string{item.Desc, "[" + strings.Join(meta, " | ") + "]"}, " ")
}

func sortedListParamNames(params map[string]listParam) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left := params[names[i]]
		right := params[names[j]]
		if left.Position == 0 && right.Position > 0 {
			return false
		}
		if left.Position > 0 && right.Position == 0 {
			return true
		}
		if left.Position > 0 && right.Position > 0 && left.Position != right.Position {
			return left.Position < right.Position
		}
		return names[i] < names[j]
	})
	return names
}

func listGroups(groups map[string]config.Group) []listGroup {
	items := make([]listGroup, 0, len(groups))
	for _, name := range ordered.Keys(groups) {
		group := groups[name]
		items = append(items, listGroup{
			Name:  name,
			Desc:  group.Desc,
			Tasks: append([]string(nil), group.Tasks...),
		})
	}
	return items
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func unknownTaskError(name string, cfg *config.Config) error {
	suggestions := nearestTaskNames(name, cfg)
	if len(suggestions) == 0 {
		return fmt.Errorf("unknown task %q", name)
	}
	return fmt.Errorf("unknown task %q. Did you mean: %s?", name, strings.Join(suggestions, ", "))
}

func nearestTaskNames(name string, cfg *config.Config) []string {
	type candidate struct {
		name  string
		score int
	}

	candidates := []candidate{}
	for taskName := range cfg.Tasks {
		score := levenshtein(name, taskName)
		if strings.Contains(taskName, name) || strings.Contains(name, taskName) {
			score--
		}
		candidates = append(candidates, candidate{name: taskName, score: score})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].score < candidates[j].score
	})

	limit := 3
	if len(candidates) < limit {
		limit = len(candidates)
	}
	out := []string{}
	for i := 0; i < limit; i++ {
		if candidates[i].score > max(3, len(name)/2+1) {
			continue
		}
		out = append(out, candidates[i].name)
	}
	return out
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(current[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, current = current, prev
	}
	return prev[len(b)]
}
