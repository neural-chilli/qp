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
        <a href="https://github.com/neural-chilli/qp/actions/workflows/docs.yml"><img src="https://github.com/neural-chilli/qp/actions/workflows/docs.yml/badge.svg" alt="Docs"></a>
        <a href="https://neural-chilli.github.io/qp/"><img src="https://img.shields.io/badge/docs-manual-1f6feb" alt="Docs Site"></a>
        <a href="https://github.com/neural-chilli/qp/releases"><img src="https://img.shields.io/github/v/release/neural-chilli/qp" alt="Latest Release"></a>
        <a href="https://go.dev/"><img src="https://img.shields.io/badge/language-Go-00ADD8?logo=go&logoColor=white" alt="Language"></a>
        <a href="https://github.com/neural-chilli/qp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/neural-chilli/qp" alt="License"></a>
      </p>
      <p><code>qp</code> is a local-first task runner and workflow runtime for humans and agents, driven by one <code>qp.yaml</code>.</p>
      <p><strong>Docs:</strong> <a href="https://neural-chilli.github.io/qp/">Manual</a> · <a href="docs/user-guide.md">User Guide (repo)</a></p>
    </td>
  </tr>
</table>

## Quickstart

Install:

```bash
go install github.com/neural-chilli/qp/cmd/qp@latest
```

Initialize in a repo:

```bash
qp init
qp list
qp help check
qp check
qp guard
```

Minimal `qp.yaml`:

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

## Common Commands

```bash
qp list
qp help <task>
qp <task>
qp guard
qp watch guard
qp check --json
qp check --events 2>events.jsonl
```

## Where To Go Next

- Full docs/manual: [https://neural-chilli.github.io/qp/](https://neural-chilli.github.io/qp/)
- Cookbook recipes: [manual/cookbook](manual/cookbook/index.qmd)
- Release process: [docs/releasing.md](docs/releasing.md)
