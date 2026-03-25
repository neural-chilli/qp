package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/qp/internal/config"
)

func TestRunWithDocsWritesGeneratedDocs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	agents := "## Local Rules\n\nKeep this section.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{Docs: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"wrote HUMANS.md", "updated AGENTS.md", "wrote CLAUDE.md"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Run() message = %q, want %q", msg, want)
		}
	}

	humans, err := os.ReadFile(filepath.Join(dir, "HUMANS.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotHumans := string(humans)
	for _, want := range []string{
		"# HUMANS",
		"Default command: `qp check`",
		"`test`: Run the test suite",
		"## Groups",
		"## Scopes",
	} {
		if !strings.Contains(gotHumans, want) {
			t.Fatalf("HUMANS.md = %q, want %q", gotHumans, want)
		}
	}

	agentRoot, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotAgentRoot := string(agentRoot)
	if !strings.Contains(gotAgentRoot, "Keep this section.") {
		t.Fatalf("AGENTS.md = %q, want original content preserved", gotAgentRoot)
	}
	for _, want := range []string{
		"<!-- qp:agents:start -->",
		"# AGENTS",
		"## How To Work Here",
		"## Tasks",
	} {
		if !strings.Contains(gotAgentRoot, want) {
			t.Fatalf("AGENTS.md = %q, want %q", gotAgentRoot, want)
		}
	}

	claude, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotClaude := string(claude)
	for _, want := range []string{"# CLAUDE", "## How To Work Here", "## Tasks"} {
		if !strings.Contains(gotClaude, want) {
			t.Fatalf("CLAUDE.md = %q, want %q", gotClaude, want)
		}
	}
}

func TestRunWithDocsIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := Run(dir, Options{Docs: true}); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{Docs: true})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if !strings.Contains(msg, "updated AGENTS.md") {
		t.Fatalf("Run() message = %q, want AGENTS.md update note", msg)
	}
	after, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("AGENTS.md changed unexpectedly on second run:\nBEFORE:\n%s\nAFTER:\n%s", string(before), string(after))
	}
}

func TestRenderAgentDocIncludesKnowledgeAccrualGuidance(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Project:     "demo",
		Description: "Demo repo",
		Default:     "check",
		Agent: config.AgentConfig{
			AccrueKnowledge: true,
		},
		Tasks: map[string]config.Task{
			"check": {
				Desc:  "Run checks",
				Needs: []string{"setup"},
				Steps: []string{"test", "build"},
				Scope: "backend",
			},
			"setup": {
				Desc: "Prepare fixtures",
				Cmd:  "make setup",
			},
			"test": {
				Desc:   "Run tests",
				Cmd:    "go test ./...",
				Safety: "idempotent",
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test", "build"}},
		},
		Scopes: map[string]config.Scope{
			"backend": {Desc: "Backend workflows", Paths: []string{"cmd/", "internal/"}},
		},
		Prompts: map[string]config.Prompt{
			"continue-backend": {Desc: "Continue backend work"},
		},
		Groups: map[string]config.Group{
			"qa": {Desc: "Verification tasks", Tasks: []string{"test", "check"}},
		},
		Context: config.ContextConfig{
			AgentFiles: []string{"README.md"},
			Include:    []string{"cmd/", "internal/"},
		},
		Codemap: config.CodemapConfig{
			Packages: map[string]config.CodemapPackage{
				"internal/backend": {Desc: "Backend service layer"},
			},
			Conventions: []string{"Keep handlers thin"},
			Glossary: map[string]string{
				"tenant": "An isolated customer account",
			},
		},
		Watch: config.WatchConfig{
			DebounceMS: 500,
			Paths:      []string{"cmd/", "internal/"},
		},
	}

	got := renderAgentDoc("AGENTS", cfg)
	for _, want := range []string{
		"## One Command",
		"Run `qp guard` after making changes",
		"## Agent Conventions",
		"Always run `qp guard` before reporting work as complete.",
		"## Knowledge Accrual",
		"Build prerequisites or ordering -> task `needs`",
		"Package purpose or structure -> `codemap.packages`",
		"`check`: Run checks",
		"Scope Description: Backend workflows",
		"Needs: `setup`",
		"## Codemap",
		"## Watch",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderAgentDoc() = %q, want %q", got, want)
		}
	}
}

func TestRenderHumansDocIncludesPromptsAndCodemapWhenConfigured(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Project:     "demo",
		Description: "Demo repo",
		Default:     "check",
		Tasks: map[string]config.Task{
			"check": {Desc: "Run checks", Cmd: "make check"},
		},
		Prompts: map[string]config.Prompt{
			"continue-backend": {Desc: "Continue backend work"},
		},
		Codemap: config.CodemapConfig{
			Packages: map[string]config.CodemapPackage{
				"internal/backend": {Desc: "Backend service layer"},
			},
			Conventions: []string{"Keep handlers thin"},
			Glossary: map[string]string{
				"tenant": "An isolated customer account",
			},
		},
	}

	got := renderHumansDoc(cfg)
	for _, want := range []string{
		"## Prompts",
		"`continue-backend`: Continue backend work",
		"## Codemap",
		"`internal/backend`: Backend service layer",
		"Conventions:",
		"Glossary:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderHumansDoc() = %q, want %q", got, want)
		}
	}
}
