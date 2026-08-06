package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

var expectedWriteCapabilities = map[string]struct{}{
	".github/workflows/release.yml:commit_release:contents":      {},
	".github/workflows/release.yml:publish_release:attestations": {},
	".github/workflows/release.yml:publish_release:contents":     {},
	".github/workflows/release.yml:publish_release:id-token":     {},
	".github/workflows/release.yml:npm:id-token":                 {},
	".github/workflows/security.yml:codeql:security-events":      {},
}

var expectedPublishSteps = []string{
	"Checkout verified release commit",
	"Setup pnpm",
	"Setup Node",
	"Install dependencies",
	"Build viewer assets",
	"Prepare release tag",
	"Run GoReleaser",
	"Attest release archives",
}

var expectedCommitSteps = []string{
	"Checkout verified source commit",
	"Set release versions",
	"Commit release versions",
}

var expectedVerifyPrefix = []string{
	"Checkout",
	"Require current default branch tip",
	"Resolve release tag",
}

const (
	expectedAttestationAction    = "actions/attest@a1948c3f048ba23858d222213b7c278aabede763"
	expectedAttestationChecksums = "dist/checksums.txt"
)

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fail(err.Error())
		return
	}
	workflowDirectory := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(workflowDirectory)
	if err != nil {
		fail(err.Error())
		return
	}

	var failures []string
	observedWrites := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yml" && filepath.Ext(entry.Name()) != ".yaml") {
			continue
		}
		path := filepath.Join(workflowDirectory, entry.Name())
		relativePath := filepath.ToSlash(filepath.Join(".github", "workflows", entry.Name()))
		source, err := os.ReadFile(path)
		if err != nil {
			fail(err.Error())
			return
		}
		var document yaml.Node
		if err := yaml.Unmarshal(source, &document); err != nil {
			fail(fmt.Sprintf("%s: invalid YAML: %v", relativePath, err))
			return
		}
		rootNode := documentRoot(&document)
		if rootNode == nil || rootNode.Kind != yaml.MappingNode {
			failures = append(failures, fmt.Sprintf("%s: workflow root must be a YAML mapping", relativePath))
			continue
		}
		inspectPermissions(relativePath, "", mappingValue(rootNode, "permissions"), observedWrites, &failures)
		jobs := mappingValue(rootNode, "jobs")
		if jobs == nil || jobs.Kind != yaml.MappingNode {
			continue
		}
		for index := 0; index+1 < len(jobs.Content); index += 2 {
			jobName := jobs.Content[index].Value
			job := dereference(jobs.Content[index+1])
			if job == nil || job.Kind != yaml.MappingNode {
				continue
			}
			inspectPermissions(relativePath, jobName, mappingValue(job, "permissions"), observedWrites, &failures)
			if relativePath == ".github/workflows/release.yml" {
				inspectReleaseJob(relativePath, jobName, job, &failures)
			}
		}
	}

	for capability := range expectedWriteCapabilities {
		if _, ok := observedWrites[capability]; !ok {
			failures = append(failures, "missing reviewed write capability: "+capability)
		}
	}
	sort.Strings(failures)
	if len(failures) > 0 {
		fmt.Fprintln(os.Stderr, "workflow permission check failed:")
		for _, failure := range failures {
			fmt.Fprintln(os.Stderr, "- "+failure)
		}
		os.Exit(1)
	}
	fmt.Println("Workflow write capabilities and release input handling are isolated to reviewed jobs")
}

func inspectPermissions(relativePath, jobName string, permissions *yaml.Node, observed map[string]struct{}, failures *[]string) {
	permissions = dereference(permissions)
	if permissions == nil {
		return
	}
	location := nodeLocation(relativePath, permissions)
	if permissions.Kind == yaml.ScalarNode {
		if strings.EqualFold(strings.TrimSpace(permissions.Value), "write-all") {
			*failures = append(*failures, fmt.Sprintf("%s: write-all is forbidden", location))
		}
		return
	}
	if permissions.Kind != yaml.MappingNode {
		*failures = append(*failures, fmt.Sprintf("%s: permissions must be a scalar or mapping", location))
		return
	}
	for index := 0; index+1 < len(permissions.Content); index += 2 {
		scope := permissions.Content[index].Value
		value := dereference(permissions.Content[index+1])
		if value == nil || value.Kind != yaml.ScalarNode || !strings.EqualFold(strings.TrimSpace(value.Value), "write") {
			continue
		}
		capability := fmt.Sprintf("%s:%s:%s", relativePath, jobName, scope)
		if jobName == "" {
			*failures = append(*failures, fmt.Sprintf("%s: write permission must be scoped to a named job", nodeLocation(relativePath, value)))
			continue
		}
		observed[capability] = struct{}{}
		if _, ok := expectedWriteCapabilities[capability]; !ok {
			*failures = append(*failures, fmt.Sprintf("%s: unexpected write capability %s on job %s", nodeLocation(relativePath, value), scope, jobName))
		}
	}
}

