package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type Job struct {
	Path        string        `json:"path"`
	ID          string        `json:"id"`
	Enabled     bool          `json:"enabled"`
	Schedule    ScheduleSpec  `json:"schedule,omitempty"`
	Agent       AgentSpec     `json:"agent"`
	Workspace   WorkspaceSpec `json:"workspace"`
	Sandbox     SandboxSpec   `json:"sandbox"`
	Verify      VerifySpec    `json:"verify,omitempty"`
	Output      OutputSpec    `json:"output,omitempty"`
	Concurrency Concurrency   `json:"concurrency,omitempty"`
	Prompt      string        `json:"prompt"`
}

type ScheduleSpec struct {
	Cron     string `json:"cron,omitempty"`
	Every    string `json:"every,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type AgentSpec struct {
	Command          string   `json:"command"`
	Args             []string `json:"args,omitempty"`
	Timeout          string   `json:"timeout,omitempty"`
	CompletionSignal string   `json:"completion_signal,omitempty"`
}

type WorkspaceSpec struct {
	Repo        string `json:"repo,omitempty"`
	Base        string `json:"base,omitempty"`
	Strategy    string `json:"strategy,omitempty"`
	Branch      string `json:"branch,omitempty"`
	DirtyPolicy string `json:"dirty_policy,omitempty"`
}

type SandboxSpec struct {
	Type    string   `json:"type,omitempty"`
	Image   string   `json:"image,omitempty"`
	Network string   `json:"network,omitempty"`
	Env     []string `json:"env,omitempty"`
}

type VerifySpec struct {
	Commands []string `json:"commands,omitempty"`
	Timeout  string   `json:"timeout,omitempty"`
}

type OutputSpec struct {
	Commit        bool   `json:"commit,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	PR            bool   `json:"pr,omitempty"`
}

type Concurrency struct {
	Key    string `json:"key,omitempty"`
	Policy string `json:"policy,omitempty"`
}

type ValidationIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Issues []ValidationIssue
}

type JobLoadError struct {
	Path string
	Err  error
}

func (err JobLoadError) Error() string {
	if err.Path == "" {
		return err.Err.Error()
	}
	return fmt.Sprintf("%s: %v", err.Path, err.Err)
}

func (err JobLoadError) Unwrap() error {
	return err.Err
}

func (err ValidationError) Error() string {
	if len(err.Issues) == 0 {
		return "agent job is invalid"
	}
	return fmt.Sprintf("agent job is invalid: %s: %s", err.Issues[0].Field, err.Issues[0].Message)
}

func ParseJobFile(path string) (Job, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Job{}, err
	}
	document, err := okf.ParseFrontmatterDocument(content)
	if err != nil {
		return Job{}, err
	}
	for _, warning := range document.Warnings {
		if strings.Contains(warning.Message, "is repeated; later value wins") {
			return Job{}, ValidationError{Issues: []ValidationIssue{{
				Field:   "frontmatter",
				Message: fmt.Sprintf("line %d: %s", warning.Line, warning.Message),
			}}}
		}
	}
	job, err := jobFromFrontmatter(document.Data)
	if err != nil {
		return Job{}, err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Job{}, err
	}
	job.Path = absolute
	job.Prompt = document.Body
	if err := ValidateJob(job); err != nil {
		return job, err
	}
	return job, nil
}

func DiscoverJobs(path string) ([]Job, error) {
	jobs, failures, err := DiscoverJobsLenient(path)
	if err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		return nil, failures[0].Err
	}
	return jobs, nil
}

func DiscoverJobsLenient(path string) ([]Job, []JobLoadError, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		job, err := ParseJobFile(path)
		if err != nil {
			return nil, []JobLoadError{{Path: path, Err: err}}, nil
		}
		return []Job{job}, nil, nil
	}

	var jobs []Job
	var failures []JobLoadError
	err = filepath.WalkDir(path, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			failures = append(failures, JobLoadError{Path: current, Err: err})
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "node_modules" || name == ".openknowledge" {
				return filepath.SkipDir
			}
			return nil
		}
		extension := strings.ToLower(filepath.Ext(current))
		if extension != ".md" && extension != ".markdown" {
			return nil
		}
		job, err := ParseJobFile(current)
		if err != nil {
			failures = append(failures, JobLoadError{Path: current, Err: err})
			return nil
		}
		jobs = append(jobs, job)
		return nil
	})
	if err != nil {
		return nil, failures, err
	}
	return jobs, failures, nil
}

