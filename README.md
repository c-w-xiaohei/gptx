# gptx

`gptx` is an open-source Go CLI for OpenAI-compatible search and image workflows.

It focuses on two workflows:

- Web search and grounded answers through Responses + `web_search`
- Image generation and image editing through Images APIs, including reference-image generation

This repository is CLI-only (no server mode).

## Install and Update

Install the latest release with Go:

```bash
go install github.com/c-w-xiaohei/gptx/cmd/gptx@latest
```

Install a specific tagged version:

```bash
go install github.com/c-w-xiaohei/gptx/cmd/gptx@v0.2.0
```

The binary is installed to:

```bash
$(go env GOPATH)/bin/gptx
```

Make sure that directory is on `PATH`. For fish:

```fish
fish_add_path (go env GOPATH)/bin
```

Release archives are available on GitHub Releases:

- `https://github.com/c-w-xiaohei/gptx/releases`

Verify:

```bash
gptx version
gptx version check
gptx status
```

Update by running:

```bash
gptx update
```

`gptx update` also prints a Linux GitHub release archive fallback using `gh release download`, `checksums.txt` verification, and installation to `$HOME/.local/bin/gptx`.

## Agent Skill

This repository includes a `gptx` skill for agent environments that support `npx skills`. It documents when to use `gptx`, how to configure it, and practical search/image workflows.

Install it globally:

```bash
npx skills add https://github.com/c-w-xiaohei/gptx -g --skill gptx -y
```

Or install it into the current project only:

```bash
npx skills add https://github.com/c-w-xiaohei/gptx --skill gptx -y
```

Verify global installation:

```bash
npx skills list -g
```

## Configuration

Use environment variables or global flags.

Required:

- `GPTX_OPENAI_API_KEY` (API key)

Optional:

- `GPTX_OPENAI_BASE_URL` (defaults to `https://api.openai.com/v1`)

By default, `gptx` targets the official OpenAI API. You can override `GPTX_OPENAI_BASE_URL` to use an OpenAI-compatible gateway or proxy.

Example:

```bash
export GPTX_OPENAI_API_KEY=***
export GPTX_OPENAI_BASE_URL=https://your-gateway.example/v1
```

Do not put secrets in source code, docs, command history screenshots, or committed files.

## Build and Run

```bash
go build ./cmd/gptx
./gptx version
```

Or run directly:

```bash
go run ./cmd/gptx version
```

## Commands

- `gptx search <query>`
- `gptx image generate <prompt>`
- `gptx image edit <prompt>`
- `gptx job <start|list|status|result|logs|cancel|rm>`
- `gptx version`
- `gptx version check`
- `gptx update`

Run help:

```bash
gptx --help
gptx search --help
gptx image generate --help
gptx image edit --help
gptx job --help
gptx version check --help
gptx update --help
```

## Search Behavior

`gptx search` uses `POST /responses` with these defaults:

- model: `gpt-5.4-mini`
- tools include `web_search`
- `store=false`
- `input` sent as a list item (not a plain string)
- default instructions require broad search, inline `[number]` citations, and a final `References` section

`--model` is supported to override the default search model.

Deep search is for longer cited research. Enable it with `--deep`:

- model defaults to `gpt-5.5`
- `reasoning.effort=high`
- `web_search.search_context_size=high`
- `max_tool_calls=8`
- `max_output_tokens=8000`
- supports `--bg` for local background jobs

Some OpenAI-compatible gateways reject `max_tool_calls`. In that case, deep search retries once without `max_tool_calls` and reports `compatibility_fallback` in JSON output while preserving the other deep defaults.

Ordinary search stays foreground-oriented for quick lookups. `--bg` is only valid with `--deep` for search.

Examples:

```bash
gptx search "latest updates on OpenAI Responses API"
gptx search "latest updates on OpenAI Responses API" --model gpt-5.4-mini
gptx search "best practices for OpenAI Responses prompts" --deep --bg
gptx search "incident timeline" --deep --instructions-file ./instructions.txt --json --bg
```

