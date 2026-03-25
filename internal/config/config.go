package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	celpkg "github.com/neural-chilli/qp/internal/cel"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Project     string             `yaml:"project"`
	Description string             `yaml:"description"`
	Default     string             `yaml:"default"`
	Vars        map[string]string  `yaml:"vars"`
	Templates   map[string]string  `yaml:"templates"`
	Profiles    map[string]Profile `yaml:"profiles"`
	Defaults    DefaultsConfig     `yaml:"defaults"`
	EnvFile     string             `yaml:"env_file"`
	Tasks       map[string]Task    `yaml:"tasks"`
	Aliases     map[string]string  `yaml:"aliases"`
	Groups      map[string]Group   `yaml:"groups"`
	Guards      map[string]Guard   `yaml:"guards"`
	Scopes      map[string]Scope   `yaml:"scopes"`
	Prompts     map[string]Prompt  `yaml:"prompts"`
	Agent       AgentConfig        `yaml:"agent"`
	Context     ContextConfig      `yaml:"context"`
	Codemap     CodemapConfig      `yaml:"codemap"`
	Serve       ServeConfig        `yaml:"serve"`
	Watch       WatchConfig        `yaml:"watch"`
}

type Profile struct {
	Vars  map[string]string      `yaml:"vars"`
	Tasks map[string]ProfileTask `yaml:"tasks"`
}

type ProfileTask struct {
	When    string            `yaml:"when"`
	Timeout string            `yaml:"timeout"`
	Env     map[string]string `yaml:"env"`
}

type Task struct {
	Desc            string            `yaml:"desc"`
	Cmd             string            `yaml:"cmd"`
	Steps           []string          `yaml:"steps"`
	Run             string            `yaml:"run"`
	When            string            `yaml:"when"`
	Cache           *bool             `yaml:"cache"`
	Needs           []string          `yaml:"needs"`
	Parallel        bool              `yaml:"parallel"`
	Params          map[string]Param  `yaml:"params"`
	Env             map[string]string `yaml:"env"`
	Dir             string            `yaml:"dir"`
	Shell           string            `yaml:"shell"`
	ShellArgs       []string          `yaml:"shell_args"`
	Safety          string            `yaml:"safety"`
	ErrorFormat     string            `yaml:"error_format"`
	Timeout         string            `yaml:"timeout"`
	ContinueOnError bool              `yaml:"continue_on_error"`
	Agent           *bool             `yaml:"agent"`
	Scope           string            `yaml:"scope"`
}

