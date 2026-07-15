package agents

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseJobFileUsesSharedYAMLParser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.md")
	content := `---
id: docs-audit
enabled: false
schedule: {every: 24h, timezone: Europe/Prague}
agent:
  command: codex
  args: [exec, --model, gpt-5]
verify: {commands: ["go test ./...", "openknowledge validate Wiki"]}
output: {commit: true}
---

Review the docs.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	job, err := ParseJobFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "docs-audit" || job.Enabled || job.Schedule.Every != "24h" || job.Schedule.Timezone != "Europe/Prague" {
		t.Fatalf("unexpected typed job scalars: %#v", job)
	}
	if !reflect.DeepEqual(job.Agent.Args, []string{"exec", "--model", "gpt-5"}) || !reflect.DeepEqual(job.Verify.Commands, []string{"go test ./...", "openknowledge validate Wiki"}) {
		t.Fatalf("unexpected flow collections: %#v", job)
	}
	if !job.Output.Commit || job.Prompt != "\nReview the docs.\n" {
		t.Fatalf("unexpected output or prompt: %#v", job)
	}
}

func TestParseJobFileRejectsMalformedNestedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.md")
	content := "---\nid: broken\nagent:\n  args: [exec, --model\n---\n\nPrompt.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseJobFile(path)
	if err == nil || !strings.Contains(err.Error(), "frontmatter YAML is invalid") {
		t.Fatalf("expected shared YAML parser error, got %v", err)
	}
}

func TestExecutorOverrideValidationFailsClosed(t *testing.T) {
	for _, valid := range []string{"", "host", "docker", " docker "} {
		normalized, err := NormalizeExecutorOverride(valid)
		if err != nil {
			t.Fatalf("expected executor %q to be accepted: %v", valid, err)
		}
		if normalized != strings.TrimSpace(valid) {
			t.Fatalf("expected executor %q to normalize to %q, got %q", valid, strings.TrimSpace(valid), normalized)
		}
	}
	for _, invalid := range []string{"doker", "HOST", "local"} {
		if _, err := NormalizeExecutorOverride(invalid); err == nil || !strings.Contains(err.Error(), "host or docker") {
			t.Fatalf("expected executor %q to be rejected, got %v", invalid, err)
		}
	}

	job := Job{
		ID:      "fail-closed",
		Agent:   AgentSpec{Command: "true"},
		Sandbox: SandboxSpec{Type: "host"},
	}
	if _, err := BuildRunPlan(job, time.Now(), "doker"); err == nil || !strings.Contains(err.Error(), "executor: must be host or docker") {
		t.Fatalf("expected planning to reject an unknown override before selecting a runner, got %v", err)
	}
}
