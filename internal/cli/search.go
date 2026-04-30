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
	Model            string
	Instructions     string
	InstructionsFile string
	JSON             bool
	NoStream         bool
}

func newSearchCommand(root *rootOptions) *cobra.Command {
	var opts searchOptions
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search and answer using /responses + web_search",
		Long: `Search sends a Responses API request to /responses with web_search enabled.

Defaults:
  - model: gpt-5.4-mini
  - store: false
  - input: sent as a list item (not plain string)
  - instructions: citation-focused prompt requiring [number] inline citations and a References section.

Notes:
  - Streaming is enabled by default; --no-stream is accepted for compatibility but currently has no effect.`,
		Example: `  gptx search "best practices for OpenAI responses prompts"
  gptx search "golang cobra cli examples" --model gpt-5.4-mini
  gptx search "summarize this topic" --instructions "Be concise." --json
  gptx search "incident timeline" --instructions-file ./instructions.txt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedRoot, err := resolveRootOptions(root, true)
			if err != nil {
				return err
			}
			if opts.Instructions != "" && opts.InstructionsFile != "" {
				return errors.New("--instructions and --instructions-file are mutually exclusive")
			}
			instructions := opts.Instructions
			if opts.InstructionsFile != "" {
				b, err := os.ReadFile(opts.InstructionsFile)
				if err != nil {
					return fmt.Errorf("read instructions file: %w", err)
				}
				instructions = string(b)
			}

			client := openaiapi.NewClient(resolvedRoot.BaseURL, resolvedRoot.APIKey, nil)
			ctx, cancel := context.WithTimeout(context.Background(), resolvedRoot.Timeout)
			defer cancel()

			res, err := client.Search(ctx, openaiapi.SearchRequest{
				Model:        opts.Model,
				Instructions: instructions,
				Input:        args[0],
			})
			if err != nil {
				return err
			}

			if isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, opts.JSON) {
				payload := map[string]any{
					"command": "search",
					"model":   valueOr(opts.Model, openaiapi.DefaultSearchModel),
					"query":   args[0],
					"text":    res.Text,
				}
				return writeJSON(cmd, payload)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), res.Text)
			return err
		},
	}

	cmd.Flags().StringVar(&opts.Model, "model", openaiapi.DefaultSearchModel, "Responses model (default gpt-5.4-mini)")
	cmd.Flags().StringVar(&opts.Instructions, "instructions", "", "override default search instructions")
	cmd.Flags().StringVar(&opts.InstructionsFile, "instructions-file", "", "load instructions from a file")
	cmd.Flags().BoolVar(&opts.NoStream, "no-stream", false, "accepted for compatibility; currently no effect")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "emit JSON output")
	return cmd
}
