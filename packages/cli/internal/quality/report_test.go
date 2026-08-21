package quality

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	knowledgefeedback "github.com/openknowledge-sh/openknowledge/packages/cli/internal/feedback"
	knowledgeintervention "github.com/openknowledge-sh/openknowledge/packages/cli/internal/intervention"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

func TestBuildReportsMeasuredQualityAndConcretePriorities(t *testing.T) {
	root := t.TempDir()
	writeQualityFile(t, root, "index.md", "---\ntitle: Quality test\nokf_version: \"0.2\"\n---\n\n# Quality test\n")
	writeQualityFile(t, root, "trusted.md", "---\ntype: Runbook\ntitle: Trusted\nverified:\n  - by: human:alice\n    at: 2026-08-01T00:00:00Z\nstale_after: 2027-01-01\nsources:\n  - resource: https://example.com/trusted\n---\n\n# Trusted\n")
	writeQualityFile(t, root, "risky.md", "---\ntype: Runbook\ntitle: Risky\nstale_after: 2026-01-01\nsources:\n  - resource: https://example.com/risky\n---\n\n# Risky\n")
	checks := []string{"eval:critical"}
	trustedEvidence := knowledgeusage.Evidence{ID: "trusted", Path: "trusted.md", Locator: "okf+sha256://" + qualityHex('a') + "/trusted.md#" + qualityHex('b')}
	riskyEvidence := knowledgeusage.Evidence{ID: "risky", Path: "risky.md", Locator: "okf+sha256://" + qualityHex('c') + "/risky.md#" + qualityHex('d')}
	usageEvents := []knowledgeusage.Event{
		qualityUsage("11111111111111111111111111111111", "2026-08-20T10:00:00Z", "release-1", qualityHex('1'), "http-search", "evidence-selected", trustedEvidence, checks),
		qualityUsage("22222222222222222222222222222222", "2026-08-21T10:00:00Z", "release-2", qualityHex('2'), "http-search", "evidence-selected", riskyEvidence, checks),
	}
	feedbackEvents := []knowledgefeedback.Event{{
		Type: knowledgefeedback.EventType, Version: knowledgefeedback.EventVersion, ID: "33333333333333333333333333333333", At: "2026-08-21T11:00:00Z",
		KnowledgeBase: "docs", Generation: usageEvents[1].Generation, UsageEventID: usageEvents[1].ID, QueryFingerprint: usageEvents[1].QueryFingerprint,
		Channel: usageEvents[1].Channel, Outcome: usageEvents[1].Outcome,
		Access:    knowledgefeedback.Access{Profile: "public", Agents: []string{}, Teams: []string{}, UseCases: []string{}},
		Sentiment: "negative", Reasons: []string{"incorrect"}, Evidence: []knowledgeusage.Evidence{riskyEvidence},
	}}
	index, err := okf.BuildContextIndexWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	evalReports := []knowledgeeval.Report{{
		SchemaVersion: "1", Dataset: knowledgeeval.DatasetIdentity{Type: "openknowledge.eval-dataset", Version: 1, ID: "critical", SHA256: qualityHex('9')}, Target: knowledgeeval.TargetIdentity{Root: root, Revision: index.Revision},
		Summary: knowledgeeval.Summary{Status: "pass", Total: 1, Passed: 1},
		Cases:   []knowledgeeval.CaseResult{{ID: "trusted-question", Question: "How?", Status: "pass", Agents: []string{"support"}, Context: knowledgeeval.ContextResult{Sources: []knowledgeeval.Source{{ID: "trusted", Path: "trusted.md", Locator: trustedEvidence.Locator}}}, Checks: []knowledgeeval.Check{}}},
	}}
	report, err := Build(Options{Root: root, Spec: "0.2", Now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC), Usage: usageEvents, Feedback: feedbackEvents, Evals: evalReports})
	if err != nil {
		t.Fatal(err)
	}
	metrics := map[string]Metric{}
	for _, metric := range report.Metrics {
		metrics[metric.ID] = metric
	}
	if metrics["agent-used-current-trusted-eval-covered"].Value == nil || *metrics["agent-used-current-trusted-eval-covered"].Value != 50 {
		t.Fatalf("unexpected north-star metric: %#v", metrics["agent-used-current-trusted-eval-covered"])
	}
	if metrics["trusted-answer-rate"].Value == nil || *metrics["trusted-answer-rate"].Value != 50 {
		t.Fatalf("unexpected trusted-answer rate: %#v", metrics["trusted-answer-rate"])
	}
	if metrics["unanswered-question-rate-change"].Change == nil || *metrics["unanswered-question-rate-change"].Change != 0 {
		t.Fatalf("unexpected generation comparison: %#v", metrics["unanswered-question-rate-change"])
	}
	if len(report.Concepts) != 2 || report.Concepts[0].Path != "risky.md" || report.Concepts[0].Priority != "high" || report.Concepts[0].NegativeFeedback != 1 {
		t.Fatalf("unexpected risk ordering: %#v", report.Concepts)
	}
	if report.Concepts[1].Path != "trusted.md" || !report.Concepts[1].EvalCovered || report.Concepts[1].Priority != "none" {
		t.Fatalf("unexpected trusted concept: %#v", report.Concepts[1])
	}
}

