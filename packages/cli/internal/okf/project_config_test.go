package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProjectConfigSupportsCompleteTypedTOML(t *testing.T) {
	config, err := ParseProjectConfig(`
[rules]
paths = [
  "rules",
  "policy-rules", # standard TOML multiline array and comment
]
enabled = "docs"

[validation.rules]
link-target = "error"
markdown-syntax = "off"

[html.theme]
name = "night #1"
stylesheet = "assets/theme.css"

[html.source]
github_base = "https://github.com/example/knowledge/blob/main"
entry = "Wiki"

[html.site]
base_url = "https://example.test/knowledge/"

[publish]
assets = ["whitepapers/*.pdf", "assets/public/**", "assets/public/**"]

[release]
branch = "stable"
policy = "last-passing"
outputs = ["viewer", "mcp"]

[maintenance]
mode = "autonomous"
agent = "claude"
delivery = "pull-request"
auto_merge = true
`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(config.Rules.Paths, ",") != "rules,policy-rules" || strings.Join(config.Rules.Enabled, ",") != "docs" {
		t.Fatalf("unexpected typed rules config: %#v", config.Rules)
	}
	if !config.Rules.PathsConfigured || !config.Rules.EnabledConfigured {
		t.Fatalf("expected explicit rules fields: %#v", config.Rules)
	}
	if config.Validation.Rules["link-target"] != ValidationSeverityError || config.Validation.Rules["markdown-syntax"] != ValidationSeverityOff {
		t.Fatalf("unexpected validation config: %#v", config.Validation)
	}
	if config.HTML.Theme.Name != "night #1" || config.HTML.Theme.Stylesheet != "assets/theme.css" {
		t.Fatalf("unexpected theme config: %#v", config.HTML.Theme)
	}
	if config.HTML.Source.Entry != "Wiki" || !strings.HasPrefix(config.HTML.Source.GitHubBase, "https://github.com/") {
		t.Fatalf("unexpected source config: %#v", config.HTML.Source)
	}
	if config.HTML.Site.BaseURL != "https://example.test/knowledge/" {
		t.Fatalf("unexpected site config: %#v", config.HTML.Site)
	}
	if strings.Join(config.Publish.Assets, ",") != "assets/public/**,whitepapers/*.pdf" {
		t.Fatalf("unexpected publish config: %#v", config.Publish)
	}
	if config.Release.Branch != "stable" || config.Release.Policy != ReleasePolicyLastPassing || strings.Join(config.Release.Outputs, ",") != "mcp,viewer" {
		t.Fatalf("unexpected release config: %#v", config.Release)
	}
	if config.Maintenance.Mode != MaintenanceModeAutonomous || config.Maintenance.Agent != MaintenanceAgentClaude || config.Maintenance.Delivery != MaintenanceDeliveryPullRequest || !config.Maintenance.AutoMerge {
		t.Fatalf("unexpected maintenance config: %#v", config.Maintenance)
	}
	defaultConfig, err := ParseProjectConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(defaultConfig.Rules.Enabled, ",") != "project,writing" || defaultConfig.Rules.EnabledConfigured {
		t.Fatalf("unexpected default rules: %#v", defaultConfig.Rules)
	}
	if defaultConfig.Release.Branch != "main" || defaultConfig.Release.Policy != ReleasePolicyFollowMain || len(defaultConfig.Release.Outputs) != 0 {
		t.Fatalf("unexpected default release config: %#v", defaultConfig.Release)
	}
	if defaultConfig.Maintenance.Mode != MaintenanceModeOff || defaultConfig.Maintenance.Agent != "codex" || defaultConfig.Maintenance.Delivery != MaintenanceDeliveryPullRequest || defaultConfig.Maintenance.AutoMerge {
		t.Fatalf("unexpected default maintenance config: %#v", defaultConfig.Maintenance)
	}
}

func TestParseProjectConfigAppliesReleaseAndMaintenanceDefaultsPerField(t *testing.T) {
	config, err := ParseProjectConfig(`
[release]
policy = "last-passing"

[maintenance]
mode = "propose"
auto_merge = true
`)
	if err != nil {
		t.Fatal(err)
	}
	if config.Release.Branch != "main" || config.Release.Policy != ReleasePolicyLastPassing {
		t.Fatalf("unexpected release config: %#v", config.Release)
	}
	if config.Maintenance.Mode != MaintenanceModePropose || config.Maintenance.Agent != "codex" || config.Maintenance.Delivery != MaintenanceDeliveryPullRequest || !config.Maintenance.AutoMerge {
		t.Fatalf("unexpected maintenance config: %#v", config.Maintenance)
	}
}

func TestParseProjectConfigFailsClosedAcrossEverySection(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{name: "syntax", content: "[html.theme\nname = \"night\"\n", expected: "expected character ]"},
		{name: "unknown root", content: "[deployment]\nurl = \"https://example.test\"\n", expected: "fields in the document are missing in the target struct"},
		{name: "unknown HTML field", content: "[html.theme]\ncss = \"theme.css\"\n", expected: "fields in the document are missing in the target struct"},
		{name: "wrong HTML type", content: "[html.site]\nbase_url = 42\n", expected: "cannot decode TOML integer"},
		{name: "wrong rules member", content: "[rules]\npaths = [\"rules\", 5]\n", expected: "rules.paths[1] must be a string"},
		{name: "wrong publish member", content: "[publish]\nassets = [\"assets/**\", 5]\n", expected: "publish.assets[1] must be a string"},
		{name: "removed publish enabled", content: "[publish]\nenabled = true\n", expected: "fields in the document are missing in the target struct"},
		{name: "unknown release field", content: "[release]\ntag = \"latest\"\n", expected: "fields in the document are missing in the target struct"},
		{name: "empty release branch", content: "[release]\nbranch = \" \"\n", expected: "release.branch must not be empty"},
		{name: "invalid release policy", content: "[release]\npolicy = \"always\"\n", expected: "release.policy must be follow-main or last-passing"},
		{name: "invalid release output", content: "[release]\noutputs = [\"viewer\", \"api\"]\n", expected: "release.outputs[1] must be viewer or mcp"},
		{name: "duplicate release output", content: "[release]\noutputs = [\"viewer\", \"viewer\"]\n", expected: "release.outputs[1] is duplicated"},
		{name: "wrong release output type", content: "[release]\noutputs = [\"viewer\", 5]\n", expected: "release.outputs[1] must be a string"},
		{name: "release output shorthand", content: "[release]\noutputs = \"viewer\"\n", expected: "release.outputs must be an array of strings"},
		{name: "unknown maintenance field", content: "[maintenance]\nschedule = \"daily\"\n", expected: "fields in the document are missing in the target struct"},
		{name: "invalid maintenance mode", content: "[maintenance]\nmode = \"manual\"\n", expected: "maintenance.mode must be off, propose, or autonomous"},
		{name: "empty maintenance agent", content: "[maintenance]\nagent = \" \"\n", expected: "maintenance.agent must be codex, claude, or opencode"},
		{name: "unknown maintenance agent", content: "[maintenance]\nagent = \"custom\"\n", expected: "maintenance.agent must be codex, claude, or opencode"},
		{name: "invalid maintenance delivery", content: "[maintenance]\ndelivery = \"direct\"\n", expected: "maintenance.delivery must be pull-request"},
		{name: "wrong maintenance auto merge type", content: "[maintenance]\nauto_merge = \"yes\"\n", expected: "cannot decode TOML string"},
		{name: "unsafe publish parent", content: "[publish]\nassets = \"../secret.txt\"\n", expected: "parent segments"},
		{name: "unsafe publish absolute", content: "[publish]\nassets = \"/secret.txt\"\n", expected: "clean bundle-relative pattern"},
		{name: "unsafe publish backslash", content: "[publish]\nassets = 'assets\\secret.txt'\n", expected: "forward slashes"},
		{name: "malformed publish glob", content: "[publish]\nassets = \"assets/[.txt\"\n", expected: "syntax error in pattern"},
		{name: "unknown validation rule", content: "[validation.rules]\nnot-a-rule = \"warn\"\n", expected: "unknown validation rule"},
		{name: "wrong validation severity type", content: "[validation.rules]\nlink-target = true\n", expected: "cannot assign boolean"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseProjectConfig(test.content); err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected %q, got %v", test.expected, err)
			}
		})
	}
}

