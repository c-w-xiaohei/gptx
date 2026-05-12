package cli

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJobHelpMentionsSearchImageAndBackgroundMode(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("job help should not fail: %v", err)
	}

	help := out.String()
	for _, want := range []string{"job start", "job wait", "job status", "job result", "job logs", "job cancel", "search", "image generate", "image edit", "--bg"} {
		if !strings.Contains(help, want) {
			t.Fatalf("job help missing %q, got %q", want, help)
		}
	}
}

func TestJobStartRequiresCommand(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "start"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires a command") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJobStartRejectsUnsupportedCommand(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_DIR", t.TempDir())

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "start", "--", "version"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "only search and image commands") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJobStartRejectsAPIKeyFlag(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_DIR", t.TempDir())

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "start", "--", "search", "query", "--api-key", "secret"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--api-key is not supported") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJobStartRejectsSearchWithoutDeep(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_DIR", t.TempDir())
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "start", "--", "search", "query"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "search background jobs require --deep") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJobStartRejectsSearchWithoutDeepBeforeAPIKey(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	t.Setenv("GPTX_JOB_DIR", t.TempDir())

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "start", "--", "search", "query"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "search background jobs require --deep") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJobStartRejectsDeepFlagAsFlagValue(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_DIR", t.TempDir())
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "start", "--", "search", "query", "--model", "--deep"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "search background jobs require --deep") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJobStartRejectsRootFlagsInsideStoredCommand(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_DIR", t.TempDir())

	for _, flag := range []string{"--base-url", "--timeout", "--format"} {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"job", "start", "--", "search", "query", flag, "value"})

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "must be passed before job start") {
			t.Fatalf("%s unexpected err: %v", flag, err)
		}
	}
}

func TestBackgroundRejectsRootAPIKeyFlag(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	t.Setenv("GPTX_JOB_DIR", t.TempDir())
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--api-key", "secret", "search", "query", "--bg"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--api-key is not supported") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJobStartRejectsDryRunImageCommand(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "start", "--json", "--", "image", "generate", "prompt", "--dry-run", "--out-dir", t.TempDir()})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--dry-run is not supported for background jobs") {
		t.Fatalf("unexpected err: %v", err)
	}
	entries, err := os.ReadDir(jobDir)
	if err != nil {
		t.Fatalf("read job dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry-run job should not create files, found %d entries", len(entries))
	}
}

func TestJobStartCreatesMetadataAndLogs(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "start", "--json", "--", "search", "prompt", "--deep"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("job start should not fail: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode job start json: %v\n%s", err, out.String())
	}
	jobID, ok := payload["job_id"].(string)
	if !ok || jobID == "" {
		t.Fatalf("missing job_id in %v", payload)
	}
	if got := payload["state"]; got != "queued" {
		t.Fatalf("state = %v, want queued", got)
	}

	metaPath := filepath.Join(jobDir, jobID, "job.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatalf("metadata leaked secret: %s", string(data))
	}
	for _, name := range []string{"stdout.log", "stderr.log", "status.json"} {
		if _, err := os.Stat(filepath.Join(jobDir, jobID, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestRunStoredJobPreservesBaseURLAndTimeoutFlags(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	jobID := "job_status"
	dir := filepath.Join(jobDir, jobID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "job.json"), `{"id":"job_status","operation":"status","command":["status"],"created_at":"2026-05-09T00:00:00Z","cwd":"`+filepath.ToSlash(t.TempDir())+`","stdout_path":"`+filepath.ToSlash(filepath.Join(dir, "stdout.log"))+`","stderr_path":"`+filepath.ToSlash(filepath.Join(dir, "stderr.log"))+`","metadata_path":"`+filepath.ToSlash(filepath.Join(dir, "job.json"))+`"}`)
	writeTestFile(t, filepath.Join(dir, "status.json"), `{"id":"job_status","state":"queued","operation":"status"}`)

	err := runStoredJobWithRoot(jobID, rootOptions{BaseURL: "https://example.test/v1", Timeout: 42})
	if err != nil {
		t.Fatalf("run stored job: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "result.json"))
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if !strings.Contains(string(data), "https://example.test/v1") {
		t.Fatalf("base URL was not propagated: %s", string(data))
	}
}

func TestSearchBackgroundRejectsMutuallyExclusiveInstructions(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_DIR", t.TempDir())
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "query", "--instructions", "a", "--instructions-file", "b", "--bg"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestSearchBackgroundSubmitsJob(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "query", "--deep", "--bg", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("search --deep --bg should not fail: %v", err)
	}
	if !strings.Contains(out.String(), "job_id") {
		t.Fatalf("expected job output, got %q", out.String())
	}
}

func TestSearchBackgroundWithoutDeepErrors(t *testing.T) {
	t.Setenv("GPTX_JOB_DIR", t.TempDir())
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "query", "--bg"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--bg is only supported with --deep for search") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestSearchTuningFlagsRequireDeep(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	for _, args := range [][]string{
		{"search", "query", "--reasoning-effort", "high"},
		{"search", "query", "--search-context-size", "high"},
		{"search", "query", "--max-tool-calls", "4"},
		{"search", "query", "--max-output-tokens", "2000"},
	} {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "require --deep") {
			t.Fatalf("%v unexpected err: %v", args, err)
		}
	}
}

func TestImageBackgroundSubmitsJobWithoutConflictingImageBackground(t *testing.T) {
	jobDir := t.TempDir()
	outDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "generate", "prompt", "--background", "opaque", "--bg", "--out-dir", outDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("image generate --bg should not fail: %v", err)
	}
	if !strings.Contains(out.String(), "job_id") {
		t.Fatalf("expected job output, got %q", out.String())
	}
}

func TestImageBackgroundDoesNotStoreRootTimeoutAsCommandArg(t *testing.T) {
	jobDir := t.TempDir()
	outDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--timeout", "20m", "image", "generate", "prompt", "--bg", "--out-dir", outDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("image generate --bg should not fail: %v", err)
	}
	if strings.Contains(out.String(), "--timeout") {
		t.Fatalf("root timeout leaked into stored command args: %s", out.String())
	}
}