func ValidateJob(job Job) error {
	var issues []ValidationIssue
	add := func(field string, format string, args ...any) {
		issues = append(issues, ValidationIssue{Field: field, Message: fmt.Sprintf(format, args...)})
	}

	if strings.TrimSpace(job.ID) == "" {
		add("id", "is required")
	} else if !validJobID.MatchString(job.ID) {
		add("id", "must contain only letters, numbers, dots, underscores, or hyphens")
	}
	if strings.TrimSpace(job.Agent.Command) == "" {
		add("agent.command", "is required")
	}
	if job.Agent.Timeout != "" {
		if duration, err := time.ParseDuration(job.Agent.Timeout); err != nil {
			add("agent.timeout", "must be a Go duration such as 45m or 1h")
		} else if duration <= 0 {
			add("agent.timeout", "must be positive")
		}
	}
	if job.Schedule.Every != "" {
		if duration, err := time.ParseDuration(job.Schedule.Every); err != nil {
			add("schedule.every", "must be a Go duration such as 1h or 24h")
		} else if duration <= 0 {
			add("schedule.every", "must be positive")
		}
	}
	if job.Schedule.Cron != "" {
		if err := validateCronExpression(job.Schedule.Cron); err != nil {
			add("schedule.cron", "%s", err.Error())
		}
	}
	if job.Schedule.Cron != "" && job.Schedule.Every != "" {
		add("schedule", "cron and every are mutually exclusive")
	}
	if job.Schedule.Timezone != "" && job.Schedule.Cron == "" && job.Schedule.Every == "" {
		add("schedule.timezone", "requires schedule.cron or schedule.every")
	}
	if job.Schedule.Timezone != "" {
		if _, err := time.LoadLocation(job.Schedule.Timezone); err != nil {
			add("schedule.timezone", "is not a valid IANA time zone")
		}
	}
	if job.Verify.Timeout != "" {
		if duration, err := time.ParseDuration(job.Verify.Timeout); err != nil {
			add("verify.timeout", "must be a Go duration such as 10m or 30m")
		} else if duration <= 0 {
			add("verify.timeout", "must be positive")
		}
	}
	switch job.Workspace.Strategy {
	case "", "branch":
	default:
		add("workspace.strategy", "only branch is currently supported")
	}
	switch job.Workspace.DirtyPolicy {
	case "", "fail", "allow":
	default:
		add("workspace.dirty_policy", "must be fail or allow")
	}
	switch job.Sandbox.Type {
	case "", "host", "docker":
	default:
		add("sandbox.type", "must be host or docker")
	}
	if job.Sandbox.Type == "docker" && strings.TrimSpace(job.Sandbox.Image) == "" {
		add("sandbox.image", "is required when sandbox.type is docker")
	}
	if image := job.Sandbox.Image; image != "" {
		if image != strings.TrimSpace(image) || strings.HasPrefix(image, "-") || strings.ContainsAny(image, " \t\r\n\x00") {
			add("sandbox.image", "must be one Docker image reference without whitespace or a leading hyphen")
		}
	}
	switch job.Sandbox.Network {
	case "", "none", "bridge":
	default:
		add("sandbox.network", "must be none or bridge")
	}
	seenEnvironment := make(map[string]bool)
	for _, name := range job.Sandbox.Env {
		if !validEnvironmentName.MatchString(name) {
			add("sandbox.env", "%q is not a valid environment variable name", name)
			continue
		}
		canonicalName := strings.ToUpper(name)
		if managedAgentEnvironment[canonicalName] {
			add("sandbox.env", "%s is managed by the runner and cannot be inherited", name)
		}
		if seenEnvironment[canonicalName] {
			add("sandbox.env", "%s is listed more than once", name)
		}
		seenEnvironment[canonicalName] = true
	}
	if job.Output.PR {
		add("output.pr", "is reserved for a future server/GitHub integration")
	}
	if job.Concurrency.Key != "" {
		if !validJobID.MatchString(job.Concurrency.Key) || len(job.Concurrency.Key) > 128 {
			add("concurrency.key", "must be at most 128 letters, numbers, dots, underscores, or hyphens")
		}
	} else if job.Concurrency.Policy != "" {
		add("concurrency.key", "is required when concurrency.policy is set")
	}
	switch job.Concurrency.Policy {
	case "", "skip":
	default:
		add("concurrency.policy", "must be skip")
	}
	if len(issues) > 0 {
		return ValidationError{Issues: issues}
	}
	return nil
}

