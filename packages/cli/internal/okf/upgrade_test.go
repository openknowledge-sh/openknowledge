package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradePlanMigratesExplicitVersionAndIsIdempotent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Wiki")
	if _, err := NewProject(NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "0.1", Rules: []string{"project", "writing"}}); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildUpgradePlan(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if plan.From != "0.1" || plan.To != "0.2" || len(plan.Changes) == 0 || len(plan.SemanticIssues) != 0 {
		t.Fatalf("plan=%#v", plan)
	}
	if err := ApplyUpgradePlan(plan); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(root, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `okf_version: "0.2"`) {
		t.Fatalf("index was not upgraded:\n%s", index)
	}
	for _, path := range []string{"AGENTS.md", "SETUP.MD"} {
		content, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "--spec 0.1") || !strings.Contains(string(content), "--spec 0.2") {
			t.Fatalf("managed instructions were not upgraded in %s:\n%s", path, content)
		}
	}
	second, err := BuildUpgradePlan(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Changes) != 0 || len(second.SemanticIssues) != 0 {
		t.Fatalf("second plan must be empty: %#v", second)
	}
}

func TestUpgradeRejectsUnsupportedVersionPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Wiki")
	if _, err := NewProject(NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "0.2", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildUpgradePlan(root, "0.1"); err == nil || !strings.Contains(err.Error(), "unsupported OKF upgrade path") {
		t.Fatalf("err=%v", err)
	}
}

func TestUpgradeSameVersionRepairsGeneratedSpecThenBecomesIdempotent(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Wiki")
	if _, err := NewProject(NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "0.2", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "SPEC.md", "---\ntype: Reference\n---\n\n# stale generated spec\n")
	plan, err := BuildUpgradePlan(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Path != "SPEC.md" {
		t.Fatalf("plan=%#v", plan)
	}
	if err := ApplyUpgradePlan(plan); err != nil {
		t.Fatal(err)
	}
	second, err := BuildUpgradePlan(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Changes) != 0 {
		t.Fatalf("second plan=%#v", second)
	}
}
