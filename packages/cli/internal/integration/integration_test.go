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

func TestReconcileProjectSupportsMultipleHarnessesAndPreservesGuidance(t *testing.T) {
	repo, wiki := integrationFixture(t)
	if _, err := ReconcileProject(wiki, ProjectOptions{Harnesses: []string{"claude", "codex", "codex"}, ProjectSkills: true}); err != nil {
		t.Fatal(err)
	}
	config, err := LoadFromRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(config.Harnesses, ",") != "claude,codex" || config.Version != 3 {
		t.Fatalf("config = %#v", config)
	}
	skillPath := filepath.Join(repo, ".agents", "skills", "openknowledge", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, []byte("\n## Project guidance\n\nUse the release runbook.\n")...)
	if err := os.WriteFile(skillPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := Status(repo)
	if err != nil {
		t.Fatal(err)
	}
	if states := statusStates(status); states[".agents/skills/openknowledge/SKILL.md"] != "managed" {
		t.Fatalf("guidance edit changed managed-block status: %#v", states)
	}
	content = []byte(strings.Replace(string(content), "Use openknowledge registry list", "Broken managed instructions", 1))
	if err := os.WriteFile(skillPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err = Status(repo)
	if err != nil {
		t.Fatal(err)
	}
	if states := statusStates(status); states[".agents/skills/openknowledge/SKILL.md"] != "modified" {
		t.Fatalf("managed-block corruption was not detected: %#v", states)
	}
	if _, err := RepairProject(repo); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Use the release runbook.") || !strings.Contains(string(content), ProjectManagedStart) {
		t.Fatalf("repair did not preserve guidance and restore managed block:\n%s", content)
	}
	if strings.Contains(string(content), "The connected knowledge base is") {
		t.Fatalf("project skill hardcodes the knowledge-base path:\n%s", content)
	}
}

func statusStates(status StatusResult) map[string]string {
	states := map[string]string{}
	for _, file := range status.Files {
		states[file.Path] = file.State
	}
	return states
}

func TestSetObservationTogglesAllHarnessesIdempotently(t *testing.T) {
	repo, wiki := integrationFixture(t)
	if _, err := ReconcileProject(wiki, ProjectOptions{Harnesses: []string{"codex", "opencode"}, ProjectSkills: true}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := SetObservation(repo, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{".codex/hooks.json", ".opencode/plugins/openknowledge-observer.js"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing observer %s: %v", path, err)
		}
	}
	hooks, err := os.ReadFile(filepath.Join(repo, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(hooks), "openknowledge automation insights observe --runtime codex") != 1 {
		t.Fatalf("duplicate Codex observer:\n%s", hooks)
	}
	if _, err := SetObservation(repo, false); err != nil {
		t.Fatal(err)
	}
	config, err := LoadFromRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	if config.Observe || len(config.ObservedHarnesses) != 0 {
		t.Fatalf("observation remained enabled: %#v", config)
	}
	for _, path := range []string{".codex/hooks.json", ".opencode/plugins/openknowledge-observer.js"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("observer remains at %s: %v", path, err)
		}
	}
}

func TestObservationOnlyProjectDoesNotInstallSkills(t *testing.T) {
	repo, wiki := integrationFixture(t)
	result, err := ReconcileProject(wiki, ProjectOptions{Harnesses: []string{"codex", "opencode"}, Observe: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(result.Files, "\n"), "/skills/openknowledge/SKILL.md") {
		t.Fatalf("observation-only result installed a skill: %#v", result.Files)
	}
	for _, path := range []string{".agents/skills/openknowledge/SKILL.md", ".opencode/skills/openknowledge/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("observation-only setup installed %s: %v", path, err)
		}
	}
	for _, path := range []string{".codex/hooks.json", ".opencode/plugins/openknowledge-observer.js"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("observation-only setup missed %s: %v", path, err)
		}
	}
	status, err := Status(repo)
	if err != nil {
		t.Fatal(err)
	}
	if status.ProjectSkills || !status.Observe || len(status.Files) != 2 {
		t.Fatalf("observation-only status = %#v", status)
	}
	if _, err := RepairProject(repo); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".agents/skills/openknowledge/SKILL.md", ".opencode/skills/openknowledge/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("repair installed a skill for observation-only setup %s: %v", path, err)
		}
	}
}

