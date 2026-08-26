package okf

import "slices"

func defaultHybridLifecyclePolicy() HybridLifecyclePolicy {
	return HybridLifecyclePolicy{Statuses: []string{"disputed", "extracted", "proposed", "supported", "verified"}}
}

func filterSemanticFactsByLifecycle(facts SemanticFactSet, policy HybridLifecyclePolicy) (SemanticFactSet, int) {
	if len(policy.Statuses) == 0 {
		policy = defaultHybridLifecyclePolicy()
	}
	statuses := append([]string{}, policy.Statuses...)
	slices.Sort(statuses)
	statuses = slices.Compact(statuses)
	allowed := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[status] = struct{}{}
	}
	known := make(map[string]struct{}, len(facts.Claims))
	visible := make(map[string]struct{}, len(facts.Claims))
	claims := make([]SemanticClaim, 0, len(facts.Claims))
	removed := 0
	for _, claim := range facts.Claims {
		known[claim.ID] = struct{}{}
		_, statusAllowed := allowed[claim.Status]
		if !statusAllowed || (claim.Stale && !policy.IncludeStale) {
			removed++
			continue
		}
		visible[claim.ID] = struct{}{}
		claims = append(claims, claim)
	}
	evidence := make([]SemanticEvidence, 0, len(facts.Evidence))
	for _, item := range facts.Evidence {
		if _, ok := visible[item.ClaimID]; ok {
			evidence = append(evidence, item)
		}
	}
	relations := make([]SemanticRelation, 0, len(facts.Relations))
	for _, relation := range facts.Relations {
		if _, ok := visible[relation.SourceID]; !ok {
			continue
		}
		if _, targetKnown := known[relation.TargetID]; targetKnown {
			if _, ok := visible[relation.TargetID]; !ok {
				continue
			}
		}
		relations = append(relations, relation)
	}
	references := make([]SemanticReference, 0, len(facts.References))
	for _, reference := range facts.References {
		if _, ok := visible[reference.ClaimID]; ok {
			references = append(references, reference)
		}
	}
	facts.Claims = claims
	facts.Evidence = evidence
	facts.Relations = relations
	facts.References = references
	return facts, removed
}
