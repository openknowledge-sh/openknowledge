package okf

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

func BuildSemanticFacts(root string) (SemanticFactSet, error) {
	return BuildSemanticFactsWithVersion(root, LatestSpecVersion)
}

func BuildSemanticFactsWithVersion(root string, version string) (SemanticFactSet, error) {
	validation, ast, err := parseAndValidateASTBundle(root, version)
	if err != nil {
		return SemanticFactSet{}, err
	}
	return SemanticFactsFromAST(validation, ast, time.Now()), nil
}

// SemanticFactsFromAST projects one validated AST into deterministic facts.
// Adapters must consume this projection instead of parsing Markdown again.
func SemanticFactsFromAST(validation Result, ast ASTBundle, now time.Time) SemanticFactSet {
	profile := AnalyzeClaimProfile(ast, now)
	issues := semanticFactIssues(validation, profile.Issues)

	revision := RetrievalRevision{SpecVersion: validation.SpecVersion, IndexSHA256: retrievalIndexSHA256(ast)}
	sections := semanticSectionLookup(ContextIndexFromAST(validation, ast).Sections)
	documents := make(map[string]ASTDocument, len(ast.Documents))
	for _, document := range ast.Documents {
		documents[document.Rel] = document
	}

	result := SemanticFactSet{
		SchemaVersion: SemanticFactsSchemaVersion,
		Root:          validation.Root,
		Revision:      revision,
		Valid:         len(validation.Errors) == 0 && len(profile.Issues) == 0,
		Namespaces:    semanticNamespaces(profile.Ontology.Namespaces),
		Entities:      semanticEntities(profile.Ontology.Entities),
		Predicates:    semanticPredicates(profile.Ontology.Predicates),
		EvidenceRoles: semanticEvidenceRoles(profile.Ontology.EvidenceRoles),
		Sources:       semanticSources(ast.Documents),
		Claims:        []SemanticClaim{},
		Evidence:      []SemanticEvidence{},
		Relations:     []SemanticRelation{},
		References:    semanticReferences(profile.Dependents),
		Issues:        issues,
	}

	for _, claim := range profile.Claims {
		provenance := semanticClaimProvenance(documents[claim.DeclaringPath], claim, sections, revision.IndexSHA256)
		semanticClaim := SemanticClaim{
			ID: claim.ID, Slot: claim.Slot, Subject: claim.Subject, Predicate: claim.Predicate,
			Object: claim.Object, Status: claim.Status, TrustTier: claim.TrustTier,
			Owners: append([]string{}, claim.Owners...), Scope: semanticScope(claim.Scope),
			ValidTime: claim.ValidTime, Stale: claim.Stale, StaleAfter: claim.StaleAfter,
			Verification: claim.Verification, Decisions: append([]ClaimDecision{}, claim.Decisions...),
			SectionRef: claim.SectionRef, EvidenceKeys: []string{}, Provenance: provenance,
		}
		sort.Strings(semanticClaim.Owners)
		for _, evidence := range claim.Evidence {
			key := semanticEvidenceKey(claim.ID, evidence.ID)
			semanticClaim.EvidenceKeys = append(semanticClaim.EvidenceKeys, key)
			result.Evidence = append(result.Evidence, SemanticEvidence{
				Key: key, ClaimID: claim.ID, ID: evidence.ID,
				SourceKey: semanticSourceKey(claim.DeclaringPath, evidence.SourceRef), SourceRef: evidence.SourceRef,
				Stance: evidence.Stance, Role: evidence.Role, Selector: evidence.Selector, ObservedAt: evidence.ObservedAt,
			})
		}
		sort.Strings(semanticClaim.EvidenceKeys)
		result.Claims = append(result.Claims, semanticClaim)
		result.Relations = append(result.Relations, semanticClaimRelations(claim)...)
	}

	sort.Slice(result.Claims, func(i, j int) bool { return result.Claims[i].ID < result.Claims[j].ID })
	sort.Slice(result.Evidence, func(i, j int) bool { return result.Evidence[i].Key < result.Evidence[j].Key })
	sort.Slice(result.Relations, func(i, j int) bool {
		left, right := result.Relations[i], result.Relations[j]
		if left.SourceID != right.SourceID {
			return left.SourceID < right.SourceID
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.TargetID < right.TargetID
	})
	return result
}

func semanticFactIssues(validation Result, profileIssues []Issue) []Issue {
	values := make([]Issue, 0, len(validation.Errors)+len(validation.Warnings)+len(profileIssues))
	seen := map[string]struct{}{}
	for _, issue := range append(append(append([]Issue{}, validation.Errors...), validation.Warnings...), profileIssues...) {
		key := issue.Path + "\x00" + strconv.Itoa(issue.Line) + "\x00" + issue.Rule + "\x00" + issue.Severity + "\x00" + issue.Message
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, issue)
	}
	sortIssues(values)
	return values
}

