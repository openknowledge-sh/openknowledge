package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

// TestGoldenKnowledgeLifecycle is the product acceptance path. It proves that
// one Git repository can move from setup, through an evidence-backed finding
// and fix, to answer and abstention evaluation, immutable publication, runtime
// retrieval, feedback, and rollback.
func TestGoldenKnowledgeLifecycle(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "Wiki")
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "golden@example.test")
	runGit(t, repo, "config", "user.name", "Golden Path")
	runGit(t, repo, "remote", "add", "origin", "https://github.com/example/knowledge.git")
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Product knowledge\n")
	writeMainTestFile(t, root, "api-policy.md", "---\ntype: Source\ntitle: API policy\nowner: team:platform\n---\n\n# API policy\n\nThe supported user endpoint is `/v1/users`.\n")
	writeMainTestFile(t, root, "runbook.md", `---
type: Runbook
title: User recovery
owner: team:platform
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces: {api: https://example.test/api/}
  entities: [{id: api:user-service}]
  predicates: [{id: api:userEndpoint, object_kind: literal, datatype: xsd:string, maximum_count: 1}]
sources:
  - id: api-policy
    resource: api-policy.md
    role: authoritative
    authority_approved_by: human:platform-lead
---

# User recovery

<a id="endpoint"></a>

## Endpoint

Call /v1/users to recover a user.
`)
	writeMainTestFile(t, repo, ".openknowledge/integration.toml", "version = 1\nknowledge_base = \"Wiki\"\ninsights = \"Wiki/insights\"\nruntime = \"codex\"\n")

	setup, err := setupKnowledgeGitHub(root, false, false)
	if err != nil || len(setup.Created) != 3 {
		t.Fatalf("setup CI: %#v err=%v", setup, err)
	}
	datasetPath := filepath.Join(repo, ".openknowledge", "evals", "knowledge.yaml")
	writeMainTestFile(t, repo, ".openknowledge/evals/knowledge.yaml", `type: openknowledge.eval
version: 1
id: user-api
cases:
  - id: recovery-endpoint
    question: Which endpoint recovers a user?
    agents: [support-agent]
    expect:
      sources: [runbook.md]
      evidence_contains: [/v2/users]
      answer_contains: [/v2/users]
      citation_sources: [runbook.md]
      min_citations: 1
      min_groundedness: 1
      answer_decision: answer
      min_entailed_citations: 1
  - id: unknown-payroll
    question: What is the lunar payroll account number?
    agents: [support-agent]
    expect:
      answer_decision: abstain
`)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "stale knowledge baseline")

	writeMainTestFile(t, root, "api-policy.md", "---\ntype: Source\ntitle: API policy\nowner: team:platform\n---\n\n# API policy\n\nThe supported user endpoint is `/v2/users`.\n")
	reportPath := filepath.Join(repo, ".openknowledge", "reports", "audit.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := captureMainOutput(t, func() int {
		return runAudit([]string{root, "--baseline", filepath.Join(repo, ".openknowledge", "audit-sources.json"), "--format", "json", "--out", reportPath})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("audit changed source: code=%d stderr=%q", code, stderr)
	}
	auditReport, err := knowledgeaudit.ReadReport(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	changedFinding := ""
	for _, finding := range auditReport.Findings {
		if finding.Category == "source-changed" {
			changedFinding = finding.ID
			break
		}
	}
	if changedFinding == "" {
		t.Fatalf("audit did not connect the changed source to its dependent runbook: %#v", auditReport.Findings)
	}
	t.Chdir(repo)
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runAudit([]string{"propose", changedFinding, "--report", reportPath, "--path", root})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"state": "created"`) {
		t.Fatalf("durable finding proposal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	runbookContent := strings.ReplaceAll(string(mustReadGoldenFile(t, filepath.Join(root, "runbook.md"))), "/v1/users", "/v2/users")
	writeMainTestFile(t, root, "runbook.md", runbookContent)
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runEvidence([]string{"pin", "--document", "runbook.md", "--source", "api-policy", "--path", root, "--json"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"changed": true`) {
		t.Fatalf("pin evidence: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	proposalPath := filepath.Join(repo, ".openknowledge", "proposals", "endpoint.json")
	if err := os.MkdirAll(filepath.Dir(proposalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	claimJSONBytes, _ := json.Marshal(map[string]any{"id": "api:claim/user-endpoint/2", "slot": "api:slot/user-endpoint", "subject": "api:user-service", "predicate": "api:userEndpoint", "object": map[string]any{"value": "/v2/users", "datatype": "xsd:string"}, "evidence": []any{map[string]any{"id": "api:evidence/user-endpoint/2", "sourceRef": "api-policy", "stance": "supports", "role": "primary", "selector": map[string]any{"type": "text_quote", "exact": "The supported user endpoint is `/v2/users`."}}}, "status": "proposed", "sectionRef": "#endpoint"})
	_, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"propose", "--path", root, "--from", "runbook.md", "--claim-json", string(claimJSONBytes), "--reason", "The approved policy changed", "--confidence", "0.99", "--out", proposalPath})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("claim proposal: code=%d stderr=%q", code, stderr)
	}
	_, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"apply", proposalPath, "--path", root, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("apply claim proposal: code=%d stderr=%q", code, stderr)
	}
	_, stderr, code = captureMainOutput(t, func() int {
		return runClaims([]string{"verify", "api:claim/user-endpoint/2", "--document", "runbook.md", "--approved-by", "human:platform-lead", "--path", root, "--json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("verify claim: code=%d stderr=%q", code, stderr)
	}

	stdout, stderr, code = captureMainOutput(t, func() int {
		return runEval([]string{"run", datasetPath, root, "--base", "HEAD", "--gate", "regressions", "--answer-command", os.Args[0], "--answer-arg=-test.run=^TestGoldenAnswerRunnerHelper$", "--answer-arg=--", "--answer-arg=golden", "--json"})
	})
	var comparison knowledgeeval.ComparisonReport
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &comparison) != nil || comparison.Summary.Improved != 1 || comparison.Summary.ProposedPassed != 2 {
		t.Fatalf("answer comparison: code=%d stdout=%q stderr=%q report=%#v", code, stdout, stderr, comparison)
	}

	enablePublicArtifactTest(t, root)
	runtimeSetup, err := setupKnowledgeRuntime(root, "auto", "", false, false)
	if err != nil || runtimeSetup.Executor != "github-actions" {
		t.Fatalf("runtime setup: %#v err=%v", runtimeSetup, err)
	}
	config := string(mustReadGoldenFile(t, filepath.Join(repo, deployRuntimeConfig)))
	for _, expected := range []string{"require_resolved_claims = true", "run_jobs = false", `required_checks = ["Open Knowledge checks"]`} {
		if !strings.Contains(config, expected) {
			t.Fatalf("runtime does not publish only approved green knowledge; missing %q:\n%s", expected, config)
		}
	}
	runtimeConfig, err := okruntime.LoadConfig(filepath.Join(repo, filepath.FromSlash(deployRuntimeConfig)))
	if err != nil {
		t.Fatal(err)
	}
	runtimeConfig.Runtime.StateDir = filepath.Join(repo, ".openknowledge", "runtime-state")
	runtimeConfig.ArtifactStore.Path = filepath.Join(repo, ".openknowledge", "artifacts")
	runtimeConfig.KnowledgeBases[0].Path = root
	runtimeConfig.KnowledgeBases[0].Outputs = []string{okf.ReleaseOutputMCP, okf.ReleaseOutputViewer}
	runtimeConfig.Serve.UsageEvents.Enabled = true
	runtimeConfig.Serve.UsageEvents.CaptureQueries = false
	first, err := buildRuntimeKnowledgeGenerationWithChecks(runtimeConfig, runtimeConfig.KnowledgeBases[0], "golden-first", filepath.Join(repo, ".openknowledge", "builds", "first"), true, []string{"knowledge-ci"})
	if err != nil || first.Published == nil {
		t.Fatalf("publish first green generation: %#v %v", first, err)
	}
	handler, err := newRuntimeServeHandler(runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("activate golden generation: %v", failures)
	}
	search := runtimeRequest(t, handler, http.MethodGet, "/_search?q=recover+user", "", nil)
	var searchResult runtimeSearchResponse
	if search.Code != http.StatusOK || json.Unmarshal(search.Body.Bytes(), &searchResult) != nil || searchResult.Decision != "answer" || searchResult.UsageEventID == "" {
		t.Fatalf("runtime search: code=%d result=%#v body=%s", search.Code, searchResult, search.Body.String())
	}
	feedback := runtimeRequest(t, handler, http.MethodPost, "/_feedback", `{"usageEventId":"`+searchResult.UsageEventID+`","sentiment":"positive","reasons":[]}`, nil)
	if feedback.Code != http.StatusCreated || feedback.Header().Get("X-OpenKnowledge-Generation") != first.Published.Generation {
		t.Fatalf("runtime feedback: code=%d body=%s", feedback.Code, feedback.Body.String())
	}
	initialize := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"golden","version":"1"}}}`, nil)
	session := initialize.Header().Get("Mcp-Session-Id")
	initialized := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, map[string]string{"Mcp-Session-Id": session})
	contextResult := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"openknowledge_search","arguments":{"query":"Which endpoint recovers a user?"}}}`, map[string]string{"Mcp-Session-Id": session})
	if initialize.Code != http.StatusOK || session == "" || initialized.Code != http.StatusAccepted || contextResult.Code != http.StatusOK || !strings.Contains(contextResult.Body.String(), "api:claim/user-endpoint/2") || !strings.Contains(contextResult.Body.String(), "claim_projection") || !strings.Contains(contextResult.Body.String(), "evidenceArtifacts") || !strings.Contains(contextResult.Body.String(), "api:evidence/user-endpoint/2") {
		t.Fatalf("runtime MCP evidence bundle: init=%d session=%q initialized=%d context=%d body=%s", initialize.Code, session, initialized.Code, contextResult.Code, contextResult.Body.String())
	}
	writeMainTestFile(t, root, "release-note.md", "---\ntype: Release Note\ntitle: Reviewed release\n---\n\n# Reviewed release\n\nThe endpoint guidance passed the golden lifecycle.\n")
	second, err := buildRuntimeKnowledgeGenerationWithChecks(runtimeConfig, runtimeConfig.KnowledgeBases[0], "golden-second", filepath.Join(repo, ".openknowledge", "builds", "second"), true, []string{"knowledge-ci"})
	if err != nil || second.Published == nil || second.Published.PreviousGeneration != first.Published.Generation {
		t.Fatalf("publish second green generation: %#v %v", second, err)
	}
	store := okruntime.FilesystemStore{Root: runtimeConfig.ArtifactStore.Path}
	pointer, _, err := store.Rollback(runtimeConfig.KnowledgeBases[0].ID, "")
	if err != nil || pointer.Generation != first.Published.Generation || pointer.PreviousGeneration != second.Published.Generation {
		t.Fatalf("rollback generation: %#v %v", pointer, err)
	}
}

