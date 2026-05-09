# Images With gptx

Use this reference for high-quality GPT/OpenAI-compatible image generation, reference-image generation, and image editing through the local `gptx` CLI.

This workflow adapts the strongest parts of OpenAI-style imagegen skills to `gptx`: choose the right intent, shape a production prompt, preserve edit invariants, validate output paths before paid calls, and iterate with targeted changes.

## Core Rules

- Use `gptx image generate` for new bitmap assets from text.
- Use `gptx image generate --image` when images are references for style, layout, brand, mood, subject, or composition.
- Use `gptx image edit --image` when the user asks to modify an existing image while preserving parts of it.
- Use `gptx image edit --mask` for explicit masked edits or inpainting workflows.
- Run `--dry-run --json` before a real image call unless the user only asked for command help.
- After a successful dry run, use `--bg` for normal or long real image generation/editing runs so the session can continue while the remote image API call completes.
- Do not overwrite files unless the user explicitly asks for replacement; otherwise choose a versioned filename.
- Do not ask the user to paste API keys in chat. Ask them to configure `GPTX_OPENAI_API_KEY` locally.
- Do not print raw base64. `gptx` saves decoded images and prints paths or JSON metadata.
- Treat private, sensitive, or third-party images as intentional uploads only when the user clearly asks to process them.

## Decision Tree

Classify intent before choosing the command:

1. If the user wants a new photo, illustration, mockup, icon concept, sprite, texture, hero image, ad creative, or visual variant, use `image generate`.
2. If the user supplies image files only to guide style, brand, composition, color, layout, or mood, use `image generate --image`. This is user-intent generation even though `gptx` uses `/images/edits` internally for reference attachments.
3. If the user wants to change a specific existing image while keeping some parts intact, use `image edit --image`.
4. If the user provides or requests a mask, use `image edit --mask`.
5. If the asset should be SVG/vector/code-native or must match an existing vector icon system, edit the source asset directly instead of generating a bitmap.

For many distinct assets, run separate prompts. Use `--n` for variants of one prompt, not for unrelated assets.

## Prompt Workflow

Before the dry run, turn the user's request into a compact production prompt. Ask a short clarification only when a missing detail blocks success, such as the edit target path, exact required text, output use, or whether unchanged regions must be preserved.

For detailed prompt principles and copy-ready recipes, load the focused references instead of reinventing prompt structure:

- `references/image-prompting.md` - Structure, specificity, augmentation rules, text rendering, reference images, transparency, and iteration.
- `references/image-recipes.md` - High-quality generation/editing prompt recipes adapted to `gptx` commands.

Use this schema when helpful:

```text
Use case: <product-mockup | ui-mockup | ads-marketing | logo-brand | illustration-story | photorealistic-natural | infographic-diagram | precise-object-edit | background-extraction | style-transfer | compositing>
Asset type: <where this image will be used>
Primary request: <main user request>
Input images: <path and role for each image: reference, edit target, style reference, compositing input, mask>
Subject: <main subject>
Scene/backdrop: <environment or background>
Style/medium: <photo, illustration, 3D render, pixel art, watercolor, etc.>
Composition/framing: <wide, close-up, top-down, centered, negative space, safe area>
Lighting/mood: <lighting and emotional tone>
Color palette: <brand or palette constraints>
Text (verbatim): "<exact text, if any>"
Constraints: <must keep, must include, must match>
Avoid: <watermark, distorted hands, extra text, artifacts, unwanted objects>
```

Keep prompt augmentation restrained. If the user's prompt is already specific, normalize it instead of adding new creative requirements. If the prompt is generic, add only details that materially improve the requested output.

## Generate Images

```sh
gptx image generate "minimal logo concept" --dry-run --out ./logo.png --json
gptx image generate "minimal logo concept" --out ./logo.png --json --bg
gptx image generate "create an empty state illustration matching this design system" --dry-run --image ./design-system.png --out ./empty-state.png --json
gptx image generate "create an empty state illustration matching this design system" --image ./design-system.png --out ./empty-state.png --json --bg
gptx image generate "make a product hero image in this visual style" --dry-run --image ./brand.png --image ./components.png --out ./hero.png --json
gptx image generate "make a product hero image in this visual style" --image ./brand.png --image ./components.png --out ./hero.png --json --bg
gptx image generate "an isometric city" --dry-run --n 3 --out-dir ./out --create-dirs --json
gptx image generate "an isometric city" --n 3 --out-dir ./out --create-dirs --json --bg
gptx image generate "poster" --dry-run --size 1536x1024 --quality high --output-format webp --output-compression 80 --json
gptx image generate "poster" --size 1536x1024 --quality high --output-format webp --output-compression 80 --json --bg
```

