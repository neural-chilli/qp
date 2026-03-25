# qp User Guide

`qp` is a repo-local task runner and agent integration layer driven by a single `qp.yaml`.

This guide is the practical walkthrough: what to put in `qp.yaml`, how the commands behave, and what a realistic setup looks like.

## Installing qp

If you are evaluating whether `qp` should replace or sit on top of an existing Makefile, read [docs/why-not-make.md](why-not-make.md) too. The short version is that `qp` is designed to layer over existing repo commands, not force a rip-and-replace.

If you just want the CLI:

```bash
go install github.com/neural-chilli/qp/cmd/qp@latest
```

If you use Homebrew:

```bash
brew install neural-chilli/tap/qp
```

If you prefer not to install with Go, tagged releases now also publish prebuilt archives for macOS, Linux, and Windows on GitHub.

If you are working from a local checkout:

```bash
make build
./bin/qp list
```

For local cross-platform release artifacts:

```bash
make dist
```

You can read the bundled docs from the installed binary too:

```bash
qp docs
qp docs user-guide
qp docs --list
```

You can also enable shell completions:

```bash
qp completion bash
qp completion install
qp completion install --shell powershell
```

Supported shells today:

- `bash`
- `zsh`
- `fish`
- `PowerShell`

`qp completion <shell>` prints a completion script, and `qp completion install` does a best-effort install into the standard location for the current shell.

## Windows Daemon Mode

On Windows, `qp` supports daemon mode to avoid repeated process startup overhead on each invocation.

```bash
qp setup --windows
qp daemon status
```

Available commands:

- `qp daemon start`
- `qp daemon stop`
- `qp daemon status`
- `qp daemon restart`

## Bootstrapping An Existing Repo

If a repo already has a `Makefile`, `package.json`, or a `go.mod`, start here:

```bash
qp init --from-repo
```

That tells `qp` to infer a first pass from what the repo already exposes instead of dropping in the generic starter.

Current inference sources:

- common `Makefile` targets like `test`, `build`, `check`, and `lint`
- `justfile` and `Justfile`
- common `package.json` scripts like `test`, `build`, `check`, `lint`, `dev`, and `start`
- Go module repos via `go.mod`
- Rust repos via `Cargo.toml`
- Python repos via `pyproject.toml` and `tox.ini`
- Java repos via `pom.xml`, `build.gradle`, and `build.gradle.kts`
- Docker Compose via `docker-compose.yml`, `docker-compose.yaml`, `compose.yml`, or `compose.yaml`

For Makefiles and justfiles, `qp` now imports most regular targets instead of only a tiny built-in subset. It still skips obviously awkward scaffolding targets like `clean`, but it can now keep parameterized helper targets when it can infer env-style inputs such as `$(FEATURE)`.

For `justfile`s specifically, `qp` now also carries over common recipe aliases, positional/defaulted recipe parameters, and skips `_hidden` or `[private]` recipes so the generated config matches what humans usually invoke.

For `package.json`, `qp` now detects common `npm_config_*` patterns and turns them into declared task params. That means scripts like `vite build --mode=$npm_config_mode` can scaffold to a task that exposes `mode` explicitly instead of hiding it inside the raw npm script string.

For Rust repos, `qp` now scaffolds a practical starter set around `cargo fmt`, `cargo clippy`, `cargo test`, and `cargo build`.

For Python repos, `qp` can infer from `pyproject.toml` and `tox.ini`, including common setups around `pytest`, `tox`, `python -m build`, `ruff`, and `black`.

For Java repos, `qp` now scaffolds a simple Maven or Gradle starter set around `test` and `build`, preferring `./gradlew` when a Gradle wrapper is present.

For Docker Compose repos, `qp` scaffolds a few explicit service-control helpers like `compose-up`, `compose-down`, and `compose-logs`, and marks them as agent-hidden external tasks by default.

When imported tasks look like helper or repo-mutating workflows such as `clean`, `ci-init`, `release`, `deploy`, or parameterized scaffold commands, `qp` now tends to keep them in the generated config but mark them `agent: false`. That keeps the repo surface visible to humans without encouraging autonomous agent execution for risky helper tasks.

