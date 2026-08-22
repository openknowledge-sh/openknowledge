package okf

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

var claimAllowedFields = stringSet(
	"id", "slot", "subject", "predicate", "object", "scope", "evidence", "owners",
	"status", "valid_time", "stale_after", "verification", "relations", "section_ref",
	"decisions",
)

var claimObjectAllowedFields = stringSet("ref", "value", "datatype", "language", "unit", "quantity_kind")
var claimEvidenceAllowedFields = stringSet("id", "source_ref", "stance", "role", "selector", "observed_at")
var claimSelectorAllowedFields = stringSet("type", "value", "exact", "prefix", "suffix", "start", "end", "page", "conforms_to")
var claimVerificationAllowedFields = stringSet("method", "by", "at", "evidence_refs")
var claimRelationsAllowedFields = stringSet("supersedes", "contradicts", "derived_from")
var claimTimeAllowedFields = stringSet("from", "until")

func DeriveClaimProfileSignals(frontmatter map[string]any) *ClaimProfileSignals {
	return DeriveClaimProfileSignalsAt(frontmatter, "", time.Now())
}

func DeriveClaimProfileSignalsAt(frontmatter map[string]any, path string, now time.Time) *ClaimProfileSignals {
	signals, _ := parseClaimProfileDocument(frontmatter, path, now)
	return signals
}

func parseClaimProfileDocument(frontmatter map[string]any, path string, now time.Time) (*ClaimProfileSignals, []Issue) {
	rawClaims, hasClaims := frontmatter["claims"]
	rawRefs, hasRefs := frontmatter["claim_refs"]
	activation, activated := frontmatter[ClaimProfileActivationKey]
	if !hasClaims && !hasRefs && !activated {
		return nil, nil
	}
	var issues []Issue
	add := func(message string) { issues = append(issues, claimIssue(path, message)) }
	if !activated || claimString(activation) != ClaimProfileVersionV1 {
		add(fmt.Sprintf("%s must be %q when claims or claim_refs are present", ClaimProfileActivationKey, ClaimProfileVersionV1))
	}
	signals := &ClaimProfileSignals{Profile: ClaimProfileIDV1, Claims: []Claim{}, ClaimRefs: []string{}}
	if rawOntology, exists := frontmatter[ClaimOntologyKey]; exists {
		ontology, _ := parseClaimOntology(rawOntology, path)
		signals.Ontology = &ontology
	}
	refs, refsOK := claimStringList(rawRefs)
	if hasRefs && !refsOK {
		add("claim_refs must be a list of non-empty claim occurrence IDs")
	} else {
		signals.ClaimRefs = uniqueClaimStrings(refs)
	}
	if !hasClaims {
		return signals, issues
	}
	items, ok := rawClaims.([]any)
	if !ok {
		add("claims must be a YAML list")
		return signals, issues
	}
	for index, raw := range items {
		mapping, ok := raw.(map[string]any)
		if !ok {
			add(fmt.Sprintf("claims[%d] must be a mapping", index))
			continue
		}
		claim, valid := parseClaim(mapping, fmt.Sprintf("claims[%d]", index), path, claimOwners(frontmatter), now, &issues)
		if valid {
			signals.Claims = append(signals.Claims, claim)
		}
	}
	sort.Slice(signals.Claims, func(i, j int) bool { return signals.Claims[i].ID < signals.Claims[j].ID })
	return signals, issues
}

