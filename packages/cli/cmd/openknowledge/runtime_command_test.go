package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/claimops"
	knowledgefeedback "github.com/openknowledge-sh/openknowledge/packages/cli/internal/feedback"
	knowledgeintervention "github.com/openknowledge-sh/openknowledge/packages/cli/internal/intervention"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

type runtimeHandlerRoundTripper struct {
	handler http.Handler
}

func (transport runtimeHandlerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	response := httptest.NewRecorder()
	transport.handler.ServeHTTP(response, request)
	return response.Result(), nil
}

func TestRuntimeInfoUsesStdout(t *testing.T) {
	stdout, stderr, code := captureMainOutput(t, func() int {
		runtimeInfof("runtime lifecycle %s\n", "ready")
		return 0
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "runtime lifecycle ready\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestRuntimeExchangeSummariesCarryPassingEvalAttestation(t *testing.T) {
	request := runtimeExchangeRequest{
		Version: 1, RunID: "run-1", JobID: "knowledge", Branch: "jobs/knowledge/run-1",
		BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40), BundleSHA256: strings.Repeat("c", 64), VerifyCount: 2,
		Eval:        &runtimeExchangeEval{Status: "pass", Dataset: ".openknowledge/evals/wiki.yaml", Target: "Wiki", Base: strings.Repeat("a", 40), Gate: "regressions", Regressions: 0, ProposedFailed: 1, Total: 20, BasePassed: 16, ProposedPassed: 19},
		Maintenance: &runtimeExchangeMaintenance{Risk: "medium", Approval: "human", Confidence: 0.8, Owners: []string{"github:reviewer"}, Insights: []string{"insight-1"}, Findings: []string{"OpenAPI removed /v1/users."}, Paths: []string{"Wiki/insights/one.md"}, ExpertTargets: []string{}, Status: "proposed"},
	}
	if err := validateRuntimeExchangeEval(request.Eval); err != nil {
		t.Fatal(err)
	}
	for _, summary := range []string{runtimeExchangePullRequestSummary(request), runtimeExchangeCheckSummary(request, "https://github.test/pr/1")} {
		if !strings.Contains(summary, ".openknowledge/evals/wiki.yaml") || !strings.Contains(summary, "16/20") || !strings.Contains(summary, "19/20") || !strings.Contains(summary, "0 regressions") || !strings.Contains(summary, "1 proposed failures") || !strings.Contains(summary, "medium") || !strings.Contains(summary, "80%") || (strings.Contains(summary, "Raw prompts") && !strings.Contains(summary, "OpenAPI removed /v1/users")) {
			t.Fatalf("eval attestation missing from publication summary: %s", summary)
		}
	}
	invalid := *request.Eval
	invalid.Status = "fail"
	if err := validateRuntimeExchangeEval(&invalid); err == nil {
		t.Fatal("failed eval attestation was accepted")
	}
}

func TestRuntimeExchangeSummaryExplainsClaimImpactAndHumanDecision(t *testing.T) {
	request := runtimeExchangeRequest{RunID: "run-claims", JobID: "knowledge", BaseSHA: strings.Repeat("a", 40), VerifyCount: 3}
	review := runtimeClaimReview{Changes: []runtimeClaimChange{{
		Knowledge: "wiki", ID: "auth.token-format", Path: "auth.md", BeforeValue: `"JWT"`, AfterValue: `"opaque"`,
		BeforeStatus: "verified", AfterStatus: "disputed", Sources: []string{"identity-openapi"}, Documents: 2, Evals: 4,
	}}}
	summary := runtimeExchangePullRequestSummaryWithClaims(request, review)
	for _, wanted := range []string{"## Knowledge claims", "auth.token-format", "verified → disputed", "2 docs, 4 evals", "## Human decision", "sources disagree"} {
		if !strings.Contains(summary, wanted) {
			t.Fatalf("claim review summary misses %q:\n%s", wanted, summary)
		}
	}
	if !runtimeClaimReviewRequiresHuman(review) {
		t.Fatal("disputed claim review must disable auto-merge")
	}
}

func TestRuntimeExchangeCommitRejectsRemovedVerifiedClaimHistory(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "runtime@example.test")
	runGit(t, repo, "config", "user.name", "Runtime Test")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\nokf_version: \"0.2\"\n---\n\n# Wiki\n")
	writeMainTestFile(t, repo, "Wiki/.openknowledge.toml", "[release]\noutputs = [\"viewer\", \"mcp\"]\n")
	writeMainTestFile(t, repo, "Wiki/auth.md", mainTypedClaimDocument("okn:claim/token-format/1", "verified", "JWT"))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	base := runtimeGitTest(t, repo, "rev-parse", "HEAD")
	writeMainTestFile(t, repo, "Wiki/auth.md", "---\ntype: Authentication\n---\n\n# Authentication\n\nToken format removed.\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "remove claim")
	head := runtimeGitTest(t, repo, "rev-parse", "HEAD")
	config := okruntime.Config{
		Root:           repo,
		Runtime:        okruntime.RuntimeConfig{StateDir: filepath.Join(t.TempDir(), "state")},
		KnowledgeBases: []okruntime.KnowledgeBaseConfig{{ID: "wiki", Path: filepath.Join(repo, "Wiki"), Spec: "0.2", Outputs: []string{okf.ReleaseOutputViewer}}},
	}
	if _, err := validateRuntimeExchangeCommit(context.Background(), config, repo, base, head); err == nil || !strings.Contains(err.Error(), "cannot be removed") {
		t.Fatalf("publisher accepted removed verified claim history: %v", err)
	}
}

