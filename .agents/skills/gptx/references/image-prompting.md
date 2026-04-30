# GPT Image Prompting With gptx

Use this reference to shape high-quality prompts before calling `gptx image generate` or `gptx image edit`.

## Structure

Use a consistent order: scene or backdrop, subject, key details, constraints, output intent. For complex requests, use short labeled lines rather than one long paragraph.

Include intended use. “Landing page hero,” “product listing,” “mobile UI mockup,” “pitch slide,” “game sprite,” and “infographic” set different fidelity, composition, text, and polish expectations.

For photorealism, say `photorealistic` directly and add concrete real-world texture: skin pores, wrinkles, fabric wear, material grain, dust, reflections, imperfect everyday detail, or natural lens behavior.

## Specificity Policy

- If the user prompt is already specific, preserve the specificity and normalize it into a clean spec.
- If the prompt is generic, add tasteful detail only when it materially improves the requested output.
- Do not add extra characters, props, products, slogans, palettes, story beats, or brand concepts that are not implied.
- Do not make arbitrary left/right placement decisions unless the surrounding layout or user request supports them.
- Ask only when missing information blocks success: exact text, source image path, edit target, required output path, required dimensions, or preservation constraints.

## Composition And Layout

- Specify framing and viewpoint when useful: close-up, wide, top-down, eye-level, low-angle, centered, three-quarter angle.
- Call out negative space when the asset needs room for UI, headlines, buttons, or cropping.
- For people, specify body framing, scale, gaze, and interactions when they matter: `full body visible`, `looking down at the object`, `hands naturally gripping the handlebars`.
- For UI mockups, use layout and hierarchy language instead of concept-art language.
- For diagrams and slides, specify canvas orientation, hierarchy, label placement, and whitespace.

## Constraints And Invariants

For generation, constraints prevent drift:

```text
Constraints: no logos; no watermark; no extra text; keep palette restrained; leave safe negative space for page copy.
```

For edits, invariants are mandatory. Use `change only X; keep Y unchanged` and repeat the invariants on every follow-up iteration.

```text
Change only the background. Keep the product, edges, label text, reflections, camera angle, color, and shadows unchanged.
```

For identity-preserving edits, lock face, body shape, pose, hair, expression, clothing fit, skin tone, and background unless a specific item should change.

## Text In Images

- Put literal text in quotes.
- Require verbatim rendering and no extra characters.
- Specify typography, size, color, and placement when text matters.
- Spell unusual words letter-by-letter if accuracy matters.
- Use `--quality high` for small text, dense infographics, data-heavy slides, legends, axes, footnotes, or multi-font layouts.

Example:

```text
Text (verbatim): "Yours to Create."
Typography: clean bold sans-serif, white, centered near the lower third.
Constraints: render the tagline exactly once; no extra text; no watermark.
```

## Input Images And References

Do not assume every supplied image is an edit target. Label each image by path and role inside the prompt.

Common roles:

- `edit target`: the image to modify.
- `style reference`: visual style, palette, brushwork, or texture to borrow.
- `layout reference`: composition or spacing to match.
- `brand reference`: color, typography, product styling, or design-system cues.
- `subject reference`: object, character, product, or person to preserve or reinterpret.
- `compositing input`: source object or scene to combine.
- `mask`: area control for edit workflows.

If the user provides images for style, composition, or mood guidance and does not ask to modify them, use `gptx image generate --image`. If the user asks to preserve an existing image while changing specific parts, use `gptx image edit --image`.

For compositing, describe how images interact:

```text
Input images: ./room.png: base scene; ./chair.png: object to insert.
Primary request: place the chair from ./chair.png in the left corner of the room.
Constraints: match lighting, perspective, scale, floor contact, and shadows; keep the base framing unchanged.
```

## Transparent And Background Prompts

`gptx` does not include the Codex chroma-key helper. Do not promise guaranteed alpha from generation alone.

For simple cutout-style generation, ask for a removable flat background:

```text
Create the requested subject on a perfectly flat solid #00ff00 chroma-key background for possible background removal.
The background must be one uniform color with no shadows, gradients, texture, reflections, floor plane, or lighting variation.
Keep the subject fully separated from the background with crisp edges and generous padding.
Do not use #00ff00 anywhere in the subject.
No cast shadow, no contact shadow, no reflection, no watermark, and no text unless explicitly requested.
```

Use `#ff00ff` instead of `#00ff00` for green subjects. Avoid key colors that appear in the subject.

For actual source-image background removal, prefer `gptx image edit`:

```text
Remove only the background. Preserve the subject silhouette, internal details, label text, colors, reflections, and perspective. Do not restyle the subject. Output a PNG suitable for compositing.
```

For hair, fur, feathers, smoke, glass, liquids, translucent materials, reflective objects, soft shadows, or product grounding, warn that results need visual verification and may need mask/post-processing iteration.

## Use-Case Tips

Generation:

- `photorealistic-natural`: prompt like a real captured moment; include lens, lighting, framing, real texture; avoid over-stylized polish unless requested.
- `product-mockup`: describe product, packaging, material finish, clean silhouette, label clarity, controlled lighting.
- `ui-mockup`: state target fidelity first; focus on layout, hierarchy, practical UI elements; avoid concept-art language.
- `infographic-diagram`: define audience and layout flow; label parts explicitly; require verbatim text and strong contrast.
- `scientific-educational`: define audience, lesson objective, labels, arrows, scientific constraints, and scan-friendly whitespace.
- `ads-marketing`: write like a creative brief; include audience, vibe, scene, brand position, and exact tagline if text appears.
- `logo-brand`: keep simple, scalable, vector-friendly, strong silhouette, balanced negative space.
- `productivity-visual`: name the exact artifact, define canvas and hierarchy, provide real labels/data.
- `historical-scene`: include location/date and period-accurate clothing, props, and environment.

Editing:

- `text-localization`: change only text; preserve layout, typography, spacing, hierarchy, and imagery.
- `identity-preserve`: lock identity, face, body, pose, hair, expression; change only specified elements.
- `precise-object-edit`: specify exactly what to remove or replace; preserve surrounding texture and lighting.
- `lighting-weather`: change only light, shadows, atmosphere, precipitation, or season; preserve geometry and subject identity.
- `background-extraction`: isolate subject; preserve text and edges; no restyling; verify output.
- `style-transfer`: specify palette, texture, brushwork, and what must not change; add `no extra elements`.
- `compositing`: specify what moves where; match lighting, perspective, scale, contact shadows, and base framing.
- `sketch-to-render`: preserve layout, proportions, and perspective; do not add new elements unless requested.

## Iteration

- Start with a clean base prompt.
- Inspect saved output when possible.
- Make one targeted change per follow-up.
- Re-state critical constraints and invariants on every edit iteration.
- Avoid broad follow-ups like “make it better”; say exactly what to improve.
- Keep useful variants under versioned filenames.
