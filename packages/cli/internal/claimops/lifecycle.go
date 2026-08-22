package claimops

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func CompareLifecycle(base Index, candidate Index) LifecycleReport {
	report := LifecycleReport{Valid: true, Issues: []okf.Issue{}, AuthorityChanges: CompareAuthorities(base, candidate)}
	for _, change := range report.AuthorityChanges {
		if strings.HasPrefix(change.ApprovedBy, "human:") || strings.HasPrefix(change.ApprovedBy, "github:") {
			continue
		}
		report.add(change.Path, fmt.Sprintf("source %q became authoritative without authority_approved_by human:<id> or github:<login>", change.SourceID))
	}
	candidateByKey := map[string][]Occurrence{}
	for _, occurrence := range candidate.Occurrences {
		candidateByKey[occurrence.Claim.ID] = append(candidateByKey[occurrence.Claim.ID], occurrence)
	}
	for _, previous := range base.Occurrences {
		if previous.Claim.Status != "supported" && previous.Claim.Status != "verified" && previous.Claim.Status != "disputed" {
			continue
		}
		matches := candidateByKey[previous.Claim.ID]
		if len(matches) == 0 {
			report.add(previous.Path, fmt.Sprintf("claim occurrence %q with status %s cannot be removed; preserve it and use an explicit lifecycle status", previous.Claim.ID, previous.Claim.Status))
			continue
		}
		preserved := false
		for _, current := range matches {
			if !sameClaimSemantics(current.Claim, previous.Claim) {
				continue
			}
			if !allowedHistoricalTransition(previous.Claim.Status, current.Claim.Status) {
				continue
			}
			if (current.Claim.Status == "rejected" || current.Claim.Status == "superseded" || current.Claim.Status == "archived") && !hasHumanVerifier(current) {
				continue
			}
			preserved = true
			break
		}
		if !preserved {
			report.add(previous.Path, fmt.Sprintf("claim %q must preserve its value, evidence, and approved lifecycle history", previous.Claim.ID))
		}
	}

	for path, previousSignals := range baseDocuments(base) {
		currentSignals, exists := candidateDocuments(candidate)[path]
		if !exists {
			if len(previousSignals.ClaimRefs) > 0 {
				report.add(path, "a document with claim_refs cannot be removed without preserving its dependency decision")
			}
			continue
		}
		for _, ref := range previousSignals.ClaimRefs {
			if containsString(currentSignals.ClaimRefs, ref) || documentHasHumanVerifier(candidate, path) {
				continue
			}
			report.add(path, fmt.Sprintf("claim_ref %q cannot be removed without a human verification event", ref))
		}
	}

	sort.Slice(report.Issues, func(i, j int) bool {
		if report.Issues[i].Path != report.Issues[j].Path {
			return report.Issues[i].Path < report.Issues[j].Path
		}
		return report.Issues[i].Message < report.Issues[j].Message
	})
	report.Valid = len(report.Issues) == 0
	return report
}

// CompareAuthorities reports source identities that become authoritative in
// the candidate. A matching source that was authoritative in the base is not a
// change. The result is used both by lifecycle validation and auto-merge
// routing so an agent cannot silently grant its own evidence authority.
func CompareAuthorities(base Index, candidate Index) []AuthorityChange {
	baseAuthorities := map[string]bool{}
	for _, source := range base.Sources {
		if source.Role == "authoritative" {
			baseAuthorities[indexedAuthorityKey(source)] = true
		}
	}
	changesByKey := map[string]AuthorityChange{}
	for _, source := range candidate.Sources {
		if source.Role != "authoritative" {
			continue
		}
		key := indexedAuthorityKey(source)
		if baseAuthorities[key] {
			continue
		}
		changesByKey[key] = AuthorityChange{
			Path: source.Path, SourceID: source.ID, Resource: source.Resource,
			ApprovedBy: source.AuthorityApprovedBy,
		}
	}
	changes := make([]AuthorityChange, 0, len(changesByKey))
	for _, change := range changesByKey {
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].SourceID < changes[j].SourceID
	})
	return changes
}

func indexedAuthorityKey(source IndexedSource) string {
	return source.Path + "\x00" + source.ID + "\x00" + source.Resource
}

func (report *LifecycleReport) add(path string, message string) {
	report.Issues = append(report.Issues, okf.Issue{Path: path, Line: 1, Rule: "claim-history", Message: message})
}

func sameClaimSemantics(left, right okf.Claim) bool {
	leftObject, leftErr := okf.NormalizeClaimObject(left.Object)
	rightObject, rightErr := okf.NormalizeClaimObject(right.Object)
	return leftErr == nil && rightErr == nil && leftObject == rightObject && okf.ClaimComparisonKey(left) == okf.ClaimComparisonKey(right) && left.ValidTime == right.ValidTime && reflect.DeepEqual(left.Evidence, right.Evidence)
}

func allowedHistoricalTransition(previous string, current string) bool {
	switch previous {
	case "supported":
		return current == "supported" || current == "verified" || current == "disputed" || current == "rejected" || current == "superseded" || current == "archived"
	case "verified":
		return current == "verified" || current == "disputed" || current == "superseded" || current == "archived"
	case "disputed":
		return current == "disputed" || current == "verified" || current == "rejected" || current == "superseded" || current == "archived"
	default:
		return true
	}
}

func hasHumanVerifier(occurrence Occurrence) bool {
	verification := occurrence.Claim.Verification
	if verification != nil && (strings.HasPrefix(verification.By, "human:") || strings.HasPrefix(verification.By, "github:")) {
		return true
	}
	for _, decision := range occurrence.Claim.Decisions {
		if strings.HasPrefix(decision.By, "human:") || strings.HasPrefix(decision.By, "github:") {
			return true
		}
	}
	return false
}

func baseDocuments(index Index) map[string]okf.ClaimProfileSignals {
	result := map[string]okf.ClaimProfileSignals{}
	for path, document := range index.documents {
		if signals := okf.DeriveClaimProfileSignals(document.Frontmatter.Data); signals != nil {
			result[path] = *signals
		}
	}
	return result
}

func candidateDocuments(index Index) map[string]okf.ClaimProfileSignals {
	return baseDocuments(index)
}

func documentHasHumanVerifier(index Index, path string) bool {
	document, exists := index.documents[path]
	if !exists {
		return false
	}
	signals := okf.DeriveOKFV02Signals(document.Frontmatter.Data)
	for _, event := range signals.Verified {
		if strings.HasPrefix(event.By, "human:") || strings.HasPrefix(event.By, "github:") {
			return true
		}
	}
	return false
}
