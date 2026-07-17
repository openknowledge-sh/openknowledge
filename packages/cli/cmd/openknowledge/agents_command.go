package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const defaultJobsPath = ".openknowledge/jobs"

var startDetachedJobProcess = agents.StartDetachedProcess

func runJobs(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, jobsHelpText())
		return 0
	}

	switch args[0] {
	case "new":
		return runJobsNew(args[1:])
	case "list":
		return runJobsList(args[1:])
	case "status":
		return runJobsStatus(args[1:])
	case "runs":
		return runJobsRuns(args[1:])
	case "start":
		return runJobsStart(args[1:])
	case "stop":
		return runJobsControl(args[1:], "stop")
	case "kill":
		return runJobsControl(args[1:], "kill")
	case "validate":
		return runJobsValidate(args[1:])
	case "run":
		return runJobsRun(args[1:])
	case "daemon":
		return runJobsDaemon(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown jobs subcommand: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, jobsHelpText())
		return 2
	}
}

type agentStatusOutput struct {
	SchemaVersion string             `json:"schemaVersion"`
	Path          string             `json:"path"`
	GeneratedAt   time.Time          `json:"generated_at"`
	Jobs          []agents.JobStatus `json:"jobs"`
	Issues        []agents.RunIssue  `json:"issues"`
}

func runJobsStatus(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsStatusHelpText())
		return 0
	}
	path, jsonOutput, err := parseJobsInventoryArgs(args, defaultJobsPath, "status")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	jobs, loadFailures, err := agents.DiscoverJobsLenient(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return printAgentCommandError(err)
	}
	if errors.Is(err, os.ErrNotExist) {
		jobs = []agents.Job{}
		loadFailures = nil
	}
	statuses, issues := agents.BuildJobStatuses(jobs, time.Now())
	for _, failure := range loadFailures {
		issues = append(issues, agents.RunIssue{Path: failure.Path, Error: failure.Err.Error()})
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return printAgentCommandError(err)
	}
	output := agentStatusOutput{
		SchemaVersion: okf.MachineSchemaVersion,
		Path:          absPath,
		GeneratedAt:   time.Now(),
		Jobs:          statuses,
		Issues:        issues,
	}
	if jsonOutput {
		if err := printJSON(output); err != nil {
			return printAgentCommandError(err)
		}
	} else if len(statuses) == 0 {
		fmt.Fprintf(os.Stdout, "no jobs found at %s\n", path)
	} else {
		fmt.Fprintln(os.Stdout, "JOB\tENABLED\tSCHEDULE\tNEXT_ELIGIBLE\tLAST_RUN\tLAST_STATUS\tACTIVE_RUNS")
		for _, status := range statuses {
			next := "-"
			if status.NextEligibleAt != nil {
				next = status.NextEligibleAt.Format(time.RFC3339)
			}
			lastRun := "-"
			lastStatus := "-"
			if status.LastRun != nil {
				lastRun = status.LastRun.RunID
				lastStatus = status.LastRun.Status
			}
			active := make([]string, 0, len(status.ActiveRuns))
			for _, run := range status.ActiveRuns {
				active = append(active, run.RunID)
			}
			activeRuns := "-"
			if len(active) > 0 {
				activeRuns = strings.Join(active, ",")
			}
			fmt.Fprintf(os.Stdout, "%s\t%t\t%s\t%s\t%s\t%s\t%s\n",
				status.ID, status.Enabled, statusScheduleLabel(status.Schedule), next,
				lastRun, lastStatus, activeRuns)
		}
	}
	for _, issue := range issues {
		fmt.Fprintf(os.Stderr, "job status issue at %s: %s\n", issue.Path, issue.Error)
	}
	if len(issues) > 0 {
		return 1
	}
	return 0
}

type agentRunsOutput struct {
	SchemaVersion string              `json:"schemaVersion"`
	RepoRoot      string              `json:"repo_root"`
	Runs          []agents.RunSummary `json:"runs"`
	Issues        []agents.RunIssue   `json:"issues"`
}

