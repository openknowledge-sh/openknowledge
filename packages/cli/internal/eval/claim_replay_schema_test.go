package eval

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestClaimReplayDatasetSchemaMatchesParserContract(t *testing.T) {
	path := filepath.Join("..", "..", "schemas", "eval", "v1", "claim-replay-dataset.schema.json")
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
	const schemaURL = "https://openknowledge.sh/schemas/cli/eval/v1/claim-replay-dataset.schema.json"
	if err := compiler.AddResource(schemaURL, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatal(err)
	}
	dataset := ClaimReplayDataset{
		Type: ClaimReplayDatasetType, Version: ClaimReplayDatasetVersion, ID: "forgetting",
		Checkpoints: []ClaimReplayCheckpoint{{
			ID: "retry-change", Revision: "abc123",
			Expectations: []ClaimReplayExpectation{{ClaimID: "urn:claim:retry-count", State: ClaimTruthRefuted}},
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
		t.Fatalf("parser-valid claim replay dataset does not satisfy published schema: %v", err)
	}
}
