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

func TestValidateJobRejectsUnsafeDockerSandboxValues(t *testing.T) {
	base := Job{
		ID:    "docker-job",
		Agent: AgentSpec{Command: "agent"},
		Sandbox: SandboxSpec{
			Type:    "docker",
			Image:   "example.test/agent:latest",
			Network: "none",
		},
	}
	if err := ValidateJob(base); err != nil {
		t.Fatalf("expected supported Docker sandbox to validate: %v", err)
	}

	tests := []struct {
		name     string
		image    string
		network  string
		expected string
	}{
		{name: "option-like image", image: "--privileged", network: "none", expected: "sandbox.image"},
		{name: "whitespace image", image: "agent image", network: "none", expected: "sandbox.image"},
		{name: "host network", image: "agent:latest", network: "host", expected: "sandbox.network"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			job := base
			job.Sandbox.Image = test.image
			job.Sandbox.Network = test.network
			if err := ValidateJob(job); err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected %s validation error, got %v", test.expected, err)
			}
		})
	}
}

func TestValidateJobRequiresExplicitSafeEnvironmentNames(t *testing.T) {
	base := Job{
		ID:      "environment-job",
		Agent:   AgentSpec{Command: "agent"},
		Sandbox: SandboxSpec{Type: "host", Env: []string{"OPENAI_API_KEY", "CI"}},
	}
	if err := ValidateJob(base); err != nil {
		t.Fatalf("expected environment allowlist to validate: %v", err)
	}

	for _, environment := range [][]string{
		{"API-KEY"},
		{"HOME"},
		{"home"},
		{"TOKEN", "token"},
	} {
		job := base
		job.Sandbox.Env = environment
		if err := ValidateJob(job); err == nil || !strings.Contains(err.Error(), "sandbox.env") {
			t.Fatalf("expected environment %v to be rejected, got %v", environment, err)
		}
	}
}
