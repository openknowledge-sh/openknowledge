package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecorderDefaultsToFingerprintOnlyAndGroupsGaps(t *testing.T) {
	root := filepath.Join(t.TempDir(), "usage")
	recorder, err := NewRecorder(root, false, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	input := RecordInput{
		At: now, KnowledgeBase: "docs", Generation: Generation{Name: "g1", Commit: "abc", Spec: "0.2", ContentDigest: strings.Repeat("a", 64)},
		Channel: "http-search", Query: "How do I rollback production?", Rejected: []string{"sources_required", "sources_required", "stale"},
	}
	first, err := recorder.Record(input)
	if err != nil {
		t.Fatal(err)
	}
	input.At = now.Add(time.Minute)
	input.Channel = "mcp-search"
	second, err := recorder.Record(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Query != "" || first.QueryFingerprint == "" || first.QueryFingerprint != second.QueryFingerprint || first.Outcome != "policy-rejected" {
		t.Fatalf("unexpected privacy-safe events: %#v %#v", first, second)
	}
	content, err := os.ReadFile(filepath.Join(root, "2026-08-21.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "rollback production") {
		t.Fatalf("raw query leaked into default event log: %s", content)
	}
	events, err := Read([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	found, err := recorder.Find(first.ID)
	if err != nil || found.ID != first.ID {
		t.Fatalf("usage event lookup failed: %#v err=%v", found, err)
	}
	gaps := Gaps(events, 2)
	if len(gaps) != 1 || gaps[0].Occurrences != 2 || gaps[0].Question != "" || len(gaps[0].Channels) != 2 || len(gaps[0].Rejections) != 2 || gaps[0].Rejections[0].Reason != "sources_required" || gaps[0].Rejections[0].Count != 4 {
		t.Fatalf("unexpected gap aggregation: %#v", gaps)
	}
	for _, path := range []string{root, filepath.Join(root, usageKeyFile), filepath.Join(root, "2026-08-21.jsonl")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("usage storage is not private: %s mode=%o", path, info.Mode().Perm())
		}
	}
}

func TestRecorderOptInQueryCaptureRedactsCredentials(t *testing.T) {
	recorder, err := NewRecorder(filepath.Join(t.TempDir(), "usage"), true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	event, err := recorder.Record(RecordInput{
		At: time.Now(), KnowledgeBase: "docs", Generation: Generation{Name: "g1", Commit: "abc", Spec: "0.2", ContentDigest: strings.Repeat("a", 64)},
		Channel: "http-search", Query: "Why token=ghp_abcdefghijklmnopqrstuvwxyz failed?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(event.Query, "ghp_") || !strings.Contains(event.Query, "[redacted]") {
		t.Fatalf("captured query was not redacted: %q", event.Query)
	}
}

func TestReadRejectsUnknownEventFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"openknowledge.usage","version":1,"unknown":true}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read([]string{path}); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected strict event refusal, got %v", err)
	}
}
