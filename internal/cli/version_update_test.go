package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "newer semantic version", a: "v1.2.0", b: "v1.1.9", want: 1},
		{name: "equal with optional v prefix", a: "1.2.0", b: "v1.2.0", want: 0},
		{name: "older patch", a: "v1.2.0", b: "v1.2.1", want: -1},
		{name: "dev is unknown and not newer", a: "dev", b: "v1.2.1", want: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareVersions(tc.a, tc.b); got != tc.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestVersionCheckTextFetchesAndCachesLatestRelease(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	cacheDir := t.TempDir()
	restore := setVersionCheckTestHooks(t, cacheDir, time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))
	defer restore()

	oldVersion := Version
	Version = "v1.0.0"
	t.Cleanup(func() { Version = oldVersion })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/latest" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.0","html_url":"https://github.com/c-w-xiaohei/gptx/releases/tag/v1.2.0"}`))
	}))
	defer ts.Close()
	githubLatestReleaseURL = ts.URL + "/latest"

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version", "check"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version check should not fail: %v", err)
	}
	text := out.String()
	for _, want := range []string{"current_version: v1.0.0", "latest_version: v1.2.0", "update_available: true", installHint} {
		if !strings.Contains(text, want) {
			t.Fatalf("version check output missing %q, got %q", want, text)
		}
	}

	data, err := os.ReadFile(filepath.Join(cacheDir, "gptx", "update-check.json"))
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	var cache updateCheckCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatalf("decode cache: %v", err)
	}
	if cache.CheckedAt != "2026-05-10T12:00:00Z" || cache.CurrentVersion != "v1.0.0" || cache.LatestVersion != "v1.2.0" || cache.LatestURL == "" || cache.InstallHint != installHint {
		t.Fatalf("unexpected cache: %+v", cache)
	}
}

func TestVersionCheckJSONReportsUnableToDetermineLatest(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	cacheDir := t.TempDir()
	restore := setVersionCheckTestHooks(t, cacheDir, time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))
	defer restore()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()
	githubLatestReleaseURL = ts.URL

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "json", "version", "check"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version check json should not fail: %v", err)
	}
	var payload updateCheckOutput
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	if payload.LatestVersion != "" || payload.LatestURL != "" || payload.UpdateAvailable {
		t.Fatalf("unexpected latest metadata: %+v", payload)
	}
	if !strings.Contains(payload.Message, "unable to determine latest") {
		t.Fatalf("message = %q", payload.Message)
	}
	if payload.InstallHint != installHint {
		t.Fatalf("install hint = %q", payload.InstallHint)
	}
}

func TestVersionCheckFetchFailurePreservesExistingCache(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	cacheDir := t.TempDir()
	restore := setVersionCheckTestHooks(t, cacheDir, time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))
	defer restore()

	cachePath := filepath.Join(cacheDir, "gptx", "update-check.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatal(err)
	}
	originalCache := `{"checked_at":"2026-05-09T00:00:00Z","current_version":"v1.0.0","latest_version":"v1.1.0","latest_url":"https://example.test/release","install_hint":"` + installHint + `"}`
	if err := os.WriteFile(cachePath, []byte(originalCache), 0o600); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()
	githubLatestReleaseURL = ts.URL

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version", "check"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version check should fall back to cache: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "latest_version: v1.1.0") || !strings.Contains(got, "using cached release metadata") {
		t.Fatalf("expected cached release output, got %q", got)
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if string(data) != originalCache {
		t.Fatalf("cache should not be overwritten on fetch failure\nwant %q\ngot  %q", originalCache, string(data))
	}
}

func TestVersionCheckInvalidGlobalFormatErrors(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "yaml", "version", "check"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid global format to error")
	}
	if !strings.Contains(err.Error(), "invalid --format") {
		t.Fatalf("expected invalid format error, got %v", err)
	}
}

func TestVersionCheckNoUpdateCheckSkipsNetworkAndUsesCache(t *testing.T) {
	t.Setenv("GPTX_NO_UPDATE_CHECK", "1")
	cacheDir := t.TempDir()
	restore := setVersionCheckTestHooks(t, cacheDir, time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))
	defer restore()

	cachePath := filepath.Join(cacheDir, "gptx", "update-check.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte(`{"checked_at":"2026-05-09T00:00:00Z","current_version":"v1.0.0","latest_version":"v1.1.0","latest_url":"https://example.test/release","install_hint":"`+installHint+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	githubLatestReleaseURL = "http://127.0.0.1:1/should-not-be-called"

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version", "check"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version check should use cache when network checks are disabled: %v", err)
	}
	text := out.String()
	for _, want := range []string{"latest_version: v1.1.0", "update checks disabled by GPTX_NO_UPDATE_CHECK=1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q, got %q", want, text)
		}
	}
}

func TestUpdateCommandPrintsInstallHintWithoutAPIKey(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("update should not fail: %v", err)
	}
	if got := out.String(); !strings.Contains(got, installHint) {
		t.Fatalf("update output missing install hint, got %q", got)
	}
}

