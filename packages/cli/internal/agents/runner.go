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

	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	privateRunDirMode   = 0700
	privateArtifactMode = 0600
)

type RunOptions struct {
	Context     context.Context
	Executor    string
	DryRun      bool
	ScheduledAt time.Time
	Stdout      io.Writer
	Stderr      io.Writer
}

type RunRecord struct {
	SchemaVersion string          `json:"schemaVersion"`
	RunID         string          `json:"run_id"`
	JobID         string          `json:"job_id"`
	Status        string          `json:"status"`
	ScheduledAt   time.Time       `json:"scheduled_at"`
	StartedAt     time.Time       `json:"started_at"`
	FinishedAt    time.Time       `json:"finished_at,omitempty,omitzero"`
	Plan          RunPlan         `json:"plan"`
	Agent         CommandResult   `json:"agent,omitempty,omitzero"`
	Preflight     []CommandResult `json:"preflight,omitempty"`
	Verify        []CommandResult `json:"verify,omitempty"`
	Error         string          `json:"error,omitempty"`
	StatusText    string          `json:"status_text,omitempty"`
	PatchPath     string          `json:"patch_path,omitempty"`
	Eval          *EvalResult     `json:"eval,omitempty"`
}

type EvalResult struct {
	Status         string `json:"status"`
	Dataset        string `json:"dataset"`
	Target         string `json:"target"`
	Base           string `json:"base"`
	Gate           string `json:"gate"`
	JSONPath       string `json:"json_path,omitempty"`
	MarkdownPath   string `json:"markdown_path,omitempty"`
	Regressions    int    `json:"regressions"`
	ProposedFailed int    `json:"proposed_failed"`
	Total          int    `json:"total"`
	BasePassed     int    `json:"base_passed"`
	ProposedPassed int    `json:"proposed_passed"`
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

func RunJob(job Job, options RunOptions) (record RunRecord, resultErr error) {
	if options.Context == nil {
		options.Context = context.Background()
	}
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
		return RunRecord{SchemaVersion: plan.SchemaVersion, RunID: plan.RunID, JobID: plan.JobID, Status: "planned", ScheduledAt: options.ScheduledAt, Plan: plan}, nil
	}

	releaseConcurrency, acquired, err := acquireConcurrency(plan)
	if err != nil {
		return RunRecord{}, err
	}
	if !acquired {
		return recordSkippedConcurrency(plan, options.ScheduledAt)
	}
	defer func() {
		if err := releaseConcurrency(); err != nil && resultErr == nil {
			resultErr = fmt.Errorf("release concurrency key %q: %w", plan.Concurrency.Key, err)
		}
	}()

	if _, err := os.Stat(plan.RunDir); err == nil {
		return RunRecord{}, fmt.Errorf("agent run already exists: %s", plan.RunDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return RunRecord{}, err
	}

	record = RunRecord{
		SchemaVersion: plan.SchemaVersion,
		RunID:         plan.RunID,
		JobID:         plan.JobID,
		Status:        "running",
		ScheduledAt:   options.ScheduledAt,
		StartedAt:     time.Now(),
		Plan:          plan,
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
	controller, runContext, err := newRunController(options.Context, plan)
	if err != nil {
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
		if err := writeRunRecord(plan.RunDir, record); err != nil && runErr == nil {
			runErr = err
		}
		if err := controller.Close(status); err != nil && runErr == nil {
			runErr = err
		}
		return record, runErr
	}
	if err := ensureRunnablePlan(plan, job); err != nil {
		return finish("failed", err)
	}

	if err := createWorktree(plan); err != nil {
		return finish("failed", err)
	}
	preflightTimeout, err := time.ParseDuration(plan.PreflightTimeout)
	if err != nil || preflightTimeout <= 0 {
		return finish("failed", fmt.Errorf("invalid preflight timeout %q", plan.PreflightTimeout))
	}
	for index, command := range plan.Preflight {
		preflightCtx, cancel := context.WithTimeout(runContext, preflightTimeout)
		result := runPlanCommand(preflightCtx, plan, command, fmt.Sprintf("preflight-%02d", index+1), "", controller)
		preflightTimedOut := errors.Is(preflightCtx.Err(), context.DeadlineExceeded)
		cancel()
		record.Preflight = append(record.Preflight, result)
		if err := writeRunRecord(plan.RunDir, record); err != nil {
			return finish("failed", err)
		}
		if status, runErr, cancelled := cancelledRunResult(runContext, controller); cancelled {
			return finish(status, runErr)
		}
		if preflightTimedOut {
			return finish("preflight_failed", fmt.Errorf("preflight command %q timed out after %s", command.Command, preflightTimeout))
		}
		if result.ExitCode != 0 {
			return finish("preflight_failed", fmt.Errorf("preflight command %q exited with %d", command.Command, result.ExitCode))
		}
	}

	if plan.Agent.Command != "" {
		agentTimeout := 30 * time.Minute
		if job.Agent.Timeout != "" {
			parsed, err := time.ParseDuration(job.Agent.Timeout)
			if err != nil {
				return finish("failed", err)
			}
			agentTimeout = parsed
		}
		agentCtx, cancel := context.WithTimeout(runContext, agentTimeout)
		record.Agent = runPlanCommand(agentCtx, plan, plan.Agent, "agent", plan.Prompt, controller)
		agentTimedOut := errors.Is(agentCtx.Err(), context.DeadlineExceeded)
		cancel()
		if err := writeRunRecord(plan.RunDir, record); err != nil {
			return finish("failed", err)
		}
		if status, runErr, cancelled := cancelledRunResult(runContext, controller); cancelled {
			return finish(status, runErr)
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
	}

	verifyTimeout, err := time.ParseDuration(plan.VerifyTimeout)
	if err != nil || verifyTimeout <= 0 {
		return finish("failed", fmt.Errorf("invalid verification timeout %q", plan.VerifyTimeout))
	}
	for index, command := range plan.Verify {
		verifyCtx, cancel := context.WithTimeout(runContext, verifyTimeout)
		result := runPlanCommand(verifyCtx, plan, command, fmt.Sprintf("verify-%02d", index+1), "", controller)
		verifyTimedOut := errors.Is(verifyCtx.Err(), context.DeadlineExceeded)
		cancel()
		record.Verify = append(record.Verify, result)
		if err := writeRunRecord(plan.RunDir, record); err != nil {
			return finish("failed", err)
		}
		if status, runErr, cancelled := cancelledRunResult(runContext, controller); cancelled {
			return finish(status, runErr)
		}
		if verifyTimedOut {
			return finish("verification_failed", fmt.Errorf("verification command %q timed out after %s", command.Command, verifyTimeout))
		}
		if result.ExitCode != 0 {
			return finish("verification_failed", fmt.Errorf("verification command %q exited with %d", command.Command, result.ExitCode))
		}
	}
	if plan.Eval != nil {
		evalCtx, cancel := context.WithTimeout(runContext, verifyTimeout)
		evalResult, evalErr := runPlanEval(evalCtx, plan)
		evalTimedOut := errors.Is(evalCtx.Err(), context.DeadlineExceeded)
		cancel()
		record.Eval = &evalResult
		if err := writeRunRecord(plan.RunDir, record); err != nil {
			return finish("failed", err)
		}
		if status, runErr, cancelled := cancelledRunResult(runContext, controller); cancelled {
			return finish(status, runErr)
		}
		if evalTimedOut {
			return finish("verification_failed", fmt.Errorf("knowledge eval timed out after %s", verifyTimeout))
		}
		if evalErr != nil {
			return finish("verification_failed", evalErr)
		}
	}
	if plan.Output.PR && record.Eval != nil {
		if err := persistPublicEvalArtifact(plan, *record.Eval); err != nil {
			return finish("failed", err)
		}
	}

	if plan.Output.Commit {
		if err := commitWorktree(plan); err != nil {
			return finish("failed", err)
		}
	}
	return finish("succeeded", nil)
}

func persistPublicEvalArtifact(plan RunPlan, result EvalResult) error {
	if result.JSONPath == "" || result.MarkdownPath == "" {
		return fmt.Errorf("public eval artifact requires JSON and Markdown reports")
	}
	target := filepath.Join(plan.Worktree, ".openknowledge", "reports", plan.RunID)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for source, name := range map[string]string{result.JSONPath: "eval.json", result.MarkdownPath: "eval.md"} {
		content, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, name), content, 0o644); err != nil {
			return err
		}
	}
	manifest := map[string]any{
		"type": "openknowledge.artifact", "version": 1, "kind": "eval-comparison",
		"runId": plan.RunID, "base": result.Base, "createdAt": time.Now().UTC().Format(time.RFC3339),
		"files": []string{"index.md", "eval.md", "eval.json"},
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "artifact.json"), append(content, '\n'), 0o644); err != nil {
		return err
	}
	index := fmt.Sprintf(`---
type: Open Knowledge Artifact
title: Agent answer comparison
status: stable
---

# Agent answer comparison

- Run: %s
- Base: %s
- Machine contract: [artifact.json](artifact.json)
- Human report: [eval.md](eval.md)
- Machine report: [eval.json](eval.json)
`, plan.RunID, result.Base)
	return os.WriteFile(filepath.Join(target, "index.md"), []byte(index), 0o644)
}

