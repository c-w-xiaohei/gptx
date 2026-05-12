# Search With gptx

Use `gptx search` for cited research and web investigation.

For normal agent-driven deep research, use `--deep --json --bg` by default. Foreground deep searches are more likely to fail from network wobble, shell/tool timeouts, or session interruption before the remote Responses call finishes. Use foreground deep search only when the user explicitly needs immediate terminal output and accepts the timeout risk.

## Research Strategy

Treat serious research as an iterative investigation, not a single query. Run multiple focused searches from different angles, compare answers, and keep searching until the conclusion is specific, sourced, and decision-ready.

Research must be both broad and deep. Broad means covering the relevant official docs, primary sources, competing tools/options, recent changes, known failure modes, and dissenting evidence. Deep means following important claims back to high-quality sources, checking constraints and edge cases, and resolving contradictions before producing a recommendation. If the search process is shallow or narrow, do not treat the research as complete.

Any conclusion derived from research must include cited sources. A conclusion without source citations is invalid. Prefer official documentation, source repositories, standards/specifications, release notes, reputable technical writeups, and direct primary evidence. Avoid relying on unsourced summaries when a primary source can be found.

Never fabricate, infer beyond the evidence, or present unsupported claims as facts. Every research-based conclusion must be grounded completely in valid, high-quality sources. If sources are missing, weak, outdated, or contradictory, state the uncertainty instead of filling the gap.

Write each deep research prompt as a self-contained brief. Assume `gptx search` has no access to the current conversation, codebase, screenshots, previous tool output, or unstated context. Include the exact research question, named entities, spelling variants, timeframe, geography/platform/language constraints, source-quality requirements, primary-vs-secondary separation requirements, and the expected structure of the answer. If the user gave nuanced wording, preserve it in the query instead of compressing it into a vague topic.

Prefer this shape for deep research prompts:

```text
Find/verify <specific claim or object>. Context: <domain and why it matters>. Scope: <timeframe, platforms, languages, inclusions/exclusions>. Search for variants: <spellings, aliases, related terms>. Required evidence: <primary sources, canonical URLs, downloadable files, official docs, source repos, archived pages>. Output: <separate primary sources from secondary mentions, include exact quotes, cite every factual claim, state uncertainty>.
```

Match the research shape to the task's divergence:

- Narrow lookup: run 1-2 direct searches, then answer with citations.
- Moderate comparison: run several targeted searches across official docs, alternatives, failure modes, and current recommendations.
- Highly divergent research: split independent branches into subagent-driven research when the agent environment supports it. Give each subagent a focused question, require cited findings, then synthesize the results in the parent session.

Use subagent-driven research for broad or ambiguous tasks such as ecosystem comparisons, vendor/tool selection, release strategy, security/operational tradeoffs, or anything that naturally decomposes into independent viewpoints. Avoid subagents for simple fact lookups.

Continue searching across rounds until the evidence is high quality:

- Round 1: map the landscape and identify candidate answers.
- Round 2: verify official docs, current versions, constraints, and maintenance status.
- Round 3: resolve contradictions, check edge cases, and validate the recommended path.

Stop only when additional searches are unlikely to change the conclusion, or when remaining uncertainty can be stated clearly.

For independent subquestions, run searches in parallel when the agent environment supports parallel tool calls. Examples:

```sh
gptx search "best GitHub Actions for GoReleaser Go CLI releases" --deep --json --bg
gptx search "Go install module from GitHub latest tag workflow" --deep --json --bg
gptx search "OpenAI Responses API web_search citations examples" --deep --json --bg
```

Follow-up query patterns:

- Broad landscape: `gptx search "compare current options for <topic>" --deep --json --bg`
- Official docs: `gptx search "official documentation <tool> <feature>" --deep --json --bg`
- Failure mode: `gptx search "<error message> root cause fix" --deep --json --bg`
- Alternatives: `gptx search "<tool A> vs <tool B> tradeoffs" --deep --json --bg`
- Recent state: `gptx search "<topic> current status 2026" --deep --json --bg`

Avoid vague deep prompts:

- Bad: `gptx search "research this" --deep --json --bg`
- Bad: `gptx search "find sources for the phrase" --deep --json --bg`
- Good: `gptx search "Find the original source, exact full wording, and downloadable files/pages for 'impaccable ai slop' or 'impeccable ai slop'. Focus on AI image generation/design critique contexts. Identify the author, platform, canonical URL, exact phrase spelling variants, complete quote, and any source files/posts/slides that can be captured high-fidelity. Include citations and clearly separate primary sources from secondary mentions." --deep --json --bg`

## Command Examples

```sh
gptx search "current GoReleaser GitHub Action best practices"
gptx search "OpenAI Responses web_search examples" --deep --json --bg
gptx search "OpenAI Responses web_search examples" --deep --model gpt-5.5 --json --bg
gptx search "incident timeline" --deep --instructions-file ./instructions.txt --context ./incident-notes.md --json --bg
```

Use `--json` or `--format json` when another tool or agent needs to parse results. Use ordinary foreground search for quick lookups. Use `--deep --json --bg` for normal agent-driven long research so the session can continue while the remote call completes and avoid foreground timeout failures.

Use repeatable `--context <path>` to attach local text files to the query. The CLI reads each file before the API call and appends it with fixed file boundaries in flag order. Missing files fail locally. JSON output includes `context_files`, and `query` contains the final text sent to the API.

## Behavior

- Sends `POST /responses` through the configured OpenAI base URL.
- Uses model default `gpt-5.4-mini`.
- Deep search uses model default `gpt-5.5`, `reasoning.effort=high`, `web_search.search_context_size=high`, `max_tool_calls=8`, and `max_output_tokens=8000`.
- If a compatible gateway rejects `max_tool_calls`, deep search retries once without it. JSON output reports `compatibility_fallback`, `compatibility_fallback_reason`, and the effective `max_tool_calls` value.
- Supports model override via `--model`.
- Supports repeatable `--context <path>` for local text file context.
- Enables hosted `web_search`.
- Sends `store=false`.
- Uses list-style Responses input items.
- Keeps a citation-focused default prompt requiring `[number]` inline citations and a final `References` section.

## Long-Running Search

For normal agent-driven searches and long research queries, use `--deep --json --bg` so the session can continue while the remote Responses API call completes and the result remains machine-readable:

```sh
gptx search "compare Go release automation options" --deep --json --bg
```

Then wait for the returned job ID:

```sh
gptx job wait <job_id>
```

Use diagnostics only when needed:

```sh
gptx job status <job_id>
gptx job result <job_id>
gptx job logs <job_id> --stderr
```

Keep foreground execution for quick manual lookups when direct terminal output is more useful than a background job.
