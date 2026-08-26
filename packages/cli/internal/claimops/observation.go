package claimops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

// EvidenceObservationRefreshResult describes an append-only refresh of one
// verified claim's local live evidence observations.
type EvidenceObservationRefreshResult struct {
	ClaimID  string                     `json:"claimId"`
	Document string                     `json:"document"`
	Versions []okf.ClaimEvidenceVersion `json:"versions"`
	Changed  bool                       `json:"changed"`
}

// ObserveClaimEvidenceVersions hashes the current local live resources used by
// one claim occurrence. Remote resources are intentionally skipped. For a
// pinned source, the live bytes must match the pinned artifact digest before an
// observation can be recorded.
func ObserveClaimEvidenceVersions(root string, occurrence Occurrence, by string, at time.Time) ([]okf.ClaimEvidenceVersion, error) {
	by = strings.TrimSpace(by)
	if !validEvidenceObservationActor(by) {
		return nil, fmt.Errorf("evidence observation requires by human:<id>|github:<login>|process:<id>")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	root, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return nil, err
	}
	sources := map[string]SourceRef{}
	for _, source := range occurrence.Sources {
		sources[source.ID] = source
	}
	versions := make([]okf.ClaimEvidenceVersion, 0, len(occurrence.Claim.Evidence))
	for _, evidence := range occurrence.Claim.Evidence {
		source, exists := sources[evidence.SourceRef]
		if !exists {
			return nil, fmt.Errorf("claim evidence %q source %q is unavailable", evidence.ID, evidence.SourceRef)
		}
		resource := strings.TrimSpace(source.LiveResource)
		if resource == "" {
			resource = strings.TrimSpace(source.Resource)
		}
		path, local, err := resolveLiveEvidenceResource(root, occurrence.Path, resource)
		if err != nil {
			return nil, fmt.Errorf("observe claim evidence %q: %w", evidence.ID, err)
		}
		if !local {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("observe claim evidence %q: %w", evidence.ID, err)
		}
		content, readErr := readBoundedEvidence(file)
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("observe claim evidence %q: %w", evidence.ID, readErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		digestBytes := sha256.Sum256(content)
		digest := hex.EncodeToString(digestBytes[:])
		if source.Observe == "pinned" && source.SHA256 != "" && source.SHA256 != digest {
			return nil, fmt.Errorf("claim evidence %q live source changed; pin the current bytes before refreshing verification", evidence.ID)
		}
		versions = append(versions, okf.ClaimEvidenceVersion{
			EvidenceRef: evidence.ID,
			SourceRef:   evidence.SourceRef,
			Resource:    resource,
			SHA256:      digest,
			By:          by,
			At:          at.UTC().Format(time.RFC3339),
		})
	}
	return versions, nil
}

// RefreshClaimEvidenceVersions appends fresh local observations to an existing
// verified claim. It preserves prior verification and observation history.
func RefreshClaimEvidenceVersions(root, spec, claimID, document, by string, at time.Time) (EvidenceObservationRefreshResult, error) {
	result := EvidenceObservationRefreshResult{ClaimID: strings.TrimSpace(claimID), Document: strings.TrimSpace(document), Versions: []okf.ClaimEvidenceVersion{}}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	index, err := BuildIndex(root, spec, at)
	if err != nil {
		return result, err
	}
	var matches []Occurrence
	for _, occurrence := range index.Occurrences {
		if occurrence.Claim.ID == result.ClaimID && (result.Document == "" || occurrence.Path == result.Document) {
			matches = append(matches, occurrence)
		}
	}
	if len(matches) == 0 {
		return result, fmt.Errorf("claim not found: %s", result.ClaimID)
	}
	if len(matches) != 1 {
		return result, fmt.Errorf("claim evidence refresh is ambiguous; select one declaring document")
	}
	match := matches[0]
	result.Document = match.Path
	if match.Claim.Status != "verified" || match.Claim.Verification == nil {
		return result, fmt.Errorf("claim evidence refresh requires a verified claim")
	}
	versions, err := ObserveClaimEvidenceVersions(index.Root, match, by, at)
	if err != nil {
		return result, err
	}
	if len(versions) == 0 {
		return result, fmt.Errorf("claim has no deterministically observable local evidence")
	}
	result.Versions = versions
	documentRel, path, err := resolveDocument(index.Root, match.Path)
	if err != nil {
		return result, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	updated, changed, err := appendClaimEvidenceVersionsContent(content, match.Claim, versions)
	if err != nil || !changed {
		return result, err
	}
	pageVerified := !claimStaleDateReached(match.Claim.StaleAfter, at)
	for _, occurrence := range index.Occurrences {
		if occurrence.Path != match.Path || !okf.ClaimIsActive(occurrence.Claim, at) {
			continue
		}
		if occurrence.Claim.ID == match.Claim.ID {
			continue
		}
		if occurrence.Claim.Status != "verified" || occurrence.Claim.Stale {
			pageVerified = false
			break
		}
	}
	var verified any
	if pageVerified {
		verified = []any{map[string]any{"by": by, "at": at.UTC().Format(time.RFC3339)}}
	}
	updated, err = rewriteFrontmatterField(updated, "verified", verified)
	if err != nil {
		return result, err
	}
	if err := writeAndCheck(index.Root, spec, path, documentRel, content, updated, index); err != nil {
		return result, err
	}
	result.Changed = true
	return result, nil
}

func claimStaleDateReached(value string, at time.Time) bool {
	date, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return false
	}
	current := at.UTC()
	today := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC)
	return !today.Before(date)
}

func appendClaimEvidenceVersionsContent(content []byte, target okf.Claim, versions []okf.ClaimEvidenceVersion) ([]byte, bool, error) {
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
		verification, ok := mapping["verification"].(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("verified claim has no verification mapping")
		}
		history, _ := verification["evidence_versions"].([]any)
		history = append(history, claimEvidenceVersionMaps(versions)...)
		verification["evidence_versions"] = history
		refs := claimStringListValue(verification["evidence_refs"])
		for _, version := range versions {
			refs = append(refs, version.EvidenceRef)
		}
		verification["evidence_refs"] = uniqueStrings(refs)
	}
	if matches == 0 {
		return nil, false, fmt.Errorf("claim occurrence not found: %s", target.ID)
	}
	if matches > 1 {
		return nil, false, fmt.Errorf("claim evidence refresh is ambiguous inside the document")
	}
	updated, err := rewriteFrontmatterField(content, "claims", values)
	return updated, true, err
}

func resolveLiveEvidenceResource(root, document, resource string) (string, bool, error) {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		return "", false, fmt.Errorf("live resource is empty")
	}
	parsed, err := url.Parse(resource)
	if err != nil {
		return "", false, err
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return "", false, nil
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", false, err
	}
	if filepath.IsAbs(filepath.FromSlash(path)) {
		absolute := filepath.Clean(filepath.FromSlash(path))
		relative, err := filepath.Rel(root, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", false, fmt.Errorf("live resource must stay inside the knowledge base")
		}
		return absolute, true, nil
	}
	relative := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(filepath.FromSlash(document)), filepath.FromSlash(path))))
	absolute, err := okf.ResolveBundlePath(root, relative)
	if err != nil {
		return "", false, err
	}
	return absolute, true, nil
}

func validEvidenceObservationActor(value string) bool {
	return strings.HasPrefix(value, "human:") || strings.HasPrefix(value, "github:") || strings.HasPrefix(value, "process:")
}
