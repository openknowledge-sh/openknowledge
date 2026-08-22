package okf

import (
	"fmt"
	"reflect"
	"strings"
)

var builtinClaimNamespaces = map[string]string{
	"rdf":          "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
	"rdfs":         "http://www.w3.org/2000/01/rdf-schema#",
	"xsd":          "http://www.w3.org/2001/XMLSchema#",
	"skos":         "http://www.w3.org/2004/02/skos/core#",
	"prov":         "http://www.w3.org/ns/prov#",
	"sh":           "http://www.w3.org/ns/shacl#",
	"oa":           "http://www.w3.org/ns/oa#",
	"time":         "http://www.w3.org/2006/time#",
	"qudt":         "http://qudt.org/schema/qudt/",
	"unit":         "http://qudt.org/vocab/unit/",
	"quantitykind": "http://qudt.org/vocab/quantitykind/",
	"okn":          "https://openknowledge.dev/ns/",
}

func newClaimOntology() ClaimOntology {
	namespaces := map[string]string{}
	for prefix, iri := range builtinClaimNamespaces {
		namespaces[prefix] = iri
	}
	return ClaimOntology{Namespaces: namespaces, Entities: map[string]ClaimEntity{}, Predicates: map[string]ClaimPredicate{}, EvidenceRoles: map[string]ClaimEvidenceRole{}}
}

func parseClaimOntology(raw any, path string) (ClaimOntology, []Issue) {
	ontology := newClaimOntology()
	if raw == nil {
		return ontology, nil
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return ontology, []Issue{claimIssue(path, "claim_ontology must be a mapping")}
	}
	var issues []Issue
	add := func(message string) { issues = append(issues, claimIssue(path, "claim_ontology "+message)) }
	checkClaimFields(mapping, stringSet("namespaces", "entities", "predicates", "evidence_roles"), add)
	if rawNamespaces, exists := mapping["namespaces"]; exists {
		values, ok := rawNamespaces.(map[string]any)
		if !ok {
			add("namespaces must be a prefix-to-IRI mapping")
		} else {
			for prefix, rawIRI := range values {
				iri := claimString(rawIRI)
				if strings.TrimSpace(prefix) == "" || !absoluteClaimIRI(iri) {
					add(fmt.Sprintf("namespace %q must map to an absolute IRI", prefix))
					continue
				}
				ontology.Namespaces[prefix] = iri
			}
		}
	}
	parseRegistryList(mapping["entities"], "entities", path, &issues, func(item map[string]any, label string) {
		checkClaimFields(item, stringSet("id", "types", "pref_label", "alt_labels", "deprecated", "replaced_by"), add)
		id := claimString(item["id"])
		types, typesOK := claimStringList(item["types"])
		aliases, aliasesOK := claimStringList(item["alt_labels"])
		if id == "" || !typesOK || !aliasesOK {
			add(label + " requires id and string-list types/alt_labels")
			return
		}
		if _, exists := ontology.Entities[id]; exists {
			add(fmt.Sprintf("contains duplicate entity %q", id))
			return
		}
		deprecated, deprecatedOK := item["deprecated"].(bool)
		if item["deprecated"] == nil {
			deprecatedOK = true
		}
		replacedBy := claimString(item["replaced_by"])
		if !deprecatedOK || (replacedBy != "" && !deprecated) {
			add(label + " replaced_by requires deprecated: true")
			return
		}
		ontology.Entities[id] = ClaimEntity{ID: id, Types: uniqueClaimStrings(types), PrefLabel: claimString(item["pref_label"]), AltLabels: uniqueClaimStrings(aliases), Deprecated: deprecated, ReplacedBy: replacedBy}
	})
	parseRegistryList(mapping["predicates"], "predicates", path, &issues, func(item map[string]any, label string) {
		checkClaimFields(item, stringSet("id", "subject_types", "object_kind", "datatype", "quantity_kind", "canonical_unit", "required_scope", "maximum_count", "pref_label"), add)
		id := claimString(item["id"])
		subjectTypes, subjectOK := claimStringList(item["subject_types"])
		requiredScope, scopeOK := claimStringList(item["required_scope"])
		maximum, maximumOK := claimPositiveInt(item["maximum_count"])
		if id == "" || !subjectOK || !scopeOK || !maximumOK {
			add(label + " has an invalid id, list, or maximum_count")
			return
		}
		if _, exists := ontology.Predicates[id]; exists {
			add(fmt.Sprintf("contains duplicate predicate %q", id))
			return
		}
		ontology.Predicates[id] = ClaimPredicate{ID: id, SubjectTypes: uniqueClaimStrings(subjectTypes), ObjectKind: claimString(item["object_kind"]), Datatype: claimString(item["datatype"]), QuantityKind: claimString(item["quantity_kind"]), CanonicalUnit: claimString(item["canonical_unit"]), RequiredScope: uniqueClaimStrings(requiredScope), MaximumCount: maximum, PrefLabel: claimString(item["pref_label"])}
	})
	parseRegistryList(mapping["evidence_roles"], "evidence_roles", path, &issues, func(item map[string]any, label string) {
		checkClaimFields(item, stringSet("id", "pref_label"), add)
		id := claimString(item["id"])
		if id == "" {
			add(label + " id must be non-empty")
			return
		}
		ontology.EvidenceRoles[id] = ClaimEvidenceRole{ID: id, PrefLabel: claimString(item["pref_label"])}
	})
	return ontology, issues
}

