# Image Latency And Timeout Planning

Use this reference when choosing `--timeout`, `--quality`, `--size`, `--n`, reference images, or foreground vs `--bg` for GPT/OpenAI-compatible image generation and editing.

## Bottom Line

Treat image generation latency as roughly proportional to the amount of image work requested. The strongest official planning model is token-based: OpenAI says image latency and cost are proportional to the tokens needed to render the image, and larger sizes and higher quality settings produce more tokens [1].

For production CLI usage, treat `quality`, output size/aspect ratio, `n`, and reference/input images as first-order timeout drivers. Treat prompt complexity as second-order unless it asks for dense UI, exact text, diagrams, many constraints, or high-fidelity editing.

`gptx` defaults the root `--timeout` to 20 minutes to avoid killing long background image jobs too early. Use shorter explicit timeouts for fast foreground experiments, and longer explicit timeouts for batches or very heavy reference-image edits.

## Official Evidence

| Factor | Official facts | Latency impact for CLI planning |
|---|---|---|
| Quality | GPT Image output supports quality settings such as `low`, `medium`, `high`, and `auto` [2]. OpenAI says `quality: "low"` is the fastest option for drafts, thumbnails, and quick iterations [4]. For pre-`gpt-image-2` token accounting, OpenAI lists square output tokens as 272 low, 1056 medium, and 4160 high [1]. | Large impact. Medium is about 3.9x low tokens; high is about 15.3x low and about 3.9x medium. Use `low` for drafts; reserve `high` for final/polished assets. |
| Size / aspect ratio | OpenAI lists common GPT Image sizes such as `1024x1024`, `1536x1024`, and `1024x1536` [4]. For pre-`gpt-image-2` high-quality outputs, OpenAI lists 4160 tokens for `1024x1024`, 6208 tokens for `1536x1024`, and 6240 tokens for `1024x1536` [1]. OpenAI says square images are typically fastest for `gpt-image-2` [4]. | Large impact. Landscape/portrait 1536-side outputs are about 1.5x the square token count at high quality in the official table. Prefer square or smaller sizes for fast drafts. |
| Number of images `n` | OpenAI says `n` can generate multiple images in one request and defaults to one image [5]. Image generation usage reports input, output, and total tokens for GPT image models [6]. | Roughly multiplicative timeout risk. Even if the provider parallelizes internally, more requested outputs increase work and tail latency. Default to `n=1`; use `--bg` for variants and batches. |
| Input/reference images and edits | OpenAI supports edits and reference-image workflows, including one or more image references [7]. Final cost includes input text tokens, input image tokens for edits, and image output tokens [2]. `input_fidelity` controls how strongly details are preserved for input images; `gpt-image-2` processes image inputs at high fidelity automatically [8]. | Medium to large impact. Reference images add upload/encoding time and image input tokens. High-fidelity preservation/editing adds work. Plan longer timeouts for edits and reference-image generation than pure text-to-image. |
| Masks / inpainting | OpenAI supports masks for edits. If multiple input images are provided, the mask applies to the first image [9]. | Medium impact. Masks often appear in high-fidelity edits where preserving untouched regions matters, so use background jobs and longer timeouts. |
| Prompt complexity | OpenAI's latency guide says output-token generation is usually the highest-latency step, while cutting input tokens often gives modest latency gains unless context is massive, including images [10]. For Responses API image tools, OpenAI can revise prompts and expose a `revised_prompt` [11]. | Variable / second-order. Prompt length alone is usually less important than output size/quality, but dense UI screenshots, exact text, code panels, diagrams, and many constraints increase long-tail risk. |
| Model choice | OpenAI model pages list `gpt-image-1` speed as slowest [12] and `gpt-image-1.5` speed as medium [13]. OpenAI says GPT Image 1.5 can generate images up to 4x faster than the prior image model [14]. | Large impact where the provider exposes multiple models. Prefer the newest suitable model unless compatibility requires an older one. |
| Streaming / partial images | OpenAI supports streaming image generation and partial images [11]. Each partial image adds 100 image-output tokens [15]. | Improves perceived latency, not necessarily total elapsed time. Useful for interactive previews when supported, but not a replacement for job deadlines. |
| Service tier and variance | OpenAI says Standard tier can have wider latency variation and no guaranteed latency, while Priority/Scale have defined SLAs. OpenAI recommends checking p50/p75/p95 rather than averages [16]. | Tail-latency impact. Use p95/p99 from local logs for final timeout defaults, especially with OpenAI-compatible gateways. |
| SDK timeout / retries | Official Python and Node SDKs default to 10-minute request timeouts and retry timeout errors twice by default [17][18]. | Important operational trap. A CLI can appear hung or fail late if retries/timeouts are not visible. `gptx` exposes `--timeout` and background jobs for this reason. |

## Community And Anecdotal Evidence

Use these as directional evidence only; provider, model routing, queueing, and account tier can change results.