type Param struct {
	Desc     string `yaml:"desc"`
	Env      string `yaml:"env"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
	Position int    `yaml:"position,omitempty"`
	Variadic bool   `yaml:"variadic,omitempty"`
}

type Guard struct {
	Steps []string `yaml:"steps"`
}

type Group struct {
	Desc  string   `yaml:"desc,omitempty"`
	Tasks []string `yaml:"tasks"`
}

type Scope struct {
	Desc  string   `yaml:"desc,omitempty"`
	Paths []string `yaml:"paths,omitempty"`
}

type Prompt struct {
	Desc     string `yaml:"desc"`
	Template string `yaml:"template"`
}

type AgentConfig struct {
	AccrueKnowledge bool `yaml:"accrue_knowledge"`
}

type DefaultsConfig struct {
	Dir string `yaml:"dir"`
}

type ContextConfig struct {
	FileTree     *bool       `yaml:"file_tree"`
	GitLogLines  int         `yaml:"git_log_lines"`
	GitDiff      bool        `yaml:"git_diff"`
	Todos        bool        `yaml:"todos"`
	Dependencies *bool       `yaml:"dependencies"`
	AgentFiles   []string    `yaml:"agent_files"`
	Files        []string    `yaml:"files"`
	Include      []string    `yaml:"include"`
	Exclude      []string    `yaml:"exclude"`
	Caps         ContextCaps `yaml:"caps"`
}

type ContextCaps struct {
	FileTreeEntries int `yaml:"file_tree_entries"`
	FilesMax        int `yaml:"files_max"`
	FileLines       int `yaml:"file_lines"`
	GitLogLines     int `yaml:"git_log_lines"`
	GitDiffLines    int `yaml:"git_diff_lines"`
	TodosMax        int `yaml:"todos_max"`
	AgentFileLines  int `yaml:"agent_file_lines"`
	DependencyLines int `yaml:"dependency_lines"`
}

type ServeConfig struct {
	Transport string `yaml:"transport"`
	Port      int    `yaml:"port"`
	TokenEnv  string `yaml:"token_env"`
}

type WatchConfig struct {
	DebounceMS int      `yaml:"debounce_ms"`
	Paths      []string `yaml:"paths"`
}

type CodemapConfig struct {
	Packages    map[string]CodemapPackage `yaml:"packages"`
	Conventions []string                  `yaml:"conventions"`
	Glossary    map[string]string         `yaml:"glossary"`
}

type CodemapPackage struct {
	Desc        string   `yaml:"desc"`
	KeyTypes    []string `yaml:"key_types"`
	EntryPoints []string `yaml:"entry_points"`
	Conventions []string `yaml:"conventions"`
	DependsOn   []string `yaml:"depends_on"`
}

func (s *Scope) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var paths []string
		if err := value.Decode(&paths); err != nil {
			return err
		}
		s.Paths = paths
		return nil
	case yaml.MappingNode:
		type rawScope Scope
		var raw rawScope
		if err := value.Decode(&raw); err != nil {
			return err
		}
		s.Desc = raw.Desc
		s.Paths = raw.Paths
		return nil
	default:
		return fmt.Errorf("scope must be a path list or mapping")
	}
}

func (t Task) AgentEnabled() bool {
	return t.Agent == nil || *t.Agent
}

func (t Task) CacheEnabled() bool {
	return t.Cache != nil && *t.Cache
}

func (t Task) SafetyLevel() string {
	if t.Safety != "" {
		return t.Safety
	}
	return "safe"
}

func (t Task) Type() string {
	if t.Cmd != "" {
		return "cmd"
	}
	return "pipeline"
}

func (c *Config) ResolveTaskName(name string) (string, bool) {
	if _, ok := c.Tasks[name]; ok {
		return name, true
	}
	target, ok := c.Aliases[name]
	if !ok {
		return "", false
	}
	_, ok = c.Tasks[target]
	return target, ok
}

func Load(path string) (*Config, error) {
	return LoadWithProfile(path, "")
}

func LoadWithProfile(path, profile string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}

	if profile != "" {
		if err := cfg.applyProfile(profile); err != nil {
			return nil, err
		}
	}

	cfg.applyDefaults()

	if err := cfg.Validate(filepath.Dir(path)); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyProfile(profile string) error {
	profileCfg, ok := c.Profiles[profile]
	if !ok {
		return fmt.Errorf("unknown profile %q", profile)
	}
	if c.Vars == nil {
		c.Vars = map[string]string{}
	}
	for name, value := range profileCfg.Vars {
		c.Vars[name] = value
	}
	for taskName, override := range profileCfg.Tasks {
		task, ok := c.Tasks[taskName]
		if !ok {
			return fmt.Errorf("profile %q references unknown task %q", profile, taskName)
		}
		if override.When != "" {
			task.When = override.When
		}
		if override.Timeout != "" {
			task.Timeout = override.Timeout
		}
		if len(override.Env) > 0 {
			if task.Env == nil {
				task.Env = map[string]string{}
			}
			for key, value := range override.Env {
				task.Env[key] = value
			}
		}
		c.Tasks[taskName] = task
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.Context.GitLogLines == 0 {
		c.Context.GitLogLines = 10
	}
	if c.Context.Caps.FileTreeEntries == 0 {
		c.Context.Caps.FileTreeEntries = 200
	}
	if c.Context.Caps.FilesMax == 0 {
		c.Context.Caps.FilesMax = 5
	}
	if c.Context.Caps.FileLines == 0 {
		c.Context.Caps.FileLines = 100
	}
	if c.Context.Caps.GitLogLines == 0 {
		c.Context.Caps.GitLogLines = 30
	}
	if c.Context.Caps.GitDiffLines == 0 {
		c.Context.Caps.GitDiffLines = 200
	}
	if c.Context.Caps.TodosMax == 0 {
		c.Context.Caps.TodosMax = 20
	}
	if c.Context.Caps.AgentFileLines == 0 {
		c.Context.Caps.AgentFileLines = 500
	}
	if c.Context.Caps.DependencyLines == 0 {
		c.Context.Caps.DependencyLines = 100
	}
	if c.Serve.Transport == "" {
		c.Serve.Transport = "stdio"
	}
	if c.Serve.Port == 0 {
		c.Serve.Port = 8080
	}
	if c.Serve.TokenEnv == "" {
		c.Serve.TokenEnv = "FKN_MCP_TOKEN"
	}
	if c.Watch.DebounceMS == 0 {
		c.Watch.DebounceMS = 500
	}
}

func (c *Config) Validate(repoRoot string) error {
	celEngine := celpkg.New()

	if len(c.Tasks) == 0 {
		return fmt.Errorf("qp.yaml must define at least one task")
	}

	if c.Defaults.Dir != "" {
		target := filepath.Join(repoRoot, c.Defaults.Dir)
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("defaults.dir %q: %w", c.Defaults.Dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("defaults.dir %q is not a directory", c.Defaults.Dir)
		}
	}

	for name, task := range c.Tasks {
		if task.Desc == "" {
			return fmt.Errorf("task %q: desc is required", name)
		}
		taskTypeCount := 0
		if task.Cmd != "" {
			taskTypeCount++
		}
		if len(task.Steps) > 0 {
			taskTypeCount++
		}
		if task.Run != "" {
			taskTypeCount++
		}
		if taskTypeCount != 1 {
			return fmt.Errorf("task %q: set exactly one of cmd, steps, or run", name)
		}
		if task.Run != "" && len(task.Needs) > 0 {
			return fmt.Errorf("task %q: run and needs are mutually exclusive", name)
		}
		if task.Run != "" {
			runExpr, err := ParseRunExpr(task.Run)
			if err != nil {
				return fmt.Errorf("task %q: invalid run expression: %w", name, err)
			}
			for _, ref := range RunExprRefs(runExpr) {
				if _, ok := c.Tasks[ref]; !ok {
					return fmt.Errorf("task %q references unknown run task %q", name, ref)
				}
			}
		}
		if task.When != "" {
			if err := celEngine.Validate(task.When); err != nil {
				return fmt.Errorf("task %q: invalid when expression: %w", name, err)
			}
		}
		if task.Dir != "" {
			target := filepath.Join(repoRoot, task.Dir)
			info, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("task %q: dir %q: %w", name, task.Dir, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("task %q: dir %q is not a directory", name, task.Dir)
			}
		}
		if task.Scope != "" {
			if _, ok := c.Scopes[task.Scope]; !ok {
				return fmt.Errorf("task %q references unknown scope %q", name, task.Scope)
			}
		}
		if task.Safety != "" {
			switch task.Safety {
			case "safe", "idempotent", "destructive", "external":
			default:
				return fmt.Errorf("task %q: unknown safety %q", name, task.Safety)
			}
		}
		for _, dep := range task.Needs {
			if _, ok := c.Tasks[dep]; !ok {
				return fmt.Errorf("task %q references unknown dependency %q", name, dep)
			}
		}
		if task.ErrorFormat != "" {
			switch task.ErrorFormat {
			case "go_test", "pytest", "tsc", "eslint", "generic":
			default:
				return fmt.Errorf("task %q: unknown error_format %q", name, task.ErrorFormat)
			}
		}
		for paramName, param := range task.Params {
			if isReservedParamName(paramName) {
				return fmt.Errorf("task %q param %q uses a reserved CLI flag name", name, paramName)
			}
			if param.Env == "" {
				return fmt.Errorf("task %q param %q: env is required", name, paramName)
			}
			if param.Position < 0 {
				return fmt.Errorf("task %q param %q: position must be greater than or equal to 1", name, paramName)
			}
		}
		if err := validateParamPositions(name, task.Params); err != nil {
			return err
		}
	}

	for name, entry := range c.Codemap.Packages {
		if entry.Desc == "" {
			return fmt.Errorf("codemap package %q: desc is required", name)
		}
	}

	for alias, target := range c.Aliases {
		if _, ok := c.Tasks[alias]; ok {
			return fmt.Errorf("alias %q conflicts with task of the same name", alias)
		}
		if _, ok := c.Tasks[target]; !ok {
			return fmt.Errorf("alias %q references unknown task %q", alias, target)
		}
	}

	if c.Default != "" {
		if _, ok := c.ResolveTaskName(c.Default); !ok {
			return fmt.Errorf("default task %q does not match a task or alias", c.Default)
		}
	}

	for name, guard := range c.Guards {
		if len(guard.Steps) == 0 {
			return fmt.Errorf("guard %q: steps are required", name)
		}
		for _, step := range guard.Steps {
			if _, ok := c.Tasks[step]; !ok {
				return fmt.Errorf("guard %q references unknown task %q", name, step)
			}
		}
	}

	for name, group := range c.Groups {
		if len(group.Tasks) == 0 {
			return fmt.Errorf("group %q: tasks are required", name)
		}
		for _, taskName := range group.Tasks {
			if _, ok := c.Tasks[taskName]; !ok {
				return fmt.Errorf("group %q references unknown task %q", name, taskName)
			}
		}
	}

	for name, prompt := range c.Prompts {
		if prompt.Desc == "" {
			return fmt.Errorf("prompt %q: desc is required", name)
		}
		if prompt.Template == "" {
			return fmt.Errorf("prompt %q: template is required", name)
		}
	}

	return c.validateCycles()
}

func isReservedParamName(name string) bool {
	switch name {
	case "dry-run", "json", "param":
		return true
	default:
		return false
	}
}

func validateParamPositions(taskName string, params map[string]Param) error {
	positions := map[int]string{}
	var variadicName string
	var variadicPosition int
	maxPosition := 0

	for name, param := range params {
		if param.Position == 0 {
			if param.Variadic {
				return fmt.Errorf("task %q param %q: variadic params must also declare a position", taskName, name)
			}
			continue
		}
		if param.Position < 1 {
			return fmt.Errorf("task %q param %q: position must be greater than or equal to 1", taskName, name)
		}
		if existing, ok := positions[param.Position]; ok {
			return fmt.Errorf("task %q params %q and %q share position %d", taskName, existing, name, param.Position)
		}
		positions[param.Position] = name
		if param.Position > maxPosition {
			maxPosition = param.Position
		}
		if param.Variadic {
			if variadicName != "" {
				return fmt.Errorf("task %q params %q and %q are both variadic", taskName, variadicName, name)
			}
			variadicName = name
			variadicPosition = param.Position
		}
	}

	if variadicName != "" && variadicPosition != maxPosition {
		return fmt.Errorf("task %q param %q: variadic param must have the highest position", taskName, variadicName)
	}
	return nil
}

func (c *Config) validateCycles() error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	stack := []string{}

	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			start := 0
			for i, item := range stack {
				if item == name {
					start = i
					break
				}
			}
			cycle := append(append([]string{}, stack[start:]...), name)
			return fmt.Errorf("circular task dependency: %s", joinArrow(cycle))
		}
		if visited[name] {
			return nil
		}

		task, ok := c.Tasks[name]
		if !ok {
			return fmt.Errorf("task %q references unknown task", name)
		}

		visiting[name] = true
		stack = append(stack, name)
		for _, dep := range task.Needs {
			if _, ok := c.Tasks[dep]; ok {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		for _, step := range task.Steps {
			if _, ok := c.Tasks[step]; ok {
				if err := visit(step); err != nil {
					return err
				}
			}
		}
		if task.Run != "" {
			runExpr, err := ParseRunExpr(task.Run)
			if err != nil {
				return fmt.Errorf("task %q: invalid run expression: %w", name, err)
			}
			for _, ref := range RunExprRefs(runExpr) {
				if _, ok := c.Tasks[ref]; ok {
					if err := visit(ref); err != nil {
						return err
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		visiting[name] = false
		visited[name] = true
		return nil
	}

	names := make([]string, 0, len(c.Tasks))
	for name := range c.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func joinArrow(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, part := range parts[1:] {
		out += " -> " + part
	}
	return out
}
