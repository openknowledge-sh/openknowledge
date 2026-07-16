package runtime

import (
	"strings"
	"testing"
)

func TestParseConfigIsStrictAndAppliesSafeDefaults(t *testing.T) {
	config, err := ParseConfig([]byte(`
[runtime]
state_dir = "state"

[artifact_store]
type = "filesystem"
path = "artifacts"

[[knowledge_bases]]
id = "wiki"
path = "Wiki"
route = "/docs"
publish = true
mcp = true
`))
	if err != nil {
		t.Fatal(err)
	}
	if config.Serve.Address != "127.0.0.1:8080" || config.Serve.MCPAccess != "public" {
		t.Fatalf("unexpected serve defaults: %#v", config.Serve)
	}
	if config.KnowledgeBases[0].Route != "/docs/" || config.KnowledgeBases[0].Spec != "0.1" {
		t.Fatalf("unexpected normalized knowledge base: %#v", config.KnowledgeBases[0])
	}
	if _, err := ParseConfig([]byte("[runtime]\nstate_dir='state'\nunknown=true\n")); err == nil || !strings.Contains(err.Error(), "missing in the target struct") {
		t.Fatalf("expected unknown field refusal, got %v", err)
	}
}

func TestParseConfigRejectsAmbiguousOrUnsafeValues(t *testing.T) {
	base := `
[runtime]
state_dir = "state"
[artifact_store]
type = "filesystem"
path = "artifacts"
[[knowledge_bases]]
id = "wiki"
path = "Wiki"
publish = true
`
	tests := []struct {
		name    string
		replace string
		with    string
		want    string
	}{
		{name: "store", replace: `type = "filesystem"`, with: `type = "s3"`, want: "must be filesystem"},
		{name: "route", replace: `publish = true`, with: "publish = true\nroute = \"../private\"", want: "must start with /"},
		{name: "id", replace: `id = "wiki"`, with: `id = "../wiki"`, want: "must contain only"},
		{name: "dot id", replace: `id = "wiki"`, with: `id = ".."`, want: "must contain only"},
		{name: "branch traversal", replace: `publish = true`, with: "publish = true\n[worker]\nproduction_branch = \"feature/../main\"", want: "production_branch is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(strings.Replace(base, test.replace, test.with, 1)))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}
