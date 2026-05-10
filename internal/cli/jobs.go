package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	jobStateQueued    = "queued"
	jobStateRunning   = "running"
	jobStateSucceeded = "succeeded"
	jobStateFailed    = "failed"
	jobStateCanceled  = "canceled"
	jobStateUnknown   = "unknown"

	jobEnvDir     = "GPTX_JOB_DIR"
	jobEnvNoStart = "GPTX_JOB_TEST_NO_START"
)

type jobMetadata struct {
	ID           string    `json:"id"`
	Operation    string    `json:"operation"`
	Command      []string  `json:"command"`
	CreatedAt    time.Time `json:"created_at"`
	CWD          string    `json:"cwd"`
	PID          int       `json:"pid,omitempty"`
	StdoutPath   string    `json:"stdout_path"`
	StderrPath   string    `json:"stderr_path"`
	MetadataPath string    `json:"metadata_path"`
}

type jobStatusRecord struct {
	ID         string     `json:"id"`
	State      string     `json:"state"`
	Operation  string     `json:"operation"`
	PID        int        `json:"pid,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ExitCode   *int       `json:"exit_code"`
	Error      string     `json:"error,omitempty"`
}

type jobStartOutput struct {
	Command      string   `json:"command"`
	JobID        string   `json:"job_id"`
	State        string   `json:"state"`
	Operation    string   `json:"operation"`
	Args         []string `json:"args"`
	StdoutPath   string   `json:"stdout_path"`
	StderrPath   string   `json:"stderr_path"`
	MetadataPath string   `json:"metadata_path"`
}

func newJobCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Run and inspect local background jobs",
		Long: `Run search and image commands as local background jobs.

For deep search and real image API calls, prefer background mode so long remote
calls can continue outside the interactive session. Use --bg on search --deep,
image generate, and image edit, or use job start explicitly.

Examples:
  gptx search "latest OpenAI image docs" --deep --json --bg
  gptx image generate "poster" --dry-run --out ./poster.png --json
  gptx image generate "poster" --out ./poster.png --json --bg
  gptx job start -- search "latest OpenAI image docs" --deep --json
  gptx job start -- image generate "poster" --out ./poster.png --json
  gptx job status <job_id>
  gptx job result <job_id>
  gptx job logs <job_id>
  gptx job cancel <job_id>`}
	cmd.AddCommand(newJobStartCommand(root))
	cmd.AddCommand(newJobListCommand(root))
	cmd.AddCommand(newJobStatusCommand(root))
	cmd.AddCommand(newJobResultCommand(root))
	cmd.AddCommand(newJobLogsCommand(root))
	cmd.AddCommand(newJobCancelCommand(root))
	cmd.AddCommand(newJobRemoveCommand(root))
	return cmd
}

func newJobStartCommand(root *rootOptions) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "start -- <search|image generate|image edit> [args...]",
		Short: "Start a search or image command in the local background",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("job start requires a command after --")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if rootAPIKeyFlagChanged(cmd) {
				return errors.New("--api-key is not supported for background jobs; use GPTX_OPENAI_API_KEY instead")
			}
			if err := validateJobCommand(args); err != nil {
				return err
			}
			if containsDryRunFlag(args) {
				return errors.New("--dry-run is not supported for background jobs; run dry-run in the foreground, then remove --dry-run for the real background call")
			}
			resolvedRoot, err := resolveRootOptions(root, true)
			if err != nil {
				return err
			}
			out, err := startBackgroundJob(args, resolvedRoot, "")
			if err != nil {
				return err
			}
			return writeJobStartOutput(cmd, isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, jsonOut), out)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON output")
	return cmd
}

func newJobListCommand(root *rootOptions) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local background jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedRoot, err := resolveRootOptions(root, false)
			if err != nil {
				return err
			}
			jobs, err := listJobs()
			if err != nil {
				return err
			}
			if isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, jsonOut) {
				return writeJSON(cmd, jobs)
			}
			for _, item := range jobs {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.Status.ID, item.Status.State, item.Meta.Operation); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON output")
	return cmd
}

func newJobStatusCommand(root *rootOptions) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status <job_id>",
		Short: "Show local background job status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedRoot, err := resolveRootOptions(root, false)
			if err != nil {
				return err
			}
			meta, status, err := readJob(args[0])
			if err != nil {
				return err
			}
			if isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, jsonOut) {
				return writeJSON(cmd, status)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "job_id: %s\nstate: %s\noperation: %s\nstdout: %s\nstderr: %s\n", meta.ID, status.State, meta.Operation, meta.StdoutPath, meta.StderrPath)
			return err
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON output")
	return cmd
}

func newJobResultCommand(root *rootOptions) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "result <job_id>",
		Short: "Print a local background job result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedRoot, err := resolveRootOptions(root, false)
			if err != nil {
				return err
			}
			path, err := jobFile(args[0], "result.json")
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read job result: %w", err)
			}
			if isJSONOutput(resolvedRoot.Format, resolvedRoot.JSON, jsonOut) {
				_, err = cmd.OutOrStdout().Write(data)
				if err == nil && len(data) > 0 && data[len(data)-1] != '\n' {
					_, err = fmt.Fprintln(cmd.OutOrStdout())
				}
				return err
			}
			return writeHumanJobResult(cmd.OutOrStdout(), data)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON output")
	return cmd
}

func newJobLogsCommand(root *rootOptions) *cobra.Command {
	var stderrLog bool
	cmd := &cobra.Command{
		Use:   "logs <job_id>",
		Short: "Print local background job logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "stdout.log"
			if stderrLog {
				name = "stderr.log"
			}
			path, err := jobFile(args[0], name)
			if err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open job log: %w", err)
			}
			defer f.Close()
			_, err = io.Copy(cmd.OutOrStdout(), f)
			return err
		},
	}
	cmd.Flags().BoolVar(&stderrLog, "stderr", false, "print stderr log")
	return cmd
}

func newJobCancelCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <job_id>",
		Short: "Request cancellation of a local background job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := jobFile(args[0], "cancel")
			if err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o600); err != nil {
				return fmt.Errorf("write cancel marker: %w", err)
			}
			meta, status, err := readJob(args[0])
			if err != nil {
				return err
			}
			if status.State == jobStateRunning && status.PID > 0 {
				if err := signalJobCancel(status.PID); err != nil {
					return err
				}
				finishJob(meta.ID, jobStateCanceled, 1, "canceled by user")
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "cancel requested: %s\n", args[0])
			return err
		},
	}
	return cmd
}

func newJobRemoveCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <job_id>",
		Short: "Remove local background job files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, status, err := readJob(args[0])
			if err == nil && status.State == jobStateRunning {
				return errors.New("cannot remove a running job; cancel it first")
			}
			dir, err := jobDir(args[0])
			if err != nil {
				return err
			}
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("remove job: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", args[0])
			return err
		},
	}
	return cmd
}

func newRunJobCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "__run-job <job_id>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedRoot, err := resolveRootOptions(root, false)
			if err != nil {
				return err
			}
			return runStoredJobWithRoot(args[0], resolvedRoot)
		},
	}
	return cmd
}

func startBackgroundJob(args []string, root rootOptions, operationOverride string) (jobStartOutput, error) {
	if err := validateJobCommand(args); err != nil {
		return jobStartOutput{}, err
	}
	if containsDryRunFlag(args) {
		return jobStartOutput{}, errors.New("--dry-run is not supported for background jobs; run dry-run in the foreground, then remove --dry-run for the real background call")
	}
	if containsAPIKeyFlag(args) {
		return jobStartOutput{}, errors.New("--api-key is not supported for background jobs; use GPTX_OPENAI_API_KEY instead")
	}
	rootDir, err := jobsRoot()
	if err != nil {
		return jobStartOutput{}, err
	}
	if err := os.MkdirAll(rootDir, 0o700); err != nil {
		return jobStartOutput{}, fmt.Errorf("create jobs directory: %w", err)
	}
	jobID, err := newJobID()
	if err != nil {
		return jobStartOutput{}, err
	}
	dir := filepath.Join(rootDir, jobID)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return jobStartOutput{}, fmt.Errorf("create job directory: %w", err)
	}
	stdoutPath := filepath.Join(dir, "stdout.log")
	stderrPath := filepath.Join(dir, "stderr.log")
	metadataPath := filepath.Join(dir, "job.json")
	for _, path := range []string{stdoutPath, stderrPath} {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return jobStartOutput{}, fmt.Errorf("create job log: %w", err)
		}
		_ = f.Close()
	}
	cwd, _ := os.Getwd()
	meta := jobMetadata{
		ID:           jobID,
		Operation:    operationName(args, operationOverride),
		Command:      append([]string(nil), args...),
		CreatedAt:    time.Now().UTC(),
		CWD:          cwd,
		StdoutPath:   stdoutPath,
		StderrPath:   stderrPath,
		MetadataPath: metadataPath,
	}
	status := jobStatusRecord{ID: jobID, State: jobStateQueued, Operation: meta.Operation, UpdatedAt: time.Now().UTC()}
	if err := writeJSONFile(metadataPath, meta); err != nil {
		return jobStartOutput{}, err
	}
	if err := writeJSONFile(filepath.Join(dir, "status.json"), status); err != nil {
		return jobStartOutput{}, err
	}
	if os.Getenv(jobEnvNoStart) == "" {
		pid, err := spawnJobWorker(jobID, stdoutPath, stderrPath, root)
		if err != nil {
			status.State = jobStateFailed
			status.Error = err.Error()
			code := 1
			status.ExitCode = &code
			status.UpdatedAt = time.Now().UTC()
			_ = writeJSONFile(filepath.Join(dir, "status.json"), status)
			return jobStartOutput{}, err
		}
		meta.PID = pid
		status.PID = pid
		status.State = jobStateRunning
		status.UpdatedAt = time.Now().UTC()
		if err := writeJSONFile(metadataPath, meta); err != nil {
			return jobStartOutput{}, err
		}
		if err := writeJSONFile(filepath.Join(dir, "status.json"), status); err != nil {
			return jobStartOutput{}, err
		}
	}
	return jobStartOutput{Command: "job start", JobID: jobID, State: status.State, Operation: meta.Operation, Args: meta.Command, StdoutPath: stdoutPath, StderrPath: stderrPath, MetadataPath: metadataPath}, nil
}

func rootAPIKeyFlagChanged(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Root() == nil {
		return false
	}
	return cmd.Root().PersistentFlags().Changed("api-key")
}

func spawnJobWorker(jobID, stdoutPath, stderrPath string, root rootOptions) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("resolve executable: %w", err)
	}
	args := []string{"__run-job", jobID}
	if root.BaseURL != "" {
		args = append(args, "--base-url", root.BaseURL)
	}
	if root.Timeout > 0 {
		args = append(args, "--timeout", root.Timeout.String())
	}
	child := exec.Command(exe, args...)
	stdout, err := os.OpenFile(stdoutPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(stderrPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer stderr.Close()
	child.Stdin = nil
	child.Stdout = stdout
	child.Stderr = stderr
	configureJobWorkerProcess(child)
	if err := child.Start(); err != nil {
		return 0, fmt.Errorf("start job worker: %w", err)
	}
	pid := child.Process.Pid
	if err := child.Process.Release(); err != nil {
		return 0, fmt.Errorf("release job worker: %w", err)
	}
	return pid, nil
}

func runStoredJob(jobID string) error {
	return runStoredJobWithRoot(jobID, rootOptions{})
}

func runStoredJobWithRoot(jobID string, workerRoot rootOptions) error {
	meta, status, err := readJob(jobID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	status.State = jobStateRunning
	status.StartedAt = &now
	status.UpdatedAt = now
	status.PID = os.Getpid()
	if err := writeJobStatus(jobID, status); err != nil {
		return err
	}
	if err := os.Chdir(meta.CWD); err != nil {
		finishJob(jobID, jobStateFailed, 1, fmt.Sprintf("change directory: %v", err))
		return err
	}
	runArgs := append([]string(nil), meta.Command...)
	runArgs = stripBackgroundFlag(runArgs)
	if !hasJSONFlag(runArgs) {
		runArgs = append(runArgs, "--json")
	}
	root := NewRootCommand()
	var out, errOut strings.Builder
	root.SetOut(&out)
	root.SetErr(&errOut)
	applyWorkerRootOptions(root, workerRoot)
	root.SetArgs(runArgs)
	err = root.Execute()
	if out.String() != "" {
		_ = os.WriteFile(meta.StdoutPath, []byte(out.String()), 0o600)
	}
	if errOut.String() != "" {
		_ = os.WriteFile(meta.StderrPath, []byte(errOut.String()), 0o600)
	}
	if cancelRequested(jobID) {
		finishJob(jobID, jobStateCanceled, 1, "canceled by user")
		return nil
	}
	if err != nil {
		message := err.Error()
		_ = appendFile(meta.StderrPath, message+"\n")
		finishJob(jobID, jobStateFailed, 1, message)
		return nil
	}
	trimmedOut := strings.TrimSpace(out.String())
	var output any = map[string]any{}
	if trimmedOut != "" {
		var decoded any
		if err := json.Unmarshal([]byte(trimmedOut), &decoded); err == nil {
			output = decoded
		} else {
			output = map[string]any{"text": trimmedOut}
		}
	}
	result := map[string]any{"job_id": jobID, "state": jobStateSucceeded, "operation": meta.Operation, "output": output}
	if err := writeJSONFile(filepath.Join(filepath.Dir(meta.MetadataPath), "result.json"), result); err != nil {
		finishJob(jobID, jobStateFailed, 1, err.Error())
		return err
	}
	finishJob(jobID, jobStateSucceeded, 0, "")
	return nil
}

func applyWorkerRootOptions(cmd *cobra.Command, root rootOptions) {
	if root.BaseURL != "" {
		_ = cmd.PersistentFlags().Set("base-url", root.BaseURL)
	}
	if root.Timeout > 0 {
		_ = cmd.PersistentFlags().Set("timeout", root.Timeout.String())
	}
	if root.Format != "" {
		_ = cmd.PersistentFlags().Set("format", root.Format)
	}
	if root.JSON {
		_ = cmd.PersistentFlags().Set("json", "true")
	}
}

func signalJobCancel(pid int) error {
	return signalJobProcess(pid)
}

func finishJob(jobID, state string, exitCode int, message string) {
	_, status, err := readJob(jobID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	status.State = state
	status.UpdatedAt = now
	status.FinishedAt = &now
	status.ExitCode = &exitCode
	status.Error = message
	_ = writeJobStatus(jobID, status)
	if state != jobStateSucceeded {
		_ = writeJSONFile(filepath.Join(mustJobDir(jobID), "result.json"), map[string]any{"job_id": jobID, "state": state, "error": message, "exit_code": exitCode})
	}
}

type listedJob struct {
	Meta   jobMetadata     `json:"meta"`
	Status jobStatusRecord `json:"status"`
}

func listJobs() ([]listedJob, error) {
	root, err := jobsRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read jobs directory: %w", err)
	}
	items := make([]listedJob, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, status, err := readJob(entry.Name())
		if err != nil {
			continue
		}
		items = append(items, listedJob{Meta: meta, Status: status})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Meta.CreatedAt.After(items[j].Meta.CreatedAt) })
	return items, nil
}

func readJob(jobID string) (jobMetadata, jobStatusRecord, error) {
	var meta jobMetadata
	var status jobStatusRecord
	dir, err := jobDir(jobID)
	if err != nil {
		return meta, status, err
	}
	if err := readJSONFile(filepath.Join(dir, "job.json"), &meta); err != nil {
		return meta, status, err
	}
	if err := readJSONFile(filepath.Join(dir, "status.json"), &status); err != nil {
		return meta, status, err
	}
	return meta, status, nil
}

func writeJobStatus(jobID string, status jobStatusRecord) error {
	path, err := jobFile(jobID, "status.json")
	if err != nil {
		return err
	}
	return writeJSONFile(path, status)
}

func validateJobCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("job start requires a command after --")
	}
	if containsRootOnlyFlag(args) {
		return errors.New("--base-url, --timeout, --format, and --api-key must be passed before job start; --api-key is not supported for background jobs")
	}
	if containsBackgroundFlag(args) {
		return errors.New("--bg is not supported inside job start commands")
	}
	if args[0] == "search" {
		if !containsSearchDeepFlag(args[1:]) {
			return errors.New("search background jobs require --deep")
		}
		return nil
	}
	if len(args) >= 2 && args[0] == "image" && (args[1] == "generate" || args[1] == "edit") {
		return nil
	}
	return errors.New("only search and image commands can run as jobs")
}

func commandFlagArgs(cmd *cobra.Command, exclude []string) []string {
	excluded := make(map[string]struct{}, len(exclude))
	for _, name := range exclude {
		excluded[name] = struct{}{}
	}
	args := []string{}
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if _, ok := excluded[flag.Name]; ok {
			return
		}
		if cmd.InheritedFlags().Lookup(flag.Name) != nil {
			return
		}
		if flag.Value.Type() == "bool" {
			if flag.Value.String() == "true" {
				args = append(args, "--"+flag.Name)
			}
			return
		}
		if values, ok := flag.Value.(interface{ GetSlice() []string }); ok {
			for _, v := range values.GetSlice() {
				args = append(args, "--"+flag.Name, v)
			}
			return
		}
		args = append(args, "--"+flag.Name, flag.Value.String())
	})
	return args
}

func containsBackgroundFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--bg" || strings.HasPrefix(arg, "--bg=") {
			return true
		}
	}
	return false
}

func containsSearchDeepFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--deep" || arg == "--deep=true" {
			return true
		}
		if searchFlagConsumesNextArg(arg) {
			i++
		}
	}
	return false
}

func searchFlagConsumesNextArg(arg string) bool {
	if strings.Contains(arg, "=") {
		return false
	}
	switch arg {
	case "--model", "--instructions", "--instructions-file", "--reasoning-effort", "--search-context-size", "--max-tool-calls", "--max-output-tokens":
		return true
	default:
		return false
	}
}

func containsDryRunFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--dry-run" || strings.HasPrefix(arg, "--dry-run=") {
			return true
		}
	}
	return false
}

func stripBackgroundFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--bg" || strings.HasPrefix(arg, "--bg=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func containsAPIKeyFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--api-key" || strings.HasPrefix(arg, "--api-key=") {
			return true
		}
	}
	return false
}

func containsRootOnlyFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--base-url" || strings.HasPrefix(arg, "--base-url=") ||
			arg == "--timeout" || strings.HasPrefix(arg, "--timeout=") ||
			arg == "--format" || strings.HasPrefix(arg, "--format=") ||
			arg == "--api-key" || strings.HasPrefix(arg, "--api-key=") {
			return true
		}
	}
	return false
}

func operationName(args []string, override string) string {
	if override != "" {
		return override
	}
	if len(args) >= 2 && args[0] == "image" {
		return "image." + args[1]
	}
	return args[0]
}

func hasJSONFlag(args []string) bool {
	for i, arg := range args {
		if arg == "--json" || arg == "--format=json" {
			return true
		}
		if arg == "--format" && i+1 < len(args) && args[i+1] == "json" {
			return true
		}
	}
	return false
}

func writeJobStartOutput(cmd *cobra.Command, jsonOut bool, payload jobStartOutput) error {
	if jsonOut {
		return writeJSON(cmd, payload)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), payload.JobID)
	return err
}

func writeHumanJobResult(w io.Writer, data []byte) error {
	var payload struct {
		Output json.RawMessage `json:"output"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		_, writeErr := w.Write(data)
		return writeErr
	}
	if payload.Error != "" {
		_, err := fmt.Fprintln(w, payload.Error)
		return err
	}
	if len(payload.Output) == 0 {
		_, writeErr := w.Write(data)
		return writeErr
	}
	var imageOut imageOutputJSON
	if err := json.Unmarshal(payload.Output, &imageOut); err == nil && len(imageOut.Paths) > 0 {
		for _, p := range imageOut.Paths {
			if _, err := fmt.Fprintln(w, p); err != nil {
				return err
			}
		}
		return nil
	}
	var searchOut struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(payload.Output, &searchOut); err == nil && searchOut.Text != "" {
		_, err := fmt.Fprintln(w, searchOut.Text)
		return err
	}
	_, err := w.Write(payload.Output)
	return err
}

func jobsRoot() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(jobEnvDir)); dir != "" {
		return dir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return filepath.Join(base, "gptx", "jobs"), nil
}

func newJobID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "job_" + time.Now().UTC().Format("20060102T150405") + "_" + hex.EncodeToString(b[:]), nil
}

func jobDir(jobID string) (string, error) {
	if !validJobID(jobID) {
		return "", fmt.Errorf("invalid job id: %s", jobID)
	}
	root, err := jobsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, jobID), nil
}

func mustJobDir(jobID string) string {
	dir, _ := jobDir(jobID)
	return dir
}

func jobFile(jobID string, name string) (string, error) {
	dir, err := jobDir(jobID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func validJobID(id string) bool {
	if id == "" || id == "." || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func writeJSONFile(path string, payload any) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func appendFile(path string, text string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(text)
	return err
}

func cancelRequested(jobID string) bool {
	path, err := jobFile(jobID, "cancel")
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