The goal is not to guess everything perfectly. The goal is to give you a believable first `qp.yaml` that humans and agents can edit confidently.

Treat `init --from-repo` as a strong starting point, not a source of truth. Import heuristics are intentionally best-effort and should be reviewed before you rely on the generated config.

If you also want repo-specific human and agent docs generated from `qp.yaml`:

```bash
qp init --docs
```

If you want a starter codemap inferred from source files:

```bash
qp init --codemap
```

That appends a generated `codemap.packages` block into `qp.yaml` for review and refinement.

If you want a harness-oriented starter with architecture guardrails from day one:

```bash
qp init --harness
```

That writes:

- `HUMANS.md` with a people-oriented repo workflow summary
- `AGENTS.md` with agent workflow guidance and managed `qp` sections
- `CLAUDE.md` with the same repo-specific guidance for Claude-style agent flows

The generated agent docs include:

- task summaries, including scopes, commands, and pipeline steps
- guard summaries
- scopes and prompts
- context and watch configuration highlights
- explicit "always/never" agent conventions and a required verification command
- a dedicated "One Command" section at the top for quick post-change validation

If you want agents to explicitly treat `qp.yaml` as an accrual surface for learned repo knowledge, enable:

```yaml
agent:
  accrue_knowledge: true
```

When enabled, the generated agent docs tell agents to propose structured updates back into `qp.yaml` for things like dependencies, params, codemap entries, conventions, and glossary terms.

## Validate Config

Use `qp validate` to check `qp.yaml` without running a task:

```bash
qp validate
qp validate --json
```

That is the simplest way to confirm a config edit before you try `list`, `guard`, `context`, or a task run.

`qp` looks for `qp.yaml` in the current directory and then walks upward through parent directories until it finds one. That means `qp test` works from nested folders inside the repo.

## Architecture Checks (Harness)

`qp arch-check` validates architecture boundaries defined in `qp.yaml`.

```bash
qp arch-check
qp arch-check --json
```

Current support is intentionally practical:

- configuration via top-level `architecture`
- layered/domain rules with:
  - `direction: forward`
  - `cross_domain: allow|deny`
  - `cross_cutting: <layer>`
- Go import analysis for in-repo module imports

Example:

```yaml
architecture:
  layers: [repo, service, api]
  domains:
    auth:
      root: src/auth
      layers: [repo, service, api]
    payments:
      root: src/payments
      layers: [repo, service, api]
  rules:
    - cross_domain: deny
```

`qp arch-check` exits non-zero on violations, so you can run it directly in local checks and CI.

## Editor Schema

The repo ships [qp.schema.json](../qp.schema.json) so editors can validate `qp.yaml` directly.

For YAML language server clients such as VS Code, add this at the top of `qp.yaml`:

```yaml
# yaml-language-server: $schema=./qp.schema.json
```

That gives you:

- field autocomplete
- enum suggestions such as `safety` and `error_format`
- inline validation for the common config shape

The schema is intentionally practical rather than exhaustive. It covers the public fields and common structural rules, while the CLI remains the final source of truth for runtime validation.

## Mental Model

Treat `qp.yaml` as the operational API of your repository.

That means one file defines:

- what tasks exist
- what aliases map onto those tasks
- how validation runs
- what scopes an agent should touch
- what prompts should render
- what context gets handed to an agent
- what workflow guidance agents should follow

If a repo has a `README`, `Makefile`, scattered scripts, and tribal knowledge, `qp` aims to turn that into one structured interface.

## Minimal Example

```yaml
project: my-service
description: Example Go service
default: check

tasks:
  test:
    desc: Run the test suite
    cmd: go test ./...

  build:
    desc: Build the binary
    cmd: go build -o bin/my-service ./cmd/my-service

  check:
    desc: Run local verification
    steps:
      - test
      - build
```

With that file in place:

```bash
qp list
qp test
qp run test
qp check
qp check --dry-run
qp test --json
```

When `default:` is set, running plain `qp` executes that task.

## Task Basics

Each task must have:

- a name
- a required `desc`
- exactly one of `cmd`, `steps`, or `run`

Single command task:

```yaml
tasks:
  lint:
    desc: Run static analysis
    cmd: golangci-lint run
```

Sequential pipeline:

