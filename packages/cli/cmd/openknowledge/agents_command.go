package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const defaultAgentsJobsPath = ".openknowledge/agents/jobs"

func runAgents(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, agentsHelpText())
		return 0
	}

	switch args[0] {
	case "new":
		return runAgentsNew(args[1:])
	case "list":
		return runAgentsList(args[1:])
	case "validate":
		return runAgentsValidate(args[1:])
	case "run":
		return runAgentsRun(args[1:])
	case "daemon":
		return runAgentsDaemon(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown agents subcommand: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, agentsHelpText())
		return 2
	}
}

func runAgentsNew(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, agentsNewHelpText())
		return 0
	}
	options, err := parseAgentsNewArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.reference {
		fmt.Fprint(os.Stdout, agents.RenderFrontmatterReference())
		return 0
	}
	if options.list || options.template == "" {
		fmt.Fprint(os.Stdout, agents.RenderTemplateCatalog())
		return 0
	}

	template, ok := agents.FindBuiltinTemplate(options.template)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown agent template: %s\n", options.template)
		fmt.Fprint(os.Stderr, "\n")
		fmt.Fprint(os.Stderr, agents.RenderTemplateCatalog())
		return 2
	}
	if options.out == "" {
		fmt.Fprint(os.Stdout, template.Content)
		return 0
	}
	if err := writeAgentTemplate(options.out, template.Content, options.force); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "created agent job: %s\n", options.out)
	fmt.Fprintf(os.Stdout, "validate: openknowledge agents validate %s\n", options.out)
	fmt.Fprintf(os.Stdout, "dry run: openknowledge agents run %s --dry-run\n", options.out)
	return 0
}

func runAgentsList(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, agentsListHelpText())
		return 0
	}
	jsonOutput := false
	var positionals []string
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOutput = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "unknown agents list option: %s\n", arg)
			return 2
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		fmt.Fprintln(os.Stderr, "agents list accepts at most one path")
		return 2
	}
	path := defaultAgentsJobsPath
	if len(positionals) == 1 {
		path = positionals[0]
	}
	jobs, err := agents.DiscoverJobs(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if jsonOutput {
				return printAgentListJSON(path, nil)
			}
			fmt.Fprintf(os.Stdout, "no agent jobs found at %s\n", path)
			return 0
		}
		return printAgentCommandError(err)
	}
	if len(jobs) == 0 {
		if jsonOutput {
			return printAgentListJSON(path, jobs)
		}
		fmt.Fprintf(os.Stdout, "no agent jobs found at %s\n", path)
		return 0
	}
	sort.Slice(jobs, func(first int, second int) bool {
		if jobs[first].ID != jobs[second].ID {
			return jobs[first].ID < jobs[second].ID
		}
		return jobs[first].Path < jobs[second].Path
	})
	if jsonOutput {
		return printAgentListJSON(path, jobs)
	}
	for _, job := range jobs {
		enabled := "disabled"
		if job.Enabled {
			enabled = "enabled"
		}
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", job.ID, enabled, scheduleLabel(job), job.Path)
	}
	return 0
}

type agentListOutput struct {
	SchemaVersion string           `json:"schemaVersion"`
	Path          string           `json:"path"`
	Jobs          []agentListEntry `json:"jobs"`
}

type agentListEntry struct {
	ID          string              `json:"id"`
	Enabled     bool                `json:"enabled"`
	Path        string              `json:"path"`
	Schedule    agents.ScheduleSpec `json:"schedule"`
	Agent       string              `json:"agent"`
	Sandbox     string              `json:"sandbox"`
	Concurrency agents.Concurrency  `json:"concurrency"`
}

func printAgentListJSON(path string, jobs []agents.Job) int {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	entries := make([]agentListEntry, 0, len(jobs))
	for _, job := range jobs {
		entries = append(entries, agentListEntry{
			ID:          job.ID,
			Enabled:     job.Enabled,
			Path:        job.Path,
			Schedule:    job.Schedule,
			Agent:       job.Agent.Command,
			Sandbox:     job.Sandbox.Type,
			Concurrency: job.Concurrency,
		})
	}
	output := agentListOutput{SchemaVersion: okf.MachineSchemaVersion, Path: absolutePath, Jobs: entries}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintln(os.Stdout, string(encoded))
	return 0
}

