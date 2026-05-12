package cli

import (
	"context"
	"errors"

	"github.com/c-w-xiaohei/gptx/internal/openaiapi"
	"github.com/spf13/cobra"
)

type imageOptions struct {
	Model             string
	Out               string
	OutDir            string
	Filename          string
	N                 int
	Size              string
	Quality           string
	OutputFormat      string
	Background        string
	OutputCompression int
	NExplicit         bool
	CompressionSet    bool
	Overwrite         bool
	CreateDirs        bool
	ContextFiles      []string
	DryRun            bool
	JSON              bool
	BackgroundJob     bool
}

type imageEditOptions struct {
	imageOptions
	Images        []string
	Mask          string
	InputFidelity string
}

type imageGenerateOptions struct {
	imageOptions
	Images        []string
	Moderation    string
	InputFidelity string
}

func newImageCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "image", Short: "Image generation and editing commands"}
	cmd.AddCommand(newImageGenerateCommand(root))
	cmd.AddCommand(newImageEditCommand(root))
	return cmd
}

func newImageGenerateCommand(root *rootOptions) *cobra.Command {
	var opts imageGenerateOptions
	cmd := &cobra.Command{
		Use:   "generate <prompt>",
		Short: "Generate images from text and optional reference images",
		Long: `Generate creates new image files from a prompt.

Without --image, it sends a request to /images/generations.
With one or more --image reference attachments, it uses /images/edits internally
so design systems, brand references, and style images can guide generation.

Output naming:
  - Default template: gptx-image-{timestamp}-{index}.{ext}
  - Use --out only for n=1
  - Use --out-dir and/or --filename for custom paths
  - Text mode prints one saved path per line
  - JSON mode returns one object with paths and metadata

Run guidance:
  - First use --dry-run --json in the foreground to validate paths without API calls, uploads, or writes.
  - Use repeatable --context to append text files such as briefs, copy, or SVG source as prompt context.
  - For real image API calls, prefer --bg so long generation runs can continue as local background jobs.`,
		Example: `  gptx image generate "an isometric city" --n 3 --out-dir ./out
  gptx image generate "brand icon" --dry-run --out ./icon.png --json
  gptx image generate "brand icon" --out ./icon.png --bg
  gptx image generate "use this logo direction" --context ./logo.svg --out ./logo-card.png --bg
  gptx image generate "match this design system" --image ./design-system.png --out ./screen.png --bg
  gptx image generate "poster" --size 1536x1024 --quality high --output-format webp --json --bg`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.NExplicit = cmd.Flags().Changed("n")
			opts.CompressionSet = cmd.Flags().Changed("output-compression")
			if opts.DryRun && opts.BackgroundJob {
				return errors.New("--dry-run and --bg cannot be used together; run dry-run in the foreground, then remove --dry-run and use --bg for the real call")
			}
			opts.imageOptions = opts.imageOptions.imageDefaults()
			prompt, err := promptWithContextFiles(args[0], opts.ContextFiles)
			if err != nil {
				return err
			}
			resolvedRoot, err := resolveRootOptions(root, !opts.DryRun)
			if err != nil {
				return err
			}
			if err := validateImageOptions(opts.imageOptions); err != nil {
				return err
			}
			if err := validateGenerateOnlyOptions(opts); err != nil {
				return err
			}
			if len(opts.Images) > 0 {
				if err := openaiapi.ValidateEditImageInputs(opts.Images, ""); err != nil {
					return err
				}
			}
			paths, err := planOutputPaths("gptx-image", opts.imageOptions)
			if err != nil {
				return err
			}
			if err := validatePlannedPaths(paths, opts.imageOptions); err != nil {
				return err
			}
			jsonOut := isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, opts.JSON)
			if opts.DryRun {
				return writeImageOutput(cmd, jsonOut, imageOutputJSON{
					Command:           "image generate",
					Model:             opts.Model,
					Prompt:            prompt,
					ContextFiles:      opts.ContextFiles,
					Count:             len(paths),
					Paths:             paths,
					OutputFormat:      opts.OutputFormat,
					Size:              opts.Size,
					Quality:           opts.Quality,
					Background:        opts.Background,
					Moderation:        opts.Moderation,
					OutputCompression: intPtrIfSet(opts.OutputCompression),
					InputFidelity:     opts.InputFidelity,
					DryRun:            true,
				})
			}
			if opts.BackgroundJob {
				if rootAPIKeyFlagChanged(cmd) {
					return errors.New("--api-key is not supported for background jobs; use GPTX_OPENAI_API_KEY instead")
				}
				jobArgs := append([]string{"image", "generate", args[0]}, commandFlagArgs(cmd, []string{"bg"})...)
				out, err := startBackgroundJob(jobArgs, resolvedRoot, "image.generate")
				if err != nil {
					return err
				}
				return writeJobStartOutput(cmd, jsonOut, out)
			}

			client := openaiapi.NewClient(resolvedRoot.BaseURL, resolvedRoot.APIKey, nil)
			ctx, cancel := context.WithTimeout(context.Background(), resolvedRoot.Timeout)
			defer cancel()

			var res openaiapi.ImageResults
			if len(opts.Images) > 0 {
				compression := intPtrIfSet(opts.OutputCompression)
				res, err = client.EditImage(ctx, openaiapi.EditImageRequest{
					Model:             opts.Model,
					Prompt:            prompt,
					Images:            opts.Images,
					Size:              opts.Size,
					Quality:           opts.Quality,
					N:                 opts.N,
					OutputFormat:      opts.OutputFormat,
					Background:        opts.Background,
					OutputCompression: compression,
					InputFidelity:     opts.InputFidelity,
				})
			} else {
				compression := intPtrIfSet(opts.OutputCompression)
				res, err = client.GenerateImage(ctx, openaiapi.ImageRequest{
					Model:             opts.Model,
					Prompt:            prompt,
					Size:              opts.Size,
					Quality:           opts.Quality,
					N:                 opts.N,
					OutputFormat:      opts.OutputFormat,
					Background:        opts.Background,
					OutputCompression: compression,
					Moderation:        opts.Moderation,
				})
			}
			if err != nil {
				return err
			}

			saved, revised, err := saveImages(paths, res.Images, opts.imageOptions)
			if err != nil {
				return err
			}

			return writeImageOutput(cmd, jsonOut, imageOutputJSON{
				Command:           "image generate",
				Model:             opts.Model,
				Prompt:            prompt,
				ContextFiles:      opts.ContextFiles,
				Count:             len(saved),
				Paths:             saved,
				OutputFormat:      opts.OutputFormat,
				Size:              opts.Size,
				Quality:           opts.Quality,
				Background:        opts.Background,
				Moderation:        opts.Moderation,
				OutputCompression: intPtrIfSet(opts.OutputCompression),
				InputFidelity:     opts.InputFidelity,
				RevisedPrompt:     revised,
				DryRun:            opts.DryRun,
			})
		},
	}
	bindImageFlags(cmd, &opts.imageOptions)
	cmd.Flags().StringArrayVar(&opts.Images, "image", nil, "reference image path (repeatable; uses /images/edits internally)")
	cmd.Flags().StringVar(&opts.Moderation, "moderation", "", "moderation level (auto, low; generate-only)")
	cmd.Flags().StringVar(&opts.InputFidelity, "input-fidelity", "", "input fidelity (high, low; requires --image on generate)")
	return cmd
}

