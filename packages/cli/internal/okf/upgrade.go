package okf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
)

type UpgradeChange struct {
	Action  string `json:"action"`
	Path    string `json:"path"`
	Reason  string `json:"reason"`
	before  string
	content []byte
}

type UpgradePlan struct {
	SchemaVersion  string          `json:"schemaVersion"`
	Root           string          `json:"root"`
	From           string          `json:"from"`
	To             string          `json:"to"`
	Changes        []UpgradeChange `json:"changes"`
	SemanticIssues []Issue         `json:"semanticIssues,omitempty"`
}

type versionedUpgradeMigration struct {
	From string
	To   string
}

var versionedUpgradeMigrations = map[string]versionedUpgradeMigration{
	"0.1->0.2": {From: "0.1", To: "0.2"},
}

func BuildUpgradePlan(root string, targetVersion string) (UpgradePlan, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return UpgradePlan{}, err
	}
	from, err := DeclaredBundleSpecVersion(absolute)
	if err != nil {
		return UpgradePlan{}, err
	}
	if from == "" {
		return UpgradePlan{}, fmt.Errorf("knowledge base does not declare okf_version")
	}
	if _, ok := ResolveSpecVersion(from); !ok {
		return UpgradePlan{}, fmt.Errorf("knowledge base declares unsupported OKF version: %s", from)
	}
	if _, err := LoadProjectConfig(absolute); err != nil {
		return UpgradePlan{}, fmt.Errorf("upgrade project configuration: %w", err)
	}
	to, ok := ResolveSpecVersion(targetVersion)
	if !ok {
		return UpgradePlan{}, fmt.Errorf("unsupported target OKF version: %s", strings.TrimSpace(targetVersion))
	}
	if !supportedUpgradePath(from, to) {
		return UpgradePlan{}, fmt.Errorf("unsupported OKF upgrade path: %s to %s", from, to)
	}
	plan := UpgradePlan{SchemaVersion: MachineSchemaVersion, Root: absolute, From: from, To: to}
	if from != to {
		indexPath := filepath.Join(absolute, "index.md")
		index, err := os.ReadFile(indexPath)
		if err != nil {
			return UpgradePlan{}, err
		}
		updatedIndex, err := replaceDeclaredSpecVersion(index, to)
		if err != nil {
			return UpgradePlan{}, err
		}
		plan.Changes = append(plan.Changes, UpgradeChange{Action: "update", Path: "index.md", Reason: "update declared OKF version", before: setupContentSHA256(index), content: updatedIndex})
		managedChanges, err := managedInstructionUpgradeChanges(absolute, from, to)
		if err != nil {
			return UpgradePlan{}, err
		}
		plan.Changes = append(plan.Changes, managedChanges...)
	}

	specPath := filepath.Join(absolute, "SPEC.md")
	specContent := []byte(specDocumentForVersion(to))
	if existing, err := os.ReadFile(specPath); err == nil {
		if !bytes.Equal(existing, specContent) {
			plan.Changes = append(plan.Changes, UpgradeChange{Action: "update", Path: "SPEC.md", Reason: "update pinned OKF specification", before: setupContentSHA256(existing), content: specContent})
		}
	} else if os.IsNotExist(err) {
		plan.Changes = append(plan.Changes, UpgradeChange{Action: "create", Path: "SPEC.md", Reason: "add pinned OKF specification", content: specContent})
	} else {
		return UpgradePlan{}, err
	}

	validation, err := ValidateWithVersion(absolute, to)
	if err != nil {
		return UpgradePlan{}, err
	}
	for _, issue := range validation.Errors {
		plan.SemanticIssues = append(plan.SemanticIssues, issue)
	}
	return plan, nil
}