func TestUpdateCommandTextIncludesFallbackGitHubArchiveCommands(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	oldGOOS := updateRuntimeGOOS
	oldGOARCH := updateRuntimeGOARCH
	updateRuntimeGOOS = "linux"
	updateRuntimeGOARCH = "amd64"
	t.Cleanup(func() {
		updateRuntimeGOOS = oldGOOS
		updateRuntimeGOARCH = oldGOARCH
	})

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("update should not fail: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		installHint,
		"Fallback GitHub release archive install:",
		"gh release download --repo c-w-xiaohei/gptx --pattern 'gptx_*_",
		".tar.gz' --dir \"$tmp\" --clobber",
		"gh release download --repo c-w-xiaohei/gptx --pattern checksums.txt --dir \"$tmp\" --clobber",
		"sha256sum -c --ignore-missing checksums.txt",
		"tar -xzf \"$tmp\"/gptx_*.tar.gz -C \"$tmp\"",
		"install -m 0755 \"$tmp/gptx\" \"$HOME/.local/bin/gptx\"",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("update output missing %q, got %q", want, text)
		}
	}
}

func TestUpdateCommandHonorsJSONOutput(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")

	for _, args := range [][]string{{"--format", "json", "update"}, {"--json", "update"}} {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v update should not fail: %v", args, err)
		}
		var payload updateOutput
		if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
			t.Fatalf("%v decode update json: %v\n%s", args, err, out.String())
		}
		if payload.InstallHint != installHint || !strings.Contains(payload.Message, "install or update") {
			t.Fatalf("%v unexpected payload: %+v", args, payload)
		}
	}
}

func TestUpdateCommandJSONIncludesFallbackCommands(t *testing.T) {
	t.Setenv("GPTX_OPENAI_API_KEY", "")
	oldGOOS := updateRuntimeGOOS
	oldGOARCH := updateRuntimeGOARCH
	updateRuntimeGOOS = "linux"
	updateRuntimeGOARCH = "amd64"
	t.Cleanup(func() {
		updateRuntimeGOOS = oldGOOS
		updateRuntimeGOARCH = oldGOARCH
	})

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "json", "update"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("update should not fail: %v", err)
	}
	var payload updateOutput
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode update json: %v\n%s", err, out.String())
	}
	if payload.InstallHint != installHint || !strings.Contains(payload.Message, "install or update") {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if len(payload.FallbackCommands) == 0 {
		t.Fatalf("expected fallback commands in payload: %+v", payload)
	}
}

func TestUpdateFallbackCommandsForLinuxAMD64(t *testing.T) {
	commands := updateFallbackCommands("linux", "amd64")
	if len(commands) != 8 {
		t.Fatalf("expected 8 commands, got %d: %#v", len(commands), commands)
	}
	for _, want := range []string{
		`set -e`,
		`tmp="$(mktemp -d)"`,
		`gh release download --repo c-w-xiaohei/gptx --pattern 'gptx_*_linux_amd64.tar.gz' --dir "$tmp" --clobber`,
		`gh release download --repo c-w-xiaohei/gptx --pattern checksums.txt --dir "$tmp" --clobber`,
		`(cd "$tmp" && sha256sum -c --ignore-missing checksums.txt)`,
		`tar -xzf "$tmp"/gptx_*.tar.gz -C "$tmp"`,
		`mkdir -p "$HOME/.local/bin"`,
		`install -m 0755 "$tmp/gptx" "$HOME/.local/bin/gptx"`,
	} {
		if !containsString(commands, want) {
			t.Fatalf("commands missing %q: %#v", want, commands)
		}
	}
}

func TestUpdateFallbackCommandsUnsupportedPlatform(t *testing.T) {
	if commands := updateFallbackCommands("windows", "amd64"); len(commands) != 0 {
		t.Fatalf("expected no fallback commands for unsupported platform, got %#v", commands)
	}
	if commands := updateFallbackCommands("darwin", "amd64"); len(commands) != 0 {
		t.Fatalf("expected no fallback commands for darwin until checksum verification is portable, got %#v", commands)
	}
	if commands := updateFallbackCommands("linux", "386"); len(commands) != 0 {
		t.Fatalf("expected no fallback commands for unsupported architecture, got %#v", commands)
	}
}

func TestVersionAndUpdateRejectInvalidGlobalFormat(t *testing.T) {
	for _, args := range [][]string{{"--format", "yaml", "version"}, {"--format", "yaml", "update"}} {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "invalid --format") {
			t.Fatalf("%v expected invalid format error, got %v", args, err)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestVersionAndUpdateHelpDoNotFetchLatestRelease(t *testing.T) {
	restore := setVersionCheckTestHooks(t, t.TempDir(), time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))
	defer restore()
	githubLatestReleaseURL = "http://127.0.0.1:1/should-not-be-called"

	for _, args := range [][]string{{"version", "check", "--help"}, {"update", "--help"}} {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v help should not fail or fetch: %v", args, err)
		}
		for _, want := range []string{installHint, "GPTX_NO_UPDATE_CHECK"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("%v help missing %q, got %q", args, want, out.String())
			}
		}
	}
}

func setVersionCheckTestHooks(t *testing.T, cacheDir string, now time.Time) func() {
	t.Helper()
	oldCacheDir := updateCheckUserCacheDir
	oldNow := updateCheckNow
	oldURL := githubLatestReleaseURL
	updateCheckUserCacheDir = func() (string, error) { return cacheDir, nil }
	updateCheckNow = func() time.Time { return now }
	return func() {
		updateCheckUserCacheDir = oldCacheDir
		updateCheckNow = oldNow
		githubLatestReleaseURL = oldURL
	}
}
