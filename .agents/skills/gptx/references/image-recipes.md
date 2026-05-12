# GPT Image Recipes For gptx

Use these copy-ready prompt recipes as starting points for `gptx image generate` and `gptx image edit`. Adapt them to the user's request. Do not add every detail to every prompt.

All command examples use `--dry-run --json` first. For the real call, remove `--dry-run` after checking planned paths, and add `--bg` for normal or long image API runs so the session can continue while the job completes. Use `gptx job wait <job_id>` when the next step depends on the saved output. Do not combine `--dry-run` and `--bg`.

## Generation Recipes

### Photorealistic Natural

```text
Use case: photorealistic-natural
Primary request: candid photo of an elderly sailor on a small fishing boat adjusting a net
Scene/backdrop: coastal water with soft haze
Subject: weathered skin with wrinkles and sun texture
Style/medium: photorealistic candid photo
Composition/framing: medium close-up, eye-level
Lighting/mood: soft coastal daylight, shallow depth of field, subtle film grain
Materials/textures: real skin texture, worn fabric, salt-worn wood
Constraints: natural color balance; no heavy retouching; no glamorization; no watermark
Avoid: studio polish; staged look
```

Command:

```sh
gptx image generate '<prompt>' --dry-run --quality high --output-format jpeg --out ./output/sailor-photo.jpeg --create-dirs --json
```

### Product Mockup

```text
Use case: product-mockup
Primary request: premium product photo of a matte black shampoo bottle with a minimal label
Scene/backdrop: clean studio gradient from light gray to white
Subject: single bottle centered with subtle reflection
Style/medium: premium product photography
Composition/framing: centered, slight three-quarter angle, generous padding
Lighting/mood: softbox lighting, clean highlights, controlled shadows
Materials/textures: matte plastic, crisp label printing
Constraints: no logos or trademarks; no watermark
```

Command:

```sh
gptx image generate '<prompt>' --dry-run --quality high --size 1024x1024 --out ./output/product-mockup.png --create-dirs --json
```

### UI Mockup

```text
Use case: ui-mockup
Primary request: mobile app home screen for a local farmers market with vendors and daily specials
Asset type: mobile app screen
Style/medium: realistic product UI, not concept art
Composition/framing: clean vertical mobile layout with clear hierarchy
Constraints: practical layout; clear typography; no logos or trademarks; no watermark
```

Command:

```sh
gptx image generate '<prompt>' --dry-run --quality high --size 1024x1536 --out ./output/farmers-market-ui.png --create-dirs --json
```

### Infographic Or Diagram

```text
Use case: infographic-diagram
Primary request: detailed infographic of an automatic coffee machine flow
Scene/backdrop: clean, light neutral background
Subject: bean hopper -> grinder -> brew group -> boiler -> water tank -> drip tray
Style/medium: clean vector-like infographic with clear callouts and arrows
Composition/framing: vertical poster layout, top-to-bottom flow
Text (verbatim): "Bean Hopper", "Grinder", "Brew Group", "Boiler", "Water Tank", "Drip Tray"
Constraints: clear labels; strong contrast; no logos or trademarks; no watermark
```

Command:

```sh
gptx image generate '<prompt>' --dry-run --quality high --size 1024x1536 --out ./output/coffee-machine-flow.png --create-dirs --json
```

### Scientific Educational

```text
Use case: scientific-educational
Primary request: biology diagram titled "Cellular Respiration at a Glance" for high school students
Scene/backdrop: clean white classroom handout background
Subject: glucose turns into energy inside a cell; include glycolysis, Krebs cycle, and electron transport chain
Style/medium: flat scientific diagram with consistent icons, arrows, and readable labels
Composition/framing: landscape slide-style layout with clear hierarchy and generous whitespace
Text (verbatim): "Cellular Respiration at a Glance", "Glucose", "Pyruvate", "ATP", "NADH", "FADH2", "CO2", "O2", "H2O"
Constraints: scientifically plausible; avoid tiny text; no extra decoration; no watermark
```

### Ads Marketing