func runJobsRuns(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsRunsHelpText())
		return 0
	}
	options, err := parseJobsRunsArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	runs, issues, repoRoot, err := agents.ListRuns(options.repo)
	if err != nil {
		return printAgentCommandError(err)
	}
	filtered := make([]agents.RunSummary, 0, len(runs))
	for _, run := range runs {
		if options.job != "" && run.JobID != options.job {
			continue
		}
		if options.status != "" && run.Status != options.status {
			continue
		}
		filtered = append(filtered, run)
	}
	output := agentRunsOutput{SchemaVersion: okf.MachineSchemaVersion, RepoRoot: repoRoot, Runs: filtered, Issues: issues}
	if options.json {
		if err := printJSON(output); err != nil {
			return printAgentCommandError(err)
		}
	} else if len(filtered) == 0 {
		fmt.Fprintf(os.Stdout, "no job runs found for %s\n", repoRoot)
	} else {
		fmt.Fprintln(os.Stdout, "RUN\tJOB\tSTATUS\tPHASE\tSCHEDULED\tSTARTED\tFINISHED")
		for _, run := range filtered {
			finished := "-"
			if run.FinishedAt != nil {
				finished = run.FinishedAt.Format(time.RFC3339)
			}
			phase := run.Phase
			if phase == "" {
				phase = "-"
			}
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				run.RunID, run.JobID, run.Status, phase, run.ScheduledAt.Format(time.RFC3339),
				run.StartedAt.Format(time.RFC3339), finished)
		}
	}
	for _, issue := range issues {
		fmt.Fprintf(os.Stderr, "job run record issue at %s: %s\n", issue.Path, issue.Error)
	}
	if len(issues) > 0 {
		return 1
	}
	return 0
}

type agentStartOutput struct {
	SchemaVersion string            `json:"schemaVersion"`
	SupervisorPID int               `json:"supervisor_pid"`
	Run           agents.RunSummary `json:"run"`
}

func runJobsStart(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsStartHelpText())
		return 0
	}
	options, err := parseJobsStartArgs(args)
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
	plan, err := agents.BuildRunPlan(job, scheduledAt, options.executor)
	if err != nil {
		return printAgentCommandError(err)
	}
	if _, err := os.Stat(plan.RunDir); err == nil {
		fmt.Fprintf(os.Stderr, "job run already exists: %s\n", plan.RunDir)
		return 1
	} else if !errors.Is(err, os.ErrNotExist) {
		return printAgentCommandError(err)
	}
	executable, err := os.Executable()
	if err != nil {
		return printAgentCommandError(err)
	}
	childArgs := []string{"jobs", "run", options.path, "--at", scheduledAt.Format(time.RFC3339Nano)}
	if options.executor != "" {
		childArgs = append(childArgs, "--executor", options.executor)
	}
	pid, err := startDetachedJobProcess(executable, childArgs, os.Environ())
	if err != nil {
		return printAgentCommandError(fmt.Errorf("start detached job supervisor: %w", err))
	}
	summary, err := waitForAgentRun(plan.RepoRoot, plan.RunID, 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "started job supervisor %d but its run record was not ready: %v\n", pid, err)
		return 1
	}
	output := agentStartOutput{SchemaVersion: okf.MachineSchemaVersion, SupervisorPID: pid, Run: summary}
	if options.json {
		if err := printJSON(output); err != nil {
			return printAgentCommandError(err)
		}
	} else {
		fmt.Fprintf(os.Stdout, "job run %s %s\n", summary.RunID, summary.Status)
		fmt.Fprintf(os.Stdout, "supervisor: %d\n", pid)
		fmt.Fprintf(os.Stdout, "run: %s\n", summary.RunRecord)
	}
	if agents.IsTerminalRunStatus(summary.Status) && summary.Status != "succeeded" {
		return 1
	}
	return 0
}