Generation behavior:

- Sends `POST /images/generations` for text-only generation.
- Sends multipart `POST /images/edits` internally when `--image` reference attachments are present.
- Uses model default `gpt-image-2`.
- Decodes `data[].b64_json` and saves files locally.
- Does not print raw base64 by default.
- Default filename template: `gptx-image-{timestamp}-{index}.{ext}`.
- Supports `--model`, `--background`, `--output-compression`, and text-only `--moderation`.
- Supports `--input-fidelity` only when `--image` reference attachments are present, because that path uses `/images/edits` internally.

Reference-image generation rules:

- Label each `--image` role in the prompt: style reference, layout reference, brand reference, product reference, mood reference, or compositing input.
- Say what to borrow and what not to copy.
- Prefer explicit constraints such as “match the color palette and spacing, but create a new composition.”
- Do not treat a reference image as an edit target unless the user asks to modify that image.

## Edit Images

```sh
gptx image edit "remove background" --dry-run --image ./in.png --out ./out.png --json
gptx image edit "remove background" --image ./in.png --out ./out.png --json --bg
gptx image edit "replace sky" --dry-run --image ./in.png --mask ./mask.png --n 2 --out-dir ./edits --json
gptx image edit "replace sky" --image ./in.png --mask ./mask.png --n 2 --out-dir ./edits --json --bg
gptx image edit "merge style" --dry-run --image ./a.png --image ./b.png --output-format png --json
gptx image edit "merge style" --image ./a.png --image ./b.png --output-format png --json --bg
gptx image edit "preserve product, remove background" --dry-run --image ./product.png --input-fidelity high --background transparent --output-format png --out ./cutout.png --json
gptx image edit "preserve product, remove background" --image ./product.png --input-fidelity high --background transparent --output-format png --out ./cutout.png --json --bg
```

Edit behavior:

- Sends multipart `POST /images/edits`.
- Uses model default `gpt-image-2`.
- Requires repeatable `--image` for real edit calls.
- Accepts optional `--mask`.
- Supports multiple returned images.
- Detects image MIME from `.png`, `.jpg`, `.jpeg`, or `.webp` paths.
- Default filename template: `gptx-edit-{timestamp}-{index}.{ext}`.
- Supports `--model`, `--background`, `--output-compression`, and `--input-fidelity`.

Edit prompt rules:

- State the requested change first.
- State invariants every time: what must remain unchanged.
- Preserve identity, pose, layout, product geometry, typography, brand marks, and untouched regions unless explicitly changing them.
- For masked edits, describe what the mask controls and what outside the mask must preserve.
- For background removal, say whether the output should be a transparent cutout, a solid-color background, or a replacement scene.
- For multi-image edits, label roles: edit target, object to insert, style source, background source, or mask.

Good edit prompt pattern:

```text
Remove only the background from the product photo. Keep the product shape, edges, logo, color, reflections, and perspective unchanged. Do not add text, shadows, watermarks, or new objects. Output a clean PNG suitable for ecommerce compositing.
```

## Output Planning

Always plan paths before a real call:

```sh
gptx image generate "test" --dry-run --n 2 --out-dir /tmp --json
gptx image generate "test" --dry-run --image ./design-system.png --out /tmp/ref.png --json
gptx image edit "test" --dry-run --image ./input.png --out /tmp/edit.png --json
```

Treat any dry-run behavior that calls an API, uploads images, requires an API key, or writes files as a bug.

Dry-run to real-run sequence:

