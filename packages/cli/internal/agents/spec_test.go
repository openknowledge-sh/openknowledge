package agents

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
