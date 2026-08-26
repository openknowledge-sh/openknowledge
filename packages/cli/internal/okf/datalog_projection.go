package okf

import (
	"encoding/json"
	"sort"
	"strconv"

	"codeberg.org/TauCeti/mangle-go/ast"
)

func datalogBaseProjection(facts SemanticFactSet) ([]ast.Atom, map[string][]SemanticProvenance) {
	byKey := map[string]ast.Atom{}
	sources := map[string][]SemanticProvenance{}
	add := func(atom ast.Atom, provenance ...SemanticProvenance) {
		key := atom.DisplayString()
		byKey[key] = atom
		for _, source := range provenance {
			sources[key] = appendUniqueProvenance(sources[key], source)
		}
	}
	claimByID := make(map[string]SemanticClaim, len(facts.Claims))
	for _, claim := range facts.Claims {
		claimByID[claim.ID] = claim
		object := datalogObjectValue(claim.Object)
		add(datalogStringAtom("claim", claim.ID, claim.Subject, claim.Predicate, object), claim.Provenance)
		add(datalogStringAtom("status", claim.ID, claim.Status), claim.Provenance)
		add(datalogStringAtom("trust", claim.ID, claim.TrustTier), claim.Provenance)
		add(datalogStringAtom("stale", claim.ID, strconv.FormatBool(claim.Stale)), claim.Provenance)
		add(datalogStringAtom("valid_time", claim.ID, claim.ValidTime.From, claim.ValidTime.Until), claim.Provenance)
		add(datalogStringAtom("source", claim.ID, claim.Provenance.Document, claim.Provenance.Locator), claim.Provenance)
		for _, scope := range claim.Scope {
			add(datalogStringAtom("scope", claim.ID, scope.Key, datalogObjectValue(scope.Value)), claim.Provenance)
		}
		metadata := claim.Object
		add(datalogStringAtom("object_metadata", claim.ID, metadata.Datatype, metadata.Language, metadata.Unit, metadata.QuantityKind), claim.Provenance)
	}
	for _, evidence := range facts.Evidence {
		claim, ok := claimByID[evidence.ClaimID]
		if !ok {
			continue
		}
		add(datalogStringAtom("evidence", evidence.ClaimID, evidence.ID, evidence.Stance), claim.Provenance)
	}
	for _, relation := range facts.Relations {
		claim, ok := claimByID[relation.SourceID]
		if !ok {
			continue
		}
		add(datalogStringAtom("relation", relation.SourceID, relation.Kind, relation.TargetID), claim.Provenance)
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	atoms := make([]ast.Atom, 0, len(keys))
	for _, key := range keys {
		atoms = append(atoms, byKey[key])
		sortProvenance(sources[key])
	}
	return atoms, sources
}

func datalogStringAtom(predicate string, values ...string) ast.Atom {
	args := make([]ast.BaseTerm, len(values))
	for index, value := range values {
		args[index] = ast.String(value)
	}
	return ast.NewAtom(predicate, args...)
}

func datalogObjectValue(object ClaimObject) string {
	if object.Ref != "" {
		return object.Ref
	}
	if text, ok := object.Value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(object.Value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func appendUniqueProvenance(existing []SemanticProvenance, candidate SemanticProvenance) []SemanticProvenance {
	for _, item := range existing {
		if item.Locator == candidate.Locator && item.Document == candidate.Document {
			return existing
		}
	}
	return append(existing, candidate)
}

func sortProvenance(sources []SemanticProvenance) {
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Document != sources[j].Document {
			return sources[i].Document < sources[j].Document
		}
		return sources[i].Locator < sources[j].Locator
	})
}