func runAgentsValidate(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, agentsValidateHelpText())
		return 0
	}
	jsonOutput := false
	var positionals []string
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOutput = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "unknown agents validate option: %s\n", arg)
			return 2
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		fmt.Fprintln(os.Stderr, "agents validate requires exactly one job file or directory")
		return 2
	}
	path := positionals[0]
	jobs, err := agents.DiscoverJobs(path)
	if err != nil {
		if jsonOutput {
			return printAgentValidationJSON(path, nil, err)
		}
		return printAgentCommandError(err)
	}
	if jsonOutput {
		return printAgentValidationJSON(path, jobs, nil)
	}
	if len(jobs) == 0 {
		fmt.Fprintf(os.Stdout, "no agent jobs found at %s\n", path)
		return 0
	}
	for _, job := range jobs {
		fmt.Fprintf(os.Stdout, "valid agent job: %s (%s)\n", job.ID, job.Path)
	}
	return 0
}

type agentValidationOutput struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Path          string                   `json:"path"`
	Valid         bool                     `json:"valid"`
	Jobs          []agentValidationJob     `json:"jobs"`
	Issues        []agents.ValidationIssue `json:"issues"`
	Error         string                   `json:"error,omitempty"`
}

type agentValidationJob struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func printAgentValidationJSON(path string, jobs []agents.Job, validationErr error) int {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	output := agentValidationOutput{
		SchemaVersion: okf.MachineSchemaVersion,
		Path:          absolutePath,
		Valid:         validationErr == nil,
		Jobs:          make([]agentValidationJob, 0, len(jobs)),
		Issues:        make([]agents.ValidationIssue, 0),
	}
	for _, job := range jobs {
		output.Jobs = append(output.Jobs, agentValidationJob{ID: job.ID, Path: job.Path})
	}
	if validationErr != nil {
		var typed agents.ValidationError
		if errors.As(validationErr, &typed) {
			output.Issues = append(output.Issues, typed.Issues...)
		} else {
			output.Error = validationErr.Error()
		}
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintln(os.Stdout, string(encoded))
	if validationErr != nil {
		return 1
	}
	return 0
}

func runAgentsRun(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, agentsRunHelpText())
		return 0
	}
	options, err := parseAgentsRunArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	scheduledAt, err := parseAgentScheduledAt(options.at)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	job, err := agents.ParseJobFile(options.path)
	if err != nil {
		return printAgentCommandError(err)
	}
	record, err := agents.RunJob(job, agents.RunOptions{
		Executor:    options.executor,
		DryRun:      options.dryRun,
		ScheduledAt: scheduledAt,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err != nil {
		if record.RunID != "" {
			fmt.Fprintf(os.Stderr, "agent run %s failed: %v\n", record.RunID, err)
		} else {
			fmt.Fprintf(os.Stderr, "agent run failed: %v\n", err)
		}
		return 1
	}
	if !options.dryRun {
		fmt.Fprintf(os.Stdout, "agent run %s %s\n", record.RunID, record.Status)
		fmt.Fprintf(os.Stdout, "run: %s\n", filepath.Join(record.Plan.RunDir, "run.json"))
		if record.Status == "skipped" {
			fmt.Fprintf(os.Stdout, "reason: %s\n", record.StatusText)
		} else {
			fmt.Fprintf(os.Stdout, "worktree: %s\n", record.Plan.Worktree)
		}
	}
	return 0
}

func runAgentsDaemon(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, agentsDaemonHelpText())
		return 0
	}
	options, err := parseAgentsDaemonArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	interval, err := time.ParseDuration(options.tick)
	if err != nil || interval <= 0 {
		fmt.Fprintln(os.Stderr, "--tick must be a positive Go duration")
		return 2
	}

	for {
		code := runDueAgentJobs(options.path, options.executor, options.dryRun)
		if options.once || code != 0 {
			return code
		}
		time.Sleep(interval)
	}
}

type agentsRunCLIOptions struct {
	path     string
	dryRun   bool
	at       string
	executor string
}

type agentsNewCLIOptions struct {
	template  string
	out       string
	list      bool
	reference bool
	force     bool
}

func parseAgentsNewArgs(args []string) (agentsNewCLIOptions, error) {
	var options agentsNewCLIOptions
	var positionals []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--list":
			options.list = true
		case arg == "--reference":
			options.reference = true
		case arg == "--force":
			options.force = true
		case arg == "--out":
			value, next, err := nextFlagValue(args, index, "--out")
			if err != nil {
				return options, err
			}
			options.out = value
			index = next
		case strings.HasPrefix(arg, "--out="):
			options.out = strings.TrimPrefix(arg, "--out=")
			if strings.TrimSpace(options.out) == "" {
				return options, fmt.Errorf("--out requires a value")
			}
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown agents new option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("agents new accepts at most one template id")
	}
	if len(positionals) == 1 {
		options.template = positionals[0]
	}
	if options.list && options.reference {
		return options, fmt.Errorf("--list cannot be combined with --reference")
	}
	if options.out != "" && options.template == "" {
		return options, fmt.Errorf("--out requires a template id")
	}
	return options, nil
}