```yaml
tasks:
  check:
    desc: Run local verification
    steps:
      - lint
      - test
      - build
```

Parallel pipeline:

```yaml
tasks:
  ci:
    desc: Run fast checks in parallel
    parallel: true
    steps:
      - lint
      - test
      - build
```

Nested pipeline:

```yaml
tasks:
  verify:
    desc: Shared verification workflow
    steps:
      - lint
      - test

  ci:
    desc: CI entrypoint
    steps:
      - verify
      - build
```

Nested pipeline steps now execute recursively, and the JSON output keeps the child step structure so agents can still see where failures happened inside the composed workflow.

`run` DAG syntax:

```yaml
tasks:
  release:
    desc: Release flow
    run: par(lint, test) -> build -> deploy
```

Supported operators:

- `par(a, b, c)` for parallel branches
- `a -> b -> c` for sequence
- `when(expr, if_true)` for conditional execution with skip fallback
- `when(expr, if_true, if_false)` for explicit false branch

If you omit `if_false`, the false path is skipped (that is the current default/fall-through behavior).

## Structured Errors

If a task emits stable compiler or test diagnostics, you can tell `qp` how to parse them:

```yaml
tasks:
  test:
    desc: Run Go tests
    cmd: go test ./...
    error_format: go_test
```

Supported formats today:

- `go_test`
- `pytest`
- `tsc`
- `eslint`
- `generic`

This does not replace raw stderr. It adds a parsed `errors` array to task JSON output, `guard --json`, and `repair --json` so agents do not have to scrape diagnostics back out of plain text.

Example:

```bash
qp test --json
qp guard --json
qp repair
qp repair --json
```

## CLI Output and Events

Useful global/runtime flags:

- `--no-color` disables styled terminal output.
- `--json` emits structured command output for machine consumers.
- `--events` emits NDJSON execution events.

For task and guard execution, `--events` emits:

- `plan`
- `start`
- `output`
- `done`
- `skipped`
- `complete`

When `env_file` is configured, `--events` mode also prints a one-line stderr note such as `loaded 3 vars from .env` before task execution.

Examples:

```bash
qp check --events 2>events.jsonl
qp check --events --json
qp guard --events
```

For agent/IDE tooling (including Copilot), prefer `--json` or `--events` over parsing styled terminal text.

On Windows daemon mode, `qp setup --windows` installs a PowerShell shim that preserves streamed output and updates command exit status via `$LASTEXITCODE`.

## Aliases

Aliases let you expose short or migration-friendly names for existing tasks.

Example:

```yaml
tasks:
  build:
    desc: Build the project
    cmd: go build ./...

aliases:
  b: build
  verify: build
```

Then either of these works:

```bash
qp b
qp help b
```

Aliases do not create separate tasks. They point at an existing task and reuse its behavior.

## Default Task

If you want `qp` on its own to do something useful, set a top-level default task:

```yaml
default: check

tasks:
  test:
    desc: Run tests
    cmd: go test ./...

  build:
    desc: Build the app
    cmd: go build ./...

  check:
    desc: Run the default local verification pipeline
    steps:
      - test
      - build
```

Then this works:

```bash
qp
```

The default can point at either a task name or an alias.

## Multi-File Task Config

Use top-level `includes` to split tasks across files:

```yaml
includes:
  - tasks/backend.yaml
  - tasks/frontend.yaml

tasks:
  check:
    desc: Run shared checks
    steps: [test-api, test-ui]
```

Current v1 include behavior:

- includes are resolved relative to the directory containing `qp.yaml`
- included files currently contribute `tasks` only
- duplicate task names across root and included files fail config load

## Reading Tasks Quickly

`qp list` is meant to be the fast discovery command.

Example output:

```text
check  Run local verification [pipeline | default | scope:cli]
build  Build the binary [cmd | aliases:b | params:<target>]
```

That summary view now includes:

- task type
- whether the task is the configured default
- scope
- aliases
- dependencies
- declared params
- whether a task is hidden from agents

For a single task, `qp help <task>` gives the fuller view, including a concrete usage line.

Example:

```text
build

Description: Build the binary
Aliases: b
Usage: qp build --target <value> [--dry-run] [--allow-unsafe] [--events] [--json]
Type: cmd
```

