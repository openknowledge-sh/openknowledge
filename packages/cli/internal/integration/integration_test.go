package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallGlobalCreatesOnlySelectedDiscoverySkill(t *testing.T) {
	home := t.TempDir()
	result, err := InstallGlobalForRuntime(home, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || !strings.Contains(filepath.ToSlash(result.Files[0]), ".claude/skills/openknowledge/SKILL.md") {
		t.Fatalf("files = %#v", result.Files)
	}
	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "discovery-only") {
		t.Fatalf("not a discovery skill: %s", result.Files[0])
	}
	for _, path := range []string{
		filepath.Join(home, ".agents"),
		filepath.Join(home, ".codex", "hooks.json"),
		filepath.Join(home, ".opencode"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("global integration created unrelated path %s", path)
		}
	}
}

func TestInstallProjectCreatesOnlySelectedRuntimeWithoutObservation(t *testing.T) {
	repo, wiki := integrationFixture(t)
	result, err := InstallProjectWithOptions(wiki, InstallOptions{Runtime: "opencode"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Files, "\n") != ConfigPath+"\n.opencode/skills/openknowledge/SKILL.md" {
		t.Fatalf("files = %#v", result.Files)
	}
	config, err := LoadFromRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	if config.Runtime != "opencode" || config.Observe || len(config.ManagedFiles) != 1 {
		t.Fatalf("config = %#v", config)
	}
	for _, path := range []string{
		".agents",
		".claude",
		".codex",
		".opencode/plugins/openknowledge-observer.js",
	} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("non-observing OpenCode integration created %s", path)
		}
	}
}

func TestInstallProjectMergesSelectedObservationHookIdempotently(t *testing.T) {
	repo, wiki := integrationFixture(t)
	existing := []byte("{\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"existing\"}]}]\n  }\n}\n")
	if err := os.MkdirAll(filepath.Join(repo, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".codex", "hooks.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := InstallProjectWithOptions(wiki, InstallOptions{Runtime: "codex", Observe: true}); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(filepath.Join(repo, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if strings.Count(text, "openknowledge automation insights observe --runtime codex") != 1 || !strings.Contains(text, "existing") {
		t.Fatalf("unexpected hooks:\n%s", text)
	}
	for _, path := range []string{".claude", ".opencode"} {
		if _, err := os.Stat(filepath.Join(repo, path)); !os.IsNotExist(err) {
			t.Fatalf("Codex integration created unrelated path %s", path)
		}
	}
}

func TestStatusAndRemovePreserveUserChanges(t *testing.T) {
	repo, wiki := integrationFixture(t)
	if _, err := InstallProjectWithOptions(wiki, InstallOptions{Runtime: "codex", Observe: true}); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(repo, ".agents", "skills", "openknowledge", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("user-maintained\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := Status(repo)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, file := range status.Files {
		states[file.Path] = file.State
	}
	if states[".agents/skills/openknowledge/SKILL.md"] != "modified" ||
		states[".codex/hooks.json"] != "managed" {
		t.Fatalf("status = %#v", status.Files)
	}

	result, err := Remove(repo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Preserved, "\n") != ".agents/skills/openknowledge/SKILL.md" {
		t.Fatalf("preserved = %#v", result.Preserved)
	}
	content, err := os.ReadFile(skillPath)
	if err != nil || string(content) != "user-maintained\n" {
		t.Fatalf("modified skill was not preserved: %q, %v", content, err)
	}
	for _, path := range []string{ConfigPath, ".codex/hooks.json"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("remove left managed path %s", path)
		}
	}
}

func TestInstallProjectRejectsRuntimeSwitchAndModifiedManagedFile(t *testing.T) {
	repo, wiki := integrationFixture(t)
	if _, err := InstallProjectWithOptions(wiki, InstallOptions{Runtime: "claude"}); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallProjectWithOptions(wiki, InstallOptions{Runtime: "codex"}); err == nil || !strings.Contains(err.Error(), "remove that integration") {
		t.Fatalf("runtime switch error = %v", err)
	}
	skillPath := filepath.Join(repo, ".claude", "skills", "openknowledge", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallProjectWithOptions(wiki, InstallOptions{Runtime: "claude"}); err == nil || !strings.Contains(err.Error(), "modified managed file") {
		t.Fatalf("modified file error = %v", err)
	}
}

func TestLoadRejectsManagedPathsOutsideRuntimeSurface(t *testing.T) {
	repo, _ := integrationFixture(t)
	configPath := filepath.Join(repo, filepath.FromSlash(ConfigPath))
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `version = 2
knowledge_base = "Wiki"
insights = "Wiki/insights"
runtime = "codex"

[[managed_file]]
path = "../outside"
sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
kind = "file"
owned = true
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFromRepository(repo); err == nil || !strings.Contains(err.Error(), "invalid managed file") {
		t.Fatalf("unsafe managed path error = %v", err)
	}
}

func integrationFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	return repo, wiki
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