func waitForAgentRun(repoRoot string, runID string, wait time.Duration) (agents.RunSummary, error) {
	deadline := time.Now().Add(wait)
	var lastErr error
	for time.Now().Before(deadline) {
		summary, err := agents.GetRunSummary(repoRoot, runID)
		if err == nil {
			return summary, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("run record did not appear")
	}
	return agents.RunSummary{}, lastErr
}

type agentControlOutput struct {
	SchemaVersion string            `json:"schemaVersion"`
	Action        string            `json:"action"`
	Run           agents.RunSummary `json:"run"`
}

func runJobsControl(args []string, action string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsControlHelpText(action))
		return 0
	}
	options, err := parseJobsControlArgs(args, action)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	summary, err := agents.GetRunSummary(options.repo, options.runID)
	if err != nil {
		return printAgentCommandError(err)
	}
	if !agents.IsTerminalRunStatus(summary.Status) {
		if summary.Status == "orphaned" {
			return printAgentCommandError(fmt.Errorf("%w: %s", agents.ErrRunOrphaned, options.runID))
		}
		runDir, err := agents.RunDirectory(options.repo, options.runID)
		if err != nil {
			return printAgentCommandError(err)
		}
		if err := agents.RequestRunAction(runDir, action, options.wait); err != nil {
			latest, readErr := agents.GetRunSummary(options.repo, options.runID)
			if readErr == nil && agents.IsTerminalRunStatus(latest.Status) {
				summary = latest
			} else {
				return printAgentCommandError(err)
			}
		} else {
			summary, err = agents.GetRunSummary(options.repo, options.runID)
			if err != nil {
				return printAgentCommandError(err)
			}
		}
	}
	output := agentControlOutput{SchemaVersion: okf.MachineSchemaVersion, Action: action, Run: summary}
	if options.json {
		if err := printJSON(output); err != nil {
			return printAgentCommandError(err)
		}
	} else {
		fmt.Fprintf(os.Stdout, "job run %s %s\n", summary.RunID, summary.Status)
	}
	return 0
}

func runJobsNew(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsNewHelpText())
		return 0
	}
	options, err := parseJobsNewArgs(args)
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
	fmt.Fprintf(os.Stdout, "created job: %s\n", options.out)
	fmt.Fprintf(os.Stdout, "validate: openknowledge jobs validate %s\n", options.out)
	fmt.Fprintf(os.Stdout, "dry run: openknowledge jobs run %s --dry-run\n", options.out)
	return 0
}

func runJobsList(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsListHelpText())
		return 0
	}
	jsonOutput := false
	var positionals []string
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOutput = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "unknown jobs list option: %s\n", arg)
			return 2
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		fmt.Fprintln(os.Stderr, "jobs list accepts at most one path")
		return 2
	}
	path := defaultJobsPath
	if len(positionals) == 1 {
		path = positionals[0]
	}
	jobs, err := agents.DiscoverJobs(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if jsonOutput {
				return printAgentListJSON(path, nil)
			}
			fmt.Fprintf(os.Stdout, "no jobs found at %s\n", path)
			return 0
		}
		return printAgentCommandError(err)
	}
	if len(jobs) == 0 {
		if jsonOutput {
			return printAgentListJSON(path, jobs)
		}
		fmt.Fprintf(os.Stdout, "no jobs found at %s\n", path)
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
			Agent:       job.Agent.Runtime,
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

func runJobsValidate(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsValidateHelpText())
		return 0
	}
	jsonOutput := false
	var positionals []string
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOutput = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "unknown jobs validate option: %s\n", arg)
			return 2
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		fmt.Fprintln(os.Stderr, "jobs validate requires exactly one job file or directory")
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
		fmt.Fprintf(os.Stdout, "no jobs found at %s\n", path)
		return 0
	}
	for _, job := range jobs {
		fmt.Fprintf(os.Stdout, "valid job: %s (%s)\n", job.ID, job.Path)
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

func runJobsRun(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsRunHelpText())
		return 0
	}
	options, err := parseJobsRunArgs(args)
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
	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	record, err := agents.RunJob(job, agents.RunOptions{
		Context:     runContext,
		Executor:    options.executor,
		DryRun:      options.dryRun,
		ScheduledAt: scheduledAt,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err != nil {
		if record.RunID != "" {
			fmt.Fprintf(os.Stderr, "job run %s failed: %v\n", record.RunID, err)
		} else {
			fmt.Fprintf(os.Stderr, "job run failed: %v\n", err)
		}
		return 1
	}
	if !options.dryRun {
		fmt.Fprintf(os.Stdout, "job run %s %s\n", record.RunID, record.Status)
		fmt.Fprintf(os.Stdout, "run: %s\n", filepath.Join(record.Plan.RunDir, "run.json"))
		if record.Status == "skipped" {
			fmt.Fprintf(os.Stdout, "reason: %s\n", record.StatusText)
		} else {
			fmt.Fprintf(os.Stdout, "worktree: %s\n", record.Plan.Worktree)
		}
	}
	return 0
}

func runJobsDaemon(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, jobsDaemonHelpText())
		return 0
	}
	options, err := parseJobsDaemonArgs(args)
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
		code := runDueAgentJobs(options.path, options.executor, options.dryRun, options.runtime)
		if options.once {
			return code
		}
		time.Sleep(interval)
	}
}

