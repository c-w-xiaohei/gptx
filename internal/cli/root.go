package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/c-w-xiaohei/gptx/internal/openaiapi"
	"github.com/spf13/cobra"
)

const (
	defaultTimeout = 20 * time.Minute
	defaultFormat  = "text"
)

var Version = "dev"

type rootOptions struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
	Format  string
	JSON    bool
}

func NewRootCommand() *cobra.Command {
	var root rootOptions

	cmd := &cobra.Command{
		Use:   "gptx",
		Short: "Search, generate images, and edit images with OpenAI",
		Long: `gptx helps agents and developers search the web, generate images, and edit images from a terminal.

Core features:
  - Search the web and return cited answers.
  - Generate images from prompts and save them locally.
  - Edit images with one or more input files and an optional mask.
  - Run search and image commands as local background jobs with job IDs.
  - Check local authentication and endpoint configuration with gptx status.

Authentication and config:
  - Env vars: GPTX_OPENAI_BASE_URL, GPTX_OPENAI_API_KEY.
  - Default base URL: https://api.openai.com/v1.
  - API key is required for API commands.

Image output behavior:
  - Decodes b64_json and writes image files.
  - Default templates:
      generate: gptx-image-{timestamp}-{index}.{ext}
      edit:     gptx-edit-{timestamp}-{index}.{ext}
  - Text mode prints saved paths one per line.
  - JSON mode emits one object with paths and metadata.
  - Raw base64 is never printed by default.

Background jobs:
  - For deep search and real image API calls, prefer --bg so long remote calls can continue outside the interactive session.
  - Use gptx job wait with the returned job ID to block until completion and print the final result.
  - Use gptx job status/result/logs to inspect progress, outputs, and diagnostics when needed.
  - Use foreground execution for help, status, version, and image --dry-run planning commands.

Implementation notes:
  - Search uses POST /responses with web_search, model default gpt-5.4-mini, store=false, and list-style input items.
  - Image generation uses POST /images/generations with model default gpt-image-2.
  - Image editing uses POST /images/edits multipart with model default gpt-image-2, repeatable --image, optional --mask.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&root.BaseURL, "base-url", "", "API base URL (env: GPTX_OPENAI_BASE_URL)")
	cmd.PersistentFlags().StringVar(&root.APIKey, "api-key", "", "API key (env: GPTX_OPENAI_API_KEY)")
	cmd.PersistentFlags().DurationVar(&root.Timeout, "timeout", defaultTimeout, "HTTP timeout (e.g. 30s, 2m, 20m)")
	cmd.PersistentFlags().StringVar(&root.Format, "format", defaultFormat, "output format: text or json")
	cmd.PersistentFlags().BoolVar(&root.JSON, "json", false, "shortcut for --format json")

	cmd.AddCommand(newSearchCommand(&root))
	cmd.AddCommand(newImageCommand(&root))
	cmd.AddCommand(newJobCommand(&root))
	cmd.AddCommand(newRunJobCommand(&root))
	cmd.AddCommand(newStatusCommand(&root))
	cmd.AddCommand(newVersionCommand(&root))
	cmd.AddCommand(newUpdateCommand(&root))

	cmd.Example = `  gptx version
  gptx version check
  gptx update
  gptx search "latest OpenAI Responses API updates" --model gpt-5.4-mini
  gptx search "openai responses web_search examples" --deep --json --bg
  gptx image generate "a calm coastal illustration" --n 2 --out-dir ./images --bg
  gptx image generate "logo concept" --dry-run --out ./logo.png --json
  gptx image generate "logo concept" --out ./logo.png --bg
  gptx image edit "remove the background" --dry-run --image ./in.png --mask ./mask.png --out ./edited.png --json
  gptx image edit "remove the background" --image ./in.png --mask ./mask.png --out ./edited.png --bg
  gptx job wait <job_id>
  gptx job status <job_id>
  gptx job result <job_id>

Config examples:
  export GPTX_OPENAI_API_KEY=***
  export GPTX_OPENAI_BASE_URL=https://api.openai.com/v1
  gptx search "what changed this week" --deep --bg`

	return cmd
}

type statusOutput struct {
	BaseURL          string `json:"base_url"`
	BaseURLSource    string `json:"base_url_source"`
	APIKeyConfigured bool   `json:"api_key_configured"`
	APIKeySource     string `json:"api_key_source,omitempty"`
	Authenticated    bool   `json:"authenticated"`
}

func newStatusCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local authentication and endpoint status",
		Long: `Show whether gptx can find a local API key and which base URL it will use.

This command only checks local flags and environment variables. It does not call a remote API and never prints the API key value.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := resolveStatus(root)
			if err != nil {
				return err
			}
			if isJSONOutput(root.Format, root.JSON, false) {
				return writeJSON(cmd, status)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "base_url: %s\nbase_url_source: %s\napi_key: %s\napi_key_source: %s\nauthenticated: %t\n",
				status.BaseURL,
				status.BaseURLSource,
				configuredLabel(status.APIKeyConfigured),
				valueOr(status.APIKeySource, "none"),
				status.Authenticated,
			)
			return err
		},
	}
}