```text
Use case: ads-marketing
Primary request: campaign image for a streetwear brand called Thread
Subject: group of friends hanging out together in a stylish urban setting
Style/medium: polished youth streetwear campaign photography
Composition/framing: vertical ad layout with natural poses and integrated headline space
Lighting/mood: contemporary, energetic, tasteful
Text (verbatim): "Yours to Create."
Constraints: render the tagline exactly once; clean legible typography; no extra text; no watermarks; no unrelated logos
```

### Website Hero Background

```text
Use case: stylized-concept
Asset type: landing page hero background
Primary request: minimal abstract background with a soft gradient and subtle texture
Style/medium: matte illustration / soft-rendered abstract background
Composition/framing: wide composition with usable negative space for page copy
Lighting/mood: gentle studio glow
Color palette: restrained neutral palette
Constraints: no text; no logos; no watermark
```

Command:

```sh
gptx image generate '<prompt>' --dry-run --quality high --size 1536x1024 --out ./assets/hero-background.png --create-dirs --json
```

### Feature Section Illustration

```text
Use case: stylized-concept
Asset type: feature section illustration
Primary request: simple abstract shapes suggesting connection and flow
Scene/backdrop: subtle light-gray backdrop with faint texture
Style/medium: flat illustration; soft shadows; restrained contrast
Composition/framing: centered cluster; open margins for UI
Color palette: muted neutral palette
Constraints: no text; no logos; no watermark
```

### Blog Header Image

```text
Use case: photorealistic-natural
Asset type: blog header image
Primary request: overhead desk scene with notebook, pen, and coffee cup
Scene/backdrop: warm wooden tabletop
Style/medium: photorealistic photo
Composition/framing: wide crop with clean room for page copy
Lighting/mood: soft morning light
Constraints: no text; no logos; no watermark
```

### Game Environment Concept

```text
Use case: stylized-concept
Asset type: game environment concept art
Primary request: cavernous hangar interior with tall support beams and drifting fog
Scene/backdrop: industrial hangar interior, deep scale, light haze
Subject: compact shuttle parked near the center
Style/medium: cinematic concept art, industrial realism
Composition/framing: wide-angle, low-angle
Lighting/mood: volumetric light rays through drifting fog
Constraints: no logos or trademarks; no watermark
```

### Game UI Icon

```text
Use case: stylized-concept
Asset type: game UI icon
Primary request: round shield icon with a subtle rune pattern
Style/medium: painted game UI icon
Composition/framing: centered icon; generous padding; clear silhouette
Constraints: no text; no background scene elements; no logos or trademarks; no watermark
```

### Tileable Texture

```text
Use case: stylized-concept
Asset type: tileable game texture
Primary request: worn sandstone blocks
Style/medium: seamless tileable texture; PBR-like look
Scene/backdrop: neutral lighting reference only
Constraints: seamless edges; no obvious focal elements; no text; no logos or trademarks; no watermark
```

### Low-Fi Wireframe

```text
Use case: ui-mockup
Asset type: website wireframe
Primary request: SaaS homepage layout with clear hierarchy
Style/medium: low-fi grayscale wireframe
Subject: top nav; hero with headline and CTA; three feature cards; testimonial strip; pricing preview; footer
Composition/framing: landscape desktop layout
Constraints: label major blocks; no color; no logos; no real photos; no watermark
```

### Logo Concept

```text
Use case: logo-brand
Asset type: logo concept
Primary request: geometric leaf symbol suggesting sustainability and growth
Style/medium: vector logo mark; flat colors; minimal
Composition/framing: centered mark; clear silhouette; generous margin
Color palette: deep green and off-white
Constraints: strong silhouette; balanced negative space; no gradients; no mockups; no 3D; no watermark
```

## Reference-Image Generation Recipes

### Match Design System Without Editing Screenshot

```text
Use case: ui-mockup
Asset type: empty-state illustration
Primary request: create a new empty-state illustration for a dashboard
Input images: ./design-system.png: style, spacing, color, and UI tone reference only
Style/medium: match the reference's visual language without copying the screen
Composition/framing: centered illustration with generous whitespace
Constraints: borrow palette, corner radius feel, icon weight, and spacing; do not modify or reproduce the reference screenshot; no text unless requested; no watermark
```

Command:

