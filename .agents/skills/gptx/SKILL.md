---
name: gptx
description: This skill should be used when the user asks to use `gptx`, OpenAI search, cited web research, GPT/OpenAI-compatible image generation, image editing, image variants, reference-image generation, background removal, or project bitmap assets through the local `gptx` CLI.
version: 0.1.0
---

# gptx CLI

Use `gptx` for deterministic OpenAI-compatible search, research, image generation, and image editing from shell commands.

Prefer `gptx` when a task needs predictable CLI behavior, stable text/JSON output, saved image files, or direct API-style tool use from another agent. Do not treat it as an agent runtime.

For image tasks, read `references/image.md` before running a real image command. It contains the decision tree, prompt-shaping workflow, edit invariants, dry-run gate, and output safety rules adapted from high-quality GPT image-generation skills.

## Minimal Workflow

Check local state first, without printing secrets:

```sh
command -v gptx
gptx version
gptx status
```

Use JSON when another tool or agent will parse the result. For normal searches and real image API calls, prefer local background jobs with `--bg` so the session can continue while the remote call completes:

```sh
gptx status --json
gptx search "current GoReleaser GitHub Action recommendations" --json --bg
gptx image generate "test" --dry-run --n 2 --out-dir /tmp --json
gptx image generate "test" --n 2 --out-dir /tmp --json --bg
gptx image generate "match this design system" --image ./design-system.png --dry-run --out /tmp/ref.png --json
gptx image generate "match this design system" --image ./design-system.png --out /tmp/ref.png --json --bg
gptx image edit "remove only the background; keep the product unchanged" --image ./product.png --dry-run --out /tmp/product-cutout.png --json
gptx image edit "remove only the background; keep the product unchanged" --image ./product.png --out /tmp/product-cutout.png --json --bg
```

For image generation or editing, use `--dry-run --json` first to validate planned output paths before paid API calls. For the real call, remove `--dry-run` and add `--bg` unless the user explicitly needs foreground output. Do not combine `--dry-run` and `--bg`. Do not print raw image base64 or ask the user to paste API keys into chat.

For background jobs, use `GPTX_OPENAI_API_KEY` from the environment. Do not pass explicit `--api-key`; background jobs reject it so secrets are not serialized in job metadata.

Inspect background jobs with the built-in job commands:

```sh
gptx job status <job_id>
gptx job result <job_id>
gptx job logs <job_id>
gptx job logs <job_id> --stderr
```

These examples have been verified in this project: `gptx version`, `gptx status`, `gptx status --json`, `gptx search ... --json --bg`, and image `--dry-run` planning followed by real `--bg` runs.

## Additional Resources

- `references/configure.md` - Public install/update, configuration, OpenAI-compatible base URL override, status checks, and troubleshooting.
- `references/search.md` - Research workflow, repeated/parallel search strategy, JSON output, and citation behavior.
- `references/image.md` - Image command decision tree, output planning, dry-run gate, and file safety.
- `references/image-prompting.md` - High-quality GPT image prompt structure, specificity, invariants, references, transparency, and iteration.
- `references/image-recipes.md` - Copy-ready generation, reference-image generation, editing, and multi-asset prompt recipes for `gptx`.