func TestSearchBackgroundStoresCommandFlags(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")
	instructionsFile := filepath.Join(t.TempDir(), "instructions.txt")
	writeTestFile(t, instructionsFile, "be concise")
	contextFile := filepath.Join(t.TempDir(), "context.md")
	writeTestFile(t, contextFile, "context")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "query", "--deep", "--model", "custom-model", "--instructions-file", instructionsFile, "--context", contextFile, "--reasoning-effort", "medium", "--search-context-size", "low", "--max-tool-calls", "4", "--max-output-tokens", "2000", "--json", "--bg"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("search --deep --bg should not fail: %v", err)
	}
	for _, want := range []string{"--deep", "--model", "custom-model", "--instructions-file", instructionsFile, "--context", contextFile, "--reasoning-effort", "medium", "--search-context-size", "low", "--max-tool-calls", "4", "--max-output-tokens", "2000", "--json"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stored args missing %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "--bg") {
		t.Fatalf("stored args should not include --bg: %s", out.String())
	}
}

func TestImageGenerateBackgroundStoresCommandFlags(t *testing.T) {
	jobDir := t.TempDir()
	outDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	refPath := filepath.Join(outDir, "ref.png")
	writePNGFile(t, refPath, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	contextFile := filepath.Join(t.TempDir(), "brand.md")
	writeTestFile(t, contextFile, "brand")
	cmd.SetArgs([]string{"image", "generate", "prompt", "--image", refPath, "--context", contextFile, "--n", "2", "--out-dir", outDir, "--create-dirs", "--output-format", "webp", "--json", "--bg"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("image generate --bg should not fail: %v", err)
	}
	for _, want := range []string{"--image", refPath, "--context", contextFile, "--n", "2", "--out-dir", outDir, "--create-dirs", "--output-format", "webp", "--json"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stored args missing %q: %s", want, out.String())
		}
	}
}

func TestImageEditBackgroundStoresCommandFlags(t *testing.T) {
	jobDir := t.TempDir()
	outDir := t.TempDir()
	imagePath := filepath.Join(outDir, "in.png")
	maskPath := filepath.Join(outDir, "mask.png")
	writePNGFile(t, imagePath, image.NewNRGBA(image.Rect(0, 0, 3, 3)))
	alphaMask := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	alphaMask.SetNRGBA(1, 1, color.NRGBA{A: 128})
	writePNGFile(t, maskPath, alphaMask)
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "edit", "prompt", "--image", imagePath, "--mask", maskPath, "--out", filepath.Join(outDir, "out.png"), "--json", "--bg"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("image edit --bg should not fail: %v", err)
	}
	for _, want := range []string{"--image", imagePath, "--mask", maskPath, "--out", filepath.Join(outDir, "out.png"), "--json"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stored args missing %q: %s", want, out.String())
		}
	}
}

