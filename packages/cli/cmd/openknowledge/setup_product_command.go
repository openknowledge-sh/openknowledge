package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const setupCIWorkflowPath = ".github/workflows/openknowledge-ci.yml"

type setupProductResult struct {
	SchemaVersion string   `json:"schemaVersion"`
	Profile       string   `json:"profile"`
	Knowledge     string   `json:"knowledge"`
	Repository    string   `json:"repository"`
	Plan          bool     `json:"plan"`
	Executor      string   `json:"executor,omitempty"`
	Created       []string `json:"created"`
	Preserved     []string `json:"preserved"`
}

func runSetupCI(args []string) int {
	flags := flag.NewFlagSet("setup ci", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	planOnly := flags.Bool("plan", false, "show the setup plan without writing files")
	force := flags.Bool("force", false, "replace generated CI files")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "setup ci accepts at most one knowledge base path")
		return 2
	}
	knowledge := "Wiki"
	if flags.NArg() == 1 {
		knowledge = flags.Arg(0)
	}
	result, err := setupKnowledgeCI(knowledge, *planOnly, *force)
	if err != nil {
		return printAgentCommandError(err)
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func setupKnowledgeCI(knowledgeInput string, planOnly bool, force bool) (setupProductResult, error) {
	root, repo, relative, err := setupProductRoots(knowledgeInput)
	if err != nil {
		return setupProductResult{}, err
	}
	result := setupProductResult{
		SchemaVersion: okf.MachineSchemaVersion, Profile: "ci", Knowledge: root,
		Repository: repo, Plan: planOnly, Created: []string{}, Preserved: []string{},
	}
	evalPath := filepath.Join(repo, ".openknowledge", "evals", "knowledge.yaml")
	baselinePath := filepath.Join(repo, ".openknowledge", "audit-sources.json")
	workflowPath := filepath.Join(repo, filepath.FromSlash(setupCIWorkflowPath))

	if setupHasEvalDataset(filepath.Dir(evalPath)) {
		result.Preserved = append(result.Preserved, filepath.Dir(evalPath))
	} else if planOnly {
		result.Created = append(result.Created, evalPath)
	} else {
		dataset, buildErr := buildStarterEvalDataset(root)
		if buildErr != nil {
			return result, buildErr
		}
		if err := knowledgeeval.WriteNewDataset(evalPath, dataset); err != nil {
			return result, err
		}
		result.Created = append(result.Created, evalPath)
	}

	if _, statErr := os.Stat(baselinePath); statErr == nil && !force {
		result.Preserved = append(result.Preserved, baselinePath)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return result, statErr
	} else if planOnly {
		result.Created = append(result.Created, baselinePath)
	} else {
		_, baseline, auditErr := knowledgeaudit.Run(knowledgeaudit.Options{Root: root, Spec: "latest"})
		if auditErr != nil {
			return result, auditErr
		}
		content, encodeErr := knowledgeaudit.EncodeBaseline(baseline)
		if encodeErr != nil {
			return result, encodeErr
		}
		if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
			return result, err
		}
		if err := writeOutputFileAtomically(baselinePath, content); err != nil {
			return result, err
		}
		result.Created = append(result.Created, baselinePath)
	}

	evalRelative, err := filepath.Rel(repo, setupFirstEvalDataset(filepath.Dir(evalPath), evalPath))
	if err != nil {
		return result, err
	}
	workflow := renderKnowledgeCIWorkflow(filepath.ToSlash(relative), filepath.ToSlash(evalRelative), version)
	existingWorkflow, workflowErr := os.ReadFile(workflowPath)
	workflowCurrent := workflowErr == nil && string(existingWorkflow) == workflow
	if workflowCurrent {
		result.Preserved = append(result.Preserved, workflowPath)
	} else if planOnly {
		result.Created = append(result.Created, workflowPath)
	} else if err := writeDeployRuntimeScaffoldFile(workflowPath, []byte(workflow), 0o644, force); err != nil {
		return result, err
	} else {
		result.Created = append(result.Created, workflowPath)
	}
	sort.Strings(result.Created)
	sort.Strings(result.Preserved)
	return result, nil
}

