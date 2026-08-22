package okf

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var claimSectionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]*$`)
var claimStatuses = stringSet("extracted", "proposed", "supported", "verified", "disputed", "rejected", "superseded", "archived")
var claimStances = stringSet("supports", "opposes", "contextualizes")
var claimSelectorTypes = stringSet("text_quote", "text_position", "fragment", "page", "media_fragment", "data_position")
var claimObjectKinds = stringSet("literal", "entity", "quantity")

// AnalyzeClaimProfile validates the authored claim graph. It does not infer
// truth, conflict, or supersession from document order or timestamps.
func AnalyzeClaimProfile(bundle ASTBundle, now time.Time) ClaimProfileBundle {
	return AnalyzeClaimProfileWithEvidenceRoot(bundle, now, "")
}

func AnalyzeClaimProfileWithEvidenceRoot(bundle ASTBundle, now time.Time, evidenceRoot string) ClaimProfileBundle {
	profile := ClaimProfileBundle{Documents: map[string]ClaimProfileSignals{}, Claims: []Claim{}, Dependents: map[string][]string{}, Ontology: newClaimOntology()}
	entityPaths := map[string]string{}
	for _, document := range bundle.Documents {
		if document.FrontmatterDiagnostic != nil || document.ReadDiagnostic != nil || document.UTF8Diagnostic != nil {
			continue
		}
		ontology, ontologyIssues := parseClaimOntology(document.Frontmatter.Data[ClaimOntologyKey], document.Rel)
		profile.Issues = append(profile.Issues, ontologyIssues...)
		for id := range ontology.Entities {
			if _, exists := entityPaths[id]; !exists {
				entityPaths[id] = document.Rel
			}
		}
		profile.Issues = append(profile.Issues, mergeClaimOntology(&profile.Ontology, ontology, document.Rel)...)
	}
	profile.Issues = append(profile.Issues, validateClaimEntityLabels(profile.Ontology, entityPaths)...)
	profile.Issues = append(profile.Issues, validateClaimEntityReplacements(profile.Ontology, entityPaths)...)
	claimDocuments := map[string]ASTDocument{}
	for _, document := range bundle.Documents {
		if document.FrontmatterDiagnostic != nil || document.ReadDiagnostic != nil || document.UTF8Diagnostic != nil {
			continue
		}
		signals, issues := parseClaimProfileDocument(document.Frontmatter.Data, document.Rel, now)
		profile.Issues = append(profile.Issues, issues...)
		if signals == nil {
			continue
		}
		profile.Documents[document.Rel] = *signals
		for _, claim := range signals.Claims {
			profile.Claims = append(profile.Claims, claim)
			claimDocuments[claim.ID] = document
		}
		for _, ref := range signals.ClaimRefs {
			profile.Dependents[ref] = append(profile.Dependents[ref], document.Rel)
		}
	}
	byID := map[string]Claim{}
	for _, claim := range profile.Claims {
		if previous, exists := byID[claim.ID]; exists {
			profile.Issues = append(profile.Issues, claimIssue(claim.DeclaringPath, fmt.Sprintf("claim id %q is not globally unique; it is also declared in %s", claim.ID, previous.DeclaringPath)))
		} else {
			byID[claim.ID] = claim
		}
		document := claimDocuments[claim.ID]
		profile.Issues = append(profile.Issues, validateClaim(bundle.Root, evidenceRoot, document, claim, profile.Ontology)...)
	}
	for _, claim := range profile.Claims {
		profile.Issues = append(profile.Issues, validateClaimRelations(claim, byID)...)
	}
	profile.Issues = append(profile.Issues, validateSupersessionCycles(profile.Claims)...)
	for ref, paths := range profile.Dependents {
		if _, exists := byID[ref]; !exists {
			for _, path := range paths {
				profile.Issues = append(profile.Issues, claimIssue(path, fmt.Sprintf("claim_ref %q does not match a claim occurrence id", ref)))
			}
		}
		profile.Dependents[ref] = uniqueClaimStrings(paths)
	}
	sort.Slice(profile.Claims, func(i, j int) bool { return profile.Claims[i].ID < profile.Claims[j].ID })
	sortIssues(profile.Issues)
	return profile
}

func validateClaimEntityLabels(ontology ClaimOntology, entityPaths map[string]string) []Issue {
	owners := map[string]string{}
	issues := []Issue{}
	ids := make([]string, 0, len(ontology.Entities))
	for id := range ontology.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		entity := ontology.Entities[id]
		if entity.Deprecated {
			continue
		}
		for _, label := range append([]string{entity.PrefLabel}, entity.AltLabels...) {
			normalized := strings.ToLower(strings.Join(strings.Fields(label), " "))
			if normalized == "" {
				continue
			}
			if previous, exists := owners[normalized]; exists && previous != id {
				issues = append(issues, claimIssue(entityPaths[id], fmt.Sprintf("claim_ontology label %q is ambiguous between entities %q and %q", label, previous, id)))
				continue
			}
			owners[normalized] = id
		}
	}
	return issues
}

func validateClaim(root string, evidenceRoot string, document ASTDocument, claim Claim, ontology ClaimOntology) []Issue {
	var issues []Issue
	add := func(message string) {
		issues = append(issues, claimIssue(document.Rel, fmt.Sprintf("claim %q %s", claim.ID, message)))
	}
	for name, term := range map[string]string{"id": claim.ID, "slot": claim.Slot, "subject": claim.Subject, "predicate": claim.Predicate} {
		if !validClaimTerm(term, ontology) {
			add(fmt.Sprintf("%s %q must be an absolute IRI or use a declared namespace", name, term))
		}
	}
	if _, ok := claimStatuses[claim.Status]; !ok {
		add(fmt.Sprintf("status %q is not supported", claim.Status))
	}
	if !claimOptionalDateTime(claim.ValidTime.From) {
		add("valid_time.from must be YYYY-MM-DD or RFC 3339")
	}
	if !claimOptionalDateTime(claim.ValidTime.Until) {
		add("valid_time.until must be YYYY-MM-DD or RFC 3339")
	}
	if start, ok := parseClaimBound(claim.ValidTime.From); ok {
		if end, ok := parseClaimBound(claim.ValidTime.Until); ok && !start.Before(end) {
			add("valid_time.from must be before valid_time.until")
		}
	}
	if !claimOptionalDate(claim.StaleAfter) {
		add("stale_after must use YYYY-MM-DD")
	}
	if predicate, exists := ontology.Predicates[claim.Predicate]; !exists {
		add(fmt.Sprintf("predicate %q is not declared in claim_ontology.predicates", claim.Predicate))
	} else {
		validateClaimAgainstPredicate(claim, predicate, ontology, add)
	}
	if entity, exists := ontology.Entities[claim.Subject]; !exists {
		add(fmt.Sprintf("subject %q is not declared in claim_ontology.entities", claim.Subject))
	} else if entity.Deprecated {
		add(fmt.Sprintf("subject %q is deprecated; use %q", claim.Subject, entity.ReplacedBy))
	}
	validateClaimObject(claim.Object, "object", ontology, add)
	for dimension, object := range claim.Scope {
		if !validClaimTerm(dimension, ontology) {
			add(fmt.Sprintf("scope dimension %q must be an absolute IRI or declared term", dimension))
		}
		validateClaimObject(object, "scope."+dimension, ontology, add)
	}
	validateClaimEvidence(root, evidenceRoot, document, claim, ontology, add)
	if claim.Status == "verified" && claim.Verification == nil {
		add("status verified requires verification")
	}
	if claim.Verification != nil {
		validateClaimVerification(claim, add)
	}
	hasStatusDecision := false
	for _, decision := range claim.Decisions {
		if _, ok := stringSet("rejected", "superseded", "archived")[decision.Action]; !ok {
			add(fmt.Sprintf("decision action %q is not supported", decision.Action))
		}
		if !strings.HasPrefix(decision.By, "human:") && !strings.HasPrefix(decision.By, "github:") {
			add(fmt.Sprintf("decision by %q must identify human: or github:", decision.By))
		}
		if !claimOptionalDateTime(decision.At) {
			add("decision at must be RFC 3339 or YYYY-MM-DD")
		}
		if decision.Action == claim.Status {
			hasStatusDecision = true
		}
	}
	if (claim.Status == "rejected" || claim.Status == "superseded" || claim.Status == "archived") && !hasStatusDecision {
		add(fmt.Sprintf("status %s requires a matching decisions event", claim.Status))
	}
	if claim.SectionRef != "" {
		if !strings.HasPrefix(claim.SectionRef, "#") || !claimSectionIDPattern.MatchString(strings.TrimPrefix(claim.SectionRef, "#")) {
			add("section_ref must start with # and contain an explicit HTML id")
		} else {
			wanted := strings.TrimPrefix(claim.SectionRef, "#")
			matches := 0
			for _, explicit := range document.Markdown.ExplicitIDs {
				if explicit.ID == wanted {
					matches++
				}
			}
			if matches != 1 {
				add(fmt.Sprintf("section_ref %q must match exactly one explicit HTML id; found %d", claim.SectionRef, matches))
			}
		}
	}
	return issues
}

func validateClaimAgainstPredicate(claim Claim, predicate ClaimPredicate, ontology ClaimOntology, add func(string)) {
	if _, ok := claimObjectKinds[predicate.ObjectKind]; !ok {
		add(fmt.Sprintf("predicate %q has unsupported object_kind %q", predicate.ID, predicate.ObjectKind))
	}
	switch predicate.ObjectKind {
	case "entity":
		if claim.Object.Ref == "" {
			add("object must be an entity ref")
		} else if _, exists := ontology.Entities[claim.Object.Ref]; !exists {
			add(fmt.Sprintf("object entity %q is not declared in claim_ontology.entities", claim.Object.Ref))
		}
	case "literal":
		if claim.Object.Ref != "" {
			add("object must be a literal")
		}
	case "quantity":
		if claim.Object.Ref != "" || claim.Object.Unit == "" || claim.Object.QuantityKind == "" {
			add("object must be a quantity with unit and quantity_kind")
		}
	}
	if len(predicate.SubjectTypes) > 0 {
		entity, exists := ontology.Entities[claim.Subject]
		matched := false
		if exists {
			for _, actual := range entity.Types {
				for _, required := range predicate.SubjectTypes {
					if actual == required {
						matched = true
					}
				}
			}
		}
		if !matched {
			add(fmt.Sprintf("subject %q does not have a type accepted by predicate %q", claim.Subject, predicate.ID))
		}
	}
	if predicate.Datatype != "" && claim.Object.Datatype != predicate.Datatype {
		add(fmt.Sprintf("object datatype %q does not match predicate datatype %q", claim.Object.Datatype, predicate.Datatype))
	}
	if predicate.QuantityKind != "" && claim.Object.QuantityKind != predicate.QuantityKind {
		add(fmt.Sprintf("object quantity_kind %q does not match predicate quantity_kind %q", claim.Object.QuantityKind, predicate.QuantityKind))
	}
	if predicate.CanonicalUnit != "" && claim.Object.Unit != predicate.CanonicalUnit {
		add(fmt.Sprintf("object unit %q must use canonical unit %q", claim.Object.Unit, predicate.CanonicalUnit))
	}
	for _, dimension := range predicate.RequiredScope {
		if _, exists := claim.Scope[dimension]; !exists {
			add(fmt.Sprintf("scope must contain required dimension %q", dimension))
		}
	}
}

func validateClaimObject(object ClaimObject, label string, ontology ClaimOntology, add func(string)) {
	if object.Ref != "" && !validClaimTerm(object.Ref, ontology) {
		add(fmt.Sprintf("%s ref %q must be an absolute IRI or declared term", label, object.Ref))
	} else if entity, exists := ontology.Entities[object.Ref]; exists && entity.Deprecated {
		add(fmt.Sprintf("%s ref %q is deprecated; use %q", label, object.Ref, entity.ReplacedBy))
	}
	for name, term := range map[string]string{"datatype": object.Datatype, "unit": object.Unit, "quantity_kind": object.QuantityKind} {
		if term != "" && !validClaimTerm(term, ontology) {
			add(fmt.Sprintf("%s %s %q must be an absolute IRI or declared term", label, name, term))
		}
	}
}

func validateClaimEvidence(root string, evidenceRoot string, document ASTDocument, claim Claim, ontology ClaimOntology, add func(string)) {
	sources := map[string]map[string]any{}
	if values, ok := document.Frontmatter.Data["sources"].([]any); ok {
		for _, item := range values {
			if source, ok := item.(map[string]any); ok {
				sources[claimString(source["id"])] = source
			}
		}
	}
	ids := map[string]struct{}{}
	for _, evidence := range claim.Evidence {
		if _, exists := ids[evidence.ID]; exists {
			add(fmt.Sprintf("evidence id %q is duplicated", evidence.ID))
		}
		ids[evidence.ID] = struct{}{}
		source, exists := sources[evidence.SourceRef]
		if !exists {
			add(fmt.Sprintf("evidence %q source_ref %q must match this document's sources[].id", evidence.ID, evidence.SourceRef))
		} else if claimString(source["resource"]) == "" {
			add(fmt.Sprintf("evidence %q source %q must contain resource", evidence.ID, evidence.SourceRef))
		}
		if _, ok := claimStances[evidence.Stance]; !ok {
			add(fmt.Sprintf("evidence %q has unsupported stance %q", evidence.ID, evidence.Stance))
		}
		if _, builtIn := map[string]struct{}{"primary": {}, "secondary": {}, "corroborating": {}, "method": {}}[evidence.Role]; !builtIn {
			if _, exists := ontology.EvidenceRoles[evidence.Role]; !exists {
				add(fmt.Sprintf("evidence %q role %q is not declared", evidence.ID, evidence.Role))
			}
		}
		if !claimOptionalDateTime(evidence.ObservedAt) {
			add(fmt.Sprintf("evidence %q observed_at must be RFC 3339 or YYYY-MM-DD", evidence.ID))
		}
		if evidence.Selector != nil {
			validateClaimSelector(evidence.ID, evidence.Selector, add)
			if source != nil {
				verifyClaimEvidenceSelector(root, evidenceRoot, document.Rel, evidence, source, add)
			}
		}
	}
}

func validateClaimSelector(evidenceID string, selector *ClaimSelector, add func(string)) {
	if _, ok := claimSelectorTypes[selector.Type]; !ok {
		add(fmt.Sprintf("evidence %q selector type %q is not supported", evidenceID, selector.Type))
		return
	}
	switch selector.Type {
	case "text_quote":
		if selector.Exact == "" {
			add(fmt.Sprintf("evidence %q text_quote selector requires exact", evidenceID))
		}
	case "text_position", "data_position":
		if selector.Start == nil || selector.End == nil || *selector.Start >= *selector.End {
			add(fmt.Sprintf("evidence %q %s selector requires start < end", evidenceID, selector.Type))
		}
	case "fragment", "media_fragment":
		if selector.Value == "" {
			add(fmt.Sprintf("evidence %q %s selector requires value", evidenceID, selector.Type))
		}
	case "page":
		if selector.Page == nil || *selector.Page < 1 {
			add(fmt.Sprintf("evidence %q page selector requires page >= 1", evidenceID))
		}
	}
}

func validateClaimVerification(claim Claim, add func(string)) {
	verification := claim.Verification
	if !claimOptionalDateTime(verification.At) {
		add("verification.at must be RFC 3339 or YYYY-MM-DD")
	}
	if !strings.Contains(verification.By, ":") {
		add("verification.by must be a namespaced actor ID")
	}
	evidenceIDs := map[string]struct{}{}
	for _, evidence := range claim.Evidence {
		evidenceIDs[evidence.ID] = struct{}{}
	}
	for _, ref := range verification.EvidenceRefs {
		if _, exists := evidenceIDs[ref]; !exists {
			add(fmt.Sprintf("verification evidence_ref %q does not match claim evidence", ref))
		}
	}
}

func validateClaimRelations(claim Claim, byID map[string]Claim) []Issue {
	var issues []Issue
	add := func(message string) {
		issues = append(issues, claimIssue(claim.DeclaringPath, fmt.Sprintf("claim %q %s", claim.ID, message)))
	}
	for kind, refs := range map[string][]string{"supersedes": claim.Relations.Supersedes, "contradicts": claim.Relations.Contradicts, "derived_from": claim.Relations.DerivedFrom} {
		for _, ref := range refs {
			if ref == claim.ID {
				add(kind + " cannot reference itself")
			} else if _, exists := byID[ref]; !exists {
				add(fmt.Sprintf("%s target %q does not match a claim occurrence id", kind, ref))
			}
		}
	}
	if claim.Status == "superseded" {
		found := false
		for _, other := range byID {
			for _, ref := range other.Relations.Supersedes {
				if ref == claim.ID {
					found = true
				}
			}
		}
		if !found {
			add("status superseded requires a successor that explicitly supersedes this occurrence")
		}
	}
	return issues
}

func validateSupersessionCycles(claims []Claim) []Issue {
	graph := map[string][]string{}
	paths := map[string]string{}
	for _, claim := range claims {
		graph[claim.ID] = claim.Relations.Supersedes
		paths[claim.ID] = claim.DeclaringPath
	}
	state := map[string]int{}
	var issues []Issue
	var visit func(string)
	visit = func(id string) {
		if state[id] == 2 {
			return
		}
		if state[id] == 1 {
			issues = append(issues, claimIssue(paths[id], fmt.Sprintf("claim supersedes relation contains a cycle at %q", id)))
			return
		}
		state[id] = 1
		for _, next := range graph[id] {
			visit(next)
		}
		state[id] = 2
	}
	for id := range graph {
		visit(id)
	}
	return issues
}

func claimIssue(path, message string) Issue {
	return Issue{Path: path, Line: 1, Rule: ClaimValidationRule, Message: message}
}
func claimOptionalDate(value string) bool {
	if value == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}
func claimOptionalDateTime(value string) bool {
	if value == "" {
		return true
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}
func parseClaimBound(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	return parsed, err == nil
}