func TestBuildRejectsFeedbackWithoutSuppliedGrounding(t *testing.T) {
	root := t.TempDir()
	writeQualityFile(t, root, "index.md", "---\ntitle: Empty\nokf_version: \"0.2\"\n---\n\n# Empty\n")
	feedback := knowledgefeedback.Event{
		Type: knowledgefeedback.EventType, Version: knowledgefeedback.EventVersion, ID: "33333333333333333333333333333333", At: "2026-08-21T11:00:00Z",
		KnowledgeBase: "docs", Generation: knowledgeusage.Generation{Name: "release", Commit: qualityHex('a'), Spec: "0.2", ContentDigest: qualityHex('b'), Checks: []string{}},
		UsageEventID: "22222222222222222222222222222222", QueryFingerprint: qualityHex('c'), Channel: "http-search", Outcome: "no-evidence",
		Access: knowledgefeedback.Access{Profile: "public", Agents: []string{}, Teams: []string{}, UseCases: []string{}}, Sentiment: "negative", Reasons: []string{"incorrect"}, Evidence: []knowledgeusage.Evidence{},
	}
	if _, err := Build(Options{Root: root, Spec: "0.2", Feedback: []knowledgefeedback.Event{feedback}}); err == nil {
		t.Fatal("expected ungrounded feedback to be rejected")
	}
}

func TestBuildRejectsDuplicateUsageEvents(t *testing.T) {
	root := t.TempDir()
	writeQualityFile(t, root, "index.md", "---\ntitle: Empty\nokf_version: \"0.2\"\n---\n\n# Empty\n")
	event := qualityUsage("11111111111111111111111111111111", "2026-08-20T10:00:00Z", "release", qualityHex('1'), "http-search", "evidence-selected", knowledgeusage.Evidence{ID: "missing", Path: "missing.md", Locator: "okf+sha256://" + qualityHex('a') + "/missing.md#" + qualityHex('b')}, []string{})
	if _, err := Build(Options{Root: root, Spec: "0.2", Usage: []knowledgeusage.Event{event, event}}); err == nil {
		t.Fatal("expected duplicate usage event to be rejected")
	}
}