func jobFromFrontmatter(data map[string]any) (Job, error) {
	if len(data) == 0 {
		return Job{}, ValidationError{Issues: []ValidationIssue{{Field: "frontmatter", Message: "agent job frontmatter is required"}}}
	}
	if err := validateJobFrontmatterShape(data); err != nil {
		return Job{}, err
	}
	job := Job{
		Enabled: true,
		Workspace: WorkspaceSpec{
			Repo:        ".",
			Base:        "HEAD",
			Strategy:    "branch",
			Branch:      "agents/{{id}}/{{date}}-{{run_id}}",
			DirtyPolicy: "fail",
		},
		Sandbox: SandboxSpec{Type: "host"},
	}
	job.ID = getString(data, "id")
	if enabled, ok := getBool(data, "enabled"); ok {
		job.Enabled = enabled
	}

	if schedule := getMap(data, "schedule"); schedule != nil {
		job.Schedule = ScheduleSpec{
			Cron:     getString(schedule, "cron"),
			Every:    getString(schedule, "every"),
			Timezone: getString(schedule, "timezone"),
		}
	}
	if agent := getMap(data, "agent"); agent != nil {
		job.Agent = AgentSpec{
			Command:          getString(agent, "command"),
			Args:             getStringSlice(agent, "args"),
			Timeout:          getString(agent, "timeout"),
			CompletionSignal: getString(agent, "completion_signal"),
		}
	}
	if workspace := getMap(data, "workspace"); workspace != nil {
		if value := getString(workspace, "repo"); value != "" {
			job.Workspace.Repo = value
		}
		if value := getString(workspace, "base"); value != "" {
			job.Workspace.Base = value
		}
		if value := getString(workspace, "strategy"); value != "" {
			job.Workspace.Strategy = value
		}
		if value := getString(workspace, "branch"); value != "" {
			job.Workspace.Branch = value
		}
		if value := getString(workspace, "dirty_policy"); value != "" {
			job.Workspace.DirtyPolicy = value
		}
	}
	if sandbox := getMap(data, "sandbox"); sandbox != nil {
		if value := getString(sandbox, "type"); value != "" {
			job.Sandbox.Type = value
		}
		job.Sandbox.Image = getString(sandbox, "image")
		job.Sandbox.Network = getString(sandbox, "network")
		job.Sandbox.Env = getStringSlice(sandbox, "env")
	}
	if verify := getMap(data, "verify"); verify != nil {
		job.Verify.Commands = getStringSlice(verify, "commands")
		job.Verify.Timeout = getString(verify, "timeout")
	}
	if output := getMap(data, "output"); output != nil {
		if commit, ok := getBool(output, "commit"); ok {
			job.Output.Commit = commit
		}
		job.Output.CommitMessage = getString(output, "commit_message")
		if pr, ok := getBool(output, "pr"); ok {
			job.Output.PR = pr
		}
	}
	if concurrency := getMap(data, "concurrency"); concurrency != nil {
		job.Concurrency = Concurrency{
			Key:    getString(concurrency, "key"),
			Policy: getString(concurrency, "policy"),
		}
		if job.Concurrency.Key != "" && job.Concurrency.Policy == "" {
			job.Concurrency.Policy = "skip"
		}
	}
	return job, nil
}

func getMap(data map[string]any, key string) map[string]any {
	value, ok := data[key]
	if !ok {
		return nil
	}
	typed, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return typed
}

func getString(data map[string]any, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case int:
		return fmt.Sprint(typed)
	case bool:
		return fmt.Sprint(typed)
	default:
		return ""
	}
}

func getBool(data map[string]any, key string) (bool, bool) {
	value, ok := data[key]
	if !ok || value == nil {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "yes", "on":
			return true, true
		case "false", "no", "off":
			return false, true
		}
	}
	return false, false
}

func getStringSlice(data map[string]any, key string) []string {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			switch scalar := item.(type) {
			case string:
				values = append(values, scalar)
			case int:
				values = append(values, fmt.Sprint(scalar))
			case bool:
				values = append(values, fmt.Sprint(scalar))
			}
		}
		return values
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

var validJobID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var validRunID = regexp.MustCompile(`^[a-f0-9]{24}$`)
var validEnvironmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var managedAgentEnvironment = map[string]bool{
	"HOME": true, "TMPDIR": true, "TMP": true, "TEMP": true,
	"USERPROFILE": true, "HOMEDRIVE": true, "HOMEPATH": true,
}
