package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/neural-chilli/qp/internal/config"
)

func (r *Runner) runCommand(parent context.Context, label string, task config.Task, command string, opts Options, prefix string) (runOutcome, error) {
	started := time.Now()
	if opts.Verbose && !opts.Quiet && opts.Stderr != nil && !task.Silent {
		fmt.Fprintf(opts.Stderr, "[qp] %s: %s\n", label, command)
	}
	if opts.DryRun {
		if !task.Silent {
			fmt.Fprintln(opts.Stdout, command)
		}
		now := time.Now()
		return runOutcome{status: StatusPass, exitCode: 0, started: started, finished: now}, nil
	}

	ctx := parent
	var cancel context.CancelFunc
	if task.Timeout != "" {
		timeout, err := time.ParseDuration(task.Timeout)
		if err != nil {
			return runOutcome{}, fmt.Errorf("task %q: invalid timeout %q: %w", label, task.Timeout, err)
		}
		ctx, cancel = context.WithTimeout(parent, timeout)
		defer cancel()
	}

	shell, shellArgs := resolveShell(task)
	cmdArgs := append(append([]string{}, shellArgs...), command)
	cmd := exec.CommandContext(ctx, shell, cmdArgs...)
	cmd.Dir = r.resolveTaskDir(task)
	paramValues, err := resolveParamValues(task, opts.Params)
	if err != nil {
		return runOutcome{}, fmt.Errorf("task %q: %w", label, err)
	}
	cmd.Env = mergeEnv(os.Environ(), r.globalEnv, interpolateEnv(task.Env, paramValues, map[string]string(r.cfg.Vars), r.cfg.Templates.Snippets, r.secrets), opts.Env, paramEnv(task, paramValues))

	var stdoutBuf, stderrBuf bytes.Buffer
	redactor := newSecretRedactor(r.secrets)
	stdoutEvents := newEventLineWriter(label, "stdout", opts.Events, redactor)
	stderrEvents := newEventLineWriter(label, "stderr", opts.Events, redactor)
	cmd.Stdout = io.MultiWriter(&stdoutBuf, redactingWriter(prefixedWriter(prefix, opts.Stdout), redactor), stdoutEvents)
	cmd.Stderr = io.MultiWriter(&stderrBuf, redactingWriter(prefixedWriter(prefix, opts.Stderr), redactor), stderrEvents)

	err = cmd.Run()
	stdoutEvents.Flush()
	stderrEvents.Flush()
	finished := time.Now()
	if err == nil {
		return runOutcome{
			status:   StatusPass,
			exitCode: 0,
			stdout:   redactor.Redact(stdoutBuf.String()),
			stderr:   redactor.Redact(stderrBuf.String()),
			started:  started,
			finished: finished,
		}, nil
	}

	exitCode := 1
	status := StatusFail
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		exitCode = 124
		status = StatusTimeout
	} else if errors.Is(ctx.Err(), context.Canceled) {
		exitCode = 130
		status = StatusCancelled
	} else {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if errors.Is(err, exec.ErrNotFound) {
			exitCode = 127
		}
	}

	return runOutcome{
		status:   status,
		exitCode: exitCode,
		stdout:   redactor.Redact(stdoutBuf.String()),
		stderr:   redactor.Redact(stderrBuf.String()),
		started:  started,
		finished: finished,
	}, nil
}

