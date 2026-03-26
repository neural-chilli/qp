package runner

import (
	"io"
	"testing"

	"github.com/neural-chilli/qp/internal/config"
)

func TestExecutionContextThreadsThroughPipeline(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"a":    {Desc: "step a", Cmd: "echo a"},
		"b":    {Desc: "step b", Cmd: "echo b"},
		"pipe": {Desc: "pipeline", Steps: []string{"a", "b"}},
	}}, repoRoot)

	result, err := r.Run("pipe", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("Steps = %d, want 2", len(result.Steps))
	}
}

func TestExecutionContextThreadsThroughRunExpr(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"a":   {Desc: "step a", Cmd: "echo a"},
		"b":   {Desc: "step b", Cmd: "echo b"},
		"dag": {Desc: "dag", Run: "a -> b"},
	}}, repoRoot)

	result, err := r.Run("dag", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
}

func TestNewExecutionContext(t *testing.T) {
	t.Parallel()

	ec := NewExecutionContext()
	if ec == nil {
		t.Fatal("NewExecutionContext() returned nil")
	}
	if ec.State == nil {
		t.Fatal("State is nil, want empty map")
	}
	if len(ec.State) != 0 {
		t.Fatalf("State has %d entries, want 0", len(ec.State))
	}
}