```sh
gptx image generate '<prompt>' --dry-run --image ./design-system.png --quality high --out ./assets/empty-state.png --create-dirs --json
```

### Product Hero From Brand References

```text
Use case: product-mockup
Asset type: landing page hero image
Primary request: create a polished product hero image for the new feature launch
Input images: ./brand.png: brand palette and typography reference; ./components.png: product UI/component style reference
Style/medium: premium product marketing visual
Composition/framing: wide landscape composition with usable negative space for headline copy
Lighting/mood: clean, confident, high-end
Constraints: borrow brand cues only; create a new composition; no copied UI text; no watermark
```

## Edit Recipes

### Background Removal

```text
Use case: background-extraction
Input images: ./product.png: edit target
Primary request: remove only the background from the product photo
Constraints: keep product shape, edges, logo, label text, color, reflections, perspective, and internal details unchanged; do not restyle; do not add text, shadows, watermarks, or new objects; output a clean PNG suitable for ecommerce compositing
```

Command:

```sh
gptx image edit '<prompt>' --dry-run --image ./product.png --output-format png --out ./output/product-cutout.png --create-dirs --json
```

### Text Localization

```text
Use case: text-localization
Input images: ./infographic.png: original infographic edit target
Primary request: replace "Bean Hopper", "Grinder", "Brew Group", "Boiler", "Water Tank", and "Drip Tray" with "Tolva", "Molino", "Grupo de infusión", "Caldera", "Depósito de agua", and "Bandeja de goteo"
Constraints: change only the text; preserve layout, typography, spacing, hierarchy, arrows, icons, colors, and imagery; no extra words; no watermark
```

### Identity Preserve

```text
Use case: identity-preserve
Input images: ./person.png: person photo edit target; ./jacket.png: clothing reference
Primary request: replace only the jacket with the provided garment
Constraints: preserve face, body shape, pose, hair, expression, identity, skin tone, lighting, shadows, and background; no accessories or text
```

### Precise Object Edit

```text
Use case: precise-object-edit
Input images: ./room.png: room photo edit target
Primary request: replace only the white chairs with wooden chairs
Constraints: preserve camera angle, room lighting, floor shadows, table, walls, decor, and all surrounding objects; keep everything else unchanged
```

### Lighting Or Weather Change

```text
Use case: lighting-weather
Input images: ./street.png: original photo edit target
Primary request: make it look like a winter evening with gentle snowfall
Constraints: preserve subject identity, geometry, camera angle, composition, buildings, and object placement; change only lighting, atmosphere, season, and weather
```

### Style Transfer

```text
Use case: style-transfer
Input images: ./style.png: style reference
Primary request: apply the reference image's visual style to a man riding a motorcycle on a plain white backdrop
Constraints: preserve palette, texture, brushwork, and overall treatment from the style reference; no extra elements; no watermark
```

### Compositing

```text
Use case: compositing
Input images: ./base-scene.png: base scene; ./subject.png: subject to insert
Primary request: place the subject from ./subject.png next to the person in ./base-scene.png
Constraints: match lighting, perspective, scale, floor contact, and shadows; keep the base framing unchanged; no extra elements; no watermark
```

### Sketch To Render

```text
Use case: sketch-to-render
Input images: ./drawing.png: drawing reference
Primary request: turn the drawing into a photorealistic image
Constraints: preserve layout, proportions, silhouette, and perspective; choose realistic materials and lighting; do not add new elements or text
```

## Multi-Asset Pattern

For unrelated deliverables, do not use `--n`. Create one prompt and one command per asset with semantic filenames.

Example set:

```sh
gptx image generate '<hero prompt>' --dry-run --quality high --size 1536x1024 --out ./assets/hero.png --create-dirs --json
gptx image generate '<empty-state prompt>' --dry-run --quality high --size 1024x1024 --out ./assets/empty-state.png --create-dirs --json
gptx image generate '<blog-header prompt>' --dry-run --quality high --size 1536x1024 --out ./assets/blog-header.png --create-dirs --json
```

Use `--n 3` only for three variants of one prompt:

```sh
gptx image generate '<one prompt>' --dry-run --n 3 --out-dir ./variants --filename 'hero-v{index}.{ext}' --create-dirs --json
```
