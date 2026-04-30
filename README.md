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
gptx status
```

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
- `gptx version`

Run help:

```bash
gptx --help
gptx search --help
gptx image generate --help
gptx image edit --help
```

## Search Behavior

`gptx search` uses `POST /responses` with these defaults:

- model: `gpt-5.4-mini`
- tools include `web_search`
- `store=false`
- `input` sent as a list item (not a plain string)
- default instructions require broad search, inline `[number]` citations, and a final `References` section

`--model` is supported to override the default search model.

Examples:

```bash
gptx search "latest updates on OpenAI Responses API"
gptx search "latest updates on OpenAI Responses API" --model gpt-5.4-mini
gptx search "summarize this topic" --instructions "Be concise and structured."
gptx search "incident timeline" --instructions-file ./instructions.txt --json
```

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
gptx image generate "minimal logo concept" --out ./logo.png
gptx image generate "an isometric city" --n 3 --out-dir ./out --create-dirs
gptx image generate "poster" --size 1536x1024 --quality high --output-format webp --output-compression 80 --json
gptx image edit "remove background" --image ./in.png --out ./edited.png
gptx image edit "replace sky" --image ./in.png --mask ./mask.png --n 2 --out-dir ./edits
gptx image edit "merge style" --image ./a.png --image ./b.png --output-format png --json
```

## JSON Mode

Enable structured output with either:

- global `--json` or `--format json`
- command `--json`

In JSON mode, image commands emit one object including output paths and metadata; search emits one object including query and answer text.

## Friends

- [linux.do](https://linux.do/)
