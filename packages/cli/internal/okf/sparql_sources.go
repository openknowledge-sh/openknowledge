package okf

import (
	"sort"
	"strings"
)

func sparqlSourceClaims(facts SemanticFactSet) []sparqlSourceClaim {
	namespaces := make(map[string]string, len(facts.Namespaces))
	for _, namespace := range facts.Namespaces {
		namespaces[namespace.Prefix] = namespace.IRI
	}
	builder := rdfDatasetBuilder{revision: facts.Revision.IndexSHA256, namespaces: namespaces}
	claims := make([]sparqlSourceClaim, 0, len(facts.Claims))
	for _, claim := range facts.Claims {
		terms := make(map[string]struct{}, 5)
		for _, raw := range []string{claim.ID, claim.Subject, claim.Predicate, claim.Slot} {
			if value, err := builder.term(raw); err == nil {
				terms[value] = struct{}{}
			}
		}
		if object, err := builder.object(claim.Object); err == nil {
			terms[object.Value] = struct{}{}
		}
		claims = append(claims, sparqlSourceClaim{terms: terms, provenance: claim.Provenance})
	}
	return claims
}

func (snapshot *SPARQLSnapshot) sourcesFor(values map[string]SPARQLValue) []SemanticProvenance {
	wanted := make(map[string]struct{}, len(values))
	for _, value := range values {
		wanted[value.Value] = struct{}{}
	}
	seen := map[string]struct{}{}
	var sources []SemanticProvenance
	for _, claim := range snapshot.claims {
		matched := false
		for term := range claim.terms {
			if _, ok := wanted[term]; ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		key := claim.provenance.Locator + "\x00" + claim.provenance.Document
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, claim.provenance)
	}
	sort.Slice(sources, func(i, j int) bool {
		left, right := sources[i], sources[j]
		return strings.Join([]string{left.Document, left.Locator}, "\x00") < strings.Join([]string{right.Document, right.Locator}, "\x00")
	})
	if sources == nil {
		return []SemanticProvenance{}
	}
	return sources
}