func runSetupRuntime(args []string) int {
	flags := flag.NewFlagSet("setup runtime", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	planOnly := flags.Bool("plan", false, "show the setup plan without writing files")
	force := flags.Bool("force", false, "replace generated runtime files")
	maintenance := flags.String("maintenance", "auto", "maintenance executor: auto, github-actions, or runtime")
	runtimes := flags.String("runtimes", "", "comma-separated agent runtimes")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "setup runtime accepts at most one knowledge base path")
		return 2
	}
	knowledge := "Wiki"
	if flags.NArg() == 1 {
		knowledge = flags.Arg(0)
	}
	result, err := setupKnowledgeRuntime(knowledge, *maintenance, *runtimes, *planOnly, *force)
	if err != nil {
		return printAgentCommandError(err)
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func setupKnowledgeRuntime(knowledgeInput string, executor string, runtimes string, planOnly bool, force bool) (setupProductResult, error) {
	root, repo, _, err := setupProductRoots(knowledgeInput)
	if err != nil {
		return setupProductResult{}, err
	}
	executor = strings.ToLower(strings.TrimSpace(executor))
	if executor == "auto" {
		if _, statErr := os.Stat(filepath.Join(repo, filepath.FromSlash(setupCIWorkflowPath))); statErr == nil {
			executor = "github-actions"
		} else {
			executor = "runtime"
		}
	}
	if executor != "github-actions" && executor != "runtime" {
		return setupProductResult{}, fmt.Errorf("--maintenance must be auto, github-actions, or runtime")
	}
	if executor == "runtime" && strings.TrimSpace(runtimes) == "" {
		runtimes = agents.RuntimeCodex
	}
	origin, err := runtimeGitOutput(repo, "remote", "get-url", "origin")
	if err != nil {
		return setupProductResult{}, fmt.Errorf("setup runtime requires a GitHub origin remote: %w", err)
	}
	_, githubRepository, err := normalizeGitHubDeployRepository(origin)
	if err != nil {
		return setupProductResult{}, err
	}
	result := setupProductResult{
		SchemaVersion: okf.MachineSchemaVersion, Profile: "runtime", Knowledge: root,
		Repository: repo, Plan: planOnly, Executor: executor, Created: []string{}, Preserved: []string{},
	}
	if executor == "runtime" {
		if err := setupRuntimeKnowledgeAssets(root, repo, planOnly, force, &result); err != nil {
			return result, err
		}
	}
	for _, relative := range []string{deployRuntimeDockerfile, deployRuntimeEntrypoint, deployRuntimeConfig} {
		path := filepath.Join(repo, filepath.FromSlash(relative))
		if _, statErr := os.Stat(path); statErr == nil && !force {
			result.Preserved = append(result.Preserved, path)
		} else {
			result.Created = append(result.Created, path)
		}
	}
	jobPath := filepath.Join(repo, ".openknowledge", "jobs", "knowledge-maintenance.md")
	if executor == "runtime" {
		if _, statErr := os.Stat(jobPath); statErr == nil && !force {
			result.Preserved = append(result.Preserved, jobPath)
		} else {
			result.Created = append(result.Created, jobPath)
		}
	}
	if planOnly {
		sort.Strings(result.Created)
		sort.Strings(result.Preserved)
		return result, nil
	}
	requiredChecks := []string{}
	if executor == "github-actions" {
		requiredChecks = []string{"knowledge-ci"}
	}
	_, err = scaffoldRailwayRuntime(root, deployRuntimeScaffoldOptions{
		Runtimes: runtimes, OpenKnowledgeVersion: version, CodexVersion: defaultCodexRuntimeVersion,
		ClaudeVersion: defaultClaudeRuntimeVersion, OpenCodeVersion: defaultOpenCodeRuntimeVersion,
		Force: force, RunJobs: executor == "runtime", KnowledgeCI: executor == "runtime", GitHubRepository: githubRepository,
		RequiredChecks: requiredChecks,
	})
	if err != nil {
		return result, err
	}
	if executor == "runtime" {
		content := renderKnowledgeMaintenanceJob(filepath.ToSlash(mustRelative(repo, root)), firstRuntimeName(runtimes))
		if err := writeDeployRuntimeScaffoldFile(jobPath, []byte(content), 0o644, force); err != nil {
			return result, err
		}
	}
	sort.Strings(result.Created)
	sort.Strings(result.Preserved)
	return result, nil
}

func setupRuntimeKnowledgeAssets(root string, repo string, planOnly bool, force bool, result *setupProductResult) error {
	evalPath := filepath.Join(repo, ".openknowledge", "evals", "knowledge.yaml")
	baselinePath := filepath.Join(repo, ".openknowledge", "audit-sources.json")
	if setupHasEvalDataset(filepath.Dir(evalPath)) {
		result.Preserved = append(result.Preserved, filepath.Dir(evalPath))
	} else if planOnly {
		result.Created = append(result.Created, evalPath)
	} else {
		dataset, err := buildStarterEvalDataset(root)
		if err != nil {
			return err
		}
		if err := knowledgeeval.WriteNewDataset(evalPath, dataset); err != nil {
			return err
		}
		result.Created = append(result.Created, evalPath)
	}
	if _, err := os.Stat(baselinePath); err == nil && !force {
		result.Preserved = append(result.Preserved, baselinePath)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	} else if planOnly {
		result.Created = append(result.Created, baselinePath)
	} else {
		_, baseline, err := knowledgeaudit.Run(knowledgeaudit.Options{Root: root, Spec: "latest"})
		if err != nil {
			return err
		}
		content, err := knowledgeaudit.EncodeBaseline(baseline)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
			return err
		}
		if err := writeOutputFileAtomically(baselinePath, content); err != nil {
			return err
		}
		result.Created = append(result.Created, baselinePath)
	}
	return nil
}

