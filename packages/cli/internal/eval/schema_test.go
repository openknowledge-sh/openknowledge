package eval

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestDatasetSchemaMatchesParserContract(t *testing.T) {
	path := filepath.Join("..", "..", "schemas", "eval", "v1", "dataset.schema.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource("https://openknowledge.sh/schemas/cli/eval/v1/dataset.schema.json", document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("https://openknowledge.sh/schemas/cli/eval/v1/dataset.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	noExpand := true
	allowStale := false
	requireSources := true
	minimumGroundedness := 0.75
	dataset := Dataset{
		Type: DatasetType, Version: DatasetVersion, ID: "deploy",
		Defaults: ContextSettings{Budget: 1200, Limit: 8},
		Cases: []Case{{
			ID: "rollback", Question: "How do we roll back?", Context: ContextSettings{NoExpand: &noExpand},
			Expect: Expectations{
				Sources: []string{"operations/rollback.md"}, EvidenceContains: []string{"restore"},
				AnswerContains: []string{"previous release"}, CitationSources: []string{"operations/rollback.md"},
				MinCitations: 1, MinGroundedness: &minimumGroundedness,
				MinimumTrust: "human-reviewed", AllowStale: &allowStale, AllowedStatuses: []string{"stable"}, RequireSources: &requireSources,
			},
		}},
	}
	encoded, err := json.Marshal(dataset)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("parser-valid dataset does not satisfy the published schema: %v", err)
	}
}

func TestAnswerProtocolSchemasCompile(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	commonPath := filepath.Join("..", "..", "schemas", "v1", "common.schema.json")
	common, err := os.ReadFile(commonPath)
	if err != nil {
		t.Fatal(err)
	}
	commonDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(common))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("https://openknowledge.sh/schemas/cli/v1/common.schema.json", commonDocument); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"answer-request.schema.json", "answer-response.schema.json"} {
		path := filepath.Join("..", "..", "schemas", "eval", "v1", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := compiler.AddResource("https://openknowledge.sh/schemas/cli/eval/v1/"+name, document); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"answer-request.schema.json", "answer-response.schema.json"} {
		if _, err := compiler.Compile("https://openknowledge.sh/schemas/cli/eval/v1/" + name); err != nil {
			t.Fatalf("compile %s: %v", name, err)
		}
	}
}
