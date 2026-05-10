# Configure gptx

Use this reference for installing, updating, authenticating, and troubleshooting `gptx`.

## Verify Local State

```sh
command -v gptx
gptx version
gptx version check
gptx status
gptx status --json
```

Do not print API key values. `gptx status` reports whether a key is configured without revealing it.

## Install

Install latest:

```sh
go install github.com/c-w-xiaohei/gptx/cmd/gptx@latest
```

Install a tagged version:

```sh
go install github.com/c-w-xiaohei/gptx/cmd/gptx@v0.2.0
```

The binary is usually at `$(go env GOPATH)/bin/gptx`.

## Update

Check whether a newer release is available:

```sh
gptx version check
```

Suppress network update checks and use cache/fallback output:

```sh
GPTX_NO_UPDATE_CHECK=1 gptx version check
```

Print the supported update command:

```sh
gptx update
```

On Linux amd64/arm64, `gptx update` also prints a fallback block that downloads the latest GitHub release archive with direct GitHub URLs, verifies `checksums.txt`, and installs the binary to `$HOME/.local/bin/gptx`. The command only prints this fallback; it does not execute `curl`, `go`, or any network request by itself, and it does not require the GitHub CLI.

## PATH And Fish

```fish
fish_add_path (go env GOPATH)/bin
fish_add_path ~/.local/bin
```

Set persistent environment in fish:

```fish
set -Ux GPTX_OPENAI_BASE_URL https://api.openai.com/v1
set -Ux GPTX_OPENAI_API_KEY '<api-key>'
```

## OpenAI-Compatible Gateways

`gptx` defaults to OpenAI at `https://api.openai.com/v1`.

To use an OpenAI-compatible gateway/proxy, override base URL:

```sh
export GPTX_OPENAI_BASE_URL=https://your-gateway.example/v1
```

## Release Archive Install

```sh
set -e
tmp="$(mktemp -d)"
archive="$tmp/gptx_linux_amd64.tar.gz"
checksums="$tmp/checksums.txt"
curl -fL --retry 3 -o "$archive" "https://github.com/c-w-xiaohei/gptx/releases/latest/download/gptx_linux_amd64.tar.gz"
curl -fL --retry 3 -o "$checksums" "https://github.com/c-w-xiaohei/gptx/releases/latest/download/checksums.txt"
(cd "$tmp" && sha256sum -c --ignore-missing checksums.txt)
tar -xzf "$archive" -C "$tmp"
mkdir -p "$HOME/.local/bin"
install -m 755 "$tmp/gptx" "$HOME/.local/bin/gptx"
```

## Troubleshooting

If `gptx` is missing, inspect PATH and install targets:

```sh
command -v gptx
go env GOPATH
ls "$(go env GOPATH)/bin/gptx" "$HOME/.local/bin/gptx" 2>/dev/null
```

If API commands fail with missing credentials, run `gptx status` and configure `GPTX_OPENAI_API_KEY`.