func TestRuntimeMaintenanceAttestationEnforcesExpertEvidenceBoundary(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "runtime@example.test")
	runGit(t, repo, "config", "user.name", "Runtime Test")
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\n---\n\n# Guide\n\nCurrent.\n")
	insight := `---
type: Open Knowledge Insight
title: Resolve conflict
description: Expert decision.
status: draft
okf_publish: false
okf_insight_id: expert-one
okf_insight_kind: knowledge-audit
generated:
  by: process:openknowledge-cli
  at: 2026-08-21T12:00:00Z
okf_insight_targets: [guide.md]
okf_insight_route:
  risk: high
  approval: expert
  confidence: 1
  owners: [github:expert]
tags: [insight]
---

# Resolve conflict
`
	writeMainTestFile(t, repo, "Wiki/insights/expert.md", insight)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	base, err := runtimeWorkerGit(context.Background(), okruntime.Config{}, "", repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	writeMainTestFile(t, repo, "Wiki/insights/expert.md", strings.Replace(insight, "status: draft", "status: draft\nokf_insight_status: blocked", 1)+"\n## Evidence\n\n- Current systems disagree.\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "escalate")
	head, err := runtimeWorkerGit(context.Background(), okruntime.Config{}, "", repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	route, err := runtimeMaintenanceAttestation(context.Background(), repo, base, head)
	if err != nil || route == nil || route.Approval != "expert" || route.Status != "escalated" || !reflect.DeepEqual(route.ExpertTargets, []string{"Wiki/guide.md"}) {
		t.Fatalf("unexpected expert attestation: %#v err=%v", route, err)
	}
	if err := validateRuntimeExpertBoundary(context.Background(), repo, base, head, route); err != nil {
		t.Fatalf("evidence-only escalation rejected: %v", err)
	}
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\n---\n\n# Guide\n\nChanged without expert.\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "unsafe target edit")
	unsafeHead, err := runtimeWorkerGit(context.Background(), okruntime.Config{}, "", repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeExpertBoundary(context.Background(), repo, base, unsafeHead, route); err == nil || !strings.Contains(err.Error(), "expert-only") {
		t.Fatalf("expected expert target boundary rejection, got %v", err)
	}
}

func TestRuntimeMaintenanceAttestationRejectsDeletedInsight(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "runtime@example.test")
	runGit(t, repo, "config", "user.name", "Runtime Test")
	writeMainTestFile(t, repo, "Wiki/insights/pending.md", `---
type: Open Knowledge Insight
title: Pending
description: Pending evidence.
status: draft
okf_publish: false
okf_insight_id: pending-one
okf_insight_kind: knowledge-audit
generated:
  by: process:openknowledge-cli
  at: 2026-08-21T12:00:00Z
okf_insight_targets: [guide.md]
okf_insight_route:
  risk: high
  approval: expert
  confidence: 1
  owners: [github:expert]
tags: [insight]
---

# Pending
`)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	base, err := runtimeWorkerGit(context.Background(), okruntime.Config{}, "", repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repo, "Wiki/insights/pending.md")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "delete insight")
	head, err := runtimeWorkerGit(context.Background(), okruntime.Config{}, "", repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeMaintenanceAttestation(context.Background(), repo, base, head); err == nil || !strings.Contains(err.Error(), "read changed insight") {
		t.Fatalf("expected deleted insight rejection, got %v", err)
	}
}

func TestRuntimeMaintenanceClaimMustMatchCommits(t *testing.T) {
	expected := &runtimeExchangeMaintenance{Risk: "medium", Approval: "human", Confidence: 0.8, Owners: []string{"github:reviewer"}, Insights: []string{"one"}, Findings: []string{}, Paths: []string{"Wiki/insights/one.md"}, ExpertTargets: []string{}, Status: "proposed"}
	claimed := *expected
	claimed.Risk = "low"
	claimed.Approval = "auto"
	claimed.Confidence = 0.99
	if err := validateRuntimeMaintenanceClaim(expected, &claimed); err == nil {
		t.Fatal("forged low-risk maintenance claim was accepted")
	}
	if err := validateRuntimeMaintenanceClaim(expected, expected); err != nil {
		t.Fatalf("exact maintenance claim rejected: %v", err)
	}
}

func TestRuntimeHostedMaintenanceRecordsInterventionThroughPublishedGeneration(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "runtime@example.test")
	runGit(t, repo, "config", "user.name", "Runtime Test")
	writeMainTestFile(t, repo, "Wiki/index.md", "---\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeMainTestFile(t, repo, "Wiki/.openknowledge.toml", "[release]\noutputs = [\"viewer\", \"mcp\"]\n")
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\ntitle: Guide\n---\n\n# Guide\n\nOld.\n")
	insight := `---
type: Open Knowledge Insight
title: Refresh guide
description: The guide is stale.
status: draft
okf_publish: false
okf_insight_id: refresh-one
okf_insight_kind: knowledge-audit
generated:
  by: process:openknowledge-cli
  at: 2020-08-21T10:00:00Z
okf_insight_targets: [guide.md]
okf_insight_route:
  risk: low
  approval: auto
  confidence: 0.99
  owners: []
tags: [insight]
---

# Refresh guide
`
	writeMainTestFile(t, repo, "Wiki/insights/refresh.md", insight)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	base := runtimeGitTest(t, repo, "rev-parse", "HEAD")
	writeMainTestFile(t, repo, "Wiki/guide.md", "---\ntype: Guide\ntitle: Guide\n---\n\n# Guide\n\nCurrent.\n")
	writeMainTestFile(t, repo, "Wiki/insights/refresh.md", strings.Replace(insight, "status: draft", "status: stable", 1))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "refresh")
	head := runtimeGitTest(t, repo, "rev-parse", "HEAD")
	maintenance, err := runtimeMaintenanceAttestation(context.Background(), repo, base, head)
	if err != nil || maintenance == nil || maintenance.DetectedAt != "2020-08-21T10:00:00Z" {
		t.Fatalf("unexpected maintenance attestation: %#v err=%v", maintenance, err)
	}
	state := filepath.Join(t.TempDir(), "state")
	config := okruntime.Config{
		Root: repo, Runtime: okruntime.RuntimeConfig{StateDir: state},
		ArtifactStore:  okruntime.ArtifactStoreConfig{Type: "filesystem", Path: filepath.Join(state, "artifacts")},
		KnowledgeBases: []okruntime.KnowledgeBaseConfig{{ID: "docs", Path: filepath.Join(repo, "Wiki"), Spec: "0.2", Outputs: []string{okf.ReleaseOutputViewer}}},
		GitHub:         okruntime.GitHubConfig{RequiredChecks: []string{"Verify"}},
		Worker:         okruntime.WorkerConfig{ExchangeDir: filepath.Join(state, "exchange")},
	}
	productionCommit := strings.Repeat("c", 40)
	request := runtimeExchangeRequest{
		Version: 1, RunID: "run-refresh", JobID: "refresh", BaseSHA: base, HeadSHA: head,
		ProposedAt: "2020-08-21T11:00:00Z", Maintenance: maintenance,
	}
	publication := runtimeGitHubPublication{RunID: request.RunID, Commit: productionCommit, PR: 7, PRURL: "https://github.test/owner/repo/pull/7", Checked: true, Merged: true}
	if err := recordRuntimeInterventionProposal(context.Background(), config, repo, request); err != nil {
		t.Fatal(err)
	}
	events, err := knowledgeintervention.Read([]string{filepath.Join(state, "interventions")})
	if err != nil || len(events) != 2 || events[0].Stage != "detected" || events[1].Stage != "proposed" || !reflect.DeepEqual(events[0].Targets, []string{"guide.md", "insights/refresh.md"}) {
		t.Fatalf("unexpected proposed intervention lifecycle: %#v err=%v", events, err)
	}
	result, err := buildRuntimeKnowledgeGenerationWithChecks(config, config.KnowledgeBases[0], productionCommit, filepath.Join(state, "builds", "docs"), true, []string{"Verify"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeExchangeJSON(filepath.Join(config.Worker.ExchangeDir, "runs", request.RunID, "published.json"), publication); err != nil {
		t.Fatal(err)
	}
	if err := reconcileRuntimeInterventionPublication(config, "docs", productionCommit); err != nil {
		t.Fatal(err)
	}
	if err := recordRuntimeInterventionProposal(context.Background(), config, repo, request); err != nil {
		t.Fatal(err)
	}
	if err := reconcileRuntimeInterventionPublication(config, "docs", productionCommit); err != nil {
		t.Fatal(err)
	}
	events, err = knowledgeintervention.Read([]string{filepath.Join(state, "interventions")})
	if err != nil || len(events) != 3 || events[2].Stage != "published" || events[2].Publication == nil || events[2].Publication.Generation != result.Generation || !events[2].Publication.Automated || !events[2].Publication.Verified {
		t.Fatalf("unexpected published intervention lifecycle: %#v err=%v", events, err)
	}
}

func TestRuntimeLowRiskRoutePublishesReadyPRAfterRequiredChecksAndMerges(t *testing.T) {
	var draft any
	var merged bool
	previousClient := runtimeWorkerGitHubHTTPClient
	runtimeWorkerGitHubHTTPClient = &http.Client{Transport: runtimeHandlerRoundTripper{handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/repos/owner/repo/pulls":
			_, _ = response.Write([]byte(`[]`))
		case request.Method == http.MethodPost && request.URL.Path == "/repos/owner/repo/pulls":
			var payload map[string]any
			_ = json.NewDecoder(request.Body).Decode(&payload)
			draft = payload["draft"]
			response.WriteHeader(http.StatusCreated)
			_, _ = response.Write([]byte(`{"number":7,"html_url":"https://github.test/pr/7"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/repos/owner/repo/check-runs":
			response.WriteHeader(http.StatusCreated)
			_, _ = response.Write([]byte(`{}`))
		case request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/commits/"):
			_, _ = response.Write([]byte(fmt.Sprintf(`{"check_runs":[{"id":1,"name":"Verify","head_sha":"%s","status":"completed","conclusion":"success"}]}`, strings.Repeat("b", 40))))
		case request.Method == http.MethodPut && request.URL.Path == "/repos/owner/repo/pulls/7/merge":
			merged = true
			_, _ = response.Write([]byte(`{"merged":true,"message":"merged","sha":"cccccccccccccccccccccccccccccccccccccccc"}`))
		default:
			http.Error(response, "unexpected", http.StatusNotFound)
		}
	})}}
	t.Cleanup(func() { runtimeWorkerGitHubHTTPClient = previousClient })
	config := okruntime.Config{
		Worker: okruntime.WorkerConfig{ProductionBranch: "main"},
		GitHub: okruntime.GitHubConfig{Enabled: true, APIURL: "https://api.github.test", Repository: "owner/repo", Checks: true, DraftPullRequest: true, RequiredChecks: []string{"Verify"}, AutoMergeLowRisk: true},
	}
	request := runtimeExchangeRequest{
		Version: 1, RunID: "run-1", JobID: "job-1", Branch: "jobs/job-1", BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40), BundleSHA256: strings.Repeat("c", 64),
		Maintenance: &runtimeExchangeMaintenance{Risk: "low", Approval: "auto", Confidence: 0.99, Owners: []string{"github:reviewer"}, Insights: []string{"one"}, Findings: []string{}, Paths: []string{"Wiki/insights/one.md"}, ExpertTargets: []string{}, Status: "proposed"},
	}
	publication, err := publishRuntimeGitHubRequest(context.Background(), config, "secret", request, runtimeClaimReview{})
	if err != nil || !publication.Merged || publication.Commit != strings.Repeat("c", 40) || !publication.Checked || draft != false || !merged {
		t.Fatalf("unexpected low-risk publication: %#v draft=%#v merged=%v err=%v", publication, draft, merged, err)
	}
}

func TestEnsureRuntimeStateDirectorySkipsRedundantChmod(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX directory modes")
	}
	state := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	called := false
	err := ensureRuntimeStateDirectoryWith(state, func(string, os.FileMode) error {
		called = true
		return fmt.Errorf("chmod should not be called")
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("private runtime state directory was chmodded again")
	}
}

func TestEnsureRuntimeStateDirectoryTightensExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX directory modes")
	}
	state := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(state, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ensureRuntimeStateDirectory(state); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("runtime state mode = %04o, want 0700", info.Mode().Perm())
	}
}

func TestRuntimeWorkerGitAuthorizationHeaderUsesGitHubBasicCredential(t *testing.T) {
	const expected = "AUTHORIZATION: basic eC1hY2Nlc3MtdG9rZW46dGVzdC10b2tlbg=="
	if got := runtimeWorkerGitAuthorizationHeader("test-token"); got != expected {
		t.Fatalf("authorization header = %q, want %q", got, expected)
	}
}

func TestRuntimeBuildAndServeUseOnlyVerifiedPublicGeneration(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "Wiki/index.md", "# Runtime Knowledge\n\nSearchable public guidance.\n")
	writeViewerFile(t, root, "Wiki/guide.md", "---\ntype: Guide\nsources:\n  - id: private-policy\n    resource: https://example.test/private-policy\n---\n\n# Operations\n\nImmutable snapshot activation.\n")
	writeViewerFile(t, root, "private-policy.txt", "Private policy evidence.\n")
	writeViewerFile(t, root, "Wiki/draft.md", "---\ntype: Draft\nokf_publish: false\n---\n\n# Private draft\n")
	writeViewerFile(t, root, "Wiki/search-hidden.md", "---\ntype: Guide\nokf_targets:\n  search: false\n---\n\n# Search Hidden\n\nUnique forbidden search needle.\n")
	writeViewerFile(t, root, "Wiki/mcp-hidden.md", "---\ntype: Guide\nokf_targets:\n  mcp: false\n---\n\n# MCP Hidden\n\nUnique forbidden MCP needle.\n")
	writeViewerFile(t, root, "Wiki/assets/public/logo.svg", "<svg/>\n")
	writeViewerFile(t, root, "Wiki/secret.txt", "private\n")
	writeViewerFile(t, root, "Wiki/.openknowledge/agent.log", "private log\n")
	writeViewerFile(t, root, "Wiki/.openknowledge.toml", "[release]\noutputs = [\"viewer\", \"mcp\"]\n\n[publish]\nassets = [\"assets/public/**\"]\n")
	configPath := filepath.Join(root, "runtime.toml")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"

[artifact_store]
type = "filesystem"
path = "artifacts"

[serve]
address = "127.0.0.1:8080"
mcp_access = "public"
allowed_origins = ["https://allowed.example"]

[[knowledge_bases]]
id = "wiki"
path = "Wiki"
route = "/"
`)
	config, err := okruntime.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	pin, err := claimops.PinEvidence(context.Background(), claimops.EvidencePinOptions{Root: filepath.Join(root, "Wiki"), Spec: "0.2", Document: "guide.md", SourceID: "private-policy", Input: filepath.Join(root, "private-policy.txt"), CapturedAt: time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "abc123", filepath.Join(root, "generation"), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Published == nil {
		t.Fatal("expected generation promotion")
	}
	for _, included := range []string{"public/index.html", "public/search-hidden.html", "public/mcp-hidden.html", "public/assets/openknowledge/viewer-theme.js", "public/assets/openknowledge/viewer-data.js", "public/assets/openknowledge/viewer.css", "public/assets/openknowledge/viewer.js", "public/assets/public/logo.svg", "source/index.md", "source/search-hidden.md", "source/mcp-hidden.md", "source/assets/public/logo.svg", "search/index.md", "search/mcp-hidden.md", "mcp/index.md", "mcp/search-hidden.md"} {
		if _, err := os.Stat(filepath.Join(result.Output, filepath.FromSlash(included))); err != nil {
			t.Fatalf("expected %s in generation: %v", included, err)
		}
	}
	for _, excluded := range []string{"source/draft.md", "source/secret.txt", "source/.openknowledge.toml", "source/openknowledge.toml", "source/.openknowledge/agent.log", "search/search-hidden.md", "mcp/mcp-hidden.md"} {
		if _, err := os.Stat(filepath.Join(result.Output, filepath.FromSlash(excluded))); !os.IsNotExist(err) {
			t.Fatalf("expected %s outside generation, got %v", excluded, err)
		}
	}
	evidenceArtifact := filepath.Join(result.Output, "evidence", "sha256", pin.SHA256, filepath.Base(pin.Artifact))
	if content, err := os.ReadFile(evidenceArtifact); err != nil || string(content) != "Private policy evidence.\n" {
		t.Fatalf("generation-private evidence is missing: %q %v", content, err)
	}
	for _, leaked := range []string{
		filepath.Join(result.Output, "public", ".openknowledge", "evidence"),
		filepath.Join(result.Output, "source", ".openknowledge", "evidence"),
		filepath.Join(result.Output, "search", ".openknowledge", "evidence"),
		filepath.Join(result.Output, "mcp", ".openknowledge", "evidence"),
	} {
		if _, err := os.Stat(leaked); !os.IsNotExist(err) {
			t.Fatalf("private evidence leaked into a published projection: %s", leaked)
		}
	}

	handler, err := newRuntimeServeHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("unexpected activation failures: %v", failures)
	}
	index := runtimeRequest(t, handler, http.MethodGet, "/", "", nil)
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), "Runtime Knowledge") {
		t.Fatalf("unexpected viewer response %d: %s", index.Code, index.Body.String())
	}
	if !strings.Contains(index.Header().Get("Content-Security-Policy"), "script-src 'self' https:") || strings.Contains(index.Body.String(), `<script>`) {
		t.Fatalf("runtime viewer must load generated scripts under its restrictive CSP: header=%q\n%s", index.Header().Get("Content-Security-Policy"), index.Body.String())
	}
	if index.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("runtime viewer cache policy = %q, want no-cache", index.Header().Get("Cache-Control"))
	}
	viewerScript := runtimeRequest(t, handler, http.MethodGet, "/assets/openknowledge/viewer.js", "", nil)
	if viewerScript.Code != http.StatusOK || !strings.Contains(viewerScript.Header().Get("Content-Type"), "javascript") || !strings.Contains(viewerScript.Body.String(), "OpenKnowledgeStaticData") {
		t.Fatalf("unexpected viewer script response %d %q: %s", viewerScript.Code, viewerScript.Header().Get("Content-Type"), viewerScript.Body.String())
	}
	if viewerScript.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("runtime viewer script cache policy = %q, want no-cache", viewerScript.Header().Get("Cache-Control"))
	}
	search := runtimeRequest(t, handler, http.MethodGet, "/_search?q=immutable", "", nil)
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "guide.md") {
		t.Fatalf("unexpected search response %d: %s", search.Code, search.Body.String())
	}
	leakedEvidence := runtimeRequest(t, handler, http.MethodGet, "/evidence/sha256/"+pin.SHA256+"/"+filepath.Base(pin.Artifact), "", nil)
	if leakedEvidence.Code != http.StatusNotFound || strings.Contains(leakedEvidence.Body.String(), "Private policy evidence") {
		t.Fatalf("private evidence was exposed over HTTP: %d %s", leakedEvidence.Code, leakedEvidence.Body.String())
	}
	hiddenSearch := runtimeRequest(t, handler, http.MethodGet, "/_search?q=forbidden+search+needle", "", nil)
	if hiddenSearch.Code != http.StatusOK || strings.Contains(hiddenSearch.Body.String(), "search-hidden.md") {
		t.Fatalf("search=false page leaked into runtime search %d: %s", hiddenSearch.Code, hiddenSearch.Body.String())
	}
	forbidden := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`, map[string]string{"Origin": "https://evil.example"})
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected origin refusal, got %d", forbidden.Code)
	}

	initializeBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	initialize := runtimeRequest(t, handler, http.MethodPost, "/_mcp", initializeBody, nil)
	if initialize.Code != http.StatusOK || initialize.Header().Get("Mcp-Session-Id") == "" {
		t.Fatalf("unexpected MCP initialize response %d: %s", initialize.Code, initialize.Body.String())
	}
	var response mcpResponse
	if err := json.Unmarshal(initialize.Body.Bytes(), &response); err != nil || response.Error != nil {
		t.Fatalf("unexpected MCP response %#v err=%v", response, err)
	}
	session := initialize.Header().Get("Mcp-Session-Id")
	notification := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, map[string]string{"Mcp-Session-Id": session})
	if notification.Code != http.StatusAccepted {
		t.Fatalf("expected accepted notification, got %d", notification.Code)
	}
	tools := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, map[string]string{"Mcp-Session-Id": session})
	if tools.Code != http.StatusOK || !strings.Contains(tools.Body.String(), "openknowledge_search") {
		t.Fatalf("unexpected MCP tools response %d: %s", tools.Code, tools.Body.String())
	}
	mcpSearch := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":21,"method":"tools/call","params":{"name":"openknowledge_search","arguments":{"query":"forbidden search needle"}}}`, map[string]string{"Mcp-Session-Id": session})
	if mcpSearch.Code != http.StatusOK || !strings.Contains(mcpSearch.Body.String(), "search-hidden.md") || strings.Contains(mcpSearch.Body.String(), "mcp-hidden.md") {
		t.Fatalf("runtime MCP search did not preserve its target projection %d: %s", mcpSearch.Code, mcpSearch.Body.String())
	}
	resources := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}`, map[string]string{"Mcp-Session-Id": session})
	if resources.Code != http.StatusOK || strings.Contains(resources.Body.String(), "mcp-hidden.md") || !strings.Contains(resources.Body.String(), "search-hidden.md") {
		t.Fatalf("unexpected MCP target projection %d: %s", resources.Code, resources.Body.String())
	}
}

func TestRuntimePublicationBindsRequiredChecksIntoGeneration(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "# Checked knowledge\n")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`)
	config, err := okruntime.LoadConfig(filepath.Join(root, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	config.GitHub.RequiredChecks = []string{"Knowledge Eval", "Verify"}
	config.Runtime.ReleasePolicy = okruntime.ReleasePolicyLastPassing
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "abc123", filepath.Join(root, "unattested"), true); err == nil || !strings.Contains(err.Error(), "requires verified GitHub checks") {
		t.Fatalf("expected unattested publication refusal, got %v", err)
	}
	config.Runtime.ReleasePolicy = okruntime.ReleasePolicyFollowMain
	degraded, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "abc123", filepath.Join(root, "degraded"), true)
	if err != nil || degraded.Health != okruntime.GenerationHealthDegraded || degraded.Published == nil {
		t.Fatalf("follow-main did not activate buildable degraded generation: result=%#v err=%v", degraded, err)
	}
	result, err := buildRuntimeKnowledgeGenerationWithChecks(config, config.KnowledgeBases[0], "abc123", filepath.Join(root, "attested"), true, []string{"Knowledge Eval", "Verify"})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := okruntime.LoadAndValidateGeneration(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Health != okruntime.GenerationHealthPassing || !reflect.DeepEqual(manifest.Checks, config.GitHub.RequiredChecks) {
		t.Fatalf("generation did not bind required checks: %#v", manifest.Checks)
	}
}

