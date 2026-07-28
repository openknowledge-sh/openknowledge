package main

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestInspectPermissionsRejectsWriteAll(t *testing.T) {
	t.Parallel()

	permissions := parseYAMLNode(t, "permissions: write-all\n", "permissions")
	var failures []string
	inspectPermissions(".github/workflows/example.yml", "build", permissions, map[string]struct{}{}, &failures)

	if !containsFailure(failures, "write-all is forbidden") {
		t.Fatalf("expected write-all failure, got %v", failures)
	}
}

func TestInspectPermissionsFindsQuotedAndInlineWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
	}{
		{name: "quoted", source: "permissions:\n  issues: \"write\"\n"},
		{name: "inline", source: "permissions: {contents: read, issues: write}\n"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			permissions := parseYAMLNode(t, test.source, "permissions")
			var failures []string
			inspectPermissions(".github/workflows/example.yml", "build", permissions, map[string]struct{}{}, &failures)

			if !containsFailure(failures, "unexpected write capability issues on job build") {
				t.Fatalf("expected unexpected write failure, got %v", failures)
			}
		})
	}
}

func TestInspectPermissionsRejectsWorkflowLevelWrite(t *testing.T) {
	t.Parallel()

	permissions := parseYAMLNode(t, "permissions:\n  contents: write\n", "permissions")
	var failures []string
	inspectPermissions(".github/workflows/example.yml", "", permissions, map[string]struct{}{}, &failures)

	if !containsFailure(failures, "write permission must be scoped to a named job") {
		t.Fatalf("expected workflow-level write failure, got %v", failures)
	}
}

func TestInspectReleaseJobRejectsDirectInputInterpolation(t *testing.T) {
	t.Parallel()

	job := parseYAMLNode(t, `job:
  steps:
    - name: Checkout
    - name: Require current default branch tip
    - name: Resolve release tag
      env:
        RAW_VERSION: ${{ inputs.version }}
      run: raw="${{ inputs.version }}"
`, "job")
	var failures []string
	inspectReleaseJob(".github/workflows/release.yml", "verify", job, &failures)

	if !containsFailure(failures, "must not interpolate workflow inputs in shell source") {
		t.Fatalf("expected direct interpolation failure, got %v", failures)
	}
}

func TestInspectReleaseJobAcceptsEnvironmentInput(t *testing.T) {
	t.Parallel()

	job := parseYAMLNode(t, `job:
  steps:
    - name: Checkout
    - name: Require current default branch tip
    - name: Resolve release tag
      env:
        RAW_VERSION: ${{ inputs.version }}
      run: raw="$RAW_VERSION"
`, "job")
	var failures []string
	inspectReleaseJob(".github/workflows/release.yml", "verify", job, &failures)

	if len(failures) != 0 {
		t.Fatalf("expected secure release input handling, got %v", failures)
	}
}

func parseYAMLNode(t *testing.T, source, key string) *yaml.Node {
	t.Helper()

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(source), &document); err != nil {
		t.Fatalf("parse YAML: %v", err)
	}
	return mappingValue(documentRoot(&document), key)
}

func containsFailure(failures []string, want string) bool {
	for _, failure := range failures {
		if strings.Contains(failure, want) {
			return true
		}
	}
	return false
}