func newImageEditCommand(root *rootOptions) *cobra.Command {
	var opts imageEditOptions
	cmd := &cobra.Command{
		Use:   "edit <prompt>",
		Short: "Edit images via /images/edits multipart",
		Long: `Edit sends a multipart request to /images/edits (model default gpt-image-2)
with repeatable --image flags and an optional --mask.

Output naming:
  - Default template: gptx-edit-{timestamp}-{index}.{ext}
  - Use --out only for n=1
  - Text mode prints one saved path per line
  - JSON mode returns one object with paths and metadata

Run guidance:
  - First use --dry-run --json in the foreground to validate paths and inputs without API calls, uploads, or writes.
  - Use repeatable --context to append text files such as briefs, copy, or SVG source as prompt context.
  - For real image API calls, prefer --bg so long edit runs can continue as local background jobs.`,
		Example: `  gptx image edit "remove background" --image ./in.png --out ./out.png
  gptx image edit "remove background" --dry-run --image ./in.png --out ./out.png --json
  gptx image edit "remove background" --image ./in.png --out ./out.png --bg
  gptx image edit "apply this logo guidance" --image ./screen.png --context ./logo.svg --out ./out.png --bg
  gptx image edit "replace sky" --image ./in.png --mask ./mask.png --n 2 --out-dir ./edits --bg
  gptx image edit "merge style" --image ./a.png --image ./b.png --output-format png --json --bg`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.NExplicit = cmd.Flags().Changed("n")
			opts.CompressionSet = cmd.Flags().Changed("output-compression")
			if opts.DryRun && opts.BackgroundJob {
				return errors.New("--dry-run and --bg cannot be used together; run dry-run in the foreground, then remove --dry-run and use --bg for the real call")
			}
			opts.imageOptions = opts.imageOptions.imageDefaults()
			prompt, err := promptWithContextFiles(args[0], opts.ContextFiles)
			if err != nil {
				return err
			}
			resolvedRoot, err := resolveRootOptions(root, !opts.DryRun)
			if err != nil {
				return err
			}
			if !opts.DryRun && len(opts.Images) == 0 {
				return errors.New("at least one --image is required")
			}
			if err := validateImageOptions(opts.imageOptions); err != nil {
				return err
			}
			if err := validateInputFidelity(opts.InputFidelity); err != nil {
				return err
			}
			if len(opts.Images) > 0 || opts.Mask != "" {
				if err := openaiapi.ValidateEditImageInputs(opts.Images, opts.Mask); err != nil {
					return err
				}
			}
			paths, err := planOutputPaths("gptx-edit", opts.imageOptions)
			if err != nil {
				return err
			}
			if err := validatePlannedPaths(paths, opts.imageOptions); err != nil {
				return err
			}
			jsonOut := isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, opts.JSON)
			if opts.DryRun {
				return writeImageOutput(cmd, jsonOut, imageOutputJSON{
					Command:           "image edit",
					Model:             opts.Model,
					Prompt:            prompt,
					ContextFiles:      opts.ContextFiles,
					Count:             len(paths),
					Paths:             paths,
					OutputFormat:      opts.OutputFormat,
					Size:              opts.Size,
					Quality:           opts.Quality,
					Background:        opts.Background,
					OutputCompression: intPtrIfSet(opts.OutputCompression),
					InputFidelity:     opts.InputFidelity,
					DryRun:            true,
				})
			}
			if opts.BackgroundJob {
				if rootAPIKeyFlagChanged(cmd) {
					return errors.New("--api-key is not supported for background jobs; use GPTX_OPENAI_API_KEY instead")
				}
				jobArgs := append([]string{"image", "edit", args[0]}, commandFlagArgs(cmd, []string{"bg"})...)
				out, err := startBackgroundJob(jobArgs, resolvedRoot, "image.edit")
				if err != nil {
					return err
				}
				return writeJobStartOutput(cmd, jsonOut, out)
			}

			client := openaiapi.NewClient(resolvedRoot.BaseURL, resolvedRoot.APIKey, nil)
			ctx, cancel := context.WithTimeout(context.Background(), resolvedRoot.Timeout)
			defer cancel()

			compression := intPtrIfSet(opts.OutputCompression)
			res, err := client.EditImage(ctx, openaiapi.EditImageRequest{
				Model:             opts.Model,
				Prompt:            prompt,
				Images:            opts.Images,
				Mask:              opts.Mask,
				Size:              opts.Size,
				Quality:           opts.Quality,
				N:                 opts.N,
				OutputFormat:      opts.OutputFormat,
				Background:        opts.Background,
				OutputCompression: compression,
				InputFidelity:     opts.InputFidelity,
			})
			if err != nil {
				return err
			}

			saved, revised, err := saveImages(paths, res.Images, opts.imageOptions)
			if err != nil {
				return err
			}

			return writeImageOutput(cmd, jsonOut, imageOutputJSON{
				Command:           "image edit",
				Model:             opts.Model,
				Prompt:            prompt,
				ContextFiles:      opts.ContextFiles,
				Count:             len(saved),
				Paths:             saved,
				OutputFormat:      opts.OutputFormat,
				Size:              opts.Size,
				Quality:           opts.Quality,
				Background:        opts.Background,
				OutputCompression: intPtrIfSet(opts.OutputCompression),
				InputFidelity:     opts.InputFidelity,
				RevisedPrompt:     revised,
				DryRun:            opts.DryRun,
			})
		},
	}
	bindImageFlags(cmd, &opts.imageOptions)
	cmd.Flags().StringArrayVar(&opts.Images, "image", nil, "input image path (repeatable for edits)")
	cmd.Flags().StringVar(&opts.Mask, "mask", "", "optional mask image path")
	cmd.Flags().StringVar(&opts.InputFidelity, "input-fidelity", "", "input fidelity (high, low)")
	return cmd
}

