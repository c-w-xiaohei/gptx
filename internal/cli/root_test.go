package cli

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/c-w-xiaohei/gptx/internal/openaiapi"
)

func TestRootHelpMentionsEnvEndpointsExamples(t *testing.T) {
	cmd := NewRootCommand()
	help := cmd.Long + "\n" + cmd.Example

	wants := []string{
		"/responses",
		"/images/generations",
		"/images/edits",
		"GPTX_OPENAI_BASE_URL",
		"GPTX_OPENAI_API_KEY",
		"https://api.openai.com/v1",
		"gptx search",
		"gptx image generate",
		"gptx image edit",
		"--model",
	}
	for _, want := range wants {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}

func TestRootHelpEmphasizesFeaturesOverAPIWrapping(t *testing.T) {
	cmd := NewRootCommand()
	help := cmd.Long + "\n" + cmd.Example
	if strings.Contains(help, "alias") {
		t.Fatalf("help should not mention env var aliases, got %q", help)
	}

	wants := []string{
		"Search the web",
		"Generate images",
		"Edit images",
		"gptx status",
	}
	for _, want := range wants {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing feature-oriented text %q", want)
		}
	}
	if strings.Contains(cmd.Short, "APIs") {
		t.Fatalf("short help should emphasize user-facing features, got %q", cmd.Short)
	}
}

func TestDefaultTimeoutIsTwentyMinutes(t *testing.T) {
	if defaultTimeout != 20*time.Minute {
		t.Fatalf("defaultTimeout = %s, want 20m", defaultTimeout)
	}

	cmd := NewRootCommand()
	flag := cmd.PersistentFlags().Lookup("timeout")
	if flag == nil {
		t.Fatal("timeout flag missing")
	}
	if flag.DefValue != "20m0s" {
		t.Fatalf("timeout default = %q, want 20m0s", flag.DefValue)
	}
}

func TestSearchHelpAvoidsInternalImplementationDetails(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("search help should not fail: %v", err)
	}

	help := out.String()
	if strings.Contains(help, "openaiapi.Search") {
		t.Fatalf("search help should not expose internal implementation details, got %q", help)
	}
}

func TestSearchHelpMentionsDeepModeAndBackgroundGating(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("search help should not fail: %v", err)
	}

	help := out.String()
	for _, want := range []string{"--deep", "gpt-5.5", "--reasoning-effort", "--search-context-size", "--max-tool-calls", "--max-output-tokens", "--bg is only supported with --deep"} {
		if !strings.Contains(help, want) {
			t.Fatalf("search help missing %q, got %q", want, help)
		}
	}
}

func TestSearchJSONIncludesDeepMetadata(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"deep answer\"}\n\n"))
	}))
	defer ts.Close()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--base-url", ts.URL, "search", "query", "--deep", "--reasoning-effort", "medium", "--search-context-size", "low", "--max-tool-calls", "4", "--max-output-tokens", "2000", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("deep search json should not fail: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	wants := map[string]any{
		"deep":                true,
		"model":               openaiapi.DefaultDeepSearchModel,
		"reasoning_effort":    "medium",
		"search_context_size": "low",
		"max_tool_calls":      float64(4),
		"max_output_tokens":   float64(2000),
		"query":               "query",
		"text":                "deep answer",
	}
	for key, want := range wants {
		if payload[key] != want {
			t.Fatalf("%s = %#v, want %#v in %#v", key, payload[key], want, payload)
		}
	}
}

func TestSearchJSONReportsDeepFallbackEffectiveMetadata(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	requests := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: max_tool_calls","type":"invalid_request_error","param":"max_tool_calls"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"deep answer\"}\n\n"))
	}))
	defer ts.Close()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--base-url", ts.URL, "search", "query", "--deep", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("deep search json should not fail: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	if payload["max_tool_calls"] != float64(0) {
		t.Fatalf("max_tool_calls = %#v, want 0 in %#v", payload["max_tool_calls"], payload)
	}
	if payload["compatibility_fallback"] != true {
		t.Fatalf("compatibility_fallback = %#v in %#v", payload["compatibility_fallback"], payload)
	}
	reason, ok := payload["compatibility_fallback_reason"].(string)
	if !ok || !strings.Contains(reason, "max_tool_calls") {
		t.Fatalf("compatibility_fallback_reason = %#v in %#v", payload["compatibility_fallback_reason"], payload)
	}
	if payload["reasoning_effort"] != "high" || payload["search_context_size"] != "high" || payload["max_output_tokens"] != float64(8000) {
		t.Fatalf("deep metadata = %#v", payload)
	}
	if strings.Contains(out.String(), "secret") {
		t.Fatalf("json output leaked API key: %q", out.String())
	}
}

