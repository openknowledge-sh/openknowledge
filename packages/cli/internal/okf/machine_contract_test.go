package okf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMachineContractGoldenFiles(t *testing.T) {
	fixtures := map[string]any{
		"ast": ASTBundle{
			SchemaVersion: MachineSchemaVersion,
			Root:          "/knowledge",
			SpecVersion:   "0.1",
			Documents:     []ASTDocument{},
		},
		"bundle": Bundle{
			SchemaVersion: MachineSchemaVersion,
			Root:          "/knowledge",
			SpecVersion:   "0.1",
			Files:         []BundleFile{},
		},
		"graph": Graph{
			SchemaVersion: MachineSchemaVersion,
			Root:          "/knowledge",
			SpecVersion:   "0.1",
			Type:          GraphTypeSource,
			Nodes:         []GraphNode{},
			Edges:         []GraphEdge{},
		},
		"list": ListResult{
			SchemaVersion: MachineSchemaVersion,
			Root:          "/knowledge",
			Entries:       []ListEntry{},
		},
		"search-results": SearchResultSet{
			SchemaVersion: MachineSchemaVersion,
			Root:          "/knowledge",
			Revision:      RetrievalRevision{SpecVersion: "0.1", IndexSHA256: strings.Repeat("0", 64)},
			Query:         "authentication",
			Limit:         12,
			Results:       []SearchResult{},
		},
		"search-context": ContextResult{
			SchemaVersion:   MachineSchemaVersion,
			Root:            "/knowledge",
			Revision:        RetrievalRevision{SpecVersion: "0.1", IndexSHA256: strings.Repeat("0", 64)},
			Query:           "authentication",
			Budget:          4000,
			EstimatedTokens: 0,
			Limit:           12,
			Sources:         []ContextSource{},
			Issues:          []Issue{},
		},
		"semantic-facts": SemanticFactSet{
			SchemaVersion: SemanticFactsSchemaVersion,
			Root:          "/knowledge",
			Revision:      RetrievalRevision{SpecVersion: "0.1", IndexSHA256: strings.Repeat("0", 64)},
			Valid:         true,
			Namespaces:    []SemanticNamespace{},
			Entities:      []ClaimEntity{},
			Predicates:    []ClaimPredicate{},
			EvidenceRoles: []ClaimEvidenceRole{},
			Sources:       []SemanticSource{},
			Claims:        []SemanticClaim{},
			Evidence:      []SemanticEvidence{},
			Relations:     []SemanticRelation{},
			References:    []SemanticReference{},
		},
		"vector-search": VectorSearchResultSet{
			SchemaVersion: MachineSchemaVersion,
			Root:          "/knowledge",
			Identity: VectorIndexIdentity{
				Revision: RetrievalRevision{SpecVersion: "0.1", IndexSHA256: strings.Repeat("0", 64)},
				Model: EmbeddingModel{
					Provider: "openknowledge", ID: "hashed-word-trigram", Revision: "1",
					Dimensions: 256, Metric: EmbeddingMetricCosine,
				},
				ModelFingerprint: strings.Repeat("1", 64),
			},
			Query:   "authentication",
			Limit:   12,
			Results: []VectorSearchResult{},
		},
		"rdf-dataset": RDFDataset{
			SchemaVersion: RDFDatasetSchemaVersion,
			Root:          "/knowledge",
			Revision:      RetrievalRevision{SpecVersion: "0.1", IndexSHA256: strings.Repeat("0", 64)},
			GraphIRI:      "urn:openknowledge:revision:" + strings.Repeat("0", 64),
			Quads:         []RDFQuad{},
		},
		"datalog-query": DatalogResultSet{
			SchemaVersion: DatalogQuerySchemaVersion,
			Root:          "/knowledge",
			Revision:      RetrievalRevision{SpecVersion: "0.1", IndexSHA256: strings.Repeat("0", 64)},
			Engine:        DatalogEngine{Name: DatalogEngineName, Version: DatalogEngineVersion},
			Query:         "claim(ID, Subject, Predicate, Object)",
			RuleProfile:   DatalogProfileSafe,
			Results:       []DatalogResult{},
			Policy:        DatalogPolicyReport{AllowedAccess: []string{}},
		},
		"hybrid-query": HybridResultSet{
			SchemaVersion: HybridQuerySchemaVersion,
			Root:          "/knowledge",
			Revision:      RetrievalRevision{SpecVersion: "0.1", IndexSHA256: strings.Repeat("0", 64)},
			Query:         HybridQuery{Text: "authentication", Limit: 12},
			Routes:        []HybridRoute{},
			Fusion:        HybridFusion{Method: HybridFusionRRF, RankConstant: HybridRRFConstant},
			Results:       []HybridResult{},
			Rejections:    []HybridRejection{},
		},
		"sparql-query": SPARQLResultSet{
			SchemaVersion: SPARQLQuerySchemaVersion,
			Root:          "/knowledge",
			Revision:      RetrievalRevision{SpecVersion: "0.1", IndexSHA256: strings.Repeat("0", 64)},
			Engine:        SPARQLEngine{Name: SPARQLEngineName, Version: SPARQLEngineVersion},
			QueryType:     "select",
			Variables:     []string{},
			Bindings:      []SPARQLBinding{},
			Policy:        SPARQLPolicyReport{AllowedAccess: []string{}},
		},
		"federated-search-context": FederatedContextResult{
			SchemaVersion:  MachineSchemaVersion,
			Query:          "authentication",
			Budget:         2400,
			Limit:          12,
			Fusion:         FederatedFusion{Method: "rrf", RankConstant: 60},
			KnowledgeBases: []FederatedKnowledgeBase{},
			Sources:        []FederatedContextSource{},
		},
		"federated-search-results": FederatedSearchResultSet{
			SchemaVersion:  MachineSchemaVersion,
			Query:          "authentication",
			Limit:          12,
			Fusion:         FederatedFusion{Method: "rrf", RankConstant: 60},
			KnowledgeBases: []FederatedKnowledgeBase{},
			Results:        []FederatedSearchResult{},
		},
		"validation": Result{
			SchemaVersion: MachineSchemaVersion,
			Root:          "/knowledge",
			SpecVersion:   "0.1",
			Summary:       ValidationSummary{Status: "pass"},
			Policy:        ValidationPolicyReport{},
			Checks:        []Check{},
			Issues:        []Issue{},
			Errors:        []Issue{},
			Warnings:      []Issue{},
		},
	}

	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			actual, err := json.MarshalIndent(fixture, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			actual = append(actual, '\n')
			expected, err := os.ReadFile(filepath.Join("testdata", "contracts", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("%s machine contract changed\nwant:\n%s\ngot:\n%s", name, expected, actual)
			}
		})
	}
}

func TestMachineContractSchemasDeclareCurrentVersion(t *testing.T) {
	names := []string{"ast", "bundle", "datalog-query", "federated-search-context", "federated-search-results", "graph", "hybrid-query", "list", "rdf-dataset", "registry-list", "registry-status", "search-context", "search-results", "semantic-facts", "sparql-query", "validation", "vector-search"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "schemas", "v1", name+".schema.json"))
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			if err := json.Unmarshal(content, &schema); err != nil {
				t.Fatalf("schema must be valid JSON: %v", err)
			}
			properties, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatal("schema must declare properties")
			}
			version, ok := properties["schemaVersion"].(map[string]any)
			if !ok || version["const"] != MachineSchemaVersion {
				t.Fatalf("schemaVersion must be fixed to %q: %#v", MachineSchemaVersion, version)
			}
			required, ok := schema["required"].([]any)
			if !ok || !containsJSONSchemaString(required, "schemaVersion") {
				t.Fatal("schemaVersion must be required")
			}
			id, _ := schema["$id"].(string)
			if !strings.HasSuffix(id, "/v1/"+name+".schema.json") {
				t.Fatalf("unexpected schema id: %q", id)
			}
		})
	}
}

func containsJSONSchemaString(values []any, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