func TestGoldenAnswerRunnerHelper(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "golden" {
		return
	}
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	var request knowledgeeval.AnswerRequest
	if json.Unmarshal(content, &request) != nil {
		os.Exit(3)
	}
	response := knowledgeeval.AnswerResponse{SchemaVersion: knowledgeeval.AnswerProtocolVersion, Answers: []knowledgeeval.RunnerAnswer{}}
	for _, evalCase := range request.Cases {
		if evalCase.ID == "unknown-payroll" {
			response.Answers = append(response.Answers, knowledgeeval.RunnerAnswer{CaseID: evalCase.ID, Decision: "abstain", Answer: "The knowledge base does not contain this information.", Claims: []knowledgeeval.AnswerClaim{}, RefusalReasons: []string{"no_relevant_evidence"}})
			continue
		}
		endpoint := "/v1/users"
		locator := ""
		for _, source := range evalCase.Sources {
			if strings.Contains(source.Markdown, "/v2/users") {
				endpoint = "/v2/users"
			}
			if source.Path == "runbook.md" {
				locator = source.Locator
			}
		}
		answer := "Recover the user through " + endpoint + "."
		response.Answers = append(response.Answers, knowledgeeval.RunnerAnswer{
			CaseID: evalCase.ID, Decision: "answer", Answer: answer,
			Claims:     []knowledgeeval.AnswerClaim{{Text: answer, Citations: []string{locator}}},
			Entailment: []knowledgeeval.CitationEntailmentAttestation{{Locator: locator, Status: "entailed", Method: "deterministic-test", Reason: "The selected runbook states the endpoint."}},
		})
	}
	if json.NewEncoder(os.Stdout).Encode(response) != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func mustReadGoldenFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}