## Task Options

Tasks can also include:

- `needs`
- `when`
- `params`
- `env`
- `silent`
- `defer`
- `dir`
- `shell`
- `shell_args`
- `timeout`
- `continue_on_error`
- `agent`
- `scope`

Example:

```yaml
defaults:
  dir: services/api

tasks:
  setup:
    desc: Prepare generated assets
    cmd: make setup

  integration:
    desc: Run integration tests
    cmd: go test -tags=integration ./...
    silent: true
    defer: docker compose down
    needs:
      - setup
    dir: tools
    shell: /bin/sh
    shell_args:
      - -eu
      - -c
    timeout: 5m
    env:
      APP_ENV: test
    scope: backend
```

Notes:

- `needs` runs reusable prerequisite tasks before the task itself.
- dependencies can point at either command tasks or pipeline tasks.
- pipeline steps can point at either command tasks, inline shell steps, or other pipeline tasks.
- `defaults.dir` sets the global working directory for tasks that do not declare their own `dir`.
- task `dir` overrides `defaults.dir`.
- `shell` and `shell_args` let a task opt into a specific interpreter or shell mode.
- `continue_on_error` only affects sequential pipelines.
- Parallel pipelines still fail fast in the current implementation.
- `when` is a CEL expression. If it evaluates to `false`, the task is skipped.
- CEL context includes built-ins like `env(...)`, `branch()`, and `os` (`linux`, `darwin`, or `windows`).
- `silent: true` hides resolved command strings from dry-run and JSON output surfaces.
- `defer` runs after the main command whether the task passes or fails.
- `agent: false` marks a task as not intended for autonomous agent use.
- `safety` describes how cautious agents should be:
  - `safe` for read-only or low-risk tasks
  - `idempotent` for tasks that are safe to rerun repeatedly
  - `destructive` for tasks that mutate repo state or delete things
  - `external` for tasks that reach outside the repo, such as deploy or publish flows

Tasks marked `destructive` or `external` are blocked by default at execution time. To run them anyway:

```bash
qp deploy --allow-unsafe
qp guard --allow-unsafe
qp repair --allow-unsafe
```

`--dry-run` still works without that override, so risky tasks can be inspected before they are executed.

Example `when` usage:

```yaml
tasks:
  deploy:
    desc: Deploy only from main branch
    cmd: ./scripts/deploy.sh
    when: branch() == "main"

  open-docs:
    desc: Open docs on macOS only
    cmd: open docs/index.html
    when: os == "darwin"
```

## Variables, Templates, and Profiles

Current support:

- top-level `vars` values (static or shell-resolved)
- top-level `templates` string snippets
- top-level `profiles` with:
  - `vars` overrides
  - task overrides for `when`, `timeout`, and `env`

Example:

```yaml
vars:
  region: us-east-1
  git_sha:
    sh: "git rev-parse --short HEAD"

templates:
  deploy_cmd: ./scripts/deploy --region {{vars.region}} --sha {{vars.git_sha}}

tasks:
  deploy:
    desc: Deploy
    cmd: "{{template.deploy_cmd}}"

profiles:
  prod:
    vars:
      region: eu-west-1
    tasks:
      deploy:
        when: branch() == "main"
        timeout: 10m
        env:
          DEPLOY_ENV: production
```

Profile selection currently uses environment variable:

```bash
QP_PROFILE=prod qp deploy
```

For shell-resolved vars:

- `sh` commands run during config load (fail-fast if a command fails)
- commands run from the repo root (directory containing `qp.yaml`)
- stdout is trimmed and used as the var value

## Task Caching and Skip

`qp` now supports opt-in task result caching for command tasks.

Enable per task:

```yaml
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    cache:
      paths:
        - "**/*.go"
        - go.mod
        - go.sum
```

Behavior:

- cache storage is local: `.qp/cache/`
- cache currently applies to `cmd` tasks only
- cache key includes task config, resolved command, params, env overlays, working dir, selected profile, and optional content hash from `cache.paths`
- on cache hit, command execution is skipped and cached stdout/stderr is replayed

If you prefer the original behavior, `cache: true` is still supported and skips file content hashing.

Bypass cache for a run:

```bash
qp test --no-cache
qp guard --no-cache
```