func TestRuntimeKnowledgeCIPassPersistsReportsAndBlocksChangedSource(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "Wiki")
	writeViewerFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeViewerFile(t, root, "policy.md", "---\ntype: Source\ntitle: API policy\nowner: team:platform\n---\n\n# Policy\n\nUse /v1/users.\n")
	writeViewerFile(t, root, "runbook.md", "---\ntype: Runbook\ntitle: Recovery\nowner: team:platform\nsources:\n  - id: api-policy\n    resource: policy.md\n    role: authoritative\n    authority_approved_by: human:lead\n---\n\n# Recovery\n\nUse /v1/users.\n")
	writeViewerFile(t, repo, ".openknowledge/evals/knowledge.yaml", "type: openknowledge.eval\nversion: 1\nid: runtime-ci\ncases:\n  - id: endpoint\n    question: How does user recovery work?\n    expect:\n      sources: [runbook.md]\n      evidence_contains: [/v1/users]\n")
	_, baseline, err := knowledgeaudit.Run(knowledgeaudit.Options{Root: root, Spec: "latest"})
	if err != nil {
		t.Fatal(err)
	}
	baselineContent, err := knowledgeaudit.EncodeBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, repo, ".openknowledge/audit-sources.json", string(baselineContent))
	config := okruntime.Config{
		Runtime:       okruntime.RuntimeConfig{StateDir: filepath.Join(repo, "state"), RequireResolvedClaims: true},
		ArtifactStore: okruntime.ArtifactStoreConfig{Type: "filesystem", Path: filepath.Join(repo, "artifacts")},
		Worker:        okruntime.WorkerConfig{KnowledgeCI: true},
	}
	knowledge := okruntime.KnowledgeBaseConfig{ID: "wiki", Path: root, Spec: "latest", Outputs: []string{okf.ReleaseOutputViewer}}
	commit := strings.Repeat("a", 40)
	if err := runtimeKnowledgeCIPass(config, repo, knowledge, commit); err != nil {
		t.Fatal(err)
	}
	reportDir := filepath.Join(config.Runtime.StateDir, "reports", "knowledge-ci", commit, "wiki")
	for _, name := range []string{"index.md", "artifact.json", "validation.json", "claims-validation.json", "audit.json", "audit.md", "eval.json", "eval.md"} {
		if _, err := os.Stat(filepath.Join(reportDir, name)); err != nil {
			t.Fatalf("missing runtime CI report %s: %v", name, err)
		}
	}
	writeViewerFile(t, root, "policy.md", "---\ntype: Source\ntitle: API policy\nowner: team:platform\n---\n\n# Policy\n\nUse /v2/users.\n")
	if err := runtimeKnowledgeCIPass(config, repo, knowledge, strings.Repeat("b", 40)); err == nil || !strings.Contains(err.Error(), "audit high-severity") {
		t.Fatalf("expected changed source to block runtime CI, got %v", err)
	} else {
		var gateErr *runtimeQualityGateError
		if !errors.As(err, &gateErr) {
			t.Fatalf("quality failure was not classified for follow-main: %T", err)
		}
	}
}

