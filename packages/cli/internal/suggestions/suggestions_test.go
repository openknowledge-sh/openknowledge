package suggestions

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

func TestObserveWritesOneValidPrivateSuggestionAndIgnoresRecursion(t *testing.T) {
	repo, wiki := setupRepository(t)
	index := filepath.Join(wiki, "guide.md")
	if err := os.WriteFile(index, []byte("---\ntype: Guide\nokf_publish: true\n---\n\n# Guide\n\nUpdated.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "private-code.txt"), []byte("must not enter suggestion\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 17, 14, 32, 0, 0, time.UTC)
	observation := Observation{Runtime: "codex", SessionID: "session-1", Summary: "Updated documentation. api_key=secret-value", Now: now}
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
	if item.Status != "pending" || item.Runtime != "codex" || !strings.Contains(item.Patch, "Wiki/guide.md") {
		t.Fatalf("item = %#v", item)
	}
	content, _ := os.ReadFile(first)
	if strings.Contains(item.Patch, "private-code.txt") || !strings.Contains(string(content), "Session changed `private-code.txt`") {
		t.Fatalf("outside diff boundary failed:\n%s", content)
	}
	if strings.Contains(string(content), "secret-value") || !strings.Contains(string(content), "[redacted]") {
		t.Fatalf("secret was not redacted:\n%s", content)
	}
	result, err := okf.Validate(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("validation errors: %#v", result.Errors)
	}
	set, err := okf.BuildPublicationSetWithVersion(wiki, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range set.Markdown {
		if strings.Contains(path, "suggestions/") {
			t.Fatalf("suggestion published: %#v", set)
		}
	}

	// A suggestions-only diff never produces another observation.
	if err := os.Remove(filepath.Join(repo, "private-code.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "Wiki/guide.md")
	runGit(t, repo, "commit", "-m", "guide")
	if _, created, err := Observe(repo, Observation{Runtime: "codex", SessionID: "session-2", Summary: "Only the suggestion changed.", Now: now}); err != nil || created {
		t.Fatalf("recursive observation: created=%v err=%v", created, err)
	}
}

func TestObserveIncludesUntrackedFilesAndDeduplicatesAcrossDates(t *testing.T) {
	repo, wiki := setupRepository(t)
	newPage := filepath.Join(wiki, "new.md")
	if err := os.WriteFile(newPage, []byte("---\ntype: Note\nokf_publish: false\n---\n\n# New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, created, err := Observe(repo, Observation{Runtime: "opencode", SessionID: "same-session", Now: time.Date(2026, 7, 17, 1, 0, 0, 0, time.UTC)})
	if err != nil || !created {
		t.Fatalf("observe untracked: created=%v err=%v", created, err)
	}
	item, err := Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(item.Patch, "Wiki/new.md") || len(item.Targets) != 1 || item.Targets[0] != "new.md" {
		t.Fatalf("untracked file missing: %#v", item)
	}
	second, created, err := Observe(repo, Observation{Runtime: "opencode", SessionID: "same-session", Now: time.Date(2026, 7, 18, 1, 0, 0, 0, time.UTC)})
	if err != nil || created || second != first {
		t.Fatalf("cross-day duplicate: path=%s created=%v err=%v", second, created, err)
	}
}

func TestObserveAnalyzesTranscriptWithoutPersistingRawSession(t *testing.T) {
	repo, wiki := setupRepository(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	transcript := filepath.Join(home, ".codex", "sessions", "trace.jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o755); err != nil {
		t.Fatal(err)
	}
	trace := strings.Join([]string{
		`{"role":"user","content":"RAW-USER-TRANSCRIPT-MUST-NOT-BE-STORED"}`,
		`{"role":"assistant","content":[{"type":"text","text":"Document the runtime adapters. token=very-secret-token"},{"type":"tool_use","name":"read"}]}`,
		`{"type":"tool_result","content":"file contents"}`,
		`{"type":"error","error":"temporary failure"}`,
		`{"type":"retry","retry_count":1}`,
		`{"type":"validation","validation":{"command":"openknowledge validate Wiki","ok":true}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcript, []byte(trace), 0o600); err != nil {
		t.Fatal(err)
	}
	guide := filepath.Join(wiki, "guide.md")
	content, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guide, []byte(strings.Replace(string(content), "Before.", "After. api_key=wiki-secret-value", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := []byte(fmt.Sprintf(`{"session_id":"trace-session","transcript_path":%q}`, transcript))
	path, created, err := Observe(repo, Observation{Runtime: "codex", Payload: payload, Now: time.Date(2026, 7, 17, 14, 32, 0, 0, time.UTC)})
	if err != nil || !created {
		t.Fatalf("observe transcript: created=%v err=%v", created, err)
	}
	suggestion, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(suggestion)
	if strings.Contains(text, "RAW-USER-TRANSCRIPT-MUST-NOT-BE-STORED") || strings.Contains(text, "very-secret-token") || strings.Contains(text, "wiki-secret-value") {
		t.Fatalf("raw session or credential persisted:\n%s", text)
	}
	if !strings.Contains(text, "Document the runtime adapters") || !strings.Contains(text, "[redacted]") {
		t.Fatalf("sanitized assistant outcome missing:\n%s", text)
	}
	if !strings.Contains(text, "proposed patch was omitted because it may contain a credential") {
		t.Fatalf("credential-bearing diff was not omitted:\n%s", text)
	}
	if !strings.Contains(text, "1 user messages, 1 assistant messages, 1 tool calls, 1 tool results, 1 errors, 1 retries, and 1 validation events") {
		t.Fatalf("trace evidence missing:\n%s", text)
	}
}

func TestApplyIsAtomicOnConflictAndAppliesDeclaredPatch(t *testing.T) {
	repo, wiki := setupRepository(t)
	guide := filepath.Join(wiki, "guide.md")
	baseContent := "---\ntype: Guide\nokf_publish: true\n---\n\n# Guide\n\nBefore.\n"
	if err := os.WriteFile(guide, []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}
	base := runGit(t, repo, "rev-parse", "--short=12", "HEAD")
	if err := os.WriteFile(guide, []byte(strings.Replace(baseContent, "Before.", "After.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := runGit(t, repo, "diff", "--binary", "HEAD", "--", "Wiki/guide.md")
	if err := os.WriteFile(guide, []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}
	suggestionPath := filepath.Join(wiki, "suggestions", "change.md")
	if err := os.MkdirAll(filepath.Dir(suggestionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suggestionPath, []byte(testSuggestion(base, patch)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(suggestionPath); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(guide)
	if !strings.Contains(string(updated), "After.") {
		t.Fatalf("patch not applied:\n%s", updated)
	}
	item, err := Parse(suggestionPath)
	if err != nil || item.Status != "applied" {
		t.Fatalf("status: %#v %v", item, err)
	}

	// A conflicting patch leaves both the target and suggestion unchanged.
	if err := os.WriteFile(guide, []byte(strings.Replace(baseContent, "Before.", "Different.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	conflictPath := filepath.Join(wiki, "suggestions", "conflict.md")
	if err := os.WriteFile(conflictPath, []byte(testSuggestion(base, patch)), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeTarget, _ := os.ReadFile(guide)
	beforeSuggestion, _ := os.ReadFile(conflictPath)
	if err := Apply(conflictPath); err == nil {
		t.Fatal("expected conflict")
	}
	afterTarget, _ := os.ReadFile(guide)
	afterSuggestion, _ := os.ReadFile(conflictPath)
	if string(beforeTarget) != string(afterTarget) || string(beforeSuggestion) != string(afterSuggestion) {
		t.Fatal("conflicting apply mutated filesystem")
	}
}

func TestVerifyRunEnforcesKnowledgeBaseAndDeclaredTargets(t *testing.T) {
	repo, wiki := setupRepository(t)
	guide := filepath.Join(wiki, "guide.md")
	baseContent, _ := os.ReadFile(guide)
	base := runGit(t, repo, "rev-parse", "--short=12", "HEAD")
	if err := os.WriteFile(guide, []byte(strings.Replace(string(baseContent), "Before.", "After.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := runGit(t, repo, "diff", "--binary", "HEAD", "--", "Wiki/guide.md")
	if err := os.WriteFile(guide, baseContent, 0o644); err != nil {
		t.Fatal(err)
	}
	suggestionPath := filepath.Join(wiki, "suggestions", "committed.md")
	if err := os.MkdirAll(filepath.Dir(suggestionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suggestionPath, []byte(testSuggestion(base, patch)), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "Wiki/suggestions/committed.md")
	runGit(t, repo, "commit", "-m", "pending suggestion")
	if err := Apply(suggestionPath); err != nil {
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

func testSuggestion(base, patch string) string {
	body := fmt.Sprintf(`---
type: Open Knowledge Suggestion
title: Update guide
description: Test suggestion.
status: pending
okf_publish: false
okf_suggestion_id: abc123
okf_suggestion_kind: docs
okf_suggestion_runtime: codex
okf_suggestion_created_at: 2026-07-17T14:32:00Z
okf_suggestion_base: %s
okf_suggestion_targets:
  - guide.md
tags: [suggestion]
---

# Update guide

## Suggested knowledge

Update the guide.

## Evidence

- Test.

## Proposed patch

%%s
%s
%%s
`, base, strings.TrimSpace(patch))
	return fmt.Sprintf(body, "```diff", "```")
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