func parseAgentsRunArgs(args []string) (agentsRunCLIOptions, error) {
	var options agentsRunCLIOptions
	var positionals []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--dry-run":
			options.dryRun = true
		case arg == "--at":
			value, next, err := nextFlagValue(args, index, "--at")
			if err != nil {
				return options, err
			}
			options.at = value
			index = next
		case strings.HasPrefix(arg, "--at="):
			options.at = strings.TrimPrefix(arg, "--at=")
			if strings.TrimSpace(options.at) == "" {
				return options, fmt.Errorf("--at requires a value")
			}
		case arg == "--executor":
			value, next, err := nextFlagValue(args, index, "--executor")
			if err != nil {
				return options, err
			}
			options.executor = value
			index = next
		case strings.HasPrefix(arg, "--executor="):
			options.executor = strings.TrimPrefix(arg, "--executor=")
			if strings.TrimSpace(options.executor) == "" {
				return options, fmt.Errorf("--executor requires a value")
			}
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown agents run option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return options, fmt.Errorf("agents run requires exactly one job file")
	}
	normalizedExecutor, err := agents.NormalizeExecutorOverride(options.executor)
	if err != nil {
		return options, fmt.Errorf("--executor %w", err)
	}
	options.executor = normalizedExecutor
	options.path = positionals[0]
	return options, nil
}

type agentsDaemonCLIOptions struct {
	path     string
	once     bool
	dryRun   bool
	tick     string
	executor string
}

func parseAgentsDaemonArgs(args []string) (agentsDaemonCLIOptions, error) {
	options := agentsDaemonCLIOptions{path: defaultAgentsJobsPath, tick: "1m"}
	var positionals []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--once":
			options.once = true
		case arg == "--dry-run":
			options.dryRun = true
		case arg == "--tick":
			value, next, err := nextFlagValue(args, index, "--tick")
			if err != nil {
				return options, err
			}
			options.tick = value
			index = next
		case strings.HasPrefix(arg, "--tick="):
			options.tick = strings.TrimPrefix(arg, "--tick=")
			if strings.TrimSpace(options.tick) == "" {
				return options, fmt.Errorf("--tick requires a value")
			}
		case arg == "--executor":
			value, next, err := nextFlagValue(args, index, "--executor")
			if err != nil {
				return options, err
			}
			options.executor = value
			index = next
		case strings.HasPrefix(arg, "--executor="):
			options.executor = strings.TrimPrefix(arg, "--executor=")
			if strings.TrimSpace(options.executor) == "" {
				return options, fmt.Errorf("--executor requires a value")
			}
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown agents daemon option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("agents daemon accepts at most one jobs directory")
	}
	if len(positionals) == 1 {
		options.path = positionals[0]
	}
	normalizedExecutor, err := agents.NormalizeExecutorOverride(options.executor)
	if err != nil {
		return options, fmt.Errorf("--executor %w", err)
	}
	options.executor = normalizedExecutor
	return options, nil
}

func writeAgentTemplate(path string, content string, force bool) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("--out requires a value")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("agent job already exists: %s (use --force to overwrite)", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func runDueAgentJobs(path string, executor string, dryRun bool) int {
	jobs, err := agents.DiscoverJobs(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stdout, "no agent jobs found at %s\n", path)
			return 0
		}
		return printAgentCommandError(err)
	}
	now := time.Now()
	dueCount := 0
	for _, job := range jobs {
		scheduledAt, due, err := agents.DueScheduledAt(job, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", job.ID, err)
			return 1
		}
		if !due {
			continue
		}
		plan, err := agents.BuildRunPlan(job, scheduledAt, executor)
		if err != nil {
			return printAgentCommandError(err)
		}
		if _, err := os.Stat(filepath.Join(plan.RunDir, "run.json")); err == nil {
			continue
		}
		dueCount++
		record, err := agents.RunJob(job, agents.RunOptions{
			Executor:    executor,
			DryRun:      dryRun,
			ScheduledAt: scheduledAt,
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
		})
		if err != nil {
			if record.RunID != "" {
				fmt.Fprintf(os.Stderr, "agent run %s failed: %v\n", record.RunID, err)
			} else {
				fmt.Fprintf(os.Stderr, "agent run failed: %v\n", err)
			}
			return 1
		}
		if !dryRun {
			fmt.Fprintf(os.Stdout, "agent run %s %s\n", record.RunID, record.Status)
		}
	}
	if dueCount == 0 {
		fmt.Fprintln(os.Stdout, "no due agent jobs")
	}
	return 0
}

func parseAgentScheduledAt(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Now(), nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04", value, time.Local); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("--at must be RFC3339, YYYY-MM-DD, or YYYY-MM-DD HH:MM")
}

