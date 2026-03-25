<table>
  <tr>
    <td width="200" valign="top">
      <img src="./qp-logo-rounded.png" alt="Quickly Please logo" width="220">
    </td>
    <td valign="top">
      <h1>Quickly Please (qp)</h1>
      <p>
        <a href="https://github.com/neural-chilli/qp/actions/workflows/ci.yml"><img src="https://github.com/neural-chilli/qp/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
        <a href="https://github.com/neural-chilli/qp/actions/workflows/release.yml"><img src="https://github.com/neural-chilli/qp/actions/workflows/release.yml/badge.svg" alt="Release"></a>
        <a href="https://github.com/neural-chilli/qp/releases"><img src="https://img.shields.io/github/v/release/neural-chilli/qp" alt="Latest Release"></a>
        <a href="https://go.dev/"><img src="https://img.shields.io/badge/language-Go-00ADD8?logo=go&logoColor=white" alt="Language"></a>
        <a href="https://github.com/neural-chilli/qp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/neural-chilli/qp" alt="License"></a>
      </p>
      <p><code>qp</code> is a local-first task runner for humans and agents, driven by a single <code>qp.yaml</code>.</p>
    </td>
  </tr>
</table>

## Why `qp`

- One config file (`qp.yaml`) for the repo's runnable interface
- Fast CLI for humans (`list`, `help`, `guard`, `watch`, `plan`, `repair`)
- Structured output for agents (`--json`, `--events` NDJSON stream)
- Safe execution model (`safety`, `--allow-unsafe`, `agent: false`)
- DAG execution with `run` expressions and CEL-based `when` branching
- Reusable config primitives (`vars`, `templates`, `profiles`)
- Harness primitives (`architecture`, `qp arch-check`, `qp init --harness`)

## Install

```bash
go install github.com/neural-chilli/qp/cmd/qp@latest
```

or build locally:

```bash
make build
./bin/qp list
```

## Quick Start

```bash
qp init
qp list
qp help check
qp check
qp guard
```

## Minimal `qp.yaml`

```yaml
project: my-service
default: check

tasks:
  lint:
    desc: Lint code
    cmd: golangci-lint run

  test:
    desc: Run tests
    cmd: go test ./...

  check:
    desc: Local verification
    run: par(lint, test)
```

## Branching and DAGs

`qp` supports DAG composition in `run`:

```yaml
tasks:
  release:
    desc: Build and deploy
    run: build -> when(branch() == "main", deploy, notify)
```

Also supported:

- `par(a, b, c)`
- `a -> b -> c`
- `when(expr, if_true)`
- `when(expr, if_true, if_false)`

Task-level branching is also supported:

```yaml
tasks:
  deploy:
    desc: Production deploy
    cmd: ./scripts/deploy.sh
    when: env("DEPLOY") == "1"
```

## Events and Output

- `--json` emits structured command output
- `--events` emits NDJSON runtime events (`plan`, `start`, `output`, `done`, `skipped`, `complete`)
- `--no-color` disables terminal styling

Examples:

```bash
qp check --json
qp check --events 2>events.jsonl
qp guard --events
```

## Windows Daemon Mode

For Windows AV-heavy environments:

```bash
qp setup --windows
qp daemon status
```

Available daemon commands:

- `qp daemon start`
- `qp daemon stop`
- `qp daemon status`
- `qp daemon restart`

## Docs

- [User Guide](docs/user-guide.md)
- [Why Not Just Use make?](docs/why-not-make.md)
- [Release Guide](docs/releasing.md)

## Status

`qp` is under active development. The user guide tracks implemented behavior in this repo.
