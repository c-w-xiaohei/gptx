package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/c-w-xiaohei/gptx/internal/openaiapi"
	"github.com/spf13/cobra"
)

type imageOutputJSON struct {
	Command           string   `json:"command"`
	Model             string   `json:"model"`
	Prompt            string   `json:"prompt"`
	Count             int      `json:"count"`
	Paths             []string `json:"paths"`
	OutputFormat      string   `json:"output_format"`
	Size              string   `json:"size,omitempty"`
	Quality           string   `json:"quality,omitempty"`
	Background        string   `json:"background,omitempty"`
	Moderation        string   `json:"moderation,omitempty"`
	OutputCompression *int     `json:"output_compression,omitempty"`
	InputFidelity     string   `json:"input_fidelity,omitempty"`
	RevisedPrompt     []string `json:"revised_prompt,omitempty"`
	DryRun            bool     `json:"dry_run"`
}

func validateImageOptions(opts imageOptions) error {
	if opts.NExplicit && opts.N < 1 {
		return errors.New("--n must be between 1 and 10")
	}
	if opts.N < 1 || opts.N > 10 {
		return errors.New("--n must be between 1 and 10")
	}
	if opts.Out != "" && opts.N != 1 {
		return errors.New("--out is valid only when --n=1")
	}
	format, err := normalizeOutputFormat(opts.OutputFormat)
	if err != nil {
		return err
	}
	if err := validateBackground(opts.Background, format); err != nil {
		return err
	}
	if err := validateOutputCompression(opts.OutputCompression, format, opts.CompressionSet); err != nil {
		return err
	}
	return nil
}

func validateGenerateOnlyOptions(opts imageGenerateOptions) error {
	if err := validateModeration(opts.Moderation); err != nil {
		return err
	}
	if err := validateInputFidelity(opts.InputFidelity); err != nil {
		return err
	}
	if opts.InputFidelity != "" && len(opts.Images) == 0 {
		return errors.New("--input-fidelity requires at least one --image")
	}
	if opts.Moderation != "" && len(opts.Images) > 0 {
		return errors.New("--moderation is not supported with --image")
	}
	return nil
}

func validateBackground(background, outputFormat string) error {
	if background == "" {
		return nil
	}
	switch background {
	case "auto", "transparent", "opaque":
	default:
		return fmt.Errorf("invalid --background %q (must be one of: auto, transparent, opaque)", background)
	}
	if background == "transparent" && outputFormat != "png" && outputFormat != "webp" {
		return errors.New("--background transparent requires --output-format png or webp")
	}
	return nil
}

func validateOutputCompression(v int, outputFormat string, explicit bool) error {
	if v < 0 && !explicit {
		return nil
	}
	if v < 0 {
		return errors.New("--output-compression must be between 0 and 100")
	}
	if v > 100 {
		return errors.New("--output-compression must be between 0 and 100")
	}
	if outputFormat != "jpeg" && outputFormat != "webp" {
		return errors.New("--output-compression requires --output-format jpeg or webp")
	}
	return nil
}

func validateModeration(m string) error {
	if m == "" {
		return nil
	}
	if m != "auto" && m != "low" {
		return fmt.Errorf("invalid --moderation %q (must be one of: auto, low)", m)
	}
	return nil
}

func validateInputFidelity(v string) error {
	if v == "" {
		return nil
	}
	if v != "high" && v != "low" {
		return fmt.Errorf("invalid --input-fidelity %q (must be one of: high, low)", v)
	}
	return nil
}

func intPtrIfSet(v int) *int {
	if v < 0 {
		return nil
	}
	copy := v
	return &copy
}

