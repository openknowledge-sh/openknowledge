package okf

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/parse"
)

var datalogWorkers = make(chan struct{}, 4)

func DefaultDatalogLimits() DatalogLimits {
	return DatalogLimits{
		MaxQueryBytes: 32 << 10, MaxRuleBytes: 64 << 10, MaxBaseFacts: 250_000, MaxCreatedFacts: 100_000,
		MaxResults: 1_000, MaxProofDepth: 64, Timeout: 2 * time.Second,
	}
}

func QueryDatalog(ctx context.Context, root string, query DatalogQuery, options DatalogQueryOptions) (DatalogResultSet, error) {
	return QueryDatalogWithVersion(ctx, root, LatestSpecVersion, query, options)
}

func QueryDatalogWithVersion(ctx context.Context, root, version string, query DatalogQuery, options DatalogQueryOptions) (DatalogResultSet, error) {
	snapshot, err := BuildDatalogSnapshotWithVersion(root, version, options)
	if err != nil {
		return DatalogResultSet{}, err
	}
	return snapshot.Query(ctx, query)
}

func BuildDatalogSnapshot(root string, options DatalogQueryOptions) (*DatalogSnapshot, error) {
	return BuildDatalogSnapshotWithVersion(root, LatestSpecVersion, options)
}

func BuildDatalogSnapshotWithVersion(root, version string, options DatalogQueryOptions) (*DatalogSnapshot, error) {
	facts, err := BuildSemanticFactsWithVersion(root, version)
	if err != nil {
		return nil, err
	}
	return DatalogSnapshotFromFacts(facts, options)
}

func DatalogSnapshotFromFacts(facts SemanticFactSet, options DatalogQueryOptions) (*DatalogSnapshot, error) {
	if !facts.Valid {
		return nil, errors.New("semantic facts are invalid; fix validation issues before Datalog projection")
	}
	limits := normalizeDatalogLimits(options.Limits)
	filtered, structuredPolicy := filterSemanticFactsByAccess(facts, options.AllowedAccess)
	baseFacts, sources := datalogBaseProjection(filtered)
	if len(baseFacts) > limits.MaxBaseFacts {
		return nil, fmt.Errorf("Datalog projection has %d base facts; limit is %d", len(baseFacts), limits.MaxBaseFacts)
	}
	return &DatalogSnapshot{
		root: filtered.Root, revision: filtered.Revision, baseFacts: baseFacts, baseSource: sources,
		policy: DatalogPolicyReport{
			AllowedAccess: structuredPolicy.AllowedAccess, RemovedSources: structuredPolicy.RemovedSources, RemovedClaims: structuredPolicy.RemovedClaims,
		},
		limits: limits,
	}, nil
}

