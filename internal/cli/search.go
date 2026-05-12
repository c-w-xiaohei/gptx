package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/c-w-xiaohei/gptx/internal/openaiapi"
	"github.com/spf13/cobra"
)

type searchOptions struct {
	Model             string
	Instructions      string
	InstructionsFile  string
	ContextFiles      []string
	JSON              bool
	NoStream          bool
	BackgroundJob     bool
	Deep              bool
	ReasoningEffort   string
	SearchContextSize string
	MaxToolCalls      int
	MaxOutputTokens   int
}

func newSearchCommand(root *rootOptions) *cobra.Command {
	var opts searchOptions
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search and answer using /responses + web_search",
		Long: `Search sends a Responses API request to /responses with web_search enabled.

Defaults:
  - ordinary search model: gpt-5.4-mini
  - store: false
  - input: sent as a list item (not plain string)
  - instructions: citation-focused prompt requiring [number] inline citations and a References section.

Deep search:
  - enable with --deep for high-effort cited research.
  - model defaults to gpt-5.5 unless --model overrides it.
  - defaults to reasoning.effort=high, web_search.search_context_size=high,
    max_tool_calls=8, and max_output_tokens=8000.
  - --bg is only supported with --deep for search.

Notes:
  - Streaming is enabled by default; --no-stream is accepted for compatibility but currently has no effect.
  - Use repeatable --context to append text files to the query with fixed file boundaries.

Run guidance:
  - Ordinary search remains foreground-oriented for quick lookups.
  - Prefer --deep --bg for long research so the session can continue while the remote search completes.`,
		Example: `  gptx search "golang cobra cli examples"
  gptx search "best practices for OpenAI responses prompts" --deep --bg
  gptx search "summarize this topic" --instructions "Be concise." --json
  gptx search "incident timeline" --deep --instructions-file ./instructions.txt --context ./notes.md --json --bg`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Instructions != "" && opts.InstructionsFile != "" {
				return errors.New("--instructions and --instructions-file are mutually exclusive")
			}
			if opts.BackgroundJob && rootAPIKeyFlagChanged(cmd) {
				return errors.New("--api-key is not supported for background jobs; use GPTX_OPENAI_API_KEY instead")
			}
			if err := validateSearchOptions(opts); err != nil {
				return err
			}
			resolvedRoot, err := resolveRootOptions(root, true)
			if err != nil {
				return err
			}
			if opts.BackgroundJob {
				jobArgs := append([]string{"search", args[0]}, commandFlagArgs(cmd, []string{"bg"})...)
				out, err := startBackgroundJob(jobArgs, resolvedRoot, "search")
				if err != nil {
					return err
				}
				return writeJobStartOutput(cmd, isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, opts.JSON), out)
			}
			instructions := opts.Instructions
			if opts.InstructionsFile != "" {
				b, err := os.ReadFile(opts.InstructionsFile)
				if err != nil {
					return fmt.Errorf("read instructions file: %w", err)
				}
				instructions = string(b)
			}
			input, err := promptWithContextFiles(args[0], opts.ContextFiles)
			if err != nil {
				return err
			}

			client := openaiapi.NewClient(resolvedRoot.BaseURL, resolvedRoot.APIKey, nil)
			ctx, cancel := context.WithTimeout(context.Background(), resolvedRoot.Timeout)
			defer cancel()

			res, err := client.Search(ctx, openaiapi.SearchRequest{
				Model:             searchModel(opts),
				Instructions:      instructions,
				Input:             input,
				Deep:              opts.Deep,
				ReasoningEffort:   opts.ReasoningEffort,
				SearchContextSize: opts.SearchContextSize,
				MaxToolCalls:      opts.MaxToolCalls,
				MaxOutputTokens:   opts.MaxOutputTokens,
			})
			if err != nil {
				return err
			}

			if isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, opts.JSON) {
				payload := map[string]any{
					"command":             "search",
					"deep":                opts.Deep,
					"model":               res.Model,
					"reasoning_effort":    res.ReasoningEffort,
					"search_context_size": res.SearchContextSize,
					"max_tool_calls":      res.MaxToolCalls,
					"max_output_tokens":   res.MaxOutputTokens,
					"query":               input,
					"text":                res.Text,
				}
				if len(opts.ContextFiles) > 0 {
					payload["context_files"] = opts.ContextFiles
				}
				if res.CompatibilityFallback {
					payload["compatibility_fallback"] = true
					payload["compatibility_fallback_reason"] = res.CompatibilityFallbackReason
				}
				return writeJSON(cmd, payload)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), res.Text)
			return err
		},
	}

	cmd.Flags().StringVar(&opts.Model, "model", "", "Responses model (default gpt-5.4-mini; gpt-5.5 with --deep)")
	cmd.Flags().StringVar(&opts.Instructions, "instructions", "", "override default search instructions")
	cmd.Flags().StringVar(&opts.InstructionsFile, "instructions-file", "", "load instructions from a file")
	cmd.Flags().StringArrayVar(&opts.ContextFiles, "context", nil, "text context file path to append to the query (repeatable)")
	cmd.Flags().BoolVar(&opts.NoStream, "no-stream", false, "accepted for compatibility; currently no effect")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "emit JSON output")
	cmd.Flags().BoolVar(&opts.BackgroundJob, "bg", false, "run deep search as a local background job and print a job ID")
	cmd.Flags().BoolVar(&opts.Deep, "deep", false, "enable high-effort deep search defaults")
	cmd.Flags().StringVar(&opts.ReasoningEffort, "reasoning-effort", "", "deep search reasoning effort: low, medium, high, xhigh (default high with --deep)")
	cmd.Flags().StringVar(&opts.SearchContextSize, "search-context-size", "", "deep search web context size: low, medium, high (default high with --deep)")
	cmd.Flags().IntVar(&opts.MaxToolCalls, "max-tool-calls", 0, "deep search maximum web_search tool calls (default 8 with --deep)")
	cmd.Flags().IntVar(&opts.MaxOutputTokens, "max-output-tokens", 0, "deep search maximum output tokens (default 8000 with --deep)")
	return cmd
}

