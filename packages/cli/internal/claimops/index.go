package claimops

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func BuildIndex(root string, spec string, now time.Time) (Index, error) {
	bundle, err := okf.ParseASTWithVersion(root, spec)
	if err != nil {
		return Index{}, err
	}
	profile := okf.AnalyzeClaimProfile(bundle, now)
	index := Index{
		Root: bundle.Root, SpecVersion: bundle.SpecVersion, Occurrences: []Occurrence{},
		Sources: []IndexedSource{}, Dependents: profile.Dependents,
		Ontology: profile.Ontology, Issues: append([]okf.Issue{}, profile.Issues...), documents: map[string]okf.ASTDocument{},
	}
	for _, document := range bundle.Documents {
		index.documents[document.Rel] = document
		index.Sources = append(index.Sources, indexedSources(document)...)
		signals, exists := profile.Documents[document.Rel]
		if !exists {
			continue
		}
		title := strings.TrimSpace(document.Metadata.Title)
		for _, claim := range signals.Claims {
			index.Occurrences = append(index.Occurrences, Occurrence{
				Path: document.Rel, Title: title, Claim: claim, Sources: claimSources(document, claim),
			})
		}
	}
	sort.Slice(index.Occurrences, func(i, j int) bool {
		left, right := index.Occurrences[i], index.Occurrences[j]
		if left.Claim.ID != right.Claim.ID {
			return left.Claim.ID < right.Claim.ID
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		leftValue, _ := okf.NormalizeClaimObject(left.Claim.Object)
		rightValue, _ := okf.NormalizeClaimObject(right.Claim.Object)
		return leftValue < rightValue
	})
	sort.Slice(index.Sources, func(i, j int) bool {
		if index.Sources[i].Path != index.Sources[j].Path {
			return index.Sources[i].Path < index.Sources[j].Path
		}
		return index.Sources[i].ID < index.Sources[j].ID
	})
	return index, nil
}

func FindEntities(index Index, query string) []EntityMatch {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []EntityMatch{}
	}
	references := map[string]map[string]bool{}
	for _, occurrence := range index.Occurrences {
		for _, entityID := range []string{occurrence.Claim.Subject, occurrence.Claim.Object.Ref} {
			if entityID == "" {
				continue
			}
			if references[entityID] == nil {
				references[entityID] = map[string]bool{}
			}
			references[entityID][occurrence.Claim.ID] = true
		}
	}
	matches := []EntityMatch{}
	for _, entity := range index.Ontology.Entities {
		score := 0
		reasons := []string{}
		if strings.EqualFold(entity.ID, query) {
			score, reasons = 100, append(reasons, "exact_id")
		} else if strings.Contains(strings.ToLower(entity.ID), query) {
			score, reasons = 50, append(reasons, "id_contains")
		}
		if strings.EqualFold(entity.PrefLabel, query) {
			score += 80
			reasons = append(reasons, "preferred_label")
		} else if strings.Contains(strings.ToLower(entity.PrefLabel), query) {
			score += 35
			reasons = append(reasons, "preferred_label_contains")
		}
		for _, alias := range entity.AltLabels {
			if strings.EqualFold(alias, query) {
				score += 70
				reasons = append(reasons, "alternate_label")
				break
			}
			if strings.Contains(strings.ToLower(alias), query) {
				score += 25
				reasons = append(reasons, "alternate_label_contains")
				break
			}
		}
		if score == 0 {
			continue
		}
		referencedBy := make([]string, 0, len(references[entity.ID]))
		for claimID := range references[entity.ID] {
			referencedBy = append(referencedBy, claimID)
		}
		sort.Strings(referencedBy)
		matches = append(matches, EntityMatch{Score: score, Reasons: reasons, Entity: entity, ReferencedBy: referencedBy})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Entity.ID < matches[j].Entity.ID
	})
	return matches
}

func indexedSources(document okf.ASTDocument) []IndexedSource {
	var result []IndexedSource
	values, _ := document.Frontmatter.Data["sources"].([]any)
	for _, value := range values {
		source, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id, _ := source["id"].(string)
		resource, _ := source["resource"].(string)
		role, _ := source["role"].(string)
		observe, _ := source["observe"].(string)
		sha256, _ := source["sha256"].(string)
		approvedBy, _ := source["authority_approved_by"].(string)
		id, resource = strings.TrimSpace(id), strings.TrimSpace(resource)
		if id == "" || resource == "" {
			continue
		}
		result = append(result, IndexedSource{
			Path: document.Rel, ID: id, Resource: resource, Observe: strings.TrimSpace(observe), SHA256: strings.TrimSpace(sha256), Role: strings.TrimSpace(role),
			AuthorityApprovedBy: strings.TrimSpace(approvedBy),
		})
	}
	return result
}

func claimSources(document okf.ASTDocument, claim okf.Claim) []SourceRef {
	wanted := map[string]bool{}
	for _, evidence := range claim.Evidence {
		wanted[evidence.SourceRef] = true
	}
	var result []SourceRef
	values, _ := document.Frontmatter.Data["sources"].([]any)
	for _, value := range values {
		source, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id, _ := source["id"].(string)
		id = strings.TrimSpace(id)
		if !wanted[id] {
			continue
		}
		resource, _ := source["resource"].(string)
		role, _ := source["role"].(string)
		observe, _ := source["observe"].(string)
		sha256, _ := source["sha256"].(string)
		approvedBy, _ := source["authority_approved_by"].(string)
		result = append(result, SourceRef{
			ID: id, Resource: strings.TrimSpace(resource), Observe: strings.TrimSpace(observe), SHA256: strings.TrimSpace(sha256), Role: strings.TrimSpace(role),
			AuthorityApprovedBy: strings.TrimSpace(approvedBy),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func Find(index Index, query string) []Match {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []Match{}
	}
	terms := strings.Fields(strings.NewReplacer(".", " ", "-", " ", "_", " ").Replace(query))
	var matches []Match
	for _, occurrence := range index.Occurrences {
		claim := occurrence.Claim
		value, _ := okf.NormalizeClaimObject(claim.Object)
		haystack := strings.ToLower(strings.Join([]string{
			claim.ID, claim.Slot, claim.Subject, claim.Predicate, occurrence.Path, occurrence.Title, value, fmt.Sprint(claim.Scope), evidenceSearchText(claim.Evidence),
		}, " "))
		score := 0
		var reasons []string
		switch {
		case strings.EqualFold(claim.ID, query):
			score += 100
			reasons = append(reasons, "exact_id")
		case strings.Contains(strings.ToLower(claim.ID), query):
			score += 50
			reasons = append(reasons, "id_contains")
		}
		matchedTerms := 0
		for _, term := range terms {
			if strings.Contains(haystack, term) {
				matchedTerms++
			}
		}
		if matchedTerms > 0 {
			score += matchedTerms * 5
			reasons = append(reasons, "term_match")
		}
		if score > 0 {
			matches = append(matches, Match{Score: score, Reasons: reasons, Occurrence: occurrence})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		if matches[i].Occurrence.Claim.ID != matches[j].Occurrence.Claim.ID {
			return matches[i].Occurrence.Claim.ID < matches[j].Occurrence.Claim.ID
		}
		return matches[i].Occurrence.Path < matches[j].Occurrence.Path
	})
	return matches
}

func evidenceSearchText(values []okf.ClaimEvidence) string {
	parts := make([]string, 0, len(values)*3)
	for _, evidence := range values {
		parts = append(parts, evidence.ID, evidence.SourceRef, evidence.Role, evidence.Stance)
	}
	return strings.Join(parts, " ")
}