func resolveStatus(root *rootOptions) (statusOutput, error) {
	if _, err := normalizeRootFormat(root.Format); err != nil {
		return statusOutput{}, err
	}

	baseURL, baseSource := resolveBaseURL(root.BaseURL)
	apiKey, keySource := resolveAPIKey(root.APIKey)
	configured := strings.TrimSpace(apiKey) != ""
	return statusOutput{
		BaseURL:          baseURL,
		BaseURLSource:    baseSource,
		APIKeyConfigured: configured,
		APIKeySource:     keySource,
		Authenticated:    configured,
	}, nil
}

func resolveBaseURL(flagValue string) (string, string) {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue, "--base-url"
	}
	if v := strings.TrimSpace(os.Getenv("GPTX_OPENAI_BASE_URL")); v != "" {
		return v, "GPTX_OPENAI_BASE_URL"
	}
	return openaiapi.DefaultBaseURL, "default"
}

func resolveAPIKey(flagValue string) (string, string) {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue, "--api-key"
	}
	if v := strings.TrimSpace(os.Getenv("GPTX_OPENAI_API_KEY")); v != "" {
		return v, "GPTX_OPENAI_API_KEY"
	}
	return "", ""
}

func configuredLabel(configured bool) string {
	if configured {
		return "configured"
	}
	return "missing"
}

func newVersionCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show gptx version",
		Long: `Show the local gptx version without checking the network.

Use gptx version check when you explicitly want to compare the local version with the latest GitHub release metadata. Set GPTX_NO_UPDATE_CHECK=1 to suppress network update checks where applicable.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := normalizeRootFormat(root.Format); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "gptx %s\n", currentVersion())
			return err
		},
	}
	cmd.AddCommand(newVersionCheckCommand(root))
	return cmd
}

func currentVersion() string {
	info, ok := debug.ReadBuildInfo()
	buildVersion := ""
	if ok {
		buildVersion = info.Main.Version
	}
	return resolveVersion(Version, buildVersion, ok)
}

func resolveVersion(injectedVersion, buildVersion string, hasBuildInfo bool) string {
	if strings.TrimSpace(injectedVersion) != "" && injectedVersion != "dev" {
		return injectedVersion
	}
	if hasBuildInfo && strings.TrimSpace(buildVersion) != "" && buildVersion != "(devel)" {
		return buildVersion
	}
	return "dev"
}

func resolveRootOptions(in *rootOptions, requireKey bool) (rootOptions, error) {
	out := *in
	if out.BaseURL == "" {
		out.BaseURL = firstNonEmpty(os.Getenv("GPTX_OPENAI_BASE_URL"), openaiapi.DefaultBaseURL)
	}
	if out.APIKey == "" {
		out.APIKey = firstNonEmpty(os.Getenv("GPTX_OPENAI_API_KEY"))
	}
	if out.Timeout <= 0 {
		out.Timeout = defaultTimeout
	}
	format, err := normalizeRootFormat(out.Format)
	if err != nil {
		return rootOptions{}, err
	}
	out.Format = format
	if requireKey && strings.TrimSpace(out.APIKey) == "" {
		return rootOptions{}, errors.New("API key is required (set GPTX_OPENAI_API_KEY, or pass --api-key)")
	}
	return out, nil
}

func normalizeRootFormat(format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = defaultFormat
	}
	if format != "text" && format != "json" {
		return "", fmt.Errorf("invalid --format %q (must be text or json)", format)
	}
	return format, nil
}

func writeJSON(cmd *cobra.Command, payload any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func isJSONOutput(format string, rootJSON bool, commandJSON bool) bool {
	if commandJSON || rootJSON {
		return true
	}
	return strings.EqualFold(format, "json")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
