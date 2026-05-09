# AGENTS.md

## Project Overview

This repository implements `gptx`, a production Go CLI for OpenAI-compatible APIs.

Supported commands:
- `gptx search`: Responses API with hosted `web_search`.
- `gptx image generate`: `/images/generations`, saving returned images.
- `gptx image edit`: `/images/edits` multipart uploads, saving returned images.
- `gptx job`: local background jobs for search and real image API calls.
- `gptx version`: prints the CLI version string.
- `gptx version check`: checks latest release metadata and prints update guidance.
- `gptx update`: prints the supported Go install/update command.

This repo is CLI-only. Do not add server mode, MCP transport, MCP runtime targets, or user-facing MCP configuration.

## Configuration

Use environment variables or flags for credentials and endpoint defaults:
- `GPTX_OPENAI_BASE_URL`: optional base URL, default `https://api.openai.com/v1`.
- `GPTX_OPENAI_API_KEY`: required API key for API commands.

Never commit API keys. Never put secrets in docs, examples, logs, tests, screenshots, fixtures, or error strings.

## Repository Layout

- `cmd/gptx/main.go`: process entrypoint and top-level error printing.
- `internal/cli/root.go`: root command, global flags, env/config resolution, shared output helpers.
- `internal/cli/search.go`: `gptx search` command.
- `internal/cli/image.go`: `gptx image generate` and `gptx image edit` flows.
- `internal/cli/image_output.go`: output-format validation, path planning, image writes, result output.
- `internal/cli/jobs.go`: local background job commands, metadata, worker re-exec, logs, and result handling.
- `internal/openaiapi/client.go`: OpenAI-compatible API client using `openai-go/v3`.
- `internal/*/*_test.go`: unit tests for CLI behavior and API request construction.

Keep CLI code in `internal/cli` and API request code in `internal/openaiapi`.

## Build And Run Commands

Build the CLI:
```bash
go build ./cmd/gptx
```

Run help locally:
```bash
go run ./cmd/gptx --help
go run ./cmd/gptx search --help
go run ./cmd/gptx image generate --help
go run ./cmd/gptx image edit --help
go run ./cmd/gptx job --help
go run ./cmd/gptx version check --help
go run ./cmd/gptx update --help
```

Run dry-run image commands without an API key:
```bash
go run ./cmd/gptx image generate "test icon" --dry-run --out-dir /tmp
go run ./cmd/gptx --format json image edit "remove background" --dry-run --out /tmp/edit.png
```

Run a real API command only when `GPTX_OPENAI_API_KEY` is intentionally set:
```bash
GPTX_OPENAI_API_KEY=... go run ./cmd/gptx search "OpenAI Responses API web_search docs" --deep --bg
```

## Test Commands

Run all tests:
```bash
go test ./...
```

Run one package:
```bash
go test ./internal/cli
go test ./internal/openaiapi
```

Run a single test by name:
```bash
go test ./internal/cli -run '^TestImageGenerateDryRunNoAPIKey$'
go test ./internal/openaiapi -run '^TestEditImageUsesMultipartEndpoint$'
```

Run related test groups:
```bash
go test ./internal/cli -run 'DryRun|OutputFormat|SaveImages'
```

Run extra checks before larger changes:
```bash
go test -race ./...
go vet ./...
```

## Required Verification

Before claiming completion, run:
```bash
go test ./...
go build ./cmd/gptx
```

If command UX, flags, help, or docs changed, also run the help commands listed above.

If image output behavior changed, also run:
```bash
go run ./cmd/gptx --format json image generate "test" --dry-run --n 2 --out-dir /tmp
go test ./internal/cli -run 'SaveImages|DryRun|OutputFormat'
```

## API Behavior Requirements

Use the official OpenAI Go SDK: `github.com/openai/openai-go/v3`.

Search requirements:
- Endpoint: `POST /responses` through the configured OpenAI-compatible base URL.
- Default model: `gpt-5.4-mini`.
- Deep search defaults: model `gpt-5.5`, `reasoning.effort=high`, `web_search.search_context_size=high`, `max_tool_calls=8`, and `max_output_tokens=8000`.
- Must support model override via `gptx search --model`.
- Tool: `web_search`, not `web_search_preview`.
- Input shape: list-style Responses input item, not plain string.
- `store=false` must be set.
- Keep the citation prompt: broad search, `[number]` inline citations, final `References` section.
- Parse streamed text from `response.output_text.delta`, `response.output_text.done`, or final helpers.

