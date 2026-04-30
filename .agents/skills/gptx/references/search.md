# Search With gptx

Use `gptx search` for cited research and web investigation.

## Research Strategy

Treat serious research as an iterative investigation, not a single query. Run multiple focused searches from different angles, compare answers, and keep searching until the conclusion is specific, sourced, and decision-ready.

Research must be both broad and deep. Broad means covering the relevant official docs, primary sources, competing tools/options, recent changes, known failure modes, and dissenting evidence. Deep means following important claims back to high-quality sources, checking constraints and edge cases, and resolving contradictions before producing a recommendation. If the search process is shallow or narrow, do not treat the research as complete.

Any conclusion derived from research must include cited sources. A conclusion without source citations is invalid. Prefer official documentation, source repositories, standards/specifications, release notes, reputable technical writeups, and direct primary evidence. Avoid relying on unsourced summaries when a primary source can be found.

Never fabricate, infer beyond the evidence, or present unsupported claims as facts. Every research-based conclusion must be grounded completely in valid, high-quality sources. If sources are missing, weak, outdated, or contradictory, state the uncertainty instead of filling the gap.

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
gptx search "best GitHub Actions for GoReleaser Go CLI releases" --json
gptx search "Go install module from GitHub latest tag workflow" --json
gptx search "OpenAI Responses API web_search citations examples" --json
```

Follow-up query patterns:

- Broad landscape: `gptx search "compare current options for <topic>" --json`
- Official docs: `gptx search "official documentation <tool> <feature>" --json`
- Failure mode: `gptx search "<error message> root cause fix" --json`
- Alternatives: `gptx search "<tool A> vs <tool B> tradeoffs" --json`
- Recent state: `gptx search "<topic> current status 2026" --json`

## Command Examples

```sh
gptx search "current GoReleaser GitHub Action best practices"
gptx search "OpenAI Responses web_search examples" --json
gptx search "OpenAI Responses web_search examples" --model gpt-5.4-mini --json
gptx search "incident timeline" --instructions-file ./instructions.txt --json
```

Use `--json` or `--format json` when another tool or agent needs to parse results.

## Behavior

- Sends `POST /responses` through the configured OpenAI base URL.
- Uses model default `gpt-5.4-mini`.
- Supports model override via `--model`.
- Enables hosted `web_search`.
- Sends `store=false`.
- Uses list-style Responses input items.
- Keeps a citation-focused default prompt requiring `[number]` inline citations and a final `References` section.

## Long-Running Search

For long searches, use a background process and inspect output files later:

```sh
mkdir -p /tmp/gptx-jobs
nohup gptx search "compare Go release automation options" --json \
  > /tmp/gptx-jobs/release-search.json \
  2> /tmp/gptx-jobs/release-search.err &
```

Then inspect:

```sh
ls -l /tmp/gptx-jobs
```

Prefer foreground execution with a longer command timeout when the agent tool supports explicit timeouts and output capture.
