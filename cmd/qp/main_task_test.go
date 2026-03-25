package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTaskViaAlias(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: Build the project
    cmd: printf build
aliases:
  b: build
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"b"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(b) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "build") {
		t.Fatalf("stdout = %q, want task output", readStdout())
	}
}

func TestRunNoArgsUsesDefaultTask(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
default: verify
tasks:
  check:
    desc: Check the project
    cmd: printf check
aliases:
  verify: check
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run(nil, stdout, stderr)
	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "check") {
		t.Fatalf("stdout = %q, want default task output", readStdout())
	}
}

func TestRunFindsConfigInParentDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: printf ok
`), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "nested", "child")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	restore := chdirForTest(t, subdir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"test"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(test) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "ok") {
		t.Fatalf("stdout = %q, want task output", readStdout())
	}
}

func TestRunVerbExecutesTask(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
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

	code := run([]string{"run", "test"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(run test) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "ok") {
		t.Fatalf("stdout = %q, want task output", readStdout())
	}
}

func TestRunTaskAcceptsDirectParamFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  add-feature:
    desc: Add a feature
    cmd: printf {{params.feature}}
    params:
      feature:
        desc: Feature name
        env: FEATURE
        required: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"add-feature", "--feature", "auth"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(add-feature --feature auth) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "auth") {
		t.Fatalf("stdout = %q, want direct param flag output", got)
	}
}

func TestRunTaskAcceptsDirectParamEqualsFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  add-feature:
    desc: Add a feature
    cmd: printf {{params.feature}}
    params:
      feature:
        desc: Feature name
        env: FEATURE
        required: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"add-feature", "--feature=auth"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(add-feature --feature=auth) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "auth") {
		t.Fatalf("stdout = %q, want direct param flag output", got)
	}
}

func TestRunTaskAcceptsPositionalParams(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  build:
    desc: Build
    cmd: printf "%s|%s" "{{params.target}}" "{{params.profile}}"
    params:
      target:
        env: TARGET
        required: true
        position: 1
      profile:
        env: PROFILE
        default: debug
        position: 2
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"build", "app"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(build app) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "app|debug") {
		t.Fatalf("stdout = %q, want positional param output", got)
	}
}

func TestRunTaskAcceptsVariadicPositionalParam(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  pack:
    desc: Pack
    cmd: printf "%s|%s" "{{params.target}}" "{{params.files}}"
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

	code := run([]string{"pack", "release", "a.txt", "b.txt"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(pack release a.txt b.txt) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "release|a.txt b.txt") {
		t.Fatalf("stdout = %q, want variadic positional output", got)
	}
}

func TestRunTaskRejectsDuplicatePositionalAndNamedParam(t *testing.T) {
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
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"build", "app", "--target", "other"}, stdout, stderr)
	if code != 2 {
		t.Fatalf("run(build app --target other) code = %d, want 2; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStderr(), `param "target" was provided more than once`) {
		t.Fatalf("stderr = %q, want duplicate param error", readStderr())
	}
}

func TestRunUnsafeTaskRequiresAllowUnsafe(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  deploy:
    desc: Deploy
    cmd: printf deploy
    safety: external
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"deploy"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(deploy) code = 0, want failure")
	}
	if !strings.Contains(readStderr(), "--allow-unsafe") {
		t.Fatalf("stderr = %q, want allow-unsafe guidance", readStderr())
	}
}

func TestRunUnsafeTaskAllowsAllowUnsafe(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  deploy:
    desc: Deploy
    cmd: printf deploy
    safety: external
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"deploy", "--allow-unsafe"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(deploy --allow-unsafe) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "deploy") {
		t.Fatalf("stdout = %q, want deploy output", readStdout())
	}
}

func TestRunValidateReportsSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
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

	code := run([]string{"validate"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(validate) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "qp.yaml is valid") {
		t.Fatalf("stdout = %q, want validation success", got)
	}
}

func TestRunValidateJSONReportsFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  broken:
    desc: Broken
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"validate", "--json"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(validate --json) code = 0, want failure")
	}
	if !strings.Contains(readStderr(), `set exactly one of cmd, steps, or run`) {
		t.Fatalf("stderr = %q, want validation error", readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, `"valid": false`) || !strings.Contains(output, `"error":`) {
		t.Fatalf("stdout = %q, want JSON failure payload", output)
	}
}

func TestRunValidateSuggestReportsHints(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  lint:
    desc: Lint
    cmd: printf ok
  test:
    desc: Test
    cmd: printf ok
    scope: backend
guards:
  default:
    steps: [lint]
scopes:
  backend:
    paths:
      - internal/backend/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"validate", "--suggest"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(validate --suggest) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"qp.yaml is valid", "Suggestions:", "Tasks without scope", "Scopes without description", "Scoped tasks not covered by guards"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunValidateSuggestJSONIncludesSuggestions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  lint:
    desc: Lint
    cmd: printf ok
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"validate", "--json", "--suggest"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(validate --json --suggest) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, `"valid": true`) || !strings.Contains(output, `"suggestions":`) {
		t.Fatalf("stdout = %q, want JSON suggestions payload", output)
	}
}

func TestRunContextJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
project: demo
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

	code := run([]string{"context", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(context --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"project": "demo"`, `"repo_root":`, `"sections":`, `"markdown":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunTaskEmitsNDJSONEvents(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
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

	code := run([]string{"test", "--events"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(test --events) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "ok") {
		t.Fatalf("stdout = %q, want task output", got)
	}
	events := readStderr()
	for _, want := range []string{`"type":"plan"`, `"type":"start"`, `"type":"output"`, `"type":"done"`, `"type":"complete"`} {
		if !strings.Contains(events, want) {
			t.Fatalf("events = %q, want %s", events, want)
		}
	}
}

func TestRunTaskEventsReportLoadedEnvFileVars(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
env_file: .env
tasks:
  test:
    desc: Run tests
    cmd: printf ok
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"test", "--events"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(test --events) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStderr(); !strings.Contains(got, "loaded 1 vars from .env") {
		t.Fatalf("stderr = %q, want env file load feedback", got)
	}
}

func TestRunTaskVerbosePrintsResolvedCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
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

	code := run([]string{"test", "--verbose"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(test --verbose) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "ok") {
		t.Fatalf("stdout = %q, want task output", got)
	}
	if got := readStderr(); !strings.Contains(got, "[qp] test: printf ok") {
		t.Fatalf("stderr = %q, want verbose command preview", got)
	}
}

func TestRunPipelinePrintsTimingSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: printf test
  build:
    desc: Build app
    cmd: printf build
  check:
    desc: Run checks
    steps: [test, build]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"check"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(check) code = %d, want 0; stderr=%s", code, readStderr())
	}
	got := readStdout()
	if !strings.Contains(got, "tasks passed in") {
		t.Fatalf("stdout = %q, want pipeline timing summary", got)
	}
	if !strings.Contains(got, "test:") || !strings.Contains(got, "build:") {
		t.Fatalf("stdout = %q, want step timing entries", got)
	}
}

func TestRunPipelineQuietSuppressesTimingSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qp.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: printf test
  build:
    desc: Build app
    cmd: printf build
  check:
    desc: Run checks
    steps: [test, build]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"check", "--quiet"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(check --quiet) code = %d, want 0; stderr=%s", code, readStderr())
	}
	got := readStdout()
	if strings.Contains(got, "tasks passed in") {
		t.Fatalf("stdout = %q, want timing summary suppressed", got)
	}
}
