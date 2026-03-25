# Releasing qp

This project now has a tag-driven GitHub release workflow that runs tests, builds cross-platform archives, and publishes them to GitHub Releases.

## Before Tagging

Make sure all of these are true:

- `go test ./...` passes
- `go build ./cmd/qp` succeeds
- `go vet ./...` passes
- [README.md](../README.md) matches current behavior
- [docs/user-guide.md](user-guide.md) reflects any user-facing changes
- CI is green on `main`

## Suggested First Release

Use a small explicit version such as `v0.1.0`.

That signals:

- the tool is usable
- the core surface exists
- some contracts may still evolve

## Tagging

From a clean `main` branch:

```bash
git pull origin main
go test ./...
go build -ldflags "-X main.version=v0.1.0" ./cmd/qp
git tag v0.1.0
git push origin v0.1.0
```

If you build via `make build`, the Makefile now stamps the binary version automatically from `git describe`.

If you want to preview the release artifacts locally before tagging:

```bash
make dist VERSION=v0.1.0
```

That writes:

- macOS archives for `amd64` and `arm64`
- Linux archives for `amd64` and `arm64`
- Windows archives for `amd64` and `arm64`
- `dist/checksums.txt`

## Release Notes

For the first release, keep the notes practical:

- what `qp` is
- which commands are implemented
- any known limitations worth calling out

## Install Path

Once tagged, users can install the CLI with:

```bash
go install github.com/neural-chilli/qp/cmd/qp@v0.1.0
```

For development builds, `@latest` also works:

```bash
go install github.com/neural-chilli/qp/cmd/qp@latest
```

They can also download the published release archives directly from GitHub Releases if they do not want to install with Go.

For Homebrew tap maintenance, see [docs/homebrew-tap.md](homebrew-tap.md).

The repo also includes `install.sh` for curl-pipe install flows. Keep release archive naming and `checksums.txt` intact so the installer remains compatible.