func runPlanEval(ctx context.Context, plan RunPlan) (EvalResult, error) {
	configured := plan.Eval
	result := EvalResult{
		Status: "error", Dataset: configured.Dataset, Target: configured.Target,
		Base: configured.Base, Gate: configured.Gate,
	}
	datasetPath, err := resolveEvalWorktreePath(plan.Worktree, configured.Dataset)
	if err != nil {
		return result, fmt.Errorf("resolve eval dataset: %w", err)
	}
	targetPath, err := resolveEvalWorktreePath(plan.Worktree, configured.Target)
	if err != nil {
		return result, fmt.Errorf("resolve eval target: %w", err)
	}
	loaded, err := knowledgeeval.LoadDataset(datasetPath)
	if err != nil {
		return result, err
	}
	specVersion, supported := okf.ResolveSpecVersion(configured.Spec)
	if !supported {
		return result, fmt.Errorf("unsupported OKF spec version: %s", configured.Spec)
	}
	var report knowledgeeval.ComparisonReport
	if configured.AnswerCommand == "" {
		report, err = knowledgeeval.Compare(targetPath, specVersion, loaded, configured.Base, configured.Gate)
	} else {
		timeout := time.Duration(0)
		if configured.AnswerTimeout != "" {
			timeout, err = time.ParseDuration(configured.AnswerTimeout)
			if err != nil {
				return result, err
			}
		}
		runner := knowledgeeval.AnswerRunner{
			Context: ctx, Command: configured.AnswerCommand, Args: append([]string(nil), configured.AnswerArgs...),
			Directory: plan.Worktree, Environment: hostCommandEnvironment(plan, Command{}), Timeout: timeout,
		}
		if plan.Sandbox.Type == "docker" {
			runner.Command = "docker"
			runner.Args = dockerCommandArgs(plan, Command{Command: configured.AnswerCommand, Args: configured.AnswerArgs}, "")
		}
		report, err = knowledgeeval.CompareWithAnswers(targetPath, specVersion, loaded, configured.Base, configured.Gate, runner)
	}
	if err != nil {
		return result, err
	}
	jsonContent, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return result, err
	}
	jsonPath := filepath.Join(plan.RunDir, "eval-report.json")
	markdownPath := filepath.Join(plan.RunDir, "eval-report.md")
	if err := writePrivateArtifactAtomic(jsonPath, append(jsonContent, '\n')); err != nil {
		return result, err
	}
	if err := writePrivateArtifactAtomic(markdownPath, []byte(knowledgeeval.RenderComparisonMarkdown(report))); err != nil {
		return result, err
	}
	result.Status = report.Summary.Status
	result.JSONPath = jsonPath
	result.MarkdownPath = markdownPath
	result.Regressions = report.Summary.Regressed
	result.ProposedFailed = report.Summary.ProposedFailed
	result.Total = report.Summary.Total
	result.BasePassed = report.Summary.UnchangedPassed + report.Summary.Regressed
	result.ProposedPassed = report.Summary.ProposedPassed
	if report.Summary.Status == "fail" {
		return result, fmt.Errorf("knowledge eval gate failed: %d regressions, %d proposed failures", result.Regressions, result.ProposedFailed)
	}
	return result, nil
}