func TestRemoveProjectSkillRemovesOnlyManagedBlock(t *testing.T) {
	repo, wiki := integrationFixture(t)
	if _, err := ReconcileProject(wiki, ProjectOptions{Harnesses: []string{"codex"}, ProjectSkills: true}); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(repo, ".agents", "skills", "openknowledge", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, []byte("\n## Team notes\n\nKeep this guidance.\n")...)
	if err := os.WriteFile(skillPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Remove(repo); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), ProjectManagedStart) || !strings.Contains(string(content), "Keep this guidance.") {
		t.Fatalf("remove did not preserve surrounding project guidance:\n%s", content)
	}
}

func TestLoadLegacyProjectSkillAndMigrateItsManagedKind(t *testing.T) {
	repo, wiki := integrationFixture(t)
	if _, err := InstallProjectWithOptions(wiki, InstallOptions{Runtime: "codex"}); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repo, filepath.FromSlash(ConfigPath))
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.Replace(string(content), "version = 3", "version = 2", 1)
	legacy = strings.Replace(legacy, "kind = 'project_skill'", "kind = 'file'", 1)
	if err := os.WriteFile(configPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFromRepository(repo)
	if err != nil || !loaded.ProjectSkills {
		t.Fatalf("load legacy config = %#v, %v", loaded, err)
	}
	if _, err := ReconcileProject(wiki, ProjectOptions{Harnesses: []string{"codex"}, ProjectSkills: true}); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadFromRepository(repo)
	if err != nil || loaded.Version != 3 || loaded.ManagedFiles[0].Kind != managedFileKindProjectSkill {
		t.Fatalf("migrated config = %#v, %v", loaded, err)
	}
}

func TestReconcileProjectSupportsKnowledgeBaseAtRepositoryRoot(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	if _, err := ReconcileProject(repo, ProjectOptions{Harnesses: []string{"codex"}, Observe: true, ProjectSkills: true}); err != nil {
		t.Fatal(err)
	}
	config, err := LoadFromRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	if config.KnowledgeBase != "." || config.Insights != "insights" || !config.Observe {
		t.Fatalf("root knowledge-base config = %#v", config)
	}
	for _, path := range []string{".agents/skills/openknowledge/SKILL.md", ".codex/hooks.json"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("root bundle missed %s: %v", path, err)
		}
	}
}

func TestReconcileProjectSupportsKnowledgeBaseOutsideGit(t *testing.T) {
	wiki := t.TempDir()
	result, err := ReconcileProject(wiki, ProjectOptions{Harnesses: []string{"codex"}, ProjectSkills: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Root != wiki {
		t.Fatalf("project root = %q, want %q", result.Root, wiki)
	}
	config, err := LoadFromRepository(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if config.KnowledgeBase != "." {
		t.Fatalf("knowledge base = %q, want root bundle", config.KnowledgeBase)
	}
	if _, err := os.Stat(filepath.Join(wiki, ".agents", "skills", "openknowledge", "SKILL.md")); err != nil {
		t.Fatalf("project skill missing: %v", err)
	}
}

func TestReconcileProjectAllowsMultipleBundlesInOneRepository(t *testing.T) {
	repo, first := integrationFixture(t)
	second := filepath.Join(repo, "ProductKnowledge")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileProject(first, ProjectOptions{Harnesses: []string{"codex"}, ProjectSkills: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileProject(second, ProjectOptions{Harnesses: []string{"claude"}, ProjectSkills: true}); err != nil {
		t.Fatal(err)
	}
	config, err := LoadFromRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	if config.KnowledgeBase != "Wiki" || strings.Join(config.Harnesses, ",") != "claude,codex" {
		t.Fatalf("multiple bundle config = %#v", config)
	}
	for _, path := range []string{".agents/skills/openknowledge/SKILL.md", ".claude/skills/openknowledge/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("multiple bundle setup missed %s: %v", path, err)
		}
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