func (r *Runner) runCommandWithRetry(parent context.Context, label string, task config.Task, command string, opts Options, prefix string) (runOutcome, error) {
	outcome, err := r.runCommand(parent, label, task, command, opts, prefix)
	if err != nil {
		return runOutcome{}, err
	}
	maxRetries := task.Retry
	if maxRetries <= 0 || outcome.status == StatusPass {
		return outcome, nil
	}

	delay := time.Duration(0)
	if task.RetryDelay != "" {
		parsed, err := time.ParseDuration(task.RetryDelay)
		if err != nil {
			return runOutcome{}, fmt.Errorf("task %q: invalid retry_delay %q: %w", label, task.RetryDelay, err)
		}
		delay = parsed
	}
	backoff := task.RetryBackoff
	if backoff == "" {
		backoff = "fixed"
	}
	conditions := task.RetryOn
	if len(conditions) == 0 {
		conditions = []string{"any"}
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		reason, ok := retryMatchReason(outcome, conditions)
		if !ok {
			return outcome, nil
		}
		wait := retryDelay(delay, backoff, attempt)
		if opts.Events != nil {
			opts.Events.EmitRetry(label, attempt, maxRetries, reason, wait.Milliseconds())
		}
		if wait > 0 {
			select {
			case <-parent.Done():
				return outcome, nil
			case <-time.After(wait):
			}
		}

		outcome, err = r.runCommand(parent, label, task, command, opts, prefix)
		if err != nil {
			return runOutcome{}, err
		}
		if outcome.status == StatusPass {
			return outcome, nil
		}
	}
	return outcome, nil
}

func retryMatchReason(outcome runOutcome, conditions []string) (string, bool) {
	for _, condition := range conditions {
		condition = strings.TrimSpace(condition)
		if condition == "" {
			continue
		}
		if condition == "any" {
			return fmt.Sprintf("exit_code:%d", outcome.exitCode), true
		}
		if strings.HasPrefix(condition, "exit_code:") {
			expected := strings.TrimPrefix(condition, "exit_code:")
			if fmt.Sprintf("%d", outcome.exitCode) == expected {
				return condition, true
			}
			continue
		}
		if strings.HasPrefix(condition, "stderr_contains:") {
			needle := strings.TrimPrefix(condition, "stderr_contains:")
			if needle != "" && strings.Contains(outcome.stderr, needle) {
				return condition, true
			}
		}
	}
	return "", false
}

func retryDelay(base time.Duration, backoff string, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	if backoff == "exponential" && attempt > 1 {
		return base * time.Duration(1<<(attempt-1))
	}
	return base
}

type eventLineWriter struct {
	task   string
	stream string
	events *EventStream
	redact *secretRedactor
	buf    []byte
}

func newEventLineWriter(task, stream string, events *EventStream, redactor *secretRedactor) *eventLineWriter {
	return &eventLineWriter{task: task, stream: stream, events: events, redact: redactor}
}

func (w *eventLineWriter) Write(p []byte) (int, error) {
	if w.events == nil || len(p) == 0 {
		return len(p), nil
	}
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.redact.Redact(string(w.buf[:i]))
		w.events.EmitOutput(w.task, w.stream, line)
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

func (w *eventLineWriter) Flush() {
	if w.events == nil || len(w.buf) == 0 {
		return
	}
	w.events.EmitOutput(w.task, w.stream, w.redact.Redact(string(w.buf)))
	w.buf = nil
}

func resolveParamValues(task config.Task, provided map[string]string) (map[string]string, error) {
	values := map[string]string{}
	for name, param := range task.Params {
		if provided != nil {
			if value, ok := provided[name]; ok {
				values[name] = value
				continue
			}
		}
		if param.Default != "" {
			values[name] = param.Default
			continue
		}
		if param.Required {
			return nil, fmt.Errorf("missing required param %q", name)
		}
	}
	for name, value := range provided {
		if _, ok := task.Params[name]; ok {
			values[name] = value
		}
	}
	return values, nil
}

func interpolateTaskValue(value string, params map[string]string, vars map[string]string, templates map[string]string, secrets map[string]string) string {
	out := value
	const maxInterpolationDepth = 10
	for i := 0; i < maxInterpolationDepth; i++ {
		prev := out
		for name, tpl := range templates {
			out = strings.ReplaceAll(out, "{{template."+name+"}}", tpl)
		}
		for name, paramValue := range params {
			out = strings.ReplaceAll(out, "{{params."+name+"}}", paramValue)
		}
		for name, varValue := range vars {
			out = strings.ReplaceAll(out, "{{vars."+name+"}}", varValue)
		}
		for name, secretValue := range secrets {
			out = strings.ReplaceAll(out, "{{secret."+name+"}}", secretValue)
		}
		if out == prev {
			return out
		}
	}
	return out
}

func interpolateEnv(env map[string]string, params map[string]string, vars map[string]string, templates map[string]string, secrets map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		out[key] = interpolateTaskValue(value, params, vars, templates, secrets)
	}
	return out
}

