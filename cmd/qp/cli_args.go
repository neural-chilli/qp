package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/neural-chilli/qp/internal/config"
)

func parseTaskInvocation(args []string, task config.Task) ([]string, map[string]string, error) {
	var taskArgs []string
	var positionals []string
	params := map[string]string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--var" {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s requires value", arg)
			}
			taskArgs = append(taskArgs, arg, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--var=") {
			taskArgs = append(taskArgs, arg)
			continue
		}
		if arg == "--param" {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--param requires name=value")
			}
			i++
			parts := strings.SplitN(args[i], "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return nil, nil, fmt.Errorf("--param requires name=value")
			}
			params[strings.TrimSpace(parts[0])] = parts[1]
			continue
		}
		name, value, ok, err := parseDirectParam(arg, args, i, task)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			params[name] = value
			if !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			taskArgs = append(taskArgs, arg)
			continue
		}
		positionals = append(positionals, arg)
	}

	if err := assignPositionalParams(positionals, params, task); err != nil {
		return nil, nil, err
	}
	return taskArgs, params, nil
}

func assignPositionalParams(positionals []string, params map[string]string, task config.Task) error {
	ordered := positionalParamNames(task.Params)
	if len(ordered) == 0 {
		if len(positionals) > 0 {
			return fmt.Errorf("unexpected positional arguments: %s", strings.Join(positionals, " "))
		}
		return nil
	}

	index := 0
	for _, name := range ordered {
		param := task.Params[name]
		if param.Variadic {
			if index >= len(positionals) {
				return nil
			}
			if _, exists := params[name]; exists {
				return fmt.Errorf("param %q was provided more than once", name)
			}
			params[name] = strings.Join(positionals[index:], " ")
			index = len(positionals)
			break
		}
		if index >= len(positionals) {
			break
		}
		if _, exists := params[name]; exists {
			return fmt.Errorf("param %q was provided more than once", name)
		}
		params[name] = positionals[index]
		index++
	}

	if index < len(positionals) {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(positionals[index:], " "))
	}
	return nil
}

func parseDirectParam(arg string, args []string, index int, task config.Task) (string, string, bool, error) {
	if !strings.HasPrefix(arg, "--") || arg == "--" {
		return "", "", false, nil
	}

	nameValue := strings.TrimPrefix(arg, "--")
	if nameValue == "json" || nameValue == "dry-run" || nameValue == "verbose" || nameValue == "quiet" || nameValue == "allow-unsafe" || nameValue == "events" || nameValue == "no-cache" || nameValue == "var" {
		return "", "", false, nil
	}

	if strings.Contains(nameValue, "=") {
		parts := strings.SplitN(nameValue, "=", 2)
		if _, ok := task.Params[parts[0]]; ok {
			return parts[0], parts[1], true, nil
		}
		return "", "", false, nil
	}

	if _, ok := task.Params[nameValue]; !ok {
		return "", "", false, nil
	}
	if index+1 >= len(args) {
		return "", "", false, fmt.Errorf("missing value for --%s", nameValue)
	}
	return nameValue, args[index+1], true, nil
}

func sortedParamNames(params map[string]config.Param) []string {
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

func positionalParamNames(params map[string]config.Param) []string {
	names := make([]string, 0, len(params))
	for name, param := range params {
		if param.Position > 0 {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		left := params[names[i]]
		right := params[names[j]]
		if left.Position != right.Position {
			return left.Position < right.Position
		}
		return names[i] < names[j]
	})
	return names
}

func parseSubcommandArgs(args []string, flags map[string]bool) ([]string, error) {
	// We want users to be able to place flags before or after positionals, which
	// the stdlib flag package does not support cleanly for our subcommand UX.
	var flagArgs []string
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		name := arg
		value := ""
		hasInlineValue := false
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			name = parts[0]
			value = parts[1]
			hasInlineValue = true
		}

		takesValue, ok := flags[name]
		if !ok {
			return nil, fmt.Errorf("unknown flag %q", name)
		}

		flagArgs = append(flagArgs, name)
		if takesValue {
			if hasInlineValue {
				flagArgs = append(flagArgs, value)
				continue
			}
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing value for %s", name)
			}
			i++
			flagArgs = append(flagArgs, args[i])
			continue
		}
		if hasInlineValue {
			return nil, fmt.Errorf("flag %s does not take a value", name)
		}
	}

	return append(flagArgs, positionals...), nil
}

func copyToClipboard(value string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("clipboard copy is unavailable on this system")
		}
	}

	cmd.Stdin = strings.NewReader(value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("copy to clipboard failed: %w", err)
	}
	return nil
}