func parseClaim(mapping map[string]any, label, path string, inheritedOwners []string, now time.Time, issues *[]Issue) (Claim, bool) {
	valid := true
	add := func(message string) { *issues = append(*issues, claimIssue(path, label+" "+message)); valid = false }
	checkClaimFields(mapping, claimAllowedFields, add)
	claim := Claim{
		ID: claimString(mapping["id"]), Slot: claimString(mapping["slot"]), Subject: claimString(mapping["subject"]),
		Predicate: claimString(mapping["predicate"]), Status: claimString(mapping["status"]),
		StaleAfter: claimString(mapping["stale_after"]), SectionRef: claimString(mapping["section_ref"]),
		DeclaringPath: path, Owners: inheritedOwners, Evidence: []ClaimEvidence{}, Scope: map[string]ClaimObject{},
	}
	for _, required := range []struct{ name, value string }{{"id", claim.ID}, {"slot", claim.Slot}, {"subject", claim.Subject}, {"predicate", claim.Predicate}} {
		if required.value == "" {
			add(required.name + " must be a non-empty identifier")
		}
	}
	if claim.Status == "" {
		claim.Status = "extracted"
	}
	if owners, ok := claimStringList(mapping["owners"]); ok && len(owners) > 0 {
		claim.Owners = uniqueClaimStrings(owners)
	} else if _, exists := mapping["owners"]; exists && !ok {
		add("owners must be a list of non-empty actor IDs")
	}
	object, ok := parseClaimObject(mapping["object"], label+".object", path, issues)
	if !ok {
		valid = false
	} else {
		claim.Object = object
	}
	if rawScope, exists := mapping["scope"]; exists {
		scope, ok := rawScope.(map[string]any)
		if !ok || len(scope) == 0 {
			add("scope must be a non-empty mapping of dimensions to typed objects")
		} else {
			for key, raw := range scope {
				key = strings.TrimSpace(key)
				value, objectOK := parseClaimObject(raw, label+".scope."+key, path, issues)
				if key == "" || !objectOK {
					valid = false
					continue
				}
				claim.Scope[key] = value
			}
		}
	}
	if rawEvidence, exists := mapping["evidence"]; exists {
		items, ok := rawEvidence.([]any)
		if !ok {
			add("evidence must be a list")
		} else {
			for index, raw := range items {
				evidence, evidenceOK := parseClaimEvidence(raw, fmt.Sprintf("%s.evidence[%d]", label, index), path, issues)
				if !evidenceOK {
					valid = false
				} else {
					claim.Evidence = append(claim.Evidence, evidence)
				}
			}
		}
	}
	if raw, exists := mapping["valid_time"]; exists {
		value, ok := raw.(map[string]any)
		if !ok {
			add("valid_time must be a mapping")
		} else {
			checkClaimFields(value, claimTimeAllowedFields, add)
			claim.ValidTime = ClaimTimeInterval{From: claimString(value["from"]), Until: claimString(value["until"])}
		}
	}
	if raw, exists := mapping["verification"]; exists {
		verification, ok := parseClaimVerification(raw, label+".verification", path, issues)
		if !ok {
			valid = false
		} else {
			claim.Verification = verification
		}
	}
	if raw, exists := mapping["decisions"]; exists {
		items, ok := raw.([]any)
		if !ok {
			add("decisions must be a list")
		} else {
			for index, rawDecision := range items {
				decision, decisionOK := parseClaimDecision(rawDecision, fmt.Sprintf("%s.decisions[%d]", label, index), path, issues)
				if !decisionOK {
					valid = false
				} else {
					claim.Decisions = append(claim.Decisions, decision)
				}
			}
		}
	}
	if raw, exists := mapping["relations"]; exists {
		relations, ok := parseClaimRelations(raw, label+".relations", path, issues)
		if !ok {
			valid = false
		} else {
			claim.Relations = relations
		}
	}
	claim.Stale = claimDateReached(claim.StaleAfter, now)
	claim.TrustTier = claimTrustTier(claim)
	return claim, valid
}

func parseClaimDecision(raw any, label, path string, issues *[]Issue) (ClaimDecision, bool) {
	mapping, ok := raw.(map[string]any)
	if !ok {
		*issues = append(*issues, claimIssue(path, label+" must be a mapping"))
		return ClaimDecision{}, false
	}
	valid := true
	add := func(message string) { *issues = append(*issues, claimIssue(path, label+" "+message)); valid = false }
	checkClaimFields(mapping, stringSet("action", "by", "at", "reason"), add)
	decision := ClaimDecision{Action: claimString(mapping["action"]), By: claimString(mapping["by"]), At: claimString(mapping["at"]), Reason: claimString(mapping["reason"])}
	if decision.Action == "" || decision.By == "" || decision.At == "" {
		add("action, by, and at are required")
	}
	return decision, valid
}