## Task Dependencies

Use `needs` when one task should depend on another task but still remain its own command or pipeline.

Example:

```yaml
tasks:
  test:
    desc: Run tests
    cmd: go test ./...

  build:
    desc: Build the app
    cmd: go build ./...
    needs:
      - test

  release:
    desc: Build and publish a release
    cmd: ./scripts/release.sh
    needs:
      - build
      - check
```

`needs` is different from `steps`:

- `steps` makes the task itself a pipeline
- `needs` runs prerequisite tasks first, then runs the task itself

If a dependency fails, later dependencies and the main task are skipped.

After non-JSON pipeline runs, `qp` prints a one-line timing summary that includes total runtime and per-step durations.

## Task Params

Tasks can declare named or positional params that map to environment variables and can also be interpolated into commands via `{{params.<name>}}`.

Example:

```yaml
tasks:
  add-feature:
    desc: Add a feature scaffold
    cmd: make add-feature
    safety: destructive
    params:
      feature:
        desc: Feature name
        env: FEATURE
        required: true
```

Run it with:

```bash
qp add-feature --feature auth
qp add-feature --param feature=auth
```

Positional params use `position`, and the last positional param can be variadic:

```yaml
tasks:
  pack:
    desc: Pack release artifacts
    cmd: ./scripts/pack.sh {{params.target}} {{params.files}}
    params:
      target:
        env: TARGET
        required: true
        position: 1
      files:
        env: FILES
        position: 2
        variadic: true
```

Then this works naturally:

```bash
qp pack release dist/a.tgz dist/b.tgz
```

Template interpolation also works:

```yaml
tasks:
  greet:
    desc: Greet someone
    cmd: echo hello {{params.name}}
    params:
      name:
        desc: Person to greet
        env: NAME
        default: world
```

Notes:

- declared params can be passed as direct flags like `--feature auth` or `--feature=auth`
- repeated `--param name=value` still works
- positional params are assigned by ascending `position`
- a variadic param must be the last positional param
- required params fail fast if omitted
- safety is exposed in `qp help`, `qp list`, and generated agent docs

## Guards

Guards are validation-oriented step lists that always run all steps.

Example:

```yaml
guards:
  default:
    steps:
      - test
      - build
```

Run them with:

```bash
qp guard
qp guard default
qp guard --json
```

Why guards exist:

- task pipelines are for doing work
- guards are for reporting breakage

If one guard step fails, later steps still run so you get a fuller report.

Guard steps can now reference either cmd tasks or pipeline tasks, so you can reuse something like `check` directly instead of duplicating its underlying steps.

## Groups

Groups are named task families. They do not change execution semantics, but they make discovery and repo documentation much easier once a config grows beyond a handful of tasks.

Example:

```yaml
groups:
  qa:
    desc: Verification and quality checks.
    tasks:
      - test
      - lint
      - check
```

Use them with:

```bash
qp list
qp help qa
qp list --json
```

`qp list` shows grouped sections in the human-readable view, and `qp help <group>` prints the group description and member tasks.

## Scopes

Scopes are named path groups, and they can also carry a short description of why that part of the repo exists.

Example:

```yaml
scopes:
  cli:
    desc: Main CLI commands and the execution packages they depend on.
    paths:
      - cmd/qp/
      - internal/runner/
      - internal/context/
```

Use them with:

```bash
qp scope cli
qp scope cli --json
qp scope cli --format prompt
```

Tasks can reference them:

```yaml
tasks:
  fix-cli:
    desc: Work on the CLI
    cmd: go test ./cmd/qp ./internal/runner
    scope: cli
```

The older flat-list form still works:

```yaml
scopes:
  cli:
    - cmd/qp/
    - internal/runner/
```

If a scope has a description, `qp scope cli --format prompt`, `qp help <task>`, generated repair briefs, and generated agent docs include that intent alongside the path list.

## Version

Use:

```bash
qp version
qp version --json
```

`--json` is useful for scripts or agent checks that want structured version info without scraping plain text.

## Codemap

`codemap` lets you add a lightweight semantic layer to `qp.yaml` so agents can ask what a package or concept means without reading half the repo first.

Example:

