---
name: gptx
description: This skill should be used when the user asks to use `gptx`, OpenAI search, cited web research, GPT/OpenAI-compatible image generation, image editing, image variants, reference-image generation, background removal, or project bitmap assets through the local `gptx` CLI.
version: 0.1.0
---

# gptx CLI

Use `gptx` for deterministic OpenAI-compatible search, research, image generation, and image editing from shell commands.

Prefer `gptx` when a task needs predictable CLI behavior, stable text/JSON output, saved image files, or direct API-style tool use from another agent. Do not treat it as an agent runtime.

For image tasks, read `references/image.md` before running a real image command. It contains the decision tree, prompt-shaping workflow, edit invariants, dry-run gate, and output safety rules adapted from high-quality GPT image-generation skills. For slow or high-quality image jobs, also read `references/image-latency.md` to plan `--timeout`, `--quality`, `--size`, reference-image use, and `--bg`.

## Minimal Workflow

Check local state first, without printing secrets:

```sh
command -v gptx
gptx version
gptx version check
gptx status
```

Use JSON when another tool or agent will parse the result. For agent-driven deep research, default to `search --deep --json --bg` unless the user explicitly needs foreground output. Deep research can run long and network/tool timeouts can interrupt foreground calls; background jobs keep the remote work alive. When the next step depends on completion, use `gptx job wait <job_id>` instead of manual polling. Ordinary search is foreground-oriented and rejects `--bg` unless `--deep` is set. For real image API calls, prefer local background jobs with `--bg` so the session can continue while the remote call completes:

```sh
gptx status --json
gptx search "current GoReleaser GitHub Action recommendations" --deep --json --bg
gptx search "summarize this incident" --context ./incident.md --deep --json --bg
gptx image generate "test" --dry-run --n 2 --out-dir /tmp --json
gptx image generate "test" --n 2 --out-dir /tmp --json --bg
gptx job wait <job_id>
gptx image generate "use this campaign brief" --context ./brief.md --dry-run --out /tmp/campaign-card.png --json
gptx image generate "use this campaign brief" --context ./brief.md --out /tmp/campaign-card.png --json --bg
gptx image generate "match this design system" --image ./design-system.png --dry-run --out /tmp/ref.png --json
gptx image generate "match this design system" --image ./design-system.png --out /tmp/ref.png --json --bg
gptx image edit "remove only the background; keep the product unchanged" --image ./product.png --dry-run --out /tmp/product-cutout.png --json
gptx image edit "remove only the background; keep the product unchanged" --image ./product.png --out /tmp/product-cutout.png --json --bg
```

Write deep research queries as self-contained, high-fidelity research briefs. Assume the search tool does not know the surrounding chat, repo, prior findings, user intent, acronyms, or hidden constraints. Include the exact question, important spelling variants, entities, timeframe, scope boundaries, required source quality, desired output format, and what to separate or verify. Do not pass vague prompts such as `research this` or `find sources`; preserve the user's actual investigation target in the command string.

Use repeatable `--context <path>` when the prompt/query needs local text files attached. `--context` only accepts file paths; the CLI reads them and appends their contents with fixed file boundaries before the API call. SVG files are not supported by `--context` or `--image`; rasterize SVG logos to PNG/WebP before using them as `--image` references.

For image generation or editing, use `--dry-run --json` first to validate planned output paths before paid API calls. For the real call, remove `--dry-run` and add `--bg` unless the user explicitly needs foreground output. High-quality, non-square, reference-image, edit, UI screenshot, and multi-image jobs can take many minutes; use the 20-minute default timeout or raise it explicitly with root `--timeout` for heavy jobs. Do not combine `--dry-run` and `--bg`. Do not print raw image base64 or ask the user to paste API keys into chat.

For background jobs, use `GPTX_OPENAI_API_KEY` from the environment. Do not pass explicit `--api-key`; background jobs reject it so secrets are not serialized in job metadata.

Wait for background jobs with the built-in job command:

```sh
gptx job wait <job_id>
```

Use diagnostic commands only when needed:

```sh
gptx job status <job_id>
gptx job result <job_id>
gptx job logs <job_id>
gptx job logs <job_id> --stderr
```

These examples have been verified in this project: `gptx version`, `gptx version check`, `gptx status`, `gptx status --json`, `gptx search ... --deep --json --bg`, and image `--dry-run` planning followed by real `--bg` runs.

## Additional Resources

- `references/configure.md` - Public install/update, configuration, OpenAI-compatible base URL override, status checks, and troubleshooting.
- `references/search.md` - Research workflow, repeated/parallel search strategy, JSON output, and citation behavior.
- `references/image.md` - Image command decision tree, output planning, dry-run gate, and file safety.
- `references/image-latency.md` - Evidence-backed image latency factors, timeout defaults, and planning guidance for quality, size, `n`, and reference-image workflows.
- `references/image-prompting.md` - High-quality GPT image prompt structure, specificity, invariants, references, transparency, and iteration.
- `references/image-recipes.md` - Copy-ready generation, reference-image generation, editing, and multi-asset prompt recipes for `gptx`.