| Source type | Reported observation | How to use it |
|---|---|---|
| Independent blog summary | MindStudio reports GPT Image 1 often takes 30-45 seconds, with complex requests up to about 1 minute, and recommends setting UX expectations around 30-45 seconds [19]. | Sanity check for simple to moderate jobs. It does not cover heavy reference-image landing page comps. |
| Third-party benchmark article | CometAPI reports GPT Image 1.5 simple prompts around 3-8 seconds, moderate prompts around 7-12 seconds, complex/HD prompts around 12-25 seconds, plus peak-hour variation [20]. | Directional evidence that newer models can be faster, but not a guarantee for direct OpenAI or other gateways. |
| Reddit API user report | A user reported edit endpoint latency with a reference image and subject image rising from about 30-50 seconds to about 90 seconds [21]. | Weak but relevant evidence that reference-image edits can exceed a minute and change with platform load/model changes. |

## Timeout Defaults

These are engineering defaults, not OpenAI SLAs.

| CLI scenario | Recommended timeout |
|---|---:|
| Foreground, single text-to-image, `n=1`, `1024x1024`, low/medium | 3-5 minutes |
| Foreground, high quality, non-square size, or reference/edit workflow | 5-10 minutes |
| Background image jobs default | 20 minutes |
| Very heavy reference-image UI comps, high quality, non-square, multi-image input, or `n>1` | 30 minutes or more |
| Per-poll status/log/result checks | 30-60 seconds |

## Practical Guidance

1. Run `--dry-run --json` first to validate paths without paying for a remote call.
2. Use `--bg` for normal real image API calls so shell/session interruptions do not kill the task, then use `gptx job wait <job_id>` when the next step depends on the output.
3. Keep fast drafts cheap: use `n=1`, square/smaller sizes, and `quality low` or `medium`.
4. Treat `quality high`, `1536x1024`, `1024x1536`, reference images, masks, and dense UI screenshot prompts as long-tail jobs.
5. Raise timeout explicitly for unusually heavy jobs, for example `gptx --timeout 30m image generate ... --bg`.
6. Preserve useful variants with versioned output paths rather than overwriting.
7. Log or note model, size, quality, `n`, input image count, elapsed time, and request/job ID when diagnosing latency.
8. Benchmark the exact configured base URL. OpenAI-compatible gateways can differ from OpenAI's public latency profile.

## References

[1] OpenAI Image generation guide, cost and latency / token table: https://developers.openai.com/api/docs/guides/image-generation
[2] OpenAI Image generation guide, customize output and cost components: https://developers.openai.com/api/docs/guides/image-generation
[3] OpenAI Latency optimization guide: https://developers.openai.com/api/docs/guides/latency-optimization
[4] OpenAI Image generation guide, size and quality options: https://developers.openai.com/api/docs/guides/image-generation
[5] OpenAI Image generation guide, `n` parameter: https://developers.openai.com/api/docs/guides/image-generation
[6] OpenAI Images API reference: https://developers.openai.com/api/reference/resources/images
[7] OpenAI Image generation guide, reference images: https://developers.openai.com/api/docs/guides/image-generation
[8] OpenAI Image generation guide, image input fidelity: https://developers.openai.com/api/docs/guides/image-generation
[9] OpenAI Image generation guide, masks: https://developers.openai.com/api/docs/guides/image-generation
[10] OpenAI Latency optimization guide, output vs input tokens: https://developers.openai.com/api/docs/guides/latency-optimization
[11] OpenAI Image generation guide, streaming and revised prompt: https://developers.openai.com/api/docs/guides/image-generation
[12] OpenAI GPT Image 1 model page: https://developers.openai.com/api/docs/models/gpt-image-1
[13] OpenAI GPT Image 1.5 model page: https://developers.openai.com/api/docs/models/gpt-image-1.5
[14] OpenAI, “The new ChatGPT Images is here,” Dec. 16, 2025: https://openai.com/index/new-chatgpt-images-is-here/
[15] OpenAI Image generation guide, partial image cost: https://developers.openai.com/api/docs/guides/image-generation
[16] OpenAI Help Center, troubleshooting API errors and latency: https://help.openai.com/en/articles/1000499-troubleshooting-api-errors-and-latency
[17] OpenAI Python SDK README, timeouts and retries: https://github.com/openai/openai-python
[18] OpenAI Node SDK README, timeouts and retries: https://github.com/openai/openai-node
[19] MindStudio, “What Is GPT Image 1?”: https://www.mindstudio.ai/blog/what-is-gpt-image-1-openai
[20] CometAPI, “How Long Does ChatGPT Take to Generate an Image in 2026?”: https://www.cometapi.com/how-long-does-chatgpt-take-to-generate-an-image-in-2026/
[21] Reddit community report on `gpt-image-1` edit latency: https://www.reddit.com/r/OpenAI/comments/1n2526d/is_it_just_me_or_did_gptimage1_get_slower_after/
[22] OpenAI Background mode guide: https://developers.openai.com/api/docs/guides/background