func (snapshot *DatalogSnapshot) query(ctx context.Context, request DatalogQuery) (DatalogResultSet, error) {
	if snapshot == nil {
		return DatalogResultSet{}, errors.New("Datalog snapshot is not initialized")
	}
	if err := ctx.Err(); err != nil {
		return DatalogResultSet{}, fmt.Errorf("Datalog query context is unavailable: %w", err)
	}
	if strings.TrimSpace(request.Query) == "" {
		return DatalogResultSet{}, errors.New("Datalog query is empty")
	}
	if len(request.Query) > snapshot.limits.MaxQueryBytes {
		return DatalogResultSet{}, fmt.Errorf("Datalog query is %d bytes; limit is %d", len(request.Query), snapshot.limits.MaxQueryBytes)
	}
	if len(request.Rules) > snapshot.limits.MaxRuleBytes {
		return DatalogResultSet{}, fmt.Errorf("Datalog rules are %d bytes; limit is %d", len(request.Rules), snapshot.limits.MaxRuleBytes)
	}
	profile := request.RuleProfile
	if profile == "" {
		profile = DatalogProfileSafe
	}
	if profile != DatalogProfileSafe && profile != DatalogProfileClosedWorld {
		return DatalogResultSet{}, fmt.Errorf("unsupported Datalog rule profile %q", profile)
	}
	queryAtom, err := parse.Atom(request.Query)
	if err != nil {
		return DatalogResultSet{}, fmt.Errorf("Datalog query parse error: %w", err)
	}
	if err := validateDatalogAtom(queryAtom); err != nil {
		return DatalogResultSet{}, fmt.Errorf("Datalog query: %w", err)
	}
	ruleUnit := parse.SourceUnit{}
	if strings.TrimSpace(request.Rules) != "" {
		ruleUnit, err = parse.Unit(strings.NewReader(request.Rules))
		if err != nil {
			return DatalogResultSet{}, fmt.Errorf("Datalog rule parse error: %w", err)
		}
	}
	if err := validateDatalogRules(ruleUnit.Clauses, profile); err != nil {
		return DatalogResultSet{}, err
	}
	clauses := make([]ast.Clause, 0, len(snapshot.baseFacts)+len(ruleUnit.Clauses))
	for _, fact := range snapshot.baseFacts {
		clauses = append(clauses, ast.Clause{Head: fact})
	}
	clauses = append(clauses, ruleUnit.Clauses...)
	unit := parse.SourceUnit{Decls: ruleUnit.Decls, Clauses: clauses}
	program, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		return DatalogResultSet{}, fmt.Errorf("Datalog rule analysis failed: %w", err)
	}

	queryContext := ctx
	cancel := func() {}
	if snapshot.limits.Timeout > 0 {
		queryContext, cancel = context.WithTimeout(ctx, snapshot.limits.Timeout)
	}
	defer cancel()
	select {
	case datalogWorkers <- struct{}{}:
	case <-queryContext.Done():
		return DatalogResultSet{}, fmt.Errorf("Datalog query did not start: %w", queryContext.Err())
	}
	type evaluation struct {
		store factstore.SimpleInMemoryStore
		err   error
	}
	evaluations := make(chan evaluation, 1)
	go func() {
		defer func() { <-datalogWorkers }()
		store := factstore.NewSimpleInMemoryStore()
		err := engine.EvalProgram(program, store, engine.WithCreatedFactLimit(snapshot.limits.MaxCreatedFacts))
		evaluations <- evaluation{store: store, err: err}
	}()
	var evaluated evaluation
	select {
	case evaluated = <-evaluations:
	case <-queryContext.Done():
		return DatalogResultSet{}, fmt.Errorf("Datalog query exceeded its resource deadline: %w", queryContext.Err())
	}
	if evaluated.err != nil {
		return DatalogResultSet{}, fmt.Errorf("Datalog evaluation failed: %w", evaluated.err)
	}

	atoms, truncated, err := datalogQueryFacts(evaluated.store, queryAtom, snapshot.limits.MaxResults)
	if err != nil {
		return DatalogResultSet{}, err
	}
	proofs := datalogProver{
		store: evaluated.store, rules: program.Rules, asserted: snapshot.baseSource, maxDepth: snapshot.limits.MaxProofDepth, profile: profile,
	}
	results := make([]DatalogResult, 0, len(atoms))
	for _, atom := range atoms {
		proof, ok := proofs.prove(atom, 0, map[string]bool{})
		if !ok {
			return DatalogResultSet{}, fmt.Errorf("Datalog result %s has no reproducible proof within depth %d", atom.DisplayString(), snapshot.limits.MaxProofDepth)
		}
		kind := DatalogResultDerived
		if _, asserted := snapshot.baseSource[atom.DisplayString()]; asserted {
			kind = DatalogResultAsserted
		}
		results = append(results, DatalogResult{
			Kind: kind, Atom: atom.DisplayString(), Predicate: atom.Predicate.Symbol, Values: datalogValues(atom),
			Sources: proofSources(proof, snapshot.baseSource), Proof: proof,
		})
	}
	return DatalogResultSet{
		SchemaVersion: DatalogQuerySchemaVersion, Root: snapshot.root, Revision: snapshot.revision,
		Engine: DatalogEngine{Name: DatalogEngineName, Version: DatalogEngineVersion}, Query: request.Query,
		RuleProfile: profile, Results: results, Truncated: truncated, Policy: snapshot.policy,
	}, nil
}