type jobsRunCLIOptions struct {
	path     string
	dryRun   bool
	at       string
	executor string
}

type jobsStartCLIOptions struct {
	path     string
	at       string
	executor string
	json     bool
}

type jobsRunsCLIOptions struct {
	repo   string
	job    string
	status string
	json   bool
}

type jobsControlCLIOptions struct {
	runID string
	repo  string
	wait  time.Duration
	json  bool
}

func parseJobsInventoryArgs(args []string, defaultPath string, command string) (string, bool, error) {
	jsonOutput := false
	positionals := make([]string, 0, 1)
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOutput = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown jobs %s option: %s", command, arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return "", false, fmt.Errorf("jobs %s accepts at most one path", command)
	}
	if len(positionals) == 1 {
		defaultPath = positionals[0]
	}
	return defaultPath, jsonOutput, nil
}

func parseJobsRunsArgs(args []string) (jobsRunsCLIOptions, error) {
	options := jobsRunsCLIOptions{repo: "."}
	positionals := make([]string, 0, 1)
	jobSet := false
	statusSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			options.json = true
		case arg == "--job" || arg == "--status":
			value, next, err := nextFlagValue(args, index, arg)
			if err != nil {
				return options, err
			}
			if arg == "--job" {
				options.job = value
				jobSet = true
			} else {
				options.status = value
				statusSet = true
			}
			index = next
		case strings.HasPrefix(arg, "--job="):
			options.job = strings.TrimPrefix(arg, "--job=")
			jobSet = true
		case strings.HasPrefix(arg, "--status="):
			options.status = strings.TrimPrefix(arg, "--status=")
			statusSet = true
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown jobs runs option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("jobs runs accepts at most one repository path")
	}
	if len(positionals) == 1 {
		options.repo = positionals[0]
	}
	if jobSet && strings.TrimSpace(options.job) == "" {
		return options, fmt.Errorf("--job requires a value")
	}
	if statusSet && strings.TrimSpace(options.status) == "" {
		return options, fmt.Errorf("--status requires a value")
	}
	return options, nil
}

func parseJobsStartArgs(args []string) (jobsStartCLIOptions, error) {
	var options jobsStartCLIOptions
	runArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			options.json = true
			continue
		}
		runArgs = append(runArgs, arg)
	}
	parsed, err := parseJobsRunArgs(runArgs)
	if err != nil {
		return options, errors.New(strings.Replace(err.Error(), "jobs run", "jobs start", 1))
	}
	if parsed.dryRun {
		return options, fmt.Errorf("jobs start does not support --dry-run")
	}
	options.path = parsed.path
	options.at = parsed.at
	options.executor = parsed.executor
	return options, nil
}

func parseJobsControlArgs(args []string, action string) (jobsControlCLIOptions, error) {
	defaultWait := 10 * time.Second
	if action == "kill" {
		defaultWait = 5 * time.Second
	}
	options := jobsControlCLIOptions{repo: ".", wait: defaultWait}
	positionals := make([]string, 0, 1)
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			options.json = true
		case arg == "--repo" || arg == "--wait":
			value, next, err := nextFlagValue(args, index, arg)
			if err != nil {
				return options, err
			}
			if arg == "--repo" {
				options.repo = value
			} else {
				parsed, err := time.ParseDuration(value)
				if err != nil || parsed < 0 {
					return options, fmt.Errorf("--wait must be a non-negative Go duration")
				}
				options.wait = parsed
			}
			index = next
		case strings.HasPrefix(arg, "--repo="):
			options.repo = strings.TrimPrefix(arg, "--repo=")
			if strings.TrimSpace(options.repo) == "" {
				return options, fmt.Errorf("--repo requires a value")
			}
		case strings.HasPrefix(arg, "--wait="):
			parsed, err := time.ParseDuration(strings.TrimPrefix(arg, "--wait="))
			if err != nil || parsed < 0 {
				return options, fmt.Errorf("--wait must be a non-negative Go duration")
			}
			options.wait = parsed
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown jobs %s option: %s", action, arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return options, fmt.Errorf("jobs %s requires exactly one run id", action)
	}
	options.runID = positionals[0]
	return options, nil
}

