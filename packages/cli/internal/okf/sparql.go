package okf

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	rdflib "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
)

var sparqlWorkers = make(chan struct{}, 4)

func DefaultSPARQLLimits() SPARQLLimits {
	return SPARQLLimits{
		MaxQueryBytes: 32 << 10, MaxDatasetQuads: 250_000, MaxResults: 1_000, Timeout: 2 * time.Second,
	}
}

func QuerySPARQL(ctx context.Context, root, query string, options SPARQLQueryOptions) (SPARQLResultSet, error) {
	return QuerySPARQLWithVersion(ctx, root, LatestSpecVersion, query, options)
}

func QuerySPARQLWithVersion(ctx context.Context, root, version, query string, options SPARQLQueryOptions) (SPARQLResultSet, error) {
	snapshot, err := BuildSPARQLSnapshotWithVersion(root, version, options)
	if err != nil {
		return SPARQLResultSet{}, err
	}
	return snapshot.Query(ctx, query)
}

func BuildSPARQLSnapshot(root string, options SPARQLQueryOptions) (*SPARQLSnapshot, error) {
	return BuildSPARQLSnapshotWithVersion(root, LatestSpecVersion, options)
}

func BuildSPARQLSnapshotWithVersion(root, version string, options SPARQLQueryOptions) (*SPARQLSnapshot, error) {
	facts, err := BuildSemanticFactsWithVersion(root, version)
	if err != nil {
		return nil, err
	}
	return SPARQLSnapshotFromFacts(facts, options)
}

func SPARQLSnapshotFromFacts(facts SemanticFactSet, options SPARQLQueryOptions) (*SPARQLSnapshot, error) {
	limits := normalizeSPARQLLimits(options.Limits)
	filtered, policy := filterSemanticFactsByAccess(facts, options.AllowedAccess)
	dataset, err := RDFDatasetFromFacts(filtered)
	if err != nil {
		return nil, err
	}
	if len(dataset.Quads) > limits.MaxDatasetQuads {
		return nil, fmt.Errorf("SPARQL dataset has %d quads; limit is %d", len(dataset.Quads), limits.MaxDatasetQuads)
	}
	graph := rdflib.NewGraph(rdflib.WithIdentifier(rdflib.NewURIRefUnsafe(dataset.GraphIRI)))
	for _, quad := range dataset.Quads {
		subject := rdflib.NewURIRefUnsafe(quad.Subject)
		predicate := rdflib.NewURIRefUnsafe(quad.Predicate)
		var object rdflib.Term
		if quad.Object.Type == RDFTermIRI {
			object = rdflib.NewURIRefUnsafe(quad.Object.Value)
		} else {
			var literalOptions []rdflib.LiteralOption
			if quad.Object.Language != "" {
				literalOptions = append(literalOptions, rdflib.WithLang(quad.Object.Language))
			} else if quad.Object.Datatype != "" {
				literalOptions = append(literalOptions, rdflib.WithDatatype(rdflib.NewURIRefUnsafe(quad.Object.Datatype)))
			}
			object = rdflib.NewLiteral(quad.Object.Value, literalOptions...)
		}
		graph.Add(subject, predicate, object)
	}
	return &SPARQLSnapshot{
		root: dataset.Root, revision: dataset.Revision, graphIRI: dataset.GraphIRI, graph: graph,
		claims: sparqlSourceClaims(filtered), policy: policy, limits: limits,
	}, nil
}