func planOutputPaths(prefix string, opts imageOptions) ([]string, error) {
	timestamp := time.Now().UTC().Format("20060102-150405")
	paths := make([]string, 0, opts.N)
	for i := 1; i <= opts.N; i++ {
		path, err := outputPath(prefix, timestamp, i, opts)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func validatePlannedPaths(paths []string, opts imageOptions) error {
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			return fmt.Errorf("duplicate output path planned: %s", path)
		}
		seen[path] = struct{}{}

		dir := filepath.Dir(path)
		if st, err := os.Stat(dir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if !opts.CreateDirs {
					return fmt.Errorf("output directory does not exist: %s (use --create-dirs)", dir)
				}
			} else {
				return fmt.Errorf("check output directory %q: %w", dir, err)
			}
		} else if !st.IsDir() {
			return fmt.Errorf("output directory is not a directory: %s", dir)
		}

		st, err := os.Stat(path)
		if err == nil {
			if !st.Mode().IsRegular() {
				return fmt.Errorf("output path must be a regular file path: %s", path)
			}
			if err := validateOutputPathExtension(path, opts.OutputFormat); err != nil {
				return err
			}
			if !opts.Overwrite {
				return fmt.Errorf("output file already exists: %s (use --overwrite)", path)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check output file %q: %w", path, err)
		} else {
			if err := validateOutputPathExtension(path, opts.OutputFormat); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateOutputPathExtension(path string, outputFormat string) error {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" {
		return errors.New("output path must include extension matching --output-format")
	}
	if ext == "jpg" {
		ext = "jpeg"
	}
	normalized, err := normalizeOutputFormat(outputFormat)
	if err != nil {
		return err
	}
	if ext != normalized {
		return fmt.Errorf("output path extension %q does not match --output-format %q", ext, normalized)
	}
	return nil
}

func saveImages(paths []string, images []openaiapi.ImageResult, opts imageOptions) ([]string, []string, error) {
	if len(paths) != len(images) {
		return nil, nil, fmt.Errorf("planned paths (%d) do not match image count (%d)", len(paths), len(images))
	}
	written := make([]string, 0, len(paths))
	revised := make([]string, 0, len(images))
	decoded := make([][]byte, 0, len(images))

	for i, img := range images {
		if img.RevisedPrompt != "" {
			revised = append(revised, img.RevisedPrompt)
		}
		data, err := base64.StdEncoding.DecodeString(img.B64JSON)
		if err != nil {
			return nil, nil, fmt.Errorf("decode image %d: %w", i+1, err)
		}
		decoded = append(decoded, data)
	}

	for i, data := range decoded {
		path := paths[i]
		if err := writeFile(path, data, opts); err != nil {
			for _, writtenPath := range written {
				_ = os.Remove(writtenPath)
			}
			return nil, nil, err
		}
		written = append(written, path)
	}

	return written, revised, nil
}

func outputPath(prefix, timestamp string, index int, opts imageOptions) (string, error) {
	if opts.Out != "" {
		return opts.Out, nil
	}
	ext, err := normalizeOutputFormat(opts.OutputFormat)
	if err != nil {
		return "", err
	}
	template := opts.Filename
	if template == "" {
		template = prefix + "-{timestamp}-{index}.{ext}"
	}
	file := strings.ReplaceAll(template, "{timestamp}", timestamp)
	file = strings.ReplaceAll(file, "{index}", strconv.Itoa(index))
	file = strings.ReplaceAll(file, "{ext}", ext)
	if !strings.Contains(filepath.Base(file), ".") {
		file += "." + ext
	}
	if opts.OutDir != "" {
		file = filepath.Join(opts.OutDir, file)
	}
	return file, nil
}

func writeFile(path string, data []byte, opts imageOptions) error {
	dir := filepath.Dir(path)
	if opts.CreateDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %q: %w", dir, err)
		}
	}
	if st, err := os.Stat(path); err == nil {
		if !st.Mode().IsRegular() {
			return fmt.Errorf("output path must be a regular file path: %s", path)
		}
		if !opts.Overwrite {
			return fmt.Errorf("output file already exists: %s (use --overwrite)", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check output file %q: %w", path, err)
	}

	tmp, err := os.CreateTemp(dir, ".gptx-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %q: %w", dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file %q: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file %q to %q: %w", tmpPath, path, err)
	}
	cleanup = false
	return nil
}

func writeImageOutput(cmd *cobra.Command, jsonOut bool, payload imageOutputJSON) error {
	if jsonOut {
		return writeJSON(cmd, payload)
	}
	for _, p := range payload.Paths {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), p); err != nil {
			return err
		}
	}
	return nil
}

func normalizeOutputFormat(in string) (string, error) {
	ext := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(in), "."))
	if ext == "" {
		return "", errors.New("--output-format is required")
	}
	switch ext {
	case "png", "webp", "jpeg":
		return ext, nil
	default:
		return "", fmt.Errorf("invalid --output-format %q (must be one of: png, webp, jpeg)", in)
	}
}