1. Run the same command with `--dry-run --json`.
2. Read the planned `paths` and confirm they are safe.
3. Remove `--dry-run` for the real call. Keep `--json` when another tool or agent will parse the result.
4. Add `--bg` for normal or long real image API runs. Do not combine `--dry-run` and `--bg`.
5. Inspect the returned job ID with `gptx job status`, `gptx job result`, and `gptx job logs --stderr`.
6. If the real command fails because an output file exists, choose a new versioned path unless the user explicitly asked for `--overwrite`.

Output flags:

- `--out` only when `--n=1`.
- `--out-dir` selects output directory.
- `--filename` supports `{timestamp}`, `{index}`, and `{ext}`.
- `--model` selects the image model; default is `gpt-image-2`.
- `--image` on `image generate` is repeatable and supplies reference attachments.
- `--background` accepts `auto`, `transparent`, or `opaque`; transparent requires `png` or `webp`.
- `--output-format` accepts `png`, `webp`, or `jpeg`.
- `--output-compression` accepts `0..100` and only works with `jpeg` or `webp`.
- `--moderation` accepts `auto` or `low` for text-only `image generate`; it is rejected for `generate --image` because that uses the edit endpoint.
- `--input-fidelity` accepts `high` or `low` for edit workflows and `generate --image`.
- `--overwrite` allows replacing existing files.
- `--create-dirs` creates missing output directories.
- `--dry-run` computes planned paths without requiring an API key, calling a remote API, uploading images, or writing files.
- `--bg` runs the real command as a local background job and prints a job ID. It is separate from `--background`, which controls image background mode.

Input validation:

- Input images must be regular `.png`, `.jpg`, `.jpeg`, or `.webp` files smaller than 50MB.
- At most 16 input/reference images are supported.
- Masks require at least one input image, must be PNG files smaller than 4MB, must include transparency, and must match the first input image dimensions.
- Explicit output paths must include an extension matching `--output-format`; `.jpg` is accepted for `jpeg`.

Format guidance:

- Use `png` for edits, transparency-oriented outputs, UI assets, screenshots, and assets that may need further processing.
- Use `webp` for compact web previews and final web assets when transparency and tooling support are acceptable.
- Use `jpeg` for photographic assets when transparency is not needed.

Size and quality guidance:

- Default `1024x1024` and `quality low` are suitable for drafts.
- Use `quality high` for final assets, dense text, product shots, identity-sensitive edits, and polished marketing visuals.
- Choose landscape sizes for hero images and banners, portrait sizes for stories/posters, and square sizes for icons, thumbnails, or fast drafts.

## Transparent And Background Work

`gptx` does not bundle the Codex `imagegen` chroma-key removal helper. Do not claim local background removal exists unless the workspace provides a real tool for it.

Use one of these strategies:

- For true image edits, use `gptx image edit "remove background ..." --image ./input.png --output-format png`.
- For new cutout-style assets, prompt for a clean subject on a flat solid background or transparent-background-friendly output, then save as PNG.
- Do not assume a new generated PNG has a real alpha channel. Verify the saved file after generation; if transparency is critical, use an explicit edit/mask workflow or a real post-processing tool.
- For complex subjects such as hair, fur, glass, smoke, liquids, or reflections, warn that background extraction may need iteration or a dedicated masking workflow.

## Iteration And QA

After each real generation or edit:

1. Check that the command succeeded and saved the expected path count.
2. Inspect or open the saved image when the environment supports image viewing.
3. Compare against the prompt: subject, style, composition, text accuracy, brand/reference adherence, and avoid list.
4. For edits, verify invariants: unchanged regions, identity, object geometry, lighting, shadows, perspective, and mask boundaries.
5. Iterate with one targeted change at a time. Avoid broad “make it better” follow-ups.
6. Preserve useful variants with versioned filenames instead of overwriting.

Final response for image tasks should report:

- Saved path(s)
- Whether the run was dry-run or real
- Final prompt or prompt summary
- Important options used: `--image`, `--mask`, `--n`, `--size`, `--quality`, `--output-format`
- Any limitations or recommended next iteration

## File Safety

`gptx` validates planned output paths before paid image API calls. It rejects duplicate planned paths, missing output directories unless `--create-dirs` is set, and existing output files unless `--overwrite` is set. It writes decoded image data through a temporary file before renaming into place.

Never bypass this by writing custom one-off image API scripts when `gptx image generate` or `gptx image edit` supports the task.