```yaml
codemap:
  packages:
    internal/runner:
      desc: Core task execution engine
      key_types:
        - Runner
      entry_points:
        - Runner.Run
      conventions:
        - All execution flows through runCommand

  glossary:
    guard: A validation-oriented step list that always runs all steps
```

Use it with:

```bash
qp explain internal/runner
qp explain Runner.Run
qp explain guard --json
```

In agent mode, `qp context --agent --task <name>` now also includes matching codemap entries for the task scope when they exist.

## Prompts

Prompts let you keep reusable agent handoff templates in the repo.

Example:

```yaml
prompts:
  continue-cli:
    desc: Continue work on the CLI
    template: |
      Continue work on branch {{git_branch}}.
      OS: {{os}}
      Scope: {{scope.cli}}
      Check task: {{task.check.desc}}
      Current diff:
      {{git_diff}}
```

Render with:

```bash
qp prompt continue-cli
qp prompt continue-cli --copy
```

Supported variables today:

- `{{git_branch}}`
- `{{git_diff}}`
- `{{git_log}}`
- `{{scope.<name>}}`
- `{{task.<name>.desc}}`
- `{{timestamp}}`
- `{{os}}`

Unknown variables are left unchanged and emit a warning.

## Repair

`qp repair` is the shortest path from a failing guard to an agent-ready fix brief. It reruns the target guard, keeps the latest guard cache up to date, and emits either markdown or structured JSON.

Examples:

```bash
qp repair
qp repair default --json
qp repair --brief
```

Current repair output includes:

- current guard status and step timings
- failing steps only
- parsed structured errors when `error_format` is configured
- relevant task scopes and scoped paths
- current git diff, bounded by the context diff cap
- a suggested next action

`--brief` mode emits only failing tasks, parsed errors, and scoped paths for a compact agent handoff.

## Plan

`qp plan` is the pre-edit companion to `qp repair`.

It takes the files you expect to modify and returns the matching scopes, tasks, guards, groups, and codemap entries so you can decide what to read and rerun before touching code.

Examples:

```bash
qp plan --file cmd/qp/main.go --file internal/runner/runner.go
qp plan --json cmd/qp/main.go internal/runner/runner.go
```

The output is especially useful when you want to know:

- which tasks probably own the change
- which guard is the best verification target
- which codemap entries are likely relevant
- whether the impacted tasks are marked `safe`, `idempotent`, `destructive`, or `external`

`qp plan` is intentionally scope-first, so the quality of the output improves as task scopes and codemap entries become more accurate.

## Diff Plan

`qp diff-plan` is the git-aware companion to `qp plan`.

Instead of passing explicit file paths, it reads:

- unstaged tracked changes
- staged tracked changes
- untracked files

and then runs the same matching logic against scopes, tasks, guards, groups, and codemap entries.

Examples:

```bash
qp diff-plan
qp diff-plan --json
```

This is especially useful after an edit, when you want a quick answer to “what should I rerun now?” without manually listing the files you changed.

## Agent Brief

`qp agent-brief` is the higher-level handoff command. It reuses the existing context and planning features so you can generate one markdown or JSON payload for an agent instead of running multiple commands and stitching the results together yourself.

Examples:

```bash
qp agent-brief
qp agent-brief --task check
qp agent-brief --file cmd/qp/main.go
qp agent-brief --diff
qp agent-brief --json
```

Current behavior:

- with `--task <name>`, it emits the same task-focused context used by `qp context --agent --task <name>`
- with `--file` or positional file arguments, it includes the same impact plan produced by `qp plan`
- with `--diff`, it includes the same git-aware plan produced by `qp diff-plan`
- with no selector, it emits a broad repo brief using the standard context generator
- `--json` returns the structured context payload, the optional structured plan payload, and the combined markdown brief

## Context

`qp context` generates a bounded markdown briefing for humans or agents.

Default mode:

```bash
qp context
```

Agent mode:

```bash
qp context --agent --task check
qp context --about "task safety"
```

Topic mode:

```bash
qp context --about "task safety"
```

Structured mode:

```bash
qp context --json
```

Useful flags:

```bash
qp context --out /tmp/context.md
qp context --copy
qp context --max-tokens 500
```

`--max-tokens` is a rough character-based token estimate, not an exact model tokenizer count.

