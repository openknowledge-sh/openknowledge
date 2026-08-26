package claimops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/natefinch/atomic"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	"go.yaml.in/yaml/v3"
)

func ApplyProposal(root string, spec string, proposal Proposal) (bool, error) {
	if err := ValidateProposal(proposal); err != nil {
		return false, err
	}
	document, path, err := resolveDocument(root, proposal.Document)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != proposal.DocumentSHA256 {
		return false, fmt.Errorf("proposal is stale because %s changed", document)
	}
	if err := ValidateProposalInput(root, content, document, proposal.Claim, proposal.Reason, proposal.Confidence); err != nil {
		return false, err
	}
	before, err := BuildIndex(root, spec, time.Now().UTC())
	if err != nil {
		return false, err
	}
	updated, changed, err := upsertClaimContent(content, proposal.Claim)
	if err != nil || !changed {
		return changed, err
	}
	if err := writeAndCheck(root, spec, path, document, content, updated, before); err != nil {
		return false, err
	}
	return true, nil
}

func Link(root string, spec string, claimID string, document string) (bool, error) {
	index, err := BuildIndex(root, spec, time.Now().UTC())
	if err != nil {
		return false, err
	}
	found := false
	for _, occurrence := range index.Occurrences {
		if occurrence.Claim.ID == claimID {
			found = true
			break
		}
	}
	if !found {
		return false, fmt.Errorf("claim not found: %s", claimID)
	}
	document, path, err := resolveDocument(root, document)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	parsed, err := okf.ParseFrontmatterDocument(content)
	if err != nil || !parsed.Has {
		return false, fmt.Errorf("link target requires valid YAML frontmatter")
	}
	var refs []string
	if values, ok := parsed.Data["claim_refs"].([]any); ok {
		for _, value := range values {
			if ref, ok := value.(string); ok {
				refs = append(refs, ref)
			}
		}
	}
	refs = append(refs, claimID)
	refs = uniqueStrings(refs)
	if existing := index.Dependents[claimID]; containsString(existing, document) {
		return false, nil
	}
	updated, err := rewriteFrontmatterField(content, "claim_refs", refs)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(claimStringValue(parsed.Data[okf.ClaimProfileActivationKey])) != okf.ClaimProfileVersionV1 {
		updated, err = rewriteFrontmatterField(updated, okf.ClaimProfileActivationKey, okf.ClaimProfileVersionV1)
		if err != nil {
			return false, err
		}
	}
	if err := writeAndCheck(root, spec, path, document, content, updated, index); err != nil {
		return false, err
	}
	return true, nil
}

func Archive(root string, spec string, claimID string, document string, approvedBy string) (bool, error) {
	index, err := BuildIndex(root, spec, time.Now().UTC())
	if err != nil {
		return false, err
	}
	var matches []Occurrence
	for _, occurrence := range index.Occurrences {
		if occurrence.Claim.ID == claimID && (document == "" || occurrence.Path == document) {
			matches = append(matches, occurrence)
		}
	}
	if len(matches) == 0 {
		return false, fmt.Errorf("claim not found: %s", claimID)
	}
	if len(matches) != 1 {
		matches = preferredOccurrences(matches, "verified", "disputed")
	}
	if len(matches) != 1 {
		return false, fmt.Errorf("claim archival is ambiguous; select one declaring document")
	}
	match := matches[0]
	if match.Claim.Status == "archived" {
		return false, nil
	}
	if match.Claim.Status == "verified" || match.Claim.Status == "disputed" {
		approvedBy = strings.TrimSpace(approvedBy)
		if !strings.HasPrefix(approvedBy, "human:") && !strings.HasPrefix(approvedBy, "github:") {
			return false, fmt.Errorf("archiving a %s claim requires --approved-by human:<id> or github:<login>", match.Claim.Status)
		}
	}
	document, path, err := resolveDocument(root, match.Path)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	updated, changed, err := updateClaimStatusWithVerifierContent(content, match.Claim, "archived", approvedBy, time.Now().UTC(), nil)
	if err != nil || !changed {
		return changed, err
	}
	if err := writeAndCheck(root, spec, path, document, content, updated, index); err != nil {
		return false, err
	}
	return true, nil
}

