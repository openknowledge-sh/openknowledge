package feedback

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

func TestFeedbackRecorderBindsUsageEvidenceAndStaysPrivate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "feedback")
	recorder, err := NewRecorder(root, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	usage := knowledgeusage.Event{
		Type: knowledgeusage.EventType, Version: knowledgeusage.EventVersion,
		ID: strings.Repeat("a", 32), At: "2026-08-21T12:00:00Z", KnowledgeBase: "wiki",
		Generation: knowledgeusage.Generation{Name: "generation-1", Commit: "abc", Spec: "0.2", ContentDigest: strings.Repeat("b", 64), Checks: []string{"Knowledge Eval"}},
		Channel:    "http-search", QueryFingerprint: strings.Repeat("c", 64), QueryLength: "1-32", Outcome: "evidence-selected",
		Selected: []knowledgeusage.Evidence{{ID: "guide#reset", Locator: "okf+sha256://" + strings.Repeat("d", 64) + "/guide.md#" + strings.Repeat("e", 64), Path: "guide.md"}}, Rejected: []knowledgeusage.Rejection{},
	}
	event, err := recorder.Record(RecordInput{
		At: time.Date(2026, 8, 21, 12, 30, 0, 0, time.UTC), Usage: usage,
		Access:    Access{Profile: "support", Agents: []string{"support-agent"}, Teams: []string{"support"}},
		Sentiment: "negative", Reasons: []string{"outdated", "incorrect", "outdated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.UsageEventID != usage.ID || len(event.Evidence) != 1 || event.Evidence[0].Path != "guide.md" || len(event.Reasons) != 2 || event.Reasons[0] != "incorrect" {
		t.Fatalf("unexpected feedback event: %#v", event)
	}
	events, err := Read([]string{root})
	if err != nil || len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("unexpected feedback read: %#v err=%v", events, err)
	}
	for _, path := range []string{root, filepath.Join(root, "2026-08-21.jsonl")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
			t.Fatalf("feedback storage is not private: %s mode=%o", path, info.Mode().Perm())
		}
	}
	if _, err := recorder.Record(RecordInput{At: time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC), Usage: usage, Access: Access{Profile: "support"}, Sentiment: "positive", Reasons: []string{"incorrect"}}); err == nil || !strings.Contains(err.Error(), "do not match sentiment") {
		t.Fatalf("expected positive feedback reason refusal, got %v", err)
	}
}