func semanticNamespaces(values map[string]string) []SemanticNamespace {
	result := make([]SemanticNamespace, 0, len(values))
	for prefix, iri := range values {
		result = append(result, SemanticNamespace{Prefix: prefix, IRI: iri})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Prefix < result[j].Prefix })
	return result
}

func semanticEntities(values map[string]ClaimEntity) []ClaimEntity {
	result := make([]ClaimEntity, 0, len(values))
	for _, value := range values {
		value.Types = append([]string{}, value.Types...)
		value.AltLabels = append([]string{}, value.AltLabels...)
		sort.Strings(value.Types)
		sort.Strings(value.AltLabels)
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func semanticPredicates(values map[string]ClaimPredicate) []ClaimPredicate {
	result := make([]ClaimPredicate, 0, len(values))
	for _, value := range values {
		value.SubjectTypes = append([]string{}, value.SubjectTypes...)
		value.RequiredScope = append([]string{}, value.RequiredScope...)
		sort.Strings(value.SubjectTypes)
		sort.Strings(value.RequiredScope)
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func semanticEvidenceRoles(values map[string]ClaimEvidenceRole) []ClaimEvidenceRole {
	result := make([]ClaimEvidenceRole, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func semanticSources(documents []ASTDocument) []SemanticSource {
	result := []SemanticSource{}
	for _, document := range documents {
		values, ok := document.Frontmatter.Data["sources"].([]any)
		if !ok {
			continue
		}
		for _, raw := range values {
			value, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := claimString(value["id"])
			resource := claimString(value["resource"])
			if id == "" || resource == "" {
				continue
			}
			access, _ := claimStringList(value["access"])
			access = uniqueClaimStrings(access)
			result = append(result, SemanticSource{
				Key: semanticSourceKey(document.Rel, id), Document: document.Rel, ID: id, Resource: resource,
				Title: claimString(value["title"]), Author: claimString(value["author"]), Observe: claimString(value["observe"]),
				SHA256: claimString(value["sha256"]), Role: claimString(value["role"]), Access: access,
				AuthorityApprovedBy: claimString(value["authority_approved_by"]),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func semanticScope(values map[string]ClaimObject) []SemanticScope {
	result := make([]SemanticScope, 0, len(values))
	for key, value := range values {
		result = append(result, SemanticScope{Key: key, Value: value})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func semanticClaimRelations(claim Claim) []SemanticRelation {
	var result []SemanticRelation
	for kind, targets := range map[string][]string{
		"supersedes":   claim.Relations.Supersedes,
		"contradicts":  claim.Relations.Contradicts,
		"derived_from": claim.Relations.DerivedFrom,
	} {
		for _, target := range targets {
			result = append(result, SemanticRelation{SourceID: claim.ID, Kind: kind, TargetID: target})
		}
	}
	return result
}

func semanticReferences(values map[string][]string) []SemanticReference {
	result := []SemanticReference{}
	for claimID, documents := range values {
		for _, document := range documents {
			result = append(result, SemanticReference{Document: document, ClaimID: claimID})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ClaimID != result[j].ClaimID {
			return result[i].ClaimID < result[j].ClaimID
		}
		return result[i].Document < result[j].Document
	})
	return result
}

func semanticSourceKey(document, id string) string {
	return document + "#" + id
}

func semanticEvidenceKey(claimID, id string) string {
	return claimID + "#" + id
}

type semanticSections struct {
	byPath map[string][]ContextSection
}

func semanticSectionLookup(sections []ContextSection) semanticSections {
	result := semanticSections{byPath: map[string][]ContextSection{}}
	for _, section := range sections {
		result.byPath[section.Path] = append(result.byPath[section.Path], section)
	}
	return result
}

func semanticClaimProvenance(document ASTDocument, claim Claim, sections semanticSections, revision string) SemanticProvenance {
	available := sections.byPath[claim.DeclaringPath]
	if len(available) > 0 {
		selected := available[0]
		if claim.SectionRef != "" {
			wanted := strings.TrimPrefix(claim.SectionRef, "#")
			for _, section := range available {
				for _, anchor := range section.Anchors {
					if anchor == wanted {
						selected = section
					}
				}
			}
		}
		return SemanticProvenance{
			Document: claim.DeclaringPath, DocumentID: document.ID, Locator: selected.Locator,
			ContentSHA256: selected.ContentSHA256, LineStart: selected.LineStart, LineEnd: selected.LineEnd,
		}
	}
	digest := sectionContentSHA256(document.Content)
	return SemanticProvenance{
		Document: claim.DeclaringPath, DocumentID: document.ID,
		Locator: retrievalLocator(revision, claim.DeclaringPath, digest), ContentSHA256: digest,
	}
}
