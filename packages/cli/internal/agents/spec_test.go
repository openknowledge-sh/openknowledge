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
verify: {commands: ["go test ./...", "openknowledge validate Wiki"], timeout: 10m}
output: {commit: true}
concurrency: {key: wiki-maintenance}
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
	if !job.Output.Commit || job.Verify.Timeout != "10m" || job.Prompt != "\nReview the docs.\n" {
		t.Fatalf("unexpected output or prompt: %#v", job)
	}
	if job.Concurrency.Key != "wiki-maintenance" || job.Concurrency.Policy != "skip" {
		t.Fatalf("expected default skip concurrency policy: %#v", job.Concurrency)
	}
}

func TestDiscoverJobsLenientKeepsValidJobsBesideInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "00-invalid.md")
	if err := os.WriteFile(invalidPath, []byte("---\nid: invalid\nagent: {command: agent, argz: []}\n---\nPrompt.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	validPath := filepath.Join(dir, "10-valid.md")
	if err := os.WriteFile(validPath, []byte("---\nid: valid\nagent: {command: agent}\n---\nPrompt.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	jobs, failures, err := DiscoverJobsLenient(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ID != "valid" {
		t.Fatalf("expected the valid job to survive discovery, got %#v", jobs)
	}
	if len(failures) != 1 || failures[0].Path != invalidPath || !strings.Contains(failures[0].Error(), "agent.argz") {
		t.Fatalf("expected one path-bound validation failure, got %#v", failures)
	}
	if _, err := DiscoverJobs(dir); err == nil || !strings.Contains(err.Error(), "agent.argz") {
		t.Fatalf("strict discovery must retain fail-closed behavior, got %v", err)
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

func TestParseJobFileEnforcesStrictFrontmatterSchema(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "unknown top-level field",
			content:  "---\nid: strict\nagnt: {}\nagent: {command: agent}\n---\nPrompt.\n",
			expected: "agnt: is not a supported agent job field",
		},
		{
			name:     "unknown nested field",
			content:  "---\nid: strict\nagent: {command: agent, argz: []}\n---\nPrompt.\n",
			expected: "agent.argz: is not a supported agent job field",
		},
		{
			name:     "boolean string",
			content:  "---\nid: strict\nenabled: \"true\"\nagent: {command: agent}\n---\nPrompt.\n",
			expected: "enabled: must be a boolean, got string",
		},
		{
			name:     "scalar argument list",
			content:  "---\nid: strict\nagent: {command: agent, args: exec}\n---\nPrompt.\n",
			expected: "agent.args: must be a list of strings, got string",
		},
		{
			name:     "non-string argument",
			content:  "---\nid: strict\nagent: {command: agent, args: [exec, 5]}\n---\nPrompt.\n",
			expected: "agent.args[1]: must be a string, got number",
		},
		{
			name:     "unknown concurrency policy",
			content:  "---\nid: strict\nagent: {command: agent}\nconcurrency: {key: docs, policy: cancel}\n---\nPrompt.\n",
			expected: "concurrency.policy: must be skip",
		},
		{
			name:     "concurrency policy without key",
			content:  "---\nid: strict\nagent: {command: agent}\nconcurrency: {policy: skip}\n---\nPrompt.\n",
			expected: "concurrency.key: is required",
		},
		{
			name:     "duplicate field",
			content:  "---\nid: strict\nagent:\n  command: first\n  command: second\n---\nPrompt.\n",
			expected: "frontmatter: line 5: frontmatter key \"command\" is repeated",
		},
		{
			name:     "ambiguous schedule",
			content:  "---\nid: strict\nschedule: {cron: \"0 * * * *\", every: 1h}\nagent: {command: agent}\n---\nPrompt.\n",
			expected: "schedule: cron and every are mutually exclusive",
		},
		{
			name:     "non-positive timeout",
			content:  "---\nid: strict\nagent: {command: agent, timeout: 0s}\n---\nPrompt.\n",
			expected: "agent.timeout: must be positive",
		},
		{
			name:     "non-positive interval",
			content:  "---\nid: strict\nschedule: {every: -1h}\nagent: {command: agent}\n---\nPrompt.\n",
			expected: "schedule.every: must be positive",
		},
		{
			name:     "timezone without schedule",
			content:  "---\nid: strict\nschedule: {timezone: UTC}\nagent: {command: agent}\n---\nPrompt.\n",
			expected: "schedule.timezone: requires schedule.cron or schedule.every",
		},
		{
			name:     "non-positive verification timeout",
			content:  "---\nid: strict\nagent: {command: agent}\nverify: {commands: [], timeout: 0s}\n---\nPrompt.\n",
			expected: "verify.timeout: must be positive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "job.md")
			if err := os.WriteFile(path, []byte(test.content), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := ParseJobFile(path); err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected %q, got %v", test.expected, err)
			}
		})
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

func TestCanonicalPathResolvesSymlinkedParentForMissingStatePath(t *testing.T) {
	base := t.TempDir()
	realParent := filepath.Join(base, "real")
	if err := os.Mkdir(realParent, 0755); err != nil {
		t.Fatal(err)
	}
	linkedParent := filepath.Join(base, "linked")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	canonicalParent, err := filepath.EvalSymlinks(realParent)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(canonicalParent, "missing", "agents")
	got, err := canonicalPath(filepath.Join(linkedParent, "missing", "agents"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected missing descendant to resolve through real parent %s, got %s", want, got)
	}
}

func TestRepositoryStateNameSeparatesSameNamedRepositories(t *testing.T) {
	first := repositoryStateName(filepath.Join("one", "project"))
	second := repositoryStateName(filepath.Join("two", "project"))
	if first == second || !strings.HasPrefix(first, "project-") || !strings.HasPrefix(second, "project-") {
		t.Fatalf("expected stable readable per-path namespaces, first=%q second=%q", first, second)
	}
}
