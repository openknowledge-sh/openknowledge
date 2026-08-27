package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestGitHubAutomationPlanReadsCanonicalProjectConfig(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeMainTestFile(t, repo, "Wiki/.openknowledge.toml", `[release]
branch = "stable"
policy = "last-passing"
outputs = ["viewer", "mcp"]

[maintenance]
mode = "autonomous"
agent = "codex"
delivery = "pull-request"
auto_merge = true
`)
	plan, err := buildGitHubAutomationPlan(githubAutomationOptions{
		Knowledge: filepath.Join(repo, "Wiki"), Event: githubEventSchedule, Ref: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.ReleaseBranch != "stable" || plan.ReleasePolicy != okf.ReleasePolicyLastPassing || plan.ReleaseActive {
		t.Fatalf("unexpected release plan: %#v", plan)
	}
	if strings.Join(plan.ReleaseOutputs, ",") != "mcp,viewer" {
		t.Fatalf("unexpected release outputs: %#v", plan.ReleaseOutputs)
	}
	if !plan.MaintenanceActive || plan.MaintenanceMode != okf.MaintenanceModeAutonomous || !plan.MaintenanceAutoMerge {
		t.Fatalf("unexpected maintenance plan: %#v", plan)
	}
	release, err := buildGitHubAutomationPlan(githubAutomationOptions{
		Knowledge: filepath.Join(repo, "Wiki"), Event: githubEventPush, Ref: "stable",
	})
	if err != nil || !release.ReleaseActive || !containsString(release.Actions, "release-last-passing") {
		t.Fatalf("expected configured production release: plan=%#v err=%v", release, err)
	}
}

func TestGitHubAutomationRunNeedsNoModelSecretWhenMaintenanceIsOff(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\ntitle: Recovery\nowner: team:docs\n---\n\n# Recovery\n\nRestore the previous release.\n")
	writeMainTestFile(t, repo, "Wiki/.openknowledge.toml", `[release]
branch = "main"
policy = "follow-main"

[maintenance]
mode = "off"
`)
	if _, err := setupKnowledgeGitHub(filepath.Join(repo, "Wiki"), false, false); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENKNOWLEDGE_MODEL_TOKEN", "")
	options := githubAutomationOptions{
		Knowledge: filepath.Join(repo, "Wiki"),
		Event:     githubEventPullRequest,
		Eval:      filepath.Join(repo, ".openknowledge", "evals", "knowledge.yaml"),
		Baseline:  filepath.Join(repo, ".openknowledge", "audit-sources.json"),
		Reports:   filepath.Join(t.TempDir(), "reports"),
	}
	plan, err := buildGitHubAutomationPlan(options)
	if err != nil {
		t.Fatal(err)
	}
	if plan.MaintenanceActive {
		t.Fatalf("maintenance off unexpectedly active: %#v", plan)
	}
	if code := executeGitHubAutomation(plan, options); code != 0 {
		t.Fatalf("maintenance off run failed without a model secret: %d", code)
	}
}

func TestGitHubAutomationRunAppliesReleaseHealthPolicyAndMaintenanceMode(t *testing.T) {
	originalValidate := githubAutomationValidate
	originalClaims := githubAutomationClaims
	originalAudit := githubAutomationAudit
	originalEval := githubAutomationEval
	originalMaintenance := githubAutomationMaintenance
	t.Cleanup(func() {
		githubAutomationValidate = originalValidate
		githubAutomationClaims = originalClaims
		githubAutomationAudit = originalAudit
		githubAutomationEval = originalEval
		githubAutomationMaintenance = originalMaintenance
	})

	githubAutomationValidate = func([]string) int { return 0 }
	githubAutomationClaims = func([]string) int { return 1 }
	githubAutomationAudit = func(args []string) int {
		for _, arg := range args {
			if arg == "--observe-remote" {
				t.Fatal("GitHub automation must not observe remote sources by default")
			}
		}
		return 0
	}
	githubAutomationEval = func([]string) int { return 0 }
	maintenanceRuns := 0
	githubAutomationMaintenance = func(githubAutomationPlan, githubAutomationOptions) int {
		maintenanceRuns++
		return 0
	}

	options := githubAutomationOptions{Reports: t.TempDir(), Eval: "eval.yaml", Baseline: "baseline.json"}
	plan := githubAutomationPlan{
		SchemaVersion:   okf.MachineSchemaVersion,
		Knowledge:       t.TempDir(),
		ReleasePolicy:   okf.ReleasePolicyFollowMain,
		ReleaseActive:   true,
		MaintenanceMode: okf.MaintenanceModeOff,
	}
	if code := executeGitHubAutomation(plan, options); code != 0 {
		t.Fatalf("follow-main must keep a structurally valid degraded run available, got %d", code)
	}
	if maintenanceRuns != 0 {
		t.Fatalf("maintenance mode off executed a model job %d times", maintenanceRuns)
	}
	plan.ReleaseActive = false
	if code := executeGitHubAutomation(plan, options); code == 0 {
		t.Fatal("a pull-request quality failure must remain visible even with follow-main")
	}

	plan.ReleaseActive = true
	plan.ReleasePolicy = okf.ReleasePolicyLastPassing
	if code := executeGitHubAutomation(plan, options); code == 0 {
		t.Fatal("last-passing must reject a quality failure")
	}

	plan.ReleasePolicy = okf.ReleasePolicyFollowMain
	plan.MaintenanceMode = okf.MaintenanceModeAutonomous
	plan.MaintenanceActive = true
	if code := executeGitHubAutomation(plan, options); code != 0 {
		t.Fatalf("autonomous executable handoff failed: %d", code)
	}
	if maintenanceRuns != 1 {
		t.Fatalf("autonomous mode must execute one generated job, got %d", maintenanceRuns)
	}
}

func TestRenderGitHubMaintenanceJobUsesConfiguredAgentAndBranch(t *testing.T) {
	content := renderGitHubMaintenanceJob("/repo", "Wiki", "stable", okf.MaintenanceModeAutonomous, "codex", ".openknowledge/evals/knowledge.yaml", ".openknowledge/audit-sources.json")
	for _, wanted := range []string{`runtime: "codex"`, `base: "stable"`, "strategy: branch", "Apply evidence-backed changes", "pr: false", "completion_signal: COMPLETE", "claims validate", "audit", "eval:", "gate: regressions"} {
		if !strings.Contains(content, wanted) {
			t.Fatalf("generated maintenance job missing %q:\n%s", wanted, content)
		}
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "job.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := runJobsValidate([]string{path}); code != 0 {
		t.Fatalf("generated maintenance job is invalid: %d", code)
	}
}

func TestDeliverGitHubMaintenanceCreatesPullRequest(t *testing.T) {
	originalCommand := githubAutomationCommand
	t.Cleanup(func() { githubAutomationCommand = originalCommand })

	var calls []string
	githubAutomationCommand = func(directory string, name string, args ...string) (string, error) {
		call := name + " " + strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "git rev-parse HEAD":
			return "head-sha", nil
		case "git rev-parse base-sha":
			return "base-sha", nil
		case "git push --set-upstream origin openknowledge/maintenance/run-1":
			return "", nil
		case "gh pr create --base main --head openknowledge/maintenance/run-1 --title Maintain Open Knowledge --body Automated knowledge maintenance generated and verified by Open Knowledge.":
			return "https://github.example/pull/1", nil
		default:
			return "", fmt.Errorf("unexpected command %s", call)
		}
	}
	record := agents.RunRecord{Plan: agents.RunPlan{
		Worktree: "/tmp/worktree", Branch: "openknowledge/maintenance/run-1", BaseSHA: "base-sha",
	}}
	plan := githubAutomationPlan{ReleaseBranch: "main", MaintenanceMode: okf.MaintenanceModePropose}
	if err := deliverGitHubMaintenance(record, plan); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(calls, "\n"), "gh pr merge") {
		t.Fatalf("propose mode must not auto-merge: %v", calls)
	}
}