`--about <topic>` matches against task names and descriptions, scope names and paths, codemap package entries, and glossary terms, then builds a tighter context document around those matches.

The current implementation can include:

- project summary
- file tree
- task and guard summaries
- dependency manifests
- recent git log
- configured files and agent files
- current git diff in agent mode
- a structured JSON form via `--json` with section titles, bodies, and rendered markdown
- cached last guard output in agent mode

### Context Config

Example:

```yaml
context:
  file_tree: true
  git_log_lines: 10
  git_diff: false
  dependencies: true
  agent_files:
    - README.md
  include:
    - README.md
    - cmd/
    - internal/
    - qp.yaml
  exclude:
    - .git/
    - .idea/
    - bin/
  caps:
    file_tree_entries: 80
    file_lines: 80
    git_diff_lines: 120
    agent_file_lines: 120
```

Truncation is explicit. When a section is capped, `qp` inserts a marker like:

```text
[Lines 121–165 omitted]
```

That matters because an agent reading truncated file content should never assume the file ends at the truncation point.

## Init

`qp init` scaffolds a starter config.

```bash
qp init
qp init --harness
```

It currently:

- creates `qp.yaml` if it does not exist
- creates or updates `.gitignore`
- ensures `.qp/` is ignored
- leaves an existing `qp.yaml` unchanged
- with `--harness`, scaffolds a starter `architecture` section and `arch-check` task

## Watch

`qp watch` reruns a task or guard when watched files change.

It now uses filesystem events by default for faster reruns and lower idle overhead, with the older polling path kept as a fallback if event watching is unavailable.

Examples:

```bash
qp watch test
qp watch test --path README.md
qp watch guard
qp watch guard:default
```

Config:

```yaml
watch:
  debounce_ms: 500
  paths:
    - README.md
    - cmd/
    - internal/
    - qp.yaml
```

Behavior notes:

- default watch path is repo root if no configured or CLI paths are given
- `--path` overrides the configured list for that run
- a timestamp separator is printed before each rerun
- `.git/`, `.qp/`, and `bin/` are ignored

## Realistic Example

Here is a more realistic `qp.yaml` for a Go API:

```yaml
project: payments-api
description: Internal payments service

tasks:
  test:
    desc: Run unit tests
    cmd: go test ./...

  build:
    desc: Build the API binary
    cmd: go build -o bin/payments ./cmd/payments

  run:
    desc: Start the API locally
    cmd: go run ./cmd/payments
    env:
      APP_ENV: local

  check:
    desc: Run local verification
    steps:
      - test
      - build
    scope: backend

groups:
  core:
    desc: Everyday development and verification tasks.
    tasks:
      - test
      - build
      - run
      - check

guards:
  default:
    steps:
      - test
      - build

scopes:
  backend:
    desc: Payments API handlers, application wiring, and supporting internals.
    paths:
      - cmd/payments/
      - internal/

prompts:
  fix-backend-bug:
    desc: Backend bugfix prompt
    template: |
      Fix the bug in the backend.
      Scope: {{scope.backend}}
      Task: {{task.check.desc}}
      Branch: {{git_branch}}
      Diff:
      {{git_diff}}

context:
  file_tree: true
  dependencies: true
  agent_files:
    - README.md
  include:
    - README.md
    - cmd/
    - internal/
    - qp.yaml

watch:
  debounce_ms: 500
  paths:
    - cmd/
    - internal/
    - qp.yaml
```

Typical workflow:

```bash
qp init
qp list
qp check
qp guard
qp context --agent --task check
qp prompt fix-backend-bug
qp watch guard
```

## Troubleshooting

Common issues:

- `missing qp.yaml`
  Run `qp init` or create the file manually.

- `unknown task`
  Run `qp list` or `qp help <task>`. The CLI now suggests close task names.

- guard or task exits non-zero
  That is treated as failure, by design. `qp` does not hide broken commands behind soft statuses.

- command not found
  This is a broken environment, not a skip. Fix the underlying toolchain.

- prompt variable not rendering
  Unknown variables are left unchanged and produce a warning.


## Status

This guide reflects the tool as currently implemented in this repository. It is not a promise that every future feature in the PRD is already complete.
