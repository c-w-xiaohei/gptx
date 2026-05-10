package openaiapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"strings"
	"testing"
)

func TestSearchUsesResponsesWebSearch(t *testing.T) {
	var gotAuth string
	var gotPath string
	var body map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"search result \"}\n\n"))
		_, _ = w.Write([]byte("event: response.output_text.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"search result text\"}\n\n"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	res, err := c.Search(context.Background(), SearchRequest{
		Model:        "gpt-5.5",
		Instructions: "Use search.",
		Input:        "find docs",
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/responses" {
		t.Fatalf("path = %q, want /responses", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if body["model"] != "gpt-5.5" {
		t.Fatalf("model = %#v", body["model"])
	}
	tools, ok := body["tools"].([]any)
	if !ok || len(tools) != 1 || tools[0].(map[string]any)["type"] != "web_search" {
		t.Fatalf("tools = %#v", body["tools"])
	}
	if body["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %#v", body["tool_choice"])
	}
	if body["stream"] != true {
		t.Fatalf("stream = %#v", body["stream"])
	}
	if body["store"] != false {
		t.Fatalf("store = %#v", body["store"])
	}
	input, ok := body["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("input = %#v", body["input"])
	}
	message := input[0].(map[string]any)
	content := message["content"].([]any)
	inputText := content[0].(map[string]any)
	if inputText["type"] != "input_text" || inputText["text"] != "find docs" {
		t.Fatalf("input text = %#v", inputText)
	}
	if res.Text != "search result text" {
		t.Fatalf("text = %q", res.Text)
	}
}

func TestSearchUsesDefaults(t *testing.T) {
	var body map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"ok\"}\n\n"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	if _, err := c.Search(context.Background(), SearchRequest{Input: "find docs"}); err != nil {
		t.Fatal(err)
	}
	if body["model"] != DefaultSearchModel {
		t.Fatalf("model = %#v", body["model"])
	}
	if body["model"] != "gpt-5.4-mini" {
		t.Fatalf("default search model = %#v", body["model"])
	}
	if body["instructions"] == "" {
		t.Fatalf("instructions missing in body %#v", body)
	}
	instructions := strings.ToLower(body["instructions"].(string))
	for _, want := range []string{"search broadly", "[number]", "References"} {
		if !strings.Contains(instructions, strings.ToLower(want)) {
			t.Fatalf("instructions missing %q: %s", want, instructions)
		}
	}
}

func TestDeepSearchUsesHighEffortResponsesParameters(t *testing.T) {
	var body map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"deep ok\"}\n\n"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	res, err := c.Search(context.Background(), SearchRequest{Input: "research deeply", Deep: true})
	if err != nil {
		t.Fatal(err)
	}

	if res.Text != "deep ok" {
		t.Fatalf("text = %q", res.Text)
	}
	if body["model"] != DefaultDeepSearchModel {
		t.Fatalf("model = %#v", body["model"])
	}
	reasoning, ok := body["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "high" {
		t.Fatalf("reasoning = %#v", body["reasoning"])
	}
	tools, ok := body["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v", body["tools"])
	}
	tool := tools[0].(map[string]any)
	if tool["type"] != "web_search" || tool["search_context_size"] != "high" {
		t.Fatalf("tool = %#v", tool)
	}
	if int(body["max_tool_calls"].(float64)) != 8 {
		t.Fatalf("max_tool_calls = %#v", body["max_tool_calls"])
	}
	if int(body["max_output_tokens"].(float64)) != 8000 {
		t.Fatalf("max_output_tokens = %#v", body["max_output_tokens"])
	}
	if body["store"] != false {
		t.Fatalf("store = %#v", body["store"])
	}
	input, ok := body["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("input = %#v", body["input"])
	}
	instructions := strings.ToLower(body["instructions"].(string))
	for _, want := range []string{"search broadly and deeply", "primary sources", "negative evidence", "references"} {
		if !strings.Contains(instructions, want) {
			t.Fatalf("deep instructions missing %q: %s", want, instructions)
		}
	}
}

func TestDeepSearchRetriesWithoutMaxToolCallsWhenUnsupported(t *testing.T) {
	var bodies []map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: max_tool_calls","type":"invalid_request_error","param":"max_tool_calls"}}`))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"fallback ok\"}\n\n"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	res, err := c.Search(context.Background(), SearchRequest{Input: "research deeply", Deep: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(bodies) != 2 {
		t.Fatalf("requests = %d, want 2", len(bodies))
	}
	if int(bodies[0]["max_tool_calls"].(float64)) != 8 {
		t.Fatalf("first max_tool_calls = %#v", bodies[0]["max_tool_calls"])
	}
	if _, ok := bodies[1]["max_tool_calls"]; ok {
		t.Fatalf("retry should omit max_tool_calls, got %#v", bodies[1])
	}
	if bodies[1]["model"] != DefaultDeepSearchModel {
		t.Fatalf("retry model = %#v", bodies[1]["model"])
	}
	if bodies[1]["reasoning"].(map[string]any)["effort"] != "high" {
		t.Fatalf("retry reasoning = %#v", bodies[1]["reasoning"])
	}
	tool := bodies[1]["tools"].([]any)[0].(map[string]any)
	if tool["search_context_size"] != "high" {
		t.Fatalf("retry tool = %#v", tool)
	}
	if int(bodies[1]["max_output_tokens"].(float64)) != 8000 {
		t.Fatalf("retry max_output_tokens = %#v", bodies[1]["max_output_tokens"])
	}
	if res.Text != "fallback ok" {
		t.Fatalf("text = %q", res.Text)
	}
	if !res.CompatibilityFallback {
		t.Fatalf("CompatibilityFallback = false, want true")
	}
	if !strings.Contains(res.CompatibilityFallbackReason, "max_tool_calls") {
		t.Fatalf("CompatibilityFallbackReason = %q", res.CompatibilityFallbackReason)
	}
	if res.MaxToolCalls != 0 {
		t.Fatalf("MaxToolCalls = %d, want 0", res.MaxToolCalls)
	}
}

func TestSearchReturnsDoneTextWhenStreamEndsWithoutDoneSentinel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"done text before eof\"}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	res, err := c.Search(context.Background(), SearchRequest{Input: "query"})
	if err != nil {
		t.Fatalf("search should use collected done text despite stream EOF: %v", err)
	}
	if res.Text != "done text before eof" {
		t.Fatalf("text = %q", res.Text)
	}
}

func TestSearchFallsBackWhenStreamStartsWithHeartbeat(t *testing.T) {
	requests := 0
	var bodies []map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, body)
		if requests == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(": keepalive\n\n"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"fallback text"}]}]}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	res, err := c.Search(context.Background(), SearchRequest{Input: "query"})
	if err != nil {
		t.Fatalf("search should fall back after empty stream heartbeat: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if bodies[0]["stream"] != true {
		t.Fatalf("first stream = %#v, want true", bodies[0]["stream"])
	}
	if _, ok := bodies[1]["stream"]; ok {
		t.Fatalf("fallback request should not set stream, got %#v", bodies[1]["stream"])
	}
	if res.Text != "fallback text" {
		t.Fatalf("text = %q", res.Text)
	}
}

