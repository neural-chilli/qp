package prompt

import (
	"runtime"
	"strings"
	"testing"

	"github.com/neural-chilli/qp/internal/config"
)

func TestRenderResolvesKnownPlaceholders(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prompts: map[string]config.Prompt{
			"test": {Desc: "test", Template: "OS is {{os}} at {{timestamp}}"},
		},
	}
	r := New(cfg, t.TempDir())

	rendered, warnings, err := r.Render("test")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if !strings.Contains(rendered, runtime.GOOS) {
		t.Fatalf("rendered = %q, want OS placeholder resolved", rendered)
	}
	if strings.Contains(rendered, "{{os}}") {
		t.Fatal("rendered still contains {{os}} placeholder")
	}
}

func TestRenderWarnsOnUnknownPlaceholder(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prompts: map[string]config.Prompt{
			"test": {Desc: "test", Template: "Hello {{unknown_var}}"},
		},
	}
	r := New(cfg, t.TempDir())

	rendered, warnings, err := r.Render("test")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("warnings = empty, want warning for unknown placeholder")
	}
	// Unknown placeholder should be left as-is.
	if !strings.Contains(rendered, "{{unknown_var}}") {
		t.Fatalf("rendered = %q, want unknown placeholder preserved", rendered)
	}
}

func TestRenderResolvesTaskDesc(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"build": {Desc: "Compile the project"},
		},
		Prompts: map[string]config.Prompt{
			"test": {Desc: "test", Template: "Task: {{task.build.desc}}"},
		},
	}
	r := New(cfg, t.TempDir())

	rendered, warnings, err := r.Render("test")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if !strings.Contains(rendered, "Compile the project") {
		t.Fatalf("rendered = %q, want task desc resolved", rendered)
	}
}

func TestRenderResolvesScopePaths(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Scopes: map[string]config.Scope{
			"api": {Desc: "API", Paths: []string{"internal/api/", "cmd/api/"}},
		},
		Prompts: map[string]config.Prompt{
			"test": {Desc: "test", Template: "Scope: {{scope.api}}"},
		},
	}
	r := New(cfg, t.TempDir())

	rendered, warnings, err := r.Render("test")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if !strings.Contains(rendered, "internal/api/") {
		t.Fatalf("rendered = %q, want scope paths", rendered)
	}
}

func TestRenderGitFailureReturnsEmpty(t *testing.T) {
	t.Parallel()

	// Use a non-git directory so git commands fail.
	cfg := &config.Config{
		Prompts: map[string]config.Prompt{
			"test": {Desc: "test", Template: "Branch: {{git_branch}}"},
		},
	}
	r := New(cfg, t.TempDir())

	rendered, warnings, err := r.Render("test")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	// git_branch should resolve to empty string (not trigger a warning).
	if strings.Contains(rendered, "{{git_branch}}") {
		t.Fatal("rendered still contains {{git_branch}} — git failure should return empty, not leave placeholder")
	}
}
