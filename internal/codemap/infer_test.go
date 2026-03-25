package codemap

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInferGoPackage(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, "internal", "runner")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runner.go"), []byte(`package runner

// Package runner executes tasks and pipelines.
type Runner struct{}

func New() *Runner { return &Runner{} }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := Infer(repoRoot)
	if err != nil {
		t.Fatalf("Infer() error = %v", err)
	}
	pkg, ok := out["internal/runner"]
	if !ok {
		t.Fatalf("Infer() missing package internal/runner: %+v", out)
	}
	if pkg.Desc != "runner executes tasks and pipelines." {
		t.Fatalf("Desc = %q, want package doc text", pkg.Desc)
	}
	if !slices.Contains(pkg.KeyTypes, "Runner") {
		t.Fatalf("KeyTypes = %#v, want Runner", pkg.KeyTypes)
	}
	if !slices.Contains(pkg.EntryPoints, "New") {
		t.Fatalf("EntryPoints = %#v, want New", pkg.EntryPoints)
	}
}

func TestInferSkipsUnsupportedDirs(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, "node_modules", "pkg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(`export function init() {}`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := Infer(repoRoot)
	if err != nil {
		t.Fatalf("Infer() error = %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("Infer() = %+v, want empty (node_modules skipped)", out)
	}
}
