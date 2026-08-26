package okf

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

type datalogProver struct {
	store    factstore.SimpleInMemoryStore
	rules    []ast.Clause
	asserted map[string][]SemanticProvenance
	maxDepth int
	profile  string
}

type datalogSubstitution map[string]ast.Constant

func (prover datalogProver) prove(atom ast.Atom, depth int, visiting map[string]bool) (DatalogProof, bool) {
	key := atom.DisplayString()
	if _, ok := prover.asserted[key]; ok {
		return DatalogProof{Kind: "asserted", Atom: key, Inputs: []DatalogProof{}}, true
	}
	if depth >= prover.maxDepth || visiting[key] {
		return DatalogProof{}, false
	}
	visiting[key] = true
	defer delete(visiting, key)
	rules := append([]ast.Clause{}, prover.rules...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].String() < rules[j].String() })
	for _, rule := range rules {
		if rule.Head.Predicate != atom.Predicate {
			continue
		}
		substitution, ok := unifyDatalogAtom(rule.Head, atom, datalogSubstitution{})
		if !ok {
			continue
		}
		inputs, _, ok := prover.provePremises(rule.Premises, 0, substitution, depth+1, visiting)
		if !ok {
			continue
		}
		ruleText := rule.String()
		ruleHash := sha256.Sum256([]byte(ruleText))
		return DatalogProof{
			Kind: "rule", Atom: key, RuleID: fmt.Sprintf("rule:%x", ruleHash[:8]), Rule: ruleText, Inputs: inputs,
		}, true
	}
	return DatalogProof{}, false
}

func (prover datalogProver) provePremises(premises []ast.Term, index int, substitution datalogSubstitution, depth int, visiting map[string]bool) ([]DatalogProof, datalogSubstitution, bool) {
	if index >= len(premises) {
		return []DatalogProof{}, substitution, true
	}
	switch premise := premises[index].(type) {
	case ast.Atom:
		query := applyDatalogSubstitution(premise, substitution)
		var candidates []ast.Atom
		if err := prover.store.GetFacts(query, func(atom ast.Atom) error {
			candidates = append(candidates, atom)
			return nil
		}); err != nil {
			return nil, nil, false
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].DisplayString() < candidates[j].DisplayString() })
		for _, candidate := range candidates {
			next, ok := unifyDatalogAtom(premise, candidate, copyDatalogSubstitution(substitution))
			if !ok {
				continue
			}
			proof, ok := prover.prove(candidate, depth, visiting)
			if !ok {
				continue
			}
			rest, final, ok := prover.provePremises(premises, index+1, next, depth, visiting)
			if ok {
				return append([]DatalogProof{proof}, rest...), final, true
			}
		}
	case ast.NegAtom:
		if prover.profile != DatalogProfileClosedWorld {
			return nil, nil, false
		}
		query := applyDatalogSubstitution(premise.Atom, substitution)
		if !query.IsGround() {
			return nil, nil, false
		}
		found := false
		_ = prover.store.GetFacts(query, func(atom ast.Atom) error {
			found = true
			return nil
		})
		if found {
			return nil, nil, false
		}
		proof := DatalogProof{Kind: "closed-world-absence", Atom: query.DisplayString(), Inputs: []DatalogProof{}}
		rest, final, ok := prover.provePremises(premises, index+1, substitution, depth, visiting)
		if ok {
			return append([]DatalogProof{proof}, rest...), final, true
		}
	}
	return nil, nil, false
}

func unifyDatalogAtom(pattern ast.Atom, fact ast.Atom, substitution datalogSubstitution) (datalogSubstitution, bool) {
	if pattern.Predicate != fact.Predicate || len(pattern.Args) != len(fact.Args) {
		return nil, false
	}
	for index, argument := range pattern.Args {
		factConstant, ok := fact.Args[index].(ast.Constant)
		if !ok {
			return nil, false
		}
		switch value := argument.(type) {
		case ast.Constant:
			if !value.Equals(factConstant) {
				return nil, false
			}
		case ast.Variable:
			if value.Symbol == "_" {
				continue
			}
			if existing, bound := substitution[value.Symbol]; bound {
				if !existing.Equals(factConstant) {
					return nil, false
				}
			} else {
				substitution[value.Symbol] = factConstant
			}
		default:
			return nil, false
		}
	}
	return substitution, true
}

func applyDatalogSubstitution(atom ast.Atom, substitution datalogSubstitution) ast.Atom {
	arguments := make([]ast.BaseTerm, len(atom.Args))
	for index, argument := range atom.Args {
		if variable, ok := argument.(ast.Variable); ok {
			if value, bound := substitution[variable.Symbol]; bound {
				arguments[index] = value
				continue
			}
		}
		arguments[index] = argument
	}
	return ast.NewAtom(atom.Predicate.Symbol, arguments...)
}

func copyDatalogSubstitution(source datalogSubstitution) datalogSubstitution {
	result := make(datalogSubstitution, len(source))
	for variable, value := range source {
		result[variable] = value
	}
	return result
}

func proofSources(proof DatalogProof, asserted map[string][]SemanticProvenance) []SemanticProvenance {
	var sources []SemanticProvenance
	var visit func(DatalogProof)
	visit = func(node DatalogProof) {
		if node.Kind == "asserted" {
			for _, source := range asserted[node.Atom] {
				sources = appendUniqueProvenance(sources, source)
			}
		}
		for _, input := range node.Inputs {
			visit(input)
		}
	}
	visit(proof)
	sortProvenance(sources)
	if sources == nil {
		return []SemanticProvenance{}
	}
	return sources
}
