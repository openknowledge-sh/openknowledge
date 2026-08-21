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
	installTestCodex(t, "")
	root := t.TempDir()
	runTestGit(t, root, "init")
	if err := os.MkdirAll(filepath.Join(root, "Wiki"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Wiki", "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n\n# Wiki\n\nSchema contracts.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "eval.yaml"), []byte("type: openknowledge.eval\nversion: 1\nid: schema\ncases: [{id: contracts, question: Schema contracts, expect: {min_sources: 1}}]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	jobPath := filepath.Join(root, "job.md")
	jobContent := `---
id: schema-contract
agent: {runtime: codex}
workspace: {repo: ".", base: HEAD}
concurrency: {key: schema-contract}
verify: {eval: {dataset: eval.yaml, target: Wiki, gate: all}}
---
Inspect the repository.
`
	if err := os.WriteFile(jobPath, []byte(jobContent), 0644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "add", "job.md", "eval.yaml", "Wiki")
	runTestGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "job")
	t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "job-state"))

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

func TestAgentlessRunPlanSatisfiesPublishedSchema(t *testing.T) {
	repo := t.TempDir()
	runTestGit(t, repo, "init")
	jobPath := filepath.Join(repo, "validation.md")
	content := "---\nid: validation\nworkspace: {repo: \".\", base: HEAD}\nverify: {commands: [\"true\"], eval: {dataset: eval.yaml, target: Wiki}}\n---\n"
	if err := os.WriteFile(jobPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repo, "add", "validation.md")
	runTestGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "validation")
	t.Setenv(JobsStateDirEnv, filepath.Join(t.TempDir(), "job-state"))

	job, err := ParseJobFile(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildRunPlan(job, time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := plan.JSON()
	if err != nil {
		t.Fatal(err)
	}
	schemas := compileAgentArtifactSchemas(t)
	validateAgentArtifactJSON(t, schemas[RunPlanSchemaID], encoded)
	if bytes.Contains(encoded, []byte(`"agent"`)) {
		t.Fatalf("agentless plan must omit the agent command: %s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"eval"`)) {
		t.Fatalf("eval verification plan must publish its contract: %s", encoded)
	}
}

func compileAgentArtifactSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()
	paths := []string{
		filepath.Join("..", "..", "schemas", "v1", "job-run-plan.schema.json"),
		filepath.Join("..", "..", "schemas", "v1", "job-run-record.schema.json"),
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