func managedInstructionUpgradeChanges(root, from, to string) ([]UpgradeChange, error) {
	files := []struct {
		path   string
		marker string
	}{
		{path: "AGENTS.md", marker: "description: Lightweight starter rules for agents working in this Open Knowledge wiki."},
		{path: "SETUP.MD", marker: "description: Agent handoff for creating the initial local Open Knowledge wiki."},
	}
	var changes []UpgradeChange
	for _, file := range files {
		path := filepath.Join(root, file.path)
		content, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		text := string(content)
		if !strings.Contains(text, file.marker) {
			continue
		}
		updated := strings.ReplaceAll(text, "okn validate --spec "+from, "okn validate --spec "+to)
		updated = strings.ReplaceAll(updated, "uses openknowledge spec "+from+".", "uses openknowledge spec "+to+".")
		if updated == text {
			continue
		}
		changes = append(changes, UpgradeChange{
			Action: "update", Path: file.path, Reason: "update managed OKF version instructions",
			before: setupContentSHA256(content), content: []byte(updated),
		})
	}
	return changes, nil
}

func supportedUpgradePath(from, to string) bool {
	if from == to {
		return true
	}
	_, ok := versionedUpgradeMigrations[from+"->"+to]
	return ok
}

func replaceDeclaredSpecVersion(content []byte, version string) ([]byte, error) {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("index.md must declare okf_version in YAML frontmatter")
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(lines[index]), "okf_version:") {
			lines[index] = `okf_version: "` + version + `"`
			return []byte(strings.Join(lines, "\n")), nil
		}
	}
	return nil, fmt.Errorf("index.md does not declare okf_version")
}

func ApplyUpgradePlan(plan UpgradePlan) error {
	if len(plan.SemanticIssues) > 0 {
		return fmt.Errorf("upgrade requires semantic review for %d validation issues", len(plan.SemanticIssues))
	}
	if err := verifyUpgradePlan(plan); err != nil {
		return err
	}
	for _, change := range plan.Changes {
		path := filepath.Join(plan.Root, filepath.FromSlash(change.Path))
		if !insideRoot(plan.Root, path) {
			return fmt.Errorf("upgrade change escapes bundle: %s", change.Path)
		}
		if change.Action == "create" {
			if _, err := os.Lstat(path); err == nil {
				return fmt.Errorf("refusing to replace existing upgrade target: %s", change.Path)
			} else if !os.IsNotExist(err) {
				return err
			}
		} else {
			current, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if setupContentSHA256(current) != change.before {
				return fmt.Errorf("refusing to overwrite a file changed after planning: %s", change.Path)
			}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := atomic.WriteFile(path, bytes.NewReader(change.content)); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o644); err != nil {
			return err
		}
	}
	validation, err := ValidateWithVersion(plan.Root, plan.To)
	if err != nil {
		return err
	}
	if len(validation.Errors) > 0 {
		return fmt.Errorf("upgraded bundle has %d validation errors", len(validation.Errors))
	}
	return nil
}

func verifyUpgradePlan(plan UpgradePlan) error {
	for _, change := range plan.Changes {
		path := filepath.Join(plan.Root, filepath.FromSlash(change.Path))
		if !insideRoot(plan.Root, path) {
			return fmt.Errorf("upgrade change escapes bundle: %s", change.Path)
		}
		switch change.Action {
		case "create":
			if _, err := os.Lstat(path); err == nil {
				return fmt.Errorf("refusing to replace existing upgrade target: %s", change.Path)
			} else if !os.IsNotExist(err) {
				return err
			}
		case "update":
			current, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if setupContentSHA256(current) != change.before {
				return fmt.Errorf("refusing to overwrite a file changed after planning: %s", change.Path)
			}
		default:
			return fmt.Errorf("unsupported upgrade action: %s", change.Action)
		}
	}
	return nil
}

func UpgradeReviewTask(plan UpgradePlan) string {
	var output strings.Builder
	output.WriteString("# Open Knowledge Upgrade Review\n\n")
	fmt.Fprintf(&output, "Upgrade `%s` from OKF %s to OKF %s.\n\n", plan.Root, plan.From, plan.To)
	output.WriteString("Resolve only the semantic validation issues below. Preserve authored meaning and Git history. Do not remove content to make validation pass.\n\n")
	for _, issue := range plan.SemanticIssues {
		fmt.Fprintf(&output, "- `%s`: %s\n", issue.Path, issue.Message)
	}
	output.WriteString("\nAfter the review, run:\n\n")
	fmt.Fprintf(&output, "```sh\nokn upgrade %q --to %s\nokn check %q --gate\n```\n", plan.Root, plan.To, plan.Root)
	return output.String()
}