func (snapshot *SPARQLSnapshot) query(ctx context.Context, query string) (SPARQLResultSet, error) {
	if snapshot == nil || snapshot.graph == nil {
		return SPARQLResultSet{}, errors.New("SPARQL snapshot is not initialized")
	}
	if len(query) == 0 {
		return SPARQLResultSet{}, errors.New("SPARQL query is empty")
	}
	if err := ctx.Err(); err != nil {
		return SPARQLResultSet{}, fmt.Errorf("SPARQL query context is unavailable: %w", err)
	}
	if len(query) > snapshot.limits.MaxQueryBytes {
		return SPARQLResultSet{}, fmt.Errorf("SPARQL query is %d bytes; limit is %d", len(query), snapshot.limits.MaxQueryBytes)
	}
	parsed, err := sparql.Parse(query)
	if err != nil {
		return SPARQLResultSet{}, fmt.Errorf("SPARQL parse error: %w", err)
	}
	if parsed.Type != "SELECT" && parsed.Type != "ASK" {
		return SPARQLResultSet{}, fmt.Errorf("SPARQL query type %s is not allowed; use SELECT or ASK", parsed.Type)
	}
	parsed.NamedGraphs = map[string]*rdflib.Graph{snapshot.graphIRI: snapshot.graph}
	if parsed.Type == "SELECT" && (parsed.Limit < 0 || parsed.Limit > snapshot.limits.MaxResults+1) {
		parsed.Limit = snapshot.limits.MaxResults + 1
	}

	queryContext := ctx
	cancel := func() {}
	if snapshot.limits.Timeout > 0 {
		queryContext, cancel = context.WithTimeout(ctx, snapshot.limits.Timeout)
	}
	defer cancel()
	select {
	case sparqlWorkers <- struct{}{}:
	case <-queryContext.Done():
		return SPARQLResultSet{}, fmt.Errorf("SPARQL query did not start: %w", queryContext.Err())
	}
	type queryOutcome struct {
		result *sparql.Result
		err    error
	}
	outcomes := make(chan queryOutcome, 1)
	go func() {
		defer func() { <-sparqlWorkers }()
		result, queryErr := sparql.EvalQuery(snapshot.graph, parsed, nil)
		outcomes <- queryOutcome{result: result, err: queryErr}
	}()

	var outcome queryOutcome
	select {
	case outcome = <-outcomes:
	case <-queryContext.Done():
		return SPARQLResultSet{}, fmt.Errorf("SPARQL query exceeded its resource deadline: %w", queryContext.Err())
	}
	if outcome.err != nil {
		return SPARQLResultSet{}, fmt.Errorf("SPARQL evaluation failed: %w", outcome.err)
	}
	return snapshot.resultSet(parsed, outcome.result), nil
}

func (snapshot *SPARQLSnapshot) resultSet(parsed *sparql.ParsedQuery, result *sparql.Result) SPARQLResultSet {
	output := SPARQLResultSet{
		SchemaVersion: SPARQLQuerySchemaVersion, Root: snapshot.root, Revision: snapshot.revision,
		Engine: SPARQLEngine{Name: SPARQLEngineName, Version: SPARQLEngineVersion}, QueryType: strings.ToLower(result.Type),
		Variables: append([]string{}, result.Vars...), Bindings: []SPARQLBinding{}, Policy: snapshot.policy,
	}
	if result.Type == "ASK" {
		value := result.AskResult
		output.Boolean = &value
		return output
	}
	for _, row := range result.Bindings {
		values := make(map[string]SPARQLValue, len(row))
		for variable, value := range row {
			values[variable] = sparqlValue(value)
		}
		output.Bindings = append(output.Bindings, SPARQLBinding{Values: values, Sources: snapshot.sourcesFor(values)})
	}
	if len(output.Bindings) > snapshot.limits.MaxResults {
		output.Bindings = output.Bindings[:snapshot.limits.MaxResults]
		output.Truncated = true
	}
	if len(parsed.OrderBy) == 0 {
		sort.SliceStable(output.Bindings, func(i, j int) bool {
			return sparqlBindingKey(output.Bindings[i], output.Variables) < sparqlBindingKey(output.Bindings[j], output.Variables)
		})
	}
	return output
}

func normalizeSPARQLLimits(limits SPARQLLimits) SPARQLLimits {
	defaults := DefaultSPARQLLimits()
	if limits.MaxQueryBytes <= 0 {
		limits.MaxQueryBytes = defaults.MaxQueryBytes
	}
	if limits.MaxDatasetQuads <= 0 {
		limits.MaxDatasetQuads = defaults.MaxDatasetQuads
	}
	if limits.MaxResults <= 0 {
		limits.MaxResults = defaults.MaxResults
	}
	if limits.Timeout <= 0 {
		limits.Timeout = defaults.Timeout
	}
	return limits
}

func sparqlValue(value rdflib.Term) SPARQLValue {
	switch term := value.(type) {
	case rdflib.URIRef:
		return SPARQLValue{Type: SPARQLValueIRI, Value: term.Value()}
	case rdflib.Literal:
		result := SPARQLValue{Type: SPARQLValueLiteral, Value: term.Lexical(), Language: term.Language()}
		if result.Language == "" {
			result.Datatype = term.Datatype().Value()
		}
		return result
	case rdflib.BNode:
		return SPARQLValue{Type: SPARQLValueBlank, Value: term.String()}
	default:
		return SPARQLValue{Type: SPARQLValueLiteral, Value: value.String()}
	}
}

func sparqlBindingKey(binding SPARQLBinding, variables []string) string {
	var builder strings.Builder
	for _, variable := range variables {
		value := binding.Values[variable]
		builder.WriteString(variable)
		builder.WriteByte(0)
		builder.WriteString(value.Type)
		builder.WriteByte(0)
		builder.WriteString(value.Value)
		builder.WriteByte(0)
		builder.WriteString(value.Datatype)
		builder.WriteByte(0)
		builder.WriteString(value.Language)
		builder.WriteByte(0xff)
	}
	return builder.String()
}
