package eval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	ClaimReplayDatasetType    = "openknowledge.claim-replay-eval"
	ClaimReplayDatasetVersion = 1
	ClaimReplayReportVersion  = "1"

	ClaimTruthSupported  = "supported"
	ClaimTruthRefuted    = "refuted"
	ClaimTruthUnverified = "unverified"

	ClaimClassSupported    = "supported"
	ClaimClassStale        = "stale"
	ClaimClassHallucinated = "hallucinated"
	ClaimClassUnverified   = "unverified"

	maxClaimReplayCheckpoints = 100
	maxClaimReplayClaims      = 10_000
	maxClaimReplayTextBytes   = 16_384
)

// ClaimReplayDataset records code-backed truth judgments at an ordered,
// bounded sequence of repository checkpoints. It intentionally does not ask a
// model to decide whether a claim is true.
type ClaimReplayDataset struct {
	Type        string                  `json:"type" yaml:"type"`
	Version     int                     `json:"version" yaml:"version"`
	ID          string                  `json:"id" yaml:"id"`
	Checkpoints []ClaimReplayCheckpoint `json:"checkpoints" yaml:"checkpoints"`
}

type ClaimReplayCheckpoint struct {
	ID           string                   `json:"id" yaml:"id"`
	Revision     string                   `json:"revision" yaml:"revision"`
	Expectations []ClaimReplayExpectation `json:"expectations,omitempty" yaml:"expectations,omitempty"`
}

// ClaimReplayExpectation is the recorded ground truth for one exact claim
// occurrence at a checkpoint. Refuted means the repository evidence disproves
// the claim. Unverified means the evidence can neither confirm nor refute it.
type ClaimReplayExpectation struct {
	ClaimID string `json:"claim_id" yaml:"claim_id"`
	State   string `json:"state" yaml:"state"`
}

type LoadedClaimReplayDataset struct {
	Dataset ClaimReplayDataset
	Path    string
	SHA256  string
}

// ObservedClaim is an active wiki claim returned for a checkpoint. ClaimID is
// the immutable occurrence ID. Statement and Document are report context and
// do not participate in classification.
type ObservedClaim struct {
	ClaimID   string `json:"claimId"`
	Statement string `json:"statement,omitempty"`
	Document  string `json:"document,omitempty"`
}

// ClaimSnapshotProvider lets command integration choose how to materialize a
// checkpoint. A Git-backed provider can use revision with git show or a
// temporary worktree without coupling this deterministic core to Git.
type ClaimSnapshotProvider interface {
	ClaimsAt(context.Context, ClaimReplayCheckpoint) ([]ObservedClaim, error)
}

type ClaimSnapshotFunc func(context.Context, ClaimReplayCheckpoint) ([]ObservedClaim, error)

func (fn ClaimSnapshotFunc) ClaimsAt(ctx context.Context, checkpoint ClaimReplayCheckpoint) ([]ObservedClaim, error) {
	return fn(ctx, checkpoint)
}

type ClaimReplayReport struct {
	SchemaVersion string                        `json:"schemaVersion"`
	Dataset       ClaimReplayDatasetIdentity    `json:"dataset"`
	Summary       ClaimReplaySummary            `json:"summary"`
	Checkpoints   []ClaimReplayCheckpointResult `json:"checkpoints"`
}