func printAgentCommandError(err error) int {
	var validation agents.ValidationError
	if errors.As(err, &validation) {
		fmt.Fprintln(os.Stderr, "invalid agent job:")
		for _, issue := range validation.Issues {
			fmt.Fprintf(os.Stderr, "- %s: %s\n", issue.Field, issue.Message)
		}
		return 1
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func scheduleLabel(job agents.Job) string {
	switch {
	case job.Schedule.Cron != "":
		return "cron=" + job.Schedule.Cron
	case job.Schedule.Every != "":
		return "every=" + job.Schedule.Every
	default:
		return "manual"
	}
}

func agentsHelpText() string {
	return `openknowledge agents

Experimental command group for deterministic local agent jobs from Markdown
specs with nested frontmatter. Job schema and scheduler behavior may still
change before this surface is treated as stable.

Usage:
  openknowledge agents new
  openknowledge agents new --list
  openknowledge agents new --reference
  openknowledge agents new <template>
  openknowledge agents new <template> --out <file>
  openknowledge agents list [path]
  openknowledge agents validate <job-or-dir>
  openknowledge agents run <job.md>
  openknowledge agents run <job.md> --dry-run
  openknowledge agents daemon [jobs-dir]
  openknowledge agents daemon [jobs-dir] --once
  openknowledge agents --help

Subcommands:
  new        List, print, or write built-in job templates.
  list       List job specs.
  validate   Parse and schema-check job specs.
  run        Create a Git worktree and run one job now.
  daemon     Poll scheduled jobs and run due jobs.

Default jobs directory:
  .openknowledge/agents/jobs
`
}

func agentsNewHelpText() string {
	return `openknowledge agents new

List, print, or write built-in agent job templates.

Usage:
  openknowledge agents new
  openknowledge agents new --list
  openknowledge agents new --reference
  openknowledge agents new <template>
  openknowledge agents new <template> --out <file>
  openknowledge agents new <template> --out <file> --force
  openknowledge agents new --help

Arguments:
  template     Built-in template id. Use --list to see available ids.

Flags:
  --list       Print available built-in templates.
  --reference  Print the supported nested frontmatter syntax.
  --out        Write the selected template to a file instead of stdout.
  --force      Overwrite --out when the file already exists.

Examples:
  openknowledge agents new docs-audit
  openknowledge agents new docs-audit --out .openknowledge/agents/jobs/docs-audit.md
  openknowledge agents new custom --out .openknowledge/agents/jobs/custom.md
  openknowledge agents new --reference
`
}

func agentsListHelpText() string {
	return `openknowledge agents list

List agent job specs.

Usage:
  openknowledge agents list [path]
  openknowledge agents list [path] --json
  openknowledge agents list --help

Arguments:
  path       Job file or directory. Defaults to .openknowledge/agents/jobs.

Flags:
  --json     Print the schemaVersion 1 agent inventory JSON.
`
}

func agentsValidateHelpText() string {
	return `openknowledge agents validate

Parse and schema-check agent job specs without running an agent.

Usage:
  openknowledge agents validate <job-or-dir>
  openknowledge agents validate <job-or-dir> --json
  openknowledge agents validate --help

Flags:
  --json     Print the schemaVersion 1 validation report, including failures.
`
}

func agentsRunHelpText() string {
	return `openknowledge agents run

Create an isolated Git worktree and run one agent job.

Usage:
  openknowledge agents run <job.md>
  openknowledge agents run <job.md> --dry-run
  openknowledge agents run <job.md> --at <time>
  openknowledge agents run <job.md> --executor host|docker
  openknowledge agents run --help

Flags:
  --dry-run    Print the schemaVersion 1 run plan without creating a worktree.
  --at         Scheduled time used for the deterministic run ID.
               Accepts RFC3339, YYYY-MM-DD, or YYYY-MM-DD HH:MM.
  --executor   Override sandbox.type with host or docker.

Contracts:
  Dry-run JSON and persisted run.json use the published agent-run-plan and
  agent-run-record schemas under https://openknowledge.sh/schemas/cli/v1/.
`
}

func agentsDaemonHelpText() string {
	return `openknowledge agents daemon

Poll scheduled agent jobs and run due jobs.

Usage:
  openknowledge agents daemon [jobs-dir]
  openknowledge agents daemon [jobs-dir] --once
  openknowledge agents daemon [jobs-dir] --tick <duration>
  openknowledge agents daemon [jobs-dir] --dry-run
  openknowledge agents daemon --help

Flags:
  --once       Check due jobs once and exit.
  --tick       Polling interval. Defaults to 1m.
  --dry-run    Print resolved plans for due jobs without executing.
  --executor   Override sandbox.type with host or docker.
`
}