func inspectReleaseJob(relativePath, jobName string, job *yaml.Node, failures *[]string) {
	steps := dereference(mappingValue(job, "steps"))
	if steps == nil || steps.Kind != yaml.SequenceNode {
		return
	}
	var names []string
	var attestationAction, attestationChecksums string
	for _, rawStep := range steps.Content {
		step := dereference(rawStep)
		if step == nil || step.Kind != yaml.MappingNode {
			continue
		}
		name := scalarValue(mappingValue(step, "name"))
		names = append(names, name)
		if jobName == "publish_release" && scalarValue(mappingValue(step, "uses")) == expectedAttestationAction {
			attestationAction = expectedAttestationAction
			with := dereference(mappingValue(step, "with"))
			attestationChecksums = scalarValue(mappingValue(with, "subject-checksums"))
		}
		if jobName == "verify" && name == "Resolve release tag" {
			run := scalarValue(mappingValue(step, "run"))
			env := dereference(mappingValue(step, "env"))
			rawVersion := scalarValue(mappingValue(env, "RAW_VERSION"))
			if rawVersion != "${{ inputs.version }}" {
				*failures = append(*failures, fmt.Sprintf("%s: Resolve release tag must pass inputs.version through RAW_VERSION", nodeLocation(relativePath, step)))
			}
			if strings.Contains(run, "${{ inputs.") || !strings.Contains(run, `raw="$RAW_VERSION"`) {
				*failures = append(*failures, fmt.Sprintf("%s: Resolve release tag must not interpolate workflow inputs in shell source", nodeLocation(relativePath, step)))
			}
		}
	}
	if jobName == "publish_release" {
		if !equalStrings(names, expectedPublishSteps) {
			*failures = append(*failures, fmt.Sprintf("%s: release publish job steps changed: expected %s; got %s", relativePath, strings.Join(expectedPublishSteps, ", "), strings.Join(names, ", ")))
		}
		if attestationAction != expectedAttestationAction {
			*failures = append(*failures, fmt.Sprintf("%s: release attestation action changed: expected %s; got %s", relativePath, expectedAttestationAction, emptyAsNone(attestationAction)))
		}
		if attestationChecksums != expectedAttestationChecksums {
			*failures = append(*failures, fmt.Sprintf("%s: release attestation checksums changed: expected %s; got %s", relativePath, expectedAttestationChecksums, emptyAsNone(attestationChecksums)))
		}
	}
	if jobName == "commit_release" && !equalStrings(names, expectedCommitSteps) {
		*failures = append(*failures, fmt.Sprintf("%s: release commit job steps changed: expected %s; got %s", relativePath, strings.Join(expectedCommitSteps, ", "), strings.Join(names, ", ")))
	}
	if jobName == "verify" {
		prefix := names
		if len(prefix) > len(expectedVerifyPrefix) {
			prefix = prefix[:len(expectedVerifyPrefix)]
		}
		if !equalStrings(prefix, expectedVerifyPrefix) {
			*failures = append(*failures, fmt.Sprintf("%s: release verification prefix changed: expected %s; got %s", relativePath, strings.Join(expectedVerifyPrefix, ", "), strings.Join(prefix, ", ")))
		}
	}
}

func repositoryRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(current, ".github", "workflows")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("repository root not found from %s", current)
		}
		current = parent
	}
}

func documentRoot(document *yaml.Node) *yaml.Node {
	if document == nil || len(document.Content) == 0 {
		return nil
	}
	return dereference(document.Content[0])
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	mapping = dereference(mapping)
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return dereference(mapping.Content[index+1])
		}
	}
	return nil
}

func dereference(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

func scalarValue(node *yaml.Node) string {
	node = dereference(node)
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}

func nodeLocation(path string, node *yaml.Node) string {
	if node == nil || node.Line == 0 {
		return path
	}
	return fmt.Sprintf("%s:%d", path, node.Line)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func emptyAsNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "workflow permission check failed:")
	fmt.Fprintln(os.Stderr, "- "+message)
	os.Exit(1)
}
