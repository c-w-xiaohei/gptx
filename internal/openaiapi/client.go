package openaiapi

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	_ "golang.org/x/image/webp"
)

const (
	DefaultBaseURL         = "https://api.openai.com/v1"
	DefaultSearchModel     = "gpt-5.4-mini"
	DefaultDeepSearchModel = "gpt-5.5"
	DefaultImageModel      = "gpt-image-2"
	maxImageFileBytes      = 50 * 1024 * 1024
)

const defaultSearchInstructions = "Search broadly before answering. Every factual result and conclusion must cite sources inline as [number]. End with a References section listing each [number] source URL."

const defaultDeepSearchInstructions = "Search broadly and deeply before answering. Prefer official docs, primary sources, source repositories, standards/specifications, release notes, and direct evidence. Use multiple searches where useful. Check negative evidence, limitations, contradictions, and failure modes. Distinguish facts, inference, uncertainty, and recommendations. Cite factual claims inline as [number]. End with a References section listing URLs."

type Client struct {
	openai openai.Client
}

type SearchRequest struct {
	Model             string `json:"model,omitempty" jsonschema:"OpenAI-compatible model name. Defaults to gpt-5.4-mini, or gpt-5.5 for deep search."`
	Instructions      string `json:"instructions,omitempty" jsonschema:"Optional system instructions for the Responses API."`
	Input             string `json:"input" jsonschema:"Search/research request to send to the Responses API with web_search enabled."`
	Deep              bool   `json:"deep,omitempty" jsonschema:"Enable high-effort deep search defaults."`
	ReasoningEffort   string `json:"reasoning_effort,omitempty" jsonschema:"Reasoning effort for deep search: low, medium, high, xhigh."`
	SearchContextSize string `json:"search_context_size,omitempty" jsonschema:"Web search context size for deep search: low, medium, high."`
	MaxToolCalls      int    `json:"max_tool_calls,omitempty" jsonschema:"Maximum built-in tool calls for deep search."`
	MaxOutputTokens   int    `json:"max_output_tokens,omitempty" jsonschema:"Maximum output tokens for deep search."`
}

type SearchResult struct {
	Text                        string `json:"text"`
	Model                       string `json:"model,omitempty"`
	ReasoningEffort             string `json:"reasoning_effort,omitempty"`
	SearchContextSize           string `json:"search_context_size,omitempty"`
	MaxToolCalls                int    `json:"max_tool_calls,omitempty"`
	MaxOutputTokens             int    `json:"max_output_tokens,omitempty"`
	CompatibilityFallback       bool   `json:"compatibility_fallback,omitempty"`
	CompatibilityFallbackReason string `json:"compatibility_fallback_reason,omitempty"`
}

type ImageRequest struct {
	Model             string `json:"model,omitempty" jsonschema:"Image model name. Defaults to gpt-image-2."`
	Prompt            string `json:"prompt" jsonschema:"Image generation prompt."`
	Size              string `json:"size,omitempty" jsonschema:"Image size, for example 1024x1024."`
	Quality           string `json:"quality,omitempty" jsonschema:"Image quality, for example low."`
	N                 int    `json:"n,omitempty" jsonschema:"Number of images to generate. Defaults to 1."`
	OutputFormat      string `json:"output_format,omitempty" jsonschema:"Output format, for example png."`
	Background        string `json:"background,omitempty" jsonschema:"Background mode: auto, transparent, opaque."`
	Moderation        string `json:"moderation,omitempty" jsonschema:"Moderation mode: auto, low."`
	OutputCompression *int   `json:"output_compression,omitempty" jsonschema:"Output compression 0..100 for jpeg/webp."`
}