func bindImageFlags(cmd *cobra.Command, opts *imageOptions) {
	cmd.Flags().StringVar(&opts.Model, "model", openaiapi.DefaultImageModel, "image model")
	cmd.Flags().StringVar(&opts.Out, "out", "", "output file path (valid only when --n=1)")
	cmd.Flags().StringVar(&opts.OutDir, "out-dir", "", "output directory for generated files")
	cmd.Flags().StringVar(&opts.Filename, "filename", "", "filename template override (supports {timestamp}, {index}, {ext})")
	cmd.Flags().IntVar(&opts.N, "n", 1, "number of images")
	cmd.Flags().StringVar(&opts.Size, "size", "1024x1024", "image size (e.g. 1024x1024)")
	cmd.Flags().StringVar(&opts.Quality, "quality", "low", "image quality")
	cmd.Flags().StringVar(&opts.OutputFormat, "output-format", "png", "output format (png, webp, jpeg)")
	cmd.Flags().StringVar(&opts.Background, "background", "", "background mode (auto, transparent, opaque)")
	cmd.Flags().IntVar(&opts.OutputCompression, "output-compression", -1, "output compression 0..100 (jpeg/webp only)")
	if flag := cmd.Flags().Lookup("output-compression"); flag != nil {
		flag.DefValue = "unset"
	}
	cmd.Flags().BoolVar(&opts.Overwrite, "overwrite", false, "overwrite existing files")
	cmd.Flags().BoolVar(&opts.CreateDirs, "create-dirs", false, "create output directories if missing")
	cmd.Flags().StringArrayVar(&opts.ContextFiles, "context", nil, "text context file path to append to the prompt (repeatable)")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "compute output paths without writing files")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "emit JSON output")
	cmd.Flags().BoolVar(&opts.BackgroundJob, "bg", false, "run as a local background job and print a job ID")
}

func (o imageOptions) imageDefaults() imageOptions {
	out := o
	if out.N == 0 && !out.NExplicit {
		out.N = 1
	}
	if out.Size == "" {
		out.Size = "1024x1024"
	}
	if out.Quality == "" {
		out.Quality = "low"
	}
	if out.OutputFormat == "" {
		out.OutputFormat = "png"
	}
	if out.Model == "" {
		out.Model = openaiapi.DefaultImageModel
	}
	if normalized, err := normalizeOutputFormat(out.OutputFormat); err == nil {
		out.OutputFormat = normalized
	}
	return out
}