func mergeClaimOntology(target *ClaimOntology, source ClaimOntology, path string) []Issue {
	var issues []Issue
	for prefix, iri := range source.Namespaces {
		if current, exists := target.Namespaces[prefix]; exists && current != iri {
			issues = append(issues, claimIssue(path, fmt.Sprintf("claim_ontology namespace %q conflicts with %q", prefix, current)))
			continue
		}
		target.Namespaces[prefix] = iri
	}
	for id, entity := range source.Entities {
		if current, exists := target.Entities[id]; exists && !reflect.DeepEqual(current, entity) {
			issues = append(issues, claimIssue(path, fmt.Sprintf("claim_ontology entity %q is declared more than once", id)))
		} else {
			target.Entities[id] = entity
		}
	}
	for id, predicate := range source.Predicates {
		if current, exists := target.Predicates[id]; exists && !reflect.DeepEqual(current, predicate) {
			issues = append(issues, claimIssue(path, fmt.Sprintf("claim_ontology predicate %q is declared more than once", id)))
		} else {
			target.Predicates[id] = predicate
		}
	}
	for id, role := range source.EvidenceRoles {
		if current, exists := target.EvidenceRoles[id]; exists && !reflect.DeepEqual(current, role) {
			issues = append(issues, claimIssue(path, fmt.Sprintf("claim_ontology evidence role %q is declared more than once", id)))
		} else {
			target.EvidenceRoles[id] = role
		}
	}
	return issues
}

func validateClaimEntityReplacements(ontology ClaimOntology, entityPaths map[string]string) []Issue {
	var issues []Issue
	for id, entity := range ontology.Entities {
		if entity.ReplacedBy == "" {
			continue
		}
		if entity.ReplacedBy == id {
			issues = append(issues, claimIssue(entityPaths[id], fmt.Sprintf("claim_ontology entity %q must not replace itself", id)))
			continue
		}
		target, exists := ontology.Entities[entity.ReplacedBy]
		if !exists {
			issues = append(issues, claimIssue(entityPaths[id], fmt.Sprintf("claim_ontology entity %q replaced_by %q is not declared", id, entity.ReplacedBy)))
		} else if target.Deprecated {
			issues = append(issues, claimIssue(entityPaths[id], fmt.Sprintf("claim_ontology entity %q replaced_by %q must be active", id, entity.ReplacedBy)))
		}
	}
	return issues
}

func parseRegistryList(raw any, name, path string, issues *[]Issue, visit func(map[string]any, string)) {
	if raw == nil {
		return
	}
	items, ok := raw.([]any)
	if !ok {
		*issues = append(*issues, claimIssue(path, "claim_ontology "+name+" must be a list"))
		return
	}
	for index, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			*issues = append(*issues, claimIssue(path, fmt.Sprintf("claim_ontology %s[%d] must be a mapping", name, index)))
			continue
		}
		visit(item, fmt.Sprintf("%s[%d]", name, index))
	}
}

func claimPositiveInt(value any) (int, bool) {
	if value == nil {
		return 0, true
	}
	integer, ok := value.(int)
	return integer, ok && integer > 0
}

func absoluteClaimIRI(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "urn:")
}

func validClaimTerm(value string, ontology ClaimOntology) bool {
	if absoluteClaimIRI(value) {
		return true
	}
	prefix, _, ok := strings.Cut(value, ":")
	if !ok || prefix == "" {
		return false
	}
	_, exists := ontology.Namespaces[prefix]
	return exists
}