func TestRuntimePublicationRetainsGreenGenerationForUnresolvedClaims(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Knowledge\n")
	writeViewerFile(t, root, "Wiki/auth.md", mainTypedClaimDocument("okn:claim/token-format/1", "proposed", "JWT"))
	writeViewerFile(t, root, "Wiki/handbook.md", "---\ntype: Source\n---\n\n# Handbook\n")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"
require_resolved_claims = true
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`)
	config, err := okruntime.LoadConfig(filepath.Join(root, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	config.Runtime.ReleasePolicy = okruntime.ReleasePolicyLastPassing
	if _, err := buildRuntimeKnowledgeGenerationWithChecks(config, config.KnowledgeBases[0], "abc123", filepath.Join(root, "candidate"), true, nil); err == nil || !strings.Contains(err.Error(), "unresolved claim") {
		t.Fatalf("expected unresolved release refusal, got %v", err)
	}
	config.Runtime.ReleasePolicy = okruntime.ReleasePolicyFollowMain
	result, err := buildRuntimeKnowledgeGenerationWithChecks(config, config.KnowledgeBases[0], "abc123", filepath.Join(root, "degraded-claim-release"), true, nil)
	if err != nil || result.Health != okruntime.GenerationHealthDegraded || result.Published == nil {
		t.Fatalf("follow-main did not publish unresolved claim generation as degraded: result=%#v err=%v", result, err)
	}
	config.Runtime.RequireResolvedClaims = false
	if _, err := buildRuntimeKnowledgeGenerationWithChecks(config, config.KnowledgeBases[0], "abc123", filepath.Join(root, "partial-release"), true, nil); err != nil {
		t.Fatalf("explicit partial-release policy must allow runtime refusal semantics: %v", err)
	}
}

func TestRuntimeRetrievalPolicyEnrichesAndFiltersHTTPAndMCP(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Runtime policy\n")
	writeViewerFile(t, root, "trusted.md", `---
type: Guide
title: Trusted policy
status: stable
stale_after: 2027-12-31T00:00:00Z
verified: { by: human:reviewer, at: 2026-08-20T10:00:00Z }
sources:
  - id: handbook
    resource: https://example.test/handbook
---

# Trusted policy

Runtime selection policy guidance.
`)
	writeViewerFile(t, root, "draft.md", `---
type: Guide
title: Draft policy
status: draft
stale_after: 2026-01-01T00:00:00Z
---

# Draft policy

Runtime selection policy draft. Quarantine zebra protocol.
`)
	index, err := okf.BuildContextIndexWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	policy := okruntime.RetrievalPolicyConfig{MinimumTrust: okf.OKFV02TrustMachineConfirmed, AllowStale: false, AllowedStatuses: []string{"stable"}, RequireSources: true}
	usageRoot := filepath.Join(t.TempDir(), "usage")
	usageRecorder, err := knowledgeusage.NewRecorder(usageRoot, true, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	feedbackRoot := filepath.Join(t.TempDir(), "feedback")
	feedbackRecorder, err := knowledgefeedback.NewRecorder(feedbackRoot, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	knowledge := okruntime.KnowledgeBaseConfig{ID: "wiki", Route: "/", Outputs: []string{okf.ReleaseOutputMCP, okf.ReleaseOutputViewer}}
	snapshot := runtimeGenerationSnapshot{
		Knowledge: knowledge,
		Pointer:   okruntime.ActivePointer{Generation: "generation-7"},
		Manifest:  okruntime.GenerationManifest{Commit: "abc123", Spec: "0.2", ContentDigest: strings.Repeat("a", 64), Checks: []string{"Knowledge Eval"}},
		Root:      root,
		Search:    index,
		MCP:       index,
	}
	handler := &runtimeServeHandler{
		config:    okruntime.Config{Serve: okruntime.ServeConfig{MCPAccess: "public", RetrievalPolicy: policy}, KnowledgeBases: []okruntime.KnowledgeBaseConfig{knowledge}},
		snapshots: &runtimeSnapshotManager{active: map[string]runtimeGenerationSnapshot{"wiki": snapshot}},
		semaphore: make(chan struct{}, 4), sessions: make(map[string]*runtimeMCPSession),
		now:   func() time.Time { return time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC) },
		usage: usageRecorder, feedback: feedbackRecorder,
	}

	search := runtimeRequest(t, handler, http.MethodGet, "/_search?q=selection+policy&limit=5", "", nil)
	if search.Code != http.StatusOK {
		t.Fatalf("search code=%d body=%s", search.Code, search.Body.String())
	}
	var searchResult runtimeSearchResponse
	if err := json.Unmarshal(search.Body.Bytes(), &searchResult); err != nil {
		t.Fatal(err)
	}
	if len(searchResult.Results) != 1 || searchResult.Results[0].Source.Path != "trusted.md" || searchResult.Results[0].Trust.Tier != okf.OKFV02TrustHumanReviewed {
		t.Fatalf("unexpected selected results: %#v", searchResult.Results)
	}
	if len(searchResult.UsageEventID) != 32 {
		t.Fatalf("runtime response did not expose a feedback-safe usage id: %#v", searchResult)
	}
	feedbackResponse := runtimeRequest(t, handler, http.MethodPost, "/_feedback", `{"usageEventId":"`+searchResult.UsageEventID+`","sentiment":"negative","reasons":["outdated"]}`, nil)
	if feedbackResponse.Code != http.StatusCreated || !strings.Contains(feedbackResponse.Body.String(), `"path":"trusted.md"`) || !strings.Contains(feedbackResponse.Body.String(), `"sentiment":"negative"`) {
		t.Fatalf("unexpected runtime feedback response: %d %s", feedbackResponse.Code, feedbackResponse.Body.String())
	}
	selected := searchResult.Results[0]
	if selected.Provenance.Generation.Name != "generation-7" || !reflect.DeepEqual(selected.Provenance.Generation.Checks, []string{"Knowledge Eval"}) || len(selected.Provenance.Sources) != 1 || selected.Freshness.EvaluatedAt != "2026-08-21T12:00:00Z" || len(selected.Selection.Reasons) == 0 {
		t.Fatalf("missing runtime retrieval metadata: %#v", selected)
	}
	if len(searchResult.Rejected) != 1 || searchResult.Rejected[0].Path != "draft.md" || !reflect.DeepEqual(searchResult.Rejected[0].Reasons, []string{"trust_below_minimum", "stale", "status_not_allowed", "sources_required"}) {
		t.Fatalf("unexpected policy rejections: %#v", searchResult.Rejected)
	}
	refusal := buildRuntimeSearchResponse(snapshot, policy, "quarantine zebra", 5, handler.now())
	if refusal.Decision != "refuse" || !reflect.DeepEqual(refusal.RefusalReasons, []string{"no_policy_compliant_evidence"}) || len(refusal.Results) != 0 || len(refusal.Rejected) == 0 {
		t.Fatalf("runtime did not explicitly refuse unsupported evidence: %#v", refusal)
	}

	initialize := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, nil)
	session := initialize.Header().Get("Mcp-Session-Id")
	if initialize.Code != http.StatusOK || session == "" {
		t.Fatalf("MCP initialize code=%d body=%s", initialize.Code, initialize.Body.String())
	}
	runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, map[string]string{"Mcp-Session-Id": session})
	tool := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"openknowledge_search","arguments":{"query":"selection policy","limit":5}}}`, map[string]string{"Mcp-Session-Id": session})
	if tool.Code != http.StatusOK || !strings.Contains(tool.Body.String(), `"knowledgeBase":"wiki"`) || !strings.Contains(tool.Body.String(), `"trust_below_minimum"`) || !strings.Contains(tool.Body.String(), `"route":["bm25","vector","policy_filter","rerank"]`) || !strings.Contains(tool.Body.String(), `"permissionsApplied":["profile:public"]`) || strings.Count(tool.Body.String(), `"path":"draft.md"`) == 0 {
		t.Fatalf("runtime MCP did not expose the policy-aware context contract: %d %s", tool.Code, tool.Body.String())
	}
	events, err := knowledgeusage.Read([]string{usageRoot})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Channel != "http-search" || events[1].Channel != "mcp-search" || events[0].Query != "selection policy" || !reflect.DeepEqual(events[0].Generation.Checks, []string{"Knowledge Eval"}) || len(events[0].Selected) != 1 || events[0].Selected[0].Path != "trusted.md" {
		t.Fatalf("unexpected runtime usage events: %#v", events)
	}
	feedbackEvents, err := knowledgefeedback.Read([]string{feedbackRoot})
	if err != nil || len(feedbackEvents) != 1 || feedbackEvents[0].UsageEventID != events[0].ID || feedbackEvents[0].Evidence[0].Path != "trusted.md" {
		t.Fatalf("unexpected grounded feedback events: %#v err=%v", feedbackEvents, err)
	}
}

