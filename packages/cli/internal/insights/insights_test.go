package insights

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestObserveWritesPrivateInsightWithoutPatchOrBaseAndIgnoresRecursion(t *testing.T) {
	repo, wiki := setupRepository(t)
	guide := filepath.Join(wiki, "guide.md")
	if err := os.WriteFile(guide, []byte("---\ntype: Guide\nokf_publish: true\n---\n\n# Guide\n\nUpdated.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "private-code.txt"), []byte("must not be copied\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 17, 14, 32, 0, 0, time.UTC)
	observation := Observation{Runtime: "codex", SessionID: "session-1", Summary: "Document the runtime. api_key=secret-value", Now: now}
	first, created, err := Observe(repo, observation)
	if err != nil || !created {
		t.Fatalf("observe: created=%v err=%v", created, err)
	}
	second, created, err := Observe(repo, observation)
	if err != nil || created || second != first {
		t.Fatalf("duplicate: path=%s created=%v err=%v", second, created, err)
	}
	item, err := Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != "pending" || item.Runtime != "codex" || len(item.Targets) != 1 || item.Targets[0] != "guide.md" {
		t.Fatalf("item = %#v", item)
	}
	content, _ := os.ReadFile(first)
	text := string(content)
	for _, forbidden := range []string{"secret-value", "private-code.txt\n+++", "```diff", "okf_insight_base", "okf_suggestion"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("insight persisted forbidden %q:\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, "[redacted]") || !strings.Contains(text, "Session changed `private-code.txt`") {
		t.Fatalf("sanitized evidence missing:\n%s", text)
	}
	result, err := okf.Validate(wiki)
	if err != nil || len(result.Errors) != 0 {
		t.Fatalf("validation: %#v %v", result.Errors, err)
	}
	set, err := okf.BuildPublicationSetWithVersion(wiki, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range set.Markdown {
		if strings.Contains(path, "insights/") {
			t.Fatalf("insight published: %#v", set)
		}
	}

	if err := os.Remove(filepath.Join(repo, "private-code.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "Wiki/guide.md")
	runGit(t, repo, "commit", "-m", "guide")
	if _, created, err := Observe(repo, Observation{Runtime: "codex", SessionID: "session-2", Summary: "Only the insight changed.", Now: now}); err != nil || created {
		t.Fatalf("recursive observation: created=%v err=%v", created, err)
	}
}

func TestObserveAnalyzesTranscriptWithoutPersistingRawSession(t *testing.T) {
	repo, _ := setupRepository(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	transcript := filepath.Join(home, ".codex", "sessions", "trace.jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o755); err != nil {
		t.Fatal(err)
	}
	trace := strings.Join([]string{
		`{"role":"user","content":"RAW-USER-TRANSCRIPT-MUST-NOT-BE-STORED"}`,
		`{"role":"assistant","content":"Document runtime adapters. token=very-secret-token"}`,
		`{"type":"tool_call"}`,
		`{"type":"tool_result"}`,
		`{"type":"validation"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcript, []byte(trace), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := []byte(fmt.Sprintf(`{"session_id":"trace-session","transcript_path":%q}`, transcript))
	path, created, err := Observe(repo, Observation{Runtime: "codex", Payload: payload, Now: time.Date(2026, 7, 17, 14, 32, 0, 0, time.UTC)})
	if err != nil || !created {
		t.Fatalf("observe transcript: created=%v err=%v", created, err)
	}
	content, _ := os.ReadFile(path)
	text := string(content)
	if strings.Contains(text, "RAW-USER-TRANSCRIPT-MUST-NOT-BE-STORED") || strings.Contains(text, "very-secret-token") {
		t.Fatalf("raw session or credential persisted:\n%s", text)
	}
	if !strings.Contains(text, "Document runtime adapters") || !strings.Contains(text, "[redacted]") {
		t.Fatalf("sanitized outcome missing:\n%s", text)
	}
	if !strings.Contains(text, "1 user messages, 1 assistant messages, 1 tool calls, 1 tool results") {
		t.Fatalf("trace evidence missing:\n%s", text)
	}
}

func TestResolveAndDismissRequirePendingInsight(t *testing.T) {
	_, wiki := setupRepository(t)
	directory := filepath.Join(wiki, "insights")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved := filepath.Join(directory, "resolved.md")
	dismissed := filepath.Join(directory, "dismissed.md")
	if err := os.WriteFile(resolved, []byte(testInsight("resolve")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dismissed, []byte(testInsight("dismiss")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Resolve(resolved); err != nil {
		t.Fatal(err)
	}
	if err := Dismiss(dismissed); err != nil {
		t.Fatal(err)
	}
	first, _ := Parse(resolved)
	second, _ := Parse(dismissed)
	if first.Status != "resolved" || second.Status != "dismissed" {
		t.Fatalf("statuses: %s %s", first.Status, second.Status)
	}
	if err := Resolve(resolved); err == nil {
		t.Fatal("expected non-pending resolve to fail")
	}
}

func TestParseRejectsUnsafeInsightTargetsBeforeExecution(t *testing.T) {
	content := strings.Replace(testInsight("unsafe-target"), "  - guide.md", "  - ../../README.md", 1)
	if _, err := ParseContent("unsafe.md", []byte(content)); err == nil || !strings.Contains(err.Error(), "knowledge-base-relative") {
		t.Fatalf("unsafe target was accepted: %v", err)
	}
}

func TestVerifyRunEnforcesKnowledgeBoundaryAndDeclaredTargets(t *testing.T) {
	repo, wiki := setupRepository(t)
	insightPath := filepath.Join(wiki, "insights", "committed.md")
	if err := os.MkdirAll(filepath.Dir(insightPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(insightPath, []byte(testInsight("verify")), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "Wiki/insights/committed.md")
	runGit(t, repo, "commit", "-m", "pending insight")
	guide := filepath.Join(wiki, "guide.md")
	content, _ := os.ReadFile(guide)
	if err := os.WriteFile(guide, []byte(strings.Replace(string(content), "Before.", "After.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Resolve(insightPath); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRun(wiki); err != nil {
		t.Fatalf("valid run rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRun(wiki); err == nil || !strings.Contains(err.Error(), "outside knowledge base") {
		t.Fatalf("outside edit not rejected: %v", err)
	}
}

func TestChangeGuardDetectsAgentEditsOutsideKnowledgeBase(t *testing.T) {
	repo, wiki := setupRepository(t)
	guard, err := CaptureChangeGuard(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := guard.ValidateKnowledgeOnly("Wiki", "Wiki/insights"); err == nil || !strings.Contains(err.Error(), "outside knowledge base") {
		t.Fatalf("outside edit not rejected: %v", err)
	}
	guard, _ = CaptureChangeGuard(repo)
	content, _ := os.ReadFile(filepath.Join(wiki, "guide.md"))
	if err := os.WriteFile(filepath.Join(wiki, "guide.md"), append(content, []byte("\nMore.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := guard.ValidateKnowledgeOnly("Wiki", "Wiki/insights"); err != nil {
		t.Fatalf("knowledge edit rejected: %v", err)
	}
}

func setupRepository(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	wiki := filepath.Join(repo, "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wiki, "index.md"), []byte("---\nokf_version: \"0.1\"\nokf_publish: true\n---\n\n# Wiki\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wiki, "guide.md"), []byte("---\ntype: Guide\nokf_publish: true\n---\n\n# Guide\n\nBefore.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wiki, "openknowledge.toml"), []byte("[publish]\nenabled = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := integration.InstallProject(wiki); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	return repo, wiki
}

func testInsight(id string) string {
	return fmt.Sprintf(`---
type: Open Knowledge Insight
title: Update guide
description: Test insight.
status: pending
okf_publish: false
okf_insight_id: %s
okf_insight_kind: docs
okf_insight_runtime: codex
okf_insight_created_at: 2026-07-17T14:32:00Z
okf_insight_targets:
  - guide.md
tags: [insight]
---

# Update guide

## Insight

Update the guide after checking current repository evidence.

## Evidence

- Test observation.
`, id)
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