func normalizeDatalogLimits(limits DatalogLimits) DatalogLimits {
	defaults := DefaultDatalogLimits()
	if limits.MaxQueryBytes <= 0 {
		limits.MaxQueryBytes = defaults.MaxQueryBytes
	}
	if limits.MaxRuleBytes <= 0 {
		limits.MaxRuleBytes = defaults.MaxRuleBytes
	}
	if limits.MaxBaseFacts <= 0 {
		limits.MaxBaseFacts = defaults.MaxBaseFacts
	}
	if limits.MaxCreatedFacts <= 0 {
		limits.MaxCreatedFacts = defaults.MaxCreatedFacts
	}
	if limits.MaxResults <= 0 {
		limits.MaxResults = defaults.MaxResults
	}
	if limits.MaxProofDepth <= 0 {
		limits.MaxProofDepth = defaults.MaxProofDepth
	}
	if limits.Timeout <= 0 {
		limits.Timeout = defaults.Timeout
	}
	return limits
}

func validateDatalogRules(clauses []ast.Clause, profile string) error {
	for _, clause := range clauses {
		if clause.Premises == nil {
			return fmt.Errorf("Datalog rules cannot assert new base facts: %s", clause.String())
		}
		if clause.Transform != nil || clause.HeadTime != nil {
			return fmt.Errorf("Datalog safe profiles do not allow transforms or temporal annotations: %s", clause.String())
		}
		if err := validateDatalogAtom(clause.Head); err != nil {
			return fmt.Errorf("Datalog rule head: %w", err)
		}
		for _, premise := range clause.Premises {
			switch term := premise.(type) {
			case ast.Atom:
				if err := validateDatalogAtom(term); err != nil {
					return fmt.Errorf("Datalog rule premise: %w", err)
				}
			case ast.NegAtom:
				if profile != DatalogProfileClosedWorld {
					return fmt.Errorf("Datalog negation requires explicit rule profile %q", DatalogProfileClosedWorld)
				}
				if err := validateDatalogAtom(term.Atom); err != nil {
					return fmt.Errorf("Datalog negated premise: %w", err)
				}
			default:
				return fmt.Errorf("Datalog safe profiles allow only positive atoms and declared closed-world negation, got %T", premise)
			}
		}
	}
	return nil
}

func validateDatalogAtom(atom ast.Atom) error {
	for _, argument := range atom.Args {
		switch argument.(type) {
		case ast.Variable, ast.Constant:
		default:
			return fmt.Errorf("predicate %s contains unsupported argument %T", atom.Predicate.Symbol, argument)
		}
	}
	return nil
}

func datalogQueryFacts(store factstore.SimpleInMemoryStore, query ast.Atom, limit int) ([]ast.Atom, bool, error) {
	var atoms []ast.Atom
	errStop := errors.New("Datalog result limit reached")
	err := store.GetFacts(query, func(atom ast.Atom) error {
		atoms = append(atoms, atom)
		if len(atoms) > limit {
			return errStop
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStop) {
		return nil, false, err
	}
	sort.Slice(atoms, func(i, j int) bool { return atoms[i].DisplayString() < atoms[j].DisplayString() })
	truncated := len(atoms) > limit
	if truncated {
		atoms = atoms[:limit]
	}
	return atoms, truncated, nil
}

func datalogValues(atom ast.Atom) []DatalogValue {
	values := make([]DatalogValue, 0, len(atom.Args))
	for _, argument := range atom.Args {
		constant, ok := argument.(ast.Constant)
		if !ok {
			values = append(values, DatalogValue{Type: "term", Value: argument.String()})
			continue
		}
		typeName := "constant"
		switch constant.Type {
		case ast.StringType:
			typeName = "string"
		case ast.NameType:
			typeName = "name"
		case ast.NumberType, ast.Float64Type:
			typeName = "number"
		case ast.TimeType:
			typeName = "time"
		}
		value := constant.DisplayString()
		if constant.Type == ast.StringType || constant.Type == ast.NameType {
			value = constant.Symbol
		}
		values = append(values, DatalogValue{Type: typeName, Value: value})
	}
	return values
}