func TestSearchStreamResultUsesTextOnUnexpectedJSONEOF(t *testing.T) {
	text, err := searchStreamResult("delta text", "done text", errors.New("unexpected end of JSON input"))
	if err != nil {
		t.Fatalf("expected collected text, got err: %v", err)
	}
	if text != "done text" {
		t.Fatalf("text = %q", text)
	}
}

func TestSearchStreamResultReturnsOtherErrors(t *testing.T) {
	_, err := searchStreamResult("delta text", "done text", errors.New("connection reset"))
	if err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("expected original error, got %v", err)
	}
}

func TestDeepSearchDoesNotRetryNonCompatibilityBadRequest(t *testing.T) {
	requests := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid model","type":"invalid_request_error","param":"model"}}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	_, err := c.Search(context.Background(), SearchRequest{Input: "research deeply", Deep: true})
	if err == nil {
		t.Fatal("expected error")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestDeepSearchDoesNotRetryMaxToolCallsValidationError(t *testing.T) {
	requests := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"max_tool_calls must be less than or equal to 4","type":"invalid_request_error","param":"max_tool_calls"}}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	_, err := c.Search(context.Background(), SearchRequest{Input: "research deeply", Deep: true, MaxToolCalls: 99})
	if err == nil {
		t.Fatal("expected error")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestDeepSearchAllowsOverrides(t *testing.T) {
	var body map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.done\",\"text\":\"custom ok\"}\n\n"))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	_, err := c.Search(context.Background(), SearchRequest{
		Model:             "custom-model",
		Instructions:      "custom instructions",
		Input:             "research",
		Deep:              true,
		ReasoningEffort:   "medium",
		SearchContextSize: "low",
		MaxToolCalls:      3,
		MaxOutputTokens:   1200,
	})
	if err != nil {
		t.Fatal(err)
	}

	if body["model"] != "custom-model" || body["instructions"] != "custom instructions" {
		t.Fatalf("body = %#v", body)
	}
	if body["reasoning"].(map[string]any)["effort"] != "medium" {
		t.Fatalf("reasoning = %#v", body["reasoning"])
	}
	tool := body["tools"].([]any)[0].(map[string]any)
	if tool["search_context_size"] != "low" {
		t.Fatalf("tool = %#v", tool)
	}
	if int(body["max_tool_calls"].(float64)) != 3 || int(body["max_output_tokens"].(float64)) != 1200 {
		t.Fatalf("limits = %#v %#v", body["max_tool_calls"], body["max_output_tokens"])
	}
}