func paramEnv(task config.Task, params map[string]string) map[string]string {
	if len(task.Params) == 0 {
		return nil
	}
	out := map[string]string{}
	for name, param := range task.Params {
		if value, ok := params[name]; ok {
			out[param.Env] = value
		}
	}
	return out
}

func mergeEnv(base []string, layers ...map[string]string) []string {
	merged := map[string]string{}
	for _, item := range base {
		if key, value, ok := strings.Cut(item, "="); ok {
			merged[key] = value
		}
	}
	for _, layer := range layers {
		for key, value := range layer {
			merged[key] = value
		}
	}
	out := make([]string, 0, len(merged))
	for key, value := range merged {
		out = append(out, key+"="+value)
	}
	return out
}

func loadEnvFile(path string) (map[string]string, int, bool) {
	if path == "" {
		return nil, 0, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			env[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return env, len(env), true
}

func defaultShellCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func defaultShellArgs() []string {
	if runtime.GOOS == "windows" {
		return []string{"/C"}
	}
	return []string{"-c"}
}

func resolveShell(task config.Task) (string, []string) {
	command := defaultShellCommand()
	if task.Shell != "" {
		command = task.Shell
	}
	args := defaultShellArgs()
	if len(task.ShellArgs) > 0 {
		args = append([]string{}, task.ShellArgs...)
	}
	return command, args
}

func (r *Runner) resolveTaskDir(task config.Task) string {
	if task.Dir != "" {
		return filepath.Join(r.repoRoot, task.Dir)
	}
	if r.cfg.Defaults.Dir != "" {
		return filepath.Join(r.repoRoot, r.cfg.Defaults.Dir)
	}
	return r.repoRoot
}

func prefixedWriter(prefix string, target io.Writer) io.Writer {
	if target == nil || prefix == "" {
		return target
	}
	return &linePrefixWriter{prefix: "[" + prefix + "] ", target: target, atLineStart: true}
}

type linePrefixWriter struct {
	prefix      string
	target      io.Writer
	atLineStart bool
}

func (w *linePrefixWriter) Write(p []byte) (int, error) {
	if w.target == nil {
		return len(p), nil
	}
	written := 0
	for len(p) > 0 {
		if w.atLineStart {
			if _, err := io.WriteString(w.target, w.prefix); err != nil {
				return written, err
			}
			w.atLineStart = false
		}
		i := bytes.IndexByte(p, '\n')
		if i == -1 {
			n, err := w.target.Write(p)
			written += n
			return written, err
		}
		chunk := p[:i+1]
		n, err := w.target.Write(chunk)
		written += n
		if err != nil {
			return written, err
		}
		p = p[i+1:]
		w.atLineStart = true
	}
	return written, nil
}

type secretRedactor struct {
	values []string
}

func newSecretRedactor(secrets map[string]string) *secretRedactor {
	const minRedactionLength = 8
	values := make([]string, 0, len(secrets))
	for _, value := range secrets {
		if strings.TrimSpace(value) == "" || len(value) < minRedactionLength {
			continue
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return len(values[i]) > len(values[j])
	})
	return &secretRedactor{values: values}
}

func (r *secretRedactor) Redact(text string) string {
	if r == nil || len(r.values) == 0 || text == "" {
		return text
	}
	out := text
	for _, value := range r.values {
		out = strings.ReplaceAll(out, value, "***")
	}
	return out
}

type redactWriter struct {
	target   io.Writer
	redactor *secretRedactor
}

func redactingWriter(target io.Writer, redactor *secretRedactor) io.Writer {
	if target == nil {
		return nil
	}
	return &redactWriter{target: target, redactor: redactor}
}

func (w *redactWriter) Write(p []byte) (int, error) {
	if w.target == nil {
		return len(p), nil
	}
	redacted := p
	if w.redactor != nil {
		redacted = []byte(w.redactor.Redact(string(p)))
	}
	_, err := w.target.Write(redacted)
	return len(p), err
}

func strPtr(s string) *string {
	return &s
}