func Dispute(root string, spec string, claimID string, document string) (bool, error) {
	index, err := BuildIndex(root, spec, time.Now().UTC())
	if err != nil {
		return false, err
	}
	var matches []Occurrence
	for _, occurrence := range index.Occurrences {
		if occurrence.Claim.ID == claimID && (document == "" || occurrence.Path == document) {
			matches = append(matches, occurrence)
		}
	}
	if len(matches) == 0 {
		return false, fmt.Errorf("claim not found: %s", claimID)
	}
	if len(matches) != 1 {
		allMatches := matches
		matches = preferredOccurrences(allMatches, "proposed")
		if len(matches) == 0 {
			matches = preferredOccurrences(allMatches, "verified")
		}
	}
	if len(matches) != 1 {
		return false, fmt.Errorf("claim dispute is ambiguous; select one declaring document")
	}
	match := matches[0]
	if match.Claim.Status == "disputed" {
		return false, nil
	}
	document, path, err := resolveDocument(root, match.Path)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	updated, changed, err := updateClaimStatusWithVerifierContent(content, match.Claim, "disputed", "", time.Time{}, nil)
	if err != nil || !changed {
		return changed, err
	}
	if err := writeAndCheck(root, spec, path, document, content, updated, index); err != nil {
		return false, err
	}
	return true, nil
}

func Reject(root, spec, claimID, approvedBy string) (bool, error) {
	return transitionClaimWithApproval(root, spec, claimID, "rejected", approvedBy, "")
}

func Supersede(root, spec, claimID, successorID, approvedBy string) (bool, error) {
	return transitionClaimWithApproval(root, spec, claimID, "superseded", approvedBy, successorID)
}

func transitionClaimWithApproval(root, spec, claimID, status, approvedBy, successorID string) (bool, error) {
	approvedBy = strings.TrimSpace(approvedBy)
	if !strings.HasPrefix(approvedBy, "human:") && !strings.HasPrefix(approvedBy, "github:") {
		return false, fmt.Errorf("%s requires --approved-by human:<id> or github:<login>", status)
	}
	index, err := BuildIndex(root, spec, time.Now().UTC())
	if err != nil {
		return false, err
	}
	var match *Occurrence
	for indexValue := range index.Occurrences {
		occurrence := &index.Occurrences[indexValue]
		if occurrence.Claim.ID == claimID {
			if match != nil {
				return false, fmt.Errorf("claim occurrence id is not unique: %s", claimID)
			}
			match = occurrence
		}
	}
	if match == nil {
		return false, fmt.Errorf("claim not found: %s", claimID)
	}
	if successorID != "" {
		found := false
		for _, occurrence := range index.Occurrences {
			if occurrence.Claim.ID != successorID {
				continue
			}
			for _, ref := range occurrence.Claim.Relations.Supersedes {
				if ref == claimID {
					found = true
				}
			}
		}
		if !found {
			return false, fmt.Errorf("successor %q must explicitly supersede %q", successorID, claimID)
		}
	}
	if match.Claim.Status == status {
		return false, nil
	}
	document, path, err := resolveDocument(root, match.Path)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	updated, changed, err := updateClaimStatusWithVerifierContent(content, match.Claim, status, approvedBy, time.Now().UTC(), nil)
	if err != nil || !changed {
		return changed, err
	}
	if err := writeAndCheck(root, spec, path, document, content, updated, index); err != nil {
		return false, err
	}
	return true, nil
}

func Verify(root string, spec string, claimID string, document string, approvedBy string, at time.Time) (bool, error) {
	index, err := BuildIndex(root, spec, at)
	if err != nil {
		return false, err
	}
	var matches []Occurrence
	for _, occurrence := range index.Occurrences {
		if occurrence.Claim.ID == claimID && (document == "" || occurrence.Path == document) {
			matches = append(matches, occurrence)
		}
	}
	if len(matches) == 0 {
		return false, fmt.Errorf("claim not found: %s", claimID)
	}
	if len(matches) != 1 {
		matches = preferredOccurrences(matches, "proposed", "disputed")
	}
	if len(matches) != 1 {
		return false, fmt.Errorf("claim verification is ambiguous; select one declaring document")
	}
	match := matches[0]
	if match.Claim.Status == "verified" {
		return false, nil
	}
	approvedBy = strings.TrimSpace(approvedBy)
	if !strings.HasPrefix(approvedBy, "human:") && !strings.HasPrefix(approvedBy, "github:") && !strings.HasPrefix(approvedBy, "process:") {
		return false, fmt.Errorf("verification requires --approved-by human:<id>|github:<login>|process:<id>")
	}
	document, path, err := resolveDocument(root, match.Path)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	versions, err := ObserveClaimEvidenceVersions(root, match, approvedBy, at)
	if err != nil {
		return false, err
	}
	updated, changed, err := updateClaimStatusWithVerifierContent(content, match.Claim, "verified", approvedBy, at, versions)
	if err != nil || !changed {
		return changed, err
	}
	if err := writeAndCheck(root, spec, path, document, content, updated, index); err != nil {
		return false, err
	}
	return true, nil
}

