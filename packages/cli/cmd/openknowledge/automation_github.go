package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	githubEventPullRequest      = "pull_request"
	githubEventPush             = "push"
	githubEventSchedule         = "schedule"
	githubEventWorkflowDispatch = "workflow_dispatch"
)

type githubAutomationOptions struct {
	Knowledge string
	Event     string
	Ref       string
	Base      string
	Eval      string
	Baseline  string
	Reports   string
}

type githubAutomationPlan struct {
	SchemaVersion        string   `json:"schemaVersion"`
	Knowledge            string   `json:"knowledge"`
	Config               string   `json:"config,omitempty"`
	Event                string   `json:"event"`
	Ref                  string   `json:"ref,omitempty"`
	ReleaseBranch        string   `json:"releaseBranch"`
	ReleasePolicy        string   `json:"releasePolicy"`
	ReleaseOutputs       []string `json:"releaseOutputs"`
	ReleaseActive        bool     `json:"releaseActive"`
	MaintenanceMode      string   `json:"maintenanceMode"`
	MaintenanceAgent     string   `json:"maintenanceAgent"`
	MaintenanceActive    bool     `json:"maintenanceActive"`
	MaintenanceDelivery  string   `json:"maintenanceDelivery"`
	MaintenanceAutoMerge bool     `json:"maintenanceAutoMerge"`
	Actions              []string `json:"actions"`
}

type githubAutomationResult struct {
	githubAutomationPlan
	StructureStatus   string `json:"structureStatus"`
	QualityStatus     string `json:"qualityStatus"`
	Health            string `json:"health"`
	MaintenanceStatus string `json:"maintenanceStatus"`
}

var githubAutomationValidate = runValidate
var githubAutomationClaims = runClaims
var githubAutomationAudit = runAudit
var githubAutomationEval = runEval
var githubAutomationMaintenance = runGitHubMaintenanceJob
var githubAutomationCommand = runGitHubAutomationCommand

func runAutomationGitHub(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, automationGitHubHelpText())
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	command := args[0]
	if command != "plan" && command != "run" {
		fmt.Fprintf(stderrOutput(), "unknown automation github command: %s\n", command)
		return 2
	}
	options, err := parseGitHubAutomationOptions(command, args[1:])
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	plan, err := buildGitHubAutomationPlan(options)
	if err != nil {
		return printAgentCommandError(err)
	}
	if command == "plan" {
		if err := printJSON(plan); err != nil {
			return printAgentCommandError(err)
		}
		return 0
	}
	return executeGitHubAutomation(plan, options)
}

func parseGitHubAutomationOptions(command string, args []string) (githubAutomationOptions, error) {
	flags := flag.NewFlagSet("automation github "+command, flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	options := githubAutomationOptions{}
	flags.StringVar(&options.Knowledge, "path", "Wiki", "knowledge base path")
	flags.StringVar(&options.Event, "event", os.Getenv("GITHUB_EVENT_NAME"), "GitHub event name")
	flags.StringVar(&options.Ref, "ref", os.Getenv("GITHUB_REF_NAME"), "GitHub branch or ref name")
	flags.StringVar(&options.Base, "base", "", "immutable Git base commit")
	flags.StringVar(&options.Eval, "eval", ".openknowledge/evals/knowledge.yaml", "evaluation dataset")
	flags.StringVar(&options.Baseline, "baseline", ".openknowledge/audit-sources.json", "source audit baseline")
	defaultReports := filepath.Join(os.Getenv("RUNNER_TEMP"), "openknowledge-reports")
	if strings.TrimSpace(os.Getenv("RUNNER_TEMP")) == "" {
		defaultReports = filepath.Join(".openknowledge", "reports", "github")
	}
	flags.StringVar(&options.Reports, "reports", defaultReports, "report output directory")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, fmt.Errorf("automation github %s does not accept positional arguments", command)
	}
	options.Event = strings.TrimSpace(options.Event)
	switch options.Event {
	case githubEventPullRequest, githubEventPush, githubEventSchedule, githubEventWorkflowDispatch:
	default:
		return options, fmt.Errorf("--event must be pull_request, push, schedule, or workflow_dispatch")
	}
	return options, nil
}

