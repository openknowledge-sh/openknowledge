package okf

import (
	"strings"
	"testing"
	"time"
)

func TestDeriveOKFV02SignalsTrustFreshnessAndContracts(t *testing.T) {
	metadata := map[string]any{
		"type":        "Attested Computation",
		"status":      "deprecated",
		"stale_after": "2026-08-04",
		"generated":   map[string]any{"by": "process:nightly", "at": "2026-08-01T10:00:00Z"},
		"verified": []any{
			map[string]any{"by": "agent/reviewer-v1", "at": "2026-08-02T10:00:00Z"},
			map[string]any{"by": "human:alice", "at": "2026-08-03T10:00:00Z"},
		},
		"sources": []any{map[string]any{
			"id":            "policy-1",
			"resource":      "https://example.test/policy",
			"observe":       "pinned",
			"sha256":        strings.Repeat("a", 64),
			"title":         "Revenue policy",
			"author":        "team:finance",
			"usage_count":   12,
			"last_modified": "2026-07-31",
		}},
		"usage_window": map[string]any{"from": "2026-07-01", "to": "2026-07-31"},
		"runtime":      "python3",
		"parameters":   []any{map[string]any{"name": "year", "type": "integer", "required": true}},
		"computation":  "scripts/revenue.py",
		"executor":     map[string]any{"resource": "runners/python.md", "receipt": []any{"stdout", "sha256"}},
		"attester":     map[string]any{"resource": "attesters/revenue.md"},
	}

	signals := DeriveOKFV02SignalsAt(metadata, time.Date(2026, 8, 4, 23, 0, 0, 0, time.FixedZone("test", 3600)))
	if signals.TrustTier != OKFV02TrustHumanReviewed || signals.Status != "deprecated" || !signals.Stale {
		t.Fatalf("unexpected derived trust/status/freshness: %#v", signals)
	}
	if signals.Generated == nil || signals.Generated.By != "process:nightly" || len(signals.Verified) != 2 {
		t.Fatalf("unexpected provenance: %#v", signals)
	}
	if len(signals.Sources) != 1 || signals.Sources[0].Observe != "pinned" || signals.Sources[0].SHA256 != strings.Repeat("a", 64) || signals.Sources[0].UsageWindow == nil || signals.Sources[0].UsageWindow.To != "2026-07-31" {
		t.Fatalf("expected shared usage window on source: %#v", signals.Sources)
	}
	if signals.Computation == nil || signals.Computation.Path != "scripts/revenue.py" || signals.Computation.Executor == nil || len(signals.Computation.Executor.Receipt) != 2 || signals.Computation.Attester == nil {
		t.Fatalf("unexpected computation contract: %#v", signals.Computation)
	}
}

func TestDeriveOKFV02SignalsTrustDefaults(t *testing.T) {
	tests := []struct {
		name     string
		verified any
		trust    string
	}{
		{name: "unverified", trust: OKFV02TrustUnverified},
		{name: "machine", verified: map[string]any{"by": "process:check"}, trust: OKFV02TrustMachineConfirmed},
		{name: "human", verified: []any{map[string]any{"by": "human:bob"}}, trust: OKFV02TrustHumanReviewed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signals := DeriveOKFV02SignalsAt(map[string]any{"verified": test.verified}, time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC))
			if signals.TrustTier != test.trust || signals.Status != "stable" || signals.Stale {
				t.Fatalf("unexpected defaults: %#v", signals)
			}
		})
	}
}

func TestOKFV02SourceFootnotesUseSafeAnchors(t *testing.T) {
	signals := &OKFV02Signals{Sources: []OKFV02Source{{ID: "Policy A/2026"}, {ID: "!!!"}, {ID: ""}}}
	footnotes := OKFV02SourceFootnotes(signals)
	if footnotes["Policy A/2026"] != "#ok-source-policy-a-2026" || len(footnotes) != 1 {
		t.Fatalf("unexpected footnotes: %#v", footnotes)
	}
	if anchor := OKFV02SourceAnchor("!!!"); anchor != "" || strings.Contains(anchor, "!") {
		t.Fatalf("expected punctuation-only source ID to have no anchor, got %q", anchor)
	}
}
