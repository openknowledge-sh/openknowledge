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
enabled = true
assets = ["whitepapers/*.pdf", "assets/public/**", "assets/public/**"]
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
	if !config.Publish.Enabled || strings.Join(config.Publish.Assets, ",") != "assets/public/**,whitepapers/*.pdf" {
		t.Fatalf("unexpected publish config: %#v", config.Publish)
	}
	defaultConfig, err := ParseProjectConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if defaultConfig.Publish.Enabled {
		t.Fatal("public artifact publishing must default to disabled")
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
		{name: "wrong publish enabled type", content: "[publish]\nenabled = \"yes\"\n", expected: "cannot decode TOML string"},
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
