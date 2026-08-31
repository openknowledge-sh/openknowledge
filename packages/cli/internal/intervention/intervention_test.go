package intervention

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRecorderPersistsStrictPrivateLifecycle(t *testing.T) {
	root := filepath.Join(t.TempDir(), "interventions")
	recorder, err := NewRecorder(root)
	if err != nil {
		t.Fatal(err)
	}
	detected := testEvent("11111111111111111111111111111111", "2026-08-21T10:00:00Z", "detected")
	proposed := testEvent("22222222222222222222222222222222", "2026-08-21T11:00:00Z", "proposed")
	published := testEvent("33333333333333333333333333333333", "2026-08-21T12:00:00Z", "published")
	published.Publication = &Publication{Generation: "release-2", ContentDigest: repeatHex('a', 64), Checks: []string{"eval:critical"}, Automated: true, Verified: true}
	for _, event := range []Event{detected, proposed, published} {
		if err := recorder.Append(event); err != nil {
			t.Fatal(err)
		}
	}
	appended, err := recorder.AppendIfMissing(published)
	if err != nil || appended {
		t.Fatalf("idempotent append changed the log: appended=%v err=%v", appended, err)
	}
	events, err := Read([]string{root})
	if err != nil || len(events) != 3 || events[2].Stage != "published" {
		t.Fatalf("unexpected lifecycle: %#v err=%v", events, err)
	}
	if info, err := os.Stat(root); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o700) {
		t.Fatalf("unexpected log permissions: %v err=%v", info, err)
	}
	if info, err := os.Stat(filepath.Join(root, "2026-08-21.jsonl")); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("unexpected event permissions: %v err=%v", info, err)
	}
}

func TestValidateLifecycleRejectsUnsafeOrFabricatedTransitions(t *testing.T) {
	detected := testEvent("11111111111111111111111111111111", "2026-08-21T10:00:00Z", "detected")
	published := testEvent("22222222222222222222222222222222", "2026-08-21T11:00:00Z", "published")
	published.Publication = &Publication{Generation: "release", ContentDigest: repeatHex('a', 64), Checks: []string{"eval"}, Automated: true, Verified: true}
	if err := ValidateLifecycle([]Event{detected, published}); err == nil {
		t.Fatal("expected detected-to-published transition to be rejected")
	}
	unsafe := detected
	unsafe.Targets = []string{"../secret.md"}
	if err := Validate(unsafe); err == nil {
		t.Fatal("expected unsafe target to be rejected")
	}
	forged := published
	forged.Stage = "proposed"
	forged.Publication = nil
	forged.Route = Route{Risk: "high", Approval: "auto", Confidence: .9, Owners: []string{}}
	if err := Validate(forged); err == nil {
		t.Fatal("expected forged high-risk auto route to be rejected")
	}
	humanDetected := detected
	humanDetected.Route = Route{Risk: "medium", Approval: "human", Confidence: .9, Owners: []string{"github:alice"}}
	humanProposed := humanDetected
	humanProposed.ID, humanProposed.At, humanProposed.Stage = "33333333333333333333333333333333", "2026-08-21T11:00:00Z", "proposed"
	humanPublished := humanDetected
	humanPublished.ID, humanPublished.At, humanPublished.Stage = "44444444444444444444444444444444", "2026-08-21T12:00:00Z", "published"
	humanPublished.Publication = &Publication{Generation: "release", ContentDigest: repeatHex('b', 64), Checks: []string{"eval"}, Verified: true}
	if err := ValidateLifecycle([]Event{humanDetected, humanProposed, humanPublished}); err == nil {
		t.Fatal("expected human-routed publication without approved review to be rejected")
	}
}

func testEvent(id, at, stage string) Event {
	return Event{
		Type: EventType, Version: EventVersion, ID: id, InterventionID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", At: at,
		KnowledgeBase: "docs", Stage: stage, Actor: Actor{Kind: "agent", ID: "job:maintenance"},
		Source: Source{Kind: "job-run", ID: "run-1"}, Route: Route{Risk: "low", Approval: "auto", Confidence: .9, Owners: []string{}},
		Targets: []string{"runbook.md"}, Evidence: []string{"insight:gap-1"},
	}
}

func repeatHex(value byte, count int) string {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return string(result)
}
