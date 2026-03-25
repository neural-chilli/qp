# Changelog

All notable changes to `qp` will be documented in this file.

The format is intentionally lightweight and based on tagged releases.

## [v0.4.0] - 2026-03-25

### Added

- `qp arch-check` with structured JSON output for architecture violations
- harness-oriented initialization via `qp init --harness`
- first-pass `architecture` config support (`layers`, `domains`, `rules`)
- README branding updates with logo and badges

### Changed

- CEL expression engine now uses registered `branch()` and `env()` functions instead of string rewriting
- `loadConfig()` now discovers `qp.yaml` by walking upward from subdirectories
- guard reporting now owns guard-specific result shaping (reduced runner coupling)
- default project guard/check pipeline now includes `go vet`

### Fixed

- schema now includes implemented fields (`run`, `vars`, `templates`, `profiles`, and architecture entries)
- serve token default renamed from legacy `FKN_MCP_TOKEN` to `QP_MCP_TOKEN`
- structured error extraction now falls back to generic parsing when declared parser has no matches
- repair diff cap now consistently follows `context.caps.git_diff_lines`

## [v0.3.2] - 2026-03-23

### Changed

- fixed README and docs links so they render correctly on GitHub
- added `go vet ./...` to CI
- expanded `qp.schema.json` field descriptions for better editor support

## [v0.3.1] - 2026-03-23

### Fixed

- fixed Makefile import so assigned Make variables are no longer inferred as required task params

## [v0.3.0] - 2026-03-23

### Added

- shell completions plus best-effort completion installation for bash, zsh, fish, and PowerShell
- broader `init --from-repo` coverage for Rust, Python, Java, Docker Compose, richer `justfile`, and smarter `package.json` inference
- generated repo docs via `qp init --docs` for `HUMANS.md`, `AGENTS.md`, and `CLAUDE.md`
- agent knowledge-accrual guidance via `agent.accrue_knowledge`

### Changed

- repositioned `qp` around the repo-interface and docs-generation story
- hid MCP from the public CLI and docs surface while keeping the code dormant internally

## [v0.2.0] - 2026-03-22

### Added

- event-driven watch mode via `fsnotify` with polling fallback
- nested pipeline execution
- execution-policy gating for `destructive` and `external` tasks
- `qp plan`, `qp diff-plan`, `qp repair`, and `qp agent-brief`
- codemap-backed explanations and topic-targeted context
- task safety annotations
- task groups, reusable task dependencies, aliases, explicit default tasks, richer params, task-level shell config, and global default working directory
- release artifact workflow for tagged builds

## [v0.1.1] - 2026-03-22

### Fixed

- stamped binary versions correctly for release and local stamped builds
- fixed CI formatting issues so GitHub Actions passed cleanly

## [v0.1.0] - 2026-03-22

### Added

- initial public release of Quickly Please
- YAML-driven task runner with command tasks and pipelines
- guards, scopes, prompts, context generation, watch mode, `init`, and validation
- JSON output for key commands
- repo-aware scaffolding from existing task surfaces
- generated docs and embedded offline docs
- initial release automation and install path via `go install`