func TestGenerateImageUsesImagesEndpoint(t *testing.T) {
	var gotAuth string
	var gotPath string
	var body map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1777395117,
			"data": []map[string]any{
				{
					"b64_json":       "aW1hZ2Ux",
					"revised_prompt": "revised1",
				},
				{
					"b64_json":       "aW1hZ2Uy",
					"revised_prompt": "revised2",
				},
			},
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL+"/", "test-key", ts.Client())
	compression := 62
	res, err := c.GenerateImage(context.Background(), ImageRequest{
		Model:             "gpt-image-2",
		Prompt:            "draw a cube",
		Size:              "1024x1024",
		Quality:           "low",
		N:                 2,
		OutputFormat:      "png",
		Background:        "auto",
		Moderation:        "low",
		OutputCompression: &compression,
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/images/generations" {
		t.Fatalf("path = %q, want /images/generations", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if body["model"] != "gpt-image-2" || body["prompt"] != "draw a cube" {
		t.Fatalf("body = %#v", body)
	}
	if body["background"] != "auto" {
		t.Fatalf("background = %#v", body["background"])
	}
	if body["moderation"] != "low" {
		t.Fatalf("moderation = %#v", body["moderation"])
	}
	if int(body["output_compression"].(float64)) != 62 {
		t.Fatalf("output_compression = %#v", body["output_compression"])
	}
	if len(res.Images) != 2 {
		t.Fatalf("images len = %d", len(res.Images))
	}
	if res.Images[0].B64JSON != "aW1hZ2Ux" || res.Images[0].RevisedPrompt != "revised1" {
		t.Fatalf("first result = %#v", res.Images[0])
	}
	if res.Images[1].B64JSON != "aW1hZ2Uy" || res.Images[1].RevisedPrompt != "revised2" {
		t.Fatalf("second result = %#v", res.Images[1])
	}
}

func TestGenerateImageOmitsOutputCompressionWhenUnset(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"created": 1, "data": []map[string]any{{"b64_json": "aQ=="}}})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	_, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["output_compression"]; ok {
		t.Fatalf("output_compression should be omitted, body=%#v", body)
	}
}

func TestGenerateImageSendsOutputCompressionWhenExplicitZero(t *testing.T) {
	var body map[string]any
	zero := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"created": 1, "data": []map[string]any{{"b64_json": "aQ=="}}})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	_, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: "x", OutputCompression: &zero})
	if err != nil {
		t.Fatal(err)
	}
	if int(body["output_compression"].(float64)) != 0 {
		t.Fatalf("output_compression=%#v", body["output_compression"])
	}
}

func TestEditImageUsesMultipartEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	img1 := filepath.Join(tmpDir, "img1.png")
	img2 := filepath.Join(tmpDir, "img2.png")
	mask := filepath.Join(tmpDir, "mask.png")
	writePNG(t, img1, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	writePNG(t, img2, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	alphaMask := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	alphaMask.Set(0, 0, color.NRGBA{A: 0})
	writePNG(t, mask, alphaMask)

	var gotPath string
	var gotType string
	fields := map[string][]string{}
	fileCounts := map[string]int{}
	fileTypes := map[string][]string{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotType = r.Header.Get("Content-Type")

		mediaType, params, err := mime.ParseMediaType(gotType)
		if err != nil {
			t.Fatal(err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("content type = %q", gotType)
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
			name := part.FormName()
			if part.FileName() != "" {
				fileCounts[name]++
				fileTypes[name] = append(fileTypes[name], part.Header.Get("Content-Type"))
			}
			fields[name] = append(fields[name], string(b))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1777395117,
			"data": []map[string]any{{
				"b64_json":       "ZWRpdGVk",
				"revised_prompt": "edited",
			}},
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	compression := 70
	res, err := c.EditImage(context.Background(), EditImageRequest{
		Model:             "gpt-image-2",
		Prompt:            "edit image",
		Images:            []string{img1, img2},
		Mask:              mask,
		Size:              "1024x1024",
		Quality:           "low",
		N:                 1,
		OutputFormat:      "png",
		Background:        "transparent",
		OutputCompression: &compression,
		InputFidelity:     "high",
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/images/edits" {
		t.Fatalf("path = %q, want /images/edits", gotPath)
	}
	if fields["model"][0] != "gpt-image-2" {
		t.Fatalf("model = %#v", fields["model"])
	}
	if fields["prompt"][0] != "edit image" {
		t.Fatalf("prompt = %#v", fields["prompt"])
	}
	if fields["background"][0] != "transparent" {
		t.Fatalf("background = %#v", fields["background"])
	}
	if fields["output_compression"][0] != "70" {
		t.Fatalf("output_compression = %#v", fields["output_compression"])
	}
	if fields["input_fidelity"][0] != "high" {
		t.Fatalf("input_fidelity = %#v", fields["input_fidelity"])
	}
	if fileCounts["image[]"] != 2 {
		t.Fatalf("image[] file count = %d; fields=%#v files=%#v", fileCounts["image[]"], fields, fileCounts)
	}
	for _, contentType := range fileTypes["image[]"] {
		if contentType != "image/png" {
			t.Fatalf("image[] content type = %q; all=%#v", contentType, fileTypes["image[]"])
		}
	}
	if fileCounts["mask"] != 1 {
		t.Fatalf("mask file count = %d; fields=%#v files=%#v", fileCounts["mask"], fields, fileCounts)
	}
	if fileTypes["mask"][0] != "image/png" {
		t.Fatalf("mask content type = %q", fileTypes["mask"][0])
	}
	if len(res.Images) != 1 || res.Images[0].B64JSON != "ZWRpdGVk" {
		t.Fatalf("result = %#v", res)
	}
}

func TestEditImageValidation(t *testing.T) {
	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)

	if _, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "", Images: []string{"a.png"}}); err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("err = %v", err)
	}
	if _, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: nil}); err == nil || !strings.Contains(err.Error(), "at least one image is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestEditImageRejectsUnsupportedImageExtension(t *testing.T) {
	tmpDir := t.TempDir()
	img := filepath.Join(tmpDir, "img.gif")
	if err := os.WriteFile(img, []byte("img"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)
	_, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{img}})
	if err == nil || !strings.Contains(err.Error(), "unsupported image file extension") {
		t.Fatalf("expected unsupported extension error, got: %v", err)
	}
}

func TestEditImageRejectsExtensionlessImagePath(t *testing.T) {
	tmpDir := t.TempDir()
	img := filepath.Join(tmpDir, "img")
	if err := os.WriteFile(img, []byte("img"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)
	_, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{img}})
	if err == nil || !strings.Contains(err.Error(), "must include a supported extension") {
		t.Fatalf("expected extension error, got: %v", err)
	}
}