func validateSearchOptions(opts searchOptions) error {
	if opts.BackgroundJob && !opts.Deep {
		return errors.New("--bg is only supported with --deep for search")
	}
	if !opts.Deep && (opts.ReasoningEffort != "" || opts.SearchContextSize != "" || opts.MaxToolCalls != 0 || opts.MaxOutputTokens != 0) {
		return errors.New("deep search tuning flags require --deep")
	}
	if opts.ReasoningEffort != "" && !oneOf(opts.ReasoningEffort, "low", "medium", "high", "xhigh") {
		return errors.New("invalid --reasoning-effort: must be one of low, medium, high, xhigh")
	}
	if opts.SearchContextSize != "" && !oneOf(opts.SearchContextSize, "low", "medium", "high") {
		return errors.New("invalid --search-context-size: must be one of low, medium, high")
	}
	if opts.MaxToolCalls < 0 {
		return errors.New("--max-tool-calls must be greater than or equal to 0")
	}
	if opts.MaxOutputTokens < 0 {
		return errors.New("--max-output-tokens must be greater than or equal to 0")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func searchModel(opts searchOptions) string {
	if opts.Model != "" {
		return opts.Model
	}
	if opts.Deep {
		return openaiapi.DefaultDeepSearchModel
	}
	return openaiapi.DefaultSearchModel
}

func deepStringValue(deep bool, value, defaultValue string) string {
	if !deep {
		return ""
	}
	return valueOr(value, defaultValue)
}

func deepIntValue(deep bool, value, defaultValue int) int {
	if !deep {
		return 0
	}
	if value != 0 {
		return value
	}
	return defaultValue
}
