package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadNamespacedIncludes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlDir := filepath.Join(dir, "tasks", "ml")
	if err := os.MkdirAll(mlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlDir, "training.yaml"), []byte(`
tasks:
  train:
    desc: Train model
    cmd: echo train
  preprocess:
    desc: Preprocess data
    cmd: echo preprocess
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  ml: tasks/ml/
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := cfg.Tasks["ml:train"]; !ok {
		t.Fatal("namespaced task ml:train missing")
	}
	if _, ok := cfg.Tasks["ml:preprocess"]; !ok {
		t.Fatal("namespaced task ml:preprocess missing")
	}
	if _, ok := cfg.Tasks["check"]; !ok {
		t.Fatal("root task check missing")
	}
}

func TestLoadNamespacedIncludesRewritesSteps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlDir := filepath.Join(dir, "tasks", "ml")
	if err := os.MkdirAll(mlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlDir, "pipeline.yaml"), []byte(`
tasks:
  all:
    desc: Run all
    steps: [train, evaluate]
  train:
    desc: Train
    cmd: echo train
  evaluate:
    desc: Evaluate
    cmd: echo evaluate
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  ml: tasks/ml/
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	allTask := cfg.Tasks["ml:all"]
	if len(allTask.Steps) != 2 {
		t.Fatalf("ml:all steps = %v, want 2 entries", allTask.Steps)
	}
	if allTask.Steps[0] != "ml:train" || allTask.Steps[1] != "ml:evaluate" {
		t.Fatalf("ml:all steps = %v, want [ml:train, ml:evaluate]", allTask.Steps)
	}
}

func TestLoadNamespacedIncludesRewritesNeeds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlDir := filepath.Join(dir, "tasks", "ml")
	if err := os.MkdirAll(mlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlDir, "tasks.yaml"), []byte(`
tasks:
  train:
    desc: Train
    cmd: echo train
    needs: [preprocess]
  preprocess:
    desc: Preprocess
    cmd: echo preprocess
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  ml: tasks/ml/
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	trainTask := cfg.Tasks["ml:train"]
	if len(trainTask.Needs) != 1 || trainTask.Needs[0] != "ml:preprocess" {
		t.Fatalf("ml:train needs = %v, want [ml:preprocess]", trainTask.Needs)
	}
}

func TestLoadNamespacedIncludesRewritesRunExpr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlDir := filepath.Join(dir, "tasks", "ml")
	if err := os.MkdirAll(mlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlDir, "flow.yaml"), []byte(`
tasks:
  pipeline:
    desc: Full pipeline
    run: preprocess -> train -> evaluate
  preprocess:
    desc: Preprocess
    cmd: echo preprocess
  train:
    desc: Train
    cmd: echo train
  evaluate:
    desc: Evaluate
    cmd: echo evaluate
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  ml: tasks/ml/
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	pipelineTask := cfg.Tasks["ml:pipeline"]
	if pipelineTask.Run != "ml:preprocess -> ml:train -> ml:evaluate" {
		t.Fatalf("ml:pipeline run = %q, want namespaced refs", pipelineTask.Run)
	}
}

func TestLoadGlobIncludes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "backend.yaml"), []byte(`
tasks:
  test:
    desc: Test backend
    cmd: echo test
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "frontend.yaml"), []byte(`
tasks:
  lint:
    desc: Lint frontend
    cmd: echo lint
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  - tasks/*.yaml
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := cfg.Tasks["test"]; !ok {
		t.Fatal("glob-included task test missing")
	}
	if _, ok := cfg.Tasks["lint"]; !ok {
		t.Fatal("glob-included task lint missing")
	}
}

func TestLoadDoubleStarGlobIncludes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	subDir := filepath.Join(dir, "tasks", "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "deep.yaml"), []byte(`
tasks:
  deep-task:
    desc: Deep task
    cmd: echo deep
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  - tasks/**/*.yaml
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := cfg.Tasks["deep-task"]; !ok {
		t.Fatalf("double-star glob task missing, got tasks: %v", taskNames(cfg))
	}
}

func TestLoadNamespacedIncludesMultipleFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlDir := filepath.Join(dir, "tasks", "ml")
	if err := os.MkdirAll(mlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlDir, "training.yaml"), []byte(`
tasks:
  train:
    desc: Train
    cmd: echo train
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlDir, "eval.yaml"), []byte(`
tasks:
  evaluate:
    desc: Evaluate
    cmd: echo eval
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  ml: tasks/ml/
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := cfg.Tasks["ml:train"]; !ok {
		t.Fatal("ml:train missing")
	}
	if _, ok := cfg.Tasks["ml:evaluate"]; !ok {
		t.Fatal("ml:evaluate missing")
	}
}

func TestLoadRejectsInvalidNamespace(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlDir := filepath.Join(dir, "tasks", "ml")
	if err := os.MkdirAll(mlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlDir, "t.yaml"), []byte(`
tasks:
  train:
    desc: Train
    cmd: echo train
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  "bad:ns": tasks/ml/
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want namespace validation error")
	}
	if !strings.Contains(err.Error(), "must not contain spaces or colons") {
		t.Fatalf("Load() error = %v, want namespace validation", err)
	}
}

func TestLoadGlobNoMatchErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  - tasks/*.yaml
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "qp.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want no-match error")
	}
	if !strings.Contains(err.Error(), "no files match") {
		t.Fatalf("Load() error = %v, want no-match message", err)
	}
}

func TestLoadNamespacedIncludesPreserveCrossNamespaceRefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlDir := filepath.Join(dir, "tasks", "ml")
	if err := os.MkdirAll(mlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// This task references "check" which is a root task, not in the ml namespace.
	// The reference should NOT be rewritten.
	if err := os.WriteFile(filepath.Join(mlDir, "tasks.yaml"), []byte(`
tasks:
  train:
    desc: Train
    cmd: echo train
    needs: [check]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
includes:
  ml: tasks/ml/
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	trainTask := cfg.Tasks["ml:train"]
	// "check" is NOT in the ml namespace, so should stay as "check"
	if len(trainTask.Needs) != 1 || trainTask.Needs[0] != "check" {
		t.Fatalf("ml:train needs = %v, want [check] (cross-namespace ref preserved)", trainTask.Needs)
	}
}

func TestIncludedTaskUsesRootTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Included file references a template defined in the root config.
	if err := os.WriteFile(filepath.Join(tasksDir, "backend.yaml"), []byte(`
tasks:
  backend-test:
    desc: Backend tests
    use: test-suite
    params:
      dir: backend
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
templates:
  test-suite:
    params:
      dir:
        type: string
        required: true
    tasks:
      lint:
        desc: "Lint {{param.dir}}"
        cmd: "echo lint {{param.dir}}"
      unit:
        desc: "Test {{param.dir}}"
        cmd: "echo test {{param.dir}}"
includes:
  - tasks/*.yaml
tasks:
  check:
    desc: Check
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "qp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Template expansion should produce backend-test:lint and backend-test:unit.
	if _, ok := cfg.Tasks["backend-test:lint"]; !ok {
		t.Fatalf("template-generated task backend-test:lint missing, got tasks: %v", taskNames(cfg))
	}
	if _, ok := cfg.Tasks["backend-test:unit"]; !ok {
		t.Fatalf("template-generated task backend-test:unit missing, got tasks: %v", taskNames(cfg))
	}
	// Verify template values were resolved.
	lint := cfg.Tasks["backend-test:lint"]
	if lint.Cmd != "echo lint backend" {
		t.Fatalf("backend-test:lint cmd = %q, want template-resolved value", lint.Cmd)
	}
}

func taskNames(cfg *Config) []string {
	var names []string
	for name := range cfg.Tasks {
		names = append(names, name)
	}
	return names
}
