package quality

import (
	"strings"
	"testing"
)

func TestRenderHTMLBuildsAccessibleOfflineRiskLedger(t *testing.T) {
	northStar := 62.5
	trusted := 80.0
	report := Report{
		Type: ReportType, Version: ReportVersion, EvaluatedAt: "2026-08-21T12:00:00Z",
		Bundle: BundleIdentity{Path: "/workspace/Wiki", Spec: "0.2", SHA256: strings.Repeat("a", 64)},
		Window: ObservationWindow{From: "2026-08-20T10:00:00Z", To: "2026-08-21T10:00:00Z"},
		Inputs: InputSummary{UsageEvents: 8, FeedbackEvents: 2, EvalReports: 1},
		Metrics: []Metric{
			{ID: "agent-used-current-trusted-eval-covered", Status: "measured", Unit: "percent", Value: &northStar, Note: "Grounded evidence."},
			{ID: "trusted-answer-rate", Status: "measured", Unit: "percent", Value: &trusted, Note: "Trusted answers."},
			{ID: "unanswered-question-rate", Status: "unavailable", Unit: "percent", Note: "No observations."},
		},
		Generations: []GenerationOutcome{{Name: "release-1", ContentDigest: strings.Repeat("b", 64), FirstSeen: "2026-08-20T10:00:00Z", LastSeen: "2026-08-21T10:00:00Z", UsageEvents: 8, Answered: 5, Unanswered: 3, UnansweredRate: 37.5}},
		Concepts:    []ConceptObservation{{Path: "runbook.md", Title: `Recovery </td><script>alert(1)</script>`, TrustTier: "human-reviewed", Status: "stable", Current: true, Trusted: true, EvalCoverageStatus: "measured", EvalCovered: false, Uses: 8, Answers: 5, Sources: []string{}, Priority: "medium", RiskReasons: []string{"used-without-eval"}, FeedbackReasons: []ReasonCount{}, AuditFindings: []ReasonCount{}}},
		Changes:     []ChangeObservation{},
	}
	content, err := RenderHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	html := string(content)
	for _, expected := range []string{
		"THESIS: Knowledge quality is a release ledger", `id="concept-search"`, `data-priority="medium"`, `data-coverage="uncovered"`, "62.50%", `style="width:37.50%"`, "prefers-reduced-motion", "@media print", `<noscript><style>.filter-bar { display: none; }</style></noscript>`, `priority.value = 'actionable'`, `<option value="" selected>All knowledge</option>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("dashboard is missing %q", expected)
		}
	}
	if strings.Contains(html, `<script>alert(1)</script>`) || !strings.Contains(html, `&lt;/td&gt;&lt;script&gt;alert(1)&lt;/script&gt;`) {
		t.Fatalf("dashboard did not escape concept content:\n%s", html)
	}
	if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Fatal("dashboard must not depend on network assets")
	}
}