func TestEditImageRejectsDirectoryImagePath(t *testing.T) {
	tmpDir := t.TempDir()
	imgDir := filepath.Join(tmpDir, "imgdir.png")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)
	_, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{imgDir}})
	if err == nil || !strings.Contains(err.Error(), "image path must be a regular file") {
		t.Fatalf("expected regular file error, got: %v", err)
	}
}

func TestEditImageRejectsUnsupportedMaskExtension(t *testing.T) {
	tmpDir := t.TempDir()
	img := filepath.Join(tmpDir, "img.png")
	mask := filepath.Join(tmpDir, "mask.bmp")
	if err := os.WriteFile(img, []byte("img"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mask, []byte("mask"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)
	_, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{img}, Mask: mask})
	if err == nil || !strings.Contains(err.Error(), "unsupported mask file extension") {
		t.Fatalf("expected unsupported mask extension error, got: %v", err)
	}
}

func TestEditImageRejectsDirectoryMaskPath(t *testing.T) {
	tmpDir := t.TempDir()
	img := filepath.Join(tmpDir, "img.png")
	maskDir := filepath.Join(tmpDir, "mask.png")
	if err := os.WriteFile(img, []byte("img"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(maskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)
	_, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{img}, Mask: maskDir})
	if err == nil || !strings.Contains(err.Error(), "mask path must be a regular file") {
		t.Fatalf("expected regular file mask error, got: %v", err)
	}
}

func TestEditImageRejectsMaskWithoutAlpha(t *testing.T) {
	tmpDir := t.TempDir()
	img := filepath.Join(tmpDir, "img.png")
	mask := filepath.Join(tmpDir, "mask.png")
	writePNG(t, img, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	opaque := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			opaque.SetRGBA(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	writePNG(t, mask, opaque)

	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)
	_, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{img}, Mask: mask})
	if err == nil || !strings.Contains(err.Error(), "must include transparency") {
		t.Fatalf("expected mask alpha error, got: %v", err)
	}
}

func TestEditImageRejectsMaskWithMismatchedDimensions(t *testing.T) {
	tmpDir := t.TempDir()
	img := filepath.Join(tmpDir, "img.png")
	mask := filepath.Join(tmpDir, "mask.png")
	writePNG(t, img, image.NewNRGBA(image.Rect(0, 0, 4, 4)))
	alphaMask := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	alphaMask.Set(0, 0, color.NRGBA{A: 0})
	writePNG(t, mask, alphaMask)

	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)
	_, err := c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{img}, Mask: mask})
	if err == nil || !strings.Contains(err.Error(), "same dimensions") {
		t.Fatalf("expected mask dimensions error, got: %v", err)
	}
}

func TestEditImageRejectsInputImageAtLeast50MB(t *testing.T) {
	tmpDir := t.TempDir()
	img := filepath.Join(tmpDir, "large.png")
	f, err := os.Create(img)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(50 * 1024 * 1024); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	c := NewClient("http://127.0.0.1", "test-key", http.DefaultClient)
	_, err = c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{img}})
	if err == nil || (!strings.Contains(err.Error(), "50MB") && !strings.Contains(err.Error(), "smaller than 50MB")) {
		t.Fatalf("expected 50MB size error, got: %v", err)
	}
}

func TestEditImageMaskValidationSupportsWebPFirstImage(t *testing.T) {
	tmpDir := t.TempDir()
	img := filepath.Join(tmpDir, "img.webp")
	mask := filepath.Join(tmpDir, "mask.png")

	webpData, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAQAwCdASoCAAIAAUAmJaQAA3AA/vuUAAA=")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(img, webpData, 0o600); err != nil {
		t.Fatal(err)
	}
	alphaMask := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	alphaMask.Set(0, 0, color.NRGBA{A: 0})
	writePNG(t, mask, alphaMask)

	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"created": 1777395117,
			"data": []map[string]any{{
				"b64_json": "ZWRpdGVk",
			}},
		})
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "test-key", ts.Client())
	_, err = c.EditImage(context.Background(), EditImageRequest{Prompt: "x", Images: []string{img}, Mask: mask})
	if err != nil {
		t.Fatalf("expected webp+mask validation to pass, got: %v", err)
	}
	if gotPath != "/images/edits" {
		t.Fatalf("path = %q, want /images/edits", gotPath)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
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