func parseClaimObject(raw any, label, path string, issues *[]Issue) (ClaimObject, bool) {
	mapping, ok := raw.(map[string]any)
	if !ok {
		*issues = append(*issues, claimIssue(path, label+" must be a typed object mapping"))
		return ClaimObject{}, false
	}
	valid := true
	add := func(message string) { *issues = append(*issues, claimIssue(path, label+" "+message)); valid = false }
	checkClaimFields(mapping, claimObjectAllowedFields, add)
	object := ClaimObject{Ref: claimString(mapping["ref"]), Value: mapping["value"], Datatype: claimString(mapping["datatype"]), Language: claimString(mapping["language"]), Unit: claimString(mapping["unit"]), QuantityKind: claimString(mapping["quantity_kind"])}
	_, hasValue := mapping["value"]
	if (object.Ref == "") == !hasValue {
		add("must contain exactly one of ref or value")
	}
	if hasValue && !claimScalar(object.Value) {
		add("value must be a string, number, or boolean")
	}
	if text, ok := object.Value.(string); ok && strings.TrimSpace(text) == "" {
		add("value must not be empty")
	}
	if object.Ref != "" && (object.Datatype != "" || object.Language != "" || object.Unit != "" || object.QuantityKind != "") {
		add("ref cannot have literal metadata")
	}
	if object.Language != "" && object.Datatype != "" && object.Datatype != "rdf:langString" {
		add("language requires datatype rdf:langString or no datatype")
	}
	if object.Language != "" && object.Datatype == "" {
		object.Datatype = "rdf:langString"
	}
	if hasValue && object.Datatype == "" {
		object.Datatype = inferredClaimDatatype(object.Value)
	}
	if (object.Unit == "") != (object.QuantityKind == "") {
		add("unit and quantity_kind must be present together")
	}
	return object, valid
}

func parseClaimEvidence(raw any, label, path string, issues *[]Issue) (ClaimEvidence, bool) {
	mapping, ok := raw.(map[string]any)
	if !ok {
		*issues = append(*issues, claimIssue(path, label+" must be a mapping"))
		return ClaimEvidence{}, false
	}
	valid := true
	add := func(message string) { *issues = append(*issues, claimIssue(path, label+" "+message)); valid = false }
	checkClaimFields(mapping, claimEvidenceAllowedFields, add)
	evidence := ClaimEvidence{ID: claimString(mapping["id"]), SourceRef: claimString(mapping["source_ref"]), Stance: claimString(mapping["stance"]), Role: claimString(mapping["role"]), ObservedAt: claimString(mapping["observed_at"])}
	if evidence.ID == "" {
		add("id must be non-empty")
	}
	if evidence.SourceRef == "" {
		add("source_ref must be non-empty")
	}
	if evidence.Stance == "" {
		evidence.Stance = "supports"
	}
	if evidence.Role == "" {
		evidence.Role = "primary"
	}
	if rawSelector, exists := mapping["selector"]; exists {
		selector, selectorOK := parseClaimSelector(rawSelector, label+".selector", path, issues)
		if !selectorOK {
			valid = false
		} else {
			evidence.Selector = selector
		}
	}
	return evidence, valid
}

func parseClaimSelector(raw any, label, path string, issues *[]Issue) (*ClaimSelector, bool) {
	mapping, ok := raw.(map[string]any)
	if !ok {
		*issues = append(*issues, claimIssue(path, label+" must be a mapping"))
		return nil, false
	}
	valid := true
	add := func(message string) { *issues = append(*issues, claimIssue(path, label+" "+message)); valid = false }
	checkClaimFields(mapping, claimSelectorAllowedFields, add)
	selector := &ClaimSelector{Type: claimString(mapping["type"]), Value: claimString(mapping["value"]), Exact: claimString(mapping["exact"]), Prefix: claimString(mapping["prefix"]), Suffix: claimString(mapping["suffix"]), ConformsTo: claimString(mapping["conforms_to"])}
	selector.Start = claimOptionalInt(mapping["start"], "start", add)
	selector.End = claimOptionalInt(mapping["end"], "end", add)
	selector.Page = claimOptionalInt(mapping["page"], "page", add)
	if selector.Type == "" {
		add("type must be non-empty")
	}
	return selector, valid
}

