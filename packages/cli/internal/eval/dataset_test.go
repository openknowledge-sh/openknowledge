package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDatasetParsesStrictVersionedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy.yaml")
	content := `type: openknowledge.eval
version: 1
id: deploy
defaults: {budget: 1200, limit: 8}
cases:
  - id: rollback
    question: How do we roll back a deployment?
    context: {no_expand: true}
    expect:
      sources: [operations/rollback.md]
      evidence_contains: [Restore the previous release]
      evidence_excludes: [Delete production]
      min_sources: 1
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDataset(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Dataset.ID != "deploy" || len(loaded.Dataset.Cases) != 1 || loaded.Dataset.Cases[0].Context.NoExpand == nil || !*loaded.Dataset.Cases[0].Context.NoExpand {
		t.Fatalf("unexpected dataset: %#v", loaded.Dataset)
	}
	if len(loaded.SHA256) != 64 || !filepath.IsAbs(loaded.Path) {
		t.Fatalf("expected content identity, got %#v", loaded)
	}
}

func TestLoadDatasetRejectsUnknownDuplicateAndInvalidFields(t *testing.T) {
	tests := map[string]string{
		"unknown":   "type: openknowledge.eval\nversion: 1\nid: test\nunknown: true\ncases: []\n",
		"duplicate": "type: openknowledge.eval\nversion: 1\nid: test\nid: again\ncases: []\n",
		"version":   "type: openknowledge.eval\nversion: 2\nid: test\ncases: [{id: one, question: Why?, expect: {min_sources: 1}}]\n",
		"empty":     "type: openknowledge.eval\nversion: 1\nid: test\ncases: [{id: one, question: Why?, expect: {}}]\n",
		"path":      "type: openknowledge.eval\nversion: 1\nid: test\ncases: [{id: one, question: Why?, expect: {sources: [../secret.md]}}]\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "eval.yaml")
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadDataset(path); err == nil {
				t.Fatal("expected invalid dataset to fail")
			}
		})
	}
}

func TestValidateDatasetReportsAllSemanticIssues(t *testing.T) {
	err := ValidateDataset(Dataset{Type: "other", Version: 2, ID: "", Cases: []Case{{ID: "bad id", Question: "", Expect: Expectations{}}}})
	validation, ok := err.(ValidationError)
	if !ok || len(validation.Issues) < 5 || !strings.Contains(err.Error(), "type") {
		t.Fatalf("expected accumulated issues, got %#v", err)
	}
}

func TestValidateDatasetRejectsInvalidPolicyExpectations(t *testing.T) {
	allowStale := false
	err := ValidateDataset(Dataset{Type: DatasetType, Version: DatasetVersion, ID: "policy", Cases: []Case{{
		ID: "policy", Question: "Which evidence is allowed?", Expect: Expectations{MinimumTrust: "trusted", AllowStale: &allowStale, AllowedStatuses: []string{"stable", "stable", "unknown"}},
	}}})
	if err == nil || !strings.Contains(err.Error(), "minimum_trust") {
		t.Fatalf("unexpected policy validation result: %v", err)
	}
}

func TestWriteNewDatasetCreatesPrivateValidFileAndRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated", "usage.yaml")
	dataset := Dataset{Type: DatasetType, Version: DatasetVersion, ID: "usage", Cases: []Case{{ID: "gap", Question: "How do I rollback?", Expect: Expectations{MinSources: 1}}}}
	if err := WriteNewDataset(path, dataset); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDataset(path); err != nil {
		t.Fatalf("generated dataset is not readable: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("generated usage dataset must be private, mode=%o", info.Mode().Perm())
	}
	if err := WriteNewDataset(path, dataset); err == nil || !os.IsExist(err) {
		t.Fatalf("expected safe overwrite refusal, got %v", err)
	}
}