func TestLegacyConfigEntryPointsShareStrictProjectParser(t *testing.T) {
	content := "[html.theme]\ncss = \"theme.css\"\n"
	if _, err := ParseValidationOptionsConfig(content); err == nil {
		t.Fatal("expected validation config entry point to reject unknown HTML config")
	}
	if _, err := ParseRuleCatalogConfig(content); err == nil {
		t.Fatal("expected rule config entry point to reject unknown HTML config")
	}
}

func TestLoadProjectConfigRejectsSymbolicLink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bundle")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside.toml")
	if err := os.WriteFile(outside, []byte("[html.theme]\nname = \"outside\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ValidationConfigFile)); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	if _, err := LoadProjectConfig(root); err == nil || !strings.Contains(err.Error(), "symbolic links are not supported") {
		t.Fatalf("expected linked project config to be rejected, got %v", err)
	}
}

func TestLoadProjectConfigDoesNotLoadLegacyConfigFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, legacyValidationConfigFile, "[release]\noutputs = [\"viewer\"]\n")

	config, err := LoadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.Path != "" || len(config.Release.Outputs) != 0 {
		t.Fatalf("legacy config must be ignored, got %#v", config)
	}
	if config.Release.Branch != "main" || config.Release.Policy != ReleasePolicyFollowMain || config.Maintenance.Mode != MaintenanceModeOff {
		t.Fatalf("missing canonical config must use defaults, got %#v", config)
	}
}
