package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestSetupCIPlansAppliesAndPreservesGeneratedWorkflow(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/example/knowledge.git")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\ntitle: Recovery\nowner: team:docs\nsources:\n  - id: handbook\n    resource: handbook.md\n---\n\n# Recovery\n\nRestore the previous release.\n")
	writeMainTestFile(t, repo, "Wiki/handbook.md", "---\ntype: Source\ntitle: Handbook\nowner: team:docs\nsources:\n  - resource: guide.md\n---\n\n# Handbook\n")

	plan, err := setupKnowledgeCI(filepath.Join(repo, "Wiki"), true, false)
	if err != nil || !plan.Plan || len(plan.Created) != 3 {
		t.Fatalf("unexpected CI plan: %#v err=%v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(repo, setupCIWorkflowPath)); !os.IsNotExist(err) {
		t.Fatalf("plan wrote workflow: %v", err)
	}
	result, err := setupKnowledgeCI(filepath.Join(repo, "Wiki"), false, false)
	if err != nil || len(result.Created) != 3 {
		t.Fatalf("apply CI setup: %#v err=%v", result, err)
	}
	workflow, _ := os.ReadFile(filepath.Join(repo, setupCIWorkflowPath))
	var workflowDocument map[string]any
	if err := yaml.Unmarshal(workflow, &workflowDocument); err != nil {
		t.Fatalf("generated workflow is not valid YAML: %v\n%s", err, workflow)
	}
	for _, wanted := range []string{"push:", "schedule:", "github.event.before", "claims validate", "audit-sources.json", "--observe-remote", "eval run", "openknowledge-reports", "Enforce knowledge gates"} {
		if !strings.Contains(string(workflow), wanted) {
			t.Fatalf("generated workflow missing %q:\n%s", wanted, workflow)
		}
	}
	repeated, err := setupKnowledgeCI(filepath.Join(repo, "Wiki"), false, false)
	if err != nil || len(repeated.Preserved) != 3 {
		t.Fatalf("CI setup must be idempotent: %#v err=%v", repeated, err)
	}
}

func TestSetupGitHubPlansAppliesAndPreservesActionWorkflow(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\ntitle: Recovery\nowner: team:docs\n---\n\n# Recovery\n\nRestore the previous release.\n")
	writeMainTestFile(t, repo, "Wiki/.openknowledge.toml", "[release]\nbranch = \"stable\"\npolicy = \"follow-main\"\noutputs = [\"viewer\", \"mcp\"]\n")
	t.Chdir(repo)
	output, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{"github", "Wiki", "--plan"})
	})
	if code != 0 || !strings.Contains(output, `"profile": "github"`) {
		t.Fatalf("setup github must accept documented trailing flags, code=%d stdout=%s stderr=%s", code, output, stderr)
	}

	plan, err := setupKnowledgeGitHub(filepath.Join(repo, "Wiki"), true, false)
	if err != nil || !plan.Plan || plan.Profile != "github" || len(plan.Created) != 3 {
		t.Fatalf("unexpected GitHub plan: %#v err=%v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(repo, setupGitHubWorkflowPath)); !os.IsNotExist(err) {
		t.Fatalf("plan wrote workflow: %v", err)
	}
	result, err := setupKnowledgeGitHub(filepath.Join(repo, "Wiki"), false, false)
	if err != nil || len(result.Created) != 3 {
		t.Fatalf("apply GitHub setup: %#v err=%v", result, err)
	}
	workflow, err := os.ReadFile(filepath.Join(repo, setupGitHubWorkflowPath))
	if err != nil {
		t.Fatal(err)
	}
	var workflowDocument map[string]any
	if err := yaml.Unmarshal(workflow, &workflowDocument); err != nil {
		t.Fatalf("generated workflow is not valid YAML: %v\n%s", err, workflow)
	}
	for _, wanted := range []string{"pull_request:", "push:", `branches: ["stable"]`, "schedule:", "workflow_dispatch:", "openknowledge-sh/openknowledge@v", "OPENKNOWLEDGE_MODEL_TOKEN", "GH_TOKEN", "contents: write", "pull-requests: write", "path: \"Wiki\""} {
		if !strings.Contains(string(workflow), wanted) {
			t.Fatalf("generated workflow missing %q:\n%s", wanted, workflow)
		}
	}
	checksStart := strings.Index(string(workflow), "  checks:")
	maintenanceStart := strings.Index(string(workflow), "  maintenance:")
	if checksStart < 0 || maintenanceStart <= checksStart {
		t.Fatalf("generated workflow must separate checks and maintenance jobs:\n%s", workflow)
	}
	checksJob := string(workflow)[checksStart:maintenanceStart]
	if strings.Contains(checksJob, "OPENKNOWLEDGE_MODEL_TOKEN") || strings.Contains(checksJob, "contents: write") {
		t.Fatalf("ordinary checks must not receive model credentials or write permission:\n%s", checksJob)
	}
	for _, duplicatedPolicy := range []string{"claims validate", "--observe-remote", "eval run"} {
		if strings.Contains(string(workflow), duplicatedPolicy) {
			t.Fatalf("generated workflow duplicates config-driven automation %q:\n%s", duplicatedPolicy, workflow)
		}
	}
	repeated, err := setupKnowledgeGitHub(filepath.Join(repo, "Wiki"), false, false)
	if err != nil || len(repeated.Preserved) != 3 {
		t.Fatalf("GitHub setup must be idempotent: %#v err=%v", repeated, err)
	}
}