func setupProductRoots(knowledgeInput string) (string, string, string, error) {
	root, err := okf.ResolveKnowledgeRoot(knowledgeInput)
	if err != nil {
		return "", "", "", err
	}
	repo, err := runtimeGitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", "", fmt.Errorf("setup profile requires a Git repository: %w", err)
	}
	if evaluated, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		root = evaluated
	}
	if evaluated, evalErr := filepath.EvalSymlinks(repo); evalErr == nil {
		repo = evaluated
	}
	relative, err := filepath.Rel(repo, root)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", "", fmt.Errorf("knowledge base must stay inside the Git repository")
	}
	return root, repo, relative, nil
}

func buildStarterEvalDataset(root string) (knowledgeeval.Dataset, error) {
	bundle, err := okf.ParseASTWithVersion(root, "latest")
	if err != nil {
		return knowledgeeval.Dataset{}, err
	}
	var cases []knowledgeeval.Case
	seen := map[string]int{}
	for _, document := range bundle.Documents {
		if document.Reserved || document.FrontmatterDiagnostic != nil || document.ReadDiagnostic != nil || strings.TrimSpace(document.Metadata.Type) == "" {
			continue
		}
		title := strings.TrimSpace(document.Metadata.Title)
		if title == "" && len(document.Markdown.Headings) > 0 {
			title = strings.TrimSpace(document.Markdown.Headings[0].Text)
		}
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(document.Rel), filepath.Ext(document.Rel))
		}
		id := sanitizeDeployName(strings.TrimSuffix(document.Rel, filepath.Ext(document.Rel)))
		if id == "" {
			id = "knowledge"
		}
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[id])
		}
		cases = append(cases, knowledgeeval.Case{
			ID: id, Question: fmt.Sprintf("What does the %s documentation say?", title),
			Agents: []string{"knowledge-agent"},
			Expect: knowledgeeval.Expectations{Sources: []string{document.Rel}, MinSources: 1},
		})
		if len(cases) == 5 {
			break
		}
	}
	if len(cases) == 0 {
		return knowledgeeval.Dataset{}, fmt.Errorf("setup ci cannot generate eval questions because the knowledge base has no concept documents")
	}
	return knowledgeeval.Dataset{Type: knowledgeeval.DatasetType, Version: knowledgeeval.DatasetVersion, ID: "knowledge-ci", Cases: cases}, nil
}

func setupHasEvalDataset(directory string) bool {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml") {
			return true
		}
	}
	return false
}

func setupFirstEvalDataset(directory string, fallback string) string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fallback
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml") {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	if len(paths) == 0 {
		return fallback
	}
	sort.Strings(paths)
	return paths[0]
}