func TestImageGenerateRejectsDryRunWithBackground(t *testing.T) {
	jobDir := t.TempDir()
	outDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "generate", "prompt", "--dry-run", "--bg", "--out-dir", outDir, "--json"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--dry-run and --bg cannot be used together") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestImageEditRejectsDryRunWithBackground(t *testing.T) {
	jobDir := t.TempDir()
	outDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	t.Setenv("GPTX_OPENAI_API_KEY", "secret")
	t.Setenv("GPTX_JOB_TEST_NO_START", "1")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"image", "edit", "prompt", "--dry-run", "--bg", "--out-dir", outDir, "--json"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--dry-run and --bg cannot be used together") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJobListStatusResultLogsAndCancel(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	jobID := "job_test123"
	dir := filepath.Join(jobDir, jobID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "job.json"), `{"id":"job_test123","operation":"search","command":["search","q"],"created_at":"2026-05-09T00:00:00Z","stdout_path":"`+filepath.ToSlash(filepath.Join(dir, "stdout.log"))+`","stderr_path":"`+filepath.ToSlash(filepath.Join(dir, "stderr.log"))+`"}`)
	writeTestFile(t, filepath.Join(dir, "status.json"), `{"id":"job_test123","state":"succeeded","operation":"search","exit_code":0}`)
	writeTestFile(t, filepath.Join(dir, "result.json"), `{"job_id":"job_test123","state":"succeeded","output":{"text":"answer"}}`)
	writeTestFile(t, filepath.Join(dir, "stdout.log"), "answer\n")
	writeTestFile(t, filepath.Join(dir, "stderr.log"), "warn\n")

	for _, args := range [][]string{
		{"job", "list", "--json"},
		{"job", "status", jobID, "--json"},
		{"job", "result", jobID},
		{"job", "logs", jobID, "--stderr"},
		{"job", "cancel", jobID},
	} {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v failed: %v", args, err)
		}
		if strings.TrimSpace(out.String()) == "" {
			t.Fatalf("%v produced empty output", args)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "cancel")); err != nil {
		t.Fatalf("cancel marker missing: %v", err)
	}
}

func TestJobWaitPrintsSucceededResult(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	jobID := "job_wait_success"
	dir := filepath.Join(jobDir, jobID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "job.json"), `{"id":"job_wait_success","operation":"search","command":["search","q"],"created_at":"2026-05-09T00:00:00Z","stdout_path":"`+filepath.ToSlash(filepath.Join(dir, "stdout.log"))+`","stderr_path":"`+filepath.ToSlash(filepath.Join(dir, "stderr.log"))+`"}`)
	writeTestFile(t, filepath.Join(dir, "status.json"), `{"id":"job_wait_success","state":"succeeded","operation":"search","exit_code":0}`)
	writeTestFile(t, filepath.Join(dir, "result.json"), `{"job_id":"job_wait_success","state":"succeeded","output":{"text":"answer"}}`)

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "wait", jobID})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("job wait should not fail: %v", err)
	}
	if !strings.Contains(out.String(), "answer") {
		t.Fatalf("wait output missing result: %q", out.String())
	}
}

func TestJobWaitReturnsErrorForFailedJob(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	jobID := "job_wait_failed"
	dir := filepath.Join(jobDir, jobID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "job.json"), `{"id":"job_wait_failed","operation":"image.generate","command":["image","generate","p"],"created_at":"2026-05-09T00:00:00Z","stdout_path":"`+filepath.ToSlash(filepath.Join(dir, "stdout.log"))+`","stderr_path":"`+filepath.ToSlash(filepath.Join(dir, "stderr.log"))+`"}`)
	writeTestFile(t, filepath.Join(dir, "status.json"), `{"id":"job_wait_failed","state":"failed","operation":"image.generate","exit_code":1,"error":"context deadline exceeded"}`)
	writeTestFile(t, filepath.Join(dir, "result.json"), `{"job_id":"job_wait_failed","state":"failed","error":"context deadline exceeded","exit_code":1}`)

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "wait", jobID, "--json"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out.String(), `"state":"failed"`) {
		t.Fatalf("wait json output missing failed result: %q", out.String())
	}
}

func TestJobRemoveRejectsRunningJob(t *testing.T) {
	jobDir := t.TempDir()
	t.Setenv("GPTX_JOB_DIR", jobDir)
	jobID := "job_running"
	dir := filepath.Join(jobDir, jobID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "job.json"), `{"id":"job_running","operation":"search","command":["search","q"],"created_at":"2026-05-09T00:00:00Z","stdout_path":"`+filepath.ToSlash(filepath.Join(dir, "stdout.log"))+`","stderr_path":"`+filepath.ToSlash(filepath.Join(dir, "stderr.log"))+`"}`)
	writeTestFile(t, filepath.Join(dir, "status.json"), `{"id":"job_running","state":"running","operation":"search","pid":12345}`)

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"job", "rm", jobID})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot remove a running job") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
