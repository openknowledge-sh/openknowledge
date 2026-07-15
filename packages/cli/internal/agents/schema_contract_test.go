package agents

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestRuntimeAgentArtifactsSatisfyPublishedSchemas(t *testing.T) {
	root := t.TempDir()
	runTestGit(t, root, "init")
	jobPath := filepath.Join(root, "job.md")
	jobContent := `---
id: schema-contract
agent: {command: git, args: [--version]}
workspace: {repo: ".", base: HEAD}
concurrency: {key: schema-contract}
---
Inspect the repository.
`
	if err := os.WriteFile(jobPath, []byte(jobContent), 0644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "add", "job.md")
	runTestGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "job")
	t.Setenv(AgentsStateDirEnv, filepath.Join(t.TempDir(), "agent-state"))

	job, err := ParseJobFile(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	scheduledAt := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	plan, err := BuildRunPlan(job, scheduledAt, "")
	if err != nil {
		t.Fatal(err)
	}
	planJSON, err := plan.JSON()
	if err != nil {
		t.Fatal(err)
	}
	schemas := compileAgentArtifactSchemas(t)
	validateAgentArtifactJSON(t, schemas[RunPlanSchemaID], planJSON)

	record, err := RunJob(job, RunOptions{ScheduledAt: scheduledAt})
	if err != nil {
		t.Fatal(err)
	}
	if record.SchemaVersion != "1" || record.Plan.SchemaVersion != "1" {
		t.Fatalf("missing agent artifact schema identity: %#v", record)
	}
	recordJSON, err := os.ReadFile(filepath.Join(record.Plan.RunDir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	validateAgentArtifactJSON(t, schemas[RunRecordSchemaID], recordJSON)
}

func compileAgentArtifactSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()
	root := filepath.Join("..", "..", "schemas", "v1")
	paths := []string{
		filepath.Join(root, "agent-run-plan.schema.json"),
		filepath.Join(root, "agent-run-record.schema.json"),
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	ids := []string{RunPlanSchemaID, RunRecordSchemaID}
	for index, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
		if err != nil {
			t.Fatal(err)
		}
		if err := compiler.AddResource(ids[index], document); err != nil {
			t.Fatal(err)
		}
	}
	result := make(map[string]*jsonschema.Schema, len(ids))
	for _, id := range ids {
		schema, err := compiler.Compile(id)
		if err != nil {
			t.Fatal(err)
		}
		result[id] = schema
	}
	return result
}

func validateAgentArtifactJSON(t *testing.T, schema *jsonschema.Schema, content []byte) {
	t.Helper()
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("runtime agent artifact does not satisfy its published schema: %v\n%s", err, content)
	}
}