func TestRuntimeEvidenceBundleProjectsClaimsConflictsAndMissingKnowledge(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	if conflicts := runtimeClaimConflicts(nil, now); conflicts == nil {
		t.Fatal("runtime conflicts must encode an empty result as [] instead of null")
	}
	left := okf.Claim{ID: "okn:claim/timeout/one", Slot: "okn:slot/timeout", Subject: "okn:service/api", Predicate: "okn:timeout", Object: okf.ClaimObject{Value: 30, Datatype: "xsd:integer"}, Status: "verified"}
	right := okf.Claim{ID: "okn:claim/timeout/two", Slot: left.Slot, Subject: left.Subject, Predicate: left.Predicate, Object: okf.ClaimObject{Value: 60, Datatype: "xsd:integer"}, Status: "verified", Relations: okf.ClaimRelations{Contradicts: []string{left.ID}}}
	response := runtimeContextResponse{
		Route: []string{"bm25", "vector", "policy_filter", "rerank"}, Decision: "answer", RefusalReasons: []string{}, Rejected: []runtimeRejectedCandidate{},
		Sources: []runtimeContextSource{{Source: okf.ContextSource{Relation: "outgoing-link", ClaimProfile: &okf.ClaimProfileSignals{Profile: okf.ClaimProfileIDV1, Claims: []okf.Claim{right, left}, ClaimRefs: []string{}}}}},
		Claims:  []okf.Claim{}, Conflicts: []runtimeEvidenceConflict{}, MissingKnowledge: []runtimeMissingKnowledge{},
	}
	populateRuntimeEvidenceBundle(&response, now)
	if !reflect.DeepEqual(response.Route, []string{"bm25", "vector", "policy_filter", "rerank", "link_expansion", "claim_projection"}) || len(response.Claims) != 2 || response.Claims[0].ID != left.ID {
		t.Fatalf("unexpected evidence bundle claims: %#v", response)
	}
	if len(response.Conflicts) != 2 || response.Conflicts[0].Kind != "explicit_contradiction" || response.Conflicts[1].Kind != "incompatible_values" {
		t.Fatalf("expected explicit and typed-value conflicts: %#v", response.Conflicts)
	}
	refusal := runtimeContextResponse{Decision: "refuse", RefusalReasons: []string{"no_relevant_evidence"}, Claims: []okf.Claim{}, Conflicts: []runtimeEvidenceConflict{}, MissingKnowledge: []runtimeMissingKnowledge{}, Sources: []runtimeContextSource{}}
	populateRuntimeEvidenceBundle(&refusal, now)
	if len(refusal.MissingKnowledge) != 1 || refusal.MissingKnowledge[0].Kind != "no_relevant_evidence" {
		t.Fatalf("refusal did not expose missing knowledge: %#v", refusal)
	}
}

