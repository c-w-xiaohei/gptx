package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const installHint = "go install github.com/c-w-xiaohei/gptx/cmd/gptx@latest"

var (
	githubLatestReleaseURL  = "https://api.github.com/repos/c-w-xiaohei/gptx/releases/latest"
	updateCheckUserCacheDir = os.UserCacheDir
	updateCheckNow          = time.Now
	updateRuntimeGOOS       = runtime.GOOS
	updateRuntimeGOARCH     = runtime.GOARCH
)

type updateCheckCache struct {
	CheckedAt      string `json:"checked_at"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	LatestURL      string `json:"latest_url"`
	InstallHint    string `json:"install_hint"`
}

type updateCheckOutput struct {
	CheckedAt       string `json:"checked_at"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	LatestURL       string `json:"latest_url,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	InstallHint     string `json:"install_hint"`
	Message         string `json:"message"`
	Source          string `json:"source"`
}

type updateOutput struct {
	InstallHint      string   `json:"install_hint"`
	FallbackCommands []string `json:"fallback_commands,omitempty"`
	Message          string   `json:"message"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func newVersionCheckCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check whether a newer gptx release is available",
		Long: `Check the latest gptx GitHub release metadata and print update guidance.

This command does not require GPTX_OPENAI_API_KEY. It may perform a network request and caches metadata under os.UserCacheDir()/gptx/update-check.json with checked_at, current_version, latest_version, latest_url, and install_hint.

Set GPTX_NO_UPDATE_CHECK=1 to suppress the network request and use the local cache when available.

Supported install/update command:
  ` + installHint,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := normalizeRootFormat(root.Format); err != nil {
				return err
			}
			out, err := runVersionCheck()
			if err != nil {
				return err
			}
			if isJSONOutput(root.Format, root.JSON, false) {
				return writeJSON(cmd, out)
			}
			return writeVersionCheckText(cmd, out)
		},
	}
}

func newUpdateCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Print the supported gptx update command",
		Long: `Print the supported install/update command for gptx.

This command does not require GPTX_OPENAI_API_KEY and does not perform a network request. Set GPTX_NO_UPDATE_CHECK=1 to suppress network update checks where applicable.

Supported install/update command:
  ` + installHint + `

On linux amd64/arm64, this command also prints a GitHub release archive fallback using the local GOOS/GOARCH.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := normalizeRootFormat(root.Format)
			if err != nil {
				return err
			}
			fallbackCommands := updateFallbackCommands(updateRuntimeGOOS, updateRuntimeGOARCH)
			out := updateOutput{
				InstallHint:      installHint,
				FallbackCommands: fallbackCommands,
				Message:          "run this command to install or update gptx",
			}
			if isJSONOutput(format, root.JSON, false) {
				return writeJSON(cmd, out)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Run this command to install or update gptx:\n  %s\n", installHint); err != nil {
				return err
			}
			if len(fallbackCommands) == 0 {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "\nNo generic GitHub release archive fallback is available for this platform.")
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "\nFallback GitHub release archive install:\n  %s\n", strings.Join(fallbackCommands, "\n  "))
			return err
		},
	}
}

func updateFallbackCommands(goos, goarch string) []string {
	if goos != "linux" {
		return nil
	}
	if goarch != "amd64" && goarch != "arm64" {
		return nil
	}
	return []string{
		`set -e`,
		`tmp="$(mktemp -d)"`,
		`latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' https://github.com/c-w-xiaohei/gptx/releases/latest)`,
		`tag="${latest_url##*/}"`,
		`assets_url="https://github.com/c-w-xiaohei/gptx/releases/expanded_assets/${tag}"`,
		fmt.Sprintf(`asset_path=$(curl -fsSL "$assets_url" | grep -oE '/c-w-xiaohei/gptx/releases/download/[^"<> ]+gptx_[^"<> ]+_%s_%s\.tar\.gz' | head -n 1)`, goos, goarch),
		`checksums_path=$(curl -fsSL "$assets_url" | grep -oE '/c-w-xiaohei/gptx/releases/download/[^"<> ]+checksums\.txt' | head -n 1)`,
		`archive_name="${asset_path##*/}"`,
		`archive="$tmp/$archive_name"`,
		`checksums="$tmp/checksums.txt"`,
		`curl -fL --retry 3 -o "$archive" "https://github.com${asset_path}"`,
		`curl -fL --retry 3 -o "$checksums" "https://github.com${checksums_path}"`,
		`(cd "$tmp" && sha256sum -c --ignore-missing checksums.txt)`,
		`tar -xzf "$archive" -C "$tmp"`,
		`mkdir -p "$HOME/.local/bin"`,
		`install -m 0755 "$tmp/gptx" "$HOME/.local/bin/gptx"`,
	}
}