func TestDeliverGitHubMaintenanceAutonomousEnablesAutoMerge(t *testing.T) {
	originalCommand := githubAutomationCommand
	t.Cleanup(func() { githubAutomationCommand = originalCommand })

	var calls []string
	githubAutomationCommand = func(directory string, name string, args ...string) (string, error) {
		call := name + " " + strings.Join(args, " ")
		calls = append(calls, call)
		switch {
		case call == "git rev-parse HEAD":
			return "head-sha", nil
		case call == "git rev-parse base-sha":
			return "base-sha", nil
		case strings.HasPrefix(call, "git push "):
			return "", nil
		case strings.HasPrefix(call, "gh pr create "):
			return "https://github.example/pull/1", nil
		case call == "gh pr merge openknowledge/maintenance/run-1 --auto --squash --delete-branch":
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command %s", call)
		}
	}
	record := agents.RunRecord{Plan: agents.RunPlan{
		Worktree: "/tmp/worktree", Branch: "openknowledge/maintenance/run-1", BaseSHA: "base-sha",
	}}
	plan := githubAutomationPlan{ReleaseBranch: "main", MaintenanceMode: okf.MaintenanceModeAutonomous}
	if err := deliverGitHubMaintenance(record, plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(calls, "\n"), "gh pr merge openknowledge/maintenance/run-1 --auto --squash --delete-branch") {
		t.Fatalf("autonomous mode did not enable auto-merge: %v", calls)
	}
}

func TestDeliverGitHubMaintenanceSkipsDeliveryWithoutChanges(t *testing.T) {
	originalCommand := githubAutomationCommand
	t.Cleanup(func() { githubAutomationCommand = originalCommand })

	githubAutomationCommand = func(directory string, name string, args ...string) (string, error) {
		if name == "git" && len(args) == 2 && args[0] == "rev-parse" {
			return "same-sha", nil
		}
		return "", fmt.Errorf("unexpected delivery command: %s %s", name, strings.Join(args, " "))
	}
	record := agents.RunRecord{Plan: agents.RunPlan{Worktree: "/tmp/worktree", Branch: "branch", BaseSHA: "base-sha"}}
	if err := deliverGitHubMaintenance(record, githubAutomationPlan{ReleaseBranch: "main", MaintenanceMode: okf.MaintenanceModeAutonomous}); err != nil {
		t.Fatal(err)
	}
}