func renderKnowledgeCIWorkflow(knowledgePath string, evalPath string, cliVersion string) string {
	return fmt.Sprintf(`name: Open Knowledge CI

on:
  pull_request:
  push:
    branches: [main]
  schedule:
    - cron: "17 3 * * *"
  workflow_dispatch:

permissions:
  contents: read

jobs:
  knowledge-ci:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-node@v4
        with:
          node-version: 22
      - name: Install Open Knowledge
        run: npm install --global @openknowledge-sh/openknowledge@%s
      - name: Validate knowledge structure
        id: structure
        continue-on-error: true
        env:
          KNOWLEDGE_PATH: %s
        run: |
          mkdir -p "$RUNNER_TEMP/openknowledge-reports"
          okn validate "$KNOWLEDGE_PATH"
      - name: Validate claim lifecycle
        id: claims
        continue-on-error: true
        env:
          KNOWLEDGE_PATH: %s
          BASE_SHA: ${{ github.event.pull_request.base.sha || github.event.before }}
        run: |
          if [ -n "$BASE_SHA" ] && [ "$BASE_SHA" != "0000000000000000000000000000000000000000" ]; then
            git worktree add --detach "$RUNNER_TEMP/openknowledge-base" "$BASE_SHA"
            okn claims validate --path "$KNOWLEDGE_PATH" \
              --against "$RUNNER_TEMP/openknowledge-base/$KNOWLEDGE_PATH" --json \
              > "$RUNNER_TEMP/openknowledge-reports/claims-validation.json"
          else
            okn claims validate --path "$KNOWLEDGE_PATH" --json \
              > "$RUNNER_TEMP/openknowledge-reports/claims-validation.json"
          fi
      - name: Audit knowledge
        id: audit
        continue-on-error: true
        env:
          KNOWLEDGE_PATH: %s
        run: |
          set +e
          okn audit "$KNOWLEDGE_PATH" --baseline .openknowledge/audit-sources.json \
            --observe-remote \
            --fail-on high --format json \
            --out "$RUNNER_TEMP/openknowledge-reports/audit.json" \
            --markdown-out "$RUNNER_TEMP/openknowledge-reports/audit.md"
          exit $?
      - name: Evaluate answer impact
        id: eval
        continue-on-error: true
        env:
          KNOWLEDGE_PATH: %s
          EVAL_PATH: %s
          BASE_SHA: ${{ github.event.pull_request.base.sha || github.event.before }}
        run: |
          set +e
          if [ -n "$BASE_SHA" ] && [ "$BASE_SHA" != "0000000000000000000000000000000000000000" ]; then
            okn eval run "$EVAL_PATH" "$KNOWLEDGE_PATH" --base "$BASE_SHA" \
              --gate regressions --format json \
              --out "$RUNNER_TEMP/openknowledge-reports/eval.json"
            eval_status=$?
            okn eval run "$EVAL_PATH" "$KNOWLEDGE_PATH" --base "$BASE_SHA" \
              --gate regressions --format markdown \
              --out "$RUNNER_TEMP/openknowledge-reports/eval.md"
          else
            okn eval run "$EVAL_PATH" "$KNOWLEDGE_PATH" --format json \
              --out "$RUNNER_TEMP/openknowledge-reports/eval.json"
            eval_status=$?
            okn eval run "$EVAL_PATH" "$KNOWLEDGE_PATH" --format markdown \
              --out "$RUNNER_TEMP/openknowledge-reports/eval.md"
          fi
          markdown_status=$?
          if [ -f "$RUNNER_TEMP/openknowledge-reports/eval.md" ]; then
            cat "$RUNNER_TEMP/openknowledge-reports/eval.md" >> "$GITHUB_STEP_SUMMARY"
          fi
          if [ "$eval_status" -ne 0 ]; then exit "$eval_status"; fi
          exit "$markdown_status"
      - name: Upload knowledge reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: openknowledge-reports
          path: ${{ runner.temp }}/openknowledge-reports
          if-no-files-found: warn
          retention-days: 30
      - name: Enforce knowledge gates
        if: always()
        env:
          STRUCTURE_OUTCOME: ${{ steps.structure.outcome }}
          CLAIMS_OUTCOME: ${{ steps.claims.outcome }}
          AUDIT_OUTCOME: ${{ steps.audit.outcome }}
          EVAL_OUTCOME: ${{ steps.eval.outcome }}
        run: |
          if [ "$STRUCTURE_OUTCOME" != success ] || \
             [ "$CLAIMS_OUTCOME" != success ] || \
             [ "$AUDIT_OUTCOME" != success ]; then
            exit 1
          fi
          if [ "$EVAL_OUTCOME" != success ] && [ "$EVAL_OUTCOME" != skipped ]; then
            exit 1
          fi
`, cliVersion, jsonString(knowledgePath), jsonString(knowledgePath), jsonString(knowledgePath), jsonString(knowledgePath), jsonString(evalPath))
}

func renderKnowledgeMaintenanceJob(knowledgePath string, runtimeName string) string {
	return fmt.Sprintf(`---
id: knowledge-maintenance
enabled: true
schedule:
  every: 24h
  timezone: UTC
agent:
  runtime: %s
  timeout: 45m
workspace:
  repo: "."
  base: main
  strategy: branch
  branch: "jobs/{{id}}/{{date}}-{{run_id}}"
  dirty_policy: fail
sandbox:
  type: host
verify:
  commands:
    - openknowledge validate %s
  eval:
    dataset: .openknowledge/evals/knowledge.yaml
    target: %s
    gate: regressions
output:
  commit: true
  pr: true
  commit_message: "Maintain agent knowledge"
concurrency:
  key: knowledge-maintenance
  policy: skip
---

Run this audit for the configured knowledge base:

openknowledge audit %s --baseline .openknowledge/audit-sources.json \
  --observe-remote --format json --out .openknowledge/reports/audit.json

Convert actionable findings with openknowledge audit propose. Apply only evidence-backed changes.
After an accepted source change, update the same baseline in the proposed branch.
Preserve unresolved conflicts and request an owner decision. End with COMPLETE.
`, runtimeName, knowledgePath, knowledgePath, knowledgePath)
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func mustRelative(root string, target string) string {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return relative
}

func firstRuntimeName(value string) string {
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			return item
		}
	}
	return agents.RuntimeCodex
}