type jobsNewCLIOptions struct {
	template  string
	out       string
	list      bool
	reference bool
	force     bool
}

func parseJobsNewArgs(args []string) (jobsNewCLIOptions, error) {
	var options jobsNewCLIOptions
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
			return options, fmt.Errorf("unknown jobs new option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("jobs new accepts at most one template id")
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

func parseJobsRunArgs(args []string) (jobsRunCLIOptions, error) {
	var options jobsRunCLIOptions
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
			return options, fmt.Errorf("unknown jobs run option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return options, fmt.Errorf("jobs run requires exactly one job file")
	}
	normalizedExecutor, err := agents.NormalizeExecutorOverride(options.executor)
	if err != nil {
		return options, fmt.Errorf("--executor %w", err)
	}
	options.executor = normalizedExecutor
	options.path = positionals[0]
	return options, nil
}

type jobsDaemonCLIOptions struct {
	path     string
	once     bool
	dryRun   bool
	tick     string
	executor string
	runtime  string
}

func parseJobsDaemonArgs(args []string) (jobsDaemonCLIOptions, error) {
	options := jobsDaemonCLIOptions{path: defaultJobsPath, tick: "1m"}
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
		case arg == "--runtime":
			value, next, err := nextFlagValue(args, index, "--runtime")
			if err != nil {
				return options, err
			}
			options.runtime = strings.ToLower(value)
			index = next
		case strings.HasPrefix(arg, "--runtime="):
			options.runtime = strings.ToLower(strings.TrimPrefix(arg, "--runtime="))
			if strings.TrimSpace(options.runtime) == "" {
				return options, fmt.Errorf("--runtime requires a value")
			}
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown jobs daemon option: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) > 1 {
		return options, fmt.Errorf("jobs daemon accepts at most one jobs directory")
	}
	if len(positionals) == 1 {
		options.path = positionals[0]
	}
	normalizedExecutor, err := agents.NormalizeExecutorOverride(options.executor)
	if err != nil {
		return options, fmt.Errorf("--executor %w", err)
	}
	options.executor = normalizedExecutor
	if options.runtime != "" {
		if _, err := agents.HarnessForRuntime(options.runtime); err != nil {
			return options, err
		}
	}
	return options, nil
}

func writeAgentTemplate(path string, content string, force bool) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("--out requires a value")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("job already exists: %s (use --force to overwrite)", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func runDueAgentJobs(path string, executor string, dryRun bool, runtimeFilter ...string) int {
	jobs, loadFailures, err := agents.DiscoverJobsLenient(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stdout, "no jobs found at %s\n", path)
			return 0
		}
		return printAgentCommandError(err)
	}
	failureCount := len(loadFailures)
	for _, failure := range loadFailures {
		fmt.Fprintf(os.Stderr, "job %s failed to load: %v\n", failure.Path, failure.Err)
	}
	now := time.Now()
	dueCount := 0
	for _, job := range jobs {
		if len(runtimeFilter) > 0 && runtimeFilter[0] != "" && job.Agent.Runtime != runtimeFilter[0] {
			continue
		}
		scheduledAt, due, err := agents.DueScheduledAt(job, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", job.ID, err)
			failureCount++
			continue
		}
		if !due {
			continue
		}
		plan, err := agents.BuildRunPlan(job, scheduledAt, executor)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job %s failed to plan: %v\n", job.ID, err)
			failureCount++
			continue
		}
		runRecord := filepath.Join(plan.RunDir, "run.json")
		if _, err := os.Stat(runRecord); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "job %s could not inspect run record: %v\n", job.ID, err)
			failureCount++
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
				fmt.Fprintf(os.Stderr, "job run %s failed: %v\n", record.RunID, err)
			} else {
				fmt.Fprintf(os.Stderr, "job run failed: %v\n", err)
			}
			failureCount++
			continue
		}
		if !dryRun {
			fmt.Fprintf(os.Stdout, "job run %s %s\n", record.RunID, record.Status)
		}
	}
	if dueCount == 0 && failureCount == 0 {
		fmt.Fprintln(os.Stdout, "no due jobs")
	}
	if failureCount > 0 {
		fmt.Fprintf(os.Stderr, "jobs daemon pass completed with %d failure(s)\n", failureCount)
		return 1
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
		fmt.Fprintln(os.Stderr, "invalid job:")
		for _, issue := range validation.Issues {
			fmt.Fprintf(os.Stderr, "- %s: %s\n", issue.Field, issue.Message)
		}
		return 1
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func scheduleLabel(job agents.Job) string {
	return statusScheduleLabel(job.Schedule)
}

func statusScheduleLabel(schedule agents.ScheduleSpec) string {
	switch {
	case schedule.Cron != "":
		return "cron=" + schedule.Cron
	case schedule.Every != "":
		return "every=" + schedule.Every
	default:
		return "manual"
	}
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func jobsHelpText() string {
	return `openknowledge jobs

Experimental command group for deterministic local jobs from Markdown
specs with nested frontmatter. Job schema and scheduler behavior may still
change before this surface is treated as stable.

Usage:
  openknowledge jobs new
  openknowledge jobs new --list
  openknowledge jobs new --reference
  openknowledge jobs new <template>
  openknowledge jobs new <template> --out <file>
  openknowledge jobs list [path]
  openknowledge jobs status [jobs-dir]
  openknowledge jobs runs [repo]
  openknowledge jobs start <job.md>
  openknowledge jobs stop <run-id>
  openknowledge jobs kill <run-id>
  openknowledge jobs validate <job-or-dir>
  openknowledge jobs run <job.md>
  openknowledge jobs run <job.md> --dry-run
  openknowledge jobs daemon [jobs-dir]
  openknowledge jobs daemon [jobs-dir] --once
  openknowledge jobs --help

Subcommands:
  new        List, print, or write built-in job templates.
  list       List job specs.
  status     Show schedules, next eligible slots, and active/latest runs.
  runs       List current and historical runs for a repository.
  start      Start one job in a detached supervisor.
  stop       Request cancellation of a live run.
  kill       Force cancellation of a live run's command process tree.
  validate   Parse and schema-check job specs.
  run        Create a Git worktree and run one job now.
  daemon     Poll scheduled jobs and run due jobs.

Default jobs directory:
  .openknowledge/jobs
`
}

func jobsStatusHelpText() string {
	return `openknowledge jobs status

Show schedules, next eligible slots, and active/latest runs for jobs.

Usage:
  openknowledge jobs status [jobs-dir]
  openknowledge jobs status [jobs-dir] --json

The next eligible time is a scheduling slot, not a guarantee that a run will
start. Scheduled jobs run only while a jobs daemon is active.
`
}

func jobsRunsHelpText() string {
	return `openknowledge jobs runs

List current and historical job runs for a Git repository.

Usage:
  openknowledge jobs runs [repo]
  openknowledge jobs runs [repo] --job <id>
  openknowledge jobs runs [repo] --status <status>
  openknowledge jobs runs [repo] --json

Runs are ordered newest first. A persisted running record without a live
supervisor is reported as orphaned.
`
}

func jobsStartHelpText() string {
	return `openknowledge jobs start

Start one job in a detached supervisor and return after it is observable.

Usage:
  openknowledge jobs start <job.md>
  openknowledge jobs start <job.md> --at <time>
  openknowledge jobs start <job.md> --executor host|docker
  openknowledge jobs start <job.md> --json

Flags:
  --at         Scheduled time used for the deterministic run ID.
  --executor   Override sandbox.type with host or docker.
  --json       Print a schemaVersion 1 start result.
`
}

func jobsControlHelpText(action string) string {
	description := "Request cancellation from the live run supervisor."
	defaultWait := "10s"
	if action == "kill" {
		description = "Force cancellation of the live run's current command process tree."
		defaultWait = "5s"
	}
	return fmt.Sprintf(`openknowledge jobs %s

%s

Usage:
  openknowledge jobs %s <run-id>
  openknowledge jobs %s <run-id> --repo <path>
  openknowledge jobs %s <run-id> --wait <duration>
  openknowledge jobs %s <run-id> --json

Flags:
  --repo       Git repository that owns the run. Defaults to the current repo.
  --wait       Time to wait for terminal state. Defaults to %s; 0 returns after
               writing the supervisor request.
  --json       Print a schemaVersion 1 control result.
`, action, description, action, action, action, action, defaultWait)
}

func jobsNewHelpText() string {
	return `openknowledge jobs new

List, print, or write built-in job templates.

Usage:
  openknowledge jobs new
  openknowledge jobs new --list
  openknowledge jobs new --reference
  openknowledge jobs new <template>
  openknowledge jobs new <template> --out <file>
  openknowledge jobs new <template> --out <file> --force
  openknowledge jobs new --help

Arguments:
  template     Built-in template id. Use --list to see available ids.

Flags:
  --list       Print available built-in templates.
  --reference  Print the supported nested frontmatter syntax.
  --out        Write the selected template to a file instead of stdout.
  --force      Overwrite --out when the file already exists.

Examples:
  openknowledge jobs new docs-audit
  openknowledge jobs new docs-audit --out .openknowledge/jobs/docs-audit.md
  openknowledge jobs new custom --out .openknowledge/jobs/custom.md
  openknowledge jobs new --reference
`
}

func jobsListHelpText() string {
	return `openknowledge jobs list

List job specs.

Usage:
  openknowledge jobs list [path]
  openknowledge jobs list [path] --json
  openknowledge jobs list --help

Arguments:
  path       Job file or directory. Defaults to .openknowledge/jobs.

Flags:
  --json     Print the schemaVersion 1 job inventory JSON.
`
}

func jobsValidateHelpText() string {
	return `openknowledge jobs validate

Parse and schema-check job specs without running an agent.

Usage:
  openknowledge jobs validate <job-or-dir>
  openknowledge jobs validate <job-or-dir> --json
  openknowledge jobs validate --help

Flags:
  --json     Print the schemaVersion 1 validation report, including failures.
`
}

func jobsRunHelpText() string {
	return `openknowledge jobs run

Create an isolated Git worktree and run one job.

Usage:
  openknowledge jobs run <job.md>
  openknowledge jobs run <job.md> --dry-run
  openknowledge jobs run <job.md> --at <time>
  openknowledge jobs run <job.md> --executor host|docker
  openknowledge jobs run --help

Flags:
  --dry-run    Print the schemaVersion 1 run plan without creating a worktree.
  --at         Scheduled time used for the deterministic run ID.
               Accepts RFC3339, YYYY-MM-DD, or YYYY-MM-DD HH:MM.
  --executor   Override sandbox.type with host or docker.

Contracts:
  Dry-run JSON and persisted run.json use the published schemaVersion 1 agent
  plan and run-record schemas, including cancelled and killed states.
`
}

func jobsDaemonHelpText() string {
	return `openknowledge jobs daemon

Poll scheduled jobs and run due jobs.

Usage:
  openknowledge jobs daemon [jobs-dir]
  openknowledge jobs daemon [jobs-dir] --once
  openknowledge jobs daemon [jobs-dir] --tick <duration>
  openknowledge jobs daemon [jobs-dir] --dry-run
  openknowledge jobs daemon [jobs-dir] --runtime <runtime>
  openknowledge jobs daemon --help

Flags:
  --once       Check due jobs once and exit.
  --tick       Polling interval. Defaults to 1m.
  --dry-run    Print resolved plans for due jobs without executing.
  --executor   Override sandbox.type with host or docker.
  --runtime    Run only codex, claude, or opencode jobs.

Behavior:
  A pass attempts every loadable due job. Per-file, scheduling, planning, and
  execution failures are reported without blocking later jobs. --once exits 1
  after the pass when any job failed; polling mode continues at the next tick.
`
}