type ClaimReplayDatasetIdentity struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	ID      string `json:"id"`
	Path    string `json:"path,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
}

type ClaimReplaySummary struct {
	Total        int `json:"total"`
	Supported    int `json:"supported"`
	Stale        int `json:"stale"`
	Hallucinated int `json:"hallucinated"`
	Unverified   int `json:"unverified"`
}

type ClaimReplayCheckpointResult struct {
	ID       string                   `json:"id"`
	Revision string                   `json:"revision"`
	Summary  ClaimReplaySummary       `json:"summary"`
	Claims   []ClaimReplayClaimResult `json:"claims"`
}

type ClaimReplayClaimResult struct {
	ClaimID                 string `json:"claimId"`
	Statement               string `json:"statement,omitempty"`
	Document                string `json:"document,omitempty"`
	Classification          string `json:"classification"`
	RecordedState           string `json:"recordedState"`
	ExpectationRecorded     bool   `json:"expectationRecorded"`
	Observed                bool   `json:"observed"`
	LastSupportedCheckpoint string `json:"lastSupportedCheckpoint,omitempty"`
}

func LoadClaimReplayDataset(path string) (LoadedClaimReplayDataset, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return LoadedClaimReplayDataset{}, err
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return LoadedClaimReplayDataset{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	var dataset ClaimReplayDataset
	if err := decoder.Decode(&dataset); err != nil {
		return LoadedClaimReplayDataset{}, fmt.Errorf("claim replay eval dataset YAML is invalid: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return LoadedClaimReplayDataset{}, errors.New("claim replay eval dataset YAML must contain one document")
		}
		return LoadedClaimReplayDataset{}, fmt.Errorf("claim replay eval dataset YAML is invalid: %w", err)
	}
	if err := ValidateClaimReplayDataset(dataset); err != nil {
		return LoadedClaimReplayDataset{}, err
	}
	digest := sha256.Sum256(content)
	return LoadedClaimReplayDataset{
		Dataset: dataset,
		Path:    absolute,
		SHA256:  hex.EncodeToString(digest[:]),
	}, nil
}

func ValidateClaimReplayDataset(dataset ClaimReplayDataset) error {
	if dataset.Type != ClaimReplayDatasetType {
		return fmt.Errorf("claim replay eval dataset type must be %q", ClaimReplayDatasetType)
	}
	if dataset.Version != ClaimReplayDatasetVersion {
		return fmt.Errorf("claim replay eval dataset version must be %d", ClaimReplayDatasetVersion)
	}
	if !datasetIDPattern.MatchString(dataset.ID) {
		return errors.New("claim replay eval dataset id is invalid")
	}
	if len(dataset.Checkpoints) == 0 || len(dataset.Checkpoints) > maxClaimReplayCheckpoints {
		return fmt.Errorf("claim replay eval checkpoints must contain between 1 and %d entries", maxClaimReplayCheckpoints)
	}
	checkpointIDs := map[string]bool{}
	totalExpectations := 0
	for checkpointIndex, checkpoint := range dataset.Checkpoints {
		prefix := fmt.Sprintf("claim replay checkpoint %d", checkpointIndex)
		if !datasetIDPattern.MatchString(checkpoint.ID) {
			return fmt.Errorf("%s id is invalid", prefix)
		}
		if checkpointIDs[checkpoint.ID] {
			return fmt.Errorf("%s id must be unique", prefix)
		}
		checkpointIDs[checkpoint.ID] = true
		if err := validateClaimReplayText(checkpoint.Revision, 256); err != nil {
			return fmt.Errorf("%s revision %w", prefix, err)
		}
		claimIDs := map[string]bool{}
		for expectationIndex, expectation := range checkpoint.Expectations {
			if err := validateClaimReplayText(expectation.ClaimID, 1024); err != nil {
				return fmt.Errorf("%s expectation %d claim_id %w", prefix, expectationIndex, err)
			}
			if claimIDs[expectation.ClaimID] {
				return fmt.Errorf("%s expectation %d claim_id must be unique", prefix, expectationIndex)
			}
			claimIDs[expectation.ClaimID] = true
			switch expectation.State {
			case ClaimTruthSupported, ClaimTruthRefuted, ClaimTruthUnverified:
			default:
				return fmt.Errorf("%s expectation %d state must be supported, refuted, or unverified", prefix, expectationIndex)
			}
		}
		totalExpectations += len(checkpoint.Expectations)
		if totalExpectations > maxClaimReplayClaims {
			return fmt.Errorf("claim replay eval dataset must contain at most %d expectations", maxClaimReplayClaims)
		}
	}
	return nil
}

func RunClaimReplay(ctx context.Context, loaded LoadedClaimReplayDataset, provider ClaimSnapshotProvider) (ClaimReplayReport, error) {
	if provider == nil {
		return ClaimReplayReport{}, errors.New("claim replay eval requires a snapshot provider")
	}
	if err := ValidateClaimReplayDataset(loaded.Dataset); err != nil {
		return ClaimReplayReport{}, err
	}
	report := ClaimReplayReport{
		SchemaVersion: ClaimReplayReportVersion,
		Dataset: ClaimReplayDatasetIdentity{
			Type: loaded.Dataset.Type, Version: loaded.Dataset.Version, ID: loaded.Dataset.ID,
			Path: loaded.Path, SHA256: loaded.SHA256,
		},
		Checkpoints: make([]ClaimReplayCheckpointResult, 0, len(loaded.Dataset.Checkpoints)),
	}
	lastSupported := map[string]string{}
	for _, checkpoint := range loaded.Dataset.Checkpoints {
		if err := ctx.Err(); err != nil {
			return ClaimReplayReport{}, err
		}
		observed, err := provider.ClaimsAt(ctx, checkpoint)
		if err != nil {
			return ClaimReplayReport{}, fmt.Errorf("claim replay checkpoint %s (%s): %w", checkpoint.ID, checkpoint.Revision, err)
		}
		if err := validateObservedClaims(observed); err != nil {
			return ClaimReplayReport{}, fmt.Errorf("claim replay checkpoint %s (%s): %w", checkpoint.ID, checkpoint.Revision, err)
		}
		observed = append([]ObservedClaim{}, observed...)
		sort.Slice(observed, func(i, j int) bool { return observed[i].ClaimID < observed[j].ClaimID })
		truth := make(map[string]string, len(checkpoint.Expectations))
		for _, expectation := range checkpoint.Expectations {
			truth[expectation.ClaimID] = expectation.State
		}
		checkpointResult := ClaimReplayCheckpointResult{
			ID: checkpoint.ID, Revision: checkpoint.Revision,
			Claims: make([]ClaimReplayClaimResult, 0, len(observed)),
		}
		observedIDs := make(map[string]bool, len(observed))
		for _, claim := range observed {
			observedIDs[claim.ClaimID] = true
			state, recorded := truth[claim.ClaimID]
			if !recorded {
				state = ClaimTruthUnverified
			}
			classification := classifyReplayClaim(state, lastSupported[claim.ClaimID] != "")
			result := ClaimReplayClaimResult{
				ClaimID: claim.ClaimID, Statement: claim.Statement, Document: claim.Document,
				Classification: classification, RecordedState: state,
				ExpectationRecorded: recorded, Observed: true,
			}
			if classification == ClaimClassStale {
				result.LastSupportedCheckpoint = lastSupported[claim.ClaimID]
			}
			checkpointResult.Claims = append(checkpointResult.Claims, result)
			incrementClaimReplaySummary(&checkpointResult.Summary, classification)
			incrementClaimReplaySummary(&report.Summary, classification)
		}
		for _, expectation := range checkpoint.Expectations {
			if observedIDs[expectation.ClaimID] {
				continue
			}
			checkpointResult.Claims = append(checkpointResult.Claims, ClaimReplayClaimResult{
				ClaimID: expectation.ClaimID, Classification: ClaimClassUnverified,
				RecordedState: expectation.State, ExpectationRecorded: true, Observed: false,
			})
			incrementClaimReplaySummary(&checkpointResult.Summary, ClaimClassUnverified)
			incrementClaimReplaySummary(&report.Summary, ClaimClassUnverified)
		}
		sort.Slice(checkpointResult.Claims, func(i, j int) bool { return checkpointResult.Claims[i].ClaimID < checkpointResult.Claims[j].ClaimID })
		// Truth history advances after the checkpoint is classified. This makes a
		// newly refuted claim stale only when an earlier checkpoint supported it.
		for _, expectation := range checkpoint.Expectations {
			if expectation.State == ClaimTruthSupported {
				lastSupported[expectation.ClaimID] = checkpoint.ID
			}
		}
		report.Checkpoints = append(report.Checkpoints, checkpointResult)
	}
	return report, nil
}

func classifyReplayClaim(state string, previouslySupported bool) string {
	switch state {
	case ClaimTruthSupported:
		return ClaimClassSupported
	case ClaimTruthRefuted:
		if previouslySupported {
			return ClaimClassStale
		}
		return ClaimClassHallucinated
	default:
		return ClaimClassUnverified
	}
}

func validateObservedClaims(claims []ObservedClaim) error {
	if len(claims) > maxClaimReplayClaims {
		return fmt.Errorf("snapshot must contain at most %d claims", maxClaimReplayClaims)
	}
	seen := map[string]bool{}
	for index, claim := range claims {
		if err := validateClaimReplayText(claim.ClaimID, 1024); err != nil {
			return fmt.Errorf("observed claim %d claimId %w", index, err)
		}
		if seen[claim.ClaimID] {
			return fmt.Errorf("observed claim %d claimId must be unique", index)
		}
		seen[claim.ClaimID] = true
		if len(claim.Statement) > maxClaimReplayTextBytes {
			return fmt.Errorf("observed claim %d statement must be at most %d bytes", index, maxClaimReplayTextBytes)
		}
		if len(claim.Document) > 4096 {
			return fmt.Errorf("observed claim %d document must be at most 4096 bytes", index)
		}
	}
	return nil
}

func validateClaimReplayText(value string, maximum int) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("is required")
	}
	if value != strings.TrimSpace(value) {
		return errors.New("must not have leading or trailing whitespace")
	}
	if len(value) > maximum {
		return fmt.Errorf("must be at most %d bytes", maximum)
	}
	return nil
}

func incrementClaimReplaySummary(summary *ClaimReplaySummary, classification string) {
	summary.Total++
	switch classification {
	case ClaimClassSupported:
		summary.Supported++
	case ClaimClassStale:
		summary.Stale++
	case ClaimClassHallucinated:
		summary.Hallucinated++
	case ClaimClassUnverified:
		summary.Unverified++
	}
}