Image generation requirements:
- Endpoint: `/images/generations`.
- Default model: `gpt-image-2`.
- Prompt is required.
- Decode `data[].b64_json` and save files locally.
- Do not print raw image base64 by default.

Image edit requirements:
- Endpoint: `/images/edits` using multipart form uploads.
- Default model: `gpt-image-2`.
- `--image` is repeatable for real edit calls.
- `--mask` is optional.
- Support multiple returned images.
- Validate planned output paths before paid/remote API calls.

## CLI UX Rules

Help text is product surface. Agents learn this CLI from `--help`.

- Document endpoint mapping, defaults, env vars, file saving, JSON mode, and examples in help text.
- Keep examples copy-pasteable and shell-safe.
- Do not mention MCP, server transports, or remote MCP URLs in user-facing help/docs.
- Prefer task-oriented commands over raw endpoint names.
- Text mode search prints the answer to stdout.
- Text mode image commands print saved paths to stdout, one per line.
- JSON mode emits one JSON object to stdout on success.
- For normal agent-driven deep search and real image API calls, prefer `--bg` so long remote calls can continue as local background jobs.
- Ordinary search is foreground-oriented; search `--bg` is only supported with `--deep`.
- `gptx version` must stay fast and local; use `gptx version check` for explicit network/cache update checks.
- `gptx update` prints `go install github.com/c-w-xiaohei/gptx/cmd/gptx@latest` and must not require an API key or network.
- `gptx job status/result/logs` inspect local background jobs by job ID.
- Errors and diagnostics go to stderr and must return non-zero.
- Missing API key must be visible and actionable.
- `--dry-run` must not require an API key, call a remote API, upload images, or write files.
- `--dry-run` and `--bg` must not be combined.
- Background jobs must reject explicit `--api-key`; use `GPTX_OPENAI_API_KEY` so secrets are not serialized in job metadata.

## File Output Rules

Image commands must plan and validate outputs before calling image APIs.

- `--out` is valid only when `--n=1`.
- `--out-dir` plus `--filename` supports multiple outputs.
- Default templates: `gptx-image-{timestamp}-{index}.{ext}` and `gptx-edit-{timestamp}-{index}.{ext}`.
- Supported output formats: `png`, `webp`, `jpeg`; normalize leading dots and case.
- Reject duplicate planned paths.
- Reject existing output files unless `--overwrite` is set.
- Reject missing output directories unless `--create-dirs` is set.
- Write decoded image data through a temporary file, then rename into place.
- Avoid leaving partial files after decode/write errors.

## Go Style Guidelines

- Run `gofmt` on modified Go files.
- Keep imports grouped by standard library, blank line, third-party/project imports.
- Prefer small structs for command options and request parameters.
- Keep CLI parsing separate from API client logic.
- Keep filesystem/output helpers separate from command construction.
- Use package-private helpers unless external use is required.
- Avoid global mutable state; construct Cobra commands with local option structs.
- Keep command constructors testable, e.g. `NewRootCommand()` and package-private builders.
- Use explicit names: `rootOptions`, `imageOptions`, `EditImageRequest`, `ImageResults`.
- Avoid clever abstractions; duplicate small command-specific code when it improves help and behavior.

## Error Handling Guidelines

- Return errors instead of calling `log.Fatal` outside `cmd/gptx/main.go`.
- `cmd/gptx/main.go` prints command errors and exits non-zero.
- Never include API keys or Authorization headers in errors.
- Wrap local file errors with the relevant path.
- Prefer actionable CLI errors: say what failed and how to fix it.
- Validate local usage errors before making network calls.
- Preserve upstream API errors, but sanitize anything that could contain secrets.

## Testing Guidelines

- Use `httptest` for API request-shape tests in `internal/openaiapi`.
- Do not hit real APIs in unit tests.
- Use temporary directories for file-save tests.
- Keep fake image payloads small base64 strings.
- Test command behavior through Cobra command execution rather than shelling out when possible.
- Use real command help output tests for critical docs if changing help text.
- Add regression tests for every bug fix.

## Cursor and Copilot Rules

No Cursor rules were found in `.cursor/rules/` or `.cursorrules`.

No Copilot instructions were found at `.github/copilot-instructions.md`.

If such files are added later, merge their durable project guidance into this file.
