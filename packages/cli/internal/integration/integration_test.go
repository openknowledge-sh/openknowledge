package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallGlobalCreatesOnlyDiscoverySkills(t *testing.T) {
	home := t.TempDir()
	result, err := InstallGlobal(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("files = %#v", result.Files)
	}
	for _, path := range result.Files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "discovery-only") {
			t.Fatalf("not a discovery skill: %s", path)
		}
	}
	for _, path := range []string{filepath.Join(home, ".codex", "hooks.json"), filepath.Join(home, ".claude", "settings.json")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("global integration created hook file %s", path)
		}
	}
}

func TestInstallProjectWritesConfigSkillsAndMergesHooksIdempotently(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := []byte("{\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"existing\"}]}]\n  }\n}\n")
	if err := os.MkdirAll(filepath.Join(repo, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".codex", "hooks.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := InstallProject(wiki); err != nil {
			t.Fatal(err)
		}
	}
	config, err := LoadFromRepository(repo)
	if err != nil {
		t.Fatal(err)
	}
	if config.KnowledgeBase != "Wiki" || config.Insights != "Wiki/insights" {
		t.Fatalf("config = %#v", config)
	}
	for _, path := range []string{".agents/skills/openknowledge/SKILL.md", ".claude/skills/openknowledge/SKILL.md", ".opencode/plugins/openknowledge-observer.js"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	plugin, err := os.ReadFile(filepath.Join(repo, ".opencode", "plugins", "openknowledge-observer.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plugin), "client.session.messages") || !strings.Contains(string(plugin), "JSON.stringify({ event, trace })") {
		t.Fatalf("OpenCode plugin does not forward the session trace:\n%s", plugin)
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
	if strings.Count(text, "openknowledge insights observe --runtime codex") != 1 || !strings.Contains(text, "existing") {
		t.Fatalf("unexpected hooks:\n%s", text)
	}
	claudeSettings, err := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(claudeSettings), "openknowledge insights observe --runtime claude") != 1 || !strings.Contains(string(claudeSettings), `"async": true`) {
		t.Fatalf("unexpected Claude hooks:\n%s", claudeSettings)
	}
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
