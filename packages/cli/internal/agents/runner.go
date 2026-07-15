package agents

import (
	"context"
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
)

const (
	privateRunDirMode   = 0700
	privateArtifactMode = 0600
)

type RunOptions struct {
	Executor    string
	DryRun      bool
	ScheduledAt time.Time
	Stdout      io.Writer
	Stderr      io.Writer
}

type RunRecord struct {
	RunID       string          `json:"run_id"`
	JobID       string          `json:"job_id"`
	Status      string          `json:"status"`
	ScheduledAt time.Time       `json:"scheduled_at"`
	StartedAt   time.Time       `json:"started_at"`
	FinishedAt  time.Time       `json:"finished_at,omitempty"`
	Plan        RunPlan         `json:"plan"`
	Agent       CommandResult   `json:"agent,omitempty"`
	Verify      []CommandResult `json:"verify,omitempty"`
	Error       string          `json:"error,omitempty"`
	StatusText  string          `json:"status_text,omitempty"`
	PatchPath   string          `json:"patch_path,omitempty"`
}

type CommandResult struct {
	Command    string        `json:"command"`
	Args       []string      `json:"args,omitempty"`
	Shell      bool          `json:"shell,omitempty"`
	ExitCode   int           `json:"exit_code"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Duration   time.Duration `json:"duration"`
	StdoutLog  string        `json:"stdout_log,omitempty"`
	StderrLog  string        `json:"stderr_log,omitempty"`
	Error      string        `json:"error,omitempty"`
}

func RunJob(job Job, options RunOptions) (RunRecord, error) {
	if options.ScheduledAt.IsZero() {
		options.ScheduledAt = time.Now()
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = io.Discard
	}

	plan, err := BuildRunPlan(job, options.ScheduledAt, options.Executor)
	if err != nil {
		return RunRecord{}, err
	}
	if options.DryRun {
		data, err := plan.JSON()
		if err != nil {
			return RunRecord{}, err
		}
		fmt.Fprintln(options.Stdout, string(data))
		return RunRecord{RunID: plan.RunID, JobID: plan.JobID, Status: "planned", ScheduledAt: options.ScheduledAt, Plan: plan}, nil
	}

	if err := ensureRunnablePlan(plan, job); err != nil {
		return RunRecord{}, err
	}

	record := RunRecord{
		RunID:       plan.RunID,
		JobID:       plan.JobID,
		Status:      "running",
		ScheduledAt: options.ScheduledAt,
		StartedAt:   time.Now(),
		Plan:        plan,
	}
	runParent := filepath.Dir(plan.RunDir)
	if err := os.MkdirAll(runParent, privateRunDirMode); err != nil {
		return RunRecord{}, fmt.Errorf("create run parent directory: %w", err)
	}
	if err := os.Chmod(runParent, privateRunDirMode); err != nil {
		return RunRecord{}, fmt.Errorf("secure run parent directory: %w", err)
	}
	if err := os.Mkdir(plan.RunDir, privateRunDirMode); err != nil {
		return RunRecord{}, fmt.Errorf("create run directory: %w", err)
	}
	if err := writeRunInputs(plan); err != nil {
		return record, err
	}
	if err := writeRunRecord(plan.RunDir, record); err != nil {
		return record, err
	}

	finish := func(status string, runErr error) (RunRecord, error) {
		record.Status = status
		record.FinishedAt = time.Now()
		if runErr != nil {
			record.Error = runErr.Error()
		}
		record.StatusText = worktreeStatus(plan.Worktree)
		record.PatchPath = filepath.Join(plan.RunDir, "diff.patch")
		_ = writePatch(plan, record.PatchPath)
		_ = writeRunRecord(plan.RunDir, record)
		return record, runErr
	}

	if err := createWorktree(plan); err != nil {
		return finish("failed", err)
	}

	agentTimeout := 30 * time.Minute
	if job.Agent.Timeout != "" {
		parsed, err := time.ParseDuration(job.Agent.Timeout)
		if err != nil {
			return finish("failed", err)
		}
		agentTimeout = parsed
	}
	agentCtx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	record.Agent = runPlanCommand(agentCtx, plan, plan.Agent, "agent", plan.Prompt)
	agentTimedOut := errors.Is(agentCtx.Err(), context.DeadlineExceeded)
	cancel()
	if err := writeRunRecord(plan.RunDir, record); err != nil {
		return record, err
	}
	if agentTimedOut {
		return finish("failed", fmt.Errorf("agent command timed out after %s", agentTimeout))
	}
	if record.Agent.ExitCode != 0 {
		return finish("failed", fmt.Errorf("agent command exited with %d", record.Agent.ExitCode))
	}
	if signal := job.Agent.CompletionSignal; signal != "" && !logsContain(record.Agent, signal) {
		return finish("failed", fmt.Errorf("agent output did not contain completion signal %q", signal))
	}

	verifyTimeout, err := time.ParseDuration(plan.VerifyTimeout)
	if err != nil || verifyTimeout <= 0 {
		return finish("failed", fmt.Errorf("invalid verification timeout %q", plan.VerifyTimeout))
	}
	for index, command := range plan.Verify {
		verifyCtx, cancel := context.WithTimeout(context.Background(), verifyTimeout)
		result := runPlanCommand(verifyCtx, plan, command, fmt.Sprintf("verify-%02d", index+1), "")
		verifyTimedOut := errors.Is(verifyCtx.Err(), context.DeadlineExceeded)
		cancel()
		record.Verify = append(record.Verify, result)
		if err := writeRunRecord(plan.RunDir, record); err != nil {
			return record, err
		}
		if verifyTimedOut {
			return finish("verification_failed", fmt.Errorf("verification command %q timed out after %s", command.Command, verifyTimeout))
		}
		if result.ExitCode != 0 {
			return finish("verification_failed", fmt.Errorf("verification command %q exited with %d", command.Command, result.ExitCode))
		}
	}

	if plan.Output.Commit {
		if err := commitWorktree(plan); err != nil {
			return finish("failed", err)
		}
	}
	return finish("succeeded", nil)
}

func ensureRunnablePlan(plan RunPlan, job Job) error {
	if _, err := os.Stat(plan.RunDir); err == nil {
		return fmt.Errorf("agent run already exists: %s", plan.RunDir)
	}
	if job.Workspace.DirtyPolicy != "allow" {
		status, err := gitOutput(plan.RepoRoot, "status", "--porcelain")
		if err != nil {
			return err
		}
		if strings.TrimSpace(status) != "" {
			return fmt.Errorf("repository has uncommitted changes; set workspace.dirty_policy: allow to run anyway")
		}
	}
	for _, name := range plan.Sandbox.Env {
		if _, present := os.LookupEnv(name); !present {
			return fmt.Errorf("sandbox.env variable %s is not set in the runner environment", name)
		}
	}
	return nil
}

func writeRunInputs(plan RunPlan) error {
	if err := copyFile(plan.JobFile, filepath.Join(plan.RunDir, "job.md")); err != nil {
		return err
	}
	if err := writePrivateArtifact(filepath.Join(plan.RunDir, "prompt.md"), []byte(plan.Prompt)); err != nil {
		return err
	}
	data, err := plan.JSON()
	if err != nil {
		return err
	}
	return writePrivateArtifact(filepath.Join(plan.RunDir, "plan.json"), append(data, '\n'))
}

func createWorktree(plan RunPlan) error {
	parent := filepath.Dir(plan.Worktree)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	cmd := exec.Command("git", "worktree", "add", "-b", plan.Branch, plan.Worktree, plan.Base)
	cmd.Dir = plan.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create git worktree: %w\n%s", err, string(output))
	}
	return nil
}

func runPlanCommand(ctx context.Context, plan RunPlan, command Command, logPrefix string, stdin string) CommandResult {
	stdoutLog := filepath.Join(plan.RunDir, logPrefix+".stdout.log")
	stderrLog := filepath.Join(plan.RunDir, logPrefix+".stderr.log")
	started := time.Now()
	result := CommandResult{
		Command:   command.Command,
		Args:      append([]string(nil), command.Args...),
		Shell:     command.Shell,
		ExitCode:  -1,
		StartedAt: started,
		StdoutLog: stdoutLog,
		StderrLog: stderrLog,
	}
	if plan.Sandbox.Type == "host" {
		if err := ensurePrivateHostRuntime(plan); err != nil {
			result.Error = err.Error()
			result.FinishedAt = time.Now()
			result.Duration = result.FinishedAt.Sub(started)
			return result
		}
	}

	stdoutFile, err := createPrivateArtifact(stdoutLog)
	if err != nil {
		result.Error = err.Error()
		result.FinishedAt = time.Now()
		result.Duration = result.FinishedAt.Sub(started)
		return result
	}
	defer stdoutFile.Close()
	stderrFile, err := createPrivateArtifact(stderrLog)
	if err != nil {
		result.Error = err.Error()
		result.FinishedAt = time.Now()
		result.Duration = result.FinishedAt.Sub(started)
		return result
	}
	defer stderrFile.Close()

	execCommand := commandForPlan(ctx, plan, command)
	configureCommandCancellation(execCommand)
	execCommand.Stdin = strings.NewReader(stdin)
	execCommand.Stdout = stdoutFile
	execCommand.Stderr = stderrFile
	err = execCommand.Run()
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(started)
	if execCommand.ProcessState != nil {
		result.ExitCode = execCommand.ProcessState.ExitCode()
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func commandForPlan(ctx context.Context, plan RunPlan, command Command) *exec.Cmd {
	if plan.Sandbox.Type == "docker" {
		return exec.CommandContext(ctx, "docker", dockerCommandArgs(plan, command)...)
	}
	if command.Shell {
		cmd := exec.CommandContext(ctx, "sh", "-lc", command.Command)
		cmd.Dir = plan.Worktree
		cmd.Env = hostCommandEnvironment(plan)
		return cmd
	}
	cmd := exec.CommandContext(ctx, command.Command, command.Args...)
	cmd.Dir = plan.Worktree
	cmd.Env = hostCommandEnvironment(plan)
	return cmd
}

func dockerCommandArgs(plan RunPlan, command Command) []string {
	network := plan.Sandbox.Network
	if network == "" {
		network = "none"
	}
	args := []string{
		"run", "--rm", "-i", "--init",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", "512",
		"--network", network,
	}
	for _, name := range plan.Sandbox.Env {
		args = append(args, "--env", name)
	}
	args = append(args,
		"-v", plan.Worktree+":/workspace",
		"-w", "/workspace",
		"--", plan.Sandbox.Image,
	)
	if command.Shell {
		return append(args, "sh", "-lc", command.Command)
	}
	args = append(args, command.Command)
	return append(args, command.Args...)
}

func ensurePrivateHostRuntime(plan RunPlan) error {
	for _, path := range []string{hostHomePath(plan), hostTempPath(plan)} {
		if err := os.MkdirAll(path, privateRunDirMode); err != nil {
			return err
		}
		if err := os.Chmod(path, privateRunDirMode); err != nil {
			return err
		}
	}
	return nil
}

func hostCommandEnvironment(plan RunPlan) []string {
	environment := make(map[string]string)
	for _, name := range []string{
		"PATH", "LANG", "LC_ALL", "LC_CTYPE", "TERM", "COLORTERM",
		"NO_COLOR", "FORCE_COLOR", "SystemRoot", "WINDIR", "ComSpec", "PATHEXT",
	} {
		if value, present := os.LookupEnv(name); present {
			environment[name] = value
		}
	}
	home := hostHomePath(plan)
	temp := hostTempPath(plan)
	for _, name := range []string{"HOME", "USERPROFILE"} {
		environment[name] = home
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		environment[name] = temp
	}
	for _, name := range plan.Sandbox.Env {
		if value, present := os.LookupEnv(name); present {
			environment[name] = value
		}
	}

	names := make([]string, 0, len(environment))
	for name := range environment {
		names = append(names, name)
	}
	sort.Strings(names)
	values := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, name+"="+environment[name])
	}
	return values
}

func hostHomePath(plan RunPlan) string {
	return filepath.Join(plan.RunDir, "home")
}

func hostTempPath(plan RunPlan) string {
	return filepath.Join(plan.RunDir, "tmp")
}

func commitWorktree(plan RunPlan) error {
	status, err := gitOutput(plan.Worktree, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	if _, err := gitOutput(plan.Worktree, "add", "-A"); err != nil {
		return err
	}
	message := plan.Output.CommitMessage
	if strings.TrimSpace(message) == "" {
		message = "Run agent job " + plan.JobID
	}
	if _, err := gitOutput(plan.Worktree, "commit", "-m", message); err != nil {
		return err
	}
	return nil
}

func worktreeStatus(worktree string) string {
	status, err := gitOutput(worktree, "status", "--short")
	if err != nil {
		return ""
	}
	return status
}

func writePatch(plan RunPlan, path string) error {
	add := exec.Command("git", "add", "-N", ".")
	add.Dir = plan.Worktree
	_ = add.Run()
	cmd := exec.Command("git", "diff", "--binary", plan.BaseSHA)
	cmd.Dir = plan.Worktree
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	return writePrivateArtifact(path, output)
}

func writeRunRecord(runDir string, record RunRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateArtifact(filepath.Join(runDir, "run.json"), append(data, '\n'))
}

func copyFile(source string, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return writePrivateArtifact(target, content)
}

func writePrivateArtifact(path string, content []byte) error {
	file, err := createPrivateArtifact(path)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func createPrivateArtifact(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, privateArtifactMode)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(privateArtifactMode); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func logsContain(result CommandResult, needle string) bool {
	for _, path := range []string{result.StdoutLog, result.StderrLog} {
		content, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(content), needle) {
			return true
		}
	}
	return false
}