// ApproveAuthority records the human identity that accepted one source as an
// authority. The receipt is source-scoped so lifecycle comparison can reject
// silent authority elevation in a later candidate.
func ApproveAuthority(root string, spec string, sourceID string, document string, approvedBy string) (bool, error) {
	approvedBy = strings.TrimSpace(approvedBy)
	if !strings.HasPrefix(approvedBy, "human:") && !strings.HasPrefix(approvedBy, "github:") {
		return false, fmt.Errorf("authority approval requires --approved-by human:<id> or github:<login>")
	}
	document, path, err := resolveDocument(root, document)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	parsed, err := okf.ParseFrontmatterDocument(content)
	if err != nil || !parsed.Has {
		return false, fmt.Errorf("authority document requires valid YAML frontmatter")
	}
	values, ok := parsed.Data["sources"].([]any)
	if !ok {
		return false, fmt.Errorf("authority document has no sources")
	}
	matches, changed := 0, false
	for _, value := range values {
		source, ok := value.(map[string]any)
		if !ok || strings.TrimSpace(claimStringValue(source["id"])) != strings.TrimSpace(sourceID) {
			continue
		}
		matches++
		if strings.TrimSpace(claimStringValue(source["role"])) != "authoritative" {
			return false, fmt.Errorf("source %q must have role authoritative before approval", sourceID)
		}
		if strings.TrimSpace(claimStringValue(source["authority_approved_by"])) == approvedBy {
			continue
		}
		source["authority_approved_by"] = approvedBy
		changed = true
	}
	if matches == 0 {
		return false, fmt.Errorf("source not found: %s", sourceID)
	}
	if matches > 1 {
		return false, fmt.Errorf("source approval is ambiguous inside the document")
	}
	if !changed {
		return false, nil
	}
	updated, err := rewriteFrontmatterField(content, "sources", values)
	if err != nil {
		return false, err
	}
	before, err := BuildIndex(root, spec, time.Now().UTC())
	if err != nil {
		return false, err
	}
	if err := writeAndCheck(root, spec, path, document, content, updated, before); err != nil {
		return false, err
	}
	return true, nil
}

func upsertClaimContent(content []byte, claim AuthoredClaim) ([]byte, bool, error) {
	parsed, err := okf.ParseFrontmatterDocument(content)
	if err != nil || !parsed.Has {
		return nil, false, fmt.Errorf("claim document requires valid YAML frontmatter")
	}
	var claims []map[string]any
	if values, ok := parsed.Data["claims"].([]any); ok {
		for _, value := range values {
			mapping, ok := value.(map[string]any)
			if !ok {
				return nil, false, fmt.Errorf("existing claims must contain mappings")
			}
			claims = append(claims, copyMap(mapping))
		}
	}
	replacement := authoredClaimMap(claim)
	for _, existing := range claims {
		id, _ := existing["id"].(string)
		if strings.TrimSpace(id) == claim.ID {
			return nil, false, fmt.Errorf("claim occurrence id already exists: %s", claim.ID)
		}
	}
	claims = append(claims, replacement)
	values := make([]any, 0, len(claims))
	for _, item := range claims {
		values = append(values, item)
	}
	updated, err := rewriteFrontmatterField(content, "claims", values)
	return updated, true, err
}