func TestStatusShowsConfiguredAuthenticationWithoutSecret(t *testing.T) {
	t.Setenv("GPTX_OPENAI_BASE_URL", "https://example.test/v1")
	t.Setenv("GPTX_OPENAI_API_KEY", "secret-key")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status should not fail: %v", err)
	}
	text := out.String()
	for _, want := range []string{"base_url: https://example.test/v1", "api_key: configured", "api_key_source: GPTX_OPENAI_API_KEY"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status output missing %q, got %q", want, text)
		}
	}
	if strings.Contains(text, "secret-key") {
		t.Fatalf("status output leaked API key: %q", text)
	}
}

func TestStatusJSONShowsMissingAuthentication(t *testing.T) {
	t.Setenv("GPTX_OPENAI_BASE_URL", "")
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"status", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status json should not fail: %v", err)
	}
	text := out.String()
	for _, want := range []string{"\"authenticated\": false", "\"api_key_configured\": false", "\"base_url\": \"https://api.openai.com/v1\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("status json missing %q, got %q", want, text)
		}
	}
}

func TestVersionCommandUsesVersionVariable(t *testing.T) {
	oldVersion := Version
	Version = "v1.2.3"
	t.Cleanup(func() { Version = oldVersion })

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version should not fail: %v", err)
	}
	if got, want := strings.TrimSpace(out.String()), "gptx v1.2.3"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func TestResolveVersionFallsBackToBuildInfoVersion(t *testing.T) {
	got := resolveVersion("dev", "v0.1.0", true)
	if got != "v0.1.0" {
		t.Fatalf("resolveVersion = %q, want %q", got, "v0.1.0")
	}
}

func TestResolveVersionKeepsInjectedVersion(t *testing.T) {
	got := resolveVersion("0.1.0", "v0.2.0", true)
	if got != "0.1.0" {
		t.Fatalf("resolveVersion = %q, want %q", got, "0.1.0")
	}
}

func TestResolveVersionUsesDevForMissingBuildInfoVersion(t *testing.T) {
	for _, buildVersion := range []string{"", "(devel)"} {
		got := resolveVersion("dev", buildVersion, true)
		if got != "dev" {
			t.Fatalf("resolveVersion with build version %q = %q, want dev", buildVersion, got)
		}
	}
}