func TestRootActionDelegatesPolicyToAutomationBridge(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("action metadata is not valid YAML: %v", err)
	}
	for _, wanted := range []string{"automation github plan", "automation github run", "maintenance_active", "Install configured maintenance agent"} {
		if !strings.Contains(string(content), wanted) {
			t.Fatalf("root action missing %q:\n%s", wanted, content)
		}
	}
	if strings.Contains(string(content), "--observe-remote") {
		t.Fatalf("root action must not enable remote observation by default:\n%s", content)
	}
}

func TestSetupRuntimeChoosesOneMaintenanceExecutor(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/example/knowledge.git")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\ntitle: Recovery\nowner: team:docs\n---\n\n# Recovery\n\nRestore the previous version.\n")
	writeMainTestFile(t, repo, "Wiki/.openknowledge.toml", "[release]\nbranch = \"stable\"\npolicy = \"last-passing\"\noutputs = [\"viewer\", \"mcp\"]\n")
	result, err := setupKnowledgeRuntime(filepath.Join(repo, "Wiki"), "auto", "codex", false, false)
	if err != nil || result.Executor != "runtime" {
		t.Fatalf("runtime fallback setup: %#v err=%v", result, err)
	}
	config, _ := os.ReadFile(filepath.Join(repo, deployRuntimeConfig))
	if !strings.Contains(string(config), "run_jobs = true") || !strings.Contains(string(config), "knowledge_ci = true") || !strings.Contains(string(config), `runtimes = ["codex"]`) || !strings.Contains(string(config), "required_checks = []") || !strings.Contains(string(config), `release_policy = "last-passing"`) || !strings.Contains(string(config), `production_branch = "stable"`) {
		t.Fatalf("runtime maintenance was not enabled:\n%s", config)
	}
	jobPath := filepath.Join(repo, ".openknowledge", "jobs", "knowledge-maintenance.md")
	if _, err := os.Stat(jobPath); err != nil {
		t.Fatalf("runtime maintenance job missing: %v", err)
	}
	job, err := os.ReadFile(jobPath)
	if err != nil || !strings.Contains(string(job), `base: "stable"`) {
		t.Fatalf("runtime maintenance job does not use the configured release branch: %v\n%s", err, job)
	}
	for _, path := range []string{".openknowledge/evals/knowledge.yaml", ".openknowledge/audit-sources.json"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("runtime Knowledge CI input missing %s: %v", path, err)
		}
	}
	repeated, err := setupKnowledgeRuntime(filepath.Join(repo, "Wiki"), "auto", "codex", false, false)
	if err != nil || len(repeated.Created) != 0 {
		t.Fatalf("runtime setup must be idempotent: %#v err=%v", repeated, err)
	}
}

func TestSetupRuntimeRequiresExplicitReleaseOutput(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/example/knowledge.git")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeMainTestFile(t, repo, "Wiki/.openknowledge.toml", "[release]\noutputs = []\n")
	_, err := setupKnowledgeRuntime(filepath.Join(repo, "Wiki"), "auto", "", true, false)
	if err == nil || !strings.Contains(err.Error(), "at least one release output") {
		t.Fatalf("expected local-only runtime setup refusal, got %v", err)
	}
}