func runVersionCheck() (updateCheckOutput, error) {
	current := currentVersion()
	if updateChecksDisabled() {
		if cache, err := readUpdateCheckCache(); err == nil {
			return outputFromCache(cache, current, "cache", "update checks disabled by GPTX_NO_UPDATE_CHECK=1"), nil
		}
		checkedAt := updateCheckNow().UTC().Format(time.RFC3339)
		return updateCheckOutput{
			CheckedAt:      checkedAt,
			CurrentVersion: current,
			InstallHint:    installHint,
			Message:        "update checks disabled by GPTX_NO_UPDATE_CHECK=1; unable to determine latest version",
			Source:         "disabled",
		}, nil
	}

	checkedAt := updateCheckNow().UTC().Format(time.RFC3339)
	release, err := fetchLatestRelease()
	if err != nil {
		if cache, cacheErr := readUpdateCheckCache(); cacheErr == nil {
			return outputFromCache(cache, current, "cache", "unable to refresh latest version; using cached release metadata"), nil
		}
		return updateCheckOutput{
			CheckedAt:      checkedAt,
			CurrentVersion: current,
			InstallHint:    installHint,
			Message:        "unable to determine latest version; use the install hint to update",
			Source:         "github",
		}, nil
	}
	cache := updateCheckCache{
		CheckedAt:      checkedAt,
		CurrentVersion: current,
		LatestVersion:  strings.TrimSpace(release.TagName),
		LatestURL:      strings.TrimSpace(release.HTMLURL),
		InstallHint:    installHint,
	}
	_ = writeUpdateCheckCache(cache)
	if strings.TrimSpace(cache.LatestVersion) == "" {
		return outputFromCache(cache, current, "github", "unable to determine latest version; use the install hint to update"), nil
	}
	return outputFromCache(cache, current, "github", updateMessage(current, cache.LatestVersion)), nil
}

func fetchLatestRelease() (githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, githubLatestReleaseURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gptx-update-check")
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubRelease{}, fmt.Errorf("latest release request failed: %s", resp.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubRelease{}, fmt.Errorf("latest release metadata missing tag_name")
	}
	return release, nil
}

func outputFromCache(cache updateCheckCache, currentVersion, source, message string) updateCheckOutput {
	current := cache.CurrentVersion
	if strings.TrimSpace(currentVersion) != "" {
		current = currentVersion
	}
	return updateCheckOutput{
		CheckedAt:       cache.CheckedAt,
		CurrentVersion:  current,
		LatestVersion:   cache.LatestVersion,
		LatestURL:       cache.LatestURL,
		UpdateAvailable: compareVersions(cache.LatestVersion, current) > 0,
		InstallHint:     firstNonEmpty(cache.InstallHint, installHint),
		Message:         message,
		Source:          source,
	}
}

func updateMessage(currentVersion, latestVersion string) string {
	switch compareVersions(latestVersion, currentVersion) {
	case 1:
		return "update available"
	case 0:
		return "gptx is up to date"
	default:
		return "latest version is not newer than current version"
	}
}

func writeVersionCheckText(cmd *cobra.Command, out updateCheckOutput) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "current_version: %s\nlatest_version: %s\nlatest_url: %s\nupdate_available: %t\ninstall_hint: %s\nmessage: %s\n",
		out.CurrentVersion,
		valueOr(out.LatestVersion, "unknown"),
		valueOr(out.LatestURL, "unknown"),
		out.UpdateAvailable,
		out.InstallHint,
		out.Message,
	)
	return err
}

func readUpdateCheckCache() (updateCheckCache, error) {
	path, err := updateCheckCachePath()
	if err != nil {
		return updateCheckCache{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return updateCheckCache{}, err
	}
	var cache updateCheckCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return updateCheckCache{}, err
	}
	return cache, nil
}

func writeUpdateCheckCache(cache updateCheckCache) error {
	path, err := updateCheckCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-check-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func updateCheckCachePath() (string, error) {
	dir, err := updateCheckUserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gptx", "update-check.json"), nil
}

func updateChecksDisabled() bool {
	return strings.TrimSpace(os.Getenv("GPTX_NO_UPDATE_CHECK")) == "1"
}

func compareVersions(a, b string) int {
	ap, aok := parseVersionParts(a)
	bp, bok := parseVersionParts(b)
	if !aok && !bok {
		return 0
	}
	if !aok {
		return -1
	}
	if !bok {
		return 1
	}
	for i := 0; i < len(ap) || i < len(bp); i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func parseVersionParts(version string) ([]int, bool) {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	if version == "" || version == "dev" {
		return nil, false
	}
	version = strings.Split(version, "-")[0]
	fields := strings.Split(version, ".")
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			return nil, false
		}
		value, err := strconv.Atoi(field)
		if err != nil {
			return nil, false
		}
		parts = append(parts, value)
	}
	return parts, len(parts) > 0
}