func updateClaimStatusWithVerifierContent(content []byte, target okf.Claim, status string, approvedBy string, at time.Time, versions []okf.ClaimEvidenceVersion) ([]byte, bool, error) {
	parsed, err := okf.ParseFrontmatterDocument(content)
	if err != nil || !parsed.Has {
		return nil, false, fmt.Errorf("claim document requires valid YAML frontmatter")
	}
	values, ok := parsed.Data["claims"].([]any)
	if !ok {
		return nil, false, fmt.Errorf("claim document has no claims")
	}
	matches := 0
	for _, value := range values {
		mapping, ok := value.(map[string]any)
		if !ok || !mappingMatchesClaim(mapping, target) {
			continue
		}
		matches++
		mapping["status"] = status
		if approvedBy != "" {
			if status == "verified" {
				verification := map[string]any{"method": "claim-review", "by": approvedBy, "at": at.UTC().Format(time.RFC3339)}
				if len(target.Evidence) > 0 {
					refs := make([]string, 0, len(target.Evidence))
					for _, evidence := range target.Evidence {
						refs = append(refs, evidence.ID)
					}
					verification["evidence_refs"] = uniqueStrings(refs)
				}
				if len(versions) > 0 {
					verification["evidence_versions"] = claimEvidenceVersionMaps(versions)
				}
				mapping["verification"] = verification
			} else {
				decisions, _ := mapping["decisions"].([]any)
				decisions = append(decisions, map[string]any{"action": status, "by": approvedBy, "at": at.UTC().Format(time.RFC3339)})
				mapping["decisions"] = decisions
			}
		}
	}
	if matches == 0 {
		return nil, false, fmt.Errorf("claim occurrence not found: %s", target.ID)
	}
	if matches > 1 {
		return nil, false, fmt.Errorf("claim verification is ambiguous inside the document")
	}
	updated, err := rewriteFrontmatterField(content, "claims", values)
	return updated, true, err
}

func claimEvidenceVersionMaps(versions []okf.ClaimEvidenceVersion) []any {
	values := make([]any, 0, len(versions))
	for _, version := range versions {
		values = append(values, map[string]any{
			"evidence_ref": version.EvidenceRef,
			"source_ref":   version.SourceRef,
			"resource":     version.Resource,
			"sha256":       version.SHA256,
			"by":           version.By,
			"at":           version.At,
		})
	}
	return values
}

func mappingMatchesClaim(mapping map[string]any, target okf.Claim) bool {
	if strings.TrimSpace(claimStringValue(mapping["id"])) != target.ID {
		return false
	}
	return true
}

func preferredOccurrences(matches []Occurrence, statuses ...string) []Occurrence {
	wanted := map[string]bool{}
	for _, status := range statuses {
		wanted[status] = true
	}
	var result []Occurrence
	for _, match := range matches {
		if wanted[match.Claim.Status] {
			result = append(result, match)
		}
	}
	return result
}

func claimStringValue(value any) string {
	text, _ := value.(string)
	return text
}

func authoredClaimMap(claim AuthoredClaim) map[string]any {
	result := map[string]any{
		"id": claim.ID, "slot": claim.Slot, "subject": claim.Subject, "predicate": claim.Predicate,
		"object": claimObjectMap(claim.Object), "status": "proposed",
	}
	if len(claim.Scope) > 0 {
		scope := map[string]any{}
		for key, object := range claim.Scope {
			scope[key] = claimObjectMap(object)
		}
		result["scope"] = scope
	}
	if len(claim.Evidence) > 0 {
		items := make([]any, 0, len(claim.Evidence))
		for _, evidence := range claim.Evidence {
			items = append(items, claimEvidenceMap(evidence))
		}
		result["evidence"] = items
	}
	if claim.ValidTime.From != "" || claim.ValidTime.Until != "" {
		result["valid_time"] = map[string]any{"from": claim.ValidTime.From, "until": claim.ValidTime.Until}
	}
	if len(claim.Relations.Supersedes)+len(claim.Relations.Contradicts)+len(claim.Relations.DerivedFrom) > 0 {
		result["relations"] = map[string]any{"supersedes": claim.Relations.Supersedes, "contradicts": claim.Relations.Contradicts, "derived_from": claim.Relations.DerivedFrom}
	}
	for key, value := range map[string]string{"section_ref": claim.SectionRef, "stale_after": claim.StaleAfter} {
		if strings.TrimSpace(value) != "" {
			result[key] = strings.TrimSpace(value)
		}
	}
	return result
}

