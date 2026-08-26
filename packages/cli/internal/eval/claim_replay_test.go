package eval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunClaimReplayClassifiesRecordedClaimsAcrossCheckpoints(t *testing.T) {
	dataset := ClaimReplayDataset{
		Type: ClaimReplayDatasetType, Version: ClaimReplayDatasetVersion, ID: "forgetting",
		Checkpoints: []ClaimReplayCheckpoint{
			{ID: "initial", Revision: "1111111", Expectations: []ClaimReplayExpectation{
				{ClaimID: "urn:claim:retry-count", State: ClaimTruthSupported},
				{ClaimID: "urn:claim:invented-flag", State: ClaimTruthRefuted},
				{ClaimID: "urn:claim:opaque-runtime", State: ClaimTruthUnverified},
			}},
			{ID: "changed", Revision: "2222222", Expectations: []ClaimReplayExpectation{
				{ClaimID: "urn:claim:retry-count", State: ClaimTruthRefuted},
				{ClaimID: "urn:claim:invented-flag", State: ClaimTruthRefuted},
				{ClaimID: "urn:claim:new-default", State: ClaimTruthSupported},
			}},
		},
	}
	snapshots := map[string][]ObservedClaim{
		"initial": {
			{ClaimID: "urn:claim:opaque-runtime"},
			{ClaimID: "urn:claim:retry-count", Statement: "Tasks retry three times.", Document: "tasks.md"},
			{ClaimID: "urn:claim:invented-flag"},
			{ClaimID: "urn:claim:not-in-oracle"},
		},
		"changed": {
			{ClaimID: "urn:claim:new-default"},
			{ClaimID: "urn:claim:retry-count"},
			{ClaimID: "urn:claim:invented-flag"},
		},
	}
	report, err := RunClaimReplay(context.Background(), LoadedClaimReplayDataset{Dataset: dataset}, ClaimSnapshotFunc(
		func(_ context.Context, checkpoint ClaimReplayCheckpoint) ([]ObservedClaim, error) {
			return append([]ObservedClaim{}, snapshots[checkpoint.ID]...), nil
		},
	))
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != ClaimReplayReportVersion {
		t.Fatalf("unexpected report schema version: %q", report.SchemaVersion)
	}
	wantSummary := ClaimReplaySummary{Total: 7, Supported: 2, Stale: 1, Hallucinated: 2, Unverified: 2}
	if !reflect.DeepEqual(report.Summary, wantSummary) {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	initial := report.Checkpoints[0]
	if got := replayClasses(initial.Claims); !reflect.DeepEqual(got, []string{
		"urn:claim:invented-flag=hallucinated",
		"urn:claim:not-in-oracle=unverified",
		"urn:claim:opaque-runtime=unverified",
		"urn:claim:retry-count=supported",
	}) {
		t.Fatalf("unexpected initial classifications: %v", got)
	}
	if initial.Claims[1].ExpectationRecorded {
		t.Fatal("claim absent from the recorded oracle must stay unverified")
	}
	changed := report.Checkpoints[1]
	if got := replayClasses(changed.Claims); !reflect.DeepEqual(got, []string{
		"urn:claim:invented-flag=hallucinated",
		"urn:claim:new-default=supported",
		"urn:claim:retry-count=stale",
	}) {
		t.Fatalf("unexpected changed classifications: %v", got)
	}
	if changed.Claims[2].LastSupportedCheckpoint != "initial" {
		t.Fatalf("stale claim lost prior support checkpoint: %#v", changed.Claims[2])
	}
}

func TestRunClaimReplayUsesTruthHistoryEvenWhenEarlierSnapshotOmittedClaim(t *testing.T) {
	dataset := ClaimReplayDataset{
		Type: ClaimReplayDatasetType, Version: ClaimReplayDatasetVersion, ID: "history",
		Checkpoints: []ClaimReplayCheckpoint{
			{ID: "before", Revision: "a", Expectations: []ClaimReplayExpectation{{ClaimID: "urn:claim:limit", State: ClaimTruthSupported}}},
			{ID: "after", Revision: "b", Expectations: []ClaimReplayExpectation{{ClaimID: "urn:claim:limit", State: ClaimTruthRefuted}}},
		},
	}
	report, err := RunClaimReplay(context.Background(), LoadedClaimReplayDataset{Dataset: dataset}, ClaimSnapshotFunc(
		func(_ context.Context, checkpoint ClaimReplayCheckpoint) ([]ObservedClaim, error) {
			if checkpoint.ID == "after" {
				return []ObservedClaim{{ClaimID: "urn:claim:limit"}}, nil
			}
			return nil, nil
		},
	))
	if err != nil {
		t.Fatal(err)
	}
	missing := report.Checkpoints[0].Claims[0]
	if missing.Observed || missing.Classification != ClaimClassUnverified || !missing.ExpectationRecorded {
		t.Fatalf("missing expected claim must be explicit and unverified: %#v", missing)
	}
	claim := report.Checkpoints[1].Claims[0]
	if claim.Classification != ClaimClassStale || claim.LastSupportedCheckpoint != "before" {
		t.Fatalf("expected durable truth history to classify stale: %#v", claim)
	}
}

func TestLoadClaimReplayDatasetIsStrictAndContentAddressed(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "claims.yaml")
	content := `type: openknowledge.claim-replay-eval
version: 1
id: forgetting
checkpoints:
  - id: before
    revision: abc123
    expectations:
      - claim_id: urn:claim:retry-count
        state: supported
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadClaimReplayDataset(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Path != path || len(loaded.SHA256) != 64 || loaded.Dataset.Checkpoints[0].Revision != "abc123" {
		t.Fatalf("unexpected loaded dataset: %#v", loaded)
	}
	invalidPath := filepath.Join(directory, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte(content+"unknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadClaimReplayDataset(invalidPath); err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("expected strict unknown-field error, got %v", err)
	}
}

func TestValidateClaimReplayDatasetRejectsAmbiguousOrUnboundedInput(t *testing.T) {
	valid := ClaimReplayDataset{
		Type: ClaimReplayDatasetType, Version: ClaimReplayDatasetVersion, ID: "bounded",
		Checkpoints: []ClaimReplayCheckpoint{{ID: "one", Revision: "HEAD", Expectations: []ClaimReplayExpectation{{ClaimID: "urn:claim:one", State: ClaimTruthSupported}}}},
	}
	duplicate := valid
	duplicate.Checkpoints[0].Expectations = append(duplicate.Checkpoints[0].Expectations, duplicate.Checkpoints[0].Expectations[0])
	if err := ValidateClaimReplayDataset(duplicate); err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("expected duplicate claim rejection, got %v", err)
	}
	invalidState := valid
	invalidState.Checkpoints[0].Expectations = []ClaimReplayExpectation{{ClaimID: "urn:claim:one", State: "maybe"}}
	if err := ValidateClaimReplayDataset(invalidState); err == nil || !strings.Contains(err.Error(), "state must be") {
		t.Fatalf("expected state rejection, got %v", err)
	}
	tooMany := valid
	tooMany.Checkpoints = make([]ClaimReplayCheckpoint, maxClaimReplayCheckpoints+1)
	if err := ValidateClaimReplayDataset(tooMany); err == nil || !strings.Contains(err.Error(), "between 1 and") {
		t.Fatalf("expected checkpoint bound rejection, got %v", err)
	}
}

func TestRunClaimReplayRejectsProviderFailuresAndDuplicateClaims(t *testing.T) {
	loaded := LoadedClaimReplayDataset{Dataset: ClaimReplayDataset{
		Type: ClaimReplayDatasetType, Version: ClaimReplayDatasetVersion, ID: "provider",
		Checkpoints: []ClaimReplayCheckpoint{{ID: "one", Revision: "abc"}},
	}}
	providerError := errors.New("snapshot unavailable")
	_, err := RunClaimReplay(context.Background(), loaded, ClaimSnapshotFunc(func(context.Context, ClaimReplayCheckpoint) ([]ObservedClaim, error) {
		return nil, providerError
	}))
	if !errors.Is(err, providerError) {
		t.Fatalf("expected wrapped provider error, got %v", err)
	}
	_, err = RunClaimReplay(context.Background(), loaded, ClaimSnapshotFunc(func(context.Context, ClaimReplayCheckpoint) ([]ObservedClaim, error) {
		return []ObservedClaim{{ClaimID: "urn:claim:one"}, {ClaimID: "urn:claim:one"}}, nil
	}))
	if err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("expected duplicate observed claim rejection, got %v", err)
	}
}

func replayClasses(claims []ClaimReplayClaimResult) []string {
	result := make([]string, 0, len(claims))
	for _, claim := range claims {
		result = append(result, claim.ClaimID+"="+claim.Classification)
	}
	return result
}