type ImageResult struct {
	B64JSON       string `json:"b64_json"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type ImageResults struct {
	Images []ImageResult `json:"images"`
}

type EditImageRequest struct {
	Model             string   `json:"model,omitempty" jsonschema:"Image model name. Defaults to gpt-image-2."`
	Prompt            string   `json:"prompt" jsonschema:"Image edit prompt."`
	Images            []string `json:"images" jsonschema:"Input image file paths (1-16 items)."`
	Mask              string   `json:"mask,omitempty" jsonschema:"Optional mask image file path."`
	Size              string   `json:"size,omitempty" jsonschema:"Image size, for example 1024x1024."`
	Quality           string   `json:"quality,omitempty" jsonschema:"Image quality, for example low."`
	N                 int      `json:"n,omitempty" jsonschema:"Number of images to generate. Defaults to 1."`
	OutputFormat      string   `json:"output_format,omitempty" jsonschema:"Output format, for example png."`
	Background        string   `json:"background,omitempty" jsonschema:"Background mode: auto, transparent, opaque."`
	OutputCompression *int     `json:"output_compression,omitempty" jsonschema:"Output compression 0..100 for jpeg/webp."`
	InputFidelity     string   `json:"input_fidelity,omitempty" jsonschema:"Input fidelity: high, low."`
}

func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 600 * time.Second}
	}
	return &Client{openai: openai.NewClient(
		option.WithBaseURL(strings.TrimRight(baseURL, "/")),
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
	)}
}

func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	if strings.TrimSpace(req.Input) == "" {
		return SearchResult{}, errors.New("input is required")
	}
	if req.Model == "" {
		if req.Deep {
			req.Model = DefaultDeepSearchModel
		} else {
			req.Model = DefaultSearchModel
		}
	}
	if req.Instructions == "" {
		if req.Deep {
			req.Instructions = defaultDeepSearchInstructions
		} else {
			req.Instructions = defaultSearchInstructions
		}
	}
	if req.Deep {
		if req.ReasoningEffort == "" {
			req.ReasoningEffort = "high"
		}
		if req.SearchContextSize == "" {
			req.SearchContextSize = "high"
		}
		if req.MaxToolCalls == 0 {
			req.MaxToolCalls = 8
		}
		if req.MaxOutputTokens == 0 {
			req.MaxOutputTokens = 8000
		}
	}

	params := searchResponseParams(req)
	result := SearchResult{
		Model:             req.Model,
		ReasoningEffort:   req.ReasoningEffort,
		SearchContextSize: req.SearchContextSize,
		MaxToolCalls:      req.MaxToolCalls,
		MaxOutputTokens:   req.MaxOutputTokens,
	}
	text, err := c.searchText(ctx, params)
	if err != nil && req.Deep && isUnsupportedMaxToolCallsError(err) {
		result.CompatibilityFallback = true
		result.CompatibilityFallbackReason = compatibilityFallbackReason(err)
		result.MaxToolCalls = 0
		params.MaxToolCalls = param.Opt[int64]{}
		text, err = c.searchText(ctx, params)
	}
	if err != nil {
		return SearchResult{}, err
	}
	if text == "" {
		return SearchResult{}, errors.New("responses API returned no output_text")
	}
	result.Text = text
	return result, nil
}

func searchResponseParams(req SearchRequest) responses.ResponseNewParams {
	params := responses.ResponseNewParams{
		Model:        shared.ResponsesModel(req.Model),
		Instructions: openai.String(req.Instructions),
		Store:        openai.Bool(false),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{
				responses.ResponseInputItemParamOfMessage(
					responses.ResponseInputMessageContentListParam{
						responses.ResponseInputContentParamOfInputText(req.Input),
					},
					responses.EasyInputMessageRoleUser,
				),
			},
		},
		Tools: []responses.ToolUnionParam{searchWebSearchTool(req)},
		ToolChoice: responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsAuto),
		},
	}
	if req.Deep {
		params.Reasoning = shared.ReasoningParam{Effort: shared.ReasoningEffort(req.ReasoningEffort)}
		params.MaxToolCalls = openai.Int(int64(req.MaxToolCalls))
		params.MaxOutputTokens = openai.Int(int64(req.MaxOutputTokens))
	}
	return params
}

func (c *Client) searchText(ctx context.Context, params responses.ResponseNewParams) (string, error) {
	text, err := c.searchStreaming(ctx, params)
	if err != nil {
		return "", err
	}
	if text == "" {
		resp, err := c.openai.Responses.New(ctx, params)
		if err != nil {
			return "", err
		}
		text = resp.OutputText()
	}
	return text, nil
}

func isUnsupportedMaxToolCallsError(err error) bool {
	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		return false
	}
	message := strings.ToLower(apiErr.Message)
	return strings.Contains(message, "unsupported parameter") && strings.Contains(message, "max_tool_calls")
}

func compatibilityFallbackReason(err error) string {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) && apiErr.Message != "" {
		return apiErr.Message
	}
	return "unsupported parameter: max_tool_calls"
}

func searchWebSearchTool(req SearchRequest) responses.ToolUnionParam {
	tool := responses.WebSearchToolParam{Type: responses.WebSearchToolTypeWebSearch}
	if req.Deep {
		tool.SearchContextSize = responses.WebSearchToolSearchContextSize(req.SearchContextSize)
	}
	return responses.ToolUnionParam{OfWebSearch: &tool}
}

func (c *Client) searchStreaming(ctx context.Context, params responses.ResponseNewParams) (string, error) {
	stream := c.openai.Responses.NewStreaming(ctx, params)
	var builder strings.Builder
	var doneText string
	for stream.Next() {
		event := stream.Current()
		switch event.Type {
		case "response.output_text.delta":
			builder.WriteString(event.Delta)
		case "response.output_text.done":
			doneText = event.Text
		case "response.completed":
			if text := event.Response.OutputText(); text != "" {
				doneText = text
			}
		case "response.failed", "error":
			if event.Message != "" {
				return "", fmt.Errorf("responses API stream error: %s", event.Message)
			}
		}
	}
	return searchStreamResult(builder.String(), doneText, stream.Err())
}

func searchStreamResult(deltaText, doneText string, err error) (string, error) {
	if err != nil {
		text := firstNonEmptyString(doneText, deltaText)
		if text != "" && isUnexpectedJSONEOF(err) {
			return text, nil
		}
		return "", err
	}
	return firstNonEmptyString(doneText, deltaText), nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func isUnexpectedJSONEOF(err error) bool {
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, fs.ErrInvalid) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "unexpected end of json input")
}

func (c *Client) GenerateImage(ctx context.Context, req ImageRequest) (ImageResults, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return ImageResults{}, errors.New("prompt is required")
	}
	if req.Model == "" {
		req.Model = DefaultImageModel
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.Quality == "" {
		req.Quality = "low"
	}
	if req.N == 0 {
		req.N = 1
	}
	if req.OutputFormat == "" {
		req.OutputFormat = "png"
	}

	params := openai.ImageGenerateParams{
		Model:        openai.ImageModel(req.Model),
		Prompt:       req.Prompt,
		Size:         openai.ImageGenerateParamsSize(req.Size),
		Quality:      openai.ImageGenerateParamsQuality(req.Quality),
		N:            openai.Int(int64(req.N)),
		OutputFormat: openai.ImageGenerateParamsOutputFormat(req.OutputFormat),
	}
	if req.Background != "" {
		params.Background = openai.ImageGenerateParamsBackground(req.Background)
	}
	if req.Moderation != "" {
		params.Moderation = openai.ImageGenerateParamsModeration(req.Moderation)
	}
	if req.OutputCompression != nil {
		params.OutputCompression = openai.Int(int64(*req.OutputCompression))
	}

	resp, err := c.openai.Images.Generate(ctx, params)
	if err != nil {
		return ImageResults{}, err
	}
	if len(resp.Data) == 0 {
		return ImageResults{}, errors.New("images API returned no images")
	}
	results := make([]ImageResult, 0, len(resp.Data))
	for _, item := range resp.Data {
		if item.B64JSON == "" {
			continue
		}
		results = append(results, ImageResult{
			B64JSON:       item.B64JSON,
			RevisedPrompt: item.RevisedPrompt,
		})
	}
	if len(results) == 0 {
		return ImageResults{}, errors.New("images API returned no b64_json")
	}
	return ImageResults{Images: results}, nil
}

func (c *Client) EditImage(ctx context.Context, req EditImageRequest) (ImageResults, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return ImageResults{}, errors.New("prompt is required")
	}
	if len(req.Images) == 0 {
		return ImageResults{}, errors.New("at least one image is required")
	}
	if len(req.Images) > 16 {
		return ImageResults{}, errors.New("a maximum of 16 images is supported")
	}
	if req.Model == "" {
		req.Model = DefaultImageModel
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.Quality == "" {
		req.Quality = "low"
	}
	if req.N == 0 {
		req.N = 1
	}
	if req.OutputFormat == "" {
		req.OutputFormat = "png"
	}

	if err := ValidateEditImageInputs(req.Images, req.Mask); err != nil {
		return ImageResults{}, err
	}

	images, closeImages, err := openImageReaders(req.Images)
	if err != nil {
		return ImageResults{}, err
	}
	defer closeImages()

	var maskReader io.Reader
	if req.Mask != "" {
		maskPath := strings.TrimSpace(req.Mask)
		maskContentType := "image/png"
		maskFile, err := os.Open(maskPath)
		if err != nil {
			return ImageResults{}, fmt.Errorf("open mask %q: %w", maskPath, err)
		}
		defer maskFile.Close()
		maskReader = typedFile{File: maskFile, contentType: maskContentType}
	}

	params := openai.ImageEditParams{
		Model:        openai.ImageModel(req.Model),
		Prompt:       req.Prompt,
		N:            openai.Int(int64(req.N)),
		Size:         openai.ImageEditParamsSize(req.Size),
		Quality:      openai.ImageEditParamsQuality(req.Quality),
		OutputFormat: openai.ImageEditParamsOutputFormat(req.OutputFormat),
		Image: openai.ImageEditParamsImageUnion{
			OfFileArray: images,
		},
	}
	if maskReader != nil {
		params.Mask = maskReader
	}
	if req.Background != "" {
		params.Background = openai.ImageEditParamsBackground(req.Background)
	}
	if req.OutputCompression != nil {
		params.OutputCompression = openai.Int(int64(*req.OutputCompression))
	}
	if req.InputFidelity != "" {
		params.InputFidelity = openai.ImageEditParamsInputFidelity(req.InputFidelity)
	}

	resp, err := c.openai.Images.Edit(ctx, params)
	if err != nil {
		return ImageResults{}, err
	}
	if len(resp.Data) == 0 {
		return ImageResults{}, errors.New("images API returned no images")
	}
	results := make([]ImageResult, 0, len(resp.Data))
	for _, item := range resp.Data {
		if item.B64JSON == "" {
			continue
		}
		results = append(results, ImageResult{
			B64JSON:       item.B64JSON,
			RevisedPrompt: item.RevisedPrompt,
		})
	}
	if len(results) == 0 {
		return ImageResults{}, errors.New("images API returned no b64_json")
	}
	return ImageResults{Images: results}, nil
}

func openImageReaders(paths []string) ([]io.Reader, func(), error) {
	readers := make([]io.Reader, 0, len(paths))
	files := make([]*os.File, 0, len(paths))
	closeFn := func() {
		for _, f := range files {
			_ = f.Close()
		}
	}

	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			closeFn()
			return nil, nil, errors.New("image path is required")
		}
		contentType, err := validateImagePathAndContentType(trimmed, "image")
		if err != nil {
			closeFn()
			return nil, nil, err
		}
		f, err := os.Open(trimmed)
		if err != nil {
			closeFn()
			return nil, nil, fmt.Errorf("open image %q: %w", trimmed, err)
		}
		if _, err := f.Stat(); err != nil {
			_ = f.Close()
			closeFn()
			return nil, nil, fmt.Errorf("stat image %q: %w", trimmed, err)
		}
		files = append(files, f)
		readers = append(readers, typedFile{File: f, contentType: contentType})
	}

	return readers, closeFn, nil
}

type typedFile struct {
	*os.File
	contentType string
}

func (f typedFile) ContentType() string {
	return f.contentType
}

func imageContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func validateImagePathAndContentType(path, kind string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("check %s %q: %w", kind, path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s path must be a regular file: %s", kind, path)
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return "", fmt.Errorf("%s path must include a supported extension (.png, .jpg, .jpeg, .webp): %s", kind, path)
	}
	contentType := imageContentType(path)
	if contentType == "" {
		return "", fmt.Errorf("unsupported %s file extension %q for %s (supported: .png, .jpg, .jpeg, .webp)", kind, filepath.Ext(path), path)
	}
	if kind == "image" && info.Size() >= maxImageFileBytes {
		return "", fmt.Errorf("image file must be smaller than 50MB: %s", path)
	}
	return contentType, nil
}

func validateMaskPath(path string) (string, error) {
	if _, err := validateImagePathAndContentType(path, "mask"); err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(path)) != ".png" {
		return "", fmt.Errorf("unsupported mask file extension %q for %s (supported: .png)", filepath.Ext(path), path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("check mask %q: %w", path, err)
	}
	if info.Size() >= 4*1024*1024 {
		return "", fmt.Errorf("mask file must be smaller than 4MB: %s", path)
	}
	return "image/png", nil
}

func decodeImageConfig(path, kind string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, fmt.Errorf("open %s %q: %w", kind, path, err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return image.Config{}, fmt.Errorf("decode %s %q: %w", kind, path, err)
	}
	return cfg, nil
}

func validateMaskDimensionsAndAlpha(maskPath string, first image.Config) error {
	cfg, err := decodeImageConfig(maskPath, "mask")
	if err != nil {
		return err
	}
	if cfg.Width != first.Width || cfg.Height != first.Height {
		return errors.New("mask must have the same dimensions as the first input image")
	}
	f, err := os.Open(maskPath)
	if err != nil {
		return fmt.Errorf("open mask %q: %w", maskPath, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode mask %q: %w", maskPath, err)
	}
	b := img.Bounds()
	hasTransparency := false
	for y := b.Min.Y; y < b.Max.Y && !hasTransparency; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := color.NRGBAModel.Convert(img.At(x, y)).RGBA()
			if a < 0xffff {
				hasTransparency = true
				break
			}
		}
	}
	if !hasTransparency {
		return errors.New("mask must include transparency (alpha channel)")
	}
	return nil
}

func ValidateEditImageInputs(images []string, mask string) error {
	if len(images) > 16 {
		return errors.New("a maximum of 16 images is supported")
	}
	trimmedImages := make([]string, 0, len(images))
	for _, path := range images {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return errors.New("image path is required")
		}
		if _, err := validateImagePathAndContentType(trimmed, "image"); err != nil {
			return err
		}
		trimmedImages = append(trimmedImages, trimmed)
	}
	if mask == "" {
		return nil
	}
	maskPath := strings.TrimSpace(mask)
	if maskPath == "" {
		return errors.New("mask path is required")
	}
	if len(trimmedImages) == 0 {
		return errors.New("--mask requires at least one --image for validation")
	}
	if _, err := validateMaskPath(maskPath); err != nil {
		return err
	}
	firstImageConfig, err := decodeImageConfig(trimmedImages[0], "image")
	if err != nil {
		return err
	}
	if err := validateMaskDimensionsAndAlpha(maskPath, firstImageConfig); err != nil {
		return err
	}
	return nil
}
