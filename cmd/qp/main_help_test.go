package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/qp/internal/config"
)

func TestUnknownTaskErrorSuggestsNearestTask(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"check": {Desc: "Check", Cmd: "echo check"},
			"build": {Desc: "Build", Cmd: "echo build"},
		},
	}

	err := unknownTaskError("chek", cfg)
	if err == nil {
		t.Fatal("unknownTaskError() = nil, want error")
	}
	if !strings.Contains(err.Error(), "Did you mean: check?") {
		t.Fatalf("unknownTaskError() = %v, want suggestion", err)
	}
}

func TestRunHelpForTask(t *testing.T) {
	repoRoot := repoRootForTest(t)
	restore := chdirForTest(t, repoRoot)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "check"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help check) code = %d, want 0; stderr=%s", code, readStderr())
	}

	output := readStdout()
	if !strings.Contains(output, "Description: Run the default local verification pipeline") {
		t.Fatalf("stdout = %q, want task description", output)
	}
	if !strings.Contains(output, "Default: true") {
		t.Fatalf("stdout = %q, want default marker", output)
	}
	if !strings.Contains(output, "Scope Desc: Main CLI commands and closely-related execution packages.") {
		t.Fatalf("stdout = %q, want scope description", output)
	}
	if !strings.Contains(output, "Needs: fmt") {
		t.Fatalf("stdout = %q, want dependency detail", output)
	}
	if !strings.Contains(output, "Safety: idempotent") {
		t.Fatalf("stdout = %q, want safety", output)
	}
	if !strings.Contains(output, "Usage: qp check [--dry-run] [--verbose] [--quiet] [--no-cache] [--allow-unsafe] [--events] [--json]") {
		t.Fatalf("stdout = %q, want usage", output)
	}
	if !strings.Contains(output, "Steps:\n- test\n- vet\n- build") {
		t.Fatalf("stdout = %q, want steps", output)
	}
}

func TestRunUnknownTaskShowsSuggestion(t *testing.T) {
	repoRoot := repoRootForTest(t)
	restore := chdirForTest(t, repoRoot)
	defer restore()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"chek"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(chek) code = 0, want failure")
	}
	if !strings.Contains(readStderr(), `unknown task "chek". Did you mean: check`) {
		t.Fatalf("stderr = %q, want suggestion", readStderr())
	}
}

func TestRunHelpForAlias(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
default: b
tasks:
  test:
    desc: Run tests
    cmd: echo test
  build:
    desc: Build the project
    cmd: echo build
aliases:
  b: build
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "b"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help b) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, "Alias For: build") {
		t.Fatalf("stdout = %q, want alias target", output)
	}
	if !strings.Contains(output, "Default: true") {
		t.Fatalf("stdout = %q, want default marker", output)
	}
	if !strings.Contains(output, "Aliases: b") {
		t.Fatalf("stdout = %q, want aliases list", output)
	}
	if !strings.Contains(output, "Usage: qp b [--dry-run] [--verbose] [--quiet] [--no-cache] [--allow-unsafe] [--events] [--json]") {
		t.Fatalf("stdout = %q, want alias usage", output)
	}
}

func TestRunHelpForGroup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
  build:
    desc: Build the app
    cmd: echo build
groups:
  qa:
    desc: Verification tasks
    tasks:
      - test
      - build
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "qa"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help qa) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"group qa", "Description: Verification tasks", "- test: Run tests", "- build: Build the app"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunHelpShowsPositionalUsage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: Build
    cmd: printf ok
    params:
      target:
        env: TARGET
        required: true
        position: 1
      files:
        env: FILES
        position: 2
        variadic: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "build"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help build) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"Usage: qp build <target> [<files...>] [--dry-run] [--verbose] [--quiet] [--no-cache] [--allow-unsafe] [--events] [--json]", "positional[1]", "positional[2...]"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunHelpShowsShellConfiguration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  strict:
    desc: Run in strict shell mode
    cmd: printf ok
    shell: /bin/sh
    shell_args:
      - -eu
      - -c
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "strict"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help strict) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, "Shell: /bin/sh") {
		t.Fatalf("stdout = %q, want shell", output)
	}
	if !strings.Contains(output, "Shell Args: -eu -c") {
		t.Fatalf("stdout = %q, want shell args", output)
	}
}

func TestRunHelpShowsInheritedDefaultDir(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
defaults:
  dir: app
tasks:
  test:
    desc: Run tests
    cmd: printf ok
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "test"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help test) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, "Dir: app (inherited default)") {
		t.Fatalf("stdout = %q, want inherited default dir", output)
	}
}

func TestRunHelpShowsTaskSafety(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  deploy:
    desc: Deploy the service
    cmd: ./scripts/deploy.sh
    safety: external
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "deploy"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help deploy) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "Safety: external") {
		t.Fatalf("stdout = %q, want safety output", readStdout())
	}
}

func TestRunDocsPrintsEmbeddedGuide(t *testing.T) {
	t.Parallel()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"docs", "user-guide"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(docs user-guide) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "# qp User Guide") {
		t.Fatalf("stdout = %q, want embedded docs output", readStdout())
	}
}

func TestRunDocsList(t *testing.T) {
	t.Parallel()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"docs", "--list"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(docs --list) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"readme", "user-guide", "why-not-make", "releasing"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestResolvedVersionPrefersExplicitLdflagsValue(t *testing.T) {
	previous := version
	version = "v9.9.9"
	t.Cleanup(func() {
		version = previous
	})

	if got := resolvedVersion(); got != "v9.9.9" {
		t.Fatalf("resolvedVersion() = %q, want explicit version", got)
	}
}

func TestRunVersionJSON(t *testing.T) {
	previous := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = previous
	})

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"version", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(version --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, `"name": "qp"`) || !strings.Contains(output, `"version": "v1.2.3"`) {
		t.Fatalf("stdout = %q, want JSON version payload", output)
	}
}

func TestRunScopeAllowsFlagAfterPositional(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: printf ok
scopes:
  cli:
    desc: CLI command surface
    paths:
      - cmd/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"scope", "cli", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(scope cli --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), `"scope": "cli"`) {
		t.Fatalf("stdout = %q, want JSON scope output", readStdout())
	}
	if !strings.Contains(readStdout(), `"desc": "CLI command surface"`) {
		t.Fatalf("stdout = %q, want scope description", readStdout())
	}
}

func TestRunScopePromptIncludesScopeIntent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: printf ok
scopes:
  cli:
    desc: CLI command surface
    paths:
      - cmd/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"scope", "cli", "--format", "prompt"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(scope cli --format prompt) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"Only modify files in the declared scope `cli`: cmd/", "Scope intent: CLI command surface"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunScopeCoverageJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal", "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "internal", "orphan"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "api", "handler.go"), []byte("package api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "orphan", "job.go"), []byte("package orphan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: printf ok
scopes:
  api:
    desc: API scope
    paths:
      - internal/api/
`), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"scope", "--coverage", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(scope --coverage --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"covered": [`, `"internal/api"`, `"orphaned": [`, `"internal/orphan"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunDocsRejectsUnknownFlag(t *testing.T) {
	t.Parallel()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"docs", "--wat", "user-guide"}, stdout, stderr)
	if code != 2 {
		t.Fatalf("run(docs --wat user-guide) code = %d, want 2; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStderr(), `unknown flag "--wat"`) {
		t.Fatalf("stderr = %q, want unknown flag error", readStderr())
	}
}