func claimObjectMap(object okf.ClaimObject) map[string]any {
	result := map[string]any{}
	if object.Ref != "" {
		result["ref"] = object.Ref
	} else {
		result["value"] = object.Value
	}
	for key, value := range map[string]string{"datatype": object.Datatype, "language": object.Language, "unit": object.Unit, "quantity_kind": object.QuantityKind} {
		if value != "" {
			result[key] = value
		}
	}
	return result
}

func claimEvidenceMap(evidence okf.ClaimEvidence) map[string]any {
	result := map[string]any{"id": evidence.ID, "source_ref": evidence.SourceRef, "stance": evidence.Stance, "role": evidence.Role}
	if evidence.ObservedAt != "" {
		result["observed_at"] = evidence.ObservedAt
	}
	if evidence.Selector != nil {
		selector := map[string]any{"type": evidence.Selector.Type}
		for key, value := range map[string]string{"value": evidence.Selector.Value, "exact": evidence.Selector.Exact, "prefix": evidence.Selector.Prefix, "suffix": evidence.Selector.Suffix, "conforms_to": evidence.Selector.ConformsTo} {
			if value != "" {
				selector[key] = value
			}
		}
		if evidence.Selector.Start != nil {
			selector["start"] = *evidence.Selector.Start
		}
		if evidence.Selector.End != nil {
			selector["end"] = *evidence.Selector.End
		}
		if evidence.Selector.Page != nil {
			selector["page"] = *evidence.Selector.Page
		}
		result["selector"] = selector
	}
	return result
}

func rewriteFrontmatterField(content []byte, field string, value any) ([]byte, error) {
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("document requires YAML frontmatter")
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return nil, fmt.Errorf("frontmatter block is not closed")
	}
	block := strings.Join(lines[1:closing], "\n")
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(block), &document); err != nil || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter must be a YAML mapping")
	}
	var replacement []string
	if value != nil {
		encoded, err := yaml.Marshal(map[string]any{field: value})
		if err != nil {
			return nil, err
		}
		replacement = strings.Split(strings.TrimSuffix(string(encoded), "\n"), "\n")
	}
	mapping := document.Content[0]
	start, end := closing, closing
	found := false
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, node := mapping.Content[index], mapping.Content[index+1]
		if key.Value != field {
			continue
		}
		start = key.Line
		end = maxYAMLLine(node) + 1
		found = true
		break
	}
	if value == nil && !found {
		return content, nil
	}
	updated := make([]string, 0, len(lines)-end+start+len(replacement))
	updated = append(updated, lines[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, lines[end:]...)
	return []byte(strings.Join(updated, "\n")), nil
}

func maxYAMLLine(node *yaml.Node) int {
	maximum := node.Line
	for _, child := range node.Content {
		if line := maxYAMLLine(child); line > maximum {
			maximum = line
		}
	}
	return maximum
}

func writeAndCheck(root string, spec string, path string, document string, original []byte, updated []byte, before Index) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := atomic.WriteFile(path, bytes.NewReader(updated)); err != nil {
		return err
	}
	if err := os.Chmod(path, info.Mode().Perm()); err != nil {
		_ = atomic.WriteFile(path, bytes.NewReader(original))
		return err
	}
	after, err := BuildIndex(root, spec, time.Now().UTC())
	if err == nil {
		err = newDocumentClaimIssue(before.Issues, after.Issues, document)
	}
	if err != nil {
		_ = atomic.WriteFile(path, bytes.NewReader(original))
		_ = os.Chmod(path, info.Mode().Perm())
		return err
	}
	return nil
}

func newDocumentClaimIssue(before []okf.Issue, after []okf.Issue, document string) error {
	existing := map[string]bool{}
	for _, issue := range before {
		if issue.Path == document {
			existing[issue.Rule+"::"+issue.Message] = true
		}
	}
	for _, issue := range after {
		if issue.Path == document && !existing[issue.Rule+"::"+issue.Message] {
			return fmt.Errorf("claim edit is invalid: %s", issue.Message)
		}
	}
	return nil
}

func copyMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func containsString(values []string, wanted string) bool {
	index := sort.SearchStrings(values, wanted)
	return index < len(values) && values[index] == wanted
}
