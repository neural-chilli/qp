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

## Versioning Guidance

Use explicit semver tags (for example `v0.5.0`) and keep [CHANGELOG.md](../CHANGELOG.md) updated before tagging.

For this project stage:

- use minor bumps (`v0.x+1.0`) for roadmap slices with notable user-facing behavior changes
- use patch bumps (`v0.x.y+1`) for bug-fix or small compatibility updates

## Tagging

From a clean `main` branch:

```bash
git pull origin main
go test ./...
go build -ldflags "-X main.version=v0.1.0" ./cmd/qp
git tag v0.1.0
git push origin v0.1.0
```

If you want to preview the release artifacts locally before tagging:

```bash
VERSION=v0.1.0
TAG="${VERSION}"
OUT_VERSION="${VERSION#v}"
ROOT_DIR="$(pwd)"
mkdir -p dist

build_one() {
  os="$1"
  arch="$2"
  ext="$3"
  bin="qp"
  [ "$os" = "windows" ] && bin="qp.exe"
  tmp="$(mktemp -d)"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w -X main.version=${TAG}" -o "${tmp}/${bin}" ./cmd/qp
  base="qp_v${OUT_VERSION}_${os}_${arch}"
  if [ "$ext" = "zip" ]; then
    (cd "$tmp" && zip -q "${ROOT_DIR}/dist/${base}.zip" "$bin")
  else
    tar -C "$tmp" -czf "dist/${base}.tar.gz" "$bin"
  fi
  rm -rf "$tmp"
}

build_one darwin amd64 tar.gz
build_one darwin arm64 tar.gz
build_one linux amd64 tar.gz
build_one linux arm64 tar.gz
build_one windows amd64 zip
build_one windows arm64 zip
(cd dist && sha256sum *.tar.gz *.zip > checksums.txt)
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