func buildGitHubAutomationPlan(options githubAutomationOptions) (githubAutomationPlan, error) {
	root, err := okf.ResolveKnowledgeRoot(options.Knowledge)
	if err != nil {
		return githubAutomationPlan{}, err
	}
	config, err := okf.LoadProjectConfig(root)
	if err != nil {
		return githubAutomationPlan{}, err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return githubAutomationPlan{}, err
	}
	maintenanceActive := (options.Event == githubEventSchedule || options.Event == githubEventWorkflowDispatch) && config.Maintenance.Mode != okf.MaintenanceModeOff
	releaseActive := options.Event == githubEventPush && strings.TrimSpace(options.Ref) == config.Release.Branch && len(config.Release.Outputs) > 0
	actions := []string{"validate-structure", "validate-claims", "audit", "evaluate"}
	if releaseActive {
		actions = append(actions, "release-"+config.Release.Policy)
	}
	if maintenanceActive {
		actions = append(actions, "maintenance-"+config.Maintenance.Mode)
	}
	return githubAutomationPlan{
		SchemaVersion:        okf.MachineSchemaVersion,
		Knowledge:            absRoot,
		Config:               config.Path,
		Event:                options.Event,
		Ref:                  strings.TrimSpace(options.Ref),
		ReleaseBranch:        config.Release.Branch,
		ReleasePolicy:        config.Release.Policy,
		ReleaseOutputs:       append([]string{}, config.Release.Outputs...),
		ReleaseActive:        releaseActive,
		MaintenanceMode:      config.Maintenance.Mode,
		MaintenanceAgent:     config.Maintenance.Agent,
		MaintenanceActive:    maintenanceActive,
		MaintenanceDelivery:  config.Maintenance.Delivery,
		MaintenanceAutoMerge: config.Maintenance.AutoMerge,
		Actions:              actions,
	}, nil
}