func TestValidateImageOptionsOutRequiresN1(t *testing.T) {
	err := validateImageOptions(imageOptions{Out: "a.png", N: 2, OutputFormat: "png", OutputCompression: -1})
	if err == nil || !strings.Contains(err.Error(), "valid only when --n=1") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestOutputPathDefaultTemplateGenerate(t *testing.T) {
	path, err := outputPath("gptx-image", "20260429-120000", 2, imageOptions{OutputFormat: "png"})
	if err != nil {
		t.Fatal(err)
	}
	want := "gptx-image-20260429-120000-2.png"
	if path != want {
		t.Fatalf("path=%q want=%q", path, want)
	}
}

func TestOutputPathDefaultTemplateEdit(t *testing.T) {
	path, err := outputPath("gptx-edit", "20260429-120000", 1, imageOptions{OutputFormat: "webp"})
	if err != nil {
		t.Fatal(err)
	}
	want := "gptx-edit-20260429-120000-1.webp"
	if path != want {
		t.Fatalf("path=%q want=%q", path, want)
	}
}

func TestValidateImageOptionsOutputFormat(t *testing.T) {
	if err := validateImageOptions(imageOptions{N: 1, OutputFormat: "PNG", OutputCompression: -1}); err != nil {
		t.Fatalf("expected PNG to be accepted: %v", err)
	}
	err := validateImageOptions(imageOptions{N: 1, OutputFormat: "gif", OutputCompression: -1})
	if err == nil || !strings.Contains(err.Error(), "must be one of: png, webp, jpeg") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidateImageOptionsRangeAndEnums(t *testing.T) {
	if err := validateImageOptions(imageOptions{N: 10, OutputFormat: "png", OutputCompression: -1}); err != nil {
		t.Fatalf("expected n=10 accepted: %v", err)
	}
	err := validateImageOptions(imageOptions{N: 11, OutputFormat: "png", OutputCompression: -1})
	if err == nil || !strings.Contains(err.Error(), "--n must be between 1 and 10") {
		t.Fatalf("unexpected err: %v", err)
	}
	err = validateImageOptions(imageOptions{N: 1, OutputFormat: "jpeg", Background: "bad", OutputCompression: -1})
	if err == nil || !strings.Contains(err.Error(), "invalid --background") {
		t.Fatalf("unexpected err: %v", err)
	}
	err = validateImageOptions(imageOptions{N: 1, OutputFormat: "jpeg", OutputCompression: 101})
	if err == nil || !strings.Contains(err.Error(), "--output-compression must be between 0 and 100") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidateImageOptionsCrossFieldRules(t *testing.T) {
	err := validateImageOptions(imageOptions{N: 1, OutputFormat: "jpeg", Background: "transparent", OutputCompression: -1})
	if err == nil || !strings.Contains(err.Error(), "--background transparent requires --output-format png or webp") {
		t.Fatalf("unexpected err: %v", err)
	}
	err = validateImageOptions(imageOptions{N: 1, OutputFormat: "png", OutputCompression: 80})
	if err == nil || !strings.Contains(err.Error(), "--output-compression requires --output-format jpeg or webp") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageGenerateDryRunNoAPIKey(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--n", "2", "--output-format", "png"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should not require api key: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 paths, got %d output=%q", len(lines), out.String())
	}
	if !strings.Contains(lines[0], "gptx-image-") || !strings.HasSuffix(lines[0], ".png") {
		t.Fatalf("unexpected first path: %q", lines[0])
	}
}

func TestImageGenerateHelpDocumentsReferenceImages(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "generate", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help should not fail: %v", err)
	}
	help := out.String()
	for _, want := range []string{"--image", "reference", "/images/edits", "--model", "--background", "--output-compression", "--moderation", "--input-fidelity"} {
		if !strings.Contains(help, want) {
			t.Fatalf("generate help missing %q, got %q", want, help)
		}
	}
}

func TestImageEditHelpIncludesNewFlags(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "edit", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help should not fail: %v", err)
	}
	help := out.String()
	for _, want := range []string{"--model", "--background", "--output-compression", "--input-fidelity"} {
		if !strings.Contains(help, want) {
			t.Fatalf("edit help missing %q, got %q", want, help)
		}
	}
}

func TestImageGenerateWithReferenceImageUsesEditEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "design-system.png")
	outPath := filepath.Join(tmpDir, "out.png")
	if err := os.WriteFile(imagePath, []byte("reference"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotPath string
	var gotPrompt string
	var gotImageContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if gotPath == "/images/edits" {
			mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				t.Fatal(err)
			}
			if mediaType != "multipart/form-data" {
				t.Fatalf("content type = %q", mediaType)
			}
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				b, err := io.ReadAll(part)
				if err != nil {
					t.Fatal(err)
				}
				switch part.FormName() {
				case "prompt":
					gotPrompt = string(b)
				case "image[]":
					gotImageContentType = part.Header.Get("Content-Type")
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1777395117,
			"data": []map[string]any{{
				"b64_json": "cmVmLWdlbg==",
			}},
		})
	}))
	defer ts.Close()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--base-url", ts.URL,
		"--api-key", "test-key",
		"image", "generate", "match this design system",
		"--image", imagePath,
		"--out", outPath,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate with reference image should not fail: %v", err)
	}
	if gotPath != "/images/edits" {
		t.Fatalf("path = %q, want /images/edits", gotPath)
	}
	if gotPrompt != "match this design system" {
		t.Fatalf("prompt = %q", gotPrompt)
	}
	if gotImageContentType != "image/png" {
		t.Fatalf("reference image content type = %q", gotImageContentType)
	}
	if got := strings.TrimSpace(out.String()); got != outPath {
		t.Fatalf("stdout = %q, want %q", got, outPath)
	}
	if b, err := os.ReadFile(outPath); err != nil || string(b) != "ref-gen" {
		t.Fatalf("saved output err=%v data=%q", err, string(b))
	}
}

func TestImageEditDryRunNoAPIKeyNoImageRequired(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "edit", "prompt", "--dry-run", "--output-format", "jpeg", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should not require api key or images: %v", err)
	}
	if !strings.Contains(out.String(), "\"dry_run\": true") {
		t.Fatalf("expected dry_run json true, output=%q", out.String())
	}
	if !strings.Contains(out.String(), "\"output_format\": \"jpeg\"") {
		t.Fatalf("expected normalized output format, output=%q", out.String())
	}
}