Use `--deep --bg` for normal agent-driven long research queries. The command prints a local job ID; inspect it with `gptx job status <job_id>`, `gptx job result <job_id>`, and `gptx job logs <job_id> --stderr`.

## Image Generate Behavior

`gptx image generate` creates new images with default model `gpt-image-2`.

- Without `--image`, it uses `POST /images/generations` for text-only generation.
- With one or more `--image` attachments, it uses `POST /images/edits` internally so design systems, brand references, screenshots, and style images can guide generation.

The API returns `b64_json`; the CLI decodes and saves files locally.

Common output flags and rules:

- `--out` is valid only when `--n=1`
- `--out-dir` selects the output directory
- `--filename` supports templates with `{timestamp}`, `{index}`, and `{ext}`
- `--output-format` supports `png`, `webp`, and `jpeg` (case-insensitive)
- `--overwrite` allows replacing existing files
- `--create-dirs` creates missing output directories
- `--dry-run` plans and validates outputs without API calls, uploads, or file writes
- `--bg` runs a real image command as a local background job and prints a job ID

Default generate filename template:

- `gptx-image-{timestamp}-{index}.{ext}`

## Image Edit Behavior

`gptx image edit` uses `POST /images/edits` multipart with default model `gpt-image-2` for explicit edits and mask-based workflows.

- `--image` is repeatable and required
- `--mask` is optional
- `--model`, `--background`, `--output-compression`, and `--input-fidelity` are supported
- response `b64_json` is decoded and saved to files

Default edit filename template:

- `gptx-edit-{timestamp}-{index}.{ext}`

Output safety and validation:

- planned output paths are validated before paid API calls
- duplicate paths are rejected
- existing files are rejected unless `--overwrite` is set
- missing output directories are rejected unless `--create-dirs` is set
- writes use temporary files then atomic rename to avoid partial output files

Examples:

```bash
gptx image generate "minimal logo concept" --dry-run --out ./logo.png --json
gptx image generate "minimal logo concept" --out ./logo.png --bg
gptx image generate "an isometric city" --dry-run --n 3 --out-dir ./out --create-dirs --json
gptx image generate "an isometric city" --n 3 --out-dir ./out --create-dirs --bg
gptx image generate "poster" --size 1536x1024 --quality high --output-format webp --output-compression 80 --json --bg
gptx image edit "remove background" --dry-run --image ./in.png --out ./edited.png --json
gptx image edit "remove background" --image ./in.png --out ./edited.png --bg
gptx image edit "replace sky" --image ./in.png --mask ./mask.png --n 2 --out-dir ./edits --bg
gptx image edit "merge style" --image ./a.png --image ./b.png --output-format png --json --bg
```

Run image commands with `--dry-run --json` first to validate paths. For the real call, remove `--dry-run` and add `--bg` unless foreground output is specifically needed. Do not combine `--dry-run` and `--bg`.

## Background Jobs

`gptx` can run deep search, `image generate`, and `image edit` as local background jobs. This is intended for normal agent-driven research and real image API calls that may outlive the interactive session.

Shortcut examples:

```bash
gptx search "latest OpenAI image docs" --deep --json --bg
gptx image generate "poster" --out ./poster.png --json --bg
gptx image edit "remove background" --image ./in.png --out ./edited.png --json --bg
```

Explicit job examples:

```bash
gptx job start -- search "latest OpenAI image docs" --deep --json
gptx job start -- image generate "poster" --out ./poster.png --json
gptx job status <job_id>
gptx job result <job_id>
gptx job logs <job_id> --stderr
```

Background jobs use local metadata and log files. Use `GPTX_OPENAI_API_KEY` for credentials; explicit `--api-key` is rejected for background jobs so secrets are not serialized into job metadata.

## JSON Mode

Enable structured output with either:

- global `--json` or `--format json`
- command `--json`

In JSON mode, image commands emit one object including output paths and metadata; search emits one object including query and answer text.

## Friends

- [linux.do](https://linux.do/)