func executeGitHubAutomation(plan githubAutomationPlan, options githubAutomationOptions) int {
	if err := os.MkdirAll(options.Reports, 0o755); err != nil {
		return printAgentCommandError(err)
	}
	result := githubAutomationResult{
		githubAutomationPlan: plan,
		StructureStatus:      "passing",
		QualityStatus:        "passing",
		Health:               "passing",
		MaintenanceStatus:    "off",
	}
	validationPath := filepath.Join(options.Reports, "validation.json")
	if githubAutomationValidate([]string{"--format", "json", "--out", validationPath, plan.Knowledge}) != 0 {
		result.StructureStatus = "failing"
		result.QualityStatus = "skipped"
		result.Health = "failing"
		_ = printJSON(result)
		return 1
	}

	qualityFailed := false
	claimArgs := []string{"validate", "--path", plan.Knowledge, "--json"}
	if githubAutomationClaims(claimArgs) != 0 {
		qualityFailed = true
	}
	auditArgs := []string{plan.Knowledge, "--baseline", options.Baseline, "--fail-on", "high", "--format", "json", "--out", filepath.Join(options.Reports, "audit.json"), "--markdown-out", filepath.Join(options.Reports, "audit.md")}
	if githubAutomationAudit(auditArgs) != 0 {
		qualityFailed = true
	}
	evalArgs := []string{"run", options.Eval, plan.Knowledge, "--format", "json", "--out", filepath.Join(options.Reports, "eval.json")}
	if strings.TrimSpace(options.Base) != "" && options.Base != "0000000000000000000000000000000000000000" {
		evalArgs = append(evalArgs, "--base", options.Base, "--gate", "regressions")
	}
	if githubAutomationEval(evalArgs) != 0 {
		qualityFailed = true
	}
	if qualityFailed {
		result.QualityStatus = "failing"
		result.Health = "degraded"
	}

	if plan.MaintenanceActive {
		result.MaintenanceStatus = "running"
		if githubAutomationMaintenance(plan, options) != 0 {
			result.MaintenanceStatus = "failing"
			_ = printJSON(result)
			return 1
		}
		result.MaintenanceStatus = "completed"
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	if qualityFailed {
		followMainRelease := plan.ReleaseActive && plan.ReleasePolicy == okf.ReleasePolicyFollowMain
		if !followMainRelease && !plan.MaintenanceActive {
			return 1
		}
	}
	return 0
}

func runGitHubMaintenanceJob(plan githubAutomationPlan, options githubAutomationOptions) int {
	if token := strings.TrimSpace(os.Getenv("OPENKNOWLEDGE_MODEL_TOKEN")); token != "" {
		credential := "OPENAI_API_KEY"
		switch plan.MaintenanceAgent {
		case "claude":
			credential = "ANTHROPIC_API_KEY"
		case "opencode":
			credential = "OPENCODE_API_KEY"
		}
		if strings.TrimSpace(os.Getenv(credential)) == "" {
			if err := os.Setenv(credential, token); err != nil {
				fmt.Fprintln(stderrOutput(), err)
				return 1
			}
			defer os.Unsetenv(credential)
		}
	}
	repo, err := runtimeGitOutput(plan.Knowledge, "rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	knowledge, err := filepath.Rel(repo, plan.Knowledge)
	if err != nil || knowledge == ".." || strings.HasPrefix(knowledge, ".."+string(filepath.Separator)) {
		fmt.Fprintln(stderrOutput(), "knowledge base must stay inside the Git repository")
		return 1
	}
	knowledge = filepath.ToSlash(knowledge)
	directory, err := os.MkdirTemp("", "openknowledge-github-maintenance-")
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	defer os.RemoveAll(directory)
	jobPath := filepath.Join(directory, "github-maintenance.md")
	content := renderGitHubMaintenanceJob(repo, knowledge, plan.ReleaseBranch, plan.MaintenanceMode, plan.MaintenanceAgent, options.Eval, options.Baseline)
	if err := os.WriteFile(jobPath, []byte(content), 0o600); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	job, err := agents.ParseJobFile(jobPath)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	record, err := agents.RunJob(job, agents.RunOptions{
		Context:  context.Background(),
		Executor: "host",
		Stdout:   os.Stdout,
		Stderr:   stderrOutput(),
	})
	if err != nil {
		fmt.Fprintf(stderrOutput(), "maintenance job failed: %v\n", err)
		return 1
	}
	if record.Status != "succeeded" {
		fmt.Fprintf(stderrOutput(), "maintenance job ended with status %s\n", record.Status)
		return 1
	}
	if err := deliverGitHubMaintenance(record, plan); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	return 0
}

func deliverGitHubMaintenance(record agents.RunRecord, plan githubAutomationPlan) error {
	head, err := githubAutomationCommand(record.Plan.Worktree, "git", "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve maintenance head: %w", err)
	}
	base, err := githubAutomationCommand(record.Plan.Worktree, "git", "rev-parse", record.Plan.BaseSHA)
	if err != nil {
		return fmt.Errorf("resolve maintenance base: %w", err)
	}
	if strings.TrimSpace(head) == strings.TrimSpace(base) {
		fmt.Fprintln(os.Stdout, "maintenance produced no changes")
		return nil
	}

	branch := record.Plan.Branch
	if _, err := githubAutomationCommand(record.Plan.Worktree, "git", "push", "--set-upstream", "origin", branch); err != nil {
		return fmt.Errorf("push maintenance branch %s: %w", branch, err)
	}
	prURL, err := githubAutomationCommand(record.Plan.Worktree, "gh", "pr", "create",
		"--base", plan.ReleaseBranch,
		"--head", branch,
		"--title", "Maintain Open Knowledge",
		"--body", "Automated knowledge maintenance generated and verified by Open Knowledge.")
	if err != nil {
		return fmt.Errorf("create maintenance pull request: %w", err)
	}
	fmt.Fprintf(os.Stdout, "maintenance pull request: %s\n", strings.TrimSpace(prURL))

	if plan.MaintenanceMode == okf.MaintenanceModeAutonomous || plan.MaintenanceAutoMerge {
		if _, err := githubAutomationCommand(record.Plan.Worktree, "gh", "pr", "merge", branch, "--auto", "--squash", "--delete-branch"); err != nil {
			return fmt.Errorf("enable maintenance pull request auto-merge: %w", err)
		}
	}
	return nil
}

func runGitHubAutomationCommand(directory string, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func renderGitHubMaintenanceJob(repo string, knowledge string, branch string, mode string, agent string, evalPath string, baselinePath string) string {
	autonomy := "Propose evidence-backed changes. Do not merge or push directly."
	if mode == okf.MaintenanceModeAutonomous {
		autonomy = "Apply evidence-backed changes in the generated branch. Do not merge or push directly."
	}
	validateCommand := "openknowledge validate " + githubShellQuote(knowledge)
	claimsCommand := "openknowledge claims validate --path " + githubShellQuote(knowledge) + " --json"
	auditCommand := "openknowledge audit " + githubShellQuote(knowledge) + " --baseline " + githubShellQuote(baselinePath) + " --fail-on high"
	return fmt.Sprintf(`---
id: github-maintenance
enabled: true
schedule:
  every: 24h
  timezone: UTC
agent:
  runtime: %s
  timeout: 45m
  completion_signal: COMPLETE
workspace:
  repo: %s
  base: %s
  strategy: branch
  branch: "openknowledge/maintenance/{{date}}-{{run_id}}"
  dirty_policy: fail
sandbox:
  type: host
verify:
  commands:
    - %s
    - %s
    - %s
  eval:
    dataset: %s
    target: %s
    gate: regressions
output:
  commit: true
  pr: false
  commit_message: "Maintain Open Knowledge"
concurrency:
  key: github-maintenance
  policy: skip
---

Audit %s with the configured Open Knowledge rules and validation policy.
%s
Preserve unresolved conflicts and report missing evidence. End with COMPLETE.
`, jsonString(agent), jsonString(repo), jsonString(branch), jsonString(validateCommand), jsonString(claimsCommand), jsonString(auditCommand), jsonString(evalPath), jsonString(knowledge), knowledge, autonomy)
}

func githubShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func automationGitHubHelpText() string {
	return `openknowledge automation github

Plan or run the GitHub Action bridge from the bundle's .openknowledge.toml.

Usage:
  openknowledge automation github plan --event <event> [--path <wiki>] [--ref <branch>]
  openknowledge automation github run --event <event> [--path <wiki>] [--ref <branch>] [--base <sha>]

Events:
  pull_request, push, schedule, workflow_dispatch

The bridge reads [release] and [maintenance] from .openknowledge.toml. It does
not observe remote sources by default. Scheduled and manual runs execute a
generated maintenance job only when maintenance.mode is propose or autonomous.
`
}
