package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Command struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Shell   bool     `json:"shell,omitempty"`
}

type RunPlan struct {
	RunID       string      `json:"run_id"`
	JobID       string      `json:"job_id"`
	JobFile     string      `json:"job_file"`
	ScheduledAt time.Time   `json:"scheduled_at"`
	Repo        string      `json:"repo"`
	RepoRoot    string      `json:"repo_root"`
	Base        string      `json:"base"`
	BaseSHA     string      `json:"base_sha"`
	Branch      string      `json:"branch"`
	Worktree    string      `json:"worktree"`
	RunDir      string      `json:"run_dir"`
	Prompt      string      `json:"prompt"`
	Agent       Command     `json:"agent"`
	Verify      []Command   `json:"verify,omitempty"`
	Sandbox     SandboxSpec `json:"sandbox"`
	Output      OutputSpec  `json:"output,omitempty"`
}

func BuildRunPlan(job Job, scheduledAt time.Time, executorOverride string) (RunPlan, error) {
	if err := ValidateJob(job); err != nil {
		return RunPlan{}, err
	}
	executorOverride, err := NormalizeExecutorOverride(executorOverride)
	if err != nil {
		return RunPlan{}, ValidationError{Issues: []ValidationIssue{{Field: "executor", Message: err.Error()}}}
	}
	repo := job.Workspace.Repo
	if repo == "" {
		repo = "."
	}
	if !filepath.IsAbs(repo) && job.Path != "" {
		repo = filepath.Join(filepath.Dir(job.Path), repo)
	}
	absoluteRepo, err := filepath.Abs(repo)
	if err != nil {
		return RunPlan{}, err
	}
	repoRoot, err := gitOutput(absoluteRepo, "rev-parse", "--show-toplevel")
	if err != nil {
		return RunPlan{}, fmt.Errorf("resolve git repository: %w", err)
	}
	base := job.Workspace.Base
	if base == "" {
		base = "HEAD"
	}
	baseSHA, err := gitOutput(repoRoot, "rev-parse", base)
	if err != nil {
		return RunPlan{}, fmt.Errorf("resolve workspace base %q: %w", base, err)
	}

	jobHash, err := fileSHA256(job.Path)
	if err != nil {
		return RunPlan{}, err
	}
	runID := stableRunID(job.ID, scheduledAt, jobHash, baseSHA)
	branch := renderTemplate(job.Workspace.Branch, templateValues(job, scheduledAt, runID, ""))
	if branch == "" {
		branch = renderTemplate("agents/{{id}}/{{date}}-{{run_id}}", templateValues(job, scheduledAt, runID, ""))
	}
	branch = sanitizeBranch(branch)
	values := templateValues(job, scheduledAt, runID, branch)
	branch = renderTemplate(branch, values)

	runDir := filepath.Join(repoRoot, ".openknowledge", "agents", "runs", runID)
	worktree := filepath.Join(repoRoot, ".openknowledge", "agents", "worktrees", runID)
	sandbox := job.Sandbox
	if executorOverride != "" {
		sandbox.Type = executorOverride
	}
	if sandbox.Type == "" {
		sandbox.Type = "host"
	}
	if sandbox.Type == "docker" && sandbox.Image == "" {
		return RunPlan{}, ValidationError{Issues: []ValidationIssue{{Field: "sandbox.image", Message: "is required when using the docker executor"}}}
	}

	verify := make([]Command, 0, len(job.Verify.Commands))
	for _, command := range job.Verify.Commands {
		verify = append(verify, Command{Command: command, Shell: true})
	}

	prompt := renderTemplate(job.Prompt, values)
	return RunPlan{
		RunID:       runID,
		JobID:       job.ID,
		JobFile:     job.Path,
		ScheduledAt: scheduledAt,
		Repo:        absoluteRepo,
		RepoRoot:    repoRoot,
		Base:        base,
		BaseSHA:     baseSHA,
		Branch:      branch,
		Worktree:    worktree,
		RunDir:      runDir,
		Prompt:      prompt,
		Agent:       Command{Command: job.Agent.Command, Args: append([]string(nil), job.Agent.Args...)},
		Verify:      verify,
		Sandbox:     sandbox,
		Output:      job.Output,
	}, nil
}

func NormalizeExecutorOverride(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case "", "host", "docker":
		return value, nil
	default:
		return "", fmt.Errorf("must be host or docker")
	}
}

func (plan RunPlan) JSON() ([]byte, error) {
	return json.MarshalIndent(plan, "", "  ")
}

func stableRunID(jobID string, scheduledAt time.Time, jobHash string, baseSHA string) string {
	hash := sha256.Sum256([]byte(strings.Join([]string{
		jobID,
		scheduledAt.UTC().Format(time.RFC3339),
		jobHash,
		baseSHA,
	}, "\n")))
	return hex.EncodeToString(hash[:])[:24]
}

func fileSHA256(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func templateValues(job Job, scheduledAt time.Time, runID string, branch string) map[string]string {
	return map[string]string{
		"id":           job.ID,
		"date":         scheduledAt.Format("20060102-150405"),
		"scheduled_at": scheduledAt.Format(time.RFC3339),
		"run_id":       runID,
		"branch":       branch,
	}
}

func renderTemplate(input string, values map[string]string) string {
	output := input
	for key, value := range values {
		output = strings.ReplaceAll(output, "{{"+key+"}}", value)
	}
	return output
}

func sanitizeBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.ReplaceAll(branch, " ", "-")
	branch = strings.ReplaceAll(branch, "\\", "/")
	parts := strings.Split(branch, "/")
	for index, part := range parts {
		part = strings.Trim(part, ".")
		part = strings.TrimSpace(part)
		if part == "" {
			part = "run"
		}
		parts[index] = part
	}
	return strings.Join(parts, "/")
}
