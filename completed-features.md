# Completed Features

This file tracks implemented roadmap items and delivery depth.

Depth legend:

- `Scaffolded`: command surface/basic wiring exists.
- `Partial`: usable core behavior, not feature-complete.
- `Solid`: implemented and tested for current scope.

## P0

| Feature | Depth | Notes |
|---|---|---|
| Rename & Branding (`fkn` -> `qp`) | Solid | Binary/config/schema/module/docs/tests renamed; branding now `Quickly Please`. |
| Daemon Mode (Windows) | Partial | `qp daemon start/stop/status/restart`; `qp setup --windows`; named-pipe IPC server/client; PowerShell shim install; auto-proxy from normal invocations when daemon is running; shim now preserves streamed output and updates `$LASTEXITCODE` without terminating the shell. |
| Coloured & Formatted Output | Partial | Styled task/guard/error/watch output via lipgloss; `NO_COLOR` + `--no-color` support. |
| DAG Execution Syntax (`run`) | Solid | `par(...)`, `->`, nested refs, parser + validation + execution. |

## P1

| Feature | Depth | Notes |
|---|---|---|
| CEL Expression Engine | Partial | `internal/cel` evaluator with bool evaluation and validation helpers. |
| Conditional Branching | Partial | Task-level `when`; DAG-level `when(expr, if_true[, if_false])`. `switch(...)` not implemented yet. |
| NDJSON Event Stream | Partial | `--events` emits `plan/start/output/done/skipped/complete` for task/guard execution. |
| Variables | Partial | Top-level `vars` supported in task command/env interpolation and CEL eval context (`vars` / `var`). |
| Templates | Partial | Top-level `templates` string snippets supported via `{{template.<name>}}` interpolation. |
| Profiles | Partial | Top-level `profiles` overlays for vars and task `when`/`timeout`/`env`; selected via `QP_PROFILE`. |
| Task Caching / Skip | Partial | Opt-in `cache: true` for cmd tasks; local cache in `.qp/cache`; runtime bypass via `--no-cache`; cache-hit surfaced via skip/event signals. |

## In Progress / Not Yet

| Feature | Status |
|---|---|
| Harness Engineering Support | Not started |
| Scaffolding (harness-focused) | Not started |
| LLM Node Type | Not started |
| Shared State | Not started |
| CEL + LLM Evaluation | Not started |
| MCP Client Node Type | Not started |
| `until` Cycle Support | Not started |
| Expression Node Type | Not started |
| Approval Gate Node Type | Not started |
| Flows as MCP Tools | Not started |
| Declarative MCP Server Builder | Not started |
| Concurrency Control | Not started |
| Structured Logging | Not started |
| Context Dump | Not started |
| Secrets Management | Not started |
| Retry / Advanced Dry Run / Advanced Diff-Plan / Harness extras | Not started |