func TestRuntimeRetrievalRejectsDisputedSparseClaim(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Runtime claims\n")
	writeViewerFile(t, root, "auth.md", mainTypedClaimDocument("okn:claim/token-format/1", "disputed", "JWT"))
	index, err := okf.BuildContextIndexWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	var section okf.ContextSection
	for _, candidate := range index.Sections {
		if candidate.Path == "auth.md" {
			section = candidate
			break
		}
	}
	if section.Path == "" {
		t.Fatal("missing auth retrieval section")
	}
	snapshot := runtimeGenerationSnapshot{
		Knowledge: okruntime.KnowledgeBaseConfig{ID: "wiki"},
		Pointer:   okruntime.ActivePointer{Generation: "generation-claims"},
		Manifest:  okruntime.GenerationManifest{Commit: "abc123", Spec: "0.2", ContentDigest: strings.Repeat("a", 64)},
	}
	policy := okruntime.RetrievalPolicyConfig{MinimumTrust: okf.OKFV02TrustUnverified, AllowStale: true, AllowedStatuses: []string{"stable"}}
	_, rejected := runtimeMetadata(snapshot, section, policy, publicRuntimeAccess(), time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if !reflect.DeepEqual(rejected, []string{"claim_disputed"}) {
		t.Fatalf("expected disputed claim rejection, got %v", rejected)
	}
}

func TestRuntimeRetrievalServesVerifiedReplacementAndPreservesDeprecatedHistory(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Runtime claims\n")
	writeViewerFile(t, root, "auth.md", strings.Replace(mainTypedClaimDocument("okn:claim/token-format/2", "verified", "opaque"), "claims:\n", `claims:
  - id: okn:claim/token-format/1
    slot: okn:slot/token-format
    subject: okn:service/auth
    predicate: auth:tokenFormat
    object: {value: JWT, datatype: xsd:string}
    evidence: [{id: okn:evidence/token-format/old, source_ref: identity-openapi, stance: supports, role: primary}]
    status: superseded
`, 1))
	index, err := okf.BuildContextIndexWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	var section okf.ContextSection
	for _, candidate := range index.Sections {
		if candidate.Path == "auth.md" {
			section = candidate
			break
		}
	}
	snapshot := runtimeGenerationSnapshot{
		Knowledge: okruntime.KnowledgeBaseConfig{ID: "wiki"},
		Pointer:   okruntime.ActivePointer{Generation: "generation-claims"},
		Manifest:  okruntime.GenerationManifest{Commit: "abc123", Spec: "0.2", ContentDigest: strings.Repeat("a", 64)},
	}
	policy := okruntime.RetrievalPolicyConfig{MinimumTrust: okf.OKFV02TrustUnverified, AllowStale: true, AllowedStatuses: []string{"stable"}, RequireSources: true}
	_, rejected := runtimeMetadata(snapshot, section, policy, publicRuntimeAccess(), time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if len(rejected) != 0 {
		t.Fatalf("expected verified replacement to remain servable while history is preserved, got %v", rejected)
	}
}

func TestRuntimeAccessProfilesAuthorizeAndRouteRetrieval(t *testing.T) {
	root := t.TempDir()
	writeViewerFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Home\n")
	writeViewerFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Support guide\nsources:\n  - id: private-runbook\n    resource: https://example.test/private-runbook\n    access: [profile:support]\n---\n\n# Support guide\n\nReset the customer account.\n")
	index, err := okf.BuildContextIndexWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	policy := okruntime.RetrievalPolicyConfig{MinimumTrust: "unverified", AllowStale: true, AllowedStatuses: []string{"draft", "stable", "deprecated"}}
	knowledge := okruntime.KnowledgeBaseConfig{ID: "wiki", Route: "/", Outputs: []string{okf.ReleaseOutputMCP, okf.ReleaseOutputViewer}}
	snapshot := runtimeGenerationSnapshot{Knowledge: knowledge, Pointer: okruntime.ActivePointer{Generation: "generation-1"}, Manifest: okruntime.GenerationManifest{Commit: "abc", Spec: "0.2", ContentDigest: strings.Repeat("a", 64)}, Root: root, Search: index, MCP: index}
	support := runtimeAccessProfile{config: okruntime.AccessProfileConfig{ID: "support", KnowledgeBases: []string{"wiki"}, Agents: []string{"support-agent"}, Teams: []string{"support"}, UseCases: []string{"customer-support"}}, token: strings.Repeat("s", 32)}
	viewer := runtimeAccessProfile{config: okruntime.AccessProfileConfig{ID: "viewer", KnowledgeBases: []string{"wiki"}, Agents: []string{"viewer-agent"}}, token: strings.Repeat("v", 32)}
	admin := runtimeAccessProfile{config: okruntime.AccessProfileConfig{ID: "admin", KnowledgeBases: []string{"admin"}, Teams: []string{"admin"}}, token: strings.Repeat("a", 32)}
	handler := &runtimeServeHandler{
		config:    okruntime.Config{Serve: okruntime.ServeConfig{MCPAccess: "token", RetrievalPolicy: policy}, KnowledgeBases: []okruntime.KnowledgeBaseConfig{knowledge}},
		snapshots: &runtimeSnapshotManager{active: map[string]runtimeGenerationSnapshot{"wiki": snapshot}}, semaphore: make(chan struct{}, 4),
		profiles: []runtimeAccessProfile{admin, support, viewer}, sessions: make(map[string]*runtimeMCPSession), now: time.Now,
	}
	disabledFeedback := runtimeRequest(t, handler, http.MethodPost, "/_feedback", `{}`, nil)
	if disabledFeedback.Code != http.StatusNotFound {
		t.Fatalf("disabled feedback endpoint exposed authentication state: %d %s", disabledFeedback.Code, disabledFeedback.Body.String())
	}
	for name, test := range map[string]struct {
		headers map[string]string
		want    int
	}{
		"missing":   {nil, http.StatusUnauthorized},
		"unknown":   {map[string]string{"Authorization": "Bearer " + strings.Repeat("x", 32)}, http.StatusUnauthorized},
		"forbidden": {map[string]string{"Authorization": "Bearer " + admin.token}, http.StatusForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			response := runtimeRequest(t, handler, http.MethodGet, "/_search?q=customer", "", test.headers)
			if response.Code != test.want {
				t.Fatalf("code=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
	supportHeaders := map[string]string{"Authorization": "Bearer " + support.token}
	search := runtimeRequest(t, handler, http.MethodGet, "/_search?q=customer", "", supportHeaders)
	var result runtimeSearchResponse
	if search.Code != http.StatusOK || json.Unmarshal(search.Body.Bytes(), &result) != nil || result.Access.Profile != "support" || !reflect.DeepEqual(result.Access.Agents, []string{"support-agent"}) {
		t.Fatalf("unexpected routed search: code=%d result=%#v body=%s", search.Code, result, search.Body.String())
	}
	viewerSearch := runtimeRequest(t, handler, http.MethodGet, "/_search?q=customer", "", map[string]string{"Authorization": "Bearer " + viewer.token})
	var viewerResult runtimeSearchResponse
	if viewerSearch.Code != http.StatusOK || json.Unmarshal(viewerSearch.Body.Bytes(), &viewerResult) != nil || viewerResult.Decision != "refuse" || len(viewerResult.Rejected) != 1 || !reflect.DeepEqual(viewerResult.Rejected[0].Reasons, []string{"source_access_denied"}) {
		t.Fatalf("restricted source leaked to viewer profile: code=%d result=%#v body=%s", viewerSearch.Code, viewerResult, viewerSearch.Body.String())
	}
	initialize := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, supportHeaders)
	session := initialize.Header().Get("Mcp-Session-Id")
	if initialize.Code != http.StatusOK || session == "" {
		t.Fatalf("unexpected profile MCP initialize: %d %s", initialize.Code, initialize.Body.String())
	}
	changedProfile := runtimeRequest(t, handler, http.MethodPost, "/_mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, map[string]string{"Authorization": "Bearer " + viewer.token, "Mcp-Session-Id": session})
	if changedProfile.Code != http.StatusForbidden {
		t.Fatalf("profile-switched session was accepted: %d %s", changedProfile.Code, changedProfile.Body.String())
	}
	handler.preview = true
	preview := runtimeRequest(t, handler, http.MethodGet, "/_search?q=customer", "", supportHeaders)
	if preview.Header().Get("X-OpenKnowledge-Preview") != "true" || preview.Header().Get("X-OpenKnowledge-Generation") != "generation-1" {
		t.Fatalf("preview generation headers are missing: %#v", preview.Header())
	}
}

func TestRuntimeReleaseCommandsStagePreviewPinAndRollback(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "# First release\n\nAlpha knowledge.\n")
	configPath := filepath.Join(root, "runtime.toml")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`)
	build := func(commit string, extra ...string) runtimeBuildResult {
		args := []string{"--config", configPath, "--commit", commit}
		args = append(args, extra...)
		stdout, stderr, code := captureMainOutput(t, func() int { return runRuntimeBuild(args) })
		if code != 0 {
			t.Fatalf("build %s failed: code=%d stderr=%s", commit, code, stderr)
		}
		var result runtimeBuildResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := build("first")
	writeViewerFile(t, root, "Wiki/index.md", "# Second release\n\nBeta knowledge.\n")
	second := build("second", "--stage")
	if !second.Staged || second.Published != nil || second.Generation == first.Generation {
		t.Fatalf("unexpected staged build: first=%#v second=%#v", first, second)
	}
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runRuntimeReleases([]string{"--config", configPath})
	})
	var releases runtimeReleasesResult
	if code != 0 || json.Unmarshal([]byte(stdout), &releases) != nil || len(releases.Releases) != 2 || releases.ActiveGeneration != first.Generation {
		t.Fatalf("unexpected releases: code=%d stderr=%s result=%#v", code, stderr, releases)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runRuntimePreview([]string{"--config", configPath, "--generation", second.Generation, "--check"})
	})
	var preview runtimeReleaseActionResult
	if code != 0 || json.Unmarshal([]byte(stdout), &preview) != nil || preview.Action != "preview" || preview.Generation != second.Generation {
		t.Fatalf("unexpected preview check: code=%d stderr=%s result=%#v", code, stderr, preview)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runRuntimeCache([]string{"rebuild", "--config", configPath, "--generation", second.Generation})
	})
	var rebuilt runtimeCacheResult
	if code != 0 || json.Unmarshal([]byte(stdout), &rebuilt) != nil || rebuilt.Action != "rebuild" || len(rebuilt.Entries) != 2 || rebuilt.Entries[0].State != "rebuilt" || rebuilt.Entries[1].State != "rebuilt" {
		t.Fatalf("unexpected cache rebuild: code=%d stderr=%s result=%#v", code, stderr, rebuilt)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runRuntimeCache([]string{"status", "--config", configPath, "--generation", second.Generation})
	})
	var cacheStatus runtimeCacheResult
	if code != 0 || json.Unmarshal([]byte(stdout), &cacheStatus) != nil || len(cacheStatus.Entries) != 2 || cacheStatus.Entries[0].State != "ready" || cacheStatus.Entries[1].State != "ready" {
		t.Fatalf("unexpected cache status: code=%d stderr=%s result=%#v", code, stderr, cacheStatus)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runRuntimeCache([]string{"prune", "--config", configPath})
	})
	var prunePreview runtimeCacheResult
	if code != 0 || json.Unmarshal([]byte(stdout), &prunePreview) != nil || prunePreview.Applied == nil || *prunePreview.Applied || len(prunePreview.Removed) != 0 {
		t.Fatalf("unexpected cache prune preview: code=%d stderr=%s result=%#v", code, stderr, prunePreview)
	}
	config, err := okruntime.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
	if active, _, err := store.Active("wiki"); err != nil || active.Generation != first.Generation {
		t.Fatalf("preview changed production pin: %#v err=%v", active, err)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runRuntimePin([]string{"--config", configPath, "--generation", second.Generation})
	})
	var pinned runtimeReleaseActionResult
	if code != 0 || json.Unmarshal([]byte(stdout), &pinned) != nil || pinned.Action != "pin" || pinned.PreviousGeneration != first.Generation || pinned.Generation != second.Generation {
		t.Fatalf("unexpected pin: code=%d stderr=%s result=%#v", code, stderr, pinned)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return runRuntimeRollback([]string{"--config", configPath})
	})
	var rolledBack runtimeReleaseActionResult
	if code != 0 || json.Unmarshal([]byte(stdout), &rolledBack) != nil || rolledBack.Action != "rollback" || rolledBack.PreviousGeneration != second.Generation || rolledBack.Generation != first.Generation {
		t.Fatalf("unexpected rollback: code=%d stderr=%s result=%#v", code, stderr, rolledBack)
	}
}

func TestRuntimeAccessProfilesReplaceLegacyMCPToken(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SUPPORT_KNOWLEDGE_TOKEN", strings.Repeat("s", 32))
	config := okruntime.Config{
		Runtime:       okruntime.RuntimeConfig{StateDir: filepath.Join(root, "state")},
		ArtifactStore: okruntime.ArtifactStoreConfig{Type: "filesystem", Path: filepath.Join(root, "artifacts")},
		Serve: okruntime.ServeConfig{
			MCPAccess:      "token",
			MCPTokenEnv:    "LEGACY_TOKEN_IS_INTENTIONALLY_UNSET",
			MaxConcurrency: 1,
		},
		KnowledgeBases: []okruntime.KnowledgeBaseConfig{{ID: "wiki", Outputs: []string{okf.ReleaseOutputMCP, okf.ReleaseOutputViewer}}},
		AccessProfiles: []okruntime.AccessProfileConfig{{
			ID: "support", TokenEnv: "SUPPORT_KNOWLEDGE_TOKEN", KnowledgeBases: []string{"wiki"}, Agents: []string{"support-agent"},
		}},
	}
	if _, err := newRuntimeServeHandler(config); err != nil {
		t.Fatalf("profile-backed runtime unexpectedly required legacy MCP token: %v", err)
	}
	t.Setenv("SECOND_KNOWLEDGE_TOKEN", strings.Repeat("s", 32))
	config.AccessProfiles = append(config.AccessProfiles, okruntime.AccessProfileConfig{
		ID: "second", TokenEnv: "SECOND_KNOWLEDGE_TOKEN", KnowledgeBases: []string{"wiki"}, Teams: []string{"support"},
	})
	if _, err := newRuntimeServeHandler(config); err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("expected duplicate profile token refusal, got %v", err)
	}
}

func TestRuntimeServeCachesSearchIndexPerGeneration(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "# Cached generation v1\n\nAlpha search needle.\n")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[serve]
max_concurrency = 16
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
route = "/"
`)
	config, err := okruntime.LoadConfig(filepath.Join(root, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "first", filepath.Join(root, "first"), true); err != nil {
		t.Fatal(err)
	}
	handler, err := newRuntimeServeHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	var indexBuilds atomic.Int32
	handler.snapshots.buildSearchIndex = func(root string, version string, evidenceRoot string) (okf.ContextIndex, error) {
		indexBuilds.Add(1)
		return okf.BuildContextIndexWithVersion(root, version)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("first activation failed: %v", failures)
	}
	if count := indexBuilds.Load(); count != 1 {
		t.Fatalf("search index builds after first activation = %d, want 1", count)
	}
	cachedHandler, err := newRuntimeServeHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	var redundantBuilds atomic.Int32
	cachedHandler.snapshots.buildSearchIndex = func(root string, version string, evidenceRoot string) (okf.ContextIndex, error) {
		redundantBuilds.Add(1)
		return okf.BuildContextIndexWithVersion(root, version)
	}
	if failures := cachedHandler.snapshots.refresh(); len(failures) != 0 || redundantBuilds.Load() != 0 {
		t.Fatalf("persistent index cache was not reused: failures=%v builds=%d", failures, redundantBuilds.Load())
	}
	before, ok := handler.snapshots.snapshot("wiki")
	if !ok || before.Search.Revision.IndexSHA256 == "" {
		t.Fatalf("expected activated snapshot to contain a search index: %#v", before)
	}

	// Generation directories are immutable in production. Mutating this test
	// fixture after activation proves requests use the snapshot's built index
	// instead of reparsing the projection on every search.
	searchPath := filepath.Join(before.Root, "search", "index.md")
	originalSearch, err := os.ReadFile(searchPath)
	if err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, before.Root, "search/index.md", "# Mutated after activation\n\nUncached mutation needle.\n")
	alpha := runtimeRequest(t, handler, http.MethodGet, "/_search?q=alpha", "", nil)
	var alphaResult okf.SearchResultSet
	if err := json.Unmarshal(alpha.Body.Bytes(), &alphaResult); err != nil {
		t.Fatal(err)
	}
	if alpha.Code != http.StatusOK || len(alphaResult.Results) == 0 {
		t.Fatalf("cached v1 search failed %d: %s", alpha.Code, alpha.Body.String())
	}
	mutated := runtimeRequest(t, handler, http.MethodGet, "/_search?q=uncached", "", nil)
	var mutatedResult okf.SearchResultSet
	if err := json.Unmarshal(mutated.Body.Bytes(), &mutatedResult); err != nil {
		t.Fatal(err)
	}
	if mutated.Code != http.StatusOK || len(mutatedResult.Results) != 0 {
		t.Fatalf("post-activation filesystem mutation reached cached search %d: %s", mutated.Code, mutated.Body.String())
	}
	if count := indexBuilds.Load(); count != 1 {
		t.Fatalf("search requests rebuilt snapshot index: build count = %d, want 1", count)
	}
	if err := os.WriteFile(searchPath, originalSearch, 0644); err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("unchanged refresh failed: %v", failures)
	}
	if count := indexBuilds.Load(); count != 1 {
		t.Fatalf("unchanged refresh rebuilt snapshot index: build count = %d, want 1", count)
	}

	writeViewerFile(t, root, "Wiki/index.md", "# Cached generation v2\n\nBeta search needle.\n")
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "second", filepath.Join(root, "second"), true); err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("second activation failed: %v", failures)
	}
	if count := indexBuilds.Load(); count != 2 {
		t.Fatalf("search index builds after second activation = %d, want 2", count)
	}
	after, ok := handler.snapshots.snapshot("wiki")
	if !ok || after.Pointer.ContentDigest == before.Pointer.ContentDigest || after.Search.Revision == before.Search.Revision {
		t.Fatalf("expected second generation to replace the cached search index: before=%#v after=%#v", before, after)
	}
	beta := runtimeRequest(t, handler, http.MethodGet, "/_search?q=beta", "", nil)
	var betaResult okf.SearchResultSet
	if err := json.Unmarshal(beta.Body.Bytes(), &betaResult); err != nil {
		t.Fatal(err)
	}
	if beta.Code != http.StatusOK || len(betaResult.Results) == 0 {
		t.Fatalf("cached v2 search failed %d: %s", beta.Code, beta.Body.String())
	}

	const requests = 8
	var wait sync.WaitGroup
	errors := make(chan error, requests)
	for index := 0; index < requests; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			request := httptest.NewRequest(http.MethodGet, "/_search?q=beta", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "index.md") {
				errors <- fmt.Errorf("concurrent search response %d: %s", response.Code, response.Body.String())
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if count := indexBuilds.Load(); count != 2 {
		t.Fatalf("concurrent search requests rebuilt snapshot index: build count = %d, want 2", count)
	}

	writeViewerFile(t, root, "Wiki/index.md", "# Cached generation v3\n\nGamma search needle.\n")
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "third", filepath.Join(root, "third"), true); err != nil {
		t.Fatal(err)
	}
	handler.snapshots.buildSearchIndex = func(string, string, string) (okf.ContextIndex, error) {
		indexBuilds.Add(1)
		return okf.ContextIndex{}, fmt.Errorf("injected index build failure")
	}
	if failures := handler.snapshots.refresh(); len(failures) == 0 || !strings.Contains(failures[0].Error(), "search index") {
		t.Fatalf("expected search index activation failure, got %v", failures)
	}
	retained, ok := handler.snapshots.snapshot("wiki")
	if !ok || retained.Pointer.ContentDigest != after.Pointer.ContentDigest || retained.Search.Revision != after.Search.Revision {
		t.Fatalf("failed index build replaced last valid snapshot: before=%#v after=%#v", after, retained)
	}
	if count := indexBuilds.Load(); count != 3 {
		t.Fatalf("search index builds after rejected generation = %d, want 3", count)
	}
}

func TestRuntimeServeRetainsLastValidGeneration(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "# Stable\n")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`)
	config, err := okruntime.LoadConfig(filepath.Join(root, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildRuntimeKnowledgeGeneration(config, config.KnowledgeBases[0], "stable", filepath.Join(root, "generation"), true); err != nil {
		t.Fatal(err)
	}
	manager := newRuntimeSnapshotManager(config)
	if failures := manager.refresh(); len(failures) != 0 {
		t.Fatal(failures)
	}
	before, _ := manager.snapshot("wiki")
	if err := os.WriteFile(filepath.Join(before.Root, "public", "index.html"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if failures := manager.refresh(); len(failures) == 0 {
		t.Fatal("expected invalid active generation refresh to fail")
	}
	after, ok := manager.snapshot("wiki")
	if !ok || after.Pointer.ContentDigest != before.Pointer.ContentDigest {
		t.Fatalf("expected last valid snapshot to remain active: %#v", after)
	}
}

func TestRuntimePrivateTransportSynchronizesVerifiedGenerationAndSeparatesCapabilities(t *testing.T) {
	root := t.TempDir()
	enablePublicArtifactTest(t, filepath.Join(root, "Wiki"))
	writeViewerFile(t, root, "Wiki/index.md", "# Private transport v1\n")
	writeViewerFile(t, root, "runtime.toml", `
[runtime]
state_dir = "publisher-state"
[artifact_store]
type = "filesystem"
path = "publisher-artifacts"
[publisher_api]
enabled = true
address = "127.0.0.1:8090"
artifact_token_env = "TEST_ARTIFACT_TOKEN"
exchange_token_env = "TEST_EXCHANGE_TOKEN"
[worker]
exchange_dir = "publisher-exchange"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`)
	artifactCapability := strings.Repeat("a", 40)
	exchangeCapability := strings.Repeat("e", 40)
	t.Setenv("TEST_ARTIFACT_TOKEN", artifactCapability)
	t.Setenv("TEST_EXCHANGE_TOKEN", exchangeCapability)
	publisherConfig, err := okruntime.LoadConfig(filepath.Join(root, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := buildRuntimeKnowledgeGeneration(publisherConfig, publisherConfig.KnowledgeBases[0], "first", filepath.Join(root, "first"), true)
	if err != nil {
		t.Fatal(err)
	}
	publisherHandler, err := newRuntimePublisherAPIHandler(publisherConfig)
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := runtimeRequest(t, publisherHandler, http.MethodGet, "/v1/artifacts/wiki/active.json", "", map[string]string{"Authorization": "Bearer " + exchangeCapability})
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("exchange capability read artifact endpoint: %d", unauthorized.Code)
	}
	crossScope := runtimeRequest(t, publisherHandler, http.MethodGet, "/v1/exchange/source.bundle", "", map[string]string{"Authorization": "Bearer " + artifactCapability})
	if crossScope.Code != http.StatusUnauthorized {
		t.Fatalf("artifact capability read exchange endpoint: %d", crossScope.Code)
	}
	if err := os.MkdirAll(publisherConfig.Worker.ExchangeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publisherConfig.Worker.ExchangeDir, "source.bundle"), []byte("private git bundle"), 0644); err != nil {
		t.Fatal(err)
	}
	source := runtimeRequest(t, publisherHandler, http.MethodGet, "/v1/exchange/source.bundle", "", map[string]string{"Authorization": "Bearer " + exchangeCapability})
	if source.Code != http.StatusOK || source.Body.String() != "private git bundle" {
		t.Fatalf("unexpected source exchange response %d: %q", source.Code, source.Body.String())
	}
	proposal := filepath.Join(root, "proposal")
	if err := os.MkdirAll(proposal, 0755); err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, proposal, "branch.bundle", "untrusted branch bundle")
	bundleDigest, err := okf.SHA256File(filepath.Join(proposal, "branch.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	writeViewerFile(t, proposal, "request.json", fmt.Sprintf(`{"version":1,"run_id":"run-1","job_id":"refresh","branch":"agent/refresh","base_sha":"%s","head_sha":"%s","bundle_sha256":"%s","verify_count":1}`+"\n", strings.Repeat("a", 40), strings.Repeat("b", 40), bundleDigest))
	var proposalArchive bytes.Buffer
	if err := okruntime.WriteDirectoryArchive(&proposalArchive, proposal); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPut, "/v1/exchange/runs/run-1", bytes.NewReader(proposalArchive.Bytes()))
		request.Header.Set("Authorization", "Bearer "+exchangeCapability)
		response := httptest.NewRecorder()
		publisherHandler.ServeHTTP(response, request)
		want := http.StatusCreated
		if attempt == 1 {
			want = http.StatusNoContent
		}
		if response.Code != want {
			t.Fatalf("exchange upload attempt %d = %d, want %d: %s", attempt, response.Code, want, response.Body.String())
		}
	}
	if _, err := os.Stat(filepath.Join(publisherConfig.Worker.ExchangeDir, "runs", "run-1", "request.json")); err != nil {
		t.Fatalf("publisher did not atomically store proposal: %v", err)
	}

	serveConfig := publisherConfig
	serveConfig.PublisherAPI.Enabled = false
	serveConfig.ArtifactStore = okruntime.ArtifactStoreConfig{Type: "http", Path: filepath.Join(root, "serve-cache"), URL: "http://127.0.0.1:8090", TokenEnv: "TEST_ARTIFACT_TOKEN"}
	serveConfig.Runtime.StateDir = filepath.Join(root, "serve-state")
	handler, err := newRuntimeServeHandler(serveConfig)
	if err != nil {
		t.Fatal(err)
	}
	handler.snapshots.client.Transport = runtimeHandlerRoundTripper{handler: publisherHandler}
	if failures := handler.snapshots.refresh(); len(failures) != 0 {
		t.Fatalf("remote activation failed: %v", failures)
	}
	page := runtimeRequest(t, handler, http.MethodGet, "/", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Private transport v1") {
		t.Fatalf("unexpected remotely synchronized page %d: %s", page.Code, page.Body.String())
	}
	before, ok := handler.snapshots.snapshot("wiki")
	if !ok || before.Pointer.ContentDigest != first.ContentDigest {
		t.Fatalf("unexpected first remote snapshot: %#v", before)
	}

	writeViewerFile(t, root, "Wiki/index.md", "# Private transport v2\n")
	second, err := buildRuntimeKnowledgeGeneration(publisherConfig, publisherConfig.KnowledgeBases[0], "second", filepath.Join(root, "second"), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publisherConfig.ArtifactStore.Path, "wiki", "generations", second.Generation, "public", "index.html"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if failures := handler.snapshots.refresh(); len(failures) == 0 {
		t.Fatal("expected publisher to refuse a tampered active generation")
	}
	after, ok := handler.snapshots.snapshot("wiki")
	if !ok || after.Pointer.ContentDigest != before.Pointer.ContentDigest {
		t.Fatalf("serve did not retain last verified generation: %#v", after)
	}
}

func TestRuntimeWorkerReconcilesProductionBranchIntoArtifactStore(t *testing.T) {
	base := t.TempDir()
	repository := filepath.Join(base, "repository-source")
	enablePublicArtifactTest(t, filepath.Join(repository, "Wiki"))
	writeViewerFile(t, repository, "Wiki/index.md", "# First generation\n")
	writeViewerFile(t, repository, "runtime.toml", `
[runtime]
state_dir = "../worker-state"
[artifact_store]
type = "filesystem"
path = "../artifacts"
[worker]
repo = "."
production_branch = "main"
poll_interval = "1s"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
`)
	runtimeGitTest(t, repository, "init", "-b", "main")
	runtimeGitTest(t, repository, "config", "user.name", "Runtime Test")
	runtimeGitTest(t, repository, "config", "user.email", "runtime@example.test")
	runtimeGitTest(t, repository, "add", ".")
	runtimeGitTest(t, repository, "commit", "-m", "first")
	config, err := okruntime.LoadConfig(filepath.Join(repository, "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimePublisherPass(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	publisherCheckout := filepath.Join(config.Runtime.StateDir, "publisher-repository")
	agentCheckout, err := syncRuntimeAgentRepository(t.Context(), config, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if agentCheckout == publisherCheckout {
		t.Fatal("publisher and agent must never share a checkout")
	}
	publisherGit, err := os.Stat(filepath.Join(publisherCheckout, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	agentGit, err := os.Stat(filepath.Join(agentCheckout, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(publisherGit, agentGit) {
		t.Fatal("publisher and agent must never share Git metadata")
	}
	writeViewerFile(t, agentCheckout, "agent-only.txt", "untrusted workspace mutation\n")
	if _, err := os.Stat(filepath.Join(publisherCheckout, "agent-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("agent mutation reached credentialed publisher checkout: %v", err)
	}
	store := okruntime.FilesystemStore{Root: config.ArtifactStore.Path}
	_, firstRoot, err := store.Active("wiki")
	if err != nil {
		t.Fatal(err)
	}
	first, err := okruntime.LoadAndValidateGeneration(firstRoot)
	if err != nil {
		t.Fatal(err)
	}

	writeViewerFile(t, repository, "Wiki/guide.md", "---\ntype: Guide\n---\n\n# New guide\n")
	runtimeGitTest(t, repository, "add", "Wiki/guide.md")
	runtimeGitTest(t, repository, "commit", "-m", "second")
	if err := runtimePublisherPass(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	_, secondRoot, err := store.Active("wiki")
	if err != nil {
		t.Fatal(err)
	}
	second, err := okruntime.LoadAndValidateGeneration(secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if first.Commit == second.Commit || first.ContentDigest == second.ContentDigest {
		t.Fatalf("expected production reconciliation to activate new generation: first=%#v second=%#v", first, second)
	}
	if _, err := os.Stat(filepath.Join(secondRoot, "source", "guide.md")); err != nil {
		t.Fatalf("expected second generation content: %v", err)
	}
}

func TestRuntimePlanReportsAndEnforcesRequiredAgentRuntimes(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, ".openknowledge", "jobs")
	writeViewerFile(t, root, ".openknowledge/jobs/codex.md", "---\nid: codex-job\nagent: {runtime: codex}\n---\nMaintain docs.\n")
	writeViewerFile(t, root, ".openknowledge/jobs/claude.md", "---\nid: claude-job\nagent: {runtime: claude}\n---\nMaintain docs.\n")
	config := okruntime.Config{Worker: okruntime.WorkerConfig{RunJobs: true, Repo: root, JobsPath: jobs, Runtimes: []string{"claude", "codex"}}}
	required, err := runtimeRequiredRuntimes(config)
	if err != nil || !reflect.DeepEqual(required, []string{"claude", "codex"}) {
		t.Fatalf("required=%#v err=%v", required, err)
	}
	config.Worker.Runtimes = []string{"codex"}
	if _, err := runtimeRequiredRuntimes(config); err == nil || !strings.Contains(err.Error(), "requires runtime claude") {
		t.Fatalf("expected missing worker refusal, got %v", err)
	}
}

func TestRuntimePlanReportsNoRequiredRuntimeWithoutJobDefinitions(t *testing.T) {
	root := t.TempDir()
	config := okruntime.Config{Worker: okruntime.WorkerConfig{
		RunJobs: true, Repo: root, JobsPath: filepath.Join(root, ".openknowledge", "jobs"), Runtimes: []string{"codex"},
	}}
	required, err := runtimeRequiredRuntimes(config)
	if err != nil || len(required) != 0 {
		t.Fatalf("required=%#v err=%v", required, err)
	}
}

func TestRuntimeAgentCleanupUsesRuntimeStateAndRetainsAuditRecord(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "repository")
	if err := os.Mkdir(repository, 0755); err != nil {
		t.Fatal(err)
	}
	runtimeGitTest(t, repository, "init", "-b", "main")
	runtimeGitTest(t, repository, "config", "user.name", "Runtime Test")
	runtimeGitTest(t, repository, "config", "user.email", "runtime@example.test")
	writeViewerFile(t, repository, "README.md", "runtime cleanup\n")
	runtimeGitTest(t, repository, "add", "README.md")
	runtimeGitTest(t, repository, "commit", "-m", "initial")

	stateDir := filepath.Join(root, "state")
	jobsState := filepath.Join(stateDir, "jobs-codex")
	t.Setenv(agents.JobsStateDirEnv, jobsState)
	runsDir, err := agents.RepositoryRunDirectory(repository)
	if err != nil {
		t.Fatal(err)
	}
	runID := "aaaaaaaaaaaaaaaaaaaaaaaa"
	runDir := filepath.Join(runsDir, runID)
	worktree := filepath.Join(filepath.Dir(runsDir), "worktrees", runID)
	if err := os.MkdirAll(filepath.Dir(worktree), 0700); err != nil {
		t.Fatal(err)
	}
	runtimeGitTest(t, repository, "worktree", "add", "-b", "agent/cleanup-test", worktree)
	for _, artifact := range []string{"home/cache.bin", "tmp/download.bin", "diff.patch"} {
		writeViewerFile(t, runDir, artifact, strings.Repeat("x", 1024))
	}
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	record := agents.RunRecord{
		SchemaVersion: "1",
		RunID:         runID,
		JobID:         "cleanup-test",
		Status:        "failed",
		ScheduledAt:   now,
		StartedAt:     now,
		FinishedAt:    now.Add(time.Minute),
		Plan: agents.RunPlan{
			RunID:    runID,
			JobID:    "cleanup-test",
			RepoRoot: repository,
			Worktree: worktree,
			RunDir:   runDir,
		},
	}
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), content, 0600); err != nil {
		t.Fatal(err)
	}

	config := okruntime.Config{Runtime: okruntime.RuntimeConfig{StateDir: stateDir}}
	if err := cleanupRuntimeAgentRuns(t.Context(), config, repository, "codex"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("terminal worktree still exists: %v", err)
	}
	for _, artifact := range []string{"home", "tmp", "diff.patch"} {
		if _, err := os.Stat(filepath.Join(runDir, artifact)); !os.IsNotExist(err) {
			t.Fatalf("terminal artifact %s still exists: %v", artifact, err)
		}
	}
	if _, err := os.Stat(filepath.Join(runDir, "run.json")); err != nil {
		t.Fatalf("audit record was removed: %v", err)
	}
}

func runtimeRequest(t *testing.T, handler http.Handler, method string, target string, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func runtimeGitTest(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