func TestImageGenerateDryRunJSONIncludesNewOptions(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--json", "--model", "gpt-image-2", "--background", "auto", "--moderation", "low", "--output-format", "webp", "--output-compression", "55"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should not fail: %v", err)
	}
	for _, want := range []string{"\"model\": \"gpt-image-2\"", "\"background\": \"auto\"", "\"moderation\": \"low\"", "\"output_compression\": 55"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %q", want, out.String())
		}
	}
}

func TestImageEditDryRunJSONIncludesNewOptions(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "edit", "prompt", "--dry-run", "--json", "--model", "gpt-image-2", "--background", "opaque", "--output-format", "jpeg", "--output-compression", "65", "--input-fidelity", "high"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should not fail: %v", err)
	}
	for _, want := range []string{"\"model\": \"gpt-image-2\"", "\"background\": \"opaque\"", "\"input_fidelity\": \"high\"", "\"output_compression\": 65"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %q", want, out.String())
		}
	}
}

func TestImageGenerateInputFidelityRequiresImage(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--input-fidelity", "high"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--input-fidelity requires at least one --image") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageGenerateWithReferenceImageRejectsModeration(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	dir := t.TempDir()
	img := filepath.Join(dir, "in.png")
	writePNGFile(t, img, image.NewNRGBA(image.Rect(0, 0, 2, 2)))

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--image", img, "--moderation", "low"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--moderation is not supported with --image") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageGenerateDryRunWithImageValidatesLocalInput(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--image", "/no/such/file.png"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "check image") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageEditDryRunWithMaskValidatesLocalInputs(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	dir := t.TempDir()
	img := filepath.Join(dir, "in.png")
	mask := filepath.Join(dir, "mask.png")
	writePNGFile(t, img, image.NewNRGBA(image.Rect(0, 0, 3, 3)))
	alphaMask := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	alphaMask.Set(0, 0, color.NRGBA{A: 0})
	writePNGFile(t, mask, alphaMask)

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "edit", "prompt", "--dry-run", "--image", img, "--mask", mask})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "same dimensions") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageEditDryRunMaskWithoutImageErrors(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "edit", "prompt", "--dry-run", "--mask", "/no/such/mask.png", "--json"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--mask requires at least one --image") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageGenerateDryRunRejectsMoreThan16ReferenceImages(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	dir := t.TempDir()
	args := []string{"image", "generate", "prompt", "--dry-run"}
	for i := 0; i < 17; i++ {
		path := filepath.Join(dir, strconv.Itoa(i)+".png")
		writePNGFile(t, path, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
		args = append(args, "--image", path)
	}
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "maximum of 16 images") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageEditDryRunRejectsMoreThan16Images(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	dir := t.TempDir()
	args := []string{"image", "edit", "prompt", "--dry-run"}
	for i := 0; i < 17; i++ {
		path := filepath.Join(dir, "e"+strconv.Itoa(i)+".png")
		writePNGFile(t, path, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
		args = append(args, "--image", path)
	}
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "maximum of 16 images") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageOutputPathExtensionMustMatchOutputFormat(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--out", filepath.Join(tmpDir, "x.jpg"), "--output-format", "png"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "output path extension") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageOutputPathJPGAcceptedForJPEGFormat(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--out", filepath.Join(tmpDir, "x.jpg"), "--output-format", "jpeg"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected .jpg accepted for jpeg output format: %v", err)
	}
}

func TestImageGenerateDryRunExplicitNZeroErrors(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--n", "0"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--n must be between 1 and 10") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageGenerateDryRunExplicitNegativeOutputCompressionErrors(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--output-format", "webp", "--output-compression", "-1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--output-compression must be between 0 and 100") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageGenerateDryRunOutWithoutExtensionErrors(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	tmpDir := t.TempDir()

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--out", filepath.Join(tmpDir, "noext"), "--output-format", "png"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "must include extension") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestDryRunValidatesPathWithoutCreateDirs(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--out-dir", "./does-not-exist/child"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "output directory does not exist") {
		t.Fatalf("expected missing dir validation error, got: %v", err)
	}
	_ = os.RemoveAll("./does-not-exist")
}

func TestImageGenerateDryRunHonorsGlobalFormatJSON(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "json", "image", "generate", "prompt", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected json dry-run with global format: %v", err)
	}
	if !strings.Contains(out.String(), "\"command\": \"image generate\"") {
		t.Fatalf("expected json output, got: %q", out.String())
	}
}

