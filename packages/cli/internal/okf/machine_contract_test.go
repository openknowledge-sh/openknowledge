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
			Query:         "authentication",
			Limit:         12,
			Results:       []SearchResult{},
		},
		"search-context": ContextResult{
			SchemaVersion:   MachineSchemaVersion,
			Root:            "/knowledge",
			Query:           "authentication",
			Budget:          4000,
			EstimatedTokens: 0,
			Limit:           12,
			Sources:         []ContextSource{},
			Issues:          []Issue{},
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
	names := []string{"ast", "bundle", "graph", "list", "registry-list", "registry-status", "search-context", "search-results", "validation"}
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