func parseClaimVerification(raw any, label, path string, issues *[]Issue) (*ClaimVerification, bool) {
	mapping, ok := raw.(map[string]any)
	if !ok {
		*issues = append(*issues, claimIssue(path, label+" must be a mapping"))
		return nil, false
	}
	valid := true
	add := func(message string) { *issues = append(*issues, claimIssue(path, label+" "+message)); valid = false }
	checkClaimFields(mapping, claimVerificationAllowedFields, add)
	refs, ok := claimStringList(mapping["evidence_refs"])
	if !ok {
		add("evidence_refs must be a list of evidence IDs")
	}
	verification := &ClaimVerification{Method: claimString(mapping["method"]), By: claimString(mapping["by"]), At: claimString(mapping["at"]), EvidenceRefs: uniqueClaimStrings(refs)}
	if verification.Method == "" || verification.By == "" || verification.At == "" {
		add("method, by, and at are required")
	}
	return verification, valid
}

func parseClaimRelations(raw any, label, path string, issues *[]Issue) (ClaimRelations, bool) {
	mapping, ok := raw.(map[string]any)
	if !ok {
		*issues = append(*issues, claimIssue(path, label+" must be a mapping"))
		return ClaimRelations{}, false
	}
	valid := true
	add := func(message string) { *issues = append(*issues, claimIssue(path, label+" "+message)); valid = false }
	checkClaimFields(mapping, claimRelationsAllowedFields, add)
	read := func(key string) []string {
		values, ok := claimStringList(mapping[key])
		if !ok {
			add(key + " must be a list of claim IDs")
			return nil
		}
		return uniqueClaimStrings(values)
	}
	return ClaimRelations{Supersedes: read("supersedes"), Contradicts: read("contradicts"), DerivedFrom: read("derived_from")}, valid
}

func checkClaimFields(mapping map[string]any, allowed map[string]struct{}, add func(string)) {
	for key := range mapping {
		if _, ok := allowed[key]; !ok {
			add(fmt.Sprintf("contains unknown field %q", key))
		}
	}
}

func claimOptionalInt(value any, name string, add func(string)) *int {
	if value == nil {
		return nil
	}
	integer, ok := value.(int)
	if !ok || integer < 0 {
		add(name + " must be a non-negative integer")
		return nil
	}
	return &integer
}

func inferredClaimDatatype(value any) string {
	switch value.(type) {
	case string:
		return "xsd:string"
	case bool:
		return "xsd:boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "xsd:integer"
	default:
		return "xsd:decimal"
	}
}

func claimString(value any) string { text, _ := value.(string); return strings.TrimSpace(text) }

func claimStringList(value any) ([]string, bool) {
	if value == nil {
		return nil, true
	}
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value := claimString(item)
		if value == "" {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

func claimOwners(frontmatter map[string]any) []string {
	var owners []string
	for _, key := range []string{"owner", "owners"} {
		switch value := frontmatter[key].(type) {
		case string:
			owners = append(owners, value)
		case []any:
			for _, item := range value {
				owners = append(owners, claimString(item))
			}
		}
	}
	return uniqueClaimStrings(owners)
}

func claimScalar(value any) bool {
	switch value.(type) {
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

func uniqueClaimStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func claimDateReached(value string, now time.Time) bool {
	if value == "" {
		return false
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return false
	}
	current := now.UTC()
	today := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC)
	return !today.Before(date)
}

func claimTrustTier(claim Claim) string {
	if claim.Status != "verified" || claim.Verification == nil {
		return OKFV02TrustUnverified
	}
	if strings.HasPrefix(claim.Verification.By, "human:") || strings.HasPrefix(claim.Verification.By, "github:") {
		return OKFV02TrustHumanReviewed
	}
	return OKFV02TrustMachineConfirmed
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
