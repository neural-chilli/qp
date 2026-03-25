package cel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	celgo "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Eval(expression string, vars map[string]any) (any, error) {
	env, err := e.newEnv(vars)
	if err != nil {
		return nil, err
	}

	ast, iss := env.Parse(expression)
	if iss != nil && iss.Err() != nil {
		return nil, iss.Err()
	}
	checked, iss := env.Check(ast)
	if iss != nil && iss.Err() != nil {
		return nil, iss.Err()
	}

	program, err := env.Program(checked)
	if err != nil {
		return nil, err
	}
	out, _, err := program.Eval(vars)
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

func (e *Engine) Validate(expression string) error {
	env, err := e.newEnv(nil)
	if err != nil {
		return err
	}
	_, iss := env.Parse(expression)
	if iss != nil && iss.Err() != nil {
		return iss.Err()
	}
	return nil
}

func (e *Engine) EvalBool(expression string, vars map[string]any) (bool, error) {
	value, err := e.Eval(expression, vars)
	if err != nil {
		return false, err
	}
	boolean, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("expression %q did not evaluate to bool", expression)
	}
	return boolean, nil
}

func sortedNames(vars map[string]any) []string {
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (e *Engine) newEnv(vars map[string]any) (*celgo.Env, error) {
	envOpts := make([]celgo.EnvOption, 0, len(vars)+2)
	for _, name := range sortedNames(vars) {
		envOpts = append(envOpts, celgo.Variable(name, celgo.DynType))
	}
	envOpts = append(envOpts,
		celgo.Function("branch",
			celgo.Overload(
				"qp_branch_no_args",
				[]*celgo.Type{},
				celgo.StringType,
				celgo.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.String(branchValue(vars))
				}),
			),
		),
		celgo.Function("profile",
			celgo.Overload(
				"qp_profile_no_args",
				[]*celgo.Type{},
				celgo.StringType,
				celgo.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.String(profileValue(vars))
				}),
			),
		),
		celgo.Function("env",
			celgo.Overload(
				"qp_env_lookup",
				[]*celgo.Type{celgo.StringType},
				celgo.StringType,
				celgo.UnaryBinding(func(arg ref.Val) ref.Val {
					return types.String(envValue(vars, fmt.Sprint(arg.Value())))
				}),
			),
		),
		celgo.Function("tag",
			celgo.Overload(
				"qp_tag_no_args",
				[]*celgo.Type{},
				celgo.StringType,
				celgo.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.String(tagValue(vars))
				}),
			),
		),
		celgo.Function("param",
			celgo.Overload(
				"qp_param_lookup",
				[]*celgo.Type{celgo.StringType},
				celgo.StringType,
				celgo.UnaryBinding(func(arg ref.Val) ref.Val {
					return types.String(paramValue(vars, fmt.Sprint(arg.Value())))
				}),
			),
		),
		celgo.Function("has_param",
			celgo.Overload(
				"qp_has_param_lookup",
				[]*celgo.Type{celgo.StringType},
				celgo.BoolType,
				celgo.UnaryBinding(func(arg ref.Val) ref.Val {
					return types.Bool(hasParamValue(vars, fmt.Sprint(arg.Value())))
				}),
			),
		),
		celgo.Function("file_exists",
			celgo.Overload(
				"qp_file_exists_lookup",
				[]*celgo.Type{celgo.StringType},
				celgo.BoolType,
				celgo.UnaryBinding(func(arg ref.Val) ref.Val {
					return types.Bool(fileExistsValue(vars, fmt.Sprint(arg.Value())))
				}),
			),
		),
	)
	return celgo.NewEnv(envOpts...)
}

func branchValue(vars map[string]any) string {
	if vars == nil {
		return ""
	}
	value, ok := vars["branch"]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func envValue(vars map[string]any, key string) string {
	if vars == nil {
		return ""
	}
	value, ok := vars["env"]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case map[string]string:
		return typed[key]
	case map[string]any:
		if found, ok := typed[key]; ok && found != nil {
			return fmt.Sprint(found)
		}
	}
	return ""
}

func profileValue(vars map[string]any) string {
	if vars == nil {
		return ""
	}
	value, ok := vars["profile"]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func tagValue(vars map[string]any) string {
	if vars == nil {
		return ""
	}
	value, ok := vars["tag"]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func paramValue(vars map[string]any, key string) string {
	if vars == nil {
		return ""
	}
	value, ok := vars["params"]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case map[string]string:
		return typed[key]
	case map[string]any:
		if found, ok := typed[key]; ok && found != nil {
			return fmt.Sprint(found)
		}
	}
	return ""
}

func hasParamValue(vars map[string]any, key string) bool {
	if vars == nil {
		return false
	}
	value, ok := vars["params"]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case map[string]string:
		_, ok := typed[key]
		return ok
	case map[string]any:
		_, ok := typed[key]
		return ok
	}
	return false
}

func fileExistsValue(vars map[string]any, p string) bool {
	if strings.TrimSpace(p) == "" {
		return false
	}
	target := p
	if vars != nil {
		if root, ok := vars["repo_root"]; ok && root != nil {
			rootPath := fmt.Sprint(root)
			if rootPath != "" && !filepath.IsAbs(target) {
				target = filepath.Join(rootPath, target)
			}
		}
	}
	_, err := os.Stat(target)
	return err == nil
}
