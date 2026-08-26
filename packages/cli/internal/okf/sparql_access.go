package okf

import "slices"

func filterSemanticFactsByAccess(facts SemanticFactSet, allowed []string) (SemanticFactSet, SPARQLPolicyReport) {
	allowed = append([]string{}, allowed...)
	slices.Sort(allowed)
	allowed = slices.Compact(allowed)
	report := SPARQLPolicyReport{AllowedAccess: allowed}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, access := range allowed {
		allowedSet[access] = struct{}{}
	}

	visibleSources := make(map[string]struct{}, len(facts.Sources))
	filteredSources := make([]SemanticSource, 0, len(facts.Sources))
	for _, source := range facts.Sources {
		if !semanticSourceAllowed(source, allowedSet) {
			report.RemovedSources++
			continue
		}
		visibleSources[source.Key] = struct{}{}
		filteredSources = append(filteredSources, source)
	}

	visibleEvidence := make(map[string]SemanticEvidence, len(facts.Evidence))
	for _, evidence := range facts.Evidence {
		if _, allowed := visibleSources[evidence.SourceKey]; allowed {
			visibleEvidence[evidence.Key] = evidence
		}
	}

	knownClaims := make(map[string]struct{}, len(facts.Claims))
	visibleClaims := make(map[string]struct{}, len(facts.Claims))
	filteredClaims := make([]SemanticClaim, 0, len(facts.Claims))
	for _, claim := range facts.Claims {
		knownClaims[claim.ID] = struct{}{}
		visible := true
		for _, evidenceKey := range claim.EvidenceKeys {
			if _, allowed := visibleEvidence[evidenceKey]; !allowed {
				visible = false
				break
			}
		}
		if !visible {
			report.RemovedClaims++
			continue
		}
		visibleClaims[claim.ID] = struct{}{}
		filteredClaims = append(filteredClaims, claim)
	}

	filteredEvidence := make([]SemanticEvidence, 0, len(visibleEvidence))
	for _, evidence := range facts.Evidence {
		if _, visible := visibleClaims[evidence.ClaimID]; !visible {
			continue
		}
		if _, visible := visibleEvidence[evidence.Key]; visible {
			filteredEvidence = append(filteredEvidence, evidence)
		}
	}
	filteredRelations := make([]SemanticRelation, 0, len(facts.Relations))
	for _, relation := range facts.Relations {
		if _, visible := visibleClaims[relation.SourceID]; !visible {
			continue
		}
		if _, known := knownClaims[relation.TargetID]; known {
			if _, visible := visibleClaims[relation.TargetID]; !visible {
				continue
			}
		}
		filteredRelations = append(filteredRelations, relation)
	}
	filteredReferences := make([]SemanticReference, 0, len(facts.References))
	for _, reference := range facts.References {
		if _, visible := visibleClaims[reference.ClaimID]; visible {
			filteredReferences = append(filteredReferences, reference)
		}
	}

	facts.Sources = filteredSources
	facts.Claims = filteredClaims
	facts.Evidence = filteredEvidence
	facts.Relations = filteredRelations
	facts.References = filteredReferences
	return facts, report
}

func semanticSourceAllowed(source SemanticSource, allowed map[string]struct{}) bool {
	if len(source.Access) == 0 {
		return true
	}
	for _, access := range source.Access {
		if _, ok := allowed[access]; ok {
			return true
		}
	}
	return false
}