func TestImageGenerateDryRunInvalidGlobalFormatErrors(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "yaml", "image", "generate", "prompt", "--dry-run"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid --format") {
		t.Fatalf("expected invalid format error, got: %v", err)
	}
}

func writePNGFile(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestSaveImagesMultiImageSuccess(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")}
	images := []openaiapi.ImageResult{{B64JSON: "aGVsbG8="}, {B64JSON: "d29ybGQ="}}

	saved, revised, err := saveImages(paths, images, imageOptions{OutputFormat: "png"})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 || len(revised) != 0 {
		t.Fatalf("saved=%v revised=%v", saved, revised)
	}
	b0, err := os.ReadFile(paths[0])
	if err != nil || string(b0) != "hello" {
		t.Fatalf("path0 err=%v data=%q", err, string(b0))
	}
	b1, err := os.ReadFile(paths[1])
	if err != nil || string(b1) != "world" {
		t.Fatalf("path1 err=%v data=%q", err, string(b1))
	}
}

func TestSaveImagesMissingDirectoryWithoutCreateDirs(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "missing", "a.png")}
	images := []openaiapi.ImageResult{{B64JSON: "aGVsbG8="}}

	_, _, err := saveImages(paths, images, imageOptions{OutputFormat: "png", CreateDirs: false})
	if err == nil || !strings.Contains(err.Error(), "create temp file") {
		t.Fatalf("expected write error for missing dir, got: %v", err)
	}
}

func TestSaveImagesExistingFileWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.png")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	images := []openaiapi.ImageResult{{B64JSON: "bmV3"}}

	_, _, err := saveImages([]string{path}, images, imageOptions{OutputFormat: "png", Overwrite: false})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected exists error, got: %v", err)
	}
}

func TestValidatePlannedPathsCreateDirsRejectsExistingFileAsDirectory(t *testing.T) {
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parentFile, "out.png")

	err := validatePlannedPaths([]string{path}, imageOptions{CreateDirs: true, OutputFormat: "png"})
	if err == nil || !strings.Contains(err.Error(), "output directory is not a directory") {
		t.Fatalf("expected not-a-directory error, got: %v", err)
	}
}

func TestValidatePlannedPathsOutPathExistingDirectoryRejectsEvenWithOverwrite(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "existing-dir")
	if err := os.MkdirAll(outPath, 0o755); err != nil {
		t.Fatal(err)
	}

	err := validatePlannedPaths([]string{outPath}, imageOptions{Overwrite: true, OutputFormat: "png"})
	if err == nil || !strings.Contains(err.Error(), "output path must be a regular file path") {
		t.Fatalf("expected regular-file-path error, got: %v", err)
	}
}

func TestSaveImagesDecodeFailureDoesNotWriteAnyFile(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")}
	images := []openaiapi.ImageResult{{B64JSON: "aGVsbG8="}, {B64JSON: "!!!not-base64!!!"}}

	_, _, err := saveImages(paths, images, imageOptions{OutputFormat: "png"})
	if err == nil || !strings.Contains(err.Error(), "decode image 2") {
		t.Fatalf("expected decode error, got: %v", err)
	}
	if _, statErr := os.Stat(paths[0]); !os.IsNotExist(statErr) {
		t.Fatalf("expected first file not written, stat err: %v", statErr)
	}
}

func TestSaveImagesWriteFailureCleansUpWrittenFiles(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "a.png"), filepath.Join(dir, "b.png")}
	images := []openaiapi.ImageResult{{B64JSON: "aGVsbG8="}, {B64JSON: "d29ybGQ="}}

	blockDir := filepath.Join(dir, "b.png")
	if err := os.MkdirAll(blockDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := saveImages(paths, images, imageOptions{OutputFormat: "png"})
	if err == nil || !strings.Contains(err.Error(), "output path must be a regular file path") {
		t.Fatalf("expected write error, got: %v", err)
	}
	if _, statErr := os.Stat(paths[0]); !os.IsNotExist(statErr) {
		t.Fatalf("expected first file cleaned up, stat err: %v", statErr)
	}
}

func TestPlanAndValidateDuplicateFilenameWithN2(t *testing.T) {
	opts := imageOptions{N: 2, Filename: "same.png", OutputFormat: "png"}
	paths, err := planOutputPaths("gptx-image", opts)
	if err != nil {
		t.Fatal(err)
	}
	err = validatePlannedPaths(paths, opts)
	if err == nil || !strings.Contains(err.Error(), "duplicate output path planned") {
		t.Fatalf("expected duplicate path error, got: %v", err)
	}
}
