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

func TestSetupRuntimeChoosesOneMaintenanceExecutor(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/example/knowledge.git")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\ntitle: Recovery\nowner: team:docs\n---\n\n# Recovery\n\nRestore the previous version.\n")
	result, err := setupKnowledgeRuntime(filepath.Join(repo, "Wiki"), "auto", "codex", false, false)
	if err != nil || result.Executor != "runtime" {
		t.Fatalf("runtime fallback setup: %#v err=%v", result, err)
	}
	config, _ := os.ReadFile(filepath.Join(repo, deployRuntimeConfig))
	if !strings.Contains(string(config), "run_jobs = true") || !strings.Contains(string(config), "knowledge_ci = true") || !strings.Contains(string(config), `runtimes = ["codex"]`) || !strings.Contains(string(config), "required_checks = []") {
		t.Fatalf("runtime maintenance was not enabled:\n%s", config)
	}
	if _, err := os.Stat(filepath.Join(repo, ".openknowledge", "jobs", "knowledge-maintenance.md")); err != nil {
		t.Fatalf("runtime maintenance job missing: %v", err)
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