func resolveEvalWorktreePath(worktree string, relative string) (string, error) {
	root, err := canonicalPath(worktree)
	if err != nil {
		return "", err
	}
	candidate, err := canonicalPath(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	if !pathInside(root, candidate) {
		return "", fmt.Errorf("path must stay inside the job worktree")
	}
	return candidate, nil
}

func recordSkippedConcurrency(plan RunPlan, scheduledAt time.Time) (RunRecord, error) {
	now := time.Now()
	record := RunRecord{
		SchemaVersion: plan.SchemaVersion,
		RunID:         plan.RunID,
		JobID:         plan.JobID,
		Status:        "skipped",
		ScheduledAt:   scheduledAt,
		StartedAt:     now,
		FinishedAt:    now,
		Plan:          plan,
		StatusText:    fmt.Sprintf("concurrency key %q is already running", plan.Concurrency.Key),
	}
	runParent := filepath.Dir(plan.RunDir)
	if err := os.MkdirAll(runParent, privateRunDirMode); err != nil {
		return record, fmt.Errorf("create run parent directory: %w", err)
	}
	if err := os.Chmod(runParent, privateRunDirMode); err != nil {
		return record, fmt.Errorf("secure run parent directory: %w", err)
	}
	if err := os.Mkdir(plan.RunDir, privateRunDirMode); err != nil {
		if errors.Is(err, os.ErrExist) {
			return record, nil
		}
		return record, fmt.Errorf("create skipped run directory: %w", err)
	}
	if err := writeRunInputs(plan); err != nil {
		return record, err
	}
	if err := writeRunRecord(plan.RunDir, record); err != nil {
		return record, err
	}
	return record, nil
}

func ensureRunnablePlan(plan RunPlan, job Job) error {
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
	for _, name := range plan.Agent.Env {
		if _, present := os.LookupEnv(name); !present {
			return fmt.Errorf("agent credential variable %s is not set in the runner environment", name)
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

func runPlanCommand(ctx context.Context, plan RunPlan, command Command, logPrefix string, stdin string, controller *runController) CommandResult {
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

	execCommand := commandForPlan(ctx, plan, command, stdin)
	cancellation := configureCommandCancellation(execCommand)
	defer cancellation.close()
	if command.PromptMode == PromptStdin || command.PromptMode == "" {
		execCommand.Stdin = strings.NewReader(stdin)
	}
	execCommand.Stdout = stdoutFile
	execCommand.Stderr = stderrFile
	err = execCommand.Start()
	if err == nil {
		cancellation.attach(execCommand)
		if controller != nil {
			if controlErr := controller.setCommand(execCommand, logPrefix); controlErr != nil {
				_ = forceCommandCancellation(execCommand)
				waitErr := execCommand.Wait()
				err = errors.Join(controlErr, waitErr)
			} else {
				err = execCommand.Wait()
			}
			_ = controller.clearCommand()
		} else {
			err = execCommand.Wait()
		}
	}
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

func commandForPlan(ctx context.Context, plan RunPlan, command Command, prompt string) *exec.Cmd {
	if plan.Sandbox.Type == "docker" {
		return exec.CommandContext(ctx, "docker", dockerCommandArgs(plan, command, prompt)...)
	}
	if command.Shell {
		cmd := exec.CommandContext(ctx, "sh", "-lc", command.Command)
		cmd.Dir = plan.Worktree
		cmd.Env = hostCommandEnvironment(plan, command)
		return cmd
	}
	arguments := append([]string(nil), command.Args...)
	if command.PromptMode == PromptArgument && prompt != "" {
		arguments = append(arguments, prompt)
	}
	cmd := exec.CommandContext(ctx, command.Command, arguments...)
	cmd.Dir = plan.Worktree
	cmd.Env = hostCommandEnvironment(plan, command)
	return cmd
}

func dockerCommandArgs(plan RunPlan, command Command, prompt string) []string {
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
	for _, name := range command.Env {
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
	args = append(args, command.Args...)
	if command.PromptMode == PromptArgument && prompt != "" {
		args = append(args, prompt)
	}
	return args
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

func hostCommandEnvironment(plan RunPlan, command Command) []string {
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
	for _, name := range command.Env {
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
		message = "Run job " + plan.JobID
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
	return writePrivateArtifactAtomic(filepath.Join(runDir, "run.json"), append(data, '\n'))
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

func writePrivateArtifactAtomic(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), privateRunDirMode); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-tmp-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}
	if err := temporary.Chmod(privateArtifactMode); err != nil {
		cleanup()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	return os.Chmod(path, privateArtifactMode)
}

func cancelledRunResult(runContext context.Context, controller *runController) (string, error, bool) {
	switch controller.Action() {
	case "kill":
		return "killed", errRunKilled, true
	case "stop":
		return "cancelled", errRunStopped, true
	}
	if errors.Is(runContext.Err(), context.Canceled) {
		return "cancelled", context.Cause(runContext), true
	}
	return "", nil, false
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