func TestBuildMeasuresInterventionOutcomesWithoutGlobalScore(t *testing.T) {
	root := t.TempDir()
	writeQualityFile(t, root, "index.md", "---\ntitle: Interventions\nokf_version: \"0.2\"\n---\n\n# Interventions\n")
	auto := qualityInterventionLifecycle("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1", "low", "auto", []string{}, "audit-finding", "finding-1", "2026-08-21T10:00:00Z", "2026-08-21T14:00:00Z")
	auto[2].Publication = &knowledgeintervention.Publication{Generation: "release-auto", ContentDigest: qualityHex('a'), Checks: []string{"eval:critical"}, Automated: true, Verified: true}
	auto[2].FindingOutcome = "confirmed"
	human := qualityInterventionLifecycle("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "2", "medium", "human", []string{"github:alice"}, "manual", "request-2", "2026-08-21T10:00:00Z", "2026-08-21T20:00:00Z")
	reviewed := human[1]
	reviewed.ID = "44444444444444444444444444444444"
	reviewed.At = "2026-08-21T12:00:00Z"
	reviewed.Stage = "reviewed"
	reviewed.Review = &knowledgeintervention.Review{Decision: "approved", DurationMinutes: 12}
	human = []knowledgeintervention.Event{human[0], human[1], reviewed, human[2]}
	human[3].Publication = &knowledgeintervention.Publication{Generation: "release-human", ContentDigest: qualityHex('b'), Checks: []string{"eval:critical"}, Automated: false, Verified: true}
	dismissed := auto[0]
	dismissed.ID = "55555555555555555555555555555555"
	dismissed.InterventionID = "cccccccccccccccccccccccccccccccc"
	dismissed.At = "2026-08-21T11:00:00Z"
	dismissed.Stage = "dismissed"
	dismissed.Source.ID = "finding-2"
	dismissed.FindingOutcome = "false-positive"
	dismissed.Reason = "Evidence disproved the finding."
	detectedDismissed := dismissed
	detectedDismissed.ID = "66666666666666666666666666666666"
	detectedDismissed.At = "2026-08-21T09:00:00Z"
	detectedDismissed.Stage = "detected"
	detectedDismissed.FindingOutcome = ""
	detectedDismissed.Reason = ""
	events := append(append(auto, human...), detectedDismissed, dismissed)
	report, err := Build(Options{Root: root, Spec: "0.2", Interventions: events})
	if err != nil {
		t.Fatal(err)
	}
	metrics := map[string]Metric{}
	for _, metric := range report.Metrics {
		metrics[metric.ID] = metric
		if metric.ID == "global-quality-score" {
			t.Fatal("quality report must not introduce a global score")
		}
	}
	for id, expected := range map[string]float64{
		"detection-to-published-fix":        7,
		"human-review-minutes-per-fix":      12,
		"audit-false-positive-rate":         50,
		"safely-automated-maintenance-rate": 50,
	} {
		if metrics[id].Value == nil || *metrics[id].Value != expected {
			t.Fatalf("unexpected %s: %#v", id, metrics[id])
		}
	}
	if report.Inputs.InterventionEvents != len(events) {
		t.Fatalf("unexpected intervention input count: %#v", report.Inputs)
	}
}

func qualityInterventionLifecycle(interventionID, idPrefix, risk, approval string, owners []string, sourceKind, sourceID, detectedAt, publishedAt string) []knowledgeintervention.Event {
	base := knowledgeintervention.Event{
		Type: knowledgeintervention.EventType, Version: knowledgeintervention.EventVersion, InterventionID: interventionID,
		KnowledgeBase: "docs", Actor: knowledgeintervention.Actor{Kind: "agent", ID: "job:maintenance"},
		Source: knowledgeintervention.Source{Kind: sourceKind, ID: sourceID}, Route: knowledgeintervention.Route{Risk: risk, Approval: approval, Confidence: .9, Owners: owners},
		Targets: []string{"runbook.md"}, Evidence: []string{"insight:gap"},
	}
	detected := base
	detected.ID, detected.At, detected.Stage = idPrefix+"1111111111111111111111111111111", detectedAt, "detected"
	proposed := base
	proposed.ID, proposed.At, proposed.Stage = idPrefix+"2222222222222222222222222222222", "2026-08-21T11:00:00Z", "proposed"
	published := base
	published.ID, published.At, published.Stage = idPrefix+"3333333333333333333333333333333", publishedAt, "published"
	return []knowledgeintervention.Event{detected, proposed, published}
}

func qualityUsage(id, at, generation, digest, channel, outcome string, evidence knowledgeusage.Evidence, checks []string) knowledgeusage.Event {
	return knowledgeusage.Event{
		Type: knowledgeusage.EventType, Version: knowledgeusage.EventVersion, ID: id, At: at, KnowledgeBase: "docs",
		Generation: knowledgeusage.Generation{Name: generation, Commit: qualityHex('e'), Spec: "0.2", ContentDigest: digest, Checks: checks},
		Channel:    channel, QueryFingerprint: qualityHex('f'), QueryLength: "1-32", Outcome: outcome,
		Selected: []knowledgeusage.Evidence{evidence}, Rejected: []knowledgeusage.Rejection{},
	}
}

func qualityHex(value byte) string {
	return string(make([]byte, 0)) + repeatQuality(value, 64)
}

func repeatQuality(value byte, count int) string {
	content := make([]byte, count)
	for index := range content {
		content[index] = value
	}
	return string(content)
}

func writeQualityFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
